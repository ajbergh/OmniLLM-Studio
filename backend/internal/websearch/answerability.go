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

func ResultsLikelyAnswerable(plan SearchPlan, results []SearchResult) bool {
	if len(results) == 0 {
		return false
	}
	if plan.AnswerShape != AnswerShapeDirect {
		return true
	}
	for _, result := range results {
		text := result.Title + " " + result.Snippet
		if clockTimePattern.MatchString(text) {
			return true
		}
	}
	return false
}
