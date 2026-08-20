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
	// SearchIntentPricing covers API/product pricing and cost lookups. It is
	// distinct from SearchIntentPrice (markets and stock quotes) because the two
	// want opposite freshness policies: a stock quote needs the last 24 hours,
	// while an API price lives on a docs page that may not have changed in
	// months. Collapsing them is what caused pricing lookups to be filtered down
	// to nothing.
	SearchIntentPricing SearchIntent = "pricing"
	// SearchIntentBenchmark covers leaderboard, eval, and capability comparisons.
	SearchIntentBenchmark SearchIntent = "benchmark"
	// SearchIntentRelease covers version, release, and availability questions.
	SearchIntentRelease SearchIntent = "release"
)

// SourceClass labels the kind of evidence a plan needs. Sufficiency is judged
// against these rather than against a raw result count.
type SourceClass string

const (
	// SourceClassOfficial is a first-party page: a vendor's own pricing, docs, or
	// release notes.
	SourceClassOfficial SourceClass = "official"
	// SourceClassPrimary is the originating publisher of a measurement or event,
	// as opposed to coverage of it.
	SourceClassPrimary SourceClass = "primary"
	// SourceClassAny accepts secondary coverage.
	SourceClassAny SourceClass = "any"
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

	// PreferredDomains ranks sources without excluding others. AllowedDomains is
	// a hard restriction; for pricing and benchmark work a hard restriction is
	// too brittle, because the authoritative host differs per vendor.
	PreferredDomains []string
	// RequiredSourceClass is the weakest class of evidence that satisfies this
	// plan.
	RequiredSourceClass SourceClass
	// MinSources is the number of distinct sources needed before the evidence is
	// considered sufficient. Comparative numeric claims need corroboration.
	MinSources int
	// RequiresCitations marks answers whose factual claims must be traceable.
	RequiresCitations bool
}

var (
	timeQuestionPattern = regexp.MustCompile(`(?i)\b(what time|when does|when is|start time|kickoff|tip[- ]?off|puck drop|first pitch)\b`)
	sportsEventPattern  = regexp.MustCompile(`(?i)\b(game|match|world cup|super bowl|final|play|playing|packers|nfl|nba|mlb|nhl|soccer|football)\b`)
	researchPattern     = regexp.MustCompile(`(?i)\b(deep research|comprehensive|detailed analysis|compare all|investigate|report on)\b`)

	// comparisonPattern recognises comparative framing without requiring the
	// literal phrases researchPattern looks for. The reviewed failure case
	// ("compare benchmark versus cost") matched none of those, so it was planned
	// as an ordinary one-shot lookup.
	comparisonPattern = regexp.MustCompile(`(?i)\b(compare|comparison|versus|vs\.?|which (one|model|option|provider) |best .{1,40} (for|under)|trade-?offs?|pros and cons)\b`)

	// pricingPattern covers API and product cost questions, which are answered by
	// vendor pricing pages rather than by news.
	pricingPattern = regexp.MustCompile(`(?i)\b(pricing|price per|cost per|how much (does|is|do).{0,30}(cost|charge)|per (million |1m |1k |thousand )?tokens?|subscription cost|api cost|api pricing)\b`)

	// benchmarkPattern covers evals, leaderboards, and capability rankings.
	benchmarkPattern = regexp.MustCompile(`(?i)\b(benchmarks?|leaderboards?|eval(uation)?s? scores?|swe-?bench|mmlu|humaneval|gpqa|arena (score|elo)|sota|state of the art|ranking)\b`)

	// releasePattern covers version and availability questions.
	releasePattern = regexp.MustCompile(`(?i)\b(versions?|releases?|release notes|changelog|generally available|deprecat(ed|ion)|end of life|available (now|models?)|newest|latest version)\b`)

	// marketPattern keeps genuine market data on the fast, tightly-filtered path.
	marketPattern = regexp.MustCompile(`(?i)\b(stock price|share price|ticker|exchange rate|crypto|bitcoin|ethereum|market cap|index (closed|opened))\b`)
)

func BuildSearchPlan(userText string, now time.Time, timezone string) SearchPlan {
	triggered, toolCall := ShouldWebSearch(userText, now, timezone)
	if !triggered || toolCall == nil {
		return SearchPlan{}
	}

	lower := strings.ToLower(strings.TrimSpace(userText))
	plan := SearchPlan{
		NeedsWeb:            true,
		Intent:              SearchIntentGeneral,
		AnswerShape:         AnswerShapeStandard,
		Queries:             []string{toolCall.Arguments.Query},
		TimeRange:           toolCall.Arguments.TimeRange,
		MaxResults:          6,
		MaxIterations:       2,
		SearchContextSize:   "medium",
		NativePreferred:     true,
		RequiredSourceClass: SourceClassAny,
		MinSources:          1,
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
		plan.RequiredSourceClass = SourceClassOfficial

	// Benchmarks are checked before the sports "score" branch. "Leaderboard
	// scores" and "benchmark scores" both contain the substring "score", so the
	// sports branch previously swallowed them and planned a 4-result brief
	// lookup for what is a multi-source research question. The sports guard keeps
	// "NFL power rankings" out of the benchmark path.
	case benchmarkPattern.MatchString(lower) && !sportsEventPattern.MatchString(lower):
		plan.Intent = SearchIntentBenchmark
		plan.AnswerShape = AnswerShapeResearch
		plan.MaxResults = 10
		plan.MaxIterations = 3
		plan.SearchContextSize = "high"
		plan.TimeRange = ""
		plan.PreferredDomains = officialBenchmarkDomains
		plan.RequiredSourceClass = SourceClassPrimary
		plan.RequiresCitations = true
		plan.MinSources = 2
		plan.Queries = benchmarkQueries(userText, now)

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

	// Pricing is checked before the generic market branch so that "API pricing"
	// does not inherit a 24-hour stock-quote freshness window.
	case pricingPattern.MatchString(lower):
		plan.Intent = SearchIntentPricing
		plan.AnswerShape = AnswerShapeStandard
		plan.MaxResults = 8
		plan.MaxIterations = 3
		plan.SearchContextSize = "medium"
		// Vendor pricing pages are stable documents, not news. A recency filter
		// removes the authoritative source.
		plan.TimeRange = ""
		plan.PreferredDomains = officialPricingDomains
		plan.RequiredSourceClass = SourceClassOfficial
		plan.RequiresCitations = true
		plan.MinSources = 2
		plan.Queries = pricingQueries(userText, now)

	case marketPattern.MatchString(lower) || strings.Contains(lower, "stock"):
		plan.Intent = SearchIntentPrice
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 3
		plan.SearchContextSize = "low"
		if plan.TimeRange == "" {
			plan.TimeRange = "24h"
		}

	case releasePattern.MatchString(lower):
		plan.Intent = SearchIntentRelease
		plan.AnswerShape = AnswerShapeStandard
		plan.MaxResults = 8
		plan.MaxIterations = 2
		plan.SearchContextSize = "medium"
		// Release notes are dated documents; filtering to the last day hides the
		// release the user is asking about unless it shipped today.
		plan.TimeRange = ""
		plan.RequiredSourceClass = SourceClassOfficial
		plan.RequiresCitations = true
		plan.Queries = releaseQueries(userText, now)

	case strings.Contains(lower, "news") || strings.Contains(lower, "headline") || strings.Contains(lower, "breaking"):
		plan.Intent = SearchIntentNews
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 5
		if plan.TimeRange == "" {
			plan.TimeRange = "24h"
		}

	case researchPattern.MatchString(lower) || comparisonPattern.MatchString(lower):
		plan.AnswerShape = AnswerShapeResearch
		plan.MaxResults = 10
		plan.MaxIterations = 3
		plan.SearchContextSize = "high"
		plan.TimeRange = ""
		plan.RequiresCitations = true
		plan.MinSources = 2
		plan.Queries = researchQueries(userText, now)

	case strings.Contains(lower, "price") || strings.Contains(lower, "market"):
		// Remaining bare price/market mentions: treat as market data.
		plan.Intent = SearchIntentPrice
		plan.AnswerShape = AnswerShapeBrief
		plan.MaxResults = 3
		plan.SearchContextSize = "low"
	}

	return normalizePlan(plan)
}

// normalizePlan keeps the plan internally consistent.
//
// MaxIterations above len(Queries) is dead configuration: searchWithPlan clamps
// its loop to the query count, so the extra iterations never run. Advertising
// them made the search policy look broader than it was, which is how the
// documented "up to 3 targeted iterations" diverged from a single actual search.
func normalizePlan(plan SearchPlan) SearchPlan {
	if plan.MaxIterations > len(plan.Queries) {
		plan.MaxIterations = len(plan.Queries)
	}
	if plan.MaxIterations < 1 && len(plan.Queries) > 0 {
		plan.MaxIterations = 1
	}
	if plan.MinSources < 1 {
		plan.MinSources = 1
	}
	return plan
}

// officialPricingDomains ranks first-party pricing pages ahead of aggregators.
// Aggregator pages are frequently months stale, which is exactly how confident
// wrong prices reach an answer.
var officialPricingDomains = []string{
	"anthropic.com", "openai.com", "ai.google.dev", "cloud.google.com",
	"azure.microsoft.com", "aws.amazon.com", "openrouter.ai", "mistral.ai",
	"groq.com", "deepseek.com", "x.ai", "cohere.com",
}

// officialBenchmarkDomains ranks the publishers of benchmark results ahead of
// commentary about them.
var officialBenchmarkDomains = []string{
	"swebench.com", "lmarena.ai", "artificialanalysis.ai", "paperswithcode.com",
	"huggingface.co", "arxiv.org", "github.com", "epoch.ai",
	"anthropic.com", "openai.com", "ai.google.dev",
}

// queryVariants builds a bounded, de-duplicated query set.
//
// MaxIterations above 1 used to be dead: BuildSearchPlan emitted exactly one
// query for every non-schedule intent, and searchWithPlan clamps its iteration
// count to len(Queries). Raising MaxIterations without also raising the query
// count changed nothing.
func queryVariants(base string, suffixes ...string) []string {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil
	}
	out := make([]string, 0, len(suffixes)+1)
	seen := map[string]bool{}
	add := func(q string) {
		q = strings.Join(strings.Fields(q), " ")
		if q == "" {
			return
		}
		key := strings.ToLower(q)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, q)
	}
	add(base)
	for _, suffix := range suffixes {
		add(base + " " + suffix)
	}
	return out
}

func pricingQueries(userText string, now time.Time) []string {
	base := buildSearchQuery(userText, now)
	return queryVariants(base,
		"official pricing page",
		fmt.Sprintf("price per million tokens %d", now.Year()),
	)
}

func benchmarkQueries(userText string, now time.Time) []string {
	base := buildSearchQuery(userText, now)
	return queryVariants(base,
		"official benchmark results",
		fmt.Sprintf("leaderboard %d", now.Year()),
	)
}

func releaseQueries(userText string, now time.Time) []string {
	base := buildSearchQuery(userText, now)
	return queryVariants(base,
		"official release notes changelog",
	)
}

func researchQueries(userText string, now time.Time) []string {
	base := buildSearchQuery(userText, now)
	return queryVariants(base,
		"official documentation",
		fmt.Sprintf("comparison %d", now.Year()),
	)
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
	// Only AllowedDomains is forwarded as a hard restriction. PreferredDomains is
	// deliberately *not* sent: provider-native search offers no ranking hint, only
	// an allow/block list, and turning a preference into a restriction would drop
	// the authoritative source for any vendor missing from the list.
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

// openRouterNativeSearchPrefixes lists the model families known to support
// OpenRouter's openrouter:web_search server tool.
//
// This used to be an unconditional `return true` for the whole provider, which
// assumed every model behind an OpenRouter profile supports server-side search.
// A successful HTTP response is not evidence that grounding happened: a route
// that ignores the tool returns 200 with an ungrounded answer, and the
// orchestrator then treated it as a successful web search. Being wrong in this
// direction produces a confident stale answer; being wrong the other way costs
// one local search, so the list is deliberately conservative.
var openRouterNativeSearchPrefixes = []string{
	"anthropic/claude-",
	"openai/gpt-4.1", "openai/gpt-5", "openai/o3", "openai/o4",
	"google/gemini-2", "google/gemini-3",
	"perplexity/",
	"x-ai/grok-3", "x-ai/grok-4",
}

// SupportsNativeSearch reports whether a provider/model pair can ground its own
// answer, removing the need for a separate local search and summarization pass.
//
// A false result is not a failure: it routes the turn to the local
// Brave/DuckDuckGo path, which every provider can use.
func SupportsNativeSearch(providerType, model string) bool {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	model = strings.ToLower(strings.TrimSpace(model))
	switch providerType {
	case "openrouter":
		for _, prefix := range openRouterNativeSearchPrefixes {
			if strings.HasPrefix(model, prefix) {
				return true
			}
		}
		return false
	case "anthropic":
		// Anthropic runs web search as a Messages API server tool. It is not
		// available through the OpenAI-compatibility endpoint, so the LLM
		// transport rewrites the request; see llm/anthropic_search.go.
		return llm.SupportsAnthropicNativeSearch(model)
	case "gemini":
		return strings.HasPrefix(model, "gemini-2") || strings.HasPrefix(model, "gemini-3")
	case "openai":
		return strings.HasPrefix(model, "gpt-4.1") || strings.HasPrefix(model, "gpt-5") ||
			strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4")
	default:
		return false
	}
}
