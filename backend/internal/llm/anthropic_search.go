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
	converted, systemParts := anthropicMessagesFromHistory(messages)
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

	// The Messages API accepts custom tools alongside a server tool, so a grounded
	// Anthropic turn can also call our tools. The adapter previously sent only the
	// server tool and dropped the caller's, forcing the same either/or the Gemini
	// adapter forced.
	anthropicTools := []map[string]interface{}{searchTool}
	rawTools, _ := source["tools"].([]interface{})
	anthropicTools = append(anthropicTools, anthropicCustomTools(rawTools)...)

	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   converted,
		"tools":      anthropicTools,
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
		// A tool_use block is a request to call one of our tools.
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
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
	toolCalls := result.toolCalls()
	// Only append the markdown source block on a final answer; on a tool-call
	// turn the loop continues and a "Sources" list would land mid-conversation.
	if len(toolCalls) == 0 {
		content = appendSourceMap(content, sources)
	}
	message := map[string]interface{}{"role": "assistant", "content": content}
	if annotations := annotationsFromSourceMap(sources); annotations != nil {
		message["annotations"] = annotations
	}
	finishReason := "stop"
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
		finishReason = "tool_calls"
	}
	converted := map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"message":       message,
			"finish_reason": finishReason,
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

// anthropicPendingToolUse accumulates a streaming tool_use block.
type anthropicPendingToolUse struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

// anthropicStreamEvent models the Messages API SSE events this adapter reads.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
		// input_json_delta fragments carry a tool_use block's arguments.
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	ContentBlock struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Input   json.RawMessage `json:"input"`
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
		// A tool_use block streams as content_block_start (id and name) followed
		// by input_json_delta fragments, so arguments accumulate per block index
		// and are emitted once the block stops. Emitting on start would send an
		// empty argument object.
		pendingToolUse := map[int]*anthropicPendingToolUse{}
		emittedToolCalls := 0

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
				if event.Delta.Type == "input_json_delta" {
					if pending, ok := pendingToolUse[event.Index]; ok {
						pending.Arguments.WriteString(event.Delta.PartialJSON)
					}
				}
			case "content_block_stop":
				if pending, ok := pendingToolUse[event.Index]; ok {
					delete(pendingToolUse, event.Index)
					arguments := strings.TrimSpace(pending.Arguments.String())
					if arguments == "" {
						arguments = "{}"
					}
					writeOpenAIToolCallChunk(writer, []interface{}{map[string]interface{}{
						"index": emittedToolCalls,
						"id":    pending.ID,
						"type":  "function",
						"function": map[string]interface{}{
							"name":      pending.Name,
							"arguments": arguments,
						},
					}})
					emittedToolCalls++
				}
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" && strings.TrimSpace(event.ContentBlock.Name) != "" {
					pendingToolUse[event.Index] = &anthropicPendingToolUse{
						ID:   event.ContentBlock.ID,
						Name: event.ContentBlock.Name,
					}
				}
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
		// does, so the markdown block is not interleaved with the answer. On a
		// tool-call turn the loop continues, so only the structured citations go
		// out — a "Sources" block would land mid-conversation.
		if emittedToolCalls == 0 {
			if sourceText := appendSourceMap("", sources); sourceText != "" {
				writeOpenAIStreamChunkWithAnnotations(writer, sourceText, nil, annotationsFromSourceMap(sources))
			}
		} else if annotations := annotationsFromSourceMap(sources); len(annotations) > 0 {
			writeOpenAIStreamChunkWithAnnotations(writer, "", nil, annotations)
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

// anthropicCustomTools converts OpenAI tool definitions to Messages API custom
// tools.
//
// The Messages API takes a flat {name, description, input_schema} rather than
// OpenAI's nested {type, function:{...}}, and accepts these alongside a server
// tool — which is what makes grounding and tool calling coexist here.
func anthropicCustomTools(rawTools []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rawTools))
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if toolType, _ := tool["type"].(string); toolType != "" && toolType != "function" {
			continue
		}
		function, ok := tool["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		converted := map[string]interface{}{"name": name}
		if description, _ := function["description"].(string); description != "" {
			converted["description"] = description
		}
		// input_schema is required and must be an object schema. A tool with no
		// parameters still needs the field, so an empty object stands in.
		schema := map[string]interface{}{"type": "object"}
		if params, ok := function["parameters"].(map[string]interface{}); ok && len(params) > 0 {
			schema = params
		}
		converted["input_schema"] = schema
		out = append(out, converted)
	}
	return out
}

// anthropicMessagesFromHistory converts OpenAI-shaped history to Messages API
// messages, preserving tool calls and their results.
//
// Two shapes have to survive or a multi-round loop cannot work: an assistant turn
// carrying tool_calls becomes tool_use content blocks, and each role:"tool"
// result becomes a tool_result block on a user turn keyed by the same id. An
// earlier version skipped any message with empty text, which silently dropped
// both — an assistant turn that only calls a tool has no text.
func anthropicMessagesFromHistory(messages []interface{}) ([]map[string]interface{}, []string) {
	converted := make([]map[string]interface{}, 0, len(messages))
	var systemParts []string

	// Consecutive tool results must be merged onto one user turn: the Messages API
	// rejects a conversation with two user turns in a row.
	var pendingResults []map[string]interface{}
	flushResults := func() {
		if len(pendingResults) == 0 {
			return
		}
		blocks := make([]map[string]interface{}, len(pendingResults))
		copy(blocks, pendingResults)
		converted = append(converted, map[string]interface{}{"role": "user", "content": blocks})
		pendingResults = nil
	}

	for _, rawMessage := range messages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		content := messageText(message["content"])

		switch role {
		case "system", "developer":
			if strings.TrimSpace(content) != "" {
				systemParts = append(systemParts, content)
			}

		case "tool":
			toolCallID, _ := message["tool_call_id"].(string)
			block := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": strings.TrimSpace(toolCallID),
				"content":     content,
			}
			pendingResults = append(pendingResults, block)

		case "assistant":
			flushResults()
			blocks := make([]map[string]interface{}, 0, 2)
			if strings.TrimSpace(content) != "" {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": content})
			}
			if rawCalls, ok := message["tool_calls"].([]interface{}); ok {
				blocks = append(blocks, anthropicToolUseBlocks(rawCalls)...)
			}
			if len(blocks) > 0 {
				converted = append(converted, map[string]interface{}{"role": "assistant", "content": blocks})
			}

		default:
			flushResults()
			if strings.TrimSpace(content) != "" {
				converted = append(converted, map[string]interface{}{
					"role":    "user",
					"content": []map[string]interface{}{{"type": "text", "text": content}},
				})
			}
		}
	}
	flushResults()
	return converted, systemParts
}

// anthropicToolUseBlocks converts assistant tool calls to tool_use blocks.
func anthropicToolUseBlocks(rawCalls []interface{}) []map[string]interface{} {
	blocks := make([]map[string]interface{}, 0, len(rawCalls))
	for _, raw := range rawCalls {
		call, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		function, ok := call["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		id, _ := call["id"].(string)
		input := map[string]interface{}{}
		if encoded, _ := function["arguments"].(string); strings.TrimSpace(encoded) != "" {
			// Malformed arguments must not drop the block: the tool_use must exist
			// to pair with the tool_result that follows, or the request is invalid.
			_ = json.Unmarshal([]byte(encoded), &input)
		}
		blocks = append(blocks, map[string]interface{}{
			"type":  "tool_use",
			"id":    strings.TrimSpace(id),
			"name":  name,
			"input": input,
		})
	}
	return blocks
}

// toolCalls returns the response's tool_use blocks in OpenAI tool_calls shape.
func (r anthropicMessagesResponse) toolCalls() []interface{} {
	calls := make([]interface{}, 0, len(r.Content))
	for _, block := range r.Content {
		if block.Type != "tool_use" || strings.TrimSpace(block.Name) == "" {
			continue
		}
		arguments := "{}"
		if len(block.Input) > 0 {
			arguments = string(block.Input)
		}
		calls = append(calls, map[string]interface{}{
			"index": len(calls),
			"id":    block.ID,
			"type":  "function",
			"function": map[string]interface{}{
				"name":      block.Name,
				"arguments": arguments,
			},
		})
	}
	if len(calls) == 0 {
		return nil
	}
	return calls
}
