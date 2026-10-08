package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// TestChatHTTPToolLoopEndToEnd exercises the real synchronous and SSE handlers
// through the production router, a persisted SQLite conversation, and a
// deterministic provider that asks for one calculator tool call before answering.
func TestChatHTTPToolLoopEndToEnd(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	var syncInitial, syncFinal, streamInitial, streamFinal atomic.Int32

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var request struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []interface{} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) == 0 {
			http.Error(w, "invalid provider request", http.StatusBadRequest)
			return
		}
		last := request.Messages[len(request.Messages)-1]
		writeSSE := func(delta map[string]interface{}) {
			payload, err := json.Marshal(map[string]interface{}{
				"choices": []interface{}{map[string]interface{}{"delta": delta}},
			})
			if err != nil {
				panic(err)
			}
			fmt.Fprint(w, "data: ", string(payload), string([]byte{10, 10}))
		}
		toolCall := map[string]interface{}{
			"index": 0, "id": "calc-call-42", "type": "function",
			"function": map[string]interface{}{
				"name": "calculator", "arguments": `{"expression":"6*7"}`,
			},
		}
		if last.Role == "tool" {
			if last.Content != "42" {
				http.Error(w, "calculator returned wrong result", http.StatusBadRequest)
				return
			}
			if request.Stream {
				streamFinal.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				writeSSE(map[string]interface{}{"content": "Calculated 42"})
				fmt.Fprint(w, "data: [DONE]", string([]byte{10, 10}))
				return
			}
			syncFinal.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": "Calculated 42"}}},
				"usage": map[string]interface{}{"prompt_tokens": 11, "completion_tokens": 3, "cost": 0.02},
			})
			return
		}
		if len(request.Tools) == 0 {
			http.Error(w, "expected declared tools on first round", http.StatusBadRequest)
			return
		}
		if request.Stream {
			streamInitial.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			writeSSE(map[string]interface{}{"tool_calls": []interface{}{toolCall}})
			fmt.Fprint(w, "data: [DONE]", string([]byte{10, 10}))
			return
		}
		syncInitial.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{
				"content": "", "tool_calls": []interface{}{toolCall},
			}}},
			"usage": map[string]interface{}{"prompt_tokens": 8, "completion_tokens": 4, "cost": 0.01},
		})
	}))
	defer provider.Close()

	baseURL := provider.URL + "/v1"
	model := "fixture-tool-chat"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Chat tool-loop fixture", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
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
	response, data := client.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "chat_tool_user", "password": "tool-passphrase"})
	chatHTTPRequireStatus(t, response, data, http.StatusCreated)

	response, data = client.request(t, http.MethodPost, "/v1/conversations/",
		map[string]interface{}{"title": "Tool-loop HTTP regression"})
	chatHTTPRequireStatus(t, response, data, http.StatusCreated)
	conversationID, ok := chatHTTPObject(t, data)["id"].(string)
	if !ok || conversationID == "" {
		t.Fatalf("missing conversation ID: %s", data)
	}
	basePath := "/v1/conversations/" + conversationID + "/messages"
	toolRequest := map[string]interface{}{
		"content": "Calculate six times seven using the calculator.",
		"web_search": false, "tool_mode": "specific", "required_tool": "calculator",
	}

	response, data = client.request(t, http.MethodPost, basePath, toolRequest)
	chatHTTPRequireStatus(t, response, data, http.StatusOK)
	syncResponse := chatHTTPObject(t, data)
	if syncResponse["content"] != "Calculated 42" {
		t.Fatalf("sync tool-loop answer = %q", data)
	}
	var syncMetadata map[string]interface{}
	metaText, _ := syncResponse["metadata_json"].(string)
	if err := json.Unmarshal([]byte(metaText), &syncMetadata); err != nil {
		t.Fatalf("decode persisted sync tool metadata: %v, payload=%s", err, data)
	}
	if calls, ok := syncMetadata["tool_calls"].([]interface{}); !ok || len(calls) != 1 {
		t.Fatalf("missing sync tool call metadata: %v", syncMetadata)
	}
	if results, ok := syncMetadata["tool_results"].([]interface{}); !ok || len(results) != 1 {
		t.Fatalf("missing sync tool result metadata: %v", syncMetadata)
	}
	if syncMetadata["tool_requirement_unfulfilled"] == true {
		t.Fatalf("successful calculator marked unfulfilled: %v", syncMetadata)
	}

	response, data = client.request(t, http.MethodPost, basePath+"/stream", toolRequest)
	chatHTTPRequireStatus(t, response, data, http.StatusOK)
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", response.Header.Get("Content-Type"))
	}

	var names []string
	var completed map[string]interface{}
	for _, frame := range strings.Split(strings.TrimSpace(string(data)), string([]byte{10, 10})) {
		pieces := strings.SplitN(frame, string([]byte{10})+"data: ", 2)
		if len(pieces) != 2 || !strings.HasPrefix(pieces[0], "event: ") {
			t.Fatalf("invalid SSE frame: %q", frame)
		}
		name := strings.TrimPrefix(pieces[0], "event: ")
		names = append(names, name)
		if name == "done" {
			completed = chatHTTPObject(t, []byte(pieces[1]))
		}
	}
	contains := func(name string) bool {
		for _, got := range names {
			if got == name {
				return true
			}
		}
		return false
	}
	if len(names) < 3 || names[0] != "start" || names[len(names)-1] != "done" || !contains("tool_result") {
		t.Fatalf("tool result lifecycle missing or out of order: %v", names)
	}
	if completed["content"] != "Calculated 42" {
		t.Fatalf("streamed final answer = %v", completed)
	}
	if calls, ok := completed["tool_calls"].([]interface{}); !ok || len(calls) != 1 {
		t.Fatalf("SSE done missing tool calls: %v", completed)
	}
	if results, ok := completed["tool_results"].([]interface{}); !ok || len(results) != 1 {
		t.Fatalf("SSE done missing tool results: %v", completed)
	}
	if syncInitial.Load() != 1 || syncFinal.Load() != 1 ||
		streamInitial.Load() != 1 || streamFinal.Load() != 1 {
		t.Fatalf("unexpected provider rounds sync=%d/%d stream=%d/%d",
			syncInitial.Load(), syncFinal.Load(), streamInitial.Load(), streamFinal.Load())
	}
	response, data = client.request(t, http.MethodGet, basePath, nil)
	chatHTTPRequireStatus(t, response, data, http.StatusOK)
	var saved []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode persisted conversation: %v", err)
	}
	if len(saved) != 4 || saved[1].Content != "Calculated 42" || saved[3].Content != "Calculated 42" {
		t.Fatalf("conversation missing saved tool-loop answers: %+v", saved)
	}
}
