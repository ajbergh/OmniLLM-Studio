package urlcontext

import "strings"

// triggerPhrases are patterns that indicate the user wants the URL to be read.
var triggerPhrases = []string{
	"review", "analyze", "analyse", "summarize", "summarise",
	"read", "inspect", "look at", "check out",
	"what does this say", "what is missing", "features missing",
	"what should be added", "what's missing", "whats missing",
	"compare", "explain this", "explain the",
	"is this accurate", "is this good", "audit",
	"evaluate", "tell me about this", "tell me about the",
	"what are the risks", "how would you improve",
	"turn this into", "create a plan based on",
	"based on this", "based on the", "from this link",
	"using this link", "from this url", "using this url",
	"in this repo", "in this repository", "this project",
	"what features", "what's in this", "whats in this",
	"security review", "code review", "architecture",
	"give me a", "give a", "provide a", "write a review",
}

// nonLookupPhrases indicate the user is just sharing a URL without asking for analysis.
var nonLookupPhrases = []string{
	"open ", "bookmark ", "save this", "this is my website",
	"visit ", "go to ", "navigate to ", "click on ",
}

// goalKeywords map trigger phrases to specific AnalysisGoals.
var goalKeywords = []struct {
	phrases []string
	goal    AnalysisGoal
}{
	{
		phrases: []string{"feature", "missing", "gap", "what should be added", "what could be added", "what's missing"},
		goal:    GoalFeatureGapReview,
	},
	{
		phrases: []string{"architecture", "system design", "design", "structure"},
		goal:    GoalArchitectureReview,
	},
	{
		phrases: []string{"security", "vulnerability", "vulnerabilities", "risk", "risks", "audit", "pentest"},
		goal:    GoalSecurityReview,
	},
	{
		phrases: []string{"code review", "review the code", "review this code", "code quality"},
		goal:    GoalCodeReview,
	},
	{
		phrases: []string{"summarize", "summarise", "summary", "what does this say", "tldr", "tl;dr"},
		goal:    GoalSummarize,
	},
	{
		phrases: []string{"explain", "what is", "what's", "how does"},
		goal:    GoalExplain,
	},
	{
		phrases: []string{"compare", "difference", "vs ", "versus"},
		goal:    GoalCompare,
	},
}

// RequiresURLContext returns (true, goal) when the message appears to require
// reading the linked URL(s). It uses deterministic heuristics only — no LLM.
func RequiresURLContext(message string, urls []string, forceOnURL bool) (bool, AnalysisGoal) {
	if len(urls) == 0 {
		return false, GoalUnknown
	}

	lower := strings.ToLower(message)

	// Non-lookup short-circuit: user is just sharing a link, not asking for analysis.
	for _, phrase := range nonLookupPhrases {
		if strings.Contains(lower, phrase) {
			return false, GoalUnknown
		}
	}

	// Check for analysis trigger phrases.
	triggered := false
	for _, phrase := range triggerPhrases {
		if strings.Contains(lower, phrase) {
			triggered = true
			break
		}
	}

	// When forceOnURL is enabled, also trigger if the message contains a
	// question or substantive request alongside a URL.
	if !triggered && forceOnURL {
		if strings.Contains(lower, "?") || strings.Contains(lower, "please") ||
			strings.Contains(lower, "can you") || strings.Contains(lower, "could you") ||
			strings.Contains(lower, "what ") || strings.Contains(lower, "how ") ||
			strings.Contains(lower, "should ") || strings.Contains(lower, "would ") ||
			strings.Contains(lower, "list ") || strings.Contains(lower, "show ") {
			triggered = true
		}
	}

	if !triggered {
		return false, GoalUnknown
	}

	// Map to analysis goal.
	goal := detectGoal(lower)
	return true, goal
}

// detectGoal maps the message to the most specific AnalysisGoal.
func detectGoal(lower string) AnalysisGoal {
	for _, entry := range goalKeywords {
		for _, phrase := range entry.phrases {
			if strings.Contains(lower, phrase) {
				return entry.goal
			}
		}
	}
	// Generic review / explain fallbacks.
	if strings.Contains(lower, "review") {
		return GoalReview
	}
	return GoalSummarize
}
