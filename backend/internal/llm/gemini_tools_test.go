package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// groundedGeminiPayload runs a request through the real transform and returns the
// outgoing Gemini body.
func groundedGeminiPayload(t *testing.T, source map[string]interface{}, stream bool) map[string]interface{} {
	t.Helper()
	parsed, err := url.Parse("https://generativelanguage.googleapis.com/v1beta/openai/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL:    parsed,
		Header: http.Header{"Authorization": []string{"Bearer k"}},
		Body:   io.NopCloser(strings.NewReader("{}")),
	}
	if err := transformGeminiGroundedRequest(req, source, &NativeSearchConfig{Enabled: true}, stream); err != nil {
		t.Fatalf("transform: %v", err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("outgoing body is not JSON: %v\n%s", err, body)
	}
	return payload
}

// TestGeminiGroundingCarriesFunctionDeclarations is the point of unification: one
// request holds both the provider's search tool and the caller's tools. Before
// this, the adapter emitted only google_search and discarded the tools, so a
// grounded turn could not call anything.
func TestGeminiGroundingCarriesFunctionDeclarations(t *testing.T) {
	source := map[string]interface{}{
		"model": "gemini-3.7-flash",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "latest prices, then total them"},
		},
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "calculator",
					"description": "Evaluate an expression",
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"expression": map[string]interface{}{"type": "string"}},
						"required":   []interface{}{"expression"},
					},
				},
			},
		},
	}

	payload := groundedGeminiPayload(t, source, false)
	tools, ok := payload["tools"].([]interface{})
	if !ok || len(tools) != 2 {
		t.Fatalf("expected google_search plus function_declarations, got %#v", payload["tools"])
	}

	first, _ := tools[0].(map[string]interface{})
	if _, present := first["google_search"]; !present {
		t.Errorf("grounding must still be present: %#v", first)
	}
	second, _ := tools[1].(map[string]interface{})
	declarations, ok := second["function_declarations"].([]interface{})
	if !ok || len(declarations) != 1 {
		t.Fatalf("expected one function declaration, got %#v", second)
	}
	declaration, _ := declarations[0].(map[string]interface{})
	if declaration["name"] != "calculator" {
		t.Errorf("declaration name = %v", declaration["name"])
	}
	if _, present := declaration["parameters"]; !present {
		t.Error("the parameter schema must survive")
	}
	// AUTO, not ANY: the model must be free to answer from grounding alone.
	config, ok := payload["tool_config"].(map[string]interface{})
	if !ok {
		t.Fatal("tool_config is required when function declarations are sent")
	}
	inner, _ := config["function_calling_config"].(map[string]interface{})
	if inner["mode"] != "AUTO" {
		t.Errorf("mode = %v, want AUTO", inner["mode"])
	}
}

func TestGeminiGroundingWithoutToolsOmitsDeclarations(t *testing.T) {
	source := map[string]interface{}{
		"model":    "gemini-3.7-flash",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "latest news"}},
	}
	payload := groundedGeminiPayload(t, source, false)
	tools, _ := payload["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("expected grounding only, got %#v", tools)
	}
	if _, present := payload["tool_config"]; present {
		t.Error("tool_config must be omitted when no functions are declared")
	}
}

// TestGeminiHistoryPreservesToolRoundTrip is the requirement that makes a
// multi-round loop possible. An assistant turn that only calls a tool has empty
// content, and the previous implementation skipped every message with blank text
// — so round two arrived with no record that the tool had run and the model
// called it again.
func TestGeminiHistoryPreservesToolRoundTrip(t *testing.T) {
	source := map[string]interface{}{
		"model": "gemini-3.7-flash",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "Be terse."},
			map[string]interface{}{"role": "user", "content": "what is 2+2"},
			map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":   "call_1",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "calculator",
							"arguments": `{"expression":"2+2"}`,
						},
					},
				},
			},
			map[string]interface{}{"role": "tool", "tool_call_id": "call_1", "content": "4"},
		},
	}

	payload := groundedGeminiPayload(t, source, false)
	if payload["system_instruction"] == nil {
		t.Error("system content must move to system_instruction")
	}
	contents, _ := payload["contents"].([]interface{})
	if len(contents) != 3 {
		t.Fatalf("expected user, model(functionCall), user(functionResponse); got %d: %#v", len(contents), contents)
	}

	model, _ := contents[1].(map[string]interface{})
	if model["role"] != "model" {
		t.Errorf("tool-calling turn role = %v, want model", model["role"])
	}
	modelParts, _ := model["parts"].([]interface{})
	if len(modelParts) != 1 {
		t.Fatalf("expected one functionCall part, got %#v", modelParts)
	}
	callPart, _ := modelParts[0].(map[string]interface{})
	call, ok := callPart["functionCall"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a functionCall part, got %#v", callPart)
	}
	if call["name"] != "calculator" {
		t.Errorf("functionCall name = %v", call["name"])
	}
	args, _ := call["args"].(map[string]interface{})
	if args["expression"] != "2+2" {
		t.Errorf("arguments must be decoded into args: %#v", args)
	}

	result, _ := contents[2].(map[string]interface{})
	if result["role"] != "user" {
		t.Errorf("function responses go on a user turn, got %v", result["role"])
	}
	resultParts, _ := result["parts"].([]interface{})
	resultPart, _ := resultParts[0].(map[string]interface{})
	response, ok := resultPart["functionResponse"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a functionResponse part, got %#v", resultPart)
	}
	// Gemini keys responses by function name, not by call id, so the name must be
	// recovered from the assistant turn that requested it.
	if response["name"] != "calculator" {
		t.Errorf("functionResponse name = %v, want calculator (recovered from tool_call_id)", response["name"])
	}
}

func TestGeminiSchemaSanitization(t *testing.T) {
	// Tool schemas here are written for OpenAI, which accepts keywords Gemini
	// rejects. Passing one through unchanged is a 400, and a 400 on a grounded
	// request degrades to local search instead of surfacing.
	raw := map[string]interface{}{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"additionalProperties": false,
		"type":                 "object",
		"title":                "Args",
		"properties": map[string]interface{}{
			"n": map[string]interface{}{"type": "integer", "default": 1, "exclusiveMinimum": 0},
		},
		"required": []interface{}{"n"},
	}
	sanitized, ok := sanitizeGeminiSchema(raw).(map[string]interface{})
	if !ok {
		t.Fatalf("sanitize returned %T", sanitizeGeminiSchema(raw))
	}
	for _, banned := range []string{"$schema", "additionalProperties", "title"} {
		if _, present := sanitized[banned]; present {
			t.Errorf("%q must be stripped", banned)
		}
	}
	if sanitized["type"] != "object" {
		t.Error("type must survive")
	}
	properties, _ := sanitized["properties"].(map[string]interface{})
	n, _ := properties["n"].(map[string]interface{})
	if n["type"] != "integer" {
		t.Error("nested type must survive")
	}
	for _, banned := range []string{"default", "exclusiveMinimum"} {
		if _, present := n[banned]; present {
			t.Errorf("nested %q must be stripped", banned)
		}
	}
}

// TestGeminiResponseSurfacesToolCalls covers the other half: without a
// tool_calls finish reason and mapped calls, the loop never sees what the model
// asked for and the turn silently ends.
func TestGeminiResponseSurfacesToolCalls(t *testing.T) {
	body := `{"candidates":[{"content":{"parts":[
		{"text":"Checking."},
		{"functionCall":{"name":"calculator","args":{"expression":"2+2"}}}
	]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3}}`

	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	converted, err := transformGeminiResponse(resp)
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
					Type     string `json:"type"`
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
		t.Errorf("finish_reason = %q; the loop keys on this to continue", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(choice.Message.ToolCalls))
	}
	tc := choice.Message.ToolCalls[0]
	if tc.Function.Name != "calculator" {
		t.Errorf("name = %q", tc.Function.Name)
	}
	if !strings.Contains(tc.Function.Arguments, "2+2") {
		t.Errorf("arguments = %q", tc.Function.Arguments)
	}
	if tc.ID == "" || tc.Type != "function" {
		t.Errorf("id/type = %q/%q; Gemini assigns no id so one must be synthesized", tc.ID, tc.Type)
	}
	// A tool-call turn must not carry the markdown source block: the loop
	// continues and a "Sources" list would land mid-conversation.
	if strings.Contains(choice.Message.Content, "**Sources:**") {
		t.Error("source block must be withheld until the final answer")
	}
}

func TestGeminiResponseThoughtPartsAreNotAnswerText(t *testing.T) {
	body := `{"candidates":[{"content":{"parts":[
		{"text":"internal reasoning","thought":true},
		{"text":"The answer is 4."}
	]}}],"usageMetadata":{}}`
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	converted, err := transformGeminiResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(converted.Body)
	if strings.Contains(string(out), "internal reasoning") {
		t.Error("reasoning parts must not be concatenated into the answer")
	}
	if !strings.Contains(string(out), "The answer is 4.") {
		t.Error("answer text must survive")
	}
}

// TestGeminiStreamEmitsToolCalls covers the streaming half. Gemini sends complete
// functionCall parts rather than fragmented arguments, so each becomes one
// finished tool-call delta.
func TestGeminiStreamEmitsToolCalls(t *testing.T) {
	events := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Looking. "}]}}]}`,
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"calculator","args":{"expression":"2+2"}}}]}}]}`,
		`data: {"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":2}}`,
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
	out, err := io.ReadAll(wrapGeminiStream(upstream).Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)

	if !strings.Contains(text, "Looking. ") {
		t.Errorf("text deltas lost:\n%s", text)
	}
	if !strings.Contains(text, `"tool_calls"`) || !strings.Contains(text, "calculator") {
		t.Errorf("tool call was not emitted:\n%s", text)
	}
	if !strings.Contains(text, `"prompt_tokens":7`) {
		t.Errorf("usage lost:\n%s", text)
	}
	if !strings.HasSuffix(strings.TrimSpace(text), "data: [DONE]") {
		t.Errorf("stream must terminate:\n%s", text)
	}
}

func TestGeminiGroundedRequestWithoutTools(t *testing.T) {
	source := map[string]interface{}{
		"model":       "gemini-3.7-flash",
		"messages":    []interface{}{map[string]interface{}{"role": "user", "content": "hi"}},
		"tools":       []interface{}{map[string]interface{}{"type": "function"}},
		"tool_choice": "auto",
	}
	stripped := geminiGroundedRequestWithoutTools(source)
	if _, present := stripped["tools"]; present {
		t.Error("tools must be removed for the grounding-only retry")
	}
	if _, present := stripped["tool_choice"]; present {
		t.Error("tool_choice must be removed too")
	}
	if stripped["model"] != "gemini-3.7-flash" {
		t.Error("the rest of the request must be preserved")
	}
	if _, present := source["tools"]; !present {
		t.Error("the original request must not be mutated")
	}
}

// rejectFunctionDeclarationsTransport answers 400 for any request carrying
// function declarations and 200 otherwise, simulating a model family that refuses
// grounding and function calling together.
type rejectFunctionDeclarationsTransport struct {
	attempts [][]byte
}

func (t *rejectFunctionDeclarationsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	t.attempts = append(t.attempts, body)

	if strings.Contains(string(body), "function_declarations") {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":400,"message":"Tool use with function calling is unsupported"}}`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"candidates":[{"content":{"parts":[{"text":"grounded answer"}]}}],"usageMetadata":{}}`)),
	}, nil
}

// TestGeminiRejectionFallsBackToGroundingOnly is the safety net for the one thing
// this change cannot verify offline: whether a given Gemini model family accepts
// google_search and function_declarations in the same request.
//
// If it does not, the answer is a 400, and a 400 on a grounded request degrades to
// local search rather than surfacing. So the transport retries once with grounding
// alone. Grounding is the half worth keeping — without it the answer is
// ungrounded, without tools it is merely less capable.
func TestGeminiRejectionFallsBackToGroundingOnly(t *testing.T) {
	base := &rejectFunctionDeclarationsTransport{}
	transport := &nativeSearchTransport{base: base}

	plugin := NativeSearchPlugin(&NativeSearchConfig{Enabled: true})
	body, err := json.Marshal(map[string]interface{}{
		"model":    "gemini-3.7-flash",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "latest news"}},
		"tools": []interface{}{map[string]interface{}{
			"type":     "function",
			"function": map[string]interface{}{"name": "calculator"},
		}},
		"plugins": []interface{}{map[string]interface{}{"id": plugin.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}

	target, err := url.Parse("https://generativelanguage.googleapis.com/v1beta/openai/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL:    target,
		Header: http.Header{"Authorization": []string{"Bearer k"}},
		Body:   io.NopCloser(strings.NewReader(string(body))),
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if len(base.attempts) != 2 {
		t.Fatalf("expected a rejected attempt then a grounding-only retry, got %d attempt(s)", len(base.attempts))
	}
	if !strings.Contains(string(base.attempts[0]), "function_declarations") {
		t.Error("the first attempt should carry the function declarations")
	}
	if strings.Contains(string(base.attempts[1]), "function_declarations") {
		t.Error("the retry must drop the function declarations")
	}
	if !strings.Contains(string(base.attempts[1]), "google_search") {
		t.Error("the retry must keep grounding — that is the half worth preserving")
	}
	// The retry must target the native endpoint, not a doubly-rewritten path.
	if strings.Count(string(base.attempts[1]), "chat/completions") != 0 {
		t.Error("the retry path must not contain the OpenAI-compat suffix")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want the retry's 200", resp.StatusCode)
	}

	answer, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(answer), "grounded answer") {
		t.Errorf("the retry's grounded answer must be returned: %s", answer)
	}
}
