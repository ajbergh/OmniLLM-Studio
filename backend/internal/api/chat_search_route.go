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

// turnOwnershipInputs are the facts, beyond the plan and the prompt, that decide
// whether retrieval may answer a turn by itself.
type turnOwnershipInputs struct {
	// NativeGrounding is true when the active provider and model can ground
	// their own answer.
	NativeGrounding bool
	// IntegrationsConnected is true when the turn's catalog contains a tool from
	// a connected MCP server or plugin.
	IntegrationsConnected bool
}

// retrievalMayOwnTurn reports whether the search orchestrator may answer a turn
// by itself, instead of retrieving as a preflight and handing off to the tool
// loop.
//
// Turn ownership and tool calling are mutually exclusive: the orchestrator paths
// generate an answer and return, so the tool loop never runs and no MCP, plugin,
// or app tool can be invoked. But the reverse costs something real too — a
// preflight can only use the *local* search provider, because provider-native
// grounding is inseparable from generation. Routing a turn away from working
// native grounding and onto an unreliable local scraper makes every answer worse.
//
// So this is a two-sided decision, not a one-sided safety rule.
func retrievalMayOwnTurn(plan websearch.SearchPlan, prompt string, in turnOwnershipInputs) bool {
	if !plan.NeedsWeb {
		return false
	}

	// A prompt that must act on the retrieved data, or that names a private or
	// account-scoped source, always needs the tool loop. No provider capability
	// changes that.
	if requiresPostRetrievalTools(prompt) || referencesPrivateSource(prompt) {
		return false
	}

	// With connected integrations, stay conservative: only a strongly-typed
	// single-fact lookup may take the turn. Keyword matching cannot distinguish
	// "the latest from Alice on the launch" (wants Slack) from "the latest news"
	// (wants the web), and starving a connected integration reads as the feature
	// being broken.
	if in.IntegrationsConnected {
		return plan.AnswerShape == websearch.AnswerShapeDirect
	}

	// Native grounding no longer requires giving up the tool loop: the Gemini and
	// Anthropic adapters emit their search tool alongside the caller's function
	// declarations, so one request can ground itself *and* call tools. Owning the
	// turn is therefore only worth it for the Direct shape, where the constrained
	// summarizer prompt and the clock-time answer validation genuinely help and no
	// tool is plausible anyway.
	if in.NativeGrounding {
		return plan.AnswerShape == websearch.AnswerShapeDirect
	}

	// Local-only provider: owning the turn buys nothing over a preflight, since
	// both perform one local search. Keep the tool loop available so the model
	// can retry, browse, or calculate.
	switch plan.AnswerShape {
	case websearch.AnswerShapeDirect, websearch.AnswerShapeBrief:
		return true
	default:
		return false
	}
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
// integration would make this always true, which would silently route every
// research turn onto the local search provider even when the active model has
// working native grounding.
func integrationToolsConnected(catalog []llm.Tool) bool {
	for _, tool := range catalog {
		name := tool.Function.Name
		if strings.HasPrefix(name, "mcp_") || strings.HasPrefix(name, "plugin_") {
			return true
		}
	}
	return false
}
