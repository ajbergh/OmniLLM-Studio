package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/turncontext"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// recordingProvider captures every SearchRequest the orchestrator issues, so the
// test can assert on what the server actually decided rather than on what the
// model asked for.
type recordingProvider struct {
	mu       sync.Mutex
	requests []websearch.SearchRequest
	results  []websearch.SearchResult
	err      error
}

func (p *recordingProvider) Name() string { return "recording" }

func (p *recordingProvider) Search(_ context.Context, req websearch.SearchRequest) (*websearch.SearchResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return &websearch.SearchResponse{
		Query:     req.Query,
		TimeRange: req.TimeRange,
		Results:   p.results,
	}, nil
}

func (p *recordingProvider) seen() []websearch.SearchRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]websearch.SearchRequest(nil), p.requests...)
}

func newToolUnderTest(t *testing.T, provider *recordingProvider) *WebSearchTool {
	t.Helper()
	orch := websearch.NewOrchestrator(provider, nil, nil)
	return NewWebSearchTool(orch, "openai", "gpt-5")
}

func turnCtx(t *testing.T) context.Context {
	t.Helper()
	return turncontext.WithClientContext(context.Background(), turncontext.ClientContext{
		Timezone: "America/Chicago",
		Locale:   "en-GB",
		Country:  "GB",
	})
}

func decodePayload(t *testing.T, res *ToolResult) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(res.Content), &payload); err != nil {
		t.Fatalf("tool content must be a JSON object: %v", err)
	}
	return payload
}

// TestWebSearchToolInjectsServerDefaults is the regression test for the tool
// being weaker than the REST endpoint that wraps the same orchestrator method:
// a bare {"query": "..."} used to reach the provider with no region, no locale,
// and no freshness window at all.
func TestWebSearchToolInjectsServerDefaults(t *testing.T) {
	provider := &recordingProvider{results: []websearch.SearchResult{
		{Index: 1, Title: "Kubernetes releases", URL: "https://kubernetes.io/releases/", Snippet: "current versions"},
	}}
	tool := newToolUnderTest(t, provider)

	res, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"current kubernetes release"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Content)
	}

	seen := provider.seen()
	if len(seen) == 0 {
		t.Fatal("provider was never called")
	}
	if seen[0].Region != "GB" {
		t.Errorf("Region = %q; the server must supply it from turn context", seen[0].Region)
	}
	if seen[0].Locale != "en-GB" {
		t.Errorf("Locale = %q; the server must supply it from turn context", seen[0].Locale)
	}
	if seen[0].MaxResults <= 0 {
		t.Errorf("MaxResults = %d; the server must choose a bound", seen[0].MaxResults)
	}
}

// TestWebSearchToolUsesPlannerFreshness proves the tool goes through the planner
// rather than DirectSearch: a release-intent query must carry no recency filter,
// which only the planner knows.
func TestWebSearchToolUsesPlannerFreshness(t *testing.T) {
	cases := []struct {
		name          string
		query         string
		wantTimeRange string
	}{
		{"release lookup gets no filter", "current kubernetes release", ""},
		{"pricing lookup gets no filter", "current openai api pricing", ""},
		{"breaking news keeps a tight window", "breaking news today", "24h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &recordingProvider{results: []websearch.SearchResult{
				{Index: 1, Title: "t", URL: "https://example.com/a"},
			}}
			tool := newToolUnderTest(t, provider)
			args, _ := json.Marshal(map[string]string{"query": tc.query})
			if _, err := tool.Execute(turnCtx(t), args); err != nil {
				t.Fatal(err)
			}
			seen := provider.seen()
			if seen[0].TimeRange != tc.wantTimeRange {
				t.Errorf("TimeRange = %q, want %q", seen[0].TimeRange, tc.wantTimeRange)
			}
		})
	}
}

func TestWebSearchToolHonorsExplicitTimeRange(t *testing.T) {
	provider := &recordingProvider{results: []websearch.SearchResult{
		{Index: 1, Title: "t", URL: "https://example.com/a"},
	}}
	tool := newToolUnderTest(t, provider)
	if _, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"openai pricing","time_range":"7d"}`)); err != nil {
		t.Fatal(err)
	}
	if got := provider.seen()[0].TimeRange; got != "7d" {
		t.Errorf("an explicit time_range must be honored, got %q", got)
	}
}

// TestWebSearchToolReturnsRetrievalMetadata covers the missing timestamp: the old
// payload was a bare result array, so the model had no way to tell dated results
// from undated ones and presented both as current.
func TestWebSearchToolReturnsRetrievalMetadata(t *testing.T) {
	provider := &recordingProvider{results: []websearch.SearchResult{
		{Index: 1, Title: "Anthropic pricing", URL: "https://www.anthropic.com/pricing", PublishedAt: "2 days ago"},
		{Index: 2, Title: "Blog copy", URL: "https://blog.example.com/prices"},
	}}
	tool := newToolUnderTest(t, provider)

	res, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"current anthropic api pricing"}`))
	if err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, res)
	for _, key := range []string{"query", "queries_run", "time_range", "fetched_at", "result_count", "evidence_sufficient", "guidance", "results", "intent"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("payload is missing %q", key)
		}
	}
	if got, _ := payload["fetched_at"].(string); got == "" || !strings.HasSuffix(got, "Z") {
		t.Errorf("fetched_at = %q, want a UTC timestamp", got)
	}
	// An empty window must read as an explicit statement, not an absent field.
	if got, _ := payload["time_range"].(string); !strings.HasPrefix(got, "none") {
		t.Errorf("time_range = %q, want an explicit 'none' label", got)
	}
	if payload["intent"] != "pricing" {
		t.Errorf("intent = %v, want pricing", payload["intent"])
	}

	for _, key := range []string{"fetched_at", "time_range", "evidence_sufficient", "sources"} {
		if _, ok := res.Metadata[key]; !ok {
			t.Errorf("metadata is missing %q", key)
		}
	}
}

// TestWebSearchToolRanksOfficialSourcesFirst verifies the plan's preferred
// domains reach the model in priority order, since result order is what the model
// reads first.
func TestWebSearchToolRanksOfficialSourcesFirst(t *testing.T) {
	provider := &recordingProvider{results: []websearch.SearchResult{
		{Index: 1, Title: "Aggregator", URL: "https://blog.example.com/llm-prices"},
		{Index: 2, Title: "Official", URL: "https://www.anthropic.com/pricing"},
	}}
	tool := newToolUnderTest(t, provider)

	res, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"current anthropic api pricing"}`))
	if err != nil {
		t.Fatal(err)
	}
	payload := decodePayload(t, res)
	results, ok := payload["results"].([]interface{})
	if !ok || len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %v", payload["results"])
	}
	first, _ := results[0].(map[string]interface{})
	if u, _ := first["url"].(string); !strings.Contains(u, "anthropic.com") {
		t.Errorf("first result = %q; the official pricing page must outrank the aggregator", u)
	}
}

func TestWebSearchToolFlagsInsufficientEvidence(t *testing.T) {
	// A pricing plan requires two distinct hosts including a first-party page.
	provider := &recordingProvider{results: []websearch.SearchResult{
		{Index: 1, Title: "Aggregator", URL: "https://blog.example.com/llm-prices"},
	}}
	tool := newToolUnderTest(t, provider)

	res, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"current anthropic api pricing"}`))
	if err != nil {
		t.Fatal(err)
	}
	payload := decodePayload(t, res)
	if payload["evidence_sufficient"] != false {
		t.Error("a single aggregator must not satisfy a pricing plan")
	}
	guidance, _ := payload["guidance"].(string)
	if !strings.Contains(guidance, "could not verify") {
		t.Errorf("guidance must tell the model to hedge, got %q", guidance)
	}
}

func TestWebSearchToolValidation(t *testing.T) {
	tool := newToolUnderTest(t, &recordingProvider{})
	if err := tool.Validate(json.RawMessage(`{"query":"x"}`)); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	if err := tool.Validate(json.RawMessage(`{}`)); err == nil {
		t.Error("missing query must be rejected")
	}
	if err := tool.Validate(json.RawMessage(`not json`)); err == nil {
		t.Error("invalid JSON must be rejected")
	}
}

func TestWebSearchToolPropagatesProviderFailure(t *testing.T) {
	provider := &recordingProvider{err: errProviderDown}
	tool := newToolUnderTest(t, provider)
	if _, err := tool.Execute(turnCtx(t), json.RawMessage(`{"query":"anything"}`)); err == nil {
		t.Fatal("a provider failure must surface as a tool error, not an empty success")
	}
}

func TestWebSearchToolDefinitionDocumentsServerDefaults(t *testing.T) {
	def := newToolUnderTest(t, &recordingProvider{}).Definition()
	if def.Name != "web_search" {
		t.Fatalf("Name = %q", def.Name)
	}
	schema := string(def.Parameters)
	// The schema must tell the model that omitting time_range is safe, so it does
	// not guess a window that the server would have chosen better.
	if !strings.Contains(schema, "the server infers") {
		t.Error("time_range description must state that the server decides when it is omitted")
	}
	if !strings.Contains(def.Description, "retrieval timestamp") {
		t.Error("description must advertise the retrieval timestamp the payload now carries")
	}
}

var errProviderDown = errProviderDownType{}

type errProviderDownType struct{}

func (errProviderDownType) Error() string { return "provider unreachable" }
