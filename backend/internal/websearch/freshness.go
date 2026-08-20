package websearch

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// relativeAgePattern matches Brave's free-form age strings, e.g. "2 hours ago",
// "3 days ago", "1 month ago".
var relativeAgePattern = regexp.MustCompile(`(?i)^\s*(\d+)\s*(second|minute|hour|day|week|month|year)s?\s+ago\s*$`)

// absoluteAgeLayouts covers the formats Brave uses for page_age and the formats
// providers commonly return for publication dates.
var absoluteAgeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"January 2, 2006",
	"Jan 2, 2006",
	"02 Jan 2006",
}

// ParsePublishedAt converts a provider's publication string into a timestamp.
//
// Brave returns two shapes for the same concept: an ISO timestamp in page_age
// and a human phrase like "2 hours ago" in age. Both were previously stored
// verbatim in SearchResult.PublishedAt and never parsed, so the application held
// freshness data it could not act on — it could ask a provider to filter by
// recency but could not verify that the results actually were recent.
//
// The second return value is false when the string carries no usable date, which
// is a distinct state from "old": an undated result must not be presented as
// current, but neither can it be rejected as stale.
func ParsePublishedAt(value string, now time.Time) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	if m := relativeAgePattern.FindStringSubmatch(value); m != nil {
		amount, err := strconv.Atoi(m[1])
		if err != nil || amount < 0 {
			return time.Time{}, false
		}
		var delta time.Duration
		switch strings.ToLower(m[2]) {
		case "second":
			delta = time.Duration(amount) * time.Second
		case "minute":
			delta = time.Duration(amount) * time.Minute
		case "hour":
			delta = time.Duration(amount) * time.Hour
		case "day":
			delta = time.Duration(amount) * 24 * time.Hour
		case "week":
			delta = time.Duration(amount) * 7 * 24 * time.Hour
		case "month":
			// Calendar months vary; 30 days is close enough for a freshness
			// window expressed in days, and erring long avoids rejecting a
			// borderline-fresh source.
			delta = time.Duration(amount) * 30 * 24 * time.Hour
		case "year":
			delta = time.Duration(amount) * 365 * 24 * time.Hour
		default:
			return time.Time{}, false
		}
		return now.Add(-delta), true
	}

	for _, layout := range absoluteAgeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// TimeRangeDuration converts a plan's freshness window into a duration. The
// second return value is false for an empty window, which means "no constraint"
// rather than "zero tolerance".
func TimeRangeDuration(timeRange string) (time.Duration, bool) {
	switch strings.TrimSpace(timeRange) {
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

// FreshnessReport describes how well a result set matches a plan's window.
type FreshnessReport struct {
	// Dated is the number of results with a parsable publication date.
	Dated int
	// Undated is the number without one. These are neither fresh nor stale.
	Undated int
	// WithinWindow counts dated results inside the requested window.
	WithinWindow int
	// Newest is the most recent parsable publication date, zero if none.
	Newest time.Time
	// NewestLabel is the provider's original string for the newest result, kept
	// verbatim for display.
	NewestLabel string
	// Verified is true when a window was requested, at least one result carried a
	// date, and every dated result falls inside it.
	Verified bool
}

// EvaluateFreshness measures a result set against a plan's freshness window.
//
// Verification is deliberately conservative. It requires at least one dated
// result, because a set of entirely undated results proves nothing; and it
// requires every dated result to be inside the window, because a single stale
// source among fresh ones is exactly the case where a model quotes the wrong
// number.
func EvaluateFreshness(plan SearchPlan, results []SearchResult, now time.Time) FreshnessReport {
	report := FreshnessReport{}
	window, hasWindow := TimeRangeDuration(plan.TimeRange)
	cutoff := now.Add(-window)

	allDatedInWindow := true
	for _, result := range results {
		published, ok := ParsePublishedAt(result.PublishedAt, now)
		if !ok {
			report.Undated++
			continue
		}
		report.Dated++
		if published.After(report.Newest) {
			report.Newest = published
			report.NewestLabel = strings.TrimSpace(result.PublishedAt)
		}
		if !hasWindow {
			continue
		}
		if published.Before(cutoff) {
			allDatedInWindow = false
			continue
		}
		report.WithinWindow++
	}

	report.Verified = hasWindow && report.Dated > 0 && allDatedInWindow
	return report
}
