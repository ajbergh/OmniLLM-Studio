package rag_test

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func newMigrationTestDB(t *testing.T) (*repository.ChunkRepo, *repository.EmbeddingRepo, *repository.ConversationRepo, *repository.AttachmentRepo) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return repository.NewChunkRepo(database),
		repository.NewEmbeddingRepo(database),
		repository.NewConversationRepo(database),
		repository.NewAttachmentRepo(database)
}

// TestChromemRetriever_LazyMigrate verifies that an empty chromem collection
// gets populated from legacy SQL embeddings on first Retrieve, with no
// re-embedding network call.
func TestChromemRetriever_LazyMigrate(t *testing.T) {
	chunkRepo, embedRepo, convoRepo, attachRepo := newMigrationTestDB(t)

	convo, err := convoRepo.Create("", "rag-test", nil, nil, nil)
	if err != nil {
		t.Fatalf("create convo: %v", err)
	}
	convoID := convo.ID

	att := &models.Attachment{
		ID:             "att-1",
		ConversationID: convoID,
		Type:           "file",
		MimeType:       "text/plain",
		StoragePath:    "att-1.txt",
		Bytes:          10,
	}
	if err := attachRepo.Create(att); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	// Seed two chunks with legacy SQL embeddings — orthogonal vectors so the
	// query embedding (matching one) ranks one above the other.
	chunks := []models.DocumentChunk{
		{ID: "chunk-a", AttachmentID: att.ID, ConversationID: convoID, ChunkIndex: 0, Content: "alpha document"},
		{ID: "chunk-b", AttachmentID: att.ID, ConversationID: convoID, ChunkIndex: 1, Content: "beta document"},
	}
	if err := chunkRepo.CreateBatch(chunks); err != nil {
		t.Fatalf("create chunks: %v", err)
	}

	if err := embedRepo.UpsertBatch([]models.DocumentEmbedding{
		{ChunkID: "chunk-a", Embedding: []float32{1, 0, 0}, Model: "test", Dimensions: 3},
		{ChunkID: "chunk-b", Embedding: []float32{0, 1, 0}, Model: "test", Dimensions: 3},
	}); err != nil {
		t.Fatalf("upsert embeddings: %v", err)
	}

	// Build retriever with in-memory chromem and a fake embed func that returns
	// a vector aligned with chunk-a so it should win the similarity ranking.
	store := rag.NewInMemoryVectorStore()
	r := rag.NewChromemRetrieverWithDeps(chunkRepo, store).
		WithLegacyEmbeddingRepo(embedRepo).
		WithEmbedFuncForTest(func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0}, nil
		})

	got, err := r.Retrieve(context.Background(), convoID, "find alpha", "test", "test", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results after lazy migration, got %d", len(got))
	}
	if got[0].Chunk.ID != "chunk-a" {
		t.Errorf("expected chunk-a first, got %s", got[0].Chunk.ID)
	}

	// Verify the chromem collection was actually populated (not just queried via legacy path).
	if coll := store.CollectionIfExists(convoID); coll == nil || coll.Count() != 2 {
		t.Errorf("expected migrated collection with 2 docs, got %v", coll)
	}
}

// TestChromemRetriever_NoLegacyData returns nil when both chromem and legacy
// SQL are empty.
func TestChromemRetriever_NoLegacyData(t *testing.T) {
	chunkRepo, embedRepo, convoRepo, _ := newMigrationTestDB(t)
	convo, _ := convoRepo.Create("", "empty", nil, nil, nil)

	store := rag.NewInMemoryVectorStore()
	r := rag.NewChromemRetrieverWithDeps(chunkRepo, store).
		WithLegacyEmbeddingRepo(embedRepo).
		WithEmbedFuncForTest(func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0}, nil
		})

	got, err := r.Retrieve(context.Background(), convo.ID, "anything", "p", "m", 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 results for empty index, got %d", len(got))
	}
}
