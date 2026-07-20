package rag

import (
	"context"
	"fmt"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

func TestContextPlannerBudgetAndTrustBoundary(t *testing.T) {
	planner := NewContextPlanner(ConservativeTokenEstimator{})
	plan := planner.Plan([]Evidence{
		{ID: "1", SourceType: "file", Text: strings.Repeat("alpha ", 300), Score: 1},
		{ID: "2", SourceType: "file", Text: strings.Repeat("beta ", 300), Score: 0.9},
	}, ContextPlanConfig{MaxTokens: 120, PerSourceMaxTokens: 80, MaxEvidence: 2})
	if plan.TokenCount > 120 {
		t.Fatalf("context exceeded budget: %d", plan.TokenCount)
	}
	if !strings.Contains(plan.Text, "untrusted source content") {
		t.Fatal("missing prompt-injection trust boundary")
	}
	if !plan.Truncated {
		t.Fatal("expected truncation")
	}
}

func TestContextPlannerDeduplicatesContent(t *testing.T) {
	planner := NewContextPlanner(nil)
	plan := planner.Plan([]Evidence{
		{SourceType: "file", Text: "same evidence", Score: 1},
		{SourceType: "file", Text: "same   evidence", Score: 0.9},
	}, ContextPlanConfig{MaxTokens: 100})
	if len(plan.Evidence) != 1 {
		t.Fatalf("expected one deduplicated evidence item, got %d", len(plan.Evidence))
	}
}

type countingEmbedService struct {
	mu         sync.Mutex
	batchSizes []int
}

func (s *countingEmbedService) Embed(_ context.Context, request llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	s.mu.Lock()
	s.batchSizes = append(s.batchSizes, len(request.Input))
	s.mu.Unlock()
	vectors := make([][]float32, len(request.Input))
	for index := range request.Input {
		vectors[index] = []float32{1, float32(index + 1), 0.5}
	}
	return &llm.EmbeddingResponse{Embeddings: vectors, Model: request.Model, Dimensions: 3}, nil
}

func TestVectorStoreBatchesHostedEmbeddingsAndIsolatesSpace(t *testing.T) {
	service := &countingEmbedService{}
	embed := NewLLMEmbeddingFunc(service, "OpenAI", "text-embedding-3-small")
	store := NewInMemoryVectorStore()
	chunks := make([]models.DocumentChunk, 130)
	for index := range chunks {
		chunks[index] = models.DocumentChunk{
			ID: "chunk-" + string(rune(index+1000)), AttachmentID: "att", ConversationID: "conv",
			ChunkIndex: index, Content: "document text",
		}
	}
	if err := store.IndexChunks(context.Background(), "conversation:conv", chunks, "openai", embed); err != nil {
		t.Fatalf("IndexChunks: %v", err)
	}
	service.mu.Lock()
	got := append([]int(nil), service.batchSizes...)
	service.mu.Unlock()
	if len(got) != 3 || got[0] != 64 || got[1] != 64 || got[2] != 2 {
		t.Fatalf("unexpected embedding batches: %v", got)
	}
	space, ok := EmbeddingSpaceForFunc(embed)
	if !ok {
		t.Fatal("embedding function was not registered")
	}
	physical := PhysicalCollectionName("conversation:conv", space)
	if collection := store.db.GetCollection(physical, embed); collection == nil || collection.Count() != 130 {
		t.Fatalf("expected isolated physical collection with 130 documents")
	}
	if legacy := store.db.GetCollection("conversation:conv", embed); legacy != nil {
		t.Fatal("hosted embeddings must not be written to the legacy unversioned collection")
	}
}

func TestEmbeddingSpaceFingerprintIsolation(t *testing.T) {
	openAI := EmbeddingSpace{Provider: "OpenAI", Model: "text-embedding-3-small", Normalize: true}
	gemini := EmbeddingSpace{Provider: "Gemini", Model: "gemini-embedding-001", Normalize: true}
	if openAI.RoutingFingerprint() == gemini.RoutingFingerprint() {
		t.Fatal("different embedding providers/models must not share a routing fingerprint")
	}
	if PhysicalCollectionName("workspace:abc", openAI) == PhysicalCollectionName("workspace:abc", gemini) {
		t.Fatal("different embedding spaces must use different physical collections")
	}
	if PhysicalCollectionName("workspace:abc", openAI) != PhysicalCollectionName("workspace:abc", openAI) {
		t.Fatal("physical collection naming must be deterministic")
	}
}

func TestEmbeddingSpaceCanonicalDefaults(t *testing.T) {
	space := EmbeddingSpace{Provider: " OpenAI ", Model: "model"}.Canonical()
	if space.Provider != "openai" {
		t.Fatalf("provider not normalized: %q", space.Provider)
	}
	if space.DistanceMetric != "cosine" || space.SchemaVersion != EmbeddingSchemaVersion {
		t.Fatalf("defaults not applied: %#v", space)
	}
}

func TestReciprocalRankFusionRewardsAgreement(t *testing.T) {
	fused := ReciprocalRankFusion([]RankedList{
		{Name: "vector", Items: []RankedCandidate{{ID: "a"}, {ID: "b"}, {ID: "c"}}},
		{Name: "bm25", Items: []RankedCandidate{{ID: "b"}, {ID: "a"}, {ID: "d"}}},
	}, 60)
	if len(fused) != 4 {
		t.Fatalf("got %d fused candidates", len(fused))
	}
	if fused[0].ID != "a" && fused[0].ID != "b" {
		t.Fatalf("agreement candidates should lead, got %q", fused[0].ID)
	}
	if fused[0].Score <= fused[2].Score {
		t.Fatal("multi-channel candidate should outrank single-channel candidate")
	}
}

func TestMMRSelectDiversifiesSources(t *testing.T) {
	candidates := []RankedCandidate{
		{ID: "a1", Score: 1, SourceID: "a"},
		{ID: "a2", Score: 0.99, SourceID: "a"},
		{ID: "b1", Score: 0.98, SourceID: "b"},
	}
	selected := MMRSelect(candidates, 2, 0.75)
	if len(selected) != 2 {
		t.Fatalf("got %d candidates", len(selected))
	}
	if selected[0].SourceID == selected[1].SourceID {
		t.Fatalf("expected source diversity, got %#v", selected)
	}
}

func BenchmarkVectorIndexes(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("exact/%d", size), func(b *testing.B) {
			benchmarkVectorIndex(b, NewExactVectorIndex(), size)
		})
		b.Run(fmt.Sprintf("hnsw/%d", size), func(b *testing.B) {
			benchmarkVectorIndex(b, NewHNSWVectorIndex(HNSWConfig{}), size)
		})
	}
}

func benchmarkVectorIndex(b *testing.B, index VectorIndex, size int) {
	const dimensions = 384
	ctx := context.Background()
	space := EmbeddingSpace{Provider: "benchmark", Model: "synthetic", Dimensions: dimensions, SchemaVersion: EmbeddingSchemaVersion}
	if err := index.CreateGeneration(ctx, IndexSpec{GenerationID: "bench", Space: space}); err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewSource(42)) // #nosec G404 -- deterministic benchmark data
	records := make([]VectorRecord, size)
	for row := range records {
		vector := make([]float32, dimensions)
		for column := range vector {
			vector[column] = rng.Float32()*2 - 1
		}
		records[row] = VectorRecord{ID: fmt.Sprintf("v-%d", row), Vector: vector}
	}
	for start := 0; start < len(records); start += 512 {
		end := start + 512
		if end > len(records) {
			end = len(records)
		}
		if err := index.UpsertBatch(ctx, "bench", records[start:end]); err != nil {
			b.Fatal(err)
		}
	}
	query := records[len(records)/2].Vector
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := index.Search(ctx, "bench", query, 10); err != nil {
			b.Fatal(err)
		}
	}
}

func TestExactVectorIndexSearch(t *testing.T) {
	ctx := context.Background()
	index := NewExactVectorIndex()
	space := EmbeddingSpace{Provider: "test", Model: "test", Dimensions: 3, Normalize: true}
	if err := index.CreateGeneration(ctx, IndexSpec{GenerationID: "g", Space: space}); err != nil {
		t.Fatal(err)
	}
	if err := index.UpsertBatch(ctx, "g", []VectorRecord{
		{ID: "x", Vector: []float32{1, 0, 0}},
		{ID: "y", Vector: []float32{0, 1, 0}},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := index.Search(ctx, "g", []float32{0.9, 0.1, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "x" {
		t.Fatalf("unexpected hits: %#v", hits)
	}
}

func TestHNSWVectorIndexRecallAgainstExact(t *testing.T) {
	ctx := context.Background()
	space := EmbeddingSpace{Provider: "test", Model: "test", Dimensions: 12, Normalize: true}
	exact := NewExactVectorIndex()
	hnsw := NewHNSWVectorIndex(HNSWConfig{M: 12, EFConstruction: 96, EFSearch: 96, Seed: 42})
	for _, index := range []VectorIndex{exact, hnsw} {
		if err := index.CreateGeneration(ctx, IndexSpec{GenerationID: "g", Space: space}); err != nil {
			t.Fatal(err)
		}
	}
	rng := rand.New(rand.NewSource(7)) // #nosec G404 -- deterministic test data
	records := make([]VectorRecord, 300)
	for i := range records {
		vector := make([]float32, 12)
		for d := range vector {
			vector[d] = rng.Float32()*2 - 1
		}
		records[i] = VectorRecord{ID: fmt.Sprintf("v-%03d", i), Vector: vector}
	}
	if err := exact.UpsertBatch(ctx, "g", records); err != nil {
		t.Fatal(err)
	}
	if err := hnsw.UpsertBatch(ctx, "g", records); err != nil {
		t.Fatal(err)
	}

	matches := 0
	queries := 30
	for i := 0; i < queries; i++ {
		query := records[(i*7)%len(records)].Vector
		want, _ := exact.Search(ctx, "g", query, 1)
		got, err := hnsw.Search(ctx, "g", query, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) > 0 && got[0].ID == want[0].ID {
			matches++
		}
	}
	if recall := float64(matches) / float64(queries); recall < 0.90 {
		t.Fatalf("HNSW recall too low: %.2f", recall)
	}
}
