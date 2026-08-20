package api

import (
	"context"
	"log"
	"strings"

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

// retrievalQuery picks the text to search for. The router's rewritten query is
// preferred when it supplied one, since it normalizes conversational phrasing.
func (d searchRouteDecision) retrievalQuery(userText string) string {
	if d.RewrittenQuery != "" {
		return d.RewrittenQuery
	}
	return userText
}
