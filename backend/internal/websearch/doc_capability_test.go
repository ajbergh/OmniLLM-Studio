package websearch

import "testing"

// TestDocumentedCapabilityMatrix is a documentation-drift guard.
//
// The provider capability tables in docs/Feature FAQ.md (section 2b) and
// docs/PROVIDER_AWARE_SEARCH.md tell users which providers ground their own
// answers. A table that silently diverges from SupportsNativeSearch is worse
// than no table: it tells someone their model is grounded when it is not.
//
// Update this test and both tables together.
func TestDocumentedCapabilityMatrix(t *testing.T) {
	cases := []struct {
		provider, model string
		want            bool
	}{
		{"openai", "gpt-4.1", true}, {"openai", "gpt-5", true},
		{"openai", "o3-mini", true}, {"openai", "o4", true},
		{"openai", "gpt-3.5-turbo", false},
		{"anthropic", "claude-opus-5", true}, {"anthropic", "claude-sonnet-5", true},
		{"anthropic", "claude-fable-5", true}, {"anthropic", "claude-mythos-5", true},
		{"anthropic", "claude-haiku-4-5", true},
		{"anthropic", "claude-3-5-sonnet-20241022", false},
		{"gemini", "gemini-2.0-flash", true}, {"gemini", "gemini-3-pro", true},
		{"gemini", "gemini-1.5-pro", false},
		{"openrouter", "anthropic/claude-opus-5", true},
		{"openrouter", "openai/gpt-5", true},
		{"openrouter", "google/gemini-3-pro", true},
		{"openrouter", "perplexity/sonar", true},
		{"openrouter", "meta-llama/llama-3.3-70b-instruct", false},
		{"ollama", "llama3.2", false},
		{"groq", "llama-3.3-70b", false},
		{"together", "anything", false},
		{"mistral", "mistral-large", false},
		{"openai-compatible", "custom", false},
	}
	for _, c := range cases {
		if got := SupportsNativeSearch(c.provider, c.model); got != c.want {
			t.Errorf("docs say SupportsNativeSearch(%q, %q)=%v, implementation says %v",
				c.provider, c.model, c.want, got)
		}
	}
}
