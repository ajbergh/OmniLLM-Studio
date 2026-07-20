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
