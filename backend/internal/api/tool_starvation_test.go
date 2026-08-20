package api

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// TestRetrievalDoesNotStarveToolCalling is the regression test for a reported
// production break: MCP tools stopped being called at all.
//
// The mechanism is a turn-ownership conflict. When the planner wants retrieval
// and the prompt has no follow-up-action keyword, the orchestrator answers the
// turn and returns — the tool loop never runs, so no MCP, plugin, or app tool can
// be invoked no matter how obviously the prompt needs one.
//
// That path always existed, but the Phase 1 gate widening made vastly more
// prompts reach it: "latest", "current <noun>", "search for", and "find" are all
// triggers now, and those words are exactly how people ask for tool-backed data
// ("the latest open PRs", "search my Notion", "current sprint status").
//
// Each prompt below names a data source the web cannot answer for. Retrieval may
// legitimately want to run, but it must not take the whole turn.
func TestRetrievalDoesNotStarveToolCalling(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	prompts := []string{
		"search my Notion for the Q3 roadmap",
		"what are the latest open PRs on my repo",
		"find the current sprint status in Jira",
		"look up customer ACME in the CRM",
		"what are the latest rows in my analytics table",
		"check the current status of my deployment",
	}

	for _, prompt := range prompts {
		t.Run(prompt, func(t *testing.T) {
			plan := websearch.BuildSearchPlan(prompt, now, "UTC")
			if retrievalMayOwnTurn(plan, prompt, false) {
				t.Errorf("retrieval would own this turn (NeedsWeb=%v, shape=%q), so the tool loop never runs and MCP tools cannot be called",
					plan.NeedsWeb, plan.AnswerShape)
			}
		})
	}
}

// TestSimpleLookupsMayStillOwnTheTurn is the counterweight: the cheap path must
// survive. A single-fact lookup has no plausible tool to starve, and letting the
// orchestrator own it is what keeps native grounding (one provider call instead
// of a search plus a summarization).
func TestSimpleLookupsMayStillOwnTheTurn(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	for _, prompt := range []string{
		"What time does the World Cup game start today?",
		"What's the weather today?",
		"Who won the game last night?",
	} {
		plan := websearch.BuildSearchPlan(prompt, now, "UTC")
		if !plan.NeedsWeb {
			t.Fatalf("%q should still plan retrieval", prompt)
		}
		if !retrievalMayOwnTurn(plan, prompt, false) {
			t.Errorf("%q is a single-fact lookup and should keep the cheap turn-owning path", prompt)
		}
	}
}

// TestConnectedIntegrationsNarrowTurnOwnership covers the case keyword matching
// cannot: a prompt that names no source and no action but clearly wants a tool.
// With integrations connected, only the strongly-typed Direct shape keeps the
// turn-owning path.
func TestConnectedIntegrationsNarrowTurnOwnership(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	// Brief shape: safe to own with no integrations, handed to the tool loop when
	// integrations exist.
	brief := websearch.BuildSearchPlan("What's the weather today?", now, "UTC")
	if brief.AnswerShape != websearch.AnswerShapeBrief {
		t.Fatalf("expected a brief plan, got %q", brief.AnswerShape)
	}
	if !retrievalMayOwnTurn(brief, "What's the weather today?", false) {
		t.Error("with no integrations a brief lookup should keep the cheap path")
	}
	if retrievalMayOwnTurn(brief, "What's the weather today?", true) {
		t.Error("with integrations connected a brief lookup must not take the whole turn")
	}

	// Direct shape stays owned either way: a kickoff time has no plausible tool.
	direct := websearch.BuildSearchPlan("What time does the World Cup game start today?", now, "UTC")
	if direct.AnswerShape != websearch.AnswerShapeDirect {
		t.Fatalf("expected a direct plan, got %q", direct.AnswerShape)
	}
	if !retrievalMayOwnTurn(direct, "What time does the World Cup game start today?", true) {
		t.Error("a direct single-fact lookup should keep the cheap path even with integrations")
	}
}

func TestIntegrationToolsConnected(t *testing.T) {
	named := func(names ...string) []llm.Tool {
		out := make([]llm.Tool, 0, len(names))
		for _, name := range names {
			var tool llm.Tool
			tool.Function.Name = name
			out = append(out, tool)
		}
		return out
	}

	if integrationToolsConnected(named("web_search", "calculator", "date_time")) {
		t.Error("built-in tools are not connected integrations")
	}
	if integrationToolsConnected(nil) {
		t.Error("an empty catalog has no integrations")
	}
	if !integrationToolsConnected(named("web_search", "mcp_jira_search_issues")) {
		t.Error("an MCP tool must count as a connected integration")
	}
	if !integrationToolsConnected(named("plugin_do_thing")) {
		t.Error("a plugin tool must count")
	}
	// The app_* tools are connection management, registered unconditionally at
	// startup. Counting them would make this always true and disable the cheap
	// path for everyone.
	if integrationToolsConnected(named("app_catalog", "app_connections", "app_connect_mcp", "app_disconnect")) {
		t.Error("connection-management tools are not evidence of a connected integration")
	}
}
