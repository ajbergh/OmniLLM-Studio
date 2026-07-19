package rag

// File overview: manages embedding-space-isolated chromem collections used by the default RAG runtime.

import (
	"context"
	"fmt"
	"github.com/ajbergh/omnillm-studio/internal/models"
	chromem "github.com/philippgille/chromem-go"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const defaultEmbeddingBatchSize = 64

// VectorStore manages chromem-go collections for RAG vector storage.
type VectorStore struct {
	db       *chromem.DB
	compress bool
	dataDir  string
}

// QueryResult is a normalized vector query hit from chromem.
type QueryResult struct {
	ID         string
	Similarity float64
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
	return &VectorStore{db: db, compress: compress, dataDir: chromemDir}, nil
}

// NewInMemoryVectorStore returns a non-persistent VectorStore (used in tests).
func NewInMemoryVectorStore() *VectorStore {
	return &VectorStore{db: chromem.NewDB()}
}

func collectionNameForEmbedding(logical string, embedFunc chromem.EmbeddingFunc) string {
	if space, ok := EmbeddingSpaceForFunc(embedFunc); ok {
		return PhysicalCollectionName(logical, space)
	}
	// Test functions and legacy custom embedding functions have no registered
	// identity. Keep the historical collection name for compatibility.
	return logical
}

// Collection returns (or creates) the physical collection for the logical scope
// and embedding function. Collections produced by different providers/models
// are isolated automatically.
func (s *VectorStore) Collection(
	_ context.Context,
	logicalName string,
	embedFunc chromem.EmbeddingFunc,
) (*chromem.Collection, error) {
	if strings.TrimSpace(logicalName) == "" {
		return nil, fmt.Errorf("collection name is required")
	}
	if embedFunc == nil {
		return nil, fmt.Errorf("embedding func is required")
	}
	physicalName := collectionNameForEmbedding(logicalName, embedFunc)
	metadata := map[string]string{"logical_collection": logicalName}
	if space, ok := EmbeddingSpaceForFunc(embedFunc); ok {
		metadata["embedding_provider"] = space.Provider
		metadata["embedding_model"] = space.Model
		metadata["embedding_space"] = space.RoutingFingerprint()
		metadata["embedding_schema_version"] = strconv.Itoa(space.SchemaVersion)
	}
	coll, err := s.db.GetOrCreateCollection(physicalName, metadata, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("get/create collection %s: %w", physicalName, err)
	}
	return coll, nil
}

// CollectionIfExists returns an existing legacy or physical collection. When
// multiple embedding spaces exist, callers should use Collection with the
// intended embedding function instead.
func (s *VectorStore) CollectionIfExists(logicalName string) *chromem.Collection {
	if logicalName == "" {
		return nil
	}
	if coll := s.db.GetCollection(logicalName, nil); coll != nil {
		return coll
	}
	for name, coll := range s.db.ListCollections() {
		if isPhysicalCollectionFor(logicalName, name) {
			return coll
		}
	}
	return nil
}

// PhysicalCollections lists all collections currently associated with a logical
// scope. This is primarily used by repair and delete operations.
func (s *VectorStore) PhysicalCollections(logicalName string) []string {
	if logicalName == "" {
		return nil
	}
	var names []string
	for name := range s.db.ListCollections() {
		if isPhysicalCollectionFor(logicalName, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ExportToWriter serializes the entire chromem DB.
func (s *VectorStore) ExportToWriter(w io.Writer, compress bool, encryptionKey string) error {
	if err := s.db.ExportToWriter(w, compress, encryptionKey); err != nil {
		return fmt.Errorf("chromem export: %w", err)
	}
	return nil
}

// ImportFromReader replaces the chromem DB content with the supplied data.
func (s *VectorStore) ImportFromReader(r io.ReadSeeker, encryptionKey string) error {
	if err := s.db.ImportFromReader(r, encryptionKey); err != nil {
		return fmt.Errorf("chromem import: %w", err)
	}
	return nil
}

// DeleteCollection removes every embedding-space generation associated with a
// logical collection, including the legacy unsuffixed collection.
func (s *VectorStore) DeleteCollection(logicalName string) error {
	if logicalName == "" {
		return nil
	}
	var firstErr error
	for _, name := range s.PhysicalCollections(logicalName) {
		if err := s.db.DeleteCollection(name); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete collection %s: %w", name, err)
		}
	}
	return firstErr
}

// IndexChunks embeds and stores the given chunks. Hosted providers are embedded
// in bounded batches; custom/test functions retain chromem's historical
// concurrent single-item path.
func (s *VectorStore) IndexChunks(
	ctx context.Context,
	logicalName string,
	chunks []models.DocumentChunk,
	providerType string,
	embedFunc chromem.EmbeddingFunc,
) error {
	if len(chunks) == 0 {
		return nil
	}
	if embedFunc == nil {
		return fmt.Errorf("embedding func is required")
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

	if rt, ok := embeddingRuntimeFor(embedFunc); ok && rt.batch != nil {
		for start := 0; start < len(docs); start += defaultEmbeddingBatchSize {
			end := start + defaultEmbeddingBatchSize
			if end > len(docs) {
				end = len(docs)
			}
			texts := make([]string, end-start)
			for i := start; i < end; i++ {
				texts[i-start] = docs[i].Content
			}
			vectors, err := rt.batch(ctx, texts)
			if err != nil {
				return fmt.Errorf("embed documents %d-%d for %s: %w", start, end-1, logicalName, err)
			}
			if len(vectors) != len(texts) {
				return fmt.Errorf("embedding result count mismatch for %s: got %d, want %d", logicalName, len(vectors), len(texts))
			}
			for i, vector := range vectors {
				docIndex := start + i
				docs[docIndex].Embedding = vector
				docs[docIndex].Metadata["embedding_provider"] = rt.space.Provider
				docs[docIndex].Metadata["embedding_model"] = rt.space.Model
				docs[docIndex].Metadata["embedding_space"] = rt.space.RoutingFingerprint()
				docs[docIndex].Metadata["embedding_dimensions"] = strconv.Itoa(len(vector))
			}
		}
	}

	coll, err := s.Collection(ctx, logicalName, embedFunc)
	if err != nil {
		return err
	}

	concurrency := runtime.NumCPU()
	if concurrency < 1 {
		concurrency = 1
	}
	if !strings.EqualFold(providerType, "ollama") && concurrency > 4 {
		concurrency = 4
	}
	// Precomputed batches no longer make provider calls, so use moderate write
	// concurrency without creating hundreds of persistence goroutines.
	if len(docs) > 0 && len(docs[0].Embedding) > 0 && concurrency > 8 {
		concurrency = 8
	}

	if err := coll.AddDocuments(ctx, docs, concurrency); err != nil {
		return fmt.Errorf("add documents to %s: %w", coll.Name, err)
	}
	return nil
}

// DeleteDocuments removes the given chunk IDs from every physical
// embedding-space collection associated with the logical scope.
func (s *VectorStore) DeleteDocuments(ctx context.Context, logicalName string, chunkIDs ...string) error {
	if len(chunkIDs) == 0 || logicalName == "" {
		return nil
	}
	var firstErr error
	for _, name := range s.PhysicalCollections(logicalName) {
		coll := s.db.GetCollection(name, nil)
		if coll == nil {
			continue
		}
		if err := coll.Delete(ctx, nil, nil, chunkIDs...); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete documents from %s: %w", name, err)
		}
	}
	return firstErr
}

// QuerySimilar runs semantic search against the physical embedding space
// associated with embedFunc.
func (s *VectorStore) QuerySimilar(
	ctx context.Context,
	logicalName string,
	query string,
	topK int,
	embedFunc chromem.EmbeddingFunc,
) ([]QueryResult, error) {
	if strings.TrimSpace(logicalName) == "" {
		return nil, fmt.Errorf("collection name is required")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}

	coll, err := s.Collection(ctx, logicalName, embedFunc)
	if err != nil {
		return nil, err
	}
	if coll.Count() == 0 {
		return nil, nil
	}
	if topK > coll.Count() {
		topK = coll.Count()
	}

	results, err := coll.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query collection %s: %w", coll.Name, err)
	}
	out := make([]QueryResult, 0, len(results))
	for _, result := range results {
		out = append(out, QueryResult{ID: result.ID, Similarity: float64(result.Similarity)})
	}
	return out, nil
}
