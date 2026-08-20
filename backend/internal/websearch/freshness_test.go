package websearch

import (
	"testing"
	"time"
)

var freshnessNow = time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

func TestParsePublishedAtRelative(t *testing.T) {
	cases := map[string]time.Duration{
		"2 hours ago":    2 * time.Hour,
		"1 hour ago":     time.Hour,
		"45 minutes ago": 45 * time.Minute,
		"3 days ago":     3 * 24 * time.Hour,
		"2 weeks ago":    14 * 24 * time.Hour,
		"1 month ago":    30 * 24 * time.Hour,
		"2 years ago":    2 * 365 * 24 * time.Hour,
		"  4 days ago ":  4 * 24 * time.Hour,
	}
	for input, want := range cases {
		got, ok := ParsePublishedAt(input, freshnessNow)
		if !ok {
			t.Errorf("ParsePublishedAt(%q) failed to parse", input)
			continue
		}
		if diff := freshnessNow.Sub(got); diff != want {
			t.Errorf("ParsePublishedAt(%q) age = %v, want %v", input, diff, want)
		}
	}
}

func TestParsePublishedAtAbsolute(t *testing.T) {
	// Brave sends page_age as an ISO timestamp and age as a phrase; both are the
	// same concept and both were previously stored unparsed.
	for _, input := range []string{
		"2026-08-18T00:00:00Z",
		"2026-08-18",
		"August 18, 2026",
		"Aug 18, 2026",
		"18 Aug 2026",
	} {
		got, ok := ParsePublishedAt(input, freshnessNow)
		if !ok {
			t.Errorf("ParsePublishedAt(%q) failed to parse", input)
			continue
		}
		if got.Year() != 2026 || got.Month() != time.August || got.Day() != 18 {
			t.Errorf("ParsePublishedAt(%q) = %v", input, got)
		}
	}
}

// TestParsePublishedAtUnparsable pins the three-state distinction: an undated
// result is neither fresh nor stale, and conflating it with either is how an
// undated page gets presented as current.
func TestParsePublishedAtUnparsable(t *testing.T) {
	for _, input := range []string{"", "   ", "recently", "a while back", "yesterday", "-3 days ago"} {
		if _, ok := ParsePublishedAt(input, freshnessNow); ok {
			t.Errorf("ParsePublishedAt(%q) should report no usable date", input)
		}
	}
}

func TestTimeRangeDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
	}
	for input, want := range cases {
		got, ok := TimeRangeDuration(input)
		if !ok || got != want {
			t.Errorf("TimeRangeDuration(%q) = %v,%v want %v,true", input, got, ok, want)
		}
	}
	// An empty window means "no constraint", not "zero tolerance".
	if _, ok := TimeRangeDuration(""); ok {
		t.Error("an empty window must report no constraint")
	}
	if _, ok := TimeRangeDuration("1y"); ok {
		t.Error("an unknown window must report no constraint")
	}
}

func TestEvaluateFreshnessVerified(t *testing.T) {
	plan := SearchPlan{TimeRange: "7d"}
	results := []SearchResult{
		{URL: "https://a.test/1", PublishedAt: "2 hours ago"},
		{URL: "https://b.test/2", PublishedAt: "3 days ago"},
	}
	report := EvaluateFreshness(plan, results, freshnessNow)
	if !report.Verified {
		t.Error("all dated results inside the window must verify")
	}
	if report.Dated != 2 || report.Undated != 0 || report.WithinWindow != 2 {
		t.Errorf("unexpected counts: %+v", report)
	}
	if report.NewestLabel != "2 hours ago" {
		t.Errorf("NewestLabel = %q, want the provider's own string", report.NewestLabel)
	}
}

// TestEvaluateFreshnessOneStaleSourceFailsVerification is the case that matters:
// a single stale source among fresh ones is exactly where a model quotes the
// wrong number, so "most results are fresh" must not verify.
func TestEvaluateFreshnessOneStaleSourceFailsVerification(t *testing.T) {
	plan := SearchPlan{TimeRange: "24h"}
	results := []SearchResult{
		{URL: "https://a.test/1", PublishedAt: "2 hours ago"},
		{URL: "https://b.test/2", PublishedAt: "3 months ago"},
	}
	report := EvaluateFreshness(plan, results, freshnessNow)
	if report.Verified {
		t.Error("a stale dated source must prevent verification")
	}
	if report.WithinWindow != 1 {
		t.Errorf("WithinWindow = %d, want 1", report.WithinWindow)
	}
}

func TestEvaluateFreshnessUndatedNeverVerifies(t *testing.T) {
	plan := SearchPlan{TimeRange: "24h"}
	results := []SearchResult{
		{URL: "https://a.test/1", PublishedAt: ""},
		{URL: "https://b.test/2", PublishedAt: "unknown"},
	}
	report := EvaluateFreshness(plan, results, freshnessNow)
	if report.Verified {
		t.Error("entirely undated results prove nothing and must not verify")
	}
	if report.Undated != 2 || report.Dated != 0 {
		t.Errorf("unexpected counts: %+v", report)
	}
}

// TestEvaluateFreshnessNoWindowNeverVerifies covers the pricing and release
// intents, which deliberately apply no recency filter. Absence of a filter is not
// evidence of freshness.
func TestEvaluateFreshnessNoWindowNeverVerifies(t *testing.T) {
	plan := SearchPlan{TimeRange: ""}
	results := []SearchResult{{URL: "https://a.test/1", PublishedAt: "2 hours ago"}}
	report := EvaluateFreshness(plan, results, freshnessNow)
	if report.Verified {
		t.Error("no requested window means nothing to verify against")
	}
	if report.Dated != 1 {
		t.Errorf("dates must still be counted: %+v", report)
	}
	if report.NewestLabel == "" {
		t.Error("the newest label is still useful for display")
	}
}
