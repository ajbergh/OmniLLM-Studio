package rag

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	chromem "github.com/philippgille/chromem-go"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// ChromemRetriever finds relevant chunks for a query using a chromem-go
// vector store. Hydrates chunk content from the SQLite document_chunks table.
type ChromemRetriever struct {
	llmService    *llm.Service
	chunkRepo     *repository.ChunkRepo
	vectorStore   *VectorStore
	legacyEmbeds  *repository.EmbeddingRepo // optional — used for one-shot lazy migration
	migrating     sync.Map                  // conversationID → *sync.Once to dedupe migration attempts
	migrateLogged sync.Map                  // conversationID → bool to avoid log spam after a successful migration
	testEmbedFn   chromem.EmbeddingFunc     // test-only override; nil in production
}

// NewChromemRetriever creates a ChromemRetriever.
func NewChromemRetriever(
	llmService *llm.Service,
	chunkRepo *repository.ChunkRepo,
	vectorStore *VectorStore,
) *ChromemRetriever {
	return &ChromemRetriever{
		llmService:  llmService,
		chunkRepo:   chunkRepo,
		vectorStore: vectorStore,
	}
}

// WithLegacyEmbeddingRepo enables lazy migration from the SQLite
// document_embeddings table when a chromem collection is found empty but
// legacy data exists for the conversation. Optional — leave unset to disable.
func (r *ChromemRetriever) WithLegacyEmbeddingRepo(repo *repository.EmbeddingRepo) *ChromemRetriever {
	r.legacyEmbeds = repo
	return r
}

// Retrieve returns the top-k most similar chunks for the given query text,
// scoped to a single conversation.
func (r *ChromemRetriever) Retrieve(
	ctx context.Context,
	conversationID string,
	query string,
	provider string,
	model string,
	topK int,
) ([]RetrievedChunk, error) {
	if topK <= 0 {
		topK = 5
	}
	if r.vectorStore == nil {
		return nil, fmt.Errorf("chromem retriever: vector store is nil")
	}

	var embedFunc chromem.EmbeddingFunc
	if r.testEmbedFn != nil {
		embedFunc = r.testEmbedFn
	} else {
		embedFunc = NewLLMEmbeddingFunc(r.llmService, provider, model)
	}
	coll, err := r.vectorStore.Collection(ctx, conversationID, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("get/create collection: %w", err)
	}

	if coll.Count() == 0 {
		if migrated := r.tryLazyMigrate(ctx, conversationID, coll); !migrated {
			return nil, nil
		}
	}

	// Cap topK to collection size — chromem returns an error if you ask for
	// more results than exist.
	if n := coll.Count(); topK > n {
		topK = n
	}

	results, err := coll.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem query: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	ids := make([]string, len(results))
	for i, res := range results {
		ids[i] = res.ID
	}

	chunks, err := r.chunkRepo.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("hydrate chunks: %w", err)
	}
	chunkMap := make(map[string]models.DocumentChunk, len(chunks))
	for _, c := range chunks {
		chunkMap[c.ID] = c
	}

	out := make([]RetrievedChunk, 0, len(results))
	for _, res := range results {
		if c, ok := chunkMap[res.ID]; ok {
			out = append(out, RetrievedChunk{
				Chunk: c,
				Score: float64(res.Similarity),
			})
		}
	}
	return out, nil
}

// tryLazyMigrate attempts to populate an empty chromem collection from legacy
// document_embeddings rows. Returns true if at least one document was added.
// Each conversation only gets one migration attempt per process lifetime,
// guarded by a sync.Once. Best-effort — failures are logged and swallowed.
func (r *ChromemRetriever) tryLazyMigrate(ctx context.Context, conversationID string, coll *chromem.Collection) bool {
	if r.legacyEmbeds == nil || r.chunkRepo == nil {
		return false
	}

	onceVal, _ := r.migrating.LoadOrStore(conversationID, &sync.Once{})
	once := onceVal.(*sync.Once)

	var added int
	once.Do(func() {
		added = r.migrateLegacyEmbeddings(ctx, conversationID, coll)
		if added > 0 {
			if _, logged := r.migrateLogged.LoadOrStore(conversationID, true); !logged {
				log.Printf("[rag] lazy-migrated %d legacy embeddings into chromem for conversation %s", added, conversationID)
			}
		}
	})

	// If migration ran in another call (Once already fired) and the collection
	// is now populated, return true so the caller proceeds to query it.
	return coll.Count() > 0
}

// migrateLegacyEmbeddings reads chunks + their pre-computed embeddings from
// SQLite and writes them straight into the chromem collection (no re-embed).
// Returns the number of documents written.
func (r *ChromemRetriever) migrateLegacyEmbeddings(ctx context.Context, conversationID string, coll *chromem.Collection) int {
	chunks, err := r.chunkRepo.ListByConversation(conversationID)
	if err != nil {
		log.Printf("[rag] lazy-migrate: list chunks for %s: %v", conversationID, err)
		return 0
	}
	if len(chunks) == 0 {
		return 0
	}

	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	embeddings, err := r.legacyEmbeds.GetByChunkIDs(chunkIDs)
	if err != nil {
		log.Printf("[rag] lazy-migrate: read legacy embeddings for %s: %v", conversationID, err)
		return 0
	}
	if len(embeddings) == 0 {
		return 0
	}

	docs := make([]chromem.Document, 0, len(chunks))
	for _, c := range chunks {
		vec, ok := embeddings[c.ID]
		if !ok || len(vec) == 0 {
			continue
		}
		docs = append(docs, chromem.Document{
			ID:        c.ID,
			Content:   c.Content,
			Embedding: vec,
			Metadata: map[string]string{
				"attachment_id":   c.AttachmentID,
				"conversation_id": c.ConversationID,
				"chunk_index":     strconv.Itoa(c.ChunkIndex),
			},
		})
	}
	if len(docs) == 0 {
		return 0
	}

	// Concurrency=1: pre-computed embeddings, no embed calls; chromem just
	// inserts into its in-memory map. The serial path is plenty fast.
	if err := coll.AddDocuments(ctx, docs, 1); err != nil {
		log.Printf("[rag] lazy-migrate: AddDocuments for %s: %v", conversationID, err)
		return 0
	}
	return len(docs)
}
