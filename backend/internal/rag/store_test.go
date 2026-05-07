package rag

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// staticEmbedFunc returns a deterministic embedding based on text length so
// tests don't depend on a network-backed embedding service.
func staticEmbedFunc(_ context.Context, text string) ([]float32, error) {
	// 3-dim vector encoding length, first-byte, and parity. Then normalize.
	v := []float32{
		float32(len(text)) / 100.0,
		float32(text[0]) / 255.0,
		float32(len(text) % 2),
	}
	// crude L2 normalization
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return v, nil
	}
	inv := 1.0 / float32sqrt(sum)
	for i := range v {
		v[i] *= inv
	}
	return v, nil
}

func float32sqrt(x float32) float32 {
	// Newton's method, good enough for tests.
	if x == 0 {
		return 0
	}
	g := x
	for i := 0; i < 8; i++ {
		g = 0.5 * (g + x/g)
	}
	return g
}

func TestVectorStoreIndexAndDelete(t *testing.T) {
	store := NewInMemoryVectorStore()
	ctx := context.Background()
	convoID := "conv-test"

	chunks := []models.DocumentChunk{
		{ID: "c1", AttachmentID: "a1", ConversationID: convoID, ChunkIndex: 0, Content: "the quick brown fox"},
		{ID: "c2", AttachmentID: "a1", ConversationID: convoID, ChunkIndex: 1, Content: "jumps over the lazy dog"},
	}

	if err := store.IndexChunks(ctx, convoID, chunks, "openai", staticEmbedFunc); err != nil {
		t.Fatalf("IndexChunks: %v", err)
	}

	coll := store.CollectionIfExists(convoID)
	if coll == nil {
		t.Fatal("expected collection to exist after IndexChunks")
	}
	if coll.Count() != 2 {
		t.Fatalf("expected 2 docs, got %d", coll.Count())
	}

	if err := store.DeleteDocuments(ctx, convoID, "c1"); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if coll.Count() != 1 {
		t.Fatalf("expected 1 doc after delete, got %d", coll.Count())
	}

	if err := store.DeleteCollection(convoID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if got := store.CollectionIfExists(convoID); got != nil {
		t.Fatal("expected collection to be removed")
	}
}

func TestVectorStoreCollectionRequiresEmbedFunc(t *testing.T) {
	store := NewInMemoryVectorStore()
	if _, err := store.Collection(context.Background(), "x", nil); err == nil {
		t.Fatal("expected error when embed func is nil")
	}
}

func TestVectorStoreIndexEmpty(t *testing.T) {
	store := NewInMemoryVectorStore()
	if err := store.IndexChunks(context.Background(), "c", nil, "openai", staticEmbedFunc); err != nil {
		t.Fatalf("expected no-op for empty chunks, got: %v", err)
	}
}
