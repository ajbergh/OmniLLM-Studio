package rag

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestInspectChunkIDsDetectsEqualCardinalityWrongContents(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryVectorStore()
	if err := store.IndexChunks(ctx, "conv-integrity", []models.DocumentChunk{
		{ID: "persisted-a", AttachmentID: "a", ConversationID: "conv-integrity", Content: "first snippet", ChunkIndex: 0},
		{ID: "unexpected-b", AttachmentID: "a", ConversationID: "conv-integrity", Content: "second snippet", ChunkIndex: 1},
	}, "openai", staticEmbedFunc); err != nil {
		t.Fatal(err)
	}
	report, err := store.InspectChunkIDs(ctx, "conv-integrity", []string{"persisted-a", "missing-b"})
	if err != nil {
		t.Fatal(err)
	}
	if report.AllExpectedPresent || report.ExpectedChunks != 2 || report.VectorRecords != 2 ||
		report.PhysicalCollections != 1 || report.OrphanVectorIDsChecked ||
		report.ChunksMissingAnyVector != 1 {
		t.Fatalf("equal-cardinality corrupt vector scope went undetected: %+v", report)
	}
	// Duplicate relational IDs must not inflate the expectation.
	clean, err := store.InspectChunkIDs(ctx, "conv-integrity", []string{"persisted-a", "unexpected-b", "unexpected-b"})
	if err != nil || !clean.AllExpectedPresent || clean.ExpectedChunks != 2 {
		t.Fatalf("healthy vector IDs not recognized: %+v, %v", clean, err)
	}
}

func TestInspectChunkIDsNoCollectionAndCancelled(t *testing.T) {
	store := NewInMemoryVectorStore()
	ctx := context.Background()
	report, err := store.InspectChunkIDs(ctx, "missing-conversation", []string{"chunk-1"})
	if err != nil || report.AllExpectedPresent || report.ChunksMissingAnyVector != 1 ||
		report.VectorRecords != 0 || report.PhysicalCollections != 0 {
		t.Fatalf("missing collection produced unexpected inspection: %+v: %v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.InspectChunkIDs(ctx, "missing-conversation", []string{"chunk-1"}); err == nil {
		t.Fatal("cancelled integrity scan must return an error")
	}
	if _, err := store.InspectChunkIDs(context.Background(), "", nil); err == nil {
		t.Fatal("empty scope must not succeed")
	}
	if _, err := store.InspectChunkIDs(context.Background(), "other", []string{""}); err == nil {
		t.Fatal("empty relational IDs must be rejected")
	}
}
