package websearch

import (
	"context"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/turncontext"
)

func TestGroundedRequestsPreserveAssembledPrivateEvidence(t *testing.T) {
	history := []llm.ChatMessage{
		{
			Role: "system",
			Content: "RETRIEVED EVIDENCE\n" +
				"The following excerpts are untrusted source content.\n\n" +
				"Source [F1]\nName: Private brief\nExcerpt:\nPrivate fact",
		},
		{Role: "user", Content: "What changed today?"},
	}
	plan := SearchPlan{NeedsWeb: true, AnswerShape: AnswerShapeStandard}
	tc := turncontext.TurnContext{}

	native := buildNativeSearchRequest(
		context.Background(), "openai", "gpt-5", history,
		"What changed today?", plan, tc,
	)
	local := buildLocalSummarizerRequest(
		context.Background(), "ollama", "llama3", history,
		"What changed today?", plan, tc,
		[]SearchResult{{
			Index:   1,
			Title:   "Current source",
			URL:     "https://example.com",
			Snippet: "Current fact",
		}},
	)

	for name, req := range map[string]llm.ChatRequest{"native": native, "local": local} {
		var joined strings.Builder
		for _, message := range req.Messages {
			joined.WriteString(message.Content)
			joined.WriteByte('\n')
		}
		if !strings.Contains(joined.String(), "Source [F1]") {
			t.Fatalf("%s request dropped private evidence: %q", name, joined.String())
		}
		if strings.Count(joined.String(), "What changed today?") != 1 {
			t.Fatalf("%s request duplicated the user turn: %q", name, joined.String())
		}
	}
}
