package websearch

import (
	"regexp"
	"strings"
)

var clockTimePattern = regexp.MustCompile(`(?i)\b(?:[01]?\d|2[0-3])(?::[0-5]\d)?\s*(?:a\.?m\.?|p\.?m\.?)\b|\b(?:[01]?\d|2[0-3]):[0-5]\d\b`)

func ValidateAnswer(plan SearchPlan, content string) (bool, string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false, "empty_answer"
	}
	if plan.AnswerShape != AnswerShapeDirect {
		return true, ""
	}
	lower := strings.ToLower(trimmed)
	for _, phrase := range []string{
		"how to check",
		"consult the schedule",
		"you should consult",
		"visit the official",
		"key takeaways",
		"to determine the specific",
	} {
		if strings.Contains(lower, phrase) {
			return false, "indirect_answer"
		}
	}
	if plan.Intent == SearchIntentSchedule && !clockTimePattern.MatchString(trimmed) {
		return false, "missing_start_time"
	}
	if len(strings.Fields(trimmed)) > 120 {
		return false, "direct_answer_too_long"
	}
	return true, ""
}

// ResultsLikelyAnswerable reports whether the evidence gathered so far is enough
// to stop searching.
//
// The previous implementation returned true for every non-Direct answer shape as
// soon as a single result existed. Combined with searchWithPlan breaking on that
// signal, it meant research and comparison plans always performed exactly one
// search no matter what MaxIterations said. The check is now shape-aware:
// corroboration and source class are what stop the loop, not a non-zero count.
func ResultsLikelyAnswerable(plan SearchPlan, results []SearchResult) bool {
	if len(results) == 0 {
		return false
	}

	// Direct answers need the specific fact visible in a title or snippet.
	if plan.AnswerShape == AnswerShapeDirect {
		for _, result := range results {
			if clockTimePattern.MatchString(result.Title + " " + result.Snippet) {
				return true
			}
		}
		return false
	}

	minSources := plan.MinSources
	if minSources <= 0 {
		minSources = 1
	}
	if distinctSourceCount(results) < minSources {
		return false
	}

	// A plan that names authoritative hosts is not satisfied by aggregator
	// coverage alone; keep searching while budget remains.
	if plan.RequiredSourceClass == SourceClassOfficial || plan.RequiredSourceClass == SourceClassPrimary {
		if len(plan.PreferredDomains) > 0 && !hasPreferredDomain(results, plan.PreferredDomains) {
			return false
		}
		if len(plan.AllowedDomains) > 0 && !hasPreferredDomain(results, plan.AllowedDomains) {
			return false
		}
	}

	return true
}

// distinctSourceCount counts unique hosts rather than unique URLs. Five pages
// from one vendor are one source for corroboration purposes.
func distinctSourceCount(results []SearchResult) int {
	hosts := make(map[string]struct{}, len(results))
	for _, result := range results {
		host := normalizeHost(result)
		if host == "" {
			continue
		}
		hosts[host] = struct{}{}
	}
	return len(hosts)
}

// hasPreferredDomain reports whether any result is hosted on (or under) one of
// the named domains.
func hasPreferredDomain(results []SearchResult, domains []string) bool {
	for _, result := range results {
		if matchesDomain(normalizeHost(result), domains) {
			return true
		}
	}
	return false
}

// matchesDomain performs suffix matching on a domain boundary so that
// "docs.anthropic.com" matches "anthropic.com" while "notanthropic.com" does not.
func matchesDomain(host string, domains []string) bool {
	if host == "" {
		return false
	}
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// normalizeHost derives a comparable host from a result. Source is provider
// metadata and may be a display name ("Example Wire"), so the URL is preferred.
func normalizeHost(result SearchResult) string {
	host := strings.ToLower(strings.TrimSpace(extractDomain(result.URL)))
	if host == "" || strings.Contains(host, " ") {
		host = strings.ToLower(strings.TrimSpace(result.Source))
	}
	if strings.Contains(host, " ") {
		return ""
	}
	return strings.TrimPrefix(host, "www.")
}
