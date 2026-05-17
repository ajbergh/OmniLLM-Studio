package router

type RouteName string

const (
	RouteNone               RouteName = "none"
	RouteNormalLLM          RouteName = "normal_llm"
	RouteClarify            RouteName = "clarify"
	RouteSportsLookup       RouteName = "sports_lookup"
	RouteFileSearch         RouteName = "file_search"
	RouteURLContext         RouteName = "url_context"
	RouteWebSearch          RouteName = "web_search"
	RouteBrowser            RouteName = "browser"
	RouteRAG                RouteName = "rag"
	RouteImageGeneration    RouteName = "image_generation"
	RouteMusicGeneration    RouteName = "music_generation"
	RouteArtifactGeneration RouteName = "artifact_generation"
)

type RouterMode string

const (
	RouterModeOff          RouterMode = "off"
	RouterModeSportsOnly   RouterMode = "sports_only"
	RouterModeToolsOnly    RouterMode = "tools_only"
	RouterModeAllPreflight RouterMode = "all_preflight"
)

const (
	FallbackLocalDetector = "local_detector"
	FallbackMainModel     = "main_model"
	FallbackNormalLLM     = "normal_llm"
	FallbackClarify       = "clarify"
	FallbackFailClosed    = "fail_closed"
)

type RouterDecision struct {
	Route                 RouteName          `json:"route"`
	Confidence            float64            `json:"confidence"`
	RequiresGenerationLLM bool               `json:"requires_generation_llm"`
	RewrittenQuery        string             `json:"rewritten_query,omitempty"`
	ClarifyingQuestion    string             `json:"clarifying_question,omitempty"`
	Reason                string             `json:"reason,omitempty"`
	Sports                *SportsRouteParams `json:"sports,omitempty"`
}

type SportsRouteParams struct {
	Intent             string `json:"intent,omitempty"`
	League             string `json:"league,omitempty"`
	Sport              string `json:"sport,omitempty"`
	TeamQuery          string `json:"team_query,omitempty"`
	AthleteQuery       string `json:"athlete_query,omitempty"`
	SecondAthleteQuery string `json:"second_athlete_query,omitempty"`
	Metric             string `json:"metric,omitempty"`
	Date               string `json:"date,omitempty"`
	DateLabel          string `json:"date_label,omitempty"`
	Season             *int   `json:"season,omitempty"`
	Limit              *int   `json:"limit,omitempty"`
	GameDetailSubtype  string `json:"game_detail_subtype,omitempty"`
}

type RouterTelemetry struct {
	Enabled              bool       `json:"enabled"`
	Mode                 RouterMode `json:"mode"`
	Provider             string     `json:"provider,omitempty"`
	Model                string     `json:"model,omitempty"`
	LatencyMS            int        `json:"latency_ms,omitempty"`
	Confidence           float64    `json:"confidence,omitempty"`
	Route                RouteName  `json:"route,omitempty"`
	Validated            bool       `json:"validated"`
	FallbackUsed         bool       `json:"fallback_used"`
	FallbackReason       string     `json:"fallback_reason,omitempty"`
	StructuredOutputMode string     `json:"structured_output_mode,omitempty"`
	Error                string     `json:"error,omitempty"`
}

type RouteRequest struct {
	UserMessage     string
	ConversationID  string
	UserID          string
	Mode            RouterMode
	AvailableRoutes []RouteName
}

type RouteResponse struct {
	Decision       RouterDecision
	Telemetry      RouterTelemetry
	Valid          bool
	FallbackReason string
}

type ModelSuggestion struct {
	Model            string `json:"model"`
	Label            string `json:"label"`
	Reason           string `json:"reason"`
	StructuredOutput string `json:"structured_output"`
	CostTier         string `json:"cost_tier"`
	Confidence       string `json:"confidence"`
}

type SuggestionsResponse struct {
	Provider     string            `json:"provider"`
	ProviderType string            `json:"provider_type"`
	Suggestions  []ModelSuggestion `json:"suggestions"`
	Notes        []string          `json:"notes"`
}
