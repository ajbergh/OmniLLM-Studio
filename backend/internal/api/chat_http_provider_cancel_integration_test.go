package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// TestChatHTTPClientCancellationReachesUpstream ensures that the real HTTP
// handler propagates a disconnected client's context to the model provider.
func TestChatHTTPClientCancellationReachesUpstream(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	providerStarted := make(chan struct{}, 1)
	providerCancelled := make(chan struct{}, 1)
	// Ensure a failed assertion cannot strand httptest.Server.Close waiting on
	// a deliberately blocked provider request.
	releaseProvider := make(chan struct{})

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		// Consume the request body so the Go HTTP server can detect the peer's
		// connection closing; an unread POST body can defer cancellation.
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		providerStarted <- struct{}{}
		select {
		case <-r.Context().Done():
			providerCancelled <- struct{}{}
		case <-releaseProvider:
		}
	}))
	defer provider.Close()
	defer close(releaseProvider)

	baseURL := provider.URL + "/v1"
	model := "fixture-cancel-chat"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Cancellable chat provider", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
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
		map[string]interface{}{"username": "chat_cancel_user", "password": "cancel-passphrase"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
	resp, body = client.request(t, http.MethodPost, "/v1/conversations/",
		map[string]interface{}{"title": "Client cancellation"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
	conversationID, ok := chatHTTPObject(t, body)["id"].(string)
	if !ok || conversationID == "" {
		t.Fatalf("missing conversation ID: %s", body)
	}
	messagesPath := "/v1/conversations/" + conversationID + "/messages"

	payload, err := json.Marshal(map[string]interface{}{
		"content": "An in-flight request to cancel", "tool_mode": "none", "web_search": false,
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+messagesPath, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	result := make(chan error, 1)
	go func() {
		response, err := client.client.Do(req)
		if response != nil {
			response.Body.Close()
		}
		result <- err
	}()

	select {
	case <-providerStarted:
		cancel()
	case <-time.After(8 * time.Second):
		cancel()
		t.Fatal("chat handler did not reach the local provider")
	}
	select {
	case <-providerCancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("provider context remained active after client cancellation")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client request error = %v, expected context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled client request did not return")
	}

	// The pending user turn may remain for auditability, but a canceled model
	// call must never fabricate a completed assistant reply.
	resp, body = client.request(t, http.MethodGet, messagesPath, nil)
	chatHTTPRequireStatus(t, resp, body, http.StatusOK)
	var messages []struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(body, &messages); err != nil {
		t.Fatalf("decode persisted messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("unexpected persisted canceled turn: %+v", messages)
	}
}
