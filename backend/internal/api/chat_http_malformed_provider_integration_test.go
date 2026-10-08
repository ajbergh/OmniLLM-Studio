package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// TestChatHTTPMalformedProviderResponseBoundaries makes malformed upstream
// framing observable over the authenticated production HTTP/SSE routes without
// persisting a fabricated assistant message. A truncated stream must fail even
// when its first token was syntactically valid.
func TestChatHTTPMalformedProviderResponseBoundaries(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var request struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) == 0 {
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		content := request.Messages[len(request.Messages)-1].Content
		if !request.Stream && content == "invalid sync JSON" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{not-json")
			return
		}
		if request.Stream && content == "malformed stream frame" {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {not-json"+string([]byte{10, 10}))
			return
		}
		if request.Stream && content == "truncated stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"partial content"}}]}`+string([]byte{10, 10}))
			// Deliberately close without the provider's [DONE] marker.
			return
		}
		http.Error(w, "unexpected fixture turn", http.StatusBadRequest)
	}))
	defer provider.Close()

	baseURL := provider.URL + "/v1"
	model := "fixture-malformed-chat"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Malformed chat fixture", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
	}); err != nil {
		t.Fatalf("create fixture provider: %v", err)
	}
	router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown runtime: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()

	client := chatHTTPNewClient(t, server.URL)
	resp, body := client.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "malformed_chat_user", "password": "malformed-passphrase"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)

	testCases := []struct {
		name    string
		content string
		stream  bool
	}{
		{name: "nonstream JSON decoding", content: "invalid sync JSON"},
		{name: "stream protocol violation", content: "malformed stream frame", stream: true},
		{name: "stream missing DONE", content: "truncated stream", stream: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := client.request(t, http.MethodPost, "/v1/conversations/",
				map[string]interface{}{"title": tc.name})
			chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
			conversationID, ok := chatHTTPObject(t, body)["id"].(string)
			if !ok || conversationID == "" {
				t.Fatalf("missing conversation ID: %s", body)
			}
			messagesPath := "/v1/conversations/" + conversationID + "/messages"
			path := messagesPath
			if tc.stream {
				path += "/stream"
			}
			resp, body = client.request(t, http.MethodPost, path,
				map[string]interface{}{"content": tc.content, "tool_mode": "none", "web_search": false})
			if tc.stream {
				chatHTTPRequireStatus(t, resp, body, http.StatusOK)
				if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
					t.Fatalf("expected SSE response, got %q", resp.Header.Get("Content-Type"))
				}
				if !strings.Contains(string(body), "event: start") ||
					!strings.Contains(string(body), "event: error") ||
					!strings.Contains(string(body), "LLM_STREAM_FAILED") {
					t.Fatalf("missing failure SSE sequence: %s", body)
				}
				if strings.Contains(string(body), "event: done") {
					t.Fatalf("malformed upstream stream falsely completed: %s", body)
				}
				if strings.Contains(string(body), "not-json") {
					t.Fatalf("raw malformed upstream payload was leaked: %s", body)
				}
			} else {
				chatHTTPRequireStatus(t, resp, body, http.StatusBadGateway)
				if got := chatHTTPObject(t, body)["error"]; got != "LLM request failed" {
					t.Fatalf("public response error = %v", got)
				}
				if strings.Contains(string(body), "not-json") {
					t.Fatalf("raw upstream JSON was leaked: %s", body)
				}
			}

			resp, body = client.request(t, http.MethodGet, messagesPath, nil)
			chatHTTPRequireStatus(t, resp, body, http.StatusOK)
			var messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(body, &messages); err != nil {
				t.Fatalf("decode saved messages: %v, body=%s", err, body)
			}
			if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != tc.content {
				t.Fatalf("malformed provider response produced a completed assistant: %+v", messages)
			}
		})
	}
}
