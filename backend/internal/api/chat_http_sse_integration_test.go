package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

type chatHTTPTestClient struct {
	client *http.Client
	base   string
}

func (c chatHTTPTestClient) request(t *testing.T, method, path string, body map[string]interface{}) (*http.Response, []byte) {
	t.Helper()
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, input)
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s: %v", method, path, err)
	}
	payload, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	return resp, payload
}

func chatHTTPRequireStatus(t *testing.T, resp *http.Response, payload []byte, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, want, payload)
	}
}

func chatHTTPObject(t *testing.T, payload []byte) map[string]interface{} {
	t.Helper()
	var object map[string]interface{}
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatalf("decode JSON object %q: %v", payload, err)
	}
	return object
}

func chatHTTPNewClient(t *testing.T, base string) chatHTTPTestClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return chatHTTPTestClient{client: &http.Client{Jar: jar, Timeout: 15 * time.Second}, base: base}
}

func TestAuthenticatedChatHTTPAndSSELifecycle(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	cfg.AllowPublicReg = true // Two users let us prove the conversation boundary.

	var syncCalls, streamCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var request struct {
			Stream   bool                     `json:"stream"`
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) < 2 {
			http.Error(w, "invalid chat payload", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if request.Stream {
			streamCalls.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {"choices":[{"delta":{"content":"Streaming "}}]}

")
			fmt.Fprint(w, "data: {"choices":[{"delta":{"content":"works"}}]}

")
			fmt.Fprint(w, "data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":4,"cost":0.03}}

")
			fmt.Fprint(w, "data: [DONE]

")
			return
		}
		syncCalls.Add(1)
		fmt.Fprint(w, "{"choices":[{"message":{"content":"Sync works"}}],"usage":{"prompt_tokens":6,"completion_tokens":3,"cost":0.01}}")
	}))
	defer provider.Close()
	baseURL := provider.URL + "/v1"
	model := "fixture-chat"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Local HTTP fixture", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
	}); err != nil {
		t.Fatalf("create fixture provider: %v", err)
	}

	router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown runtime services: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()

	anonymous := chatHTTPTestClient{client: &http.Client{Timeout: 15 * time.Second}, base: server.URL}
	alice := chatHTTPNewClient(t, server.URL)
	bob := chatHTTPNewClient(t, server.URL)

	// Once accounts exist, both chat routes must reject unauthenticated requests.
	resp, payload := alice.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "chat_alice", "password": "password-for-alice"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	user := chatHTTPObject(t, payload)
	if user["token"] == "" || user["token"] == nil {
		t.Fatal("registration did not return a session token")
	}
	cookieFound := false
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "omnillm_session" {
			cookieFound = true
			if !cookie.HttpOnly || cookie.Path != "/v1" || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("session cookie attributes: %+v", cookie)
			}
		}
	}
	if !cookieFound {
		t.Fatal("registration did not set a session cookie")
	}

	resp, payload = alice.request(t, http.MethodPost, "/v1/conversations/",
		map[string]interface{}{"title": "HTTP integration conversation"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	conversationID, ok := chatHTTPObject(t, payload)["id"].(string)
	if !ok || conversationID == "" {
		t.Fatalf("conversation ID missing from %q", payload)
	}
	messagesPath := "/v1/conversations/" + conversationID + "/messages"
	for _, path := range []string{messagesPath, messagesPath + "/stream"} {
		resp, payload = anonymous.request(t, http.MethodPost, path,
			map[string]interface{}{"content": "unauthorized", "tool_mode": "none"})
		chatHTTPRequireStatus(t, resp, payload, http.StatusUnauthorized)
		if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Fatalf("unauthorized %s negotiated SSE", path)
		}
	}

	resp, payload = alice.request(t, http.MethodPost, messagesPath,
		map[string]interface{}{"content": "Hello", "tool_mode": "none", "web_search": false})
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	if chatHTTPObject(t, payload)["content"] != "Sync works" {
		t.Fatalf("unexpected non-streaming response: %s", payload)
	}

	resp, payload = alice.request(t, http.MethodPost, messagesPath+"/stream",
		map[string]interface{}{"content": "Hello again", "tool_mode": "none", "web_search": false})
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream content type = %q", resp.Header.Get("Content-Type"))
	}
	var orderedEvents []string
	var streamedText string
	var done map[string]interface{}
	for _, frame := range strings.Split(strings.TrimSpace(string(payload)), "

") {
		if !strings.HasPrefix(frame, "event: ") {
			t.Fatalf("malformed SSE frame: %q", frame)
		}
		parts := strings.SplitN(frame, "
data: ", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed SSE data: %q", frame)
		}
		name := strings.TrimPrefix(parts[0], "event: ")
		orderedEvents = append(orderedEvents, name)
		event := chatHTTPObject(t, []byte(parts[1]))
		if name == "token" {
			streamedText += event["content"].(string)
		}
		if name == "done" {
			done = event
		}
	}
	if len(orderedEvents) < 4 || orderedEvents[0] != "start" ||
		orderedEvents[len(orderedEvents)-1] != "done" {
		t.Fatalf("SSE start/done ordering: %v", orderedEvents)
	}
	if streamedText != "Streaming works" || done["content"] != "Streaming works" {
		t.Fatalf("SSE body mismatch: tokens=%q done=%v", streamedText, done)
	}
	if syncCalls.Load() != 1 || streamCalls.Load() != 1 {
		t.Fatalf("fixture provider calls: sync=%d stream=%d", syncCalls.Load(), streamCalls.Load())
	}

	// Verify actual persisted messages, not just endpoint responses.
	resp, payload = alice.request(t, http.MethodGet, messagesPath, nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	var history []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(payload, &history); err != nil {
		t.Fatalf("decode persisted messages: %v; body=%s", err, payload)
	}
	if len(history) != 4 || history[0].Role != "user" ||
		history[1].Role != "assistant" || history[1].Content != "Sync works" ||
		history[2].Role != "user" || history[3].Role != "assistant" ||
		history[3].Content != "Streaming works" {
		t.Fatalf("incorrect persisted chat lifecycle: %+v", history)
	}

	resp, payload = bob.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "chat_bob", "password": "password-for-bob"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	for _, path := range []string{messagesPath, messagesPath + "/stream"} {
		resp, payload = bob.request(t, http.MethodPost, path,
			map[string]interface{}{"content": "foreign chat", "tool_mode": "none"})
		chatHTTPRequireStatus(t, resp, payload, http.StatusNotFound)
	}
	resp, payload = bob.request(t, http.MethodGet, messagesPath, nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusNotFound)
	if syncCalls.Load() != 1 || streamCalls.Load() != 1 {
		t.Fatal("foreign-user requests reached the provider")
	}

	resp, payload = alice.request(t, http.MethodPost, "/v1/auth/logout", map[string]interface{}{})
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	resp, payload = alice.request(t, http.MethodPost, messagesPath,
		map[string]interface{}{"content": "revoked", "tool_mode": "none"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusUnauthorized)
}

func TestAuthenticatedChatRejectsBadSSERequestsBeforePersistence(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	router, shutdown := NewRouterWithShutdown(database, newAgentRuntimeRouterTestConfig(t), "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	alice := chatHTTPNewClient(t, server.URL)
	resp, payload := alice.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "bad_sse", "password": "password-for-sse"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	resp, payload = alice.request(t, http.MethodPost, "/v1/conversations/",
		map[string]interface{}{"title": "SSE validation"})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	convoID, ok := chatHTTPObject(t, payload)["id"].(string)
	if !ok || convoID == "" {
		t.Fatalf("missing conversation ID: %s", payload)
	}
	path := "/v1/conversations/" + convoID + "/messages/stream"
	for _, body := range []map[string]interface{}{
		{"content": ""},
		{"content": "not sent", "tool_mode": "invalid-mode"},
		{"content": "not sent", "tool_mode": "specific", "required_tool": ""},
	} {
		resp, payload = alice.request(t, http.MethodPost, path, body)
		chatHTTPRequireStatus(t, resp, payload, http.StatusBadRequest)
		if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Fatalf("rejected request opened an SSE stream: %s", payload)
		}
	}
	resp, payload = alice.request(t, http.MethodGet,
		"/v1/conversations/"+convoID+"/messages", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	var messages []interface{}
	if err := json.Unmarshal(payload, &messages); err != nil {
		t.Fatalf("decode message list: %v; body=%s", err, payload)
	}
	if len(messages) != 0 {
		t.Fatalf("invalid SSE requests persisted messages: %s", payload)
	}
}
