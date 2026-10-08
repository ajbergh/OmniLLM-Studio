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

// TestChatHTTPToolLoopIterationBoundary proves that both transport variants
// stop a provider that repeatedly asks for tools and force a final, tool-free
// answer rather than permitting an unbounded execution chain.
func TestChatHTTPToolLoopIterationBoundary(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	var syncCalls, streamCalls atomic.Int32

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var request struct {
			Stream bool `json:"stream"`
			Tools  []interface{} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid provider request", http.StatusBadRequest)
			return
		}
		var count int32
		if request.Stream {
			count = streamCalls.Add(1)
		} else {
			count = syncCalls.Add(1)
		}
		message := map[string]interface{}{}
		if len(request.Tools) == 0 {
			message["content"] = "Bounded final answer"
		} else {
			message["tool_calls"] = []interface{}{map[string]interface{}{
				"index": 0, "id": fmt.Sprintf("call-%d", count), "type": "function",
				"function": map[string]interface{}{
					"name": "calculator", "arguments": `{"expression":"6*7"}`,
				},
			}}
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			delta := map[string]interface{}{}
			if message["content"] != nil {
				delta["content"] = message["content"]
			}
			if message["tool_calls"] != nil {
				delta["tool_calls"] = message["tool_calls"]
			}
			payload, err := json.Marshal(map[string]interface{}{
				"choices": []interface{}{map[string]interface{}{"delta": delta}},
			})
			if err != nil {
				panic(err)
			}
			fmt.Fprint(w, "data: ", string(payload), string([]byte{10, 10}))
			fmt.Fprint(w, "data: [DONE]", string([]byte{10, 10}))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": message}},
			"usage":   map[string]interface{}{"prompt_tokens": 1, "completion_tokens": 1, "cost": 0.001},
		})
	}))
	defer provider.Close()

	baseURL := provider.URL + "/v1"
	model := "fixture-tool-limit"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Loop limit provider", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
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
		map[string]interface{}{"username": "chat_loop_limit_user", "password": "loop-limit-passphrase"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)

	for _, tc := range []struct {
		name        string
		stream      bool
		maxRequests int32
	}{
		{name: "synchronous", maxRequests: syncMaxToolLoops + 1},
		{name: "SSE", stream: true, maxRequests: 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := client.request(t, http.MethodPost, "/v1/conversations/",
				map[string]interface{}{"title": tc.name + " tool limit"})
			chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
			id, ok := chatHTTPObject(t, body)["id"].(string)
			if !ok || id == "" {
				t.Fatalf("missing conversation ID: %s", body)
			}
			path := "/v1/conversations/" + id + "/messages"
			prompt := map[string]interface{}{
				"content": "Keep calculating six times seven forever", "web_search": false,
				"tool_mode": "specific", "required_tool": "calculator",
			}
			sendPath := path
			if tc.stream {
				sendPath += "/stream"
			}
			resp, body = client.request(t, http.MethodPost, sendPath, prompt)
			chatHTTPRequireStatus(t, resp, body, http.StatusOK)
			if tc.stream {
				if !strings.Contains(string(body), "event: tool_loop_limit") ||
					!strings.Contains(string(body), "TOOL_LOOP_LIMIT") ||
					!strings.Contains(string(body), "event: done") ||
					!strings.Contains(string(body), "Bounded final answer") {
					t.Fatalf("missing SSE tool limit/final answer: %s", body)
				}
			} else {
				if content := chatHTTPObject(t, body)["content"]; content != "Bounded final answer" {
					t.Fatalf("sync limit final answer = %v", content)
				}
				var meta map[string]interface{}
				value, ok := chatHTTPObject(t, body)["metadata_json"].(string)
				if !ok || json.Unmarshal([]byte(value), &meta) != nil {
					t.Fatalf("missing sync tool-loop metadata: %s", body)
				}
				if calls, ok := meta["tool_calls"].([]interface{}); !ok || len(calls) != syncMaxToolLoops {
					t.Fatalf("sync tool-call count = %v", meta["tool_calls"])
				}
			}
			got := syncCalls.Load()
			if tc.stream {
				got = streamCalls.Load()
			}
			if got != tc.maxRequests {
				t.Fatalf("provider round count = %d, want %d", got, tc.maxRequests)
			}

			resp, body = client.request(t, http.MethodGet, path, nil)
			chatHTTPRequireStatus(t, resp, body, http.StatusOK)
			var messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(body, &messages); err != nil {
				t.Fatalf("decode saved messages: %v", err)
			}
			if len(messages) != 2 || messages[0].Role != "user" ||
				messages[1].Role != "assistant" || messages[1].Content != "Bounded final answer" {
				t.Fatalf("incorrect persisted result at tool iteration limit: %+v", messages)
			}
		})
	}
}
