package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Anthropic native web search runs on the Messages API as a server tool. It is
// not available through the OpenAI-compatibility endpoint this repository
// otherwise uses for Anthropic, so a grounded request has to be converted to
// /v1/messages and its response converted back into the OpenAI-compatible shape
// Chat Studio's single SSE parser expects. That is the same strategy the Gemini
// adapter uses.
//
// Before this existed, SupportsNativeSearch returned false for Anthropic, so
// every Claude model was forced onto the Brave/DuckDuckGo fallback.

const (
	// anthropicWebSearchToolCurrent is the dynamic-filtering web search tool,
	// available on Opus 4.6 and later and Sonnet 4.6 and later.
	anthropicWebSearchToolCurrent = "web_search_20260209"
	// anthropicWebSearchToolBasic is the original variant, for older models.
	anthropicWebSearchToolBasic = "web_search_20250305"
	// anthropicMessagesVersion is the API version header the Messages API
	// requires.
	anthropicMessagesVersion = "2023-06-01"
	// anthropicMaxSearchUses bounds how many searches one turn may perform. The
	// planner already decides breadth; this is a hard ceiling so a grounded turn
	// cannot run away.
	anthropicMaxSearchUses = 5
)

// anthropicWebSearchToolType picks the newest tool variant the model supports.
//
// Sending the dynamic-filtering variant to a model that predates it is a request
// error, and sending the basic variant to a newer model silently gives up
// filtering, so the choice is made from the model name rather than defaulted.
func anthropicWebSearchToolType(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	// Families that support the dynamic-filtering variant.
	for _, prefix := range []string{
		"claude-fable-", "claude-mythos-",
		"claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6",
		"claude-sonnet-5", "claude-sonnet-4-6",
	} {
		if strings.HasPrefix(model, prefix) {
			return anthropicWebSearchToolCurrent
		}
	}
	return anthropicWebSearchToolBasic
}

// SupportsAnthropicNativeSearch reports whether a Claude model can run the
// server-side web search tool.
//
// Claude 3.x and earlier predate server tools entirely; sending one would be a
// request error rather than a graceful degradation, so they stay on the local
// fallback.
func SupportsAnthropicNativeSearch(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	for _, prefix := range []string{
		"claude-fable-", "claude-mythos-",
		"claude-opus-4", "claude-opus-5",
		"claude-sonnet-4", "claude-sonnet-5",
		"claude-haiku-4",
	} {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// transformAnthropicGroundedRequest rewrites an OpenAI-compatible request into a
// Messages API request carrying the web search server tool.
func transformAnthropicGroundedRequest(
	req *http.Request,
	source map[string]interface{},
	cfg *NativeSearchConfig,
	stream bool,
) error {
	model, _ := source["model"].(string)
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("anthropic grounded search requires a model")
	}

	messages, _ := source["messages"].([]interface{})
	converted := make([]map[string]interface{}, 0, len(messages))
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
		// The Messages API takes system content in a dedicated top-level field,
		// not as a message role.
		if role == "system" || role == "developer" {
			systemParts = append(systemParts, content)
			continue
		}
		messagesRole := "user"
		if role == "assistant" {
			messagesRole = "assistant"
		}
		converted = append(converted, map[string]interface{}{
			"role":    messagesRole,
			"content": []map[string]interface{}{{"type": "text", "text": content}},
		})
	}
	if len(converted) == 0 {
		return fmt.Errorf("anthropic grounded search requires at least one message")
	}
	// The Messages API rejects a conversation that does not begin with a user
	// turn. A leading assistant message can happen after history trimming.
	if converted[0]["role"] != "user" {
		converted = append([]map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{{"type": "text", "text": "Continue."}}},
		}, converted...)
	}

	searchTool := map[string]interface{}{
		"type":     anthropicWebSearchToolType(model),
		"name":     "web_search",
		"max_uses": anthropicMaxSearchUses,
	}
	if len(cfg.AllowedDomains) > 0 {
		// allowed_domains and blocked_domains are mutually exclusive.
		searchTool["allowed_domains"] = cfg.AllowedDomains
	} else if len(cfg.ExcludedDomains) > 0 {
		searchTool["blocked_domains"] = cfg.ExcludedDomains
	}
	if location := anthropicLocation(cfg.UserLocation); location != nil {
		searchTool["user_location"] = location
	}

	// max_tokens is required by the Messages API, unlike Chat Completions.
	maxTokens := 4096
	if value, ok := numericField(source["max_tokens"]); ok && value > 0 {
		maxTokens = int(value)
	}

	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   converted,
		"tools":      []map[string]interface{}{searchTool},
	}
	if len(systemParts) > 0 {
		payload["system"] = strings.Join(systemParts, "\n\n")
	}
	if value, ok := numericField(source["temperature"]); ok {
		payload["temperature"] = value
	}
	if stream {
		payload["stream"] = true
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// /v1/chat/completions -> /v1/messages, preserving any base-path prefix a
	// self-hosted gateway may add.
	req.URL.Path = strings.TrimSuffix(req.URL.Path, "/chat/completions") + "/messages"
	req.Header.Set("anthropic-version", anthropicMessagesVersion)
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		req.Header.Set("x-api-key", strings.TrimPrefix(auth, "Bearer "))
		req.Header.Del("Authorization")
	}
	setRequestBody(req, encoded)
	return nil
}

func anthropicLocation(location *UserLocation) map[string]interface{} {
	if location == nil {
		return nil
	}
	result := map[string]interface{}{"type": "approximate"}
	if location.City != "" {
		result["city"] = location.City
	}
	if location.Region != "" {
		result["region"] = location.Region
	}
	if location.Country != "" {
		result["country"] = location.Country
	}
	if location.Timezone != "" {
		result["timezone"] = location.Timezone
	}
	if len(result) == 1 {
		return nil
	}
	return result
}

// numericField reads a JSON number that may have decoded as float64 or json.Number.
func numericField(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// anthropicMessagesResponse models the parts of a Messages API response this
// adapter needs.
type anthropicMessagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
		// Web search results arrive as a tool-result block whose content is a
		// list of results. On error the same field is an object instead, so it is
		// decoded loosely and type-checked below.
		Content json.RawMessage `json:"content"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicWebSearchResult struct {
	Type             string `json:"type"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	PageAge          string `json:"page_age"`
	EncryptedContent string `json:"encrypted_content"`
}

// searchResultsFromBlock decodes a web_search_tool_result block's content.
//
// Server-tool errors return HTTP 200 with an error *object* where a success
// returns a *list*, so the shape has to be checked rather than assumed.
func searchResultsFromBlock(raw json.RawMessage) []anthropicWebSearchResult {
	if len(raw) == 0 {
		return nil
	}
	var results []anthropicWebSearchResult
	if json.Unmarshal(raw, &results) != nil {
		return nil
	}
	return results
}

// textAndCitations flattens the response into answer text plus its sources.
func (r anthropicMessagesResponse) textAndCitations() (string, map[string]string) {
	var text strings.Builder
	sources := map[string]string{}
	for _, block := range r.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "web_search_tool_result":
			for _, result := range searchResultsFromBlock(block.Content) {
				url := strings.TrimSpace(result.URL)
				if url == "" {
					continue
				}
				title := strings.TrimSpace(result.Title)
				if title == "" {
					title = url
				}
				sources[url] = title
			}
		}
	}
	return text.String(), sources
}

// transformAnthropicResponse converts a Messages API response into the
// OpenAI-compatible shape, carrying sources as both a markdown block and
// structured annotations.
func transformAnthropicResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	var result anthropicMessagesResponse
	// An empty content array means this is not a Messages response — most likely
	// an error envelope. Converting it anyway would produce an empty but
	// apparently successful answer and discard the detail the caller needs, so
	// the body is handed back untouched. json.Unmarshal alone is not a
	// sufficient check: unknown fields decode without error.
	if json.Unmarshal(body, &result) != nil || len(result.Content) == 0 {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	content, sources := result.textAndCitations()
	content = appendSourceMap(content, sources)
	message := map[string]interface{}{"role": "assistant", "content": content}
	if annotations := annotationsFromSourceMap(sources); annotations != nil {
		message["annotations"] = annotations
	}
	converted := map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"message":       message,
			"finish_reason": "stop",
		}},
		"usage": map[string]interface{}{
			"prompt_tokens":     result.Usage.InputTokens,
			"completion_tokens": result.Usage.OutputTokens,
		},
	}
	encoded, _ := json.Marshal(converted)
	setResponseBody(resp, encoded, "application/json")
	return resp, nil
}

// anthropicStreamEvent models the Messages API SSE events this adapter reads.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	ContentBlock struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	} `json:"content_block"`
	Message struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// wrapAnthropicStream converts a Messages API SSE stream into OpenAI-compatible
// chunks.
//
// The Messages API uses named events (content_block_delta, message_delta, …)
// rather than OpenAI's uniform choice deltas, so each is translated here and the
// rest of the pipeline keeps one parser.
func wrapAnthropicStream(resp *http.Response) *http.Response {
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
		inputTokens := 0
		outputTokens := 0

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			var event anthropicStreamEvent
			if json.Unmarshal([]byte(data), &event) != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					writeOpenAIStreamChunk(writer, event.Delta.Text, nil)
				}
			case "content_block_start":
				if event.ContentBlock.Type == "web_search_tool_result" {
					for _, result := range searchResultsFromBlock(event.ContentBlock.Content) {
						url := strings.TrimSpace(result.URL)
						if url == "" {
							continue
						}
						title := strings.TrimSpace(result.Title)
						if title == "" {
							title = url
						}
						sources[url] = title
					}
				}
			case "message_start":
				if event.Message.Usage.InputTokens > 0 {
					inputTokens = event.Message.Usage.InputTokens
				}
			case "message_delta":
				if event.Usage.OutputTokens > 0 {
					outputTokens = event.Usage.OutputTokens
				}
			}
		}

		// Sources are emitted once at the end, the same way the Gemini adapter
		// does, so the markdown block is not interleaved with the answer.
		if sourceText := appendSourceMap("", sources); sourceText != "" {
			writeOpenAIStreamChunkWithAnnotations(writer, sourceText, nil, annotationsFromSourceMap(sources))
		}
		if inputTokens > 0 || outputTokens > 0 {
			writeOpenAIStreamChunk(writer, "", map[string]interface{}{
				"prompt_tokens":     inputTokens,
				"completion_tokens": outputTokens,
			})
		}
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
	}()

	return resp
}
