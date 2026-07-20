package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNativeSearchPluginRoundTrip(t *testing.T) {
	plugin := NativeSearchPlugin(&NativeSearchConfig{
		Enabled:     true,
		ContextSize: "low",
		MaxResults:  3,
		UserLocation: &UserLocation{
			Timezone: "America/Chicago",
			Country:  "US",
		},
	})
	if !strings.HasPrefix(plugin.ID, nativeSearchPluginPrefix) {
		t.Fatalf("native marker prefix changed or was omitted: %q", plugin.ID)
	}
	body := map[string]interface{}{
		"plugins": []interface{}{map[string]interface{}{"id": plugin.ID}},
	}
	cfg, ok := nativeSearchConfigFromBody(body)
	if !ok || cfg.ContextSize != "low" || cfg.UserLocation == nil || cfg.UserLocation.Timezone != "America/Chicago" {
		t.Fatalf("unexpected native search marker: %#v", cfg)
	}
	if _, stillPresent := body["plugins"]; stillPresent {
		t.Fatalf("internal marker leaked into provider body: %#v", body)
	}
}

func TestApplyOpenAIWebSearch(t *testing.T) {
	body := map[string]interface{}{"model": "gpt-5.2"}
	applyOpenAIWebSearch(body, &NativeSearchConfig{
		ContextSize: "low",
		UserLocation: &UserLocation{
			Timezone: "America/Chicago",
			Country:  "US",
		},
		AnswerVerbosity: "low",
	})
	options, ok := body["web_search_options"].(map[string]interface{})
	if !ok || options["search_context_size"] != "low" {
		t.Fatalf("missing OpenAI web search options: %#v", body)
	}
	if body["verbosity"] != "low" {
		t.Fatalf("missing low verbosity: %#v", body)
	}
}

func TestApplyOpenRouterSearch(t *testing.T) {
	body := map[string]interface{}{}
	applyOpenRouterSearch(body, &NativeSearchConfig{
		ContextSize:     "low",
		MaxResults:      3,
		MaxTotalResults: 6,
		AllowedDomains:  []string{"fifa.com"},
	})
	tools, ok := body["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("missing OpenRouter server tool: %#v", body)
	}
	tool, _ := tools[0].(map[string]interface{})
	if tool["type"] != "openrouter:web_search" {
		t.Fatalf("unexpected OpenRouter tool: %#v", tool)
	}
}

func TestTransformGeminiGroundedRequest(t *testing.T) {
	original := map[string]interface{}{
		"model": "gemini-3.1-flash-lite",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "Answer briefly."},
			map[string]interface{}{"role": "user", "content": "What time is the game?"},
		},
		"stream": false,
	}
	requestURL, _ := url.Parse("https://generativelanguage.googleapis.com/v1beta/openai/chat/completions")
	req := &http.Request{
		Method: http.MethodPost,
		URL:    requestURL,
		Header: http.Header{"Authorization": []string{"Bearer test-key"}},
		Body:   io.NopCloser(bytes.NewReader(nil)),
	}
	if err := transformGeminiGroundedRequest(req, original, &NativeSearchConfig{Enabled: true}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.URL.Path, "/models/gemini-3.1-flash-lite:generateContent") {
		t.Fatalf("unexpected Gemini URL: %s", req.URL.String())
	}
	if req.Header.Get("x-goog-api-key") != "test-key" || req.Header.Get("Authorization") != "" {
		t.Fatalf("unexpected Gemini auth headers: %#v", req.Header)
	}
	encoded, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	tools, ok := payload["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("missing Gemini Google Search grounding: %#v", payload)
	}
}

func TestOpenAIVerbosityOnlyForGPT5(t *testing.T) {
	cfg := &NativeSearchConfig{Enabled: true, AnswerVerbosity: "low"}
	gpt41 := map[string]interface{}{"model": "gpt-4.1"}
	applyOpenAIWebSearch(gpt41, cfg)
	if _, exists := gpt41["verbosity"]; exists {
		t.Fatalf("GPT-4.1 received unsupported verbosity: %#v", gpt41)
	}
	gpt5 := map[string]interface{}{"model": "gpt-5.2"}
	applyOpenAIWebSearch(gpt5, cfg)
	if gpt5["verbosity"] != "low" {
		t.Fatalf("GPT-5 did not receive low verbosity: %#v", gpt5)
	}
}

func TestNativeSearchReplacesDeprecatedOpenRouterWebPlugin(t *testing.T) {
	marker := NativeSearchPlugin(&NativeSearchConfig{Enabled: true})
	body := map[string]interface{}{
		"plugins": []interface{}{
			map[string]interface{}{"id": "web"},
			map[string]interface{}{"id": "response-healing"},
			map[string]interface{}{"id": marker.ID},
		},
	}
	if _, ok := nativeSearchConfigFromBody(body); !ok {
		t.Fatal("native search marker was not detected")
	}
	plugins, ok := body["plugins"].([]interface{})
	if !ok || len(plugins) != 1 {
		t.Fatalf("deprecated web plugin was not replaced: %#v", body)
	}
	plugin, _ := plugins[0].(map[string]interface{})
	if plugin["id"] != "response-healing" {
		t.Fatalf("unrelated plugin was not preserved: %#v", plugins)
	}
}

func TestNativeSearchTransportIsScopedToLLMService(t *testing.T) {
	globalBefore := http.DefaultTransport
	service := NewService(nil, nil)
	if http.DefaultTransport != globalBefore {
		t.Fatal("NewService modified the global HTTP transport")
	}
	transport, ok := service.httpClient.Transport.(*nativeSearchTransport)
	if !ok {
		t.Fatalf("LLM service is missing the native search transport: %T", service.httpClient.Transport)
	}
	if transport.base == nil {
		t.Fatal("native search transport has no base RoundTripper")
	}
}
