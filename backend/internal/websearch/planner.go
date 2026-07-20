package websearch

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/turncontext"
)

type AnswerShape string

const (
	AnswerShapeDirect   AnswerShape = "direct"
	AnswerShapeBrief    AnswerShape = "brief"
	AnswerShapeStandard AnswerShape = "standard"
	AnswerShapeResearch AnswerShape = "research"
)

type SearchIntent string

const (
	SearchIntentGeneral  SearchIntent = "general"
	SearchIntentSchedule SearchIntent = "schedule"
	SearchIntentScore    SearchIntent = "score"
	SearchIntentNews     SearchIntent = "news"
	SearchIntentPrice    SearchIntent = "price"
	SearchIntentWeather  SearchIntent = "weather"
)

type SearchPlan struct {
	NeedsWeb          bool
	Intent            SearchIntent
	AnswerShape       AnswerShape
	Queries           []string
	TimeRange         string
	MaxResults        int
	MaxIterations     int
	SearchContextSize string
	AllowedDomains    []string
	RequiredFields    []string
	NativePreferred   bool
}

var (
	timeQuestionPattern = regexp.MustCompile(`(?i)\b(what time|when does|when is|start time|kickoff|tip[- ]?off|puck drop|first pitch)\b`)
	sportsEventPattern  = regexp.MustCompile(`(?i)\b(game|match|world cup|super bowl|final|play|playing|packers|nfl|nba|mlb|nhl|soccer|football)\b`)
	researchPattern     = regexp.MustCompile(`(?i)\b(deep research|comprehensive|detailed analysis|compare all|investigate|report on)\b`)
)

func BuildSearchPlan(userText string, now time.Time, timezone string) SearchPlan {
	triggered, toolCall := ShouldWebSearch(userText, now, timezone)
	if !triggered || toolCall == nil {
		return SearchPlan{}
	}

	lower := strings.ToLower(strings.TrimSpace(userText))
	plan := SearchPlan{
		NeedsWeb:          true,
		Intent:            SearchIntentGeneral,
		AnswerShape:       AnswerShapeStandard,
		Queries:           []string{toolCall.Arguments.Query},
		TimeRange:         toolCall.Arguments.TimeRange,
		MaxResults:        6,
		MaxIterations:     2,
		SearchContextSize: "medium",
		NativePreferred:   true,
	}

	switch {
	case timeQuestionPattern.MatchString(lower) && sportsEventPattern.MatchString(lower):
		plan.Intent = SearchIntentSchedule
		plan.AnswerShape = AnswerShapeDirect
		plan.MaxResults = 3
		plan.MaxIterations = 2
		plan.SearchContextSize = "low"
		plan.TimeRange = ""
		plan.RequiredFields = []string{"event", "start_time"}
		plan.AllowedDomains = []string{"fifa.com", "espn.com", "foxsports.com", "cbssports.com", "nbcsports.com"}
		plan.Queries = scheduleQueries(userText, now)
	case strings.Contains(lower, "score") || strings.Contains(lower, "who won"):
		plan.Intent = SearchIntentScore
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 4
		plan.SearchContextSize = "low"
		plan.RequiredFields = []string{"score_or_winner"}
	case strings.Contains(lower, "weather") || strings.Contains(lower, "forecast"):
		plan.Intent = SearchIntentWeather
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 3
		plan.SearchContextSize = "low"
	case strings.Contains(lower, "price") || strings.Contains(lower, "stock") || strings.Contains(lower, "market"):
		plan.Intent = SearchIntentPrice
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 3
		plan.SearchContextSize = "low"
	case strings.Contains(lower, "news") || strings.Contains(lower, "headline") || strings.Contains(lower, "breaking"):
		plan.Intent = SearchIntentNews
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 5
	case researchPattern.MatchString(lower):
		plan.AnswerShape = AnswerShapeResearch
		plan.MaxResults = 10
		plan.MaxIterations = 3
		plan.SearchContextSize = "high"
	}
	return plan
}

func scheduleQueries(userText string, now time.Time) []string {
	date := now.Format("January 2 2006")
	lower := strings.ToLower(userText)
	if strings.Contains(lower, "world cup") {
		return []string{
			fmt.Sprintf("FIFA World Cup %s match kickoff time official schedule", date),
			fmt.Sprintf("site:fifa.com World Cup %s schedule kickoff", date),
		}
	}
	cleaned := buildSearchQuery(userText, now)
	return []string{
		fmt.Sprintf("%s %s official start time", cleaned, date),
		fmt.Sprintf("%s %s schedule", cleaned, date),
	}
}

func NativeSearchConfigForPlan(plan SearchPlan, tc turncontext.TurnContext) *llm.NativeSearchConfig {
	if !plan.NeedsWeb || !plan.NativePreferred {
		return nil
	}
	maxTotal := plan.MaxResults * plan.MaxIterations
	if maxTotal < plan.MaxResults {
		maxTotal = plan.MaxResults
	}
	verbosity := "medium"
	if plan.AnswerShape == AnswerShapeDirect || plan.AnswerShape == AnswerShapeBrief {
		verbosity = "low"
	}
	return &llm.NativeSearchConfig{
		Enabled:         true,
		ContextSize:     plan.SearchContextSize,
		MaxResults:      plan.MaxResults,
		MaxTotalResults: maxTotal,
		AllowedDomains:  append([]string(nil), plan.AllowedDomains...),
		AnswerVerbosity: verbosity,
		UserLocation: &llm.UserLocation{
			Type:     "approximate",
			City:     tc.City,
			Region:   tc.Region,
			Country:  tc.Country,
			Timezone: tc.Timezone,
		},
	}
}

func SupportsNativeSearch(providerType, model string) bool {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	model = strings.ToLower(strings.TrimSpace(model))
	switch providerType {
	case "openrouter":
		return true
	case "gemini":
		return strings.HasPrefix(model, "gemini-2") || strings.HasPrefix(model, "gemini-3")
	case "openai":
		return strings.HasPrefix(model, "gpt-4.1") || strings.HasPrefix(model, "gpt-5") ||
			strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4")
	default:
		return false
	}
}
