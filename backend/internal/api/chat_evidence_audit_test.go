package api

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

var auditNow = time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

// TestAuditFillsSourcesForNativeGrounding is the fix for an empty source panel.
//
// ProcessStream's native branch returns a SearchResponse with no Results, so
// metadata.sources was an empty array for every natively-grounded answer — and
// the frontend's `data.sources || fallback` could not recover it, because an
// empty array is truthy. The citations existed, but only as markdown inside the
// answer text.
func TestAuditFillsSourcesForNativeGrounding(t *testing.T) {
	native := []llm.Citation{
		{URL: "https://www.anthropic.com/pricing", Title: "Pricing"},
		{URL: "https://openai.com/api/pricing", Title: "API pricing"},
	}
	audit := auditAnswerEvidence(websearch.SearchPlan{}, "Prices are on the vendor pages.", nil, native, auditNow)

	meta := map[string]interface{}{}
	audit.applyTo(meta)

	sources, ok := meta[metaSources].([]websearch.SearchResult)
	if !ok || len(sources) != 2 {
		t.Fatalf("native citations must populate metadata.sources, got %#v", meta[metaSources])
	}
	if sources[0].URL != "https://www.anthropic.com/pricing" || sources[0].Index != 1 {
		t.Errorf("unexpected first source: %#v", sources[0])
	}
	if _, present := meta[metaNativeCitations]; !present {
		t.Error("the structured citation list must also be recorded")
	}
	if meta[metaCitationCount] != 2 {
		t.Errorf("%s = %v, want 2", metaCitationCount, meta[metaCitationCount])
	}
}

// TestAuditDoesNotOverwriteLocalSources guards the ordering: local retrieval
// results are richer than native citations (they carry snippets and dates), so
// they must win.
func TestAuditDoesNotOverwriteLocalSources(t *testing.T) {
	local := []websearch.SearchResult{
		{Index: 1, URL: "https://www.anthropic.com/pricing", Snippet: "$3 per million"},
	}
	native := []llm.Citation{{URL: "https://example.com/other"}}
	audit := auditAnswerEvidence(websearch.SearchPlan{}, "answer", local, native, auditNow)

	meta := map[string]interface{}{metaSources: local}
	audit.applyTo(meta)

	sources, _ := meta[metaSources].([]websearch.SearchResult)
	if len(sources) != 1 || sources[0].Snippet == "" {
		t.Errorf("local results must be preserved, got %#v", sources)
	}
}

func TestAuditRecordsVerifiedFreshness(t *testing.T) {
	plan := websearch.SearchPlan{TimeRange: "24h"}
	sources := []websearch.SearchResult{
		{Index: 1, URL: "https://a.test/1", PublishedAt: "2 hours ago"},
	}
	audit := auditAnswerEvidence(plan, "Something happened [1].", sources, nil, auditNow)

	meta := map[string]interface{}{}
	audit.applyTo(meta)

	if meta[metaFreshnessVerified] != true {
		t.Error("dated results inside the window must set freshness_verified")
	}
	if meta[metaAnswerFreshness] != "2 hours ago" {
		t.Errorf("%s = %v", metaAnswerFreshness, meta[metaAnswerFreshness])
	}
}

func TestAuditOmitsFreshnessWhenUndated(t *testing.T) {
	plan := websearch.SearchPlan{TimeRange: "24h"}
	sources := []websearch.SearchResult{{Index: 1, URL: "https://a.test/1"}}
	audit := auditAnswerEvidence(plan, "Something happened [1].", sources, nil, auditNow)

	meta := map[string]interface{}{}
	audit.applyTo(meta)

	if _, present := meta[metaFreshnessVerified]; present {
		t.Error("undated results must not claim verified freshness")
	}
	if _, present := meta[metaAnswerFreshness]; present {
		t.Error("there is no newest-source label without a date")
	}
}

// TestAuditClaimWarningIsAWarning documents the deliberate limit on 4.4: the
// audit reports a signal, it does not reject. Callers must be able to render it
// without the answer having been withheld.
func TestAuditClaimWarningIsAWarning(t *testing.T) {
	sources := []websearch.SearchResult{{Index: 1, URL: "https://www.anthropic.com/pricing"}}
	answer := "Claude costs $3 per million input tokens and scores 72%."
	audit := auditAnswerEvidence(websearch.SearchPlan{}, answer, sources, nil, auditNow)

	meta := map[string]interface{}{}
	audit.applyTo(meta)

	if meta[metaClaimWarning] != claimWarningUncited {
		t.Errorf("%s = %v, want %q", metaClaimWarning, meta[metaClaimWarning], claimWarningUncited)
	}
	// The answer itself is untouched: nothing here rejects or rewrites it.
	if _, present := meta["content"]; present {
		t.Error("the audit must not attempt to modify the answer")
	}
}

func TestAuditNoWarningWhenCited(t *testing.T) {
	sources := []websearch.SearchResult{{Index: 1, URL: "https://www.anthropic.com/pricing"}}
	audit := auditAnswerEvidence(websearch.SearchPlan{}, "Claude costs $3 per million input tokens [1].", sources, nil, auditNow)

	meta := map[string]interface{}{}
	audit.applyTo(meta)
	if _, present := meta[metaClaimWarning]; present {
		t.Error("a cited claim must not be flagged")
	}
}

func TestAuditNilMetaSafe(t *testing.T) {
	audit := auditAnswerEvidence(websearch.SearchPlan{}, "x", nil, nil, auditNow)
	audit.applyTo(nil) // must not panic
}

func TestOrchestratorTimeRange(t *testing.T) {
	if got := orchestratorTimeRange(nil); got != "" {
		t.Errorf("nil result = %q", got)
	}
	if got := orchestratorTimeRange(&websearch.OrchestratorResult{}); got != "" {
		t.Errorf("missing tool call = %q", got)
	}
	result := &websearch.OrchestratorResult{
		ToolCall: &websearch.ToolCall{Arguments: websearch.SearchRequest{TimeRange: "7d"}},
	}
	if got := orchestratorTimeRange(result); got != "7d" {
		t.Errorf("orchestratorTimeRange = %q, want 7d", got)
	}
}
