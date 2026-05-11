package filelibrary

import "testing"

func TestDetectFileIntent_WithAttachmentsForcesSearch(t *testing.T) {
	intent := DetectFileIntent("hello", []string{"att-1"})
	if !intent.RequiresFileSearch {
		t.Fatal("expected file search to be required when attachments are present")
	}
	if intent.Confidence < 0.95 {
		t.Fatalf("expected high confidence, got %f", intent.Confidence)
	}
}

func TestDetectFileIntent_ExplicitGroundingPhrase(t *testing.T) {
	intent := DetectFileIntent("Answer using only the attached documents.", nil)
	if !intent.RequiresFileSearch {
		t.Fatal("expected file search for explicit grounding phrase")
	}
	if intent.Reason != "explicit_file_grounding_request" {
		t.Fatalf("unexpected reason: %s", intent.Reason)
	}
}

func TestDetectFileIntent_FileQuestionTriggersSearch(t *testing.T) {
	intent := DetectFileIntent("What does the uploaded PDF say about retention?", nil)
	if !intent.RequiresFileSearch {
		t.Fatal("expected file search for uploaded PDF question")
	}
	if intent.Confidence < 0.8 {
		t.Fatalf("expected confidence >= 0.8, got %f", intent.Confidence)
	}
}

func TestDetectFileIntent_NonRetrievalPhraseDoesNotTrigger(t *testing.T) {
	intent := DetectFileIntent("Create an Excel file that tracks issues", nil)
	if intent.RequiresFileSearch {
		t.Fatal("expected no file search for file creation request")
	}
}

func TestDetectFileIntent_NewsPromptPrefersNewsSearch(t *testing.T) {
	intent := DetectFileIntent("Find the latest news about AI chip regulation", nil)
	if intent.RequiresFileSearch {
		t.Fatal("expected no file search for general news prompt")
	}
	if intent.Reason != "prefer_news_or_web_search" {
		t.Fatalf("unexpected reason: %s", intent.Reason)
	}
}

func TestDetectFileIntent_CompareAndSummarizeSignals(t *testing.T) {
	intent := DetectFileIntent("Compare these uploaded files and summarize the risks", nil)
	if !intent.RequiresFileSearch {
		t.Fatal("expected file search for compare/summarize request")
	}
	if !intent.CompareIntent {
		t.Fatal("expected compare intent")
	}
	if !intent.SummarizeIntent {
		t.Fatal("expected summarize intent")
	}
}
