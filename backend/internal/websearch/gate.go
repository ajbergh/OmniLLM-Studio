package websearch

import (
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Deterministic keyword gate  (Rule 1 – always evaluated first)
// ---------------------------------------------------------------------------

// Each trigger carries a weight; total score must reach the threshold.
type weightedPattern struct {
	re     *regexp.Regexp
	weight int
}

const searchScoreThreshold = 2

// triggerPatterns are compiled once; each matches case-insensitively.
// Weight 2 = strong signal (triggers alone), weight 1 = weak (needs a second signal).
var triggerPatterns = []weightedPattern{
	// Temporal signals — strong
	{regexp.MustCompile(`(?i)\b(today|tonight|right now|just now|this morning|this evening)\b`), 2},
	{regexp.MustCompile(`(?i)\b(latest|breaking|live|real-?time)\b`), 2},
	{regexp.MustCompile(`(?i)\b(this week|this month|yesterday|last night)\b`), 2},
	{regexp.MustCompile(`(?i)\b(just happened|happening now|ongoing)\b`), 2},

	// "current" only strong when paired with time-sensitive nouns
	{regexp.MustCompile(`(?i)\bcurrent\s+(price|news|status|score|weather|standings|events|situation|market)\b`), 2},
	// bare "current" or "recent" = weak signal
	{regexp.MustCompile(`(?i)\b(current|recent)\b`), 1},

	// Year references — weak signal (many coding/math contexts mention years)
	{regexp.MustCompile(`(?i)\b(20[2-3]\d)\b`), 1},

	// News / events — strong
	{regexp.MustCompile(`(?i)\b(news|headlines|breaking news|announcement)\b`), 2},
	{regexp.MustCompile(`(?i)\b(current events|world events|trending|viral)\b`), 2},
	{regexp.MustCompile(`(?i)\b(released|launched|unveiled|introduced|announced)\b`), 1},

	// Verification / sourcing — strong
	{regexp.MustCompile(`(?i)\b(verify|fact.?check|is it true)\b`), 2},
	{regexp.MustCompile(`(?i)\b(look ?up|search for|find me)\b`), 2},

	// Scores / weather / stocks (inherently real-time) — strong
	{regexp.MustCompile(`(?i)\b(score|scores|standings|results|weather|forecast|stock price|market)\b`), 2},
	{regexp.MustCompile(`(?i)\b(price of|how much (does|is|do)|cost of|pricing)\b`), 2},

	// Research / comparison — weak (could be general knowledge)
	{regexp.MustCompile(`(?i)\b(who won|who is winning|who leads|election)\b`), 2},
	{regexp.MustCompile(`(?i)\b(best .{1,30} (for|in|of|under)|top \d+)\b`), 1},
	{regexp.MustCompile(`(?i)\b(vs\.?|versus|compared? to)\b`), 1},

	// How-to buy/get — strong (implies real-world product search)
	{regexp.MustCompile(`(?i)\b(how to (buy|get)|where (to|can I) (buy|find|get))\b`), 2},

	// Explicit search intent — strong
	{regexp.MustCompile(`(?i)\b(according to|what does .{1,20} say|official.?(site|website|page))\b`), 2},

	// Wh-questions ending in ? — weak signal (must combine with other signal)
	// Excludes common programming/knowledge questions via negative patterns below
	{regexp.MustCompile(`(?i)^(what|when|where|who|why|how)\b.{0,80}\?$`), 1},
}

// negativePatternsExclude — if any match, suppress web search entirely.
// These catch programming, math, and general knowledge questions.
var negativePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(code|coding|function|method|class|variable|loop|array|pointer|struct|interface|error|bug|debug|compile|syntax|algorithm|recursion|regex|import|module|package|library|framework)\b`),
	regexp.MustCompile(`(?i)\b(implement|refactor|optimize|migrate|deploy|lint|test|mock|stub|fixture)\b`),
	regexp.MustCompile(`(?i)\b(html|css|javascript|typescript|python|golang|rust|java|sql|react|vue|angular|docker|kubernetes|git|npm|pip|cargo)\b`),
	regexp.MustCompile(`(?i)\b(explain|definition of|what is a |what are |meaning of|difference between .{1,30} and)\b.*\b(in programming|in (computer )?science|in math|in code)\b`),
}

// ShouldWebSearch applies deterministic rules with scoring to decide whether
// a user message should trigger a web search. Returns the decision and a
// pre-built ToolCall ready to execute.
func ShouldWebSearch(userText string, now time.Time, tz string) (bool, *ToolCall) {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return false, nil
	}

	// Check negative patterns first — if it's clearly a programming/knowledge question, skip
	for _, neg := range negativePatterns {
		if neg.MatchString(lower) {
			return false, nil
		}
	}

	// Score trigger patterns
	score := 0
	for _, wp := range triggerPatterns {
		if wp.re.MatchString(lower) {
			score += wp.weight
			if score >= searchScoreThreshold {
				break
			}
		}
	}

	if score < searchScoreThreshold {
		return false, nil
	}

	query := buildSearchQuery(userText)
	timeRange := inferTimeRange(lower)

	tc := &ToolCall{
		Name: "web_search",
		Arguments: SearchRequest{
			Query:      query,
			TimeRange:  timeRange,
			Region:     "US",
			Locale:     "en-US",
			MaxResults: 10,
		},
	}
	return true, tc
}

// buildSearchQuery cleans up the user text into a reasonable search query.
func buildSearchQuery(userText string) string {
	q := strings.TrimSpace(userText)

	// Remove question marks and trailing punctuation for cleaner queries
	q = strings.TrimRight(q, "?!.")

	// Remove leading conversational filler to tighten the query.
	prefixes := []string{
		"what is ", "what are ", "what's ", "what was ", "what were ",
		"who is ", "who are ", "who was ",
		"when is ", "when was ", "when did ",
		"where is ", "where are ", "where can i ",
		"why is ", "why are ", "why did ",
		"how is ", "how are ", "how did ", "how does ", "how do ",
		"tell me about ", "tell me ", "can you tell me ",
		"search for ", "look up ", "find me ", "find ",
		"give me ", "show me ", "i want to know about ",
		"i need to know ", "can you find ", "please search ",
		"could you look up ", "do you know ",
	}
	lower := strings.ToLower(q)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			q = q[len(p):]
			break
		}
	}

	return strings.TrimSpace(q)
}

// inferTimeRange picks a sensible default time range from keywords.
func inferTimeRange(lower string) string {
	switch {
	case containsAny(lower, "today", "tonight", "this morning", "this evening",
		"right now", "just now", "just happened", "happening now", "breaking",
		"live", "real-time", "realtime"):
		return "24h"
	case containsAny(lower, "this week", "last night", "yesterday", "recent"):
		return "7d"
	case containsAny(lower, "this month"):
		return "30d"
	default:
		return "24h"
	}
}

func containsAny(s string, terms ...string) bool {
	for _, t := range terms {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}
