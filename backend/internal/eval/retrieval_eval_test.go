package eval

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/websearch"
)

var evalNow = time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

// TestRetrievalEvalTracksMetrics is the Phase 5 exit criterion: the suite runs in
// CI against the tracked corpus and reports recall, intent accuracy, freshness
// policy, and query expansion as numbers rather than as a pass/fail feeling.
func TestRetrievalEvalTracksMetrics(t *testing.T) {
	report := RunRetrievalEval(DefaultRetrievalScenarios(), evalNow, "UTC")

	if report.Total == 0 {
		t.Fatal("the corpus must not be empty")
	}
	t.Logf("scenarios=%d passed=%d recall=%.2f intent=%.2f freshness=%.2f expansion=%.2f fn=%d fp=%d",
		report.Total, report.Passed, report.TriggerRecall, report.IntentAccuracy,
		report.FreshnessPolicyAccuracy, report.QueryExpansionRate,
		report.FalseNegatives, report.FalsePositives)
	for category, tally := range report.ByCategory {
		t.Logf("  %-15s %d/%d", category, tally.Passed, tally.Total)
	}

	for _, failure := range report.FailedScenarios() {
		t.Errorf("%s (%s) %q: %v", failure.ScenarioID, failure.Category, failure.Prompt, failure.Failures)
	}

	// Trigger recall is the headline number: the gate previously suppressed whole
	// categories of current-information questions outright, so any regression
	// here is the defect this work fixed coming back.
	if report.TriggerRecall < 1.0 {
		t.Errorf("TriggerRecall = %.2f, want 1.00 (%d false negatives)", report.TriggerRecall, report.FalseNegatives)
	}
	if report.FreshnessPolicyAccuracy < 1.0 {
		t.Errorf("FreshnessPolicyAccuracy = %.2f, want 1.00", report.FreshnessPolicyAccuracy)
	}
	if report.QueryExpansionRate < 1.0 {
		t.Errorf("QueryExpansionRate = %.2f, want 1.00", report.QueryExpansionRate)
	}
	// False positives cost latency and money rather than correctness, so this is
	// a budget rather than a hard zero.
	if report.FalsePositives > 0 {
		t.Errorf("FalsePositives = %d, want 0", report.FalsePositives)
	}
}

func TestRetrievalEvalDetectsRegressions(t *testing.T) {
	// A scenario that asserts a freshness window the planner would not choose
	// must be reported, not silently passed.
	scenarios := []RetrievalScenario{{
		ID: "regress-01", Category: "synthetic",
		Prompt:    "What is the current OpenAI API pricing?",
		ExpectWeb: true, ExpectFreshness: "24h",
	}}
	report := RunRetrievalEval(scenarios, evalNow, "UTC")
	if report.Passed != 0 {
		t.Fatal("a wrong freshness expectation must fail")
	}
	if report.FreshnessPolicyAccuracy != 0 {
		t.Errorf("FreshnessPolicyAccuracy = %.2f, want 0", report.FreshnessPolicyAccuracy)
	}
}

func TestRetrievalEvalCountsFalseNegativesAndPositives(t *testing.T) {
	scenarios := []RetrievalScenario{
		// Should search but will not: a plain code question.
		{ID: "fn", Category: "synthetic", Prompt: "Explain the quicksort algorithm", ExpectWeb: true},
		// Should not search but will: an explicit recency signal.
		{ID: "fp", Category: "synthetic", Prompt: "What is the latest news today?", ExpectWeb: false},
	}
	report := RunRetrievalEval(scenarios, evalNow, "UTC")
	if report.FalseNegatives != 1 {
		t.Errorf("FalseNegatives = %d, want 1", report.FalseNegatives)
	}
	if report.FalsePositives != 1 {
		t.Errorf("FalsePositives = %d, want 1", report.FalsePositives)
	}
	if report.TriggerRecall != 0 {
		t.Errorf("TriggerRecall = %.2f, want 0", report.TriggerRecall)
	}
}

func TestRetrievalEvalFlagsDeadIterations(t *testing.T) {
	// The invariant that caught the documented-but-unimplemented iteration
	// policy: a plan must never advertise more iterations than it has queries.
	report := RunRetrievalEval(DefaultRetrievalScenarios(), evalNow, "UTC")
	for _, result := range report.Results {
		for _, failure := range result.Failures {
			if len(failure) > 13 && failure[:13] == "MaxIterations" {
				t.Errorf("%s: %s", result.ScenarioID, failure)
			}
		}
	}
}

func TestRatioEmptyDenominator(t *testing.T) {
	// An unexercised metric must not read as a failure.
	if got := ratio(0, 0); got != 1 {
		t.Errorf("ratio(0,0) = %v, want 1", got)
	}
	if got := ratio(1, 2); got != 0.5 {
		t.Errorf("ratio(1,2) = %v, want 0.5", got)
	}
}

// TestDefaultCorpusCoversBothDirections guards the corpus itself: a suite made
// only of should-search cases would score perfect recall while saying nothing
// about over-triggering.
func TestDefaultCorpusCoversBothDirections(t *testing.T) {
	scenarios := DefaultRetrievalScenarios()
	positive, negative := 0, 0
	categories := map[string]bool{}
	for _, scenario := range scenarios {
		if scenario.ExpectWeb {
			positive++
		} else {
			negative++
		}
		categories[scenario.Category] = true
	}
	if positive < 8 {
		t.Errorf("only %d should-search scenarios", positive)
	}
	if negative < 8 {
		t.Errorf("only %d should-not-search scenarios; recall alone is not a quality measure", negative)
	}
	for _, required := range []string{"tech-currency", "pricing", "research", "time-boxed", "code"} {
		if !categories[required] {
			t.Errorf("corpus is missing the %q category", required)
		}
	}
}

func TestRetrievalScenarioIntentsAreRealIntents(t *testing.T) {
	valid := map[websearch.SearchIntent]bool{
		websearch.SearchIntentGeneral: true, websearch.SearchIntentSchedule: true,
		websearch.SearchIntentScore: true, websearch.SearchIntentNews: true,
		websearch.SearchIntentPrice: true, websearch.SearchIntentWeather: true,
		websearch.SearchIntentPricing: true, websearch.SearchIntentBenchmark: true,
		websearch.SearchIntentRelease: true,
	}
	for _, scenario := range DefaultRetrievalScenarios() {
		if scenario.ExpectIntent == "" {
			continue
		}
		if !valid[scenario.ExpectIntent] {
			t.Errorf("%s expects unknown intent %q", scenario.ID, scenario.ExpectIntent)
		}
	}
}
