package urlcontext

import (
	"testing"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		maxURLs  int
		expected []string
	}{
		{
			name:     "bare URL",
			message:  "Review this https://github.com/ajbergh/OmniLLM-Studio.",
			maxURLs:  5,
			expected: []string{"https://github.com/ajbergh/OmniLLM-Studio"},
		},
		{
			name:     "URL with query string",
			message:  "Read: https://example.com/docs?q=test&x=1",
			maxURLs:  5,
			expected: []string{"https://example.com/docs?q=test&x=1"},
		},
		{
			name:     "markdown link",
			message:  "See [repo](https://github.com/owner/repo)",
			maxURLs:  5,
			expected: []string{"https://github.com/owner/repo"},
		},
		{
			name:     "angle bracket link",
			message:  "Look at <https://example.com/page>.",
			maxURLs:  5,
			expected: []string{"https://example.com/page"},
		},
		{
			name:     "multiple URLs deduplicated",
			message:  "https://a.com and https://b.com/path) and https://a.com",
			maxURLs:  5,
			expected: []string{"https://a.com", "https://b.com/path"},
		},
		{
			name:     "trailing punctuation stripped",
			message:  "See https://example.com, and https://other.org.",
			maxURLs:  5,
			expected: []string{"https://example.com", "https://other.org"},
		},
		{
			name:     "max URLs cap",
			message:  "https://a.com https://b.com https://c.com https://d.com",
			maxURLs:  2,
			expected: []string{"https://a.com", "https://b.com"},
		},
		{
			name:     "no URLs",
			message:  "What are the standings?",
			maxURLs:  5,
			expected: nil,
		},
		{
			name:     "http URL supported",
			message:  "See http://example.com/page",
			maxURLs:  5,
			expected: []string{"http://example.com/page"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURLs(tt.message, tt.maxURLs)
			if len(got) != len(tt.expected) {
				t.Fatalf("ExtractURLs(%q) = %v (len %d), want %v (len %d)",
					tt.message, got, len(got), tt.expected, len(tt.expected))
			}
			for i, u := range got {
				if u != tt.expected[i] {
					t.Errorf("ExtractURLs(%q)[%d] = %q, want %q", tt.message, i, u, tt.expected[i])
				}
			}
		})
	}
}
