package api

import (
	"context"
	"log"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"

	intentrouter "github.com/ajbergh/omnillm-studio/internal/router"
	"github.com/ajbergh/omnillm-studio/internal/turncontext"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// searchRouteDecision records how a turn was classified as needing (or not
// needing) current-information retrieval.
type searchRouteDecision struct {
	// NeedsWeb is the final answer.
	NeedsWeb bool
	// Source names what decided: "deterministic", "router", or "none".
	Source string
	// RewrittenQuery is the router's normalized query, when it supplied one.
	RewrittenQuery string
	// Telemetry is attached to message metadata when the router was consulted.
	Telemetry *intentrouter.RouterTelemetry
}

const (
	searchRouteSourceDeterministic = "deterministic"
	searchRouteSourceRouter        = "router"
	searchRouteSourceNone          = "none"
)

// classifyCurrentInformation decides whether a turn needs live retrieval.
//
// Two classifiers, cheapest first:
//
//  1. The deterministic gate. It costs nothing and is authoritative when it
//     fires — a strong recency signal is not something a probabilistic router
//     should be able to veto. This mirrors the existing sports precedent, where a
//     router decision of normal_llm is explicitly not allowed to suppress the
//     deterministic detector.
//  2. The semantic router, consulted only when the gate declined. This is where
//     regex-resistant phrasing gets caught ("how do the available models
//     compare?"). It costs an extra LLM call, so it runs only under the
//     tools_only and all_preflight router modes — modes that were declared in the
//     router package but, until now, unreachable.
func (h *MessageHandler) classifyCurrentInformation(
	ctx context.Context,
	userText string,
) searchRouteDecision {
	tc := turncontext.FromContext(ctx)
	if plan := websearch.BuildSearchPlan(userText, tc.Now, tc.Timezone); plan.NeedsWeb {
		return searchRouteDecision{NeedsWeb: true, Source: searchRouteSourceDeterministic}
	}

	if h.routerSvc == nil || !h.routerSvc.Enabled(ctx) {
		return searchRouteDecision{Source: searchRouteSourceNone}
	}
	mode := h.routerWebSearchMode()
	if mode == "" {
		return searchRouteDecision{Source: searchRouteSourceNone}
	}

	resp, err := h.routerSvc.Route(ctx, intentrouter.RouteRequest{
		UserMessage: userText,
		Mode:        mode,
		AvailableRoutes: []intentrouter.RouteName{
			intentrouter.RouteWebSearch,
			intentrouter.RouteNormalLLM,
		},
	})
	if err != nil {
		log.Printf("WARN: [router] web_search route failed: %v", err)
		return searchRouteDecision{Source: searchRouteSourceNone}
	}
	if resp == nil {
		return searchRouteDecision{Source: searchRouteSourceNone}
	}
	telemetry := resp.Telemetry
	if !resp.Valid {
		return searchRouteDecision{Source: searchRouteSourceNone, Telemetry: &telemetry}
	}
	if resp.Decision.Route != intentrouter.RouteWebSearch {
		return searchRouteDecision{Source: searchRouteSourceNone, Telemetry: &telemetry}
	}
	return searchRouteDecision{
		NeedsWeb:       true,
		Source:         searchRouteSourceRouter,
		RewrittenQuery: strings.TrimSpace(resp.Decision.RewrittenQuery),
		Telemetry:      &telemetry,
	}
}

// routerWebSearchMode returns the router mode to use for current-information
// classification, or "" when the configured mode does not cover it.
//
// sports_only deliberately returns "": adding an LLM call to every non-sports
// turn is a real latency and cost change, so it stays opt-in.
func (h *MessageHandler) routerWebSearchMode() intentrouter.RouterMode {
	if h.settingsRepo == nil {
		return ""
	}
	settings, err := h.settingsRepo.GetTyped()
	if err != nil {
		return ""
	}
	switch intentrouter.RouterMode(settings.RouterMode) {
	case intentrouter.RouterModeToolsOnly:
		return intentrouter.RouterModeToolsOnly
	case intentrouter.RouterModeAllPreflight:
		return intentrouter.RouterModeAllPreflight
	default:
		return ""
	}
}

// forcesRetrieval reports whether the orchestrator must skip its own gate check.
//
// The orchestrator entry points re-run BuildSearchPlan, so a turn the
// deterministic gate declined would be vetoed there even though the semantic
// router accepted it — producing a turn that reported an attempted search and
// performed none. Only a router decision needs the override; a deterministic
// decision will pass the gate again by construction.
func (d searchRouteDecision) forcesRetrieval() bool {
	return d.NeedsWeb && d.Source == searchRouteSourceRouter
}

// retrievalQuery picks the text to search for. The router's rewritten query is
// preferred when it supplied one, since it normalizes conversational phrasing.
func (d searchRouteDecision) retrievalQuery(userText string) string {
	if d.RewrittenQuery != "" {
		return d.RewrittenQuery
	}
	return userText
}

// retrievalMayOwnTurn reports whether the search orchestrator may answer a turn
// by itself, instead of retrieving as a preflight and handing off to the tool
// loop.
//
// This exists because turn ownership and tool calling are mutually exclusive:
// the orchestrator paths (Process / ProcessStream) generate an answer and
// return, so the tool loop never runs and no MCP, plugin, or app tool can be
// invoked. That was survivable while the gate was narrow. Once it correctly
// started triggering on "latest", "current <noun>", "search for", and "find", it
// began capturing the exact phrasings people use to ask for tool-backed data —
// "the latest open PRs", "search my Notion", "current sprint status" — and
// silently answered them from the public web instead.
//
// The rule: retrieval may own a turn only when there is plausibly nothing else
// to run. That is true for single-fact lookups, which is also where owning the
// turn pays for itself, because a native-grounding provider answers them in one
// call instead of a search plus a summarization.
//
// Everything else retrieves as a preflight. The model still gets fresh evidence
// in context; it also keeps its tools.
func retrievalMayOwnTurn(plan websearch.SearchPlan, prompt string, integrationsConnected bool) bool {
	if !plan.NeedsWeb {
		return false
	}
	// An explicit follow-up action always needs the tool loop.
	if requiresPostRetrievalTools(prompt) {
		return false
	}
	// A prompt that names a private or account-scoped source is asking for a
	// tool, not the public web, even when it also reads as current-information.
	if referencesPrivateSource(prompt) {
		return false
	}
	// Single-fact shapes only. Standard and Research answers are the ones most
	// likely to benefit from tools alongside evidence.
	switch plan.AnswerShape {
	case websearch.AnswerShapeDirect:
		// A direct lookup is a single fact with a strongly-typed shape (a
		// kickoff time, a score). Safe to own even with integrations connected.
		return true
	case websearch.AnswerShapeBrief:
		// Brief covers scores, weather, and market data — but also anything the
		// planner could not classify more precisely. When the user has connected
		// integrations, the phrase-matching above is not a good enough filter:
		// "what did Alice say about the launch" names no source and no action,
		// yet clearly wants a tool. Prefer the preflight and let the model choose.
		return !integrationsConnected
	default:
		return false
	}
}

// integrationToolsConnected reports whether this turn's catalog contains a tool
// from an actually-connected integration.
//
// MCP tools are named "mcp_<server>_<tool>" by mcpclient.BuildToolName, so the
// prefix only appears once a server is configured and its tools are discovered.
// Plugin tools follow the same convention.
//
// The app_* tools are deliberately NOT counted: app_catalog, app_connections,
// app_connect_mcp, and app_disconnect are connection *management* tools
// registered unconditionally at startup. Treating them as evidence of an
// integration would make this always true and disable the cheap turn-owning path
// for everyone.
//
// This is a deliberate thumb on the scale. Someone who has connected an
// integration expects it to be used, and the cost of wrongly starving it — the
// feature looks broken — is far higher than the cost of wrongly running a
// preflight, which is one extra search call.
func integrationToolsConnected(catalog []llm.Tool) bool {
	for _, tool := range catalog {
		name := tool.Function.Name
		if strings.HasPrefix(name, "mcp_") || strings.HasPrefix(name, "plugin_") {
			return true
		}
	}
	return false
}

// referencesPrivateSource reports whether a prompt points at data the public web
// cannot answer for: the user's own accounts, repositories, documents, or
// systems. These are tool territory regardless of how current the question is.
//
// Possessive framing ("my", "our") is the strongest signal, and named
// integrations are the second. The list is deliberately about *sources* rather
// than actions, so it does not overlap requiresPostRetrievalTools.
func referencesPrivateSource(prompt string) bool {
	if containsAny(prompt,
		" my ", " our ", "my repo", "my prs", "my pull requests",
		"my calendar", "my inbox", "my email", "my tickets", "my issues",
		"my workspace", "my project", "my deployment", "my notes",
	) {
		return true
	}
	return containsAny(prompt,
		"notion", "jira", "confluence", "linear", "asana", "trello",
		"slack", "salesforce", "hubspot", "zendesk", "servicenow",
		"github", "gitlab", "bitbucket", "sharepoint", "onedrive",
		"google drive", "gdrive", "dropbox", "airtable", "snowflake",
		"bigquery", "databricks", "grafana", "datadog", "pagerduty",
		"crm", "mcp server",
	)
}
