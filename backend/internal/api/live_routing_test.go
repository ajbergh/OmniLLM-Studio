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
		prompt  string
		wantOwn bool
		why     string
	}{
		{
			prompt:  "which llm has the best coding benchmark scores and pricing right now",
			wantOwn: true,
			why:     "a public-web research question must use Gemini's own grounding, not a rate-limited scraper",
		},
		{
			prompt:  "what is the current Anthropic API pricing",
			wantOwn: true,
			why:     "pricing lookups are public-web and native grounding is the better source",
		},
		{
			prompt:  "what are the latest open PRs on my repo",
			wantOwn: false,
			why:     "a private source needs the tool loop regardless of provider capability",
		},
		{
			prompt:  "find the latest exchange rate and calculate my total",
			wantOwn: false,
			why:     "a follow-up action needs the tool loop",
		},
	}

	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			plan := websearch.BuildSearchPlan(tc.prompt, now, "UTC")
			if !plan.NeedsWeb {
				t.Fatalf("expected retrieval to be planned (shape would be %q)", plan.AnswerShape)
			}
			got := retrievalMayOwnTurn(plan, tc.prompt, live)
			if got != tc.wantOwn {
				t.Errorf("ownsTurn = %v, want %v (shape=%q): %s", got, tc.wantOwn, plan.AnswerShape, tc.why)
			}
		})
	}
}
