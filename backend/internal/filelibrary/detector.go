package filelibrary

import "strings"

var fileIntentKeywords = []string{
	"file", "files", "document", "documents", "docs", "pdf", "spreadsheet", "excel", "word doc",
	"powerpoint", "presentation", "upload", "uploaded", "attached", "attachment", "library", "my files",
	"knowledge base", "source", "contract", "lease", "proposal", "deck", "guide", "manual", "transcript",
	".pdf", ".docx", ".pptx", ".xlsx", ".csv", ".txt", ".md", ".html", ".json", ".yaml", ".go", ".ts", ".tsx", ".py",
}

var forceFileGroundingPhrases = []string{
	"use only attached",
	"use only uploaded",
	"use only the attached",
	"use only the uploaded",
	"from my files",
	"answer from my file library",
	"based on the uploaded document",
	"answer using only the attached documents",
}

var compareVerbs = []string{"compare", "difference", "differences", "versus", "vs"}
var summarizeVerbs = []string{"summarize", "summary", "brief", "key points", "highlights"}
var fetchVerbs = []string{"find", "fetch", "open", "show", "retrieve"}

var nonFileSearchPatterns = []string{
	"write a file",
	"create a file",
	"create an excel file",
	"generate an excel file",
	"what is a pdf",
	"what is pdf",
}

// DetectFileIntent performs deterministic detection for when file search should run.
func DetectFileIntent(userMessage string, attachmentIDs []string) FileIntent {
	trimmed := strings.TrimSpace(userMessage)
	lower := strings.ToLower(trimmed)
	intent := FileIntent{
		SearchQuery: trimmed,
		Scope:       "auto",
	}

	if len(attachmentIDs) > 0 {
		intent.RequiresFileSearch = true
		intent.Confidence = 0.99
		intent.Reason = "attachments_present"
	}

	for _, phrase := range forceFileGroundingPhrases {
		if strings.Contains(lower, phrase) {
			intent.RequiresFileSearch = true
			intent.Confidence = maxFloat(intent.Confidence, 0.98)
			intent.Reason = "explicit_file_grounding_request"
		}
	}

	for _, pattern := range nonFileSearchPatterns {
		if strings.Contains(lower, pattern) {
			return FileIntent{
				RequiresFileSearch: false,
				SearchQuery:        trimmed,
				Scope:              "auto",
				Confidence:         0.05,
				Reason:             "non_retrieval_file_phrase",
			}
		}
	}

	keywordHits := 0
	for _, k := range fileIntentKeywords {
		if strings.Contains(lower, k) {
			keywordHits++
		}
	}

	intent.CompareIntent = containsAny(lower, compareVerbs)
	intent.SummarizeIntent = containsAny(lower, summarizeVerbs)
	intent.FetchSpecificFile = containsAny(lower, fetchVerbs) && (strings.Contains(lower, "uploaded") || strings.Contains(lower, "my files") || strings.Contains(lower, "attached"))

	if keywordHits >= 2 || intent.CompareIntent || intent.SummarizeIntent || intent.FetchSpecificFile {
		intent.RequiresFileSearch = true
		intent.Confidence = maxFloat(intent.Confidence, 0.80)
		if intent.Reason == "" {
			intent.Reason = "keyword_signal"
		}
	}

	if strings.Contains(lower, "news") && !strings.Contains(lower, "my files") && !strings.Contains(lower, "uploaded") && !strings.Contains(lower, "attached") {
		intent.RequiresFileSearch = false
		intent.Confidence = 0.20
		intent.Reason = "prefer_news_or_web_search"
	}

	if intent.RequiresFileSearch && intent.Confidence == 0 {
		intent.Confidence = 0.70
	}

	return intent
}

func containsAny(s string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
