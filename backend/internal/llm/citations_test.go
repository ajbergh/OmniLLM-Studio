package llm

import "testing"

func TestNormalizeCitations(t *testing.T) {
	in := []Citation{
		{URL: "https://a.test", Title: "A"},
		{URL: "https://a.test", Title: "A duplicate"},
		{URL: "  ", Title: "no url"},
		{URL: "https://b.test"},
	}
	got := NormalizeCitations(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 citations after de-duplication, got %d: %#v", len(got), got)
	}
	if got[0].URL != "https://a.test" || got[1].URL != "https://b.test" {
		t.Errorf("first-seen order must be preserved: %#v", got)
	}
	// A missing title falls back to the URL so the UI always has a label.
	if got[1].Title != "https://b.test" {
		t.Errorf("Title = %q, want the URL as a fallback", got[1].Title)
	}
}

func TestNormalizeCitationsEmpty(t *testing.T) {
	if got := NormalizeCitations(nil); got != nil {
		t.Errorf("nil in, nil out; got %#v", got)
	}
	if got := NormalizeCitations([]Citation{{URL: "   "}}); got != nil {
		t.Errorf("entries with no URL are dropped entirely; got %#v", got)
	}
}

func TestMergeCitationsAcrossStreamChunks(t *testing.T) {
	// Native grounding annotations can arrive in several deltas, so merging must
	// be idempotent rather than appending duplicates.
	acc := MergeCitations(nil, []Citation{{URL: "https://a.test", Title: "A"}})
	acc = MergeCitations(acc, []Citation{{URL: "https://a.test", Title: "A again"}})
	acc = MergeCitations(acc, []Citation{{URL: "https://b.test", Title: "B"}})
	if len(acc) != 2 {
		t.Fatalf("expected 2 merged citations, got %d: %#v", len(acc), acc)
	}
	if acc[0].Title != "A" {
		t.Errorf("the first title seen wins, got %q", acc[0].Title)
	}
	if got := MergeCitations(acc, nil); len(got) != 2 {
		t.Error("merging nothing must not change the accumulator")
	}
}
