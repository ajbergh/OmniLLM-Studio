package rag

import (
	"fmt"
	"strings"
)

// ContextBlock is the formatted RAG context ready for injection into a prompt.
type ContextBlock struct {
	// Text is the system-level context string to prepend to (or inject into)
	// the LLM conversation.
	Text string
	// Sources describes each chunk used, for returning to the frontend.
	Sources []SourceRef
}

// SourceRef is a lightweight reference to a single retrieved chunk, suitable
// for serializing in the SSE "done" event or API response.
type SourceRef struct {
	ChunkID      string  `json:"chunk_id"`
	AttachmentID string  `json:"attachment_id"`
	ChunkIndex   int     `json:"chunk_index"`
	Score        float64 `json:"score"`
	Preview      string  `json:"preview"` // first N chars of chunk content
}

const previewLen = 120

// BuildContext creates a ContextBlock from retrieved chunks.
// The returned Text is intended to be inserted as a system message such as:
//
//	"Use the following reference material to help answer the user's question.
//	 Cite [Source N] when you use information from a source.\n\n" + block.Text
func BuildContext(chunks []RetrievedChunk) *ContextBlock {
	if len(chunks) == 0 {
		return nil
	}

	var sb strings.Builder
	sources := make([]SourceRef, 0, len(chunks))

	for i, rc := range chunks {
		label := fmt.Sprintf("[Source %d]", i+1)
		sb.WriteString(label)
		sb.WriteString("\n")
		sb.WriteString(rc.Chunk.Content)
		sb.WriteString("\n\n")

		preview := rc.Chunk.Content
		if len(preview) > previewLen {
			preview = preview[:previewLen] + "…"
		}

		sources = append(sources, SourceRef{
			ChunkID:      rc.Chunk.ID,
			AttachmentID: rc.Chunk.AttachmentID,
			ChunkIndex:   rc.Chunk.ChunkIndex,
			Score:        rc.Score,
			Preview:      preview,
		})
	}

	return &ContextBlock{
		Text:    sb.String(),
		Sources: sources,
	}
}

// SystemPrompt returns a ready-to-use system prompt that wraps the context
// block with instructions for the LLM.
func SystemPrompt(block *ContextBlock) string {
	if block == nil || block.Text == "" {
		return ""
	}
	return fmt.Sprintf(
		"Use the following reference material to help answer the user's question. "+
			"Cite [Source N] when you use information from a source.\n\n%s",
		block.Text,
	)
}
