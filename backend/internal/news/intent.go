package news

import (
	"regexp"
	"strings"
)

// sportsTerms are terms that indicate a sports-related prompt that should be
// rejected by the news detector. These mirror the sports detector's domain.
var sportsTerms = []string{
	"sports", "mlb", "nba", "nfl", "nhl", "wnba", "mls", "epl",
	"premier league", "college football", "college basketball", "ncaa",
	"scores", "standings", "schedule", "odds", "spread", "moneyline",
	"roster", "injury report", "player stats", "team stats",
	"home runs", "touchdowns", "goals", "assists", "playoffs",
	"draft", "trade deadline", "espn", "athletic",
	// Team names that are commonly mentioned
	"cubs", "bears", "packers", "bulls", "blackhawks", "brewers",
	"yankees", "dodgers", "lakers", "warriors", "celtics", "chiefs",
	"cowboys", "eagles", "49ers", "patriots", "bills", "lions",
	"mavericks", "knicks", "heat", "nuggets", "suns", "bruins",
	"rangers", "maple leafs", "avalanche", "lightning", "panthers",
	"golden knights", "cardinals", "phillies", "braves", "giants",
	"padres", "red sox", "mets", "orioles", "astros", "mariners",
	"twins", "guardians", "tigers", "royals", "angels", "athletics",
	"pirates", "reds", "rockies", "marlins", "diamondbacks", "blue jays",
	"white sox", "rays", "nationals",
}

// newsIndicators are terms that suggest a news/current-events prompt.
var newsIndicators = []string{
	"news", "headlines", "latest", "breaking", "today",
	"this week", "current events", "developments",
	"what happened", "what is happening", "what's happening",
	"what is going on", "what's going on",
	"coverage", "stories", "front page", "newspaper",
	"article", "press", "global news", "world news",
	"top stories", "major stories", "important stories",
}

// topicToIssueSlug maps topic keywords to Actually Relevant issue slugs.
var topicToIssueSlug = map[string]string{
	// Science & Technology
	"science":                 "science-technology",
	"technology":              "science-technology",
	"tech":                    "science-technology",
	"ai":                      "science-technology",
	"artificial intelligence": "science-technology",
	"llm":                     "science-technology",
	"openai":                  "science-technology",
	"anthropic":               "science-technology",
	"google ai":               "science-technology",
	"chips":                   "science-technology",
	"semiconductor":           "science-technology",
	"space":                   "science-technology",
	"astronomy":               "science-technology",
	"research":                "science-technology",
	"robot":                   "science-technology",
	"cybersecurity":           "science-technology",
	"software":                "science-technology",
	"biotech":                 "science-technology",

	// Planet & Climate
	"climate":         "planet-climate",
	"planet":          "planet-climate",
	"environment":     "planet-climate",
	"emissions":       "planet-climate",
	"fossil fuels":    "planet-climate",
	"clean energy":    "planet-climate",
	"renewable":       "planet-climate",
	"carbon":          "planet-climate",
	"biodiversity":    "planet-climate",
	"conservation":    "planet-climate",
	"pollution":       "planet-climate",
	"oceans":          "planet-climate",
	"deforestation":   "planet-climate",
	"extreme weather": "planet-climate",

	// Existential Threats
	"existential":          "existential-threats",
	"nuclear":              "existential-threats",
	"nuclear risk":         "existential-threats",
	"biosecurity":          "existential-threats",
	"pandemic":             "existential-threats",
	"ai safety":            "existential-threats",
	"ai risk":              "existential-threats",
	"catastrophic":         "existential-threats",
	"war escalation":       "existential-threats",
	"global security":      "existential-threats",
	"autonomous weapons":   "existential-threats",
	"great power conflict": "existential-threats",

	// Human Development
	"health":        "human-development",
	"education":     "human-development",
	"poverty":       "human-development",
	"food security": "human-development",
	"inequality":    "human-development",
	"migration":     "human-development",
	"human rights":  "human-development",
	"governance":    "human-development",
	"democracy":     "human-development",
	"development":   "human-development",
	"labor":         "human-development",
	"public health": "human-development",
	"medicine":      "human-development",
	"humanitarian":  "human-development",
	"children":      "human-development",
	"economy":       "human-development",
}

// nonNewsPhrases are prompts that look news-like but are not about current events.
var nonNewsPhrases = []string{
	"write a fictional newspaper",
	"write a fake newspaper",
	"create a newspaper-style",
	"design a newspaper",
	"make a newspaper",
	"fictional newspaper article",
	"fake news article",
	"history of newspapers",
	"how to write a newspaper",
	"newspaper template",
	"newsletter design",
	"news-style landing page",
}

// DetectNewsIntent analyzes a user prompt and determines if it's a news lookup request.
func DetectNewsIntent(prompt string) NewsIntent {
	raw := strings.TrimSpace(prompt)
	if raw == "" || len(raw) < 5 {
		return NewsIntent{Handled: false}
	}

	norm := normalizeText(raw)

	// Reject non-news creative prompts first
	if isNonNewsPrompt(norm) {
		return NewsIntent{Handled: false, Reason: "non-news creative prompt"}
	}

	// Reject sports prompts
	if containsSportsTerm(norm) {
		return NewsIntent{Handled: false, Reason: "sports-related prompt"}
	}

	// Check for news indicators
	hasNewsIndicator := false
	for _, indicator := range newsIndicators {
		if strings.Contains(norm, indicator) {
			hasNewsIndicator = true
			break
		}
	}

	if !hasNewsIndicator {
		return NewsIntent{Handled: false, Reason: "no news indicator found"}
	}

	// Determine confidence
	confidence := 0.7
	if hasNewsIndicator {
		confidence = 0.75
	}

	// Extract issue slug from topic keywords
	issueSlug := detectIssueSlug(norm)

	// Extract search query
	search := extractSearchQuery(norm, issueSlug)

	// Determine intent type and page size
	intentType, pageSize := detectPresentationStyle(norm)

	// Boost confidence for strong signals
	if issueSlug != "" {
		confidence += 0.1
	}
	if intentType == NewsIntentFrontPage {
		confidence += 0.1
	}

	// Clamp confidence
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Only handle when confidence is high enough
	if confidence < 0.65 {
		return NewsIntent{Handled: false, Reason: "confidence below threshold"}
	}

	return NewsIntent{
		Handled:        true,
		Confidence:     confidence,
		Query:          raw,
		IssueSlug:      issueSlug,
		Search:         search,
		PageSize:       pageSize,
		IntentType:     intentType,
		WantsFrontPage: intentType == NewsIntentFrontPage,
		WantsBrief:     intentType == NewsIntentBrief,
		WantsDetailed:  intentType == NewsIntentDetailed,
		Reason:         "news intent detected",
	}
}

// normalizeText lowercases and collapses whitespace.
func normalizeText(s string) string {
	lower := strings.ToLower(s)
	// Collapse whitespace
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(lower, " ")
}

// isNonNewsPrompt checks if the prompt is about creating fictional news content.
func isNonNewsPrompt(norm string) bool {
	for _, phrase := range nonNewsPhrases {
		if strings.Contains(norm, phrase) {
			return true
		}
	}
	return false
}

// containsSportsTerm checks if the normalized text contains any sports-related terms.
func containsSportsTerm(norm string) bool {
	for _, term := range sportsTerms {
		if strings.Contains(norm, term) {
			return true
		}
	}
	return false
}

// detectIssueSlug maps topic keywords in the prompt to an issue slug.
func detectIssueSlug(norm string) string {
	// Check for multi-word topics first (longer matches take priority)
	type match struct {
		slug   string
		length int
	}
	var best match

	for keyword, slug := range topicToIssueSlug {
		if strings.Contains(norm, keyword) {
			if len(keyword) > best.length {
				best = match{slug: slug, length: len(keyword)}
			}
		}
	}

	return best.slug
}

// detectPresentationStyle determines the desired presentation format.
func detectPresentationStyle(norm string) (NewsIntentType, int) {
	if strings.Contains(norm, "front page") || strings.Contains(norm, "newspaper") || strings.Contains(norm, "edition") {
		return NewsIntentFrontPage, 10
	}
	if strings.Contains(norm, "brief") || strings.Contains(norm, "quick") || strings.Contains(norm, "summary") {
		return NewsIntentBrief, 5
	}
	if strings.Contains(norm, "detailed") || strings.Contains(norm, "deep dive") || strings.Contains(norm, "why it matters") {
		return NewsIntentDetailed, 8
	}

	// Check for explicit "top N" patterns
	re := regexp.MustCompile(`\btop\s+(\d{1,2})\b`)
	if matches := re.FindStringSubmatch(norm); len(matches) > 1 {
		if n := atoi(matches[1]); n > 0 && n <= 15 {
			return NewsIntentFrontPage, n
		}
	}

	return NewsIntentFrontPage, 8
}

// extractSearchQuery extracts a concise search query from the prompt.
func extractSearchQuery(norm string, issueSlug string) string {
	// Remove common prefixes
	prefixes := []string{
		"what are the latest", "what is the latest", "what's the latest",
		"show me the latest", "show me today's", "show me",
		"give me the latest", "give me today's", "give me",
		"what is happening in", "what's happening in", "what is going on with",
		"what's going on with", "tell me about", "i want to know about",
		"latest", "breaking", "top", "today's",
	}

	query := norm
	for _, prefix := range prefixes {
		query = strings.TrimPrefix(query, prefix+" ")
	}

	// Remove trailing filler
	fillers := []string{"news", "headlines", "stories", "today", "this week"}
	for _, filler := range fillers {
		query = strings.TrimSuffix(strings.TrimSpace(query), " "+filler)
	}

	query = strings.TrimSpace(query)

	// If the query is just the issue slug or empty, return empty
	if query == "" || query == issueSlug {
		return ""
	}

	// Keep it concise (max 100 chars)
	if len(query) > 100 {
		query = query[:100]
	}

	return query
}

// atoi is a simple string to int conversion for regex matches.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
