package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// TestChatHTTPProviderFailureDoesNotSaveAssistant verifies that upstream model
// failures do not leave false completed assistant turns in message history.
func TestChatHTTPProviderFailureDoesNotSaveAssistant(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	var upstreamCalls atomic.Int32

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider request", http.StatusNotFound)
			return
		}
		upstreamCalls.Add(1)
		http.Error(w, "fixture upstream detail must not leak", http.StatusServiceUnavailable)
	}))
	defer provider.Close()

	baseURL := provider.URL + "/v1"
	model := "fixture-failing-chat"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Failing chat provider", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
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
		map[string]interface{}{"username": "chat_failure_user", "password": "test-failure-password"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
	resp, body = client.request(t, http.MethodPost, "/v1/conversations/",
		map[string]interface{}{"title": "Model failure handling"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
	conversationID, ok := chatHTTPObject(t, body)["id"].(string)
	if !ok || conversationID == "" {
		t.Fatalf("missing conversation ID: %s", body)
	}
	messagesPath := "/v1/conversations/" + conversationID + "/messages"

	resp, body = client.request(t, http.MethodPost, messagesPath,
		map[string]interface{}{"content": "A non-streaming model failure", "tool_mode": "none", "web_search": false})
	chatHTTPRequireStatus(t, resp, body, http.StatusBadGateway)
	if strings.Contains(string(body), "fixture upstream detail") {
		t.Fatalf("provider error detail escaped into HTTP response: %s", body)
	}
	if got := chatHTTPObject(t, body)["error"]; got != "LLM request failed" {
		t.Fatalf("unexpected public error = %v", got)
	}

	resp, body = client.request(t, http.MethodPost, messagesPath+"/stream",
		map[string]interface{}{"content": "A streamed model failure", "tool_mode": "none", "web_search": false})
	chatHTTPRequireStatus(t, resp, body, http.StatusOK)
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected SSE content type: %q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), "event: start") || !strings.Contains(string(body), "event: error") {
		t.Fatalf("missing SSE start/error lifecycle events: %s", body)
	}
	if strings.Contains(string(body), "event: done") || strings.Contains(string(body), "fixture upstream detail") {
		t.Fatalf("failed provider stream falsely completed or leaked private details: %s", body)
	}
	if !strings.Contains(string(body), "LLM_STREAM_FAILED") {
		t.Fatalf("missing stable failure code in SSE event: %s", body)
	}

	resp, body = client.request(t, http.MethodGet, messagesPath, nil)
	chatHTTPRequireStatus(t, resp, body, http.StatusOK)
	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &messages); err != nil {
		t.Fatalf("decode persisted messages: %v, body=%s", err, body)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "user" {
		t.Fatalf("a failed model turn persisted an assistant response: %+v", messages)
	}
	// Providers may retry transient upstream errors; each user turn must
	// reach the provider, but retry count is not an API contract.
	if upstreamCalls.Load() < 2 {
		t.Fatalf("upstream calls = %d, want at least 2", upstreamCalls.Load())
	}
}
