package websearch

import "testing"

var claimSources = []SearchResult{
	{Index: 1, URL: "https://www.anthropic.com/pricing", Title: "Pricing"},
	{Index: 2, URL: "https://openai.com/api/pricing", Title: "Pricing"},
}

// TestAuditAnswerFlagsUncitedNumericClaims covers the high-confidence case the
// audit is designed for: the answer states figures that go stale and names no
// source at all. This is the shape of the reviewed failure — precise-looking
// prices and benchmark percentages with nothing behind them.
func TestAuditAnswerFlagsUncitedNumericClaims(t *testing.T) {
	answer := "Claude costs $3 per million input tokens and scores 72% on SWE-bench, " +
		"while GPT-5 is $1.25 and scores 74.9%."
	audit := AuditAnswer(answer, claimSources)

	if audit.NumericClaims == 0 {
		t.Fatal("prices and percentages must register as numeric claims")
	}
	if !audit.UncitedNumericClaims {
		t.Error("numeric claims with no citation and no hedge must be flagged")
	}
	if audit.Hedged {
		t.Error("this answer hedges nothing")
	}
}

func TestAuditAnswerAcceptsInlineMarkers(t *testing.T) {
	answer := "Claude costs $3 per million input tokens [1], and GPT-5 costs $1.25 [2]."
	audit := AuditAnswer(answer, claimSources)
	if audit.CitationMarkers < 2 {
		t.Errorf("CitationMarkers = %d, want at least 2", audit.CitationMarkers)
	}
	if audit.UncitedNumericClaims {
		t.Error("inline markers must clear the warning")
	}
}

func TestAuditAnswerAcceptsNamedHosts(t *testing.T) {
	answer := "According to anthropic.com the price is $3 per million input tokens."
	audit := AuditAnswer(answer, claimSources)
	if audit.CitedSourceHosts != 1 {
		t.Errorf("CitedSourceHosts = %d, want 1", audit.CitedSourceHosts)
	}
	if audit.UncitedNumericClaims {
		t.Error("naming an evidence host must clear the warning")
	}
}

// TestAuditAnswerAcceptsHedging is the counterpart to the degraded path: an
// answer that admits it could not verify must not also be flagged for failing to
// cite, or the two signals would contradict each other.
func TestAuditAnswerAcceptsHedging(t *testing.T) {
	for _, answer := range []string{
		"I could not verify current pricing. As of my training data it was around $3 per million tokens, which may have changed.",
		"The evidence does not cover GPT-5 pricing, so I cannot confirm the $1.25 figure.",
		"This is unverified: roughly 72% on SWE-bench.",
	} {
		audit := AuditAnswer(answer, claimSources)
		if !audit.Hedged {
			t.Errorf("expected a hedge in %q", answer)
		}
		if audit.UncitedNumericClaims {
			t.Errorf("a hedged answer must not be flagged as uncited: %q", answer)
		}
	}
}

func TestAuditAnswerNoNumericClaims(t *testing.T) {
	audit := AuditAnswer("Claude and GPT-5 are both strong at coding tasks.", claimSources)
	if audit.NumericClaims != 0 {
		t.Errorf("NumericClaims = %d, want 0", audit.NumericClaims)
	}
	if audit.UncitedNumericClaims {
		t.Error("an answer with no numeric claims cannot be uncited")
	}
}

func TestAuditAnswerVersionClaims(t *testing.T) {
	audit := AuditAnswer("The current release is 1.29.4.", nil)
	if audit.NumericClaims == 0 {
		t.Error("version numbers must register as numeric claims")
	}
	if !audit.UncitedNumericClaims {
		t.Error("an uncited version claim must be flagged")
	}
}

func TestAuditAnswerEmpty(t *testing.T) {
	audit := AuditAnswer("   ", claimSources)
	if audit.NumericClaims != 0 || audit.UncitedNumericClaims || audit.Hedged {
		t.Errorf("an empty answer must produce a zero audit, got %+v", audit)
	}
}

// TestAuditAnswerIgnoresNonEvidenceHosts guards against a false positive: the
// answer must be credited only for hosts that are actually in the evidence.
func TestAuditAnswerIgnoresNonEvidenceHosts(t *testing.T) {
	answer := "According to example.com the price is $3 per million tokens."
	audit := AuditAnswer(answer, claimSources)
	if audit.CitedSourceHosts != 0 {
		t.Errorf("CitedSourceHosts = %d; a host absent from the evidence is not a citation", audit.CitedSourceHosts)
	}
	if !audit.UncitedNumericClaims {
		t.Error("naming an unrelated host must not clear the warning")
	}
}
