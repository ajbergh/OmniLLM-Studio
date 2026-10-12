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
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// The integration fixture uses the production search-provider interface, but
// never accesses Brave, DuckDuckGo, or any external endpoint.
type chatHTTPPreflightProvider struct {
	calls atomic.Int32
}

func (*chatHTTPPreflightProvider) Name() string { return "fixture-search" }

func (*chatHTTPPreflightProvider) Capabilities() websearch.ProviderCapabilities {
	return websearch.ProviderCapabilities{
		MaxQueriesPerTurn: 1, SupportsFreshnessFilter: true, ProvidesPublicationDates: true,
	}
}

func (p *chatHTTPPreflightProvider) Search(_ context.Context, req websearch.SearchRequest) (*websearch.SearchResponse, error) {
	p.calls.Add(1)
	now := time.Now().UTC()
	return &websearch.SearchResponse{
		Query: req.Query, TimeRange: req.TimeRange, FetchedAt: now,
		Results: []websearch.SearchResult{
			{
				Index: 1, Title: "Provider A pricing fixture",
				URL: "https://openai.com/api/pricing", Source: "Provider A",
				PublishedAt: now.Add(-time.Hour).Format(time.RFC3339),
				Snippet:     "Provider A fixture evidence: 6 units",
			},
			{
				Index: 2, Title: "Provider B pricing fixture",
				URL: "https://www.anthropic.com/pricing", Source: "Provider B",
				PublishedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
				Snippet:     "Provider B fixture evidence: 7 units",
			},
		},
	}, nil
}

// Unlike source-string wiring tests, this reaches both authenticated handlers
// and executes search -> provider tool request -> calculator -> final answer.
// It fails if the streaming summarizer dereferences a nil request in the
// preflight path, if search is skipped, or if tool results/sources are lost.
func TestChatHTTPPreflightThenToolLoopSyncAndSSE(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	search := &chatHTTPPreflightProvider{}
	var syncInitial, syncFinal, streamInitial, streamFinal atomic.Int32

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider path", http.StatusNotFound)
			return
		}
		var req struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
			http.Error(w, "invalid provider request", http.StatusBadRequest)
			return
		}
		evidence := false
		for _, msg := range req.Messages {
			if msg.Role == "system" && strings.Contains(msg.Content, "WEB EVIDENCE") &&
				strings.Contains(msg.Content, "openai.com/api/pricing") {
				evidence = true
			}
		}
		if !evidence {
			http.Error(w, "retrieved evidence was not passed to the model", http.StatusBadRequest)
			return
		}
		last := req.Messages[len(req.Messages)-1]
		isFinal := last.Role == "tool"
		message := map[string]interface{}{}
		if isFinal {
			if last.Content != "42" {
				http.Error(w, "incorrect calculator result", http.StatusBadRequest)
				return
			}
			message["content"] = "The calculated value is 42 from retrieved pricing [1] and [2]."
			if req.Stream {
				streamFinal.Add(1)
			} else {
				syncFinal.Add(1)
			}
		} else {
			hasCalculator := false
			for _, tool := range req.Tools {
				if tool.Function.Name == "calculator" {
					hasCalculator = true
				}
				if tool.Function.Name == "web_search" {
					http.Error(w, "preflight must not expose redundant web_search tool", http.StatusBadRequest)
					return
				}
			}
			if !hasCalculator {
				http.Error(w, "calculator omitted after preflight", http.StatusBadRequest)
				return
			}
			message["tool_calls"] = []interface{}{map[string]interface{}{
				"index": 0, "id": "preflight-calc-42", "type": "function",
				"function": map[string]interface{}{
					"name": "calculator", "arguments": `{"expression":"6*7"}`,
				},
			}}
			if req.Stream {
				streamInitial.Add(1)
			} else {
				syncInitial.Add(1)
			}
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			delta, _ := json.Marshal(map[string]interface{}{
				"choices": []interface{}{map[string]interface{}{"delta": message}},
			})
			fmt.Fprintf(w, "data: %s\n\n", delta)
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": message}},
			"usage":   map[string]interface{}{"prompt_tokens": 11, "completion_tokens": 4, "cost": 0.02},
		})
	}))
	defer provider.Close()
	baseURL := provider.URL + "/v1"
	model := "fixture-search-tool"
	if _, err := repository.NewProviderRepo(database).Create(repository.CreateProviderInput{
		Name: "Preflight then calculator", Type: "openai", BaseURL: &baseURL, DefaultModel: &model,
	}); err != nil {
		t.Fatal(err)
	}

	router, shutdown := newRouterWithSearchProvider(database, cfg, "test", "test", search)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()
	client := chatHTTPNewClient(t, server.URL)
	resp, body := client.request(t, http.MethodPost, "/v1/auth/register",
		map[string]interface{}{"username": "preflight_tool_user", "password": "preflight-tool-password"})
	chatHTTPRequireStatus(t, resp, body, http.StatusCreated)

	for _, tc := range []struct {
		name   string
		stream bool
	}{
		{name: "sync"}, {name: "SSE", stream: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := client.request(t, http.MethodPost, "/v1/conversations/",
				map[string]interface{}{"title": "Search and tool " + tc.name})
			chatHTTPRequireStatus(t, resp, body, http.StatusCreated)
			conversationID, ok := chatHTTPObject(t, body)["id"].(string)
			if !ok || conversationID == "" {
				t.Fatalf("invalid conversation response: %s", body)
			}

			path := "/v1/conversations/" + conversationID + "/messages"
			prompt := map[string]interface{}{
				"content":    "Find the latest API prices and calculate the monthly total using a calculator",
				"web_search": true, "tool_mode": "auto",
			}
			sendPath := path
			if tc.stream {
				sendPath += "/stream"
			}
			before := search.calls.Load()
			resp, body = client.request(t, http.MethodPost, sendPath, prompt)
			chatHTTPRequireStatus(t, resp, body, http.StatusOK)
			if search.calls.Load() <= before {
				t.Fatalf("preflight never queried search provider: %s", body)
			}

			var meta map[string]interface{}
			if tc.stream {
				if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
					t.Fatalf("unexpected SSE Content-Type: %q", resp.Header.Get("Content-Type"))
				}
				var order []string
				for _, frame := range strings.Split(strings.TrimSpace(string(body)), "\n\n") {
					parts := strings.SplitN(frame, "\ndata: ", 2)
					if len(parts) != 2 || !strings.HasPrefix(parts[0], "event: ") {
						t.Fatalf("malformed SSE frame: %q", frame)
					}
					name := strings.TrimPrefix(parts[0], "event: ")
					order = append(order, name)
					if name == "done" {
						meta = chatHTTPObject(t, []byte(parts[1]))
					}
				}
				first := func(name string) int {
					for i, value := range order {
						if value == name {
							return i
						}
					}
					return -1
				}
				if first("start") != 0 || first("web_search_results") < 1 ||
					first("tool_result") <= first("web_search_results") ||
					first("done") <= first("tool_result") || order[len(order)-1] != "done" {
					t.Fatalf("SSE evidence/tool/done ordering broken: %v", order)
				}
				if meta["content"] != "The calculated value is 42 from retrieved pricing [1] and [2]." {
					t.Fatalf("streamed final content: %v", meta)
				}
			} else {
				message := chatHTTPObject(t, body)
				if message["content"] != "The calculated value is 42 from retrieved pricing [1] and [2]." {
					t.Fatalf("synchronous response: %s", body)
				}
				m, ok := message["metadata_json"].(string)
				if !ok || json.Unmarshal([]byte(m), &meta) != nil {
					t.Fatalf("missing saved metadata: %s", body)
				}
				if _, ok := meta["cost"]; !ok {
					t.Fatalf("missing provider cost in sync metadata: %v", meta)
				}
			}

			if meta["search_attempted"] != true || meta["web_search"] != true ||
				meta["search_failed"] == true || meta["search_mechanism"] != "local" {
				t.Fatalf("search metadata invalid: %#v", meta)
			}
			if sources, ok := meta["sources"].([]interface{}); !ok || len(sources) < 2 {
				t.Fatalf("missing retrieval citations/sources: %#v", meta)
			}
			if calls, ok := meta["tool_calls"].([]interface{}); !ok || len(calls) != 1 {
				t.Fatalf("missing calculator call: %#v", meta)
			}
			if results, ok := meta["tool_results"].([]interface{}); !ok || len(results) != 1 {
				t.Fatalf("missing calculator result: %#v", meta)
			}

			resp, body = client.request(t, http.MethodGet, path, nil)
			chatHTTPRequireStatus(t, resp, body, http.StatusOK)
			var saved []struct {
				Role         string `json:"role"`
				Content      string `json:"content"`
				MetadataJSON string `json:"metadata_json"`
			}
			if err := json.Unmarshal(body, &saved); err != nil {
				t.Fatalf("invalid persisted history: %v", err)
			}
			if len(saved) != 2 || saved[0].Role != "user" ||
				saved[1].Role != "assistant" || !strings.Contains(saved[1].Content, "42") {
				t.Fatalf("incorrect persisted conversation: %+v", saved)
			}
			var persisted map[string]interface{}
			if err := json.Unmarshal([]byte(saved[1].MetadataJSON), &persisted); err != nil ||
				persisted["web_search"] != true || persisted["search_mechanism"] != "local" {
				t.Fatalf("retrieval metadata not persisted: meta=%v err=%v", persisted, err)
			}
		})
	}
	if syncInitial.Load() != 1 || syncFinal.Load() != 1 ||
		streamInitial.Load() != 1 || streamFinal.Load() != 1 {
		t.Fatalf("provider rounds sync=%d/%d, stream=%d/%d",
			syncInitial.Load(), syncFinal.Load(), streamInitial.Load(), streamFinal.Load())
	}
}
