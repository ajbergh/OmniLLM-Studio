package websearch

import (
	"testing"
	"time"
)

func TestShouldWebSearch_StrongTriggers(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"today weather", "What's the weather today?", true},
		{"latest news", "What's the latest news?", true},
		{"breaking news", "Any breaking news about the election?", true},
		{"current price", "What is the current price of Bitcoin?", true},
		{"stock price", "Tesla stock price", true},
		{"look up", "Look up the population of France", true},
		{"fact check", "Fact check: Is the earth flat?", true},
		{"verify claim", "Can you verify this claim?", true},
		{"who won", "Who won the Super Bowl?", true},
		{"score", "What's the NBA score tonight?", true},
		{"real-time", "real-time stock market data", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tc := ShouldWebSearch(tt.text, now, "UTC")
			if got != tt.want {
				t.Errorf("ShouldWebSearch(%q) = %v, want %v", tt.text, got, tt.want)
			}
			if got && tc == nil {
				t.Error("expected non-nil ToolCall when search is needed")
			}
		})
	}
}

// TestShouldWebSearch_TechnologyCurrentInformation covers the regression that
// motivated reworking the gate.
//
// Subject-matter negatives used to veto the search before scoring ran, so any
// question containing "coding", "react", "kubernetes", "npm" and so on was
// answered from training data no matter how explicitly it asked for the newest
// state. These are exactly the questions where stale answers are most visible,
// and one of them ("latest version of React") was previously asserted as
// correct-to-suppress by this file.
func TestShouldWebSearch_TechnologyCurrentInformation(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		text string
	}{
		{"latest react version", "What's the latest version of React?"},
		{"best llm for coding now", "What is the best LLM for Go coding right now?"},
		{"latest coding benchmarks", "What are the latest benchmark results for coding models?"},
		{"current kubernetes release", "What's the current Kubernetes release?"},
		{"newest typescript", "What is the newest TypeScript release?"},
		{"api model today", "Which API model should I use today?"},
		{"latest model releases", "What changed in the latest model releases?"},
		{"current api pricing", "What is the current OpenAI API pricing?"},
		{"docker latest", "Is there a newer Docker Engine version than 27?"},
		{"pricing right now", "What is Claude API pricing right now?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tc := ShouldWebSearch(tt.text, now, "UTC")
			if !got {
				t.Fatalf("ShouldWebSearch(%q) = false; a technology question with an explicit recency signal must still search", tt.text)
			}
			if tc == nil {
				t.Fatal("expected non-nil ToolCall")
			}
		})
	}
}

// TestShouldWebSearch_ComparisonWithoutTemporalWord documents a deliberate limit.
// These reach the threshold through weak comparison/availability signals, but the
// gate is not the right tool for genuinely semantic phrasing; the intent router
// covers what regex cannot.
func TestShouldWebSearch_ComparisonReachesThreshold(t *testing.T) {
	now := time.Now()
	for _, text := range []string{
		"How do the available models compare?",
		"Which model has the best benchmark scores for the price?",
	} {
		if got, _ := ShouldWebSearch(text, now, "UTC"); !got {
			t.Errorf("ShouldWebSearch(%q) = false, want true", text)
		}
	}
}

func TestShouldWebSearch_CodeQuestionsStillSuppressed(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		text string
	}{
		{"coding question", "How do I implement a binary search in Python?"},
		{"debug help", "Help me debug this function error"},
		{"sql help", "Write an SQL query to find duplicates"},
		{"algorithm", "Explain the quicksort algorithm"},
		{"docker help", "How to deploy with Docker?"},
		{"refactor my code", "Refactor this function to use a map"},
		{"review my code", "Review my code below for bugs"},
		{"conceptual in programming", "What is a closure in programming?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := ShouldWebSearch(tt.text, now, "UTC"); got {
				t.Errorf("ShouldWebSearch(%q) = true, want false (code question)", tt.text)
			}
		})
	}
}

// TestShouldWebSearch_HardSuppressBeatsRecency verifies the precedence rule: a
// pasted code block or an explicit "fix this" outranks a recency word, because
// the subject is the user's code rather than the state of the world.
func TestShouldWebSearch_HardSuppressBeatsRecency(t *testing.T) {
	now := time.Now()
	tests := []string{
		"Fix this error, I'm on the latest React:\n```js\nconst x = 1\n```",
		"Refactor my latest migration script",
		"Debug the following code, it broke in the newest release",
	}
	for _, text := range tests {
		if got, _ := ShouldWebSearch(text, now, "UTC"); got {
			t.Errorf("ShouldWebSearch(%q) = true; hard suppression must outrank a recency word", text)
		}
	}
}

func TestShouldWebSearch_WeakOnly(t *testing.T) {
	now := time.Now()
	// Weak signals alone (score < threshold) should NOT trigger search.
	tests := []struct {
		name string
		text string
	}{
		{"bare year", "2024 was a good year"},
		{"bare current", "The current state of affairs"},
		{"bare comparison", "Python vs Go for backends"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ShouldWebSearch(tt.text, now, "UTC")
			if got {
				t.Errorf("ShouldWebSearch(%q) = true, want false (weak signals alone)", tt.text)
			}
		})
	}
}

func TestShouldWebSearch_EmptyInput(t *testing.T) {
	got, tc := ShouldWebSearch("", time.Now(), "UTC")
	if got {
		t.Error("empty input should return false")
	}
	if tc != nil {
		t.Error("empty input should return nil ToolCall")
	}
}

func TestSearchScoreSubtractsNegatives(t *testing.T) {
	// "how to deploy with docker?" earns 1 for the wh-question and loses 2 for
	// "deploy" plus 1 for "docker".
	if got := searchScore("how to deploy with docker?"); got >= searchScoreThreshold {
		t.Errorf("searchScore = %d, expected below threshold %d", got, searchScoreThreshold)
	}
	// Negatives must not be able to veto on their own once a genuine signal is
	// present alongside them.
	if got := searchScore("kubernetes release notes announcement 2026"); got < searchScoreThreshold {
		t.Errorf("searchScore = %d, expected at or above threshold %d", got, searchScoreThreshold)
	}
}

func TestDecisiveRecency(t *testing.T) {
	decisive := []string{
		"latest gpt-5 pricing",
		"newest kubernetes version",
		"most recent benchmark",
		"what happened today",
		"scores right now",
		"real-time market data",
		"live scores",
		"current claude model pricing",
		"current kubernetes patch release",
		"model pricing right now",
	}
	for _, text := range decisive {
		if !decisiveRecency(text) {
			t.Errorf("decisiveRecency(%q) = false, want true", text)
		}
	}

	notDecisive := []string{
		"the current state of affairs",
		"a recent conversation",
		"python vs go",
		"current thinking on microservices",
	}
	for _, text := range notDecisive {
		if decisiveRecency(text) {
			t.Errorf("decisiveRecency(%q) = true, want false", text)
		}
	}
}

func TestBuildSearchQuery(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		input string
		want  string
	}{
		{"What is the price of gold?", "the price of gold"},
		{"Search for best laptops 2024", "best laptops 2024"},
		{"Tell me about climate change", "climate change"},
		{"How does quantum computing work?", "quantum computing work"},
		{"plain query", "plain query"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := buildSearchQuery(tt.input, now)
			if got != tt.want {
				t.Errorf("buildSearchQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferTimeRange(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"today", "what happened today", "24h"},
		{"this week", "this week's top stories", "7d"},
		{"this month", "what happened this month", "30d"},
		{"yesterday", "yesterday's game results", "7d"},
		// No explicit temporal word means no freshness filter. A blanket 24h
		// default excluded official pricing pages, model cards, and benchmark
		// leaderboards, which are rarely re-published within a day.
		{"no temporal signal", "some random query", ""},
		{"pricing lookup", "current openai api pricing", ""},
		{"benchmark lookup", "best coding benchmark scores", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTimeRange(tt.input)
			if got != tt.want {
				t.Errorf("inferTimeRange(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
