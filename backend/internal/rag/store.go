// Package rag — VectorStore is a thin wrapper around chromem-go's *chromem.DB
// that gives the rest of the codebase a stable surface even if the chromem API
// changes. All chromem.* types are confined to this file plus retriever_chromem.go.
package rag

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	chromem "github.com/philippgille/chromem-go"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// VectorStore manages chromem-go collections for RAG vector storage.
type VectorStore struct {
	db       *chromem.DB
	compress bool
}

// NewVectorStore opens (or creates) a persistent chromem-go database at the
// given directory.
func NewVectorStore(dataDir string, compress bool) (*VectorStore, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("chromem data dir is required")
	}
	chromemDir := filepath.Clean(dataDir)
	db, err := chromem.NewPersistentDB(chromemDir, compress)
	if err != nil {
		return nil, fmt.Errorf("open chromem db at %s: %w", chromemDir, err)
	}
	return &VectorStore{db: db, compress: compress}, nil
}

// NewInMemoryVectorStore returns a non-persistent VectorStore (used in tests).
func NewInMemoryVectorStore() *VectorStore {
	return &VectorStore{db: chromem.NewDB()}
}

// Collection returns (or creates) the chromem collection for the given
// conversation ID, attaching the supplied EmbeddingFunc. Because chromem-go
// does not persist EmbeddingFunc across restarts, callers MUST supply a fresh
// EmbeddingFunc on every indexing or query operation.
func (s *VectorStore) Collection(
	_ context.Context,
	conversationID string,
	embedFunc chromem.EmbeddingFunc,
) (*chromem.Collection, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if embedFunc == nil {
		return nil, fmt.Errorf("embedding func is required")
	}
	coll, err := s.db.GetOrCreateCollection(conversationID, nil, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("get/create collection %s: %w", conversationID, err)
	}
	return coll, nil
}

// CollectionIfExists returns the existing collection for a conversation or
// nil if none has been created yet. Useful for deletion paths where we don't
// want to create a collection just to remove from it.
func (s *VectorStore) CollectionIfExists(conversationID string) *chromem.Collection {
	if conversationID == "" {
		return nil
	}
	return s.db.GetCollection(conversationID, nil)
}

// ExportToWriter serializes the entire chromem DB (gob-encoded, optionally
// gzip-compressed and AES-GCM-encrypted) to the given writer.
func (s *VectorStore) ExportToWriter(w io.Writer, compress bool, encryptionKey string) error {
	if err := s.db.ExportToWriter(w, compress, encryptionKey); err != nil {
		return fmt.Errorf("chromem export: %w", err)
	}
	return nil
}

// ImportFromReader replaces the chromem DB content with the gob-encoded data
// from the given reader.
func (s *VectorStore) ImportFromReader(r io.ReadSeeker, encryptionKey string) error {
	if err := s.db.ImportFromReader(r, encryptionKey); err != nil {
		return fmt.Errorf("chromem import: %w", err)
	}
	return nil
}

// DeleteCollection removes the collection (and its on-disk files, if any).
func (s *VectorStore) DeleteCollection(conversationID string) error {
	if conversationID == "" {
		return nil
	}
	if err := s.db.DeleteCollection(conversationID); err != nil {
		return fmt.Errorf("delete collection %s: %w", conversationID, err)
	}
	return nil
}

// IndexChunks embeds and stores the given chunks in the conversation's
// collection. Concurrency is capped at 4 for hosted providers to avoid
// rate-limiting; full CPU count is used only for local Ollama.
func (s *VectorStore) IndexChunks(
	ctx context.Context,
	conversationID string,
	chunks []models.DocumentChunk,
	providerType string,
	embedFunc chromem.EmbeddingFunc,
) error {
	if len(chunks) == 0 {
		return nil
	}
	coll, err := s.Collection(ctx, conversationID, embedFunc)
	if err != nil {
		return err
	}

	docs := make([]chromem.Document, len(chunks))
	for i, c := range chunks {
		docs[i] = chromem.Document{
			ID:      c.ID,
			Content: c.Content,
			Metadata: map[string]string{
				"attachment_id":   c.AttachmentID,
				"conversation_id": c.ConversationID,
				"chunk_index":     strconv.Itoa(c.ChunkIndex),
			},
		}
	}

	concurrency := runtime.NumCPU()
	if !strings.EqualFold(providerType, "ollama") && concurrency > 4 {
		concurrency = 4
	}

	if err := coll.AddDocuments(ctx, docs, concurrency); err != nil {
		return fmt.Errorf("add documents to %s: %w", conversationID, err)
	}
	return nil
}

// DeleteDocuments removes the given chunk IDs from a conversation's
// collection. No-op if the collection does not exist.
func (s *VectorStore) DeleteDocuments(ctx context.Context, conversationID string, chunkIDs ...string) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	coll := s.CollectionIfExists(conversationID)
	if coll == nil {
		return nil
	}
	if err := coll.Delete(ctx, nil, nil, chunkIDs...); err != nil {
		return fmt.Errorf("delete documents from %s: %w", conversationID, err)
	}
	return nil
}
