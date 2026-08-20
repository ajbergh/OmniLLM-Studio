package websearch

import (
	"regexp"
	"strings"
)

// AnswerAudit reports how well an answer's factual claims are backed by the
// evidence that was retrieved for it.
//
// This is deliberately a *signal*, not a verdict. "Claims are supported by
// sources" is not decidable by string matching: an answer can restate a price
// correctly in different words, and it can cite a source that does not actually
// say what the answer says. An over-eager validator that rejected correct
// answers would be worse than today's permissive one, which accepts any
// non-empty string outside the sports schedule path.
//
// So the audit records what it found and lets the caller decide. Until the
// false-positive rate is measured against an evaluation set, callers should
// surface these fields as a warning and must not use them to reject an answer.
type AnswerAudit struct {
	// NumericClaims counts distinct numeric assertions of the kind that go stale:
	// currency amounts, percentages, and version numbers.
	NumericClaims int
	// CitationMarkers counts inline references to evidence indexes.
	CitationMarkers int
	// CitedSourceHosts counts evidence hosts the answer names directly.
	CitedSourceHosts int
	// UncitedNumericClaims is true when the answer makes numeric claims and names
	// no source at all. This is the high-confidence case: not "the citation is
	// wrong" but "there is no citation".
	UncitedNumericClaims bool
	// Hedged is true when the answer acknowledges uncertainty or missing
	// evidence, which is the behaviour the degraded path asks for.
	Hedged bool
}

var (
	// currencyClaimPattern matches "$3", "$0.25", "3 USD", "€10".
	currencyClaimPattern = regexp.MustCompile(`(?i)([$€£¥]\s?\d[\d,.]*)|(\b\d[\d,.]*\s?(usd|eur|gbp|dollars?|cents?)\b)`)
	// percentClaimPattern matches "72%", "72.4 percent".
	percentClaimPattern = regexp.MustCompile(`(?i)\b\d[\d,.]*\s?(%|percent)\b`)
	// versionClaimPattern matches "v1.2", "1.29.4", "React 19".
	versionClaimPattern = regexp.MustCompile(`(?i)\b(v\s?\d+(\.\d+){1,3}|\d+\.\d+(\.\d+){1,2})\b`)
	// citationMarkerPattern matches inline evidence references: [1], [3], (2).
	citationMarkerPattern = regexp.MustCompile(`[\[(]\s?\d{1,2}\s?[\])]`)
	// hedgePattern matches the acknowledgements the degraded and
	// insufficient-evidence directives ask for.
	hedgePattern = regexp.MustCompile(`(?i)\b(could not verify|couldn't verify|unable to verify|not verified|unverified|may (be|not be) (out of date|current|up to date)|no (source|evidence)|the evidence does not|not covered by the evidence|as of my training|may have changed)\b`)
)

// AuditAnswer inspects an answer against the evidence retrieved for it.
func AuditAnswer(content string, results []SearchResult) AnswerAudit {
	audit := AnswerAudit{}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return audit
	}

	audit.NumericClaims = countMatches(trimmed,
		currencyClaimPattern, percentClaimPattern, versionClaimPattern)
	audit.CitationMarkers = len(citationMarkerPattern.FindAllString(trimmed, -1))
	audit.Hedged = hedgePattern.MatchString(trimmed)

	lower := strings.ToLower(trimmed)
	seenHosts := map[string]struct{}{}
	for _, result := range results {
		host := normalizeHost(result)
		if host == "" {
			continue
		}
		if _, dup := seenHosts[host]; dup {
			continue
		}
		if strings.Contains(lower, host) {
			seenHosts[host] = struct{}{}
		}
	}
	audit.CitedSourceHosts = len(seenHosts)

	audit.UncitedNumericClaims = audit.NumericClaims > 0 &&
		audit.CitationMarkers == 0 &&
		audit.CitedSourceHosts == 0 &&
		!audit.Hedged

	return audit
}

// countMatches sums non-overlapping matches across several patterns.
func countMatches(text string, patterns ...*regexp.Regexp) int {
	total := 0
	for _, pattern := range patterns {
		total += len(pattern.FindAllString(text, -1))
	}
	return total
}
