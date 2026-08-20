package websearch

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSearchPlanWorldCupDirectLookup(t *testing.T) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 19, 7, 30, 0, 0, loc)
	plan := BuildSearchPlan("What Time Does the World Cup Game Start Today", now, "America/Chicago")
	if !plan.NeedsWeb || plan.Intent != SearchIntentSchedule || plan.AnswerShape != AnswerShapeDirect {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if plan.TimeRange != "" {
		t.Fatalf("schedule lookup should not use a recency filter: %q", plan.TimeRange)
	}
	if len(plan.Queries) < 2 || !strings.Contains(plan.Queries[0], "July 19 2026") {
		t.Fatalf("expected exact-date targeted queries: %#v", plan.Queries)
	}
	if plan.SearchContextSize != "low" || plan.MaxResults > 3 || plan.MaxIterations != 2 {
		t.Fatalf("direct lookup should use the bounded cheap path: %#v", plan)
	}
}

// TestBuildSearchPlanShapes is the Phase 1 exit criterion: representative
// current-information prompts must produce the intended
// (NeedsWeb, Intent, AnswerShape, TimeRange, len(Queries)) tuple.
//
// The two properties that previously failed and are asserted here:
//   - research and comparison prompts emit more than one query, so
//     MaxIterations is no longer dead;
//   - pricing, benchmark, and release prompts carry no freshness filter, because
//     a 24-hour window excluded the authoritative pages they need.
func TestBuildSearchPlanShapes(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		prompt        string
		wantWeb       bool
		wantIntent    SearchIntent
		wantShape     AnswerShape
		wantTimeRange string
		minQueries    int
		requireCites  bool
	}{
		// --- The reviewed failure case ---
		{
			name:          "llm benchmark versus cost research",
			prompt:        "Research the best LLM available via API and compare benchmark versus cost",
			wantWeb:       true,
			wantIntent:    SearchIntentBenchmark,
			wantShape:     AnswerShapeResearch,
			wantTimeRange: "",
			minQueries:    3,
			requireCites:  true,
		},

		// --- Pricing: stable vendor documents, no recency filter ---
		{
			name:          "api pricing",
			prompt:        "What is the current OpenAI API pricing?",
			wantWeb:       true,
			wantIntent:    SearchIntentPricing,
			wantShape:     AnswerShapeStandard,
			wantTimeRange: "",
			minQueries:    3,
			requireCites:  true,
		},
		{
			name:          "cost per token",
			prompt:        "How much does Claude cost per million tokens?",
			wantWeb:       true,
			wantIntent:    SearchIntentPricing,
			wantShape:     AnswerShapeStandard,
			wantTimeRange: "",
			minQueries:    3,
			requireCites:  true,
		},

		// --- Benchmarks ---
		{
			name:          "leaderboard",
			prompt:        "What are the latest SWE-bench leaderboard scores?",
			wantWeb:       true,
			wantIntent:    SearchIntentBenchmark,
			wantShape:     AnswerShapeResearch,
			wantTimeRange: "",
			minQueries:    3,
			requireCites:  true,
		},

		// --- Releases and versions ---
		{
			name:          "latest react version",
			prompt:        "What's the latest version of React?",
			wantWeb:       true,
			wantIntent:    SearchIntentRelease,
			wantShape:     AnswerShapeStandard,
			wantTimeRange: "",
			minQueries:    2,
			requireCites:  true,
		},
		{
			name:          "current kubernetes release",
			prompt:        "What's the current Kubernetes release?",
			wantWeb:       true,
			wantIntent:    SearchIntentRelease,
			wantShape:     AnswerShapeStandard,
			wantTimeRange: "",
			minQueries:    2,
			requireCites:  true,
		},

		// --- News keeps a tight window ---
		{
			name:          "breaking news",
			prompt:        "What's the breaking news today?",
			wantWeb:       true,
			wantIntent:    SearchIntentNews,
			wantShape:     AnswerShapeBrief,
			wantTimeRange: "24h",
			minQueries:    1,
		},

		// --- Market data keeps a tight window ---
		{
			name:          "stock price",
			prompt:        "What is the Tesla stock price right now?",
			wantWeb:       true,
			wantIntent:    SearchIntentPrice,
			wantShape:     AnswerShapeBrief,
			wantTimeRange: "24h",
			minQueries:    1,
		},

		// --- Weather and scores unchanged ---
		{
			name:          "weather",
			prompt:        "What's the weather today?",
			wantWeb:       true,
			wantIntent:    SearchIntentWeather,
			wantShape:     AnswerShapeBrief,
			wantTimeRange: "24h",
			minQueries:    1,
		},
		{
			name:          "who won",
			prompt:        "Who won the game last night?",
			wantWeb:       true,
			wantIntent:    SearchIntentScore,
			wantShape:     AnswerShapeBrief,
			wantTimeRange: "7d",
			minQueries:    1,
		},

		// --- Explicit deep research ---
		{
			name:          "comprehensive investigation",
			prompt:        "Give me a comprehensive investigation of European energy policy",
			wantWeb:       true,
			wantIntent:    SearchIntentGeneral,
			wantShape:     AnswerShapeResearch,
			wantTimeRange: "",
			minQueries:    3,
			requireCites:  true,
		},

		// --- Not current information ---
		{
			name:    "implementation question",
			prompt:  "How do I implement a binary search in Python?",
			wantWeb: false,
		},
		{
			name:    "conceptual question",
			prompt:  "Explain the quicksort algorithm",
			wantWeb: false,
		},
		{
			name:    "pasted code",
			prompt:  "Fix the latest error here:\n```go\nx := 1\n```",
			wantWeb: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := BuildSearchPlan(tc.prompt, now, "UTC")

			if plan.NeedsWeb != tc.wantWeb {
				t.Fatalf("NeedsWeb = %v, want %v", plan.NeedsWeb, tc.wantWeb)
			}
			if !tc.wantWeb {
				return
			}
			if plan.Intent != tc.wantIntent {
				t.Errorf("Intent = %q, want %q", plan.Intent, tc.wantIntent)
			}
			if plan.AnswerShape != tc.wantShape {
				t.Errorf("AnswerShape = %q, want %q", plan.AnswerShape, tc.wantShape)
			}
			if plan.TimeRange != tc.wantTimeRange {
				t.Errorf("TimeRange = %q, want %q", plan.TimeRange, tc.wantTimeRange)
			}
			if len(plan.Queries) < tc.minQueries {
				t.Errorf("len(Queries) = %d, want >= %d: %#v", len(plan.Queries), tc.minQueries, plan.Queries)
			}
			if tc.requireCites && !plan.RequiresCitations {
				t.Error("RequiresCitations should be set for claim-bearing intents")
			}
			// MaxIterations above 1 is meaningless unless there are queries to run.
			if plan.MaxIterations > len(plan.Queries) {
				t.Errorf("MaxIterations = %d exceeds len(Queries) = %d, so the extra iterations are dead",
					plan.MaxIterations, len(plan.Queries))
			}
		})
	}
}

func TestBuildSearchPlanQueriesAreDistinct(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	plan := BuildSearchPlan("What is the current Anthropic API pricing?", now, "UTC")
	if len(plan.Queries) < 2 {
		t.Fatalf("expected an expanded query set, got %#v", plan.Queries)
	}
	seen := map[string]bool{}
	for _, q := range plan.Queries {
		if strings.TrimSpace(q) == "" {
			t.Fatal("query set must not contain empty queries")
		}
		if seen[strings.ToLower(q)] {
			t.Fatalf("duplicate query in set: %#v", plan.Queries)
		}
		seen[strings.ToLower(q)] = true
	}
}

func TestPricingPlanPrefersOfficialDomains(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	plan := BuildSearchPlan("What is the current OpenAI API pricing?", now, "UTC")
	if len(plan.PreferredDomains) == 0 {
		t.Fatal("pricing plans must rank first-party pricing pages")
	}
	if plan.RequiredSourceClass != SourceClassOfficial {
		t.Errorf("RequiredSourceClass = %q, want %q", plan.RequiredSourceClass, SourceClassOfficial)
	}
	if plan.MinSources < 2 {
		t.Errorf("MinSources = %d; a price quoted as current needs corroboration", plan.MinSources)
	}
	// AllowedDomains is a hard filter and must stay empty here: the
	// authoritative host varies per vendor, and a missing entry would silently
	// drop the only good source.
	if len(plan.AllowedDomains) != 0 {
		t.Errorf("pricing plans must rank rather than restrict, got AllowedDomains=%v", plan.AllowedDomains)
	}
}

func TestValidateDirectScheduleAnswer(t *testing.T) {
	plan := SearchPlan{Intent: SearchIntentSchedule, AnswerShape: AnswerShapeDirect}
	if ok, reason := ValidateAnswer(plan, "Argentina vs. Spain starts at 3:00 PM CDT."); !ok {
		t.Fatalf("valid answer rejected: %s", reason)
	}
	bad := "To determine the time, consult the official schedule.\n\n## Key Takeaways"
	if ok, _ := ValidateAnswer(plan, bad); ok {
		t.Fatal("indirect non-answer was accepted")
	}
}

func TestProviderNativeSearchCapabilities(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     bool
	}{
		{"openai", "gpt-5.2", true},
		{"gemini", "gemini-3.1-flash-lite", true},
		{"openrouter", "anthropic/claude-sonnet-4.5", true},
		{"ollama", "llama3.2", false},
		{"anthropic", "claude-opus-4-7", false},
		{"openai-compatible", "custom-model", false},
	}
	for _, tc := range cases {
		if got := SupportsNativeSearch(tc.provider, tc.model); got != tc.want {
			t.Errorf("SupportsNativeSearch(%q, %q)=%v want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}
