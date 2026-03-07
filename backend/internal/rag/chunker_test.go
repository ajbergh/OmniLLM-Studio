package rag

import (
	"testing"
)

func TestChunkText_Basic(t *testing.T) {
	text := "Hello world. This is a test document with some content that should be chunked."
	chunks := ChunkText(text, "att-1", "conv-1", ChunkOptions{ChunkSize: 30, Overlap: 10})

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	for i, c := range chunks {
		if c.ChunkIndex != i {
			t.Errorf("chunk %d: expected index %d, got %d", i, i, c.ChunkIndex)
		}
		if c.AttachmentID != "att-1" {
			t.Errorf("chunk %d: wrong attachment_id", i)
		}
		if c.ConversationID != "conv-1" {
			t.Errorf("chunk %d: wrong conversation_id", i)
		}
		if c.Content == "" {
			t.Errorf("chunk %d: empty content", i)
		}
		if c.ID == "" {
			t.Errorf("chunk %d: empty ID", i)
		}
	}
}

func TestChunkText_EmptyInput(t *testing.T) {
	chunks := ChunkText("", "a", "c", DefaultChunkOptions())
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestChunkText_SmallText(t *testing.T) {
	text := "Short text."
	chunks := ChunkText(text, "a", "c", ChunkOptions{ChunkSize: 1000, Overlap: 200})

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0].Content != "Short text." {
		t.Errorf("unexpected content: %q", chunks[0].Content)
	}
}

func TestChunkText_Overlap(t *testing.T) {
	// Generate a string of 100 characters
	text := ""
	for i := 0; i < 100; i++ {
		text += "a"
	}

	chunks := ChunkText(text, "a", "c", ChunkOptions{ChunkSize: 40, Overlap: 10})

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	// Verify overlap: second chunk should start at offset 30 (40 - 10)
	if chunks[1].CharOffset != 30 {
		t.Errorf("expected second chunk offset 30, got %d", chunks[1].CharOffset)
	}
}

func TestChunkMarkdown_HeadingSplit(t *testing.T) {
	text := `# Introduction
This is the intro section.

## Methods
We used these methods.

## Results
Here are the results.`

	chunks := ChunkMarkdown(text, "a", "c", ChunkOptions{ChunkSize: 500, Overlap: 50})

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks for 3 headings, got %d", len(chunks))
	}
}

func TestChunkText_TokenEstimate(t *testing.T) {
	text := "This is exactly twenty chars" // ~28 chars
	chunks := ChunkText(text, "a", "c", ChunkOptions{ChunkSize: 1000, Overlap: 0})

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].TokenCount == nil {
		t.Fatal("expected token count to be set")
	}
	if *chunks[0].TokenCount <= 0 {
		t.Errorf("expected positive token count, got %d", *chunks[0].TokenCount)
	}
}
