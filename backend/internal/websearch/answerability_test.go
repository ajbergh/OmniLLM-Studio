package websearch

import "testing"

func result(index int, url string) SearchResult {
	return SearchResult{Index: index, URL: url, Title: "t", Snippet: "s", Source: extractDomain(url)}
}

// TestResultsLikelyAnswerableStopsBeingTrivial is the regression test for the
// dead-iteration bug: the old implementation returned true for every non-Direct
// shape as soon as one result existed, so searchWithPlan broke out of its loop
// after the first query no matter what MaxIterations said.
func TestResultsLikelyAnswerableRequiresCorroboration(t *testing.T) {
	plan := SearchPlan{
		AnswerShape:         AnswerShapeResearch,
		MinSources:          2,
		RequiredSourceClass: SourceClassPrimary,
		PreferredDomains:    []string{"swebench.com"},
	}

	one := []SearchResult{result(1, "https://swebench.com/leaderboard")}
	if ResultsLikelyAnswerable(plan, one) {
		t.Error("a single source must not satisfy a plan that requires two")
	}

	// Two pages on the same host are still one source.
	sameHost := []SearchResult{
		result(1, "https://swebench.com/leaderboard"),
		result(2, "https://swebench.com/about"),
	}
	if ResultsLikelyAnswerable(plan, sameHost) {
		t.Error("two pages from one host must count as one source")
	}

	two := []SearchResult{
		result(1, "https://swebench.com/leaderboard"),
		result(2, "https://arxiv.org/abs/1234"),
	}
	if !ResultsLikelyAnswerable(plan, two) {
		t.Error("two distinct hosts including a preferred domain should satisfy the plan")
	}
}

func TestResultsLikelyAnswerableRequiresAuthoritativeHost(t *testing.T) {
	plan := SearchPlan{
		AnswerShape:         AnswerShapeStandard,
		MinSources:          2,
		RequiredSourceClass: SourceClassOfficial,
		PreferredDomains:    []string{"anthropic.com", "openai.com"},
	}

	aggregatorsOnly := []SearchResult{
		result(1, "https://blog.example.com/llm-prices"),
		result(2, "https://news.example.org/ai-costs"),
	}
	if ResultsLikelyAnswerable(plan, aggregatorsOnly) {
		t.Error("aggregator coverage alone must not satisfy an official-source plan")
	}

	withOfficial := append(aggregatorsOnly, result(3, "https://docs.anthropic.com/en/docs/about-claude/pricing"))
	if !ResultsLikelyAnswerable(plan, withOfficial) {
		t.Error("a first-party subdomain should satisfy the official-source requirement")
	}
}

func TestResultsLikelyAnswerableEmpty(t *testing.T) {
	if ResultsLikelyAnswerable(SearchPlan{AnswerShape: AnswerShapeStandard}, nil) {
		t.Error("no results is never answerable")
	}
}

func TestResultsLikelyAnswerableDirectStillNeedsTheFact(t *testing.T) {
	plan := SearchPlan{AnswerShape: AnswerShapeDirect, Intent: SearchIntentSchedule}
	noTime := []SearchResult{{Index: 1, URL: "https://espn.com/x", Title: "Match preview", Snippet: "Coverage begins soon."}}
	if ResultsLikelyAnswerable(plan, noTime) {
		t.Error("a direct schedule lookup needs a clock time in the evidence")
	}
	withTime := []SearchResult{{Index: 1, URL: "https://espn.com/x", Title: "Kickoff 3:00 PM CDT", Snippet: ""}}
	if !ResultsLikelyAnswerable(plan, withTime) {
		t.Error("a clock time in the title should satisfy the direct lookup")
	}
}

// TestResultsLikelyAnswerableDefaultsAreLenient keeps the common path cheap:
// a plain current-information question should not pay for extra iterations.
func TestResultsLikelyAnswerableDefaultsAreLenient(t *testing.T) {
	plan := SearchPlan{AnswerShape: AnswerShapeStandard, MinSources: 1, RequiredSourceClass: SourceClassAny}
	if !ResultsLikelyAnswerable(plan, []SearchResult{result(1, "https://example.com/a")}) {
		t.Error("one result should satisfy a plan with no corroboration requirement")
	}
}

func TestMatchesDomainBoundary(t *testing.T) {
	domains := []string{"anthropic.com"}
	if !matchesDomain("anthropic.com", domains) {
		t.Error("exact host must match")
	}
	if !matchesDomain("docs.anthropic.com", domains) {
		t.Error("subdomain must match")
	}
	if matchesDomain("notanthropic.com", domains) {
		t.Error("suffix match must respect the domain boundary")
	}
	if matchesDomain("", domains) {
		t.Error("empty host must not match")
	}
}

func TestNormalizeHostFallsBackToSourceName(t *testing.T) {
	// Brave returns a display name for news results ("Example Wire"), which is
	// not a host and must not be treated as one.
	if got := normalizeHost(SearchResult{URL: "", Source: "Example Wire"}); got != "" {
		t.Errorf("display names must not be treated as hosts, got %q", got)
	}
	if got := normalizeHost(SearchResult{URL: "https://www.anthropic.com/pricing"}); got != "anthropic.com" {
		t.Errorf("normalizeHost = %q, want anthropic.com", got)
	}
}

func TestRankByPreferredDomains(t *testing.T) {
	results := []SearchResult{
		result(1, "https://blog.example.com/prices"),
		result(2, "https://openai.com/api/pricing"),
		result(3, "https://news.example.org/costs"),
		result(4, "https://docs.anthropic.com/pricing"),
	}
	ranked := rankByPreferredDomains(results, []string{"openai.com", "anthropic.com"})

	if len(ranked) != 4 {
		t.Fatalf("ranking must not drop results, got %d", len(ranked))
	}
	// Ranking, not filtering: every input survives.
	if ranked[0].URL != "https://openai.com/api/pricing" || ranked[1].URL != "https://docs.anthropic.com/pricing" {
		t.Errorf("preferred domains must come first, got %q then %q", ranked[0].URL, ranked[1].URL)
	}
	// Stable within groups.
	if ranked[2].URL != "https://blog.example.com/prices" || ranked[3].URL != "https://news.example.org/costs" {
		t.Errorf("non-preferred order must be preserved, got %q then %q", ranked[2].URL, ranked[3].URL)
	}
	// Indexes are citation markers and must be renumbered contiguously.
	for i, r := range ranked {
		if r.Index != i+1 {
			t.Errorf("result %d has Index %d, want %d", i, r.Index, i+1)
		}
	}
}

func TestRankByPreferredDomainsNoPreference(t *testing.T) {
	results := []SearchResult{result(7, "https://a.test"), result(9, "https://b.test")}
	ranked := rankByPreferredDomains(results, nil)
	if ranked[0].URL != "https://a.test" || ranked[1].URL != "https://b.test" {
		t.Error("no preference must preserve order")
	}
	if ranked[0].Index != 1 || ranked[1].Index != 2 {
		t.Error("indexes must still be normalized")
	}
}
