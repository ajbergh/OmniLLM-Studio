package api

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// TestLiveConfigRoutingForGeminiResearch reproduces the reported failure against
// the observed live configuration: provider type "gemini", model
// "gemini-3.7-flash", no MCP servers, no Brave key.
//
// Before the fix, a research question routed to a preflight, which can only use
// the local provider — DuckDuckGo — which rate-limited mid-turn and produced an
// answer recommending models from over a year earlier.
func TestLiveConfigRoutingForGeminiResearch(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	// Matches the live profile.
	if !websearch.SupportsNativeSearch("gemini", "gemini-3.7-flash") {
		t.Fatal("gemini-3.7-flash should be native-capable")
	}
	live := turnOwnershipInputs{
		NativeGrounding:       true,  // gemini + gemini-3.7-flash
		IntegrationsConnected: false, // zero mcp_ tools registered
	}

	cases := []struct {
		prompt       string
		wantOwn      bool
		wantGrounded bool
		why          string
	}{
		{
			prompt:       "which llm has the best coding benchmark scores and pricing right now",
			wantOwn:      false,
			wantGrounded: true,
			why:          "grounding now rides along with the tool catalog, so this gets Gemini's index AND its tools — not DuckDuckGo",
		},
		{
			prompt:       "what is the current Anthropic API pricing",
			wantOwn:      false,
			wantGrounded: true,
			why:          "a pricing lookup should be grounded by the provider, never routed to a rate-limited scraper",
		},
		{
			prompt:       "what are the latest open PRs on my repo",
			wantOwn:      false,
			wantGrounded: false,
			why:          "a private source needs the tool loop; the public web cannot answer it, so grounding is not the point",
		},
		{
			prompt:       "find the latest exchange rate and calculate my total",
			wantOwn:      false,
			wantGrounded: true,
			why:          "grounding supplies the rate while the tool loop stays available for the calculation",
		},
		{
			prompt:       "what time does the world cup game start today",
			wantOwn:      true,
			wantGrounded: true,
			why:          "a direct single-fact lookup keeps the constrained prompt and clock-time validation",
		},
	}

	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			plan := websearch.BuildSearchPlan(tc.prompt, now, "UTC")
			if !plan.NeedsWeb {
				t.Fatalf("expected retrieval to be planned (shape would be %q)", plan.AnswerShape)
			}
			ownsTurn := retrievalMayOwnTurn(plan, tc.prompt, live)
			if ownsTurn != tc.wantOwn {
				t.Errorf("ownsTurn = %v, want %v (shape=%q): %s", ownsTurn, tc.wantOwn, plan.AnswerShape, tc.why)
			}

			// The property that actually matters: this turn must never fall back
			// to the local provider on a model that can ground itself.
			usesLocalPreflight := !ownsTurn && !live.NativeGrounding
			if usesLocalPreflight {
				t.Errorf("turn would run a local preflight on a native-capable model: %s", tc.why)
			}
			if tc.wantGrounded && !live.NativeGrounding {
				t.Error("test setup error: expected a native-capable provider")
			}
		})
	}
}
