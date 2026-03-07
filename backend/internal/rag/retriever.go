package rag

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// RetrievedChunk is a chunk with its similarity score.
type RetrievedChunk struct {
	Chunk models.DocumentChunk
	Score float64 // cosine similarity in [−1, 1]
}

// Retriever finds the most relevant chunks for a query using embedding
// similarity.
type Retriever struct {
	llmService    *llm.Service
	chunkRepo     *repository.ChunkRepo
	embeddingRepo *repository.EmbeddingRepo
}

// NewRetriever creates a Retriever.
func NewRetriever(
	llmService *llm.Service,
	chunkRepo *repository.ChunkRepo,
	embeddingRepo *repository.EmbeddingRepo,
) *Retriever {
	return &Retriever{
		llmService:    llmService,
		chunkRepo:     chunkRepo,
		embeddingRepo: embeddingRepo,
	}
}

// Retrieve returns the top-k most similar chunks for the given query text,
// scoped to a single conversation. The provider/model are used to generate
// the query embedding so they should match whatever was used at indexing time.
func (r *Retriever) Retrieve(
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

	// 1. Embed the query using the same provider/model as the stored embeddings.
	embResp, err := r.llmService.Embed(ctx, llm.EmbeddingRequest{
		Provider: provider,
		Model:    model,
		Input:    []string{query},
	})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	queryVec := embResp.Embeddings[0]

	// 2. Load all stored embeddings for the conversation.
	rows, err := r.embeddingRepo.GetAllForConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation embeddings: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil // no indexed docs yet
	}

	// 3. Compute cosine similarity for each chunk.
	scored := make([]RetrievedChunk, 0, len(rows))
	for _, row := range rows {
		sim := cosineSimilarity(queryVec, row.Embedding)
		scored = append(scored, RetrievedChunk{
			Chunk: models.DocumentChunk{ID: row.ChunkID},
			Score: sim,
		})
	}

	// 4. Sort by score descending and take top-k.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}

	// 5. Hydrate the actual chunk content.
	ids := make([]string, len(scored))
	for i, s := range scored {
		ids[i] = s.Chunk.ID
	}
	chunks, err := r.chunkRepo.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("get chunks by ids: %w", err)
	}

	chunkMap := make(map[string]models.DocumentChunk, len(chunks))
	for _, c := range chunks {
		chunkMap[c.ID] = c
	}

	// Re-attach hydrated chunks keeping the score ordering.
	result := make([]RetrievedChunk, 0, len(scored))
	for _, s := range scored {
		if c, ok := chunkMap[s.Chunk.ID]; ok {
			result = append(result, RetrievedChunk{Chunk: c, Score: s.Score})
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
