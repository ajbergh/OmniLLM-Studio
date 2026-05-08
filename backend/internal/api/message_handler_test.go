package api

import (
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

func TestComposeSystemPrompt_AlwaysIncludesMarkdownDirective(t *testing.T) {
	cases := []struct {
		name       string
		userPrompt string
	}{
		{"no user prompt", ""},
		{"with user prompt", "You are a pirate. Speak in pirate slang."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := composeSystemPrompt(tc.userPrompt)
			if !strings.Contains(got, markdownFormattingDirective) {
				t.Fatalf("composed prompt missing Markdown directive. got: %q", got)
			}
			if !strings.Contains(got, sportsLookupSystemDirective) {
				t.Fatalf("composed prompt missing sports lookup directive. got: %q", got)
			}
			if tc.userPrompt != "" && !strings.Contains(got, tc.userPrompt) {
				t.Fatalf("composed prompt missing user prompt. got: %q", got)
			}
		})
	}
}

func TestAppendToBaseSystemPrompt_TargetsMarkdownPrompt(t *testing.T) {
	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "RAG context here"},
			{Role: "system", Content: composeSystemPrompt("")},
			{Role: "user", Content: "make me a word doc"},
		},
	}
	appendToBaseSystemPrompt(&req, wordDocSystemDirective)

	if strings.Contains(req.Messages[0].Content, wordDocSystemDirective) {
		t.Fatalf("word-doc directive should NOT be appended to RAG system message")
	}
	if !strings.Contains(req.Messages[1].Content, wordDocSystemDirective) {
		t.Fatalf("word-doc directive should be appended to the markdown-base system message")
	}
}

func TestAppendToBaseSystemPrompt_FallsBackToFirstSystem(t *testing.T) {
	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "some custom system"},
			{Role: "user", Content: "hi"},
		},
	}
	appendToBaseSystemPrompt(&req, wordDocSystemDirective)
	if !strings.Contains(req.Messages[0].Content, wordDocSystemDirective) {
		t.Fatalf("expected directive to attach to first system message; got: %q", req.Messages[0].Content)
	}
}

func TestAppendToBaseSystemPrompt_PrependsWhenNoSystemMessage(t *testing.T) {
	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}
	appendToBaseSystemPrompt(&req, wordDocSystemDirective)
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" {
		t.Fatalf("expected a new system message at index 0; got: %+v", req.Messages)
	}
	if req.Messages[0].Content != wordDocSystemDirective {
		t.Fatalf("system content mismatch")
	}
}

// Verifies that when we layer the markdown directive onto a websearch
// summarizer request (which has its own system prompt without our directive),
// the directive lands on the summarizer's system message — not as a new one.
func TestAppendToBaseSystemPrompt_LayersOnSummarizer(t *testing.T) {
	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "You are a research assistant. Cite sources."},
			{Role: "user", Content: "today's MLB scores"},
		},
	}
	appendToBaseSystemPrompt(&req, markdownFormattingDirective)

	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, markdownFormattingDirective) {
		t.Fatalf("markdown directive should layer onto the summarizer's system message; got: %q", req.Messages[0].Content)
	}
	if !strings.Contains(req.Messages[0].Content, "research assistant") {
		t.Fatalf("original summarizer content was overwritten")
	}
}

func TestDetectWordDocIntent(t *testing.T) {
	yes := []string{
		"please give me a word document about cats",
		"export this as a .docx",
		"save as word file please",
		"in WORD FORMAT",
	}
	no := []string{
		"just answer in plain text",
		"summarize this article",
		"hello there",
	}
	for _, s := range yes {
		if !detectWordDocIntent(s) {
			t.Errorf("expected word-doc intent for %q", s)
		}
	}
	for _, s := range no {
		if detectWordDocIntent(s) {
			t.Errorf("did not expect word-doc intent for %q", s)
		}
	}
}
