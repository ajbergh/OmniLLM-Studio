package eval

import (
	"fmt"
	"sort"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

// RetrievalScenario is one current-information classification case.
//
// The suite is deliberately deterministic: it exercises the classifier and
// planner, not a live provider or a model. That makes it runnable in CI on every
// change, which is the point — the defects this work fixed were all silent, and
// a suite that needs network access or an API key would not have caught them.
//
// Live-retrieval quality (does Brave actually return the vendor pricing page?)
// is a separate concern and needs recorded fixtures per provider; see the
// provider tests in internal/websearch.
type RetrievalScenario struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
	// Category groups scenarios in the report so a regression can be attributed.
	Category string `json:"category"`

	// ExpectWeb is whether the turn should trigger retrieval at all.
	ExpectWeb bool `json:"expect_web"`
	// ExpectIntent, when set, is the intent the planner should choose.
	ExpectIntent websearch.SearchIntent `json:"expect_intent,omitempty"`
	// ExpectShape, when set, is the answer shape the planner should choose.
	ExpectShape websearch.AnswerShape `json:"expect_shape,omitempty"`
	// ExpectFreshness is the freshness window the plan should carry. An empty
	// string asserts *no* window, which matters for reference material: a
	// 24-hour filter on a pricing lookup removes the authoritative page.
	ExpectFreshness string `json:"expect_freshness"`
	// MinQueries is the smallest acceptable query-set size. Anything above 1
	// asserts that query expansion happened, without which MaxIterations is dead.
	MinQueries int `json:"min_queries,omitempty"`
	// RequireCitations asserts the plan marks its claims as needing sources.
	RequireCitations bool `json:"require_citations,omitempty"`
}

// RetrievalScenarioResult records the checks for one scenario.
type RetrievalScenarioResult struct {
	ScenarioID string   `json:"scenario_id"`
	Prompt     string   `json:"prompt"`
	Category   string   `json:"category"`
	Passed     bool     `json:"passed"`
	Failures   []string `json:"failures,omitempty"`

	GotWeb       bool                   `json:"got_web"`
	GotIntent    websearch.SearchIntent `json:"got_intent,omitempty"`
	GotShape     websearch.AnswerShape  `json:"got_shape,omitempty"`
	GotFreshness string                 `json:"got_freshness"`
	GotQueries   int                    `json:"got_queries"`
}

// RetrievalEvalReport aggregates the tracked metrics.
type RetrievalEvalReport struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Results     []RetrievalScenarioResult `json:"results"`

	Total  int `json:"total"`
	Passed int `json:"passed"`

	// TriggerRecall is the fraction of should-search prompts that did search.
	// This is the headline number: the gate previously suppressed whole
	// categories of current-information questions outright.
	TriggerRecall float64 `json:"trigger_recall"`
	// FalseNegatives is the count of should-search prompts that did not.
	FalseNegatives int `json:"false_negatives"`
	// FalsePositives is the count of should-not-search prompts that did. These
	// cost latency and money rather than correctness, so a nonzero value is a
	// tuning signal rather than a failure.
	FalsePositives int `json:"false_positives"`
	// IntentAccuracy is the fraction of triggered scenarios whose intent matched.
	IntentAccuracy float64 `json:"intent_accuracy"`
	// FreshnessPolicyAccuracy is the fraction whose freshness window matched.
	FreshnessPolicyAccuracy float64 `json:"freshness_policy_accuracy"`
	// QueryExpansionRate is the fraction of citation-requiring scenarios that
	// produced more than one query.
	QueryExpansionRate float64 `json:"query_expansion_rate"`

	// ByCategory reports pass counts per category.
	ByCategory map[string]CategoryTally `json:"by_category"`
}

// CategoryTally is a per-category pass count.
type CategoryTally struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
}

// RunRetrievalEval scores scenarios against the classifier and planner.
//
// now is injected so the suite is reproducible: several plans embed the current
// year in their queries.
func RunRetrievalEval(scenarios []RetrievalScenario, now time.Time, timezone string) RetrievalEvalReport {
	report := RetrievalEvalReport{
		GeneratedAt: now.UTC(),
		Total:       len(scenarios),
		ByCategory:  map[string]CategoryTally{},
	}

	shouldSearch := 0
	triggered := 0
	intentChecked, intentCorrect := 0, 0
	freshnessChecked, freshnessCorrect := 0, 0
	expansionChecked, expansionCorrect := 0, 0

	for _, scenario := range scenarios {
		plan := websearch.BuildSearchPlan(scenario.Prompt, now, timezone)
		result := RetrievalScenarioResult{
			ScenarioID:   scenario.ID,
			Prompt:       scenario.Prompt,
			Category:     scenario.Category,
			GotWeb:       plan.NeedsWeb,
			GotIntent:    plan.Intent,
			GotShape:     plan.AnswerShape,
			GotFreshness: plan.TimeRange,
			GotQueries:   len(plan.Queries),
		}

		if scenario.ExpectWeb {
			shouldSearch++
			if plan.NeedsWeb {
				triggered++
			} else {
				report.FalseNegatives++
				result.Failures = append(result.Failures, "expected retrieval, got none")
			}
		} else if plan.NeedsWeb {
			report.FalsePositives++
			result.Failures = append(result.Failures, "unexpected retrieval")
		}

		if scenario.ExpectWeb && plan.NeedsWeb {
			if scenario.ExpectIntent != "" {
				intentChecked++
				if plan.Intent == scenario.ExpectIntent {
					intentCorrect++
				} else {
					result.Failures = append(result.Failures,
						fmt.Sprintf("intent %q, want %q", plan.Intent, scenario.ExpectIntent))
				}
			}
			if scenario.ExpectShape != "" && plan.AnswerShape != scenario.ExpectShape {
				result.Failures = append(result.Failures,
					fmt.Sprintf("shape %q, want %q", plan.AnswerShape, scenario.ExpectShape))
			}

			freshnessChecked++
			if plan.TimeRange == scenario.ExpectFreshness {
				freshnessCorrect++
			} else {
				result.Failures = append(result.Failures,
					fmt.Sprintf("freshness %q, want %q", plan.TimeRange, scenario.ExpectFreshness))
			}

			if scenario.MinQueries > 0 {
				expansionChecked++
				if len(plan.Queries) >= scenario.MinQueries {
					expansionCorrect++
				} else {
					result.Failures = append(result.Failures,
						fmt.Sprintf("%d quer(y|ies), want at least %d", len(plan.Queries), scenario.MinQueries))
				}
			}
			if scenario.RequireCitations && !plan.RequiresCitations {
				result.Failures = append(result.Failures, "plan does not require citations")
			}
			// A plan must never advertise more iterations than it has queries;
			// that was how documented "up to 3 iterations" became one search.
			if plan.MaxIterations > len(plan.Queries) {
				result.Failures = append(result.Failures,
					fmt.Sprintf("MaxIterations %d exceeds %d queries", plan.MaxIterations, len(plan.Queries)))
			}
		}

		result.Passed = len(result.Failures) == 0
		if result.Passed {
			report.Passed++
		}
		tally := report.ByCategory[scenario.Category]
		tally.Total++
		if result.Passed {
			tally.Passed++
		}
		report.ByCategory[scenario.Category] = tally

		report.Results = append(report.Results, result)
	}

	report.TriggerRecall = ratio(triggered, shouldSearch)
	report.IntentAccuracy = ratio(intentCorrect, intentChecked)
	report.FreshnessPolicyAccuracy = ratio(freshnessCorrect, freshnessChecked)
	report.QueryExpansionRate = ratio(expansionCorrect, expansionChecked)

	sort.SliceStable(report.Results, func(i, j int) bool {
		return report.Results[i].ScenarioID < report.Results[j].ScenarioID
	})
	return report
}

// ratio guards division by zero and returns 1 for an empty denominator, so an
// unexercised metric does not read as a failure.
func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 1
	}
	return float64(numerator) / float64(denominator)
}

// FailedScenarios returns the scenarios that did not pass, for test output.
func (r RetrievalEvalReport) FailedScenarios() []RetrievalScenarioResult {
	var failed []RetrievalScenarioResult
	for _, result := range r.Results {
		if !result.Passed {
			failed = append(failed, result)
		}
	}
	return failed
}

// DefaultRetrievalScenarios is the tracked corpus.
//
// Every "tech-currency" entry below was suppressed outright before this work:
// the gate's negative patterns returned early on "coding", "react",
// "kubernetes", "npm" and friends, so these were answered from training data no
// matter how explicitly they asked for the newest state.
func DefaultRetrievalScenarios() []RetrievalScenario {
	return []RetrievalScenario{
		// --- Technology currency: the previously suppressed category ---
		{
			ID: "tech-01", Category: "tech-currency",
			Prompt:    "What's the latest version of React?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentRelease,
			ExpectFreshness: "", MinQueries: 2, RequireCitations: true,
		},
		{
			ID: "tech-02", Category: "tech-currency",
			Prompt:    "What is the best LLM for Go coding right now?",
			ExpectWeb: true, ExpectFreshness: "",
		},
		{
			ID: "tech-03", Category: "tech-currency",
			Prompt:    "What's the current Kubernetes release?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentRelease,
			ExpectFreshness: "", MinQueries: 2,
		},
		{
			ID: "tech-04", Category: "tech-currency",
			Prompt:    "Is there a newer Docker Engine version than 27?",
			ExpectWeb: true, ExpectFreshness: "",
		},
		{
			ID: "tech-05", Category: "tech-currency",
			Prompt:    "What are the latest benchmark results for coding models?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentBenchmark,
			ExpectShape:     websearch.AnswerShapeResearch,
			ExpectFreshness: "", MinQueries: 3, RequireCitations: true,
		},

		// --- Pricing: stable vendor documents, no recency filter ---
		{
			ID: "price-01", Category: "pricing",
			Prompt:    "What is the current OpenAI API pricing?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentPricing,
			ExpectFreshness: "", MinQueries: 3, RequireCitations: true,
		},
		{
			ID: "price-02", Category: "pricing",
			Prompt:    "How much does Claude cost per million tokens?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentPricing,
			ExpectFreshness: "", MinQueries: 3, RequireCitations: true,
		},

		// --- The reviewed failure case ---
		{
			ID: "research-01", Category: "research",
			Prompt:    "Research the best LLM available via API and compare benchmark versus cost",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentBenchmark,
			ExpectShape:     websearch.AnswerShapeResearch,
			ExpectFreshness: "", MinQueries: 3, RequireCitations: true,
		},
		{
			ID: "research-02", Category: "research",
			Prompt:    "Give me a comprehensive investigation of European energy policy",
			ExpectWeb: true, ExpectShape: websearch.AnswerShapeResearch,
			ExpectFreshness: "", MinQueries: 3, RequireCitations: true,
		},

		// --- Genuinely time-boxed: tight windows must be preserved ---
		{
			ID: "news-01", Category: "time-boxed",
			Prompt:    "What's the breaking news today?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentNews,
			ExpectFreshness: "24h",
		},
		{
			ID: "market-01", Category: "time-boxed",
			Prompt:    "What is the Tesla stock price right now?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentPrice,
			ExpectFreshness: "24h",
		},
		{
			ID: "weather-01", Category: "time-boxed",
			Prompt:    "What's the weather today?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentWeather,
			ExpectFreshness: "24h",
		},
		{
			ID: "score-01", Category: "time-boxed",
			Prompt:    "Who won the game last night?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentScore,
			ExpectFreshness: "7d",
		},

		// --- Sports schedule: the deterministic direct path ---
		{
			ID: "sched-01", Category: "sports",
			Prompt:    "What time does the World Cup game start today?",
			ExpectWeb: true, ExpectIntent: websearch.SearchIntentSchedule,
			ExpectShape:     websearch.AnswerShapeDirect,
			ExpectFreshness: "", MinQueries: 2,
		},

		// --- Must NOT search: code questions ---
		{ID: "code-01", Category: "code", Prompt: "How do I implement a binary search in Python?"},
		{ID: "code-02", Category: "code", Prompt: "Explain the quicksort algorithm"},
		{ID: "code-03", Category: "code", Prompt: "Help me debug this function error"},
		{ID: "code-04", Category: "code", Prompt: "Write an SQL query to find duplicates"},
		{ID: "code-05", Category: "code", Prompt: "How to deploy with Docker?"},
		{ID: "code-06", Category: "code", Prompt: "Refactor this function to use a map"},
		{ID: "code-07", Category: "code", Prompt: "What is a closure in programming?"},

		// --- Must NOT search: no temporal need ---
		{ID: "static-01", Category: "static", Prompt: "Python vs Go for backends"},
		{ID: "static-02", Category: "static", Prompt: "Write a haiku about autumn"},
		{ID: "static-03", Category: "static", Prompt: "The current state of affairs"},
	}
}
