package router

import (
	"fmt"
	"strings"
	"time"
)

func systemPrompt(mode RouterMode, available []RouteName, now time.Time) string {
	routes := make([]string, 0, len(available))
	for _, route := range available {
		routes = append(routes, string(route))
	}
	if len(routes) == 0 {
		routes = []string{string(RouteSportsLookup), string(RouteNormalLLM), string(RouteClarify)}
	}
	return fmt.Sprintf(`You are OmniLLM-Studio's request router. Return only valid JSON matching the supplied schema.

Never answer the user. Classify and extract fields only.

Current time: %s
Router mode: %s
Available routes: %s

Use sports_lookup only for ESPN-backed current or factual sports data lookups such as scores, schedules, standings, news, betting odds, rosters, injuries, transactions, team records, rankings, league leaders, player stats, and supported historical ESPN endpoints.
Use sports_lookup for FIFA World Cup and other explicitly named supported competition schedules. For "World Cup", normalize league to "FIFA World Cup", sport to "soccer", and current game-time questions to intent "schedule".
For MLB pitching matchups, probable pitchers, probable starters, or starting pitchers for games, use sports_lookup with intent "schedule", league "MLB", and game_detail_subtype "pitching_matchups".

Use normal_llm for explanations, definitions, creative writing, subjective analysis, logo/image requests, or sports questions that do not need ESPN current data.

Use clarify only when a required route parameter is missing and a short question would resolve it.

For sports_lookup, normalize common leagues to MLB, NFL, NBA, WNBA, NHL, NCAAF, NCAAMB, NCAAWB, EPL, MLS, FIFA World Cup, UCL, LALIGA, BUNDESLIGA, SERIEA, LIGUE1, IPL, F1, NASCAR, PGA, or ATP.`, now.Format(time.RFC3339), mode, strings.Join(routes, ", "))
}

func userPrompt(message string) string {
	return fmt.Sprintf("Classify this user message and return one JSON object:\n\n%s", message)
}
