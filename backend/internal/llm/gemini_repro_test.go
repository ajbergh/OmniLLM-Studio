package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// captureTransport records the request the nativeSearchTransport actually emits.
type captureTransport struct {
	gotURL    string
	gotBody   []byte
	gotHeader http.Header
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.gotURL = req.URL.String()
	c.gotHeader = req.Header.Clone()
	if req.Body != nil {
		c.gotBody, _ = io.ReadAll(req.Body)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}],"usageMetadata":{}}`)),
	}, nil
}

// roundTripThroughNativeTransport runs a request through the real transport so
// the test observes exactly what would leave the process.
func roundTripThroughNativeTransport(t *testing.T, baseURL, model string, stream bool, cfg *NativeSearchConfig) *captureTransport {
	t.Helper()
	capture := &captureTransport{}
	transport := &nativeSearchTransport{base: capture}

	body := map[string]interface{}{
		"model":    model,
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "what is the latest news today"}},
		"stream":   stream,
	}
	if plugin := NativeSearchPlugin(cfg); plugin.ID != "" {
		body["plugins"] = []interface{}{map[string]interface{}{"id": plugin.ID}}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(baseURL + "/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL:    parsed,
		Header: http.Header{"Authorization": []string{"Bearer test-key"}},
		Body:   io.NopCloser(strings.NewReader(string(encoded))),
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	return capture
}

// TestGeminiDirectGroundingReachesProvider walks the exact path a Gemini direct
// provider profile takes, at the default base URL, and asserts the outgoing
// request really carries Google Search grounding.
func TestGeminiDirectGroundingReachesProvider(t *testing.T) {
	const geminiBase = "https://generativelanguage.googleapis.com/v1beta/openai"

	for _, model := range []string{"gemini-3.7-flash", "gemini-3.1-flash-lite", "gemini-2.5-pro"} {
		for _, stream := range []bool{false, true} {
			name := model
			if stream {
				name += "/stream"
			}
			t.Run(name, func(t *testing.T) {
				capture := roundTripThroughNativeTransport(t, geminiBase, model, stream,
					&NativeSearchConfig{Enabled: true, ContextSize: "medium", MaxResults: 6})

				if !strings.Contains(capture.gotURL, "/models/"+model+":") {
					t.Fatalf("request did not reach the native Gemini endpoint: %s", capture.gotURL)
				}
				if capture.gotHeader.Get("x-goog-api-key") == "" {
					t.Error("api key was not moved to x-goog-api-key")
				}

				var payload map[string]interface{}
				if err := json.Unmarshal(capture.gotBody, &payload); err != nil {
					t.Fatalf("outgoing body is not JSON: %v\n%s", err, capture.gotBody)
				}
				tools, ok := payload["tools"].([]interface{})
				if !ok || len(tools) == 0 {
					t.Fatalf("no grounding tool on the wire: %s", capture.gotBody)
				}
				tool, _ := tools[0].(map[string]interface{})
				if _, present := tool["google_search"]; !present {
					t.Errorf("expected google_search grounding, got %#v", tool)
				}
				// The internal marker must never leave the process.
				if _, leaked := payload["plugins"]; leaked {
					t.Error("internal marker plugin leaked to the provider")
				}
			})
		}
	}
}

// TestGeminiModelPrefixDetection checks the capability gate that decides whether
// the marker is attached at all.
func TestGeminiModelPrefixDetection(t *testing.T) {
	cases := map[string]bool{
		"gemini-3.7-flash":        true,
		"gemini-3.7-pro":          true,
		"gemini-3-pro":            true,
		"gemini-2.5-flash":        true,
		"models/gemini-3.7-flash": false, // prefixed form — see below
		"gemini-1.5-pro":          false,
	}
	for model, want := range cases {
		if got := geminiSupportsNativeSearchForTest(model); got != want {
			t.Errorf("model %q: native-capable = %v, want %v", model, got, want)
		}
	}
}

// geminiSupportsNativeSearchForTest mirrors the websearch capability check
// without importing that package (which would be a cycle).
func geminiSupportsNativeSearchForTest(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "gemini-2") || strings.HasPrefix(model, "gemini-3")
}

// TestGeminiPathRewriteBaseURLVariants covers Gemini base URLs a user can
// plausibly configure. The rewrite trims a fixed "/openai/chat/completions"
// suffix, so any other shape produces a nonsense path and a 404.
func TestGeminiPathRewriteBaseURLVariants(t *testing.T) {
	variants := []struct {
		name    string
		baseURL string
	}{
		{"documented default", "https://generativelanguage.googleapis.com/v1beta/openai"},
		{"trailing slash", "https://generativelanguage.googleapis.com/v1beta/openai/"},
		{"no openai segment", "https://generativelanguage.googleapis.com/v1beta"},
		{"v1 instead of v1beta", "https://generativelanguage.googleapis.com/v1/openai"},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			capture := roundTripThroughNativeTransport(t, v.baseURL, "gemini-3.7-flash", true,
				&NativeSearchConfig{Enabled: true})
			t.Logf("base=%s  ->  %s", v.baseURL, capture.gotURL)
			if strings.Contains(capture.gotURL, "chat/completions") {
				t.Errorf("rewrite left the OpenAI-compat path in place: %s", capture.gotURL)
			}
			if !strings.Contains(capture.gotURL, "/models/gemini-3.7-flash:streamGenerateContent") {
				t.Errorf("did not reach the native streaming endpoint: %s", capture.gotURL)
			}
		})
	}
}
