package api

import (
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// Evidence-contract metadata keys.
const (
	metaFreshnessVerified = "freshness_verified"
	metaAnswerFreshness   = "answer_freshness"
	metaCitationCount     = "citation_count"
	metaClaimWarning      = "claim_warning"
	metaNativeCitations   = "native_citations"
)

// claimWarningUncited is the only claim-support warning currently emitted. It
// covers the high-confidence case — the answer states prices, percentages, or
// versions and names no source at all — rather than attempting to judge whether
// a named source actually supports a figure.
const claimWarningUncited = "numeric_claims_without_citation"

// evidenceAudit is the post-generation half of the evidence contract.
//
// Non-streaming turns ran ValidateAnswer; streaming turns ran nothing at all,
// because ProcessStream returns a request and the handler simply streams it. And
// ValidateAnswer itself waves through every non-Direct answer shape, so in
// practice validation was sports-only.
//
// This audit runs on both paths after the answer exists. It cannot reject a
// streamed answer — the tokens are already sent — so its output is metadata the
// UI renders, which is the same honest constraint the tool-enforcement check
// works under.
type evidenceAudit struct {
	// Sources is the local evidence the answer was grounded in.
	Sources []websearch.SearchResult
	// NativeCitations are provider-native grounding sources, which for native
	// grounding are the *only* sources: the local result set is empty there.
	NativeCitations []llm.Citation
	// Freshness measures the evidence against the plan's window.
	Freshness websearch.FreshnessReport
	// Claims records the claim-support signal.
	Claims websearch.AnswerAudit
}

// auditAnswerEvidence measures a finished answer against its evidence.
func auditAnswerEvidence(
	plan websearch.SearchPlan,
	content string,
	sources []websearch.SearchResult,
	native []llm.Citation,
	now time.Time,
) evidenceAudit {
	audit := evidenceAudit{
		Sources:         sources,
		NativeCitations: llm.NormalizeCitations(native),
	}
	// Native grounding returns no local results, so freshness is measured against
	// whichever set actually exists.
	freshnessInput := sources
	if len(freshnessInput) == 0 {
		freshnessInput = citationsAsResults(audit.NativeCitations)
	}
	audit.Freshness = websearch.EvaluateFreshness(plan, freshnessInput, now)
	audit.Claims = websearch.AuditAnswer(content, freshnessInput)
	return audit
}

// citationsAsResults adapts native citations to the result shape the freshness
// and claim helpers accept. Native citations carry no publication date, so these
// count as undated — which is the correct signal, not a defect.
func citationsAsResults(citations []llm.Citation) []websearch.SearchResult {
	if len(citations) == 0 {
		return nil
	}
	results := make([]websearch.SearchResult, 0, len(citations))
	for i, citation := range citations {
		results = append(results, websearch.SearchResult{
			Index: i + 1,
			Title: citation.Title,
			URL:   citation.URL,
		})
	}
	return results
}

// applyTo records the audit on outgoing metadata.
func (a evidenceAudit) applyTo(meta map[string]interface{}) {
	if meta == nil {
		return
	}
	if len(a.NativeCitations) > 0 {
		meta[metaNativeCitations] = a.NativeCitations
		// metadata.sources drives the source panel. Native grounding leaves the
		// local result set empty, so without this the panel was blank for every
		// natively-grounded answer while the answer text claimed to cite sources.
		if existing, ok := meta[metaSources].([]websearch.SearchResult); !ok || len(existing) == 0 {
			meta[metaSources] = citationsAsResults(a.NativeCitations)
		}
	}
	if a.Freshness.Verified {
		meta[metaFreshnessVerified] = true
	}
	if a.Freshness.NewestLabel != "" {
		meta[metaAnswerFreshness] = a.Freshness.NewestLabel
	}
	if count := a.citationCount(); count > 0 {
		meta[metaCitationCount] = count
	}
	if a.Claims.UncitedNumericClaims {
		// A warning, not a rejection. Until the false-positive rate is measured
		// against an evaluation set, this must not gate an answer.
		meta[metaClaimWarning] = claimWarningUncited
	}
}

// citationCount is the number of distinct citations the answer is backed by,
// counting inline markers and named hosts, plus provider-native sources.
func (a evidenceAudit) citationCount() int {
	count := a.Claims.CitationMarkers
	if a.Claims.CitedSourceHosts > count {
		count = a.Claims.CitedSourceHosts
	}
	if len(a.NativeCitations) > count {
		count = len(a.NativeCitations)
	}
	return count
}

// orchestratorTimeRange recovers the freshness window an orchestrator-owned turn
// used. The result carries its tool call rather than the plan, so the window is
// read back from the arguments the plan produced.
func orchestratorTimeRange(result *websearch.OrchestratorResult) string {
	if result == nil || result.ToolCall == nil {
		return ""
	}
	return result.ToolCall.Arguments.TimeRange
}
