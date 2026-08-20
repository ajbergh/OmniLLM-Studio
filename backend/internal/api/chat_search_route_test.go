package api

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// TestCompoundRequestsRetrieveAndThenAct is the Phase 3 exit criterion at the
// decision layer.
//
// For each compound prompt two things must hold together, which is exactly what
// the old design could not do:
//
//  1. the planner wants retrieval (NeedsWeb), and
//  2. the turn is classified as needing tools afterwards, so retrieval runs as a
//     preflight rather than answering and terminating the turn.
//
// Previously (2) meant "skip retrieval entirely", so (1) was discarded and
// web_search became an optional courtesy the model could decline.
func TestCompoundRequestsRetrieveAndThenAct(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		prompt string
	}{
		{"search and calculate", "Find the latest API prices and calculate the monthly total"},
		{"search and export", "Search current AI news and export the summary"},
		{"research and compare", "Research the latest available models and compare benchmark versus cost"},
		{"search and table", "Look up the latest benchmark scores and create a table"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := websearch.BuildSearchPlan(tc.prompt, now, "UTC")
			if !plan.NeedsWeb {
				t.Fatalf("BuildSearchPlan(%q).NeedsWeb = false; retrieval must be planned", tc.prompt)
			}
			if !requiresPostRetrievalTools(tc.prompt) {
				t.Fatalf("requiresPostRetrievalTools(%q) = false; the follow-up action must be recognised so retrieval runs as a preflight", tc.prompt)
			}
		})
	}
}

// TestSimpleLookupsKeepTheCheapPath guards the cost model: a plain lookup must
// not pay for a preflight plus a separate generation call when native grounding
// can do both at once.
func TestSimpleLookupsKeepTheCheapPath(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	cases := []string{
		"What is the latest exchange rate?",
		"What's the weather today?",
		"Who won the game last night?",
	}
	for _, prompt := range cases {
		if !websearch.BuildSearchPlan(prompt, now, "UTC").NeedsWeb {
			t.Errorf("BuildSearchPlan(%q).NeedsWeb = false", prompt)
		}
		if requiresPostRetrievalTools(prompt) {
			t.Errorf("requiresPostRetrievalTools(%q) = true; a plain lookup needs no follow-up tool", prompt)
		}
	}
}

func TestSearchRouteDecisionRetrievalQuery(t *testing.T) {
	original := "so like what's the deal with model prices these days"

	plain := searchRouteDecision{NeedsWeb: true, Source: searchRouteSourceDeterministic}
	if got := plain.retrievalQuery(original); got != original {
		t.Errorf("with no rewrite the original text is searched, got %q", got)
	}

	rewritten := searchRouteDecision{
		NeedsWeb:       true,
		Source:         searchRouteSourceRouter,
		RewrittenQuery: "current LLM API pricing",
	}
	if got := rewritten.retrievalQuery(original); got != "current LLM API pricing" {
		t.Errorf("the router's normalized query must be preferred, got %q", got)
	}
}

// TestStreamingPreflightWiring asserts the composition the unit tests above
// cannot reach. There is no HTTP harness for MessageHandler in this package, so
// this follows the existing source-inspection convention from
// message_handler_tool_runtime_wiring_test.go.
func TestStreamingPreflightWiring(t *testing.T) {
	text := readMessageHandlerSource(t)

	required := map[string]string{
		"h.classifyCurrentInformation(r.Context(), req.Content)": "the turn must be classified before retrieval is chosen",
		"h.orchestrator.Preflight(r.Context(), retrievalText)":   "compound turns must retrieve through the preflight",
		"preflight.EvidenceSystemMessage(":                       "preflight evidence must be injected into the generation request",
		"requiresPostRetrievalTools(req.Content)":                "the compound test selects preflight-vs-orchestrator",
	}
	for marker, why := range required {
		if !strings.Contains(text, marker) {
			t.Errorf("missing %q: %s", marker, why)
		}
	}

	// The old meaning of this function was "skip retrieval". Nothing should
	// reintroduce a path where a compound request bypasses retrieval entirely.
	if strings.Contains(text, "requiresComposableToolLoop") {
		t.Error("requiresComposableToolLoop was renamed; the old name implied skipping retrieval")
	}
	if strings.Contains(text, "webSearchEnabled && !requiresPostRetrievalTools(req.Content)") {
		t.Error("compound requests must no longer bypass retrieval")
	}
}

// TestSyncToolLoopWiring covers the other half of the asymmetry: non-streaming
// chat had no tool loop at all, so llmReq.Tools was never set and ChatComplete
// could not produce a tool call.
func TestSyncToolLoopWiring(t *testing.T) {
	text := readMessageHandlerSource(t)

	required := map[string]string{
		"h.runSyncToolLoop(":                   "the non-streaming path must run tool rounds",
		"syncPreflight.EvidenceSystemMessage(": "non-streaming compound turns must inject preflight evidence",
	}
	for marker, why := range required {
		if !strings.Contains(text, marker) {
			t.Errorf("missing %q: %s", marker, why)
		}
	}
	if !strings.Contains(text, "llmReq.Tools = selectChatToolsForContext(r.Context(), h.toolRegistry, h.toolExecutor, req.Content, syncToolSelection)") {
		t.Error("the non-streaming request must advertise a tool catalog")
	}
	// Both paths must share one classifier, or they will disagree about whether a
	// turn needs current information.
	if strings.Count(text, "h.classifyCurrentInformation(r.Context(), req.Content)") < 2 {
		t.Error("streaming and non-streaming must both use classifyCurrentInformation")
	}
}

// TestToolChoiceWiring asserts provider-level enforcement is actually sent,
// rather than the requirement living only in the system prompt.
func TestToolChoiceWiring(t *testing.T) {
	text := readMessageHandlerSource(t)
	if !strings.Contains(text, "llmReq.ToolChoice = streamToolEnforcement.toolChoiceForRound(") {
		t.Error("the streaming loop must send a provider-level tool_choice")
	}
	if !strings.Contains(text, "streamToolEnforcement.observe(finalToolCalls)") {
		t.Error("the streaming loop must verify which tools actually ran")
	}
}

func readMessageHandlerSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile("message_handler.go")
	if err != nil {
		t.Fatalf("read message_handler.go: %v", err)
	}
	return string(source)
}
