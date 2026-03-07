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

func TestShouldWebSearch_NegativePatterns(t *testing.T) {
	now := time.Now()
	// These contain programming/code keywords and should NOT trigger search.
	tests := []struct {
		name string
		text string
	}{
		{"coding question", "How do I implement a binary search in Python?"},
		{"debug help", "Help me debug this function error"},
		{"react question", "What's the latest version of React?"},
		{"sql help", "Write an SQL query to find duplicates"},
		{"algorithm", "Explain the quicksort algorithm"},
		{"docker help", "How to deploy with Docker?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ShouldWebSearch(tt.text, now, "UTC")
			if got {
				t.Errorf("ShouldWebSearch(%q) = true, want false (negative pattern should suppress)", tt.text)
			}
		})
	}
}

func TestShouldWebSearch_WeakOnly(t *testing.T) {
	now := time.Now()
	// Weak signals alone (score < threshold) should NOT trigger search.
	// Note: "What's a recent..." triggers both wh-question (1) + recent (1) = 2,
	// so that correctly triggers. We test truly isolated weak signals instead.
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

func TestBuildSearchQuery(t *testing.T) {
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
			got := buildSearchQuery(tt.input)
			if got != tt.want {
				t.Errorf("buildSearchQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferTimeRange(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"what happened today", "24h"},
		{"this week's top stories", "7d"},
		{"what happened this month", "30d"},
		{"yesterday's game results", "7d"},
		{"some random query", "24h"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := inferTimeRange(tt.input)
			if got != tt.want {
				t.Errorf("inferTimeRange(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
