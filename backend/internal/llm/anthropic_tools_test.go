package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnthropicGroundingCarriesCustomTools covers unification on the Messages
// API, which accepts custom tools alongside a server tool. The adapter previously
// sent only the server tool and dropped the caller's, forcing the same either/or
// the Gemini adapter forced.
func TestAnthropicGroundingCarriesCustomTools(t *testing.T) {
	source := map[string]interface{}{
		"model":    "claude-opus-5",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "latest prices then total"}},
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "calculator",
					"description": "Evaluate an expression",
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"expression": map[string]interface{}{"type": "string"}},
					},
				},
			},
			// A tool with no parameters still needs input_schema, which is required.
			map[string]interface{}{
				"type":     "function",
				"function": map[string]interface{}{"name": "date_time"},
			},
		},
	}

	payload := groundedAnthropicRequest(t, source, &NativeSearchConfig{Enabled: true}, false)
	tools, ok := payload["tools"].([]interface{})
	if !ok || len(tools) != 3 {
		t.Fatalf("expected the server tool plus two custom tools, got %#v", payload["tools"])
	}

	server, _ := tools[0].(map[string]interface{})
	if server["name"] != "web_search" {
		t.Errorf("the server tool must come first, got %#v", server)
	}

	calculator, _ := tools[1].(map[string]interface{})
	if calculator["name"] != "calculator" {
		t.Errorf("custom tool name = %v", calculator["name"])
	}
	if _, present := calculator["function"]; present {
		t.Error("the Messages API takes a flat tool, not OpenAI's nested function wrapper")
	}
	if _, present := calculator["input_schema"]; !present {
		t.Error("input_schema is required")
	}

	bare, _ := tools[2].(map[string]interface{})
	schema, ok := bare["input_schema"].(map[string]interface{})
	if !ok || schema["type"] != "object" {
		t.Errorf("a parameterless tool still needs an object input_schema, got %#v", bare["input_schema"])
	}
}

// TestAnthropicHistoryPreservesToolRoundTrip is the requirement for a multi-round
// loop. An assistant turn that only calls a tool has no text, and skipping blank
// messages dropped both the call and its result — so round two carried no record
// that the tool had run.
func TestAnthropicHistoryPreservesToolRoundTrip(t *testing.T) {
	source := map[string]interface{}{
		"model": "claude-opus-5",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "Be terse."},
			map[string]interface{}{"role": "user", "content": "what is 2+2"},
			map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":       "toolu_1",
						"type":     "function",
						"function": map[string]interface{}{"name": "calculator", "arguments": `{"expression":"2+2"}`},
					},
				},
			},
			map[string]interface{}{"role": "tool", "tool_call_id": "toolu_1", "content": "4"},
		},
	}

	payload := groundedAnthropicRequest(t, source, &NativeSearchConfig{Enabled: true}, false)
	if payload["system"] != "Be terse." {
		t.Errorf("system = %v", payload["system"])
	}
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 3 {
		t.Fatalf("expected user, assistant(tool_use), user(tool_result); got %d: %#v", len(messages), messages)
	}

	assistant, _ := messages[1].(map[string]interface{})
	if assistant["role"] != "assistant" {
		t.Errorf("role = %v", assistant["role"])
	}
	blocks, _ := assistant["content"].([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("expected one tool_use block, got %#v", blocks)
	}
	use, _ := blocks[0].(map[string]interface{})
	if use["type"] != "tool_use" || use["name"] != "calculator" || use["id"] != "toolu_1" {
		t.Errorf("tool_use block = %#v", use)
	}
	input, _ := use["input"].(map[string]interface{})
	if input["expression"] != "2+2" {
		t.Errorf("arguments must be decoded into input: %#v", input)
	}

	result, _ := messages[2].(map[string]interface{})
	if result["role"] != "user" {
		t.Errorf("tool results go on a user turn, got %v", result["role"])
	}
	resultBlocks, _ := result["content"].([]interface{})
	block, _ := resultBlocks[0].(map[string]interface{})
	if block["type"] != "tool_result" || block["tool_use_id"] != "toolu_1" {
		t.Errorf("tool_result block = %#v", block)
	}
}

// TestAnthropicMergesConsecutiveToolResults guards a validation error: parallel
// tool calls produce several role:"tool" messages in a row, and the Messages API
// rejects two consecutive user turns.
func TestAnthropicMergesConsecutiveToolResults(t *testing.T) {
	source := map[string]interface{}{
		"model": "claude-opus-5",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "do two things"},
			map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{"id": "a", "function": map[string]interface{}{"name": "t1", "arguments": "{}"}},
					map[string]interface{}{"id": "b", "function": map[string]interface{}{"name": "t2", "arguments": "{}"}},
				},
			},
			map[string]interface{}{"role": "tool", "tool_call_id": "a", "content": "one"},
			map[string]interface{}{"role": "tool", "tool_call_id": "b", "content": "two"},
		},
	}

	payload := groundedAnthropicRequest(t, source, &NativeSearchConfig{Enabled: true}, false)
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 3 {
		t.Fatalf("both results must share one user turn; got %d messages: %#v", len(messages), messages)
	}
	last, _ := messages[2].(map[string]interface{})
	blocks, _ := last["content"].([]interface{})
	if len(blocks) != 2 {
		t.Fatalf("expected two tool_result blocks on one turn, got %#v", blocks)
	}
}

// TestAnthropicResponseSurfacesToolCalls: without a tool_calls finish reason the
// loop never continues.
func TestAnthropicResponseSurfacesToolCalls(t *testing.T) {
	body := `{"content":[
	  {"type":"text","text":"Checking."},
	  {"type":"tool_use","id":"toolu_9","name":"calculator","input":{"expression":"2+2"}}
	],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":4}}`

	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	converted, err := transformAnthropicResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(converted.Body)

	var payload struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("converted body: %v\n%s", err, out)
	}
	choice := payload.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %q", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(choice.Message.ToolCalls))
	}
	call := choice.Message.ToolCalls[0]
	// Anthropic supplies real ids, so they must be preserved rather than
	// regenerated — the tool_result has to reference the same one.
	if call.ID != "toolu_9" {
		t.Errorf("id = %q, want the provider's own id", call.ID)
	}
	if call.Function.Name != "calculator" || !strings.Contains(call.Function.Arguments, "2+2") {
		t.Errorf("call = %#v", call)
	}
	if strings.Contains(choice.Message.Content, "**Sources:**") {
		t.Error("source block must be withheld on a tool-call turn")
	}
}

// TestAnthropicStreamAccumulatesToolInput covers the streaming shape: a tool_use
// block arrives as content_block_start followed by input_json_delta fragments,
// so emitting on start would send empty arguments.
func TestAnthropicStreamAccumulatesToolInput(t *testing.T) {
	events := strings.Join([]string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":11,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Checking. "}}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_7","name":"calculator"}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"expression\":"}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"2+2\"}"}}`,
		`data: {"type":"content_block_stop","index":1}`,
		`data: {"type":"message_delta","usage":{"output_tokens":6}}`,
		`data: {"type":"message_stop"}`,
	}, "\n\n") + "\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, events)
	}))
	defer srv.Close()

	upstream, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(wrapAnthropicStream(upstream).Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)

	if !strings.Contains(text, "Checking. ") {
		t.Errorf("text lost:\n%s", text)
	}
	if !strings.Contains(text, `"tool_calls"`) || !strings.Contains(text, "toolu_7") {
		t.Errorf("tool call not emitted:\n%s", text)
	}
	// The fragments must be reassembled into one complete argument object.
	if !strings.Contains(text, `2+2`) {
		t.Errorf("input fragments were not reassembled:\n%s", text)
	}
	if strings.Count(text, "toolu_7") != 1 {
		t.Errorf("the tool call must be emitted exactly once:\n%s", text)
	}
	if !strings.HasSuffix(strings.TrimSpace(text), "data: [DONE]") {
		t.Errorf("stream must terminate:\n%s", text)
	}
}

func TestAnthropicCustomToolsSkipsServerTools(t *testing.T) {
	// A provider-native server tool must not be re-declared as a custom tool.
	converted := anthropicCustomTools([]interface{}{
		map[string]interface{}{"type": "web_search_20260209", "name": "web_search"},
		map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "ok"}},
	})
	if len(converted) != 1 || converted[0]["name"] != "ok" {
		t.Errorf("expected only the function tool, got %#v", converted)
	}
}
