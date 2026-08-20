package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupportsAnthropicNativeSearch(t *testing.T) {
	supported := []string{
		"claude-opus-5", "claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6",
		"claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-4-5",
		"claude-fable-5", "claude-mythos-5", "CLAUDE-OPUS-5",
	}
	for _, model := range supported {
		if !SupportsAnthropicNativeSearch(model) {
			t.Errorf("SupportsAnthropicNativeSearch(%q) = false, want true", model)
		}
	}
	// Claude 3.x predates server tools; sending one is a request error, not a
	// graceful degradation, so it must stay on the local fallback.
	for _, model := range []string{"claude-3-5-sonnet-20241022", "claude-2.1", "", "  "} {
		if SupportsAnthropicNativeSearch(model) {
			t.Errorf("SupportsAnthropicNativeSearch(%q) = true, want false", model)
		}
	}
}

// TestAnthropicWebSearchToolType pins the version selection. Sending the
// dynamic-filtering variant to an older model is a request error, and sending
// the basic variant to a newer one silently drops filtering.
func TestAnthropicWebSearchToolType(t *testing.T) {
	current := []string{"claude-opus-5", "claude-opus-4-6", "claude-sonnet-5", "claude-sonnet-4-6", "claude-fable-5"}
	for _, model := range current {
		if got := anthropicWebSearchToolType(model); got != anthropicWebSearchToolCurrent {
			t.Errorf("anthropicWebSearchToolType(%q) = %q, want %q", model, got, anthropicWebSearchToolCurrent)
		}
	}
	older := []string{"claude-haiku-4-5", "claude-sonnet-4-5", "claude-3-5-sonnet-20241022"}
	for _, model := range older {
		if got := anthropicWebSearchToolType(model); got != anthropicWebSearchToolBasic {
			t.Errorf("anthropicWebSearchToolType(%q) = %q, want %q", model, got, anthropicWebSearchToolBasic)
		}
	}
}

func groundedAnthropicRequest(t *testing.T, source map[string]interface{}, cfg *NativeSearchConfig, stream bool) map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	if err := transformAnthropicGroundedRequest(req, source, cfg, stream); err != nil {
		t.Fatalf("transform: %v", err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("outgoing body is not JSON: %v", err)
	}
	// The Messages API lives at a different path and uses a different auth
	// header than the OpenAI-compatibility endpoint.
	if req.URL.Path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", req.URL.Path)
	}
	if req.Header.Get("x-api-key") != "sk-test" {
		t.Errorf("x-api-key = %q; the bearer token must move to the Messages API header", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("Authorization must be removed once x-api-key is set")
	}
	if req.Header.Get("anthropic-version") != anthropicMessagesVersion {
		t.Errorf("anthropic-version = %q", req.Header.Get("anthropic-version"))
	}
	return payload
}

func TestTransformAnthropicGroundedRequest(t *testing.T) {
	source := map[string]interface{}{
		"model": "claude-opus-5",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "Be concise."},
			map[string]interface{}{"role": "user", "content": "What is the current Claude API pricing?"},
		},
		"max_tokens":  float64(900),
		"temperature": float64(0.2),
	}
	cfg := &NativeSearchConfig{Enabled: true, UserLocation: &UserLocation{City: "Austin", Country: "US", Timezone: "America/Chicago"}}

	payload := groundedAnthropicRequest(t, source, cfg, false)

	// System content moves to a dedicated top-level field, not a message role.
	if payload["system"] != "Be concise." {
		t.Errorf("system = %v", payload["system"])
	}
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("expected 1 non-system message, got %d", len(messages))
	}
	first, _ := messages[0].(map[string]interface{})
	if first["role"] != "user" {
		t.Errorf("role = %v, want user", first["role"])
	}

	// max_tokens is required by the Messages API, unlike Chat Completions.
	if payload["max_tokens"] != float64(900) {
		t.Errorf("max_tokens = %v, want 900", payload["max_tokens"])
	}

	tools, _ := payload["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("expected exactly one server tool, got %d", len(tools))
	}
	tool, _ := tools[0].(map[string]interface{})
	if tool["type"] != anthropicWebSearchToolCurrent {
		t.Errorf("tool type = %v, want %q", tool["type"], anthropicWebSearchToolCurrent)
	}
	if tool["name"] != "web_search" {
		t.Errorf("tool name = %v", tool["name"])
	}
	if tool["max_uses"] != float64(anthropicMaxSearchUses) {
		t.Errorf("max_uses = %v, want %d", tool["max_uses"], anthropicMaxSearchUses)
	}
	if _, ok := tool["user_location"]; !ok {
		t.Error("approximate location should be forwarded")
	}
	if _, present := payload["stream"]; present {
		t.Error("a non-streaming request must not set stream")
	}
}

func TestTransformAnthropicGroundedRequestDefaultsMaxTokens(t *testing.T) {
	source := map[string]interface{}{
		"model":    "claude-sonnet-5",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hi"}},
	}
	payload := groundedAnthropicRequest(t, source, &NativeSearchConfig{Enabled: true}, true)
	if payload["max_tokens"] == nil {
		t.Fatal("max_tokens is required by the Messages API and must be defaulted")
	}
	if payload["stream"] != true {
		t.Error("a streaming request must set stream")
	}
}

// TestTransformAnthropicGroundedRequestPrependsUserTurn covers a shape the
// Messages API rejects outright: a conversation that does not start with a user
// message, which history trimming can produce.
func TestTransformAnthropicGroundedRequestPrependsUserTurn(t *testing.T) {
	source := map[string]interface{}{
		"model": "claude-opus-5",
		"messages": []interface{}{
			map[string]interface{}{"role": "assistant", "content": "Earlier answer."},
			map[string]interface{}{"role": "user", "content": "And now?"},
		},
	}
	payload := groundedAnthropicRequest(t, source, &NativeSearchConfig{Enabled: true}, false)
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 3 {
		t.Fatalf("expected a synthetic leading user turn, got %d messages", len(messages))
	}
	first, _ := messages[0].(map[string]interface{})
	if first["role"] != "user" {
		t.Errorf("first role = %v, want user", first["role"])
	}
}

func TestTransformAnthropicGroundedRequestDomainExclusivity(t *testing.T) {
	source := map[string]interface{}{
		"model":    "claude-opus-5",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "q"}},
	}
	// allowed_domains and blocked_domains are mutually exclusive; sending both is
	// a request error.
	cfg := &NativeSearchConfig{
		Enabled:         true,
		AllowedDomains:  []string{"anthropic.com"},
		ExcludedDomains: []string{"spam.test"},
	}
	payload := groundedAnthropicRequest(t, source, cfg, false)
	tools, _ := payload["tools"].([]interface{})
	tool, _ := tools[0].(map[string]interface{})
	if _, ok := tool["allowed_domains"]; !ok {
		t.Error("allowed_domains should win when both are configured")
	}
	if _, ok := tool["blocked_domains"]; ok {
		t.Error("blocked_domains must not be sent alongside allowed_domains")
	}
}

func TestTransformAnthropicGroundedRequestRequiresModelAndMessages(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/chat/completions", strings.NewReader("{}"))
	if err := transformAnthropicGroundedRequest(req, map[string]interface{}{}, &NativeSearchConfig{Enabled: true}, false); err == nil {
		t.Error("a missing model must be an error")
	}
	req2, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/chat/completions", strings.NewReader("{}"))
	source := map[string]interface{}{"model": "claude-opus-5", "messages": []interface{}{}}
	if err := transformAnthropicGroundedRequest(req2, source, &NativeSearchConfig{Enabled: true}, false); err == nil {
		t.Error("an empty conversation must be an error")
	}
}

const anthropicGroundedResponse = `{
  "content": [
    {"type": "text", "text": "Claude Opus 5 is $5 per million input tokens"},
    {"type": "web_search_tool_result", "content": [
      {"type": "web_search_result", "url": "https://www.anthropic.com/pricing", "title": "Pricing", "page_age": "2026-08-18T00:00:00Z"},
      {"type": "web_search_result", "url": "https://docs.anthropic.com/pricing", "title": ""}
    ]}
  ],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 120, "output_tokens": 45}
}`

func TestTransformAnthropicResponse(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(anthropicGroundedResponse)),
	}
	converted, err := transformAnthropicResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(converted.Body)

	var payload struct {
		Choices []struct {
			Message struct {
				Content     string `json:"content"`
				Annotations []struct {
					Type        string `json:"type"`
					URLCitation struct {
						URL   string `json:"url"`
						Title string `json:"title"`
					} `json:"url_citation"`
				} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("converted body is not OpenAI-shaped: %v\n%s", err, body)
	}
	if len(payload.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(payload.Choices))
	}
	message := payload.Choices[0].Message
	if !strings.Contains(message.Content, "$5 per million") {
		t.Errorf("answer text lost: %q", message.Content)
	}
	// Sources appear both as markdown (for readability) and as structured
	// annotations (so the backend can count and validate them).
	if !strings.Contains(message.Content, "**Sources:**") {
		t.Error("markdown source block missing")
	}
	if len(message.Annotations) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(message.Annotations))
	}
	// A blank title must fall back to the URL so the UI always has a label.
	for _, annotation := range message.Annotations {
		if annotation.URLCitation.Title == "" {
			t.Errorf("annotation for %q has no title", annotation.URLCitation.URL)
		}
	}
	if payload.Usage.PromptTokens != 120 || payload.Usage.CompletionTokens != 45 {
		t.Errorf("usage not carried over: %+v", payload.Usage)
	}
}

// TestTransformAnthropicResponseToolError covers the documented shape of a
// server-tool failure: HTTP 200 where the result content is an error *object*
// rather than a list. Decoding must not panic or invent sources.
func TestTransformAnthropicResponseToolError(t *testing.T) {
	body := `{"content":[
	  {"type":"text","text":"I could not search."},
	  {"type":"web_search_tool_result","content":{"type":"web_search_tool_result_error","error_code":"max_uses_exceeded"}}
	],"usage":{"input_tokens":10,"output_tokens":5}}`
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	converted, err := transformAnthropicResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(converted.Body)
	if strings.Contains(string(out), "**Sources:**") {
		t.Error("a tool error must not produce a source block")
	}
	if !strings.Contains(string(out), "could not search") {
		t.Error("the answer text must survive a tool error")
	}
}

func TestTransformAnthropicResponseUnknownShapePassesThrough(t *testing.T) {
	body := `{"error":{"type":"invalid_request_error","message":"bad"}}`
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	converted, err := transformAnthropicResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(converted.Body)
	if !strings.Contains(string(out), "invalid_request_error") {
		t.Error("an unrecognized payload must be handed back intact so the caller can report it")
	}
}

func TestWrapAnthropicStream(t *testing.T) {
	events := strings.Join([]string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":100,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"web_search_tool_result","content":[{"type":"web_search_result","url":"https://www.anthropic.com/pricing","title":"Pricing"}]}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Opus 5 is "}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"$5 per million."}}`,
		`data: {"type":"message_delta","usage":{"output_tokens":42}}`,
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
	converted := wrapAnthropicStream(upstream)
	out, err := io.ReadAll(converted.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)

	if !strings.Contains(text, "Opus 5 is ") || !strings.Contains(text, "$5 per million.") {
		t.Errorf("text deltas were not converted:\n%s", text)
	}
	if !strings.Contains(text, "**Sources:**") {
		t.Errorf("source block missing from the stream tail:\n%s", text)
	}
	if !strings.Contains(text, "url_citation") {
		t.Errorf("structured annotations missing from the stream tail:\n%s", text)
	}
	if !strings.Contains(text, `"prompt_tokens":100`) || !strings.Contains(text, `"completion_tokens":42`) {
		t.Errorf("usage chunk missing:\n%s", text)
	}
	if !strings.HasSuffix(strings.TrimSpace(text), "data: [DONE]") {
		t.Errorf("stream must terminate with [DONE]:\n%s", text)
	}
}

func TestNativeProviderForURLAnthropic(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/chat/completions", nil)
	if got := nativeProviderForURL(req.URL); got != "anthropic" {
		t.Errorf("nativeProviderForURL = %q, want anthropic", got)
	}
}

func TestAnthropicLocationOmitsEmpty(t *testing.T) {
	if got := anthropicLocation(nil); got != nil {
		t.Error("nil location must be omitted")
	}
	if got := anthropicLocation(&UserLocation{}); got != nil {
		t.Error("a location with no fields must be omitted rather than sent as a bare type")
	}
	got := anthropicLocation(&UserLocation{City: "Austin"})
	if got == nil || got["city"] != "Austin" || got["type"] != "approximate" {
		t.Errorf("anthropicLocation = %#v", got)
	}
}
