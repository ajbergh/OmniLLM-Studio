package rag

import (
	"container/heap"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"
)

// VectorRecord is a backend-neutral vector document.
type VectorRecord struct {
	ID       string
	Vector   []float32
	Metadata map[string]string
}

// VectorHit is a backend-neutral nearest-neighbor result.
type VectorHit struct {
	ID         string
	Similarity float64
	Metadata   map[string]string
}

// IndexSpec identifies one immutable vector index generation.
type IndexSpec struct {
	GenerationID string
	Space        EmbeddingSpace
}

// IndexStats describes the health of an index generation.
type IndexStats struct {
	GenerationID string `json:"generation_id"`
	Records      int    `json:"records"`
	Dimensions   int    `json:"dimensions"`
	Backend      string `json:"backend"`
}

// VectorIndex is the pluggable contract for exact, chromem, HNSW, or future
// remote vector backends.
type VectorIndex interface {
	CreateGeneration(ctx context.Context, spec IndexSpec) error
	UpsertBatch(ctx context.Context, generationID string, records []VectorRecord) error
	Search(ctx context.Context, generationID string, query []float32, topK int) ([]VectorHit, error)
	Delete(ctx context.Context, generationID string, ids []string) error
	Validate(ctx context.Context, generationID string) (IndexStats, error)
	Drop(ctx context.Context, generationID string) error
}

// ExactVectorIndex is a pure-Go exact-search implementation intended for local
// and small-to-medium indexes. Vectors are normalized on insert, so cosine
// similarity is a dot product at query time.
type ExactVectorIndex struct {
	mu          sync.RWMutex
	generations map[string]*exactGeneration
}

type exactGeneration struct {
	spec    IndexSpec
	dim     int
	records map[string]VectorRecord
}

func NewExactVectorIndex() *ExactVectorIndex {
	return &ExactVectorIndex{generations: make(map[string]*exactGeneration)}
}

func (i *ExactVectorIndex) CreateGeneration(_ context.Context, spec IndexSpec) error {
	if spec.GenerationID == "" {
		return fmt.Errorf("generation id is required")
	}
	if err := spec.Space.Validate(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, exists := i.generations[spec.GenerationID]; exists {
		return nil
	}
	i.generations[spec.GenerationID] = &exactGeneration{
		spec:    spec,
		dim:     spec.Space.Dimensions,
		records: make(map[string]VectorRecord),
	}
	return nil
}

func (i *ExactVectorIndex) UpsertBatch(_ context.Context, generationID string, records []VectorRecord) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	generation, ok := i.generations[generationID]
	if !ok {
		return fmt.Errorf("generation %q not found", generationID)
	}
	for _, record := range records {
		if record.ID == "" {
			return fmt.Errorf("vector record id is required")
		}
		if len(record.Vector) == 0 {
			return fmt.Errorf("vector record %q is empty", record.ID)
		}
		if generation.dim == 0 {
			generation.dim = len(record.Vector)
		}
		if len(record.Vector) != generation.dim {
			return fmt.Errorf("vector record %q has %d dimensions, want %d", record.ID, len(record.Vector), generation.dim)
		}
		copyRecord := record
		copyRecord.Vector = normalizeVectorCopy(record.Vector)
		copyRecord.Metadata = cloneStringMap(record.Metadata)
		generation.records[record.ID] = copyRecord
	}
	return nil
}

func (i *ExactVectorIndex) Search(_ context.Context, generationID string, query []float32, topK int) ([]VectorHit, error) {
	if len(query) == 0 {
		return nil, nil
	}
	i.mu.RLock()
	generation, ok := i.generations[generationID]
	if !ok {
		i.mu.RUnlock()
		return nil, fmt.Errorf("generation %q not found", generationID)
	}
	if generation.dim != len(query) {
		i.mu.RUnlock()
		return nil, fmt.Errorf("query has %d dimensions, want %d", len(query), generation.dim)
	}
	query = normalizeVectorCopy(query)
	hits := make([]VectorHit, 0, len(generation.records))
	for _, record := range generation.records {
		hits = append(hits, VectorHit{
			ID:         record.ID,
			Similarity: dotNormalized(query, record.Vector),
			Metadata:   cloneStringMap(record.Metadata),
		})
	}
	i.mu.RUnlock()

	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].Similarity == hits[b].Similarity {
			return hits[a].ID < hits[b].ID
		}
		return hits[a].Similarity > hits[b].Similarity
	})
	if topK <= 0 {
		topK = 5
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

func (i *ExactVectorIndex) Delete(_ context.Context, generationID string, ids []string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	generation, ok := i.generations[generationID]
	if !ok {
		return nil
	}
	for _, id := range ids {
		delete(generation.records, id)
	}
	return nil
}

func (i *ExactVectorIndex) Validate(_ context.Context, generationID string) (IndexStats, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	generation, ok := i.generations[generationID]
	if !ok {
		return IndexStats{}, fmt.Errorf("generation %q not found", generationID)
	}
	for id, record := range generation.records {
		if len(record.Vector) != generation.dim {
			return IndexStats{}, fmt.Errorf("record %q dimension mismatch", id)
		}
	}
	return IndexStats{
		GenerationID: generationID,
		Records:      len(generation.records),
		Dimensions:   generation.dim,
		Backend:      "exact-go",
	}, nil
}

func (i *ExactVectorIndex) Drop(_ context.Context, generationID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.generations, generationID)
	return nil
}

func dotNormalized(a, b []float32) float64 {
	var dot float64
	for index := range a {
		dot += float64(a[index]) * float64(b[index])
	}
	return dot
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// HNSWConfig configures the pure-Go approximate nearest-neighbor backend.
type HNSWConfig struct {
	M              int
	EFConstruction int
	EFSearch       int
	Seed           int64
}

func (c HNSWConfig) normalized() HNSWConfig {
	if c.M <= 0 {
		c.M = 16
	}
	if c.EFConstruction < c.M {
		c.EFConstruction = maxInt(64, c.M*4)
	}
	if c.EFSearch <= 0 {
		c.EFSearch = 64
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
	return c
}

// HNSWVectorIndex is a pure-Go HNSW implementation. It favors predictable
// portability and correctness over micro-optimizations; the exact backend
// remains the reference implementation used for recall validation.
type HNSWVectorIndex struct {
	mu          sync.RWMutex
	config      HNSWConfig
	rng         *rand.Rand
	generations map[string]*hnswGeneration
}

type hnswGeneration struct {
	Spec       IndexSpec
	Dimensions int
	EntryID    string
	MaxLevel   int
	Nodes      map[string]*hnswNode
}

type hnswNode struct {
	ID        string
	Vector    []float32
	Metadata  map[string]string
	Level     int
	Neighbors [][]string
	Deleted   bool
}

func NewHNSWVectorIndex(config HNSWConfig) *HNSWVectorIndex {
	config = config.normalized()
	return &HNSWVectorIndex{
		config:      config,
		rng:         rand.New(rand.NewSource(config.Seed)), // #nosec G404 -- deterministic ANN level generation
		generations: make(map[string]*hnswGeneration),
	}
}

func (h *HNSWVectorIndex) CreateGeneration(_ context.Context, spec IndexSpec) error {
	if spec.GenerationID == "" {
		return fmt.Errorf("generation id is required")
	}
	if err := spec.Space.Validate(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.generations[spec.GenerationID]; exists {
		return nil
	}
	h.generations[spec.GenerationID] = &hnswGeneration{
		Spec:       spec,
		Dimensions: spec.Space.Dimensions,
		MaxLevel:   -1,
		Nodes:      make(map[string]*hnswNode),
	}
	return nil
}

func (h *HNSWVectorIndex) UpsertBatch(ctx context.Context, generationID string, records []VectorRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	generation, ok := h.generations[generationID]
	if !ok {
		return fmt.Errorf("generation %q not found", generationID)
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if record.ID == "" || len(record.Vector) == 0 {
			return fmt.Errorf("vector record id and vector are required")
		}
		if generation.Dimensions == 0 {
			generation.Dimensions = len(record.Vector)
		}
		if len(record.Vector) != generation.Dimensions {
			return fmt.Errorf("record %q has %d dimensions, want %d", record.ID, len(record.Vector), generation.Dimensions)
		}
		if old := generation.Nodes[record.ID]; old != nil {
			h.removeNodeLinks(generation, old)
			delete(generation.Nodes, record.ID)
			if generation.EntryID == record.ID {
				generation.EntryID = ""
				generation.MaxLevel = -1
			}
		}
		if err := h.insert(generation, record); err != nil {
			return err
		}
	}
	if generation.EntryID == "" && len(generation.Nodes) > 0 {
		h.reselectEntry(generation)
	}
	return nil
}

func (h *HNSWVectorIndex) insert(g *hnswGeneration, record VectorRecord) error {
	level := h.randomLevel()
	node := &hnswNode{
		ID:        record.ID,
		Vector:    normalizeVectorCopy(record.Vector),
		Metadata:  cloneStringMap(record.Metadata),
		Level:     level,
		Neighbors: make([][]string, level+1),
	}
	if len(g.Nodes) == 0 {
		g.Nodes[node.ID] = node
		g.EntryID = node.ID
		g.MaxLevel = level
		return nil
	}

	// Make the node visible before establishing reciprocal edges. Otherwise
	// neighbor pruning treats the new ID as missing and removes every backlink,
	// leaving the graph reachable only from newest to oldest nodes.
	g.Nodes[node.ID] = node

	entry := g.Nodes[g.EntryID]
	if entry == nil {
		h.reselectEntry(g)
		entry = g.Nodes[g.EntryID]
	}
	current := entry
	for layer := g.MaxLevel; layer > level; layer-- {
		current = h.greedyClosest(g, node.Vector, current, layer)
	}

	maxLayer := minInt(level, g.MaxLevel)
	for layer := maxLayer; layer >= 0; layer-- {
		candidates := h.searchLayer(g, node.Vector, []string{current.ID}, h.config.EFConstruction, layer)
		neighbors := selectNearest(candidates, h.config.M)
		for _, candidate := range neighbors {
			node.Neighbors[layer] = appendUnique(node.Neighbors[layer], candidate.id)
			other := g.Nodes[candidate.id]
			if other == nil || other.Deleted || layer > other.Level {
				continue
			}
			other.Neighbors[layer] = appendUnique(other.Neighbors[layer], node.ID)
			h.pruneNeighbors(g, other, layer)
		}
		if len(neighbors) > 0 {
			current = g.Nodes[neighbors[0].id]
		}
	}

	if level > g.MaxLevel {
		g.EntryID = node.ID
		g.MaxLevel = level
	}
	return nil
}

func (h *HNSWVectorIndex) Search(ctx context.Context, generationID string, query []float32, topK int) ([]VectorHit, error) {
	if len(query) == 0 {
		return nil, nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	g, ok := h.generations[generationID]
	if !ok {
		return nil, fmt.Errorf("generation %q not found", generationID)
	}
	if g.Dimensions != len(query) {
		return nil, fmt.Errorf("query has %d dimensions, want %d", len(query), g.Dimensions)
	}
	entry := g.Nodes[g.EntryID]
	if entry == nil || entry.Deleted {
		return nil, nil
	}
	query = normalizeVectorCopy(query)
	current := entry
	for layer := g.MaxLevel; layer > 0; layer-- {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current = h.greedyClosest(g, query, current, layer)
	}
	ef := h.config.EFSearch
	if topK <= 0 {
		topK = 5
	}
	if ef < topK {
		ef = topK
	}
	candidates := h.searchLayer(g, query, []string{current.ID}, ef, 0)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	hits := make([]VectorHit, 0, len(candidates))
	for _, candidate := range candidates {
		node := g.Nodes[candidate.id]
		if node == nil || node.Deleted {
			continue
		}
		hits = append(hits, VectorHit{
			ID:         node.ID,
			Similarity: 1 - candidate.distance,
			Metadata:   cloneStringMap(node.Metadata),
		})
	}
	return hits, nil
}

func (h *HNSWVectorIndex) Delete(_ context.Context, generationID string, ids []string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.generations[generationID]
	if !ok {
		return nil
	}
	for _, id := range ids {
		if node := g.Nodes[id]; node != nil {
			node.Deleted = true
		}
	}
	if entry := g.Nodes[g.EntryID]; entry == nil || entry.Deleted {
		h.reselectEntry(g)
	}
	return nil
}

func (h *HNSWVectorIndex) Validate(_ context.Context, generationID string) (IndexStats, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	g, ok := h.generations[generationID]
	if !ok {
		return IndexStats{}, fmt.Errorf("generation %q not found", generationID)
	}
	active := 0
	for id, node := range g.Nodes {
		if len(node.Vector) != g.Dimensions {
			return IndexStats{}, fmt.Errorf("node %q dimension mismatch", id)
		}
		if len(node.Neighbors) != node.Level+1 {
			return IndexStats{}, fmt.Errorf("node %q level metadata mismatch", id)
		}
		if !node.Deleted {
			active++
		}
		for level, neighbors := range node.Neighbors {
			for _, neighborID := range neighbors {
				neighbor := g.Nodes[neighborID]
				if neighbor == nil {
					return IndexStats{}, fmt.Errorf("node %q level %d references missing node %q", id, level, neighborID)
				}
			}
		}
	}
	return IndexStats{GenerationID: generationID, Records: active, Dimensions: g.Dimensions, Backend: "hnsw-go"}, nil
}

func (h *HNSWVectorIndex) Drop(_ context.Context, generationID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.generations, generationID)
	return nil
}

// Save persists all generations using Go's gob format.
func (h *HNSWVectorIndex) Save(path string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return gob.NewEncoder(file).Encode(h.generations)
}

// Load replaces all in-memory generations from a gob snapshot.
func (h *HNSWVectorIndex) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	generations := make(map[string]*hnswGeneration)
	if err := gob.NewDecoder(file).Decode(&generations); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.generations = generations
	return nil
}

func (h *HNSWVectorIndex) randomLevel() int {
	level := 0
	probability := 1.0 / math.Log(float64(h.config.M)+1)
	for level < 32 && h.rng.Float64() < probability {
		level++
	}
	return level
}

func (h *HNSWVectorIndex) greedyClosest(g *hnswGeneration, query []float32, entry *hnswNode, level int) *hnswNode {
	current := entry
	currentDistance := cosineDistance(query, current.Vector)
	for {
		changed := false
		if level > current.Level {
			return current
		}
		for _, neighborID := range current.Neighbors[level] {
			neighbor := g.Nodes[neighborID]
			if neighbor == nil || neighbor.Deleted || level > neighbor.Level {
				continue
			}
			distance := cosineDistance(query, neighbor.Vector)
			if distance < currentDistance {
				current = neighbor
				currentDistance = distance
				changed = true
			}
		}
		if !changed {
			return current
		}
	}
}

func (h *HNSWVectorIndex) searchLayer(g *hnswGeneration, query []float32, entryIDs []string, ef, level int) []distanceCandidate {
	visited := make(map[string]struct{}, ef*2)
	candidates := &minCandidateHeap{}
	results := &maxCandidateHeap{}
	heap.Init(candidates)
	heap.Init(results)

	for _, id := range entryIDs {
		node := g.Nodes[id]
		if node == nil || node.Deleted || level > node.Level {
			continue
		}
		distance := cosineDistance(query, node.Vector)
		candidate := distanceCandidate{id: id, distance: distance}
		heap.Push(candidates, candidate)
		heap.Push(results, candidate)
		visited[id] = struct{}{}
	}

	for candidates.Len() > 0 {
		nearest := heap.Pop(candidates).(distanceCandidate)
		if results.Len() >= ef && nearest.distance > (*results)[0].distance {
			break
		}
		node := g.Nodes[nearest.id]
		if node == nil || level > node.Level {
			continue
		}
		for _, neighborID := range node.Neighbors[level] {
			if _, seen := visited[neighborID]; seen {
				continue
			}
			visited[neighborID] = struct{}{}
			neighbor := g.Nodes[neighborID]
			if neighbor == nil || neighbor.Deleted || level > neighbor.Level {
				continue
			}
			distance := cosineDistance(query, neighbor.Vector)
			if results.Len() < ef || distance < (*results)[0].distance {
				candidate := distanceCandidate{id: neighborID, distance: distance}
				heap.Push(candidates, candidate)
				heap.Push(results, candidate)
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	out := make([]distanceCandidate, results.Len())
	for index := len(out) - 1; index >= 0; index-- {
		out[index] = heap.Pop(results).(distanceCandidate)
	}
	return out
}

func (h *HNSWVectorIndex) pruneNeighbors(g *hnswGeneration, node *hnswNode, level int) {
	if level > node.Level || len(node.Neighbors[level]) <= h.config.M {
		return
	}
	candidates := make([]distanceCandidate, 0, len(node.Neighbors[level]))
	for _, id := range node.Neighbors[level] {
		other := g.Nodes[id]
		if other == nil || other.Deleted {
			continue
		}
		candidates = append(candidates, distanceCandidate{id: id, distance: cosineDistance(node.Vector, other.Vector)})
	}
	node.Neighbors[level] = nil
	for _, candidate := range selectNearest(candidates, h.config.M) {
		node.Neighbors[level] = append(node.Neighbors[level], candidate.id)
	}
}

func (h *HNSWVectorIndex) removeNodeLinks(g *hnswGeneration, node *hnswNode) {
	for level, neighbors := range node.Neighbors {
		for _, id := range neighbors {
			other := g.Nodes[id]
			if other == nil || level > other.Level {
				continue
			}
			other.Neighbors[level] = removeString(other.Neighbors[level], node.ID)
		}
	}
}

func (h *HNSWVectorIndex) reselectEntry(g *hnswGeneration) {
	g.EntryID = ""
	g.MaxLevel = -1
	for id, node := range g.Nodes {
		if node.Deleted {
			continue
		}
		if node.Level > g.MaxLevel || (node.Level == g.MaxLevel && (g.EntryID == "" || id < g.EntryID)) {
			g.EntryID = id
			g.MaxLevel = node.Level
		}
	}
}

type distanceCandidate struct {
	id       string
	distance float64
}

type minCandidateHeap []distanceCandidate

func (h minCandidateHeap) Len() int           { return len(h) }
func (h minCandidateHeap) Less(i, j int) bool { return h[i].distance < h[j].distance }
func (h minCandidateHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minCandidateHeap) Push(x any)        { *h = append(*h, x.(distanceCandidate)) }
func (h *minCandidateHeap) Pop() any {
	old := *h
	value := old[len(old)-1]
	*h = old[:len(old)-1]
	return value
}

type maxCandidateHeap []distanceCandidate

func (h maxCandidateHeap) Len() int           { return len(h) }
func (h maxCandidateHeap) Less(i, j int) bool { return h[i].distance > h[j].distance }
func (h maxCandidateHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *maxCandidateHeap) Push(x any)        { *h = append(*h, x.(distanceCandidate)) }
func (h *maxCandidateHeap) Pop() any {
	old := *h
	value := old[len(old)-1]
	*h = old[:len(old)-1]
	return value
}

func selectNearest(candidates []distanceCandidate, limit int) []distanceCandidate {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func cosineDistance(a, b []float32) float64 {
	return 1 - dotNormalized(a, b)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func removeString(values []string, value string) []string {
	for index, existing := range values {
		if existing == value {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GenerationCoordinator builds a replacement vector generation alongside the
// active generation, validates it, and atomically activates it through SQLite.
type GenerationCoordinator struct {
	repo    *repository.RAGIndexRepo
	backend VectorIndex
}

func NewGenerationCoordinator(repo *repository.RAGIndexRepo, backend VectorIndex) *GenerationCoordinator {
	return &GenerationCoordinator{repo: repo, backend: backend}
}

type BuildGenerationRequest struct {
	LogicalName      string
	ScopeType        string
	ScopeID          string
	BackendName      string
	EmbeddingSpaceID string
	Space            EmbeddingSpace
	ParserVersion    int
	ChunkerVersion   int
	Records          []VectorRecord
	BatchSize        int
}

type BuildGenerationResult struct {
	IndexID      string
	GenerationID string
	Stats        IndexStats
}

func (c *GenerationCoordinator) BuildAndActivate(ctx context.Context, req BuildGenerationRequest) (*BuildGenerationResult, error) {
	if c == nil || c.repo == nil || c.backend == nil {
		return nil, fmt.Errorf("generation coordinator is not configured")
	}
	if err := req.Space.Validate(); err != nil {
		return nil, err
	}
	indexID, err := c.repo.EnsureIndex(req.LogicalName, req.ScopeType, req.ScopeID, req.BackendName)
	if err != nil {
		return nil, err
	}
	generationID, err := c.repo.BeginGeneration(indexID, req.EmbeddingSpaceID, req.ParserVersion, req.ChunkerVersion, len(req.Records))
	if err != nil {
		return nil, err
	}
	fail := func(stageErr error, indexed int) error {
		message := stageErr.Error()
		_ = c.repo.UpdateGeneration(generationID, "failed", indexed, &message)
		_ = c.backend.Drop(context.Background(), generationID)
		return stageErr
	}

	if err := c.backend.CreateGeneration(ctx, IndexSpec{GenerationID: generationID, Space: req.Space}); err != nil {
		return nil, fail(fmt.Errorf("create generation: %w", err), 0)
	}
	if err := c.repo.UpdateGeneration(generationID, "indexing", 0, nil); err != nil {
		return nil, fail(err, 0)
	}
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 256
	}
	indexed := 0
	for start := 0; start < len(req.Records); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, fail(err, indexed)
		}
		end := start + batchSize
		if end > len(req.Records) {
			end = len(req.Records)
		}
		if err := c.backend.UpsertBatch(ctx, generationID, req.Records[start:end]); err != nil {
			return nil, fail(fmt.Errorf("index records %d-%d: %w", start, end-1, err), indexed)
		}
		indexed = end
		if err := c.repo.UpdateGeneration(generationID, "indexing", indexed, nil); err != nil {
			return nil, fail(err, indexed)
		}
	}

	if err := c.repo.UpdateGeneration(generationID, "validating", indexed, nil); err != nil {
		return nil, fail(err, indexed)
	}
	stats, err := c.backend.Validate(ctx, generationID)
	if err != nil {
		return nil, fail(fmt.Errorf("validate generation: %w", err), indexed)
	}
	if stats.Records != len(req.Records) {
		return nil, fail(fmt.Errorf("generation contains %d records, want %d", stats.Records, len(req.Records)), indexed)
	}
	if err := c.repo.UpdateGeneration(generationID, "ready", indexed, nil); err != nil {
		return nil, fail(err, indexed)
	}
	if err := c.repo.ActivateGeneration(indexID, generationID); err != nil {
		return nil, fail(fmt.Errorf("activate generation: %w", err), indexed)
	}
	return &BuildGenerationResult{IndexID: indexID, GenerationID: generationID, Stats: stats}, nil
}
