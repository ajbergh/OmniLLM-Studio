package llm

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// NativeSearchConfig is serialized into an internal request marker. A transport
// adapter converts that marker into the active provider's native search format,
// allowing the existing provider-neutral ChatComplete/ChatStream paths to stay
// unchanged.
type NativeSearchConfig struct {
	Enabled         bool          `json:"enabled"`
	ContextSize     string        `json:"context_size,omitempty"`
	MaxResults      int           `json:"max_results,omitempty"`
	MaxTotalResults int           `json:"max_total_results,omitempty"`
	AllowedDomains  []string      `json:"allowed_domains,omitempty"`
	ExcludedDomains []string      `json:"excluded_domains,omitempty"`
	UserLocation    *UserLocation `json:"user_location,omitempty"`
	AnswerVerbosity string        `json:"answer_verbosity,omitempty"`
}

type UserLocation struct {
	Type     string `json:"type,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

const nativeSearchPluginPrefix = "omnillm-native-search:"

// NativeSearchPlugin carries provider-neutral search options through the
// existing ChatRequest without changing its public contract. The marker is
// removed by nativeSearchTransport before the request leaves the process.
func NativeSearchPlugin(cfg *NativeSearchConfig) Plugin {
	if cfg == nil || !cfg.Enabled {
		return Plugin{}
	}
	payload, _ := json.Marshal(cfg)
	return Plugin{ID: nativeSearchPluginPrefix + base64.RawURLEncoding.EncodeToString(payload)}
}

func nativeSearchConfigFromBody(body map[string]interface{}) (*NativeSearchConfig, bool) {
	rawPlugins, ok := body["plugins"].([]interface{})
	if !ok || len(rawPlugins) == 0 {
		return nil, false
	}
	kept := make([]interface{}, 0, len(rawPlugins))
	var cfg *NativeSearchConfig
	for _, raw := range rawPlugins {
		plugin, ok := raw.(map[string]interface{})
		if !ok {
			kept = append(kept, raw)
			continue
		}
		id, _ := plugin["id"].(string)
		if !strings.HasPrefix(id, nativeSearchPluginPrefix) {
			kept = append(kept, raw)
			continue
		}
		encoded := strings.TrimPrefix(id, nativeSearchPluginPrefix)
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		var parsed NativeSearchConfig
		if json.Unmarshal(decoded, &parsed) == nil && parsed.Enabled {
			cfg = &parsed
		}
	}
	if cfg != nil {
		filtered := kept[:0]
		for _, raw := range kept {
			plugin, ok := raw.(map[string]interface{})
			if ok {
				id, _ := plugin["id"].(string)
				if strings.EqualFold(strings.TrimSpace(id), "web") {
					continue
				}
			}
			filtered = append(filtered, raw)
		}
		kept = filtered
	}
	if len(kept) == 0 {
		delete(body, "plugins")
	} else {
		body["plugins"] = kept
	}
	return cfg, cfg != nil
}

type nativeSearchTransport struct {
	base http.RoundTripper
}

func (t *nativeSearchTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.Body == nil || req.Method != http.MethodPost {
		return t.base.RoundTrip(req)
	}
	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(requestBody))

	var payload map[string]interface{}
	if json.Unmarshal(requestBody, &payload) != nil {
		return t.base.RoundTrip(req)
	}
	cfg, ok := nativeSearchConfigFromBody(payload)
	if !ok {
		return t.base.RoundTrip(req)
	}

	provider := nativeProviderForURL(req.URL)
	stream, _ := payload["stream"].(bool)
	switch provider {
	case "gemini":
		if err := transformGeminiGroundedRequest(req, payload, cfg, stream); err != nil {
			return nil, err
		}
	case "openrouter":
		applyOpenRouterSearch(payload, cfg)
	default:
		applyOpenAIWebSearch(payload, cfg)
	}

	if provider != "gemini" {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		setRequestBody(req, encoded)
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	if provider == "gemini" {
		if stream {
			return wrapGeminiStream(resp), nil
		}
		return transformGeminiResponse(resp)
	}
	if stream {
		return wrapAnnotationStream(resp), nil
	}
	return appendAnnotationsToResponse(resp)
}

func nativeProviderForURL(value *url.URL) string {
	if value == nil {
		return "openai"
	}
	host := strings.ToLower(value.Hostname())
	switch {
	case strings.Contains(host, "generativelanguage.googleapis.com"):
		return "gemini"
	case strings.Contains(host, "openrouter.ai"):
		return "openrouter"
	default:
		return "openai"
	}
}

func applyOpenAIWebSearch(body map[string]interface{}, cfg *NativeSearchConfig) {
	options := map[string]interface{}{}
	if cfg.ContextSize != "" {
		options["search_context_size"] = cfg.ContextSize
	}
	if location := openAILocation(cfg.UserLocation); location != nil {
		options["user_location"] = map[string]interface{}{"approximate": location}
	}
	body["web_search_options"] = options
	model, _ := body["model"].(string)
	if cfg.AnswerVerbosity != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-5") {
		body["verbosity"] = cfg.AnswerVerbosity
	} else {
		delete(body, "verbosity")
	}
}

func applyOpenRouterSearch(body map[string]interface{}, cfg *NativeSearchConfig) {
	params := map[string]interface{}{"engine": "auto"}
	if cfg.ContextSize != "" {
		params["search_context_size"] = cfg.ContextSize
	}
	if cfg.MaxResults > 0 {
		params["max_results"] = cfg.MaxResults
	}
	if cfg.MaxTotalResults > 0 {
		params["max_total_results"] = cfg.MaxTotalResults
	}
	if len(cfg.AllowedDomains) > 0 {
		params["allowed_domains"] = cfg.AllowedDomains
	}
	if len(cfg.ExcludedDomains) > 0 {
		params["excluded_domains"] = cfg.ExcludedDomains
	}
	if location := openRouterLocation(cfg.UserLocation); location != nil {
		params["user_location"] = location
	}
	webTool := map[string]interface{}{
		"type":       "openrouter:web_search",
		"parameters": params,
	}
	if tools, ok := body["tools"].([]interface{}); ok {
		body["tools"] = append(tools, webTool)
	} else {
		body["tools"] = []interface{}{webTool}
	}
}

func openAILocation(location *UserLocation) map[string]interface{} {
	if location == nil {
		return nil
	}
	result := map[string]interface{}{"type": "approximate"}
	copyLocationFields(result, location)
	return result
}

func openRouterLocation(location *UserLocation) map[string]interface{} {
	if location == nil {
		return nil
	}
	result := map[string]interface{}{"type": "approximate"}
	copyLocationFields(result, location)
	return result
}

func copyLocationFields(target map[string]interface{}, location *UserLocation) {
	if location.City != "" {
		target["city"] = location.City
	}
	if location.Region != "" {
		target["region"] = location.Region
	}
	if location.Country != "" {
		target["country"] = location.Country
	}
	if location.Timezone != "" {
		target["timezone"] = location.Timezone
	}
}

func transformGeminiGroundedRequest(req *http.Request, source map[string]interface{}, cfg *NativeSearchConfig, stream bool) error {
	model, _ := source["model"].(string)
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("gemini grounded search requires a model")
	}
	messages, _ := source["messages"].([]interface{})
	contents := make([]map[string]interface{}, 0, len(messages))
	var systemParts []string
	for _, rawMessage := range messages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		content := messageText(message["content"])
		if strings.TrimSpace(content) == "" {
			continue
		}
		if role == "system" || role == "developer" {
			systemParts = append(systemParts, content)
			continue
		}
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  geminiRole,
			"parts": []map[string]string{{"text": content}},
		})
	}
	payload := map[string]interface{}{
		"contents": contents,
		"tools":    []map[string]interface{}{{"google_search": map[string]interface{}{}}},
	}
	if len(systemParts) > 0 {
		payload["system_instruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": strings.Join(systemParts, "\n\n")}},
		}
	}
	generation := map[string]interface{}{}
	if value, ok := source["max_tokens"]; ok {
		generation["maxOutputTokens"] = value
	}
	if value, ok := source["temperature"]; ok {
		generation["temperature"] = value
	}
	if len(generation) > 0 {
		payload["generationConfig"] = generation
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	path := strings.TrimSuffix(req.URL.Path, "/openai/chat/completions")
	method := "generateContent"
	if stream {
		method = "streamGenerateContent"
	}
	req.URL.Path = fmt.Sprintf("%s/models/%s:%s", path, url.PathEscape(strings.TrimPrefix(model, "models/")), method)
	query := req.URL.Query()
	if stream {
		query.Set("alt", "sse")
	} else {
		query.Del("alt")
	}
	req.URL.RawQuery = query.Encode()

	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		req.Header.Set("x-goog-api-key", strings.TrimPrefix(auth, "Bearer "))
		req.Header.Del("Authorization")
	}
	setRequestBody(req, encoded)
	return nil
}

func messageText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		var parts []string
		for _, raw := range typed {
			part, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func setRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Del("Content-Length")
}

type geminiGroundedResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		GroundingMetadata struct {
			GroundingChunks []struct {
				Web struct {
					URI   string `json:"uri"`
					Title string `json:"title"`
				} `json:"web"`
			} `json:"groundingChunks"`
		} `json:"groundingMetadata"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

func (value geminiGroundedResponse) textAndSources() (string, map[string]string) {
	var text strings.Builder
	sources := map[string]string{}
	for _, candidate := range value.Candidates {
		for _, part := range candidate.Content.Parts {
			text.WriteString(part.Text)
		}
		for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
			sourceURL := strings.TrimSpace(chunk.Web.URI)
			if sourceURL == "" {
				continue
			}
			title := strings.TrimSpace(chunk.Web.Title)
			if title == "" {
				title = sourceURL
			}
			sources[sourceURL] = title
		}
	}
	return text.String(), sources
}

func transformGeminiResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	var result geminiGroundedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	content, sources := result.textAndSources()
	content = appendSourceMap(content, sources)
	converted := map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"message":       map[string]interface{}{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]interface{}{
			"prompt_tokens":     result.UsageMetadata.PromptTokenCount,
			"completion_tokens": result.UsageMetadata.CandidatesTokenCount,
		},
	}
	encoded, _ := json.Marshal(converted)
	setResponseBody(resp, encoded, "application/json")
	return resp, nil
}

func wrapGeminiStream(resp *http.Response) *http.Response {
	original := resp.Body
	reader, writer := io.Pipe()
	resp.Body = reader
	resp.Header.Set("Content-Type", "text/event-stream")
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	go func() {
		defer writer.Close()
		defer original.Close()
		scanner := bufio.NewScanner(original)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		sources := map[string]string{}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			var chunk geminiGroundedResponse
			if json.Unmarshal([]byte(data), &chunk) != nil {
				continue
			}
			text, chunkSources := chunk.textAndSources()
			for sourceURL, title := range chunkSources {
				sources[sourceURL] = title
			}
			if text != "" {
				writeOpenAIStreamChunk(writer, text, nil)
			}
			if chunk.UsageMetadata.PromptTokenCount > 0 || chunk.UsageMetadata.CandidatesTokenCount > 0 {
				usage := map[string]interface{}{
					"prompt_tokens":     chunk.UsageMetadata.PromptTokenCount,
					"completion_tokens": chunk.UsageMetadata.CandidatesTokenCount,
				}
				writeOpenAIStreamChunk(writer, "", usage)
			}
		}
		if sourceText := appendSourceMap("", sources); sourceText != "" {
			writeOpenAIStreamChunk(writer, sourceText, nil)
		}
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
	}()
	return resp
}

func writeOpenAIStreamChunk(writer io.Writer, content string, usage map[string]interface{}) {
	chunk := map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"delta": map[string]interface{}{"content": content},
		}},
	}
	if usage != nil {
		chunk["usage"] = usage
	}
	encoded, _ := json.Marshal(chunk)
	_, _ = fmt.Fprintf(writer, "data: %s\n\n", encoded)
}

type urlCitation struct {
	Type        string `json:"type"`
	URLCitation struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"url_citation"`
}

func appendAnnotationsToResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	choices, _ := payload["choices"].([]interface{})
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]interface{})
		message, _ := choice["message"].(map[string]interface{})
		content, _ := message["content"].(string)
		annotations := citationsFromValue(message["annotations"])
		message["content"] = appendCitations(content, annotations, nil)
	}
	encoded, _ := json.Marshal(payload)
	setResponseBody(resp, encoded, "application/json")
	return resp, nil
}

func wrapAnnotationStream(resp *http.Response) *http.Response {
	original := resp.Body
	reader, writer := io.Pipe()
	resp.Body = reader
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	go func() {
		defer writer.Close()
		defer original.Close()
		scanner := bufio.NewScanner(original)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		seen := map[string]bool{}
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "data:") || strings.HasSuffix(trimmed, "[DONE]") {
				_, _ = io.WriteString(writer, line+"\n")
				if trimmed == "data: [DONE]" {
					_, _ = io.WriteString(writer, "\n")
				}
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			var payload map[string]interface{}
			if json.Unmarshal([]byte(data), &payload) != nil {
				_, _ = io.WriteString(writer, line+"\n\n")
				continue
			}
			choices, _ := payload["choices"].([]interface{})
			if len(choices) > 0 {
				choice, _ := choices[0].(map[string]interface{})
				delta, _ := choice["delta"].(map[string]interface{})
				content, _ := delta["content"].(string)
				delta["content"] = appendCitations(content, citationsFromValue(delta["annotations"]), seen)
			}
			encoded, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(writer, "data: %s\n\n", encoded)
		}
	}()
	return resp
}

func citationsFromValue(value interface{}) []urlCitation {
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) == "null" {
		return nil
	}
	var citations []urlCitation
	_ = json.Unmarshal(encoded, &citations)
	return citations
}

func appendCitations(content string, citations []urlCitation, seen map[string]bool) string {
	if len(citations) == 0 {
		return content
	}
	localSeen := seen
	if localSeen == nil {
		localSeen = map[string]bool{}
	}
	var lines []string
	for _, citation := range citations {
		sourceURL := strings.TrimSpace(citation.URLCitation.URL)
		if sourceURL == "" || localSeen[sourceURL] {
			continue
		}
		localSeen[sourceURL] = true
		title := strings.TrimSpace(citation.URLCitation.Title)
		if title == "" {
			title = sourceURL
		}
		lines = append(lines, fmt.Sprintf("- [%s](%s)", strings.ReplaceAll(title, "]", "\\]"), sourceURL))
	}
	if len(lines) == 0 {
		return content
	}
	return strings.TrimSpace(content) + "\n\n**Sources:**\n" + strings.Join(lines, "\n")
}

func appendSourceMap(content string, sources map[string]string) string {
	if len(sources) == 0 {
		return strings.TrimSpace(content)
	}
	urls := make([]string, 0, len(sources))
	for sourceURL := range sources {
		urls = append(urls, sourceURL)
	}
	sort.Strings(urls)
	var lines []string
	for _, sourceURL := range urls {
		title := strings.TrimSpace(sources[sourceURL])
		if title == "" {
			title = sourceURL
		}
		lines = append(lines, fmt.Sprintf("- [%s](%s)", strings.ReplaceAll(title, "]", "\\]"), sourceURL))
	}
	prefix := strings.TrimSpace(content)
	if prefix != "" {
		prefix += "\n\n"
	}
	return prefix + "**Sources:**\n" + strings.Join(lines, "\n")
}

func setResponseBody(resp *http.Response, body []byte, contentType string) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Type", contentType)
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
}
