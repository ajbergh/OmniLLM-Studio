// Package search provides hybrid search (keyword + semantic) across messages
// and document chunks with reciprocal rank fusion for result merging.
package search

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// SearchMode constrains which retrieval methods are used.
type SearchMode string

const (
	ModeHybrid   SearchMode = "hybrid"
	ModeKeyword  SearchMode = "keyword"
	ModeSemantic SearchMode = "semantic"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	Type           string  `json:"type"` // "message" or "chunk"
	ConversationID string  `json:"conversation_id"`
	MessageID      string  `json:"message_id,omitempty"`
	ChunkID        string  `json:"chunk_id,omitempty"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	Role           string  `json:"role,omitempty"`
	Timestamp      string  `json:"timestamp,omitempty"`
}

// SearchOptions configures a search query.
type SearchOptions struct {
	Query              string     `json:"query"`
	Mode               SearchMode `json:"mode"`
	Limit              int        `json:"limit"`
	ConversationFilter string     `json:"conversation_id,omitempty"` // optional: restrict to one conversation
	ConversationKind   string     `json:"conversation_kind,omitempty"`
	UserID             string     `json:"-"` // if set, restrict results to this user's conversations
}

// ReindexStatus reports the progress of a reindex operation.
type ReindexStatus struct {
	Total    int    `json:"total"`
	Embedded int    `json:"embedded"`
	Status   string `json:"status"` // "running", "completed", "error"
}

// Service provides search functionality across messages and document chunks.
type Service struct {
	db               *sql.DB
	llmService       *llm.Service
	msgRepo          *repository.MessageRepo
	msgEmbeddingRepo *repository.MessageEmbeddingRepo
	settingsRepo     *repository.SettingsRepo
}

// NewService creates a new search Service.
func NewService(
	db *sql.DB,
	llmService *llm.Service,
	msgRepo *repository.MessageRepo,
	msgEmbeddingRepo *repository.MessageEmbeddingRepo,
	settingsRepo *repository.SettingsRepo,
) *Service {
	return &Service{
		db:               db,
		llmService:       llmService,
		msgRepo:          msgRepo,
		msgEmbeddingRepo: msgEmbeddingRepo,
		settingsRepo:     settingsRepo,
	}
}

// Search runs a search with the given options.
func (s *Service) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if opts.Query == "" {
		return nil, nil
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Mode == "" {
		opts.Mode = ModeHybrid
	}
	if opts.ConversationKind == "" {
		opts.ConversationKind = models.ConversationKindChat
	}

	var keywordResults []scoredResult
	var semanticResults []scoredResult

	if opts.Mode == ModeKeyword || opts.Mode == ModeHybrid {
		kw, err := s.keywordSearch(opts)
		if err != nil {
			return nil, fmt.Errorf("keyword search: %w", err)
		}
		keywordResults = kw
	}

	if opts.Mode == ModeSemantic || opts.Mode == ModeHybrid {
		sem, err := s.semanticSearch(ctx, opts)
		if err != nil {
			// Semantic search failure is non-fatal in hybrid mode
			if opts.Mode == ModeHybrid {
				// Fall back to keyword only
				semanticResults = nil
			} else {
				return nil, fmt.Errorf("semantic search: %w", err)
			}
		} else {
			semanticResults = sem
		}
	}

	var merged []SearchResult
	switch opts.Mode {
	case ModeKeyword:
		merged = toSearchResults(keywordResults, opts.Limit)
	case ModeSemantic:
		merged = toSearchResults(semanticResults, opts.Limit)
	case ModeHybrid:
		fused := reciprocalRankFusion(keywordResults, semanticResults)
		merged = toSearchResults(fused, opts.Limit)
	}

	return merged, nil
}

// scoredResult is an intermediate result used during search and ranking.
type scoredResult struct {
	SearchResult
	rawScore float64
}

// keywordSearch performs a LIKE-based keyword search on messages.
func (s *Service) keywordSearch(opts SearchOptions) ([]scoredResult, error) {
	pattern := "%" + opts.Query + "%"

	var query string
	var args []interface{}

	if opts.ConversationFilter != "" {
		query = `
			SELECT m.id, m.conversation_id, m.role, m.content, m.created_at
			FROM messages m
			JOIN conversations c ON c.id = m.conversation_id
			WHERE m.content LIKE ? AND m.conversation_id = ? AND c.kind = ?
			ORDER BY m.created_at DESC
			LIMIT ?`
		args = []interface{}{pattern, opts.ConversationFilter, opts.ConversationKind, opts.Limit * 2}
	} else if opts.UserID != "" {
		query = `
			SELECT m.id, m.conversation_id, m.role, m.content, m.created_at
			FROM messages m
			JOIN conversations c ON c.id = m.conversation_id
			WHERE m.content LIKE ? AND c.user_id = ? AND c.kind = ?
			ORDER BY m.created_at DESC
			LIMIT ?`
		args = []interface{}{pattern, opts.UserID, opts.ConversationKind, opts.Limit * 2}
	} else {
		query = `
			SELECT m.id, m.conversation_id, m.role, m.content, m.created_at
			FROM messages m
			JOIN conversations c ON c.id = m.conversation_id
			WHERE m.content LIKE ? AND c.kind = ?
			ORDER BY m.created_at DESC
			LIMIT ?`
		args = []interface{}{pattern, opts.ConversationKind, opts.Limit * 2}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("keyword query: %w", err)
	}
	defer rows.Close()

	var results []scoredResult
	for rows.Next() {
		var msgID, convID, role, content, createdAt string
		if err := rows.Scan(&msgID, &convID, &role, &content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan keyword result: %w", err)
		}
		// Simple relevance score: count of case-insensitive query occurrences / content length
		lower := strings.ToLower(content)
		queryLower := strings.ToLower(opts.Query)
		count := strings.Count(lower, queryLower)
		score := float64(count) / math.Max(float64(len(content)), 1.0)

		results = append(results, scoredResult{
			SearchResult: SearchResult{
				Type:           "message",
				ConversationID: convID,
				MessageID:      msgID,
				Content:        truncate(content, 500),
				Score:          score,
				Role:           role,
				Timestamp:      createdAt,
			},
			rawScore: score,
		})
	}
	return results, rows.Err()
}

// semanticSearch performs vector similarity search on message embeddings.
func (s *Service) semanticSearch(ctx context.Context, opts SearchOptions) ([]scoredResult, error) {
	// Get settings for embedding provider/model
	settings, err := s.getSettings()
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	// Embed the query
	embResp, err := s.llmService.Embed(ctx, llm.EmbeddingRequest{
		Provider: settings.defaultProvider,
		Model:    settings.embeddingModel,
		Input:    []string{opts.Query},
	})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	queryVec := embResp.Embeddings[0]

	// Get all message embeddings (or filtered by conversation/user)
	var rows []repository.MessageEmbeddingRow
	if opts.ConversationFilter != "" {
		rows, err = s.msgEmbeddingRepo.GetByConversation(opts.ConversationFilter)
	} else if opts.UserID != "" {
		rows, err = s.msgEmbeddingRepo.GetByUserAndConversationKind(opts.UserID, opts.ConversationKind)
	} else {
		rows, err = s.msgEmbeddingRepo.GetAllByConversationKind(opts.ConversationKind)
	}
	if err != nil {
		return nil, fmt.Errorf("get embeddings: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	// Compute cosine similarity for each embedding
	type scored struct {
		messageID string
		score     float64
	}
	scoredIDs := make([]scored, 0, len(rows))
	for _, row := range rows {
		sim := cosineSimilarity(queryVec, row.Embedding)
		scoredIDs = append(scoredIDs, scored{messageID: row.MessageID, score: sim})
	}
	sort.Slice(scoredIDs, func(i, j int) bool { return scoredIDs[i].score > scoredIDs[j].score })

	// Take top candidates
	topN := opts.Limit * 2
	if topN > len(scoredIDs) {
		topN = len(scoredIDs)
	}
	scoredIDs = scoredIDs[:topN]

	// Fetch message details for top candidates
	msgIDs := make([]string, len(scoredIDs))
	scoreMap := make(map[string]float64, len(scoredIDs))
	for i, s := range scoredIDs {
		msgIDs[i] = s.messageID
		scoreMap[s.messageID] = s.score
	}
	messages, err := s.getMessagesByIDs(msgIDs)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	var results []scoredResult
	for _, msg := range messages {
		sim := scoreMap[msg.ID]
		results = append(results, scoredResult{
			SearchResult: SearchResult{
				Type:           "message",
				ConversationID: msg.ConversationID,
				MessageID:      msg.ID,
				Content:        truncate(msg.Content, 500),
				Score:          sim,
				Role:           msg.Role,
				Timestamp:      msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
			},
			rawScore: sim,
		})
	}
	return results, nil
}

// reciprocalRankFusion merges two ranked lists using RRF (k=60).
func reciprocalRankFusion(listA, listB []scoredResult) []scoredResult {
	const rrfK = 60.0

	// Sort each list by rawScore descending
	sort.Slice(listA, func(i, j int) bool { return listA[i].rawScore > listA[j].rawScore })
	sort.Slice(listB, func(i, j int) bool { return listB[i].rawScore > listB[j].rawScore })

	// Unique key for dedup: type + messageID (or chunkID)
	resultKey := func(r SearchResult) string {
		if r.ChunkID != "" {
			return r.Type + ":" + r.ChunkID
		}
		return r.Type + ":" + r.MessageID
	}

	scores := make(map[string]float64)
	items := make(map[string]SearchResult)

	for rank, r := range listA {
		rk := resultKey(r.SearchResult)
		scores[rk] += 1.0 / (rrfK + float64(rank+1))
		if _, ok := items[rk]; !ok {
			items[rk] = r.SearchResult
		}
	}
	for rank, r := range listB {
		rk := resultKey(r.SearchResult)
		scores[rk] += 1.0 / (rrfK + float64(rank+1))
		if _, ok := items[rk]; !ok {
			items[rk] = r.SearchResult
		}
	}

	// Build merged results
	type fusedItem struct {
		result SearchResult
		score  float64
	}
	var fused []fusedItem
	for rk, result := range items {
		fused = append(fused, fusedItem{result: result, score: scores[rk]})
	}
	sort.Slice(fused, func(i, j int) bool { return fused[i].score > fused[j].score })

	results := make([]scoredResult, len(fused))
	for i, f := range fused {
		r := f.result
		r.Score = f.score
		results[i] = scoredResult{SearchResult: r, rawScore: f.score}
	}
	return results
}

type embeddingSettings struct {
	defaultProvider string
	embeddingModel  string
}

func (s *Service) getSettings() (*embeddingSettings, error) {
	all, err := s.settingsRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	appSettings := models.AppSettingsFromMap(all)

	provider := ""
	// Look for a default provider from the settings map
	if v, ok := all["default_provider"]; ok && v != "" {
		provider = v
	}

	model := appSettings.RAGEmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}

	return &embeddingSettings{
		defaultProvider: provider,
		embeddingModel:  model,
	}, nil
}

// getMessagesByIDs fetches messages by their IDs.
func (s *Service) getMessagesByIDs(ids []string) ([]models.Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, conversation_id, role, content, created_at
		FROM messages
		WHERE id IN (%s)`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("get messages by ids: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// Reindex embeds all messages that don't have embeddings yet.
func (s *Service) Reindex(ctx context.Context) (*ReindexStatus, error) {
	settings, err := s.getSettings()
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	// Get all messages
	allMsgRows, err := s.db.Query("SELECT id, content FROM messages WHERE content != '' ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer allMsgRows.Close()

	type msgPair struct {
		id      string
		content string
	}
	var allMsgs []msgPair
	for allMsgRows.Next() {
		var p msgPair
		if err := allMsgRows.Scan(&p.id, &p.content); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		allMsgs = append(allMsgs, p)
	}
	if err := allMsgRows.Err(); err != nil {
		return nil, err
	}

	total := len(allMsgs)

	// Get already embedded message IDs
	embeddedCount, err := s.msgEmbeddingRepo.CountEmbedded()
	if err != nil {
		return nil, fmt.Errorf("count embedded: %w", err)
	}

	// Get existing embedding IDs to skip
	existingIDs := make(map[string]bool)
	existingRows, err := s.db.Query("SELECT message_id FROM message_embeddings")
	if err != nil {
		return nil, fmt.Errorf("query existing embeddings: %w", err)
	}
	defer existingRows.Close()
	for existingRows.Next() {
		var id string
		if err := existingRows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan existing id: %w", err)
		}
		existingIDs[id] = true
	}

	// Filter to un-embedded messages
	var toEmbed []msgPair
	for _, m := range allMsgs {
		if !existingIDs[m.id] {
			toEmbed = append(toEmbed, m)
		}
	}

	if len(toEmbed) == 0 {
		return &ReindexStatus{
			Total:    total,
			Embedded: embeddedCount,
			Status:   "completed",
		}, nil
	}

	// Process in batches
	batchSize := 50
	for i := 0; i < len(toEmbed); i += batchSize {
		end := i + batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		texts := make([]string, len(batch))
		for j, m := range batch {
			texts[j] = m.content
		}

		embResp, err := s.llmService.Embed(ctx, llm.EmbeddingRequest{
			Provider: settings.defaultProvider,
			Model:    settings.embeddingModel,
			Input:    texts,
		})
		if err != nil {
			return &ReindexStatus{
				Total:    total,
				Embedded: embeddedCount + i,
				Status:   "error",
			}, fmt.Errorf("embed batch %d: %w", i/batchSize, err)
		}

		embeddings := make([]models.MessageEmbedding, len(batch))
		for j, m := range batch {
			embeddings[j] = models.MessageEmbedding{
				MessageID:  m.id,
				Embedding:  embResp.Embeddings[j],
				Model:      embResp.Model,
				Dimensions: embResp.Dimensions,
			}
		}

		if err := s.msgEmbeddingRepo.UpsertBatch(embeddings); err != nil {
			return &ReindexStatus{
				Total:    total,
				Embedded: embeddedCount + i,
				Status:   "error",
			}, fmt.Errorf("upsert batch %d: %w", i/batchSize, err)
		}
	}

	finalCount, _ := s.msgEmbeddingRepo.CountEmbedded()
	return &ReindexStatus{
		Total:    total,
		Embedded: finalCount,
		Status:   "completed",
	}, nil
}

// cosineSimilarity computes cosine similarity between two float32 vectors.
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

// truncate limits a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// toSearchResults converts scored results to final SearchResult slice with limit.
func toSearchResults(scored []scoredResult, limit int) []SearchResult {
	if len(scored) == 0 {
		return []SearchResult{}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].rawScore > scored[j].rawScore })
	if len(scored) > limit {
		scored = scored[:limit]
	}
	results := make([]SearchResult, len(scored))
	for i, s := range scored {
		results[i] = s.SearchResult
	}
	return results
}
