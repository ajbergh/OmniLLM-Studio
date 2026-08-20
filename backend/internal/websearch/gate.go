package websearch

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Deterministic keyword gate  (Rule 1 – always evaluated first)
// ---------------------------------------------------------------------------

// Each trigger carries a weight; the net score must reach the threshold.
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
	{regexp.MustCompile(`(?i)\b(vs\.?|versus|compared? to|comparison)\b`), 1},

	// Availability and options — weak. Present so that a comparison question with
	// no explicit temporal word can still reach the threshold.
	{regexp.MustCompile(`(?i)\b(available|availability|alternatives|options)\b`), 1},

	// Benchmarks and leaderboards — strong. These change continuously and are a
	// frequent source of confidently stale answers.
	{regexp.MustCompile(`(?i)\b(benchmarks?|leaderboards?|sota|state of the art)\b`), 2},

	// Explicit research requests — strong. The planner has a research answer
	// shape keyed on these phrases, but it was unreachable: the gate had no
	// matching trigger, so "give me a comprehensive investigation of X" scored
	// zero and never reached the planner at all.
	{regexp.MustCompile(`(?i)\b(deep research|comprehensive (analysis|investigation|overview|report|guide|comparison)|detailed analysis|investigate|report on|research (the|current|available|what))\b`), 2},

	// How-to buy/get — strong (implies real-world product search)
	{regexp.MustCompile(`(?i)\b(how to (buy|get)|where (to|can I) (buy|find|get))\b`), 2},

	// Explicit search intent — strong
	{regexp.MustCompile(`(?i)\b(according to|what does .{1,20} say|official.?(site|website|page))\b`), 2},

	// Wh-questions ending in ? — weak signal (must combine with other signal)
	// Excludes common programming/knowledge questions via negative patterns below
	{regexp.MustCompile(`(?i)^(what|when|where|who|why|how|which)\b.{0,80}\?$`), 1},
}

// decisivePatterns short-circuit the score. A prompt that explicitly asks for
// the newest state of something needs retrieval regardless of what the question
// is *about*.
//
// This exists because subject-matter suppression used to win outright: any
// prompt containing "coding", "react", "docker", "npm" and so on was refused a
// web search before scoring even ran. That conflates topic with temporal need.
// "What is the latest React version" is a current-information question that
// happens to be about software.
//
// Membership here is deliberately narrow: an explicit recency word, or "current"
// bound to a noun that actually changes. Bare "current" and "current state of"
// stay out, because they are as often rhetorical as temporal.
var decisivePatterns = []*regexp.Regexp{
	// "newer" is included alongside the superlatives: "is there a newer X than Y"
	// is a pure current-information question and cannot be answered from a
	// training cutoff.
	regexp.MustCompile(`(?i)\b(latest|newest|newer|most recent)\b`),
	regexp.MustCompile(`(?i)\b(today|tonight|right now|just now|this morning|this evening|as of (today|now))\b`),
	regexp.MustCompile(`(?i)\b(this week|this month|yesterday|last night)\b`),
	regexp.MustCompile(`(?i)\b(breaking|happening now|just (released|launched|announced|happened|shipped))\b`),
	regexp.MustCompile(`(?i)\breal-?time\b`),
	regexp.MustCompile(`(?i)\blive (scores?|results?|updates?|stream|standings)\b`),
	// "current" plus a noun that genuinely moves. Up to three intervening words
	// so "current Kubernetes patch release" and "current GPT-5 API pricing" match.
	regexp.MustCompile(`(?i)\bcurrent\s+([a-z0-9.+#/-]+\s+){0,3}(versions?|releases?|pricing|prices?|costs?|rates?|scores?|standings|status|news|weather|models?|lineup|leaderboard)\b`),
	// The same idea inverted: "the X pricing right now".
	regexp.MustCompile(`(?i)\b(versions?|releases?|pricing|prices?|costs?|models?)\b[^?.!]{0,40}\b(right now|at the moment|as of today)\b`),
}

// hardSuppressPatterns refuse retrieval outright. Unlike the subtractive
// negatives below, these outrank decisivePatterns, so the list is limited to
// cases where the user is unambiguously asking about the mechanics of code
// rather than the state of the world.
var hardSuppressPatterns = []*regexp.Regexp{
	// A fenced code block means the subject is the pasted code.
	regexp.MustCompile("```"),
	// Conceptual explanation scoped to a technical field.
	regexp.MustCompile(`(?i)\b(explain|definition of|what is a |what are |meaning of|difference between .{1,30} and)\b.*\b(in programming|in (computer )?science|in math|in code)\b`),
	// "fix/debug/refactor this" — operating on code the user supplied.
	regexp.MustCompile(`(?i)\b(write|fix|debug|refactor|rewrite|review|optimi[sz]e)\b[^?.!]{0,40}\b(this|these|the following|my|below|above)\b`),
	// Authoring help. Without this, a decisive recency word inside a "how do I
	// write X" request would hand the turn to the search summarizer, which
	// answers from web evidence instead of producing the code the user asked for.
	// Note the absence of "get" and "find": "how do I get the latest Node" is a
	// factual question, not an authoring one.
	regexp.MustCompile(`(?i)\b(how (do|can) i|show me how to|help me)\b[^?.!]{0,60}\b(write|implement|code|build|configure|set up)\b`),
}

// negativePatterns lower the score instead of vetoing the search.
//
// They previously returned early, which is what made the gate refuse
// current-information questions about software. As weights they still keep
// "how do I implement a binary search in Python" out of the search path, while
// letting an explicit recency signal through.
//
// Note what is deliberately absent compared to the old veto list: "error",
// "package", "library", "framework", and "test". Each appears constantly in
// legitimate current-information questions ("latest package versions", "test
// results"), and as a veto each blocked a whole category of valid searches.
var negativePatterns = []weightedPattern{
	// Programming nouns — strongly indicate a knowledge question.
	{regexp.MustCompile(`(?i)\b(code|coding|function|method|class|variable|loop|array|pointer|struct|interface|bug|debug|compile|syntax|algorithm|recursion|regex|import|module)\b`), 2},
	// Imperative development verbs — the user wants work done, not facts.
	{regexp.MustCompile(`(?i)\b(implement|refactor|optimize|migrate|deploy|lint|mock|stub|fixture)\b`), 2},
	// Language and tool names — weak on their own, since these are also the
	// subjects of legitimate "what changed" questions.
	{regexp.MustCompile(`(?i)\b(html|css|javascript|typescript|python|golang|rust|java|sql|react|vue|angular|docker|kubernetes|git|npm|pip|cargo)\b`), 1},
}

// ShouldWebSearch applies deterministic rules with scoring to decide whether
// a user message should trigger a web search. Returns the decision and a
// pre-built ToolCall ready to execute.
//
// Evaluation order is significant:
//  1. hard suppression (the subject is code the user supplied)
//  2. decisive recency signals (explicitly asking for the newest state)
//  3. weighted scoring, with subject-matter negatives subtracting
//
// The gate stays deliberately cheap and deterministic. It is a first pass, not
// the only classifier: semantically phrased current-information questions with
// no keyword hook are the semantic router's job.
func ShouldWebSearch(userText string, now time.Time, tz string) (bool, *ToolCall) {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return false, nil
	}

	for _, suppress := range hardSuppressPatterns {
		if suppress.MatchString(lower) {
			return false, nil
		}
	}

	if !decisiveRecency(lower) {
		if searchScore(lower) < searchScoreThreshold {
			return false, nil
		}
	}

	query := buildSearchQuery(userText, now)
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

// searchScore sums trigger weights and subtracts subject-matter negatives. All
// patterns are evaluated: the old implementation broke out of the loop once the
// threshold was reached, which is incorrect once negatives can lower the total.
func searchScore(lower string) int {
	score := 0
	for _, wp := range triggerPatterns {
		if wp.re.MatchString(lower) {
			score += wp.weight
		}
	}
	for _, wp := range negativePatterns {
		if wp.re.MatchString(lower) {
			score -= wp.weight
		}
	}
	return score
}

// decisiveRecency reports whether the prompt explicitly asks for the newest
// state of something, which overrides subject-matter negatives.
func decisiveRecency(lower string) bool {
	for _, re := range decisivePatterns {
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

// buildSearchQuery cleans up the user text into a reasonable search query.
func buildSearchQuery(userText string, now time.Time) string {
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

	q = strings.TrimSpace(q)

	// Append current year if it's a "current" or "latest" request and doesn't already have a year
	hasYear := regexp.MustCompile(`\b20\d{2}\b`).MatchString(q)
	if !hasYear {
		if containsAny(lower, "current", "this year", "latest", "now", "today", "standings") {
			q = fmt.Sprintf("%s %d", q, now.Year())
		}
	}

	return q
}

// inferTimeRange picks a freshness window from explicit temporal keywords.
//
// The default is deliberately empty rather than "24h". A 24-hour filter applied
// to every triggered search is actively harmful for the questions users most
// often ask about changing data: official pricing pages, model cards, and
// benchmark leaderboards are rarely re-published within a day, so the filter
// removes precisely the primary sources the answer needs. An empty window means
// "no recency constraint", which providers honor by omitting the filter.
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
		return ""
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
