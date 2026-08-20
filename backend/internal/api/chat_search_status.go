package api

import (
	"strings"
)

// Retrieval-status metadata keys. These form the contract the frontend reads to
// distinguish "retrieval attempted", "retrieval succeeded", "sources available",
// and "freshness verified" — states that were previously indistinguishable
// because a failed search fell through to an ordinary-looking answer.
const (
	metaSearchAttempted = "search_attempted"
	metaSearchFailed    = "search_failed"
	metaSearchReason    = "search_failure_reason"
	metaWebSearch       = "web_search"
	metaSources         = "sources"
)

// Failure reason codes. These are deliberately coarse: raw provider errors must
// never reach a client, so the handler maps them to a fixed vocabulary.
const (
	searchFailureNoResults = "no_results"
	searchFailureProvider  = "provider_error"
	searchFailureDisabled  = "provider_disabled"
)

// searchStatus records what actually happened during current-information
// retrieval for one turn.
type searchStatus struct {
	Attempted bool
	Failed    bool
	Reason    string
}

// classifySearchFailure maps a retrieval error onto the client-safe vocabulary.
// The original error is expected to have been logged at ERROR by the layer that
// produced it; only the code travels outward.
func classifySearchFailure(err error) string {
	if err == nil {
		return searchFailureNoResults
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "disabled"):
		return searchFailureDisabled
	case strings.Contains(msg, "no results"), strings.Contains(msg, "no parsable results"):
		return searchFailureNoResults
	default:
		return searchFailureProvider
	}
}

// applyTo writes the retrieval status onto an outgoing metadata map.
func (s searchStatus) applyTo(meta map[string]interface{}) {
	if meta == nil || !s.Attempted {
		return
	}
	meta[metaSearchAttempted] = true
	if s.Failed {
		meta[metaSearchFailed] = true
		if s.Reason != "" {
			meta[metaSearchReason] = s.Reason
		}
	}
}

// degradedAnswerDirective replaces the previous soft note. The old wording asked
// the model to "mention that the information may not be current", which models
// routinely ignored while still producing confident current-sounding claims.
// This version forbids the specific claim types that go stale, and the frontend
// additionally renders an unverified banner from the metadata above so the
// warning does not depend on model compliance.
const degradedAnswerDirective = `RETRIEVAL FAILED: A current-information lookup was attempted for this turn and returned no usable evidence. You have no live data.

Rules for this answer:
1. Open with one sentence stating that you could not verify current information.
2. Do not state prices, version numbers, benchmark scores, rankings, release dates, or availability as current fact.
3. If you give background from training data, label it as potentially outdated and say roughly when it was true.
4. Do not invent or guess citations, URLs, or retrieval timestamps.
5. Offer what would confirm the answer (an official pricing page, a release-notes page) instead of guessing.`
