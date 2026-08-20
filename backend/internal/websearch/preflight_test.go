package websearch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/turncontext"
)

type stubProvider struct {
	mu         sync.Mutex
	requests   []SearchRequest
	results    []SearchResult
	err        error
	maxQueries int
}

func (p *stubProvider) Name() string { return "stub" }

func (p *stubProvider) Capabilities() ProviderCapabilities {
	if p.maxQueries > 0 {
		return ProviderCapabilities{MaxQueriesPerTurn: p.maxQueries}
	}
	return ProviderCapabilities{MaxQueriesPerTurn: 5, SupportsFreshnessFilter: true, ProvidesPublicationDates: true}
}

func (p *stubProvider) Search(_ context.Context, req SearchRequest) (*SearchResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return &SearchResponse{Query: req.Query, TimeRange: req.TimeRange, Results: p.results, FetchedAt: time.Now().UTC()}, nil
}

func (p *stubProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func preflightCtx() context.Context {
	return turncontext.WithClientContext(context.Background(), turncontext.ClientContext{
		Timezone: "America/Chicago",
		Locale:   "en-US",
		Country:  "US",
	})
}

// TestPreflightGathersReusableEvidence covers the composition gap: Process and
// ProcessStream answer and terminate the turn, so a compound request could get
// retrieval or a follow-up tool but never both.
func TestPreflightGathersReusableEvidence(t *testing.T) {
	provider := &stubProvider{results: []SearchResult{
		{Index: 1, Title: "Anthropic pricing", URL: "https://www.anthropic.com/pricing", Snippet: "$3 per million input tokens", PublishedAt: "2 days ago"},
		{Index: 2, Title: "OpenAI pricing", URL: "https://openai.com/api/pricing", Snippet: "current rates"},
	}}
	orch := NewOrchestrator(provider, nil, nil)

	evidence, toolCall, err := orch.Preflight(preflightCtx(), "Find the latest API prices and calculate the monthly total", false)
	if err != nil {
		t.Fatal(err)
	}
	if evidence == nil {
		t.Fatal("expected evidence")
	}
	if toolCall == nil || toolCall.Name != "web_search" {
		t.Fatalf("expected a web_search tool call for the UI, got %#v", toolCall)
	}
	if len(evidence.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(evidence.Results))
	}
	if evidence.FetchedAt.IsZero() {
		t.Error("FetchedAt must be stamped so the model can report retrieval time")
	}
	if !evidence.Sufficient {
		t.Error("two first-party pricing pages should satisfy a pricing plan")
	}
}

func TestPreflightSkipsNonCurrentQuestions(t *testing.T) {
	provider := &stubProvider{}
	orch := NewOrchestrator(provider, nil, nil)

	evidence, toolCall, err := orch.Preflight(preflightCtx(), "Explain the quicksort algorithm", false)
	if err != nil {
		t.Fatalf("a non-current question is not an error: %v", err)
	}
	if evidence != nil || toolCall != nil {
		t.Error("no plan means no retrieval")
	}
	if provider.callCount() != 0 {
		t.Error("the provider must not be called when the plan declines")
	}
}

// TestPreflightReturnsToolCallOnFailure guards a silent-failure path: the handler
// keys its "retrieval attempted and failed" branch on a non-nil tool call, so
// returning nil there would make the failure invisible.
func TestPreflightReturnsToolCallOnFailure(t *testing.T) {
	provider := &stubProvider{err: errors.New("provider unreachable")}
	orch := NewOrchestrator(provider, nil, nil)

	evidence, toolCall, err := orch.Preflight(preflightCtx(), "Find the latest prices and calculate the total", false)
	if err == nil {
		t.Fatal("a provider failure must surface as an error")
	}
	if evidence != nil {
		t.Error("no evidence on failure")
	}
	if toolCall == nil {
		t.Fatal("the tool call must survive failure so the UI can report the attempt")
	}
}

func TestPreflightEmptyResultsAreAFailure(t *testing.T) {
	provider := &stubProvider{results: nil}
	orch := NewOrchestrator(provider, nil, nil)

	_, toolCall, err := orch.Preflight(preflightCtx(), "Find the latest prices and calculate the total", false)
	if err == nil {
		t.Fatal("zero results must be an error, not an empty success")
	}
	if toolCall == nil {
		t.Error("the tool call must survive an empty result set")
	}
}

func TestEvidenceSystemMessageContent(t *testing.T) {
	provider := &stubProvider{results: []SearchResult{
		{Index: 1, Title: "Anthropic pricing", URL: "https://www.anthropic.com/pricing", Snippet: "$3 per million input tokens"},
	}}
	orch := NewOrchestrator(provider, nil, nil)

	evidence, _, err := orch.Preflight(preflightCtx(), "Find the current API pricing and calculate the monthly total", false)
	if err != nil {
		t.Fatal(err)
	}

	msg := evidence.EvidenceSystemMessage(turncontext.FromContext(preflightCtx()))
	if msg.Role != "system" {
		t.Fatalf("Role = %q, want system", msg.Role)
	}
	for _, want := range []string{
		"WEB EVIDENCE",
		"Retrieved at:",
		"Freshness filter:",
		"anthropic.com/pricing",
		"Cite them by index",
	} {
		if !strings.Contains(msg.Content, want) {
			t.Errorf("evidence message missing %q\n---\n%s", want, msg.Content)
		}
	}
	// A pricing plan applies no recency filter, and that must be stated rather
	// than left absent, so the model does not read silence as "unknown".
	if !strings.Contains(msg.Content, "none (results are not restricted by recency)") {
		t.Error("an empty freshness window must be stated explicitly")
	}
}

func TestEvidenceSystemMessageWarnsOnInsufficientEvidence(t *testing.T) {
	// One aggregator does not satisfy a pricing plan's two-source requirement.
	provider := &stubProvider{results: []SearchResult{
		{Index: 1, Title: "Blog", URL: "https://blog.example.com/prices", Snippet: "prices"},
	}}
	orch := NewOrchestrator(provider, nil, nil)

	evidence, _, err := orch.Preflight(preflightCtx(), "Find the current API pricing and calculate the total", false)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Sufficient {
		t.Fatal("a single aggregator must not satisfy a pricing plan")
	}
	msg := evidence.EvidenceSystemMessage(turncontext.FromContext(preflightCtx()))
	if !strings.Contains(msg.Content, "did NOT meet") {
		t.Errorf("the directive must tell the model to hedge:\n%s", msg.Content)
	}
}

// TestPreflightForceOverridesGate covers a silent no-op: the semantic router can
// classify a turn as needing current information, but Preflight re-runs the
// deterministic gate and would veto it — producing a turn that reported an
// attempted search and performed none.
func TestPreflightForceOverridesGate(t *testing.T) {
	provider := &stubProvider{results: []SearchResult{
		{Index: 1, Title: "Comparison", URL: "https://example.com/models", Snippet: "..."},
	}}
	orch := NewOrchestrator(provider, nil, nil)

	// Phrasing the deterministic gate declines: no recency word, no strong signal.
	const semantic = "how do the two providers stack up for my workload"

	if BuildSearchPlan(semantic, turncontext.FromContext(preflightCtx()).Now, "UTC").NeedsWeb {
		t.Skip("the gate now triggers on this phrasing; pick another for the force test")
	}

	evidence, _, err := orch.Preflight(preflightCtx(), semantic, false)
	if err != nil || evidence != nil {
		t.Fatalf("without force the gate must still decline: evidence=%v err=%v", evidence, err)
	}
	if provider.callCount() != 0 {
		t.Fatal("no provider call without force")
	}

	forced, toolCall, err := orch.Preflight(preflightCtx(), semantic, true)
	if err != nil {
		t.Fatalf("force must retrieve: %v", err)
	}
	if forced == nil || len(forced.Results) == 0 {
		t.Fatal("force must produce evidence")
	}
	if toolCall == nil {
		t.Fatal("force must produce a tool call for the UI")
	}
	if provider.callCount() == 0 {
		t.Fatal("the provider must actually be called")
	}
}

func TestEvidenceSystemMessageNilSafe(t *testing.T) {
	var evidence *PreflightEvidence
	if msg := evidence.EvidenceSystemMessage(turncontext.TurnContext{}); msg.Content != "" {
		t.Error("a nil evidence pointer must render nothing")
	}
}

// TestPreflightRunsExpandedQuerySet ties the plan's query expansion to real
// provider calls. Before Phase 1 the plan carried one query and MaxIterations
// above 1 was dead, so a research preflight issued exactly one search.
func TestPreflightRunsExpandedQuerySet(t *testing.T) {
	// Only an aggregator comes back, so the plan's source-class requirement is
	// never satisfied and the loop uses its full iteration budget.
	provider := &stubProvider{results: []SearchResult{
		{Index: 1, Title: "Blog", URL: "https://blog.example.com/bench", Snippet: "scores"},
	}}
	orch := NewOrchestrator(provider, nil, nil)

	if _, _, err := orch.Preflight(preflightCtx(), "Research the best LLM available via API and compare benchmark versus cost", false); err != nil {
		t.Fatal(err)
	}
	if got := provider.callCount(); got < 2 {
		t.Errorf("provider was called %d time(s); a research plan must issue its expanded query set", got)
	}
}
