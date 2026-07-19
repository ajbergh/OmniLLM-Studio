package repository

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ChunkRepo handles document chunk persistence and FTS5 lexical retrieval.
type ChunkRepo struct {
	db *sql.DB
}

func NewChunkRepo(db *sql.DB) *ChunkRepo {
	if err := ensureRAGV2Schema(db); err != nil {
		log.Printf("[rag] initialize RAG v2 schema: %v", err)
	}
	return &ChunkRepo{db: db}
}

func (r *ChunkRepo) Create(c *models.DocumentChunk) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.CreatedAt = time.Now().UTC()
	if c.MetadataJSON == "" {
		c.MetadataJSON = "{}"
	}
	_, err := r.db.Exec(`
        INSERT INTO document_chunks (
            id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
            chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
            chunk_metadata_json, metadata_json, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AttachmentID, c.ConversationID, c.LibraryFileID, chunkScopeOrDefault(c.Scope), c.WorkspaceID, c.SourceType,
		c.ChunkIndex, c.Content, c.CharOffset, c.CharLength, c.TokenCount, c.PageNumber, c.SectionTitle,
		chunkMetaOrDefault(c.ChunkMetaJSON), c.MetadataJSON, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create chunk: %w", err)
	}
	return nil
}

func (r *ChunkRepo) CreateBatch(chunks []models.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
        INSERT INTO document_chunks (
            id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
            chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
            chunk_metadata_json, metadata_json, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()
	now := time.Now().UTC()
	for index := range chunks {
		chunk := &chunks[index]
		if chunk.ID == "" {
			chunk.ID = uuid.New().String()
		}
		chunk.CreatedAt = now
		if chunk.MetadataJSON == "" {
			chunk.MetadataJSON = "{}"
		}
		if chunk.ChunkMetaJSON == "" {
			chunk.ChunkMetaJSON = "{}"
		}
		if _, err := stmt.Exec(
			chunk.ID, chunk.AttachmentID, chunk.ConversationID, chunk.LibraryFileID, chunkScopeOrDefault(chunk.Scope), chunk.WorkspaceID, chunk.SourceType,
			chunk.ChunkIndex, chunk.Content, chunk.CharOffset, chunk.CharLength, chunk.TokenCount, chunk.PageNumber, chunk.SectionTitle,
			chunk.ChunkMetaJSON, chunk.MetadataJSON, chunk.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert chunk %d: %w", index, err)
		}
	}
	return tx.Commit()
}

func (r *ChunkRepo) ListByAttachment(attachmentID string) ([]models.DocumentChunk, error) {
	rows, err := r.db.Query(chunkSelect+` WHERE attachment_id = ? ORDER BY chunk_index ASC`, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("list chunks by attachment: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *ChunkRepo) ListByConversation(conversationID string) ([]models.DocumentChunk, error) {
	rows, err := r.db.Query(chunkSelect+` WHERE conversation_id = ? ORDER BY attachment_id, chunk_index ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list chunks by conversation: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *ChunkRepo) ListByLibraryFileID(libraryFileID string) ([]models.DocumentChunk, error) {
	rows, err := r.db.Query(chunkSelect+` WHERE library_file_id = ? ORDER BY chunk_index ASC`, libraryFileID)
	if err != nil {
		return nil, fmt.Errorf("list chunks by library file id: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

// KeywordChunkHit is one BM25 or lexical-fallback candidate. Lower BM25 values
// are better in SQLite FTS5, so Score is normalized to a positive higher-is-
// better value before returning.
type KeywordChunkHit struct {
	Chunk models.DocumentChunk
	Score float64
	Rank  int
}

// SearchFTSByLibraryFileIDs performs scoped FTS5/BM25 retrieval. It batches file
// filters to stay below SQLite parameter limits and falls back to tokenized LIKE
// retrieval when FTS5 is unavailable.
func (r *ChunkRepo) SearchFTSByLibraryFileIDs(libraryFileIDs []string, queryText string, limit int) ([]KeywordChunkHit, error) {
	if len(libraryFileIDs) == 0 || strings.TrimSpace(queryText) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 24
	}
	ftsQuery := buildFTSQuery(queryText)
	if ftsQuery == "" {
		return nil, nil
	}

	var all []KeywordChunkHit
	batchSize := maxSQLiteParams - 4
	for start := 0; start < len(libraryFileIDs); start += batchSize {
		end := start + batchSize
		if end > len(libraryFileIDs) {
			end = len(libraryFileIDs)
		}
		batch := libraryFileIDs[start:end]
		hits, err := r.searchFTSBatch(batch, ftsQuery, limit*3)
		if err != nil {
			if isFTSUnavailable(err) {
				return r.searchLexicalFallback(libraryFileIDs, queryText, limit)
			}
			return nil, err
		}
		all = append(all, hits...)
	}

	best := make(map[string]KeywordChunkHit, len(all))
	for _, hit := range all {
		if existing, ok := best[hit.Chunk.ID]; !ok || hit.Score > existing.Score {
			best[hit.Chunk.ID] = hit
		}
	}
	all = all[:0]
	for _, hit := range best {
		all = append(all, hit)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Score == all[j].Score {
			return all[i].Chunk.ID < all[j].Chunk.ID
		}
		return all[i].Score > all[j].Score
	})
	if len(all) > limit {
		all = all[:limit]
	}
	for index := range all {
		all[index].Rank = index
	}
	return all, nil
}

// SearchFTSByConversation performs BM25 retrieval over all chunks linked to one
// conversation. It powers the compatibility Retriever so legacy attachment RAG
// benefits from the same hybrid lexical/vector pipeline as the File Library.
func (r *ChunkRepo) SearchFTSByConversation(conversationID, queryText string, limit int) ([]KeywordChunkHit, error) {
	if strings.TrimSpace(conversationID) == "" || strings.TrimSpace(queryText) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 24
	}
	ftsQuery := buildFTSQuery(queryText)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := r.db.Query(`
        SELECT c.id, c.attachment_id, c.conversation_id, c.library_file_id, c.scope, c.workspace_id, c.source_type,
               c.chunk_index, c.content, c.char_offset, c.char_length, c.token_count, c.page_number, c.section_title,
               c.chunk_metadata_json, c.metadata_json, c.created_at,
               bm25(document_chunks_fts, 1.0, 2.0) AS rank_score
        FROM document_chunks_fts
        JOIN document_chunks c ON c.rowid = document_chunks_fts.rowid
        WHERE document_chunks_fts MATCH ? AND c.conversation_id = ?
        ORDER BY rank_score ASC
        LIMIT ?`, ftsQuery, conversationID, limit)
	if err != nil {
		if isFTSUnavailable(err) {
			return r.searchConversationLexicalFallback(conversationID, queryText, limit)
		}
		return nil, fmt.Errorf("search conversation chunks fts: %w", err)
	}
	defer rows.Close()
	var hits []KeywordChunkHit
	for rows.Next() {
		chunk, rankScore, err := scanChunkWithScore(rows)
		if err != nil {
			return nil, err
		}
		score := 1.0 / (1.0 + absFloat(rankScore))
		if rankScore < 0 {
			score = 1.0 + absFloat(rankScore)
		}
		hits = append(hits, KeywordChunkHit{Chunk: chunk, Score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range hits {
		hits[index].Rank = index
	}
	return hits, nil
}

func (r *ChunkRepo) searchConversationLexicalFallback(conversationID, queryText string, limit int) ([]KeywordChunkHit, error) {
	terms := lexicalTerms(queryText)
	if len(terms) == 0 {
		return nil, nil
	}
	conditions := make([]string, len(terms))
	args := make([]any, 0, len(terms)+2)
	args = append(args, conversationID)
	for index, term := range terms {
		conditions[index] = "LOWER(content) LIKE ?"
		args = append(args, "%"+strings.ToLower(term)+"%")
	}
	args = append(args, limit*3)
	query := fmt.Sprintf(`
        SELECT id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
               chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
               chunk_metadata_json, metadata_json, created_at
        FROM document_chunks
        WHERE conversation_id = ? AND (%s)
        LIMIT ?`, strings.Join(conditions, " OR "))
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search conversation chunks fallback: %w", err)
	}
	chunks, err := scanChunks(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	hits := make([]KeywordChunkHit, 0, len(chunks))
	for _, chunk := range chunks {
		hits = append(hits, KeywordChunkHit{Chunk: chunk, Score: lexicalMatchScore(chunk.Content, terms)})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Chunk.ID < hits[j].Chunk.ID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	for index := range hits {
		hits[index].Rank = index
	}
	return hits, nil
}

func (r *ChunkRepo) searchFTSBatch(fileIDs []string, ftsQuery string, limit int) ([]KeywordChunkHit, error) {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(fileIDs)), ",")
	args := make([]any, 0, len(fileIDs)+2)
	args = append(args, ftsQuery)
	for _, id := range fileIDs {
		args = append(args, id)
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
        SELECT c.id, c.attachment_id, c.conversation_id, c.library_file_id, c.scope, c.workspace_id, c.source_type,
               c.chunk_index, c.content, c.char_offset, c.char_length, c.token_count, c.page_number, c.section_title,
               c.chunk_metadata_json, c.metadata_json, c.created_at,
               bm25(document_chunks_fts, 1.0, 2.0) AS rank_score
        FROM document_chunks_fts
        JOIN document_chunks c ON c.rowid = document_chunks_fts.rowid
        WHERE document_chunks_fts MATCH ?
          AND c.library_file_id IN (%s)
        ORDER BY rank_score ASC
        LIMIT ?`, placeholders)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search chunks fts: %w", err)
	}
	defer rows.Close()
	var hits []KeywordChunkHit
	for rows.Next() {
		chunk, rankScore, err := scanChunkWithScore(rows)
		if err != nil {
			return nil, err
		}
		// FTS5 bm25 is usually negative for strong matches. Convert it into a
		// monotonic positive score without relying on cross-query calibration.
		score := 1.0 / (1.0 + absFloat(rankScore))
		if rankScore < 0 {
			score = 1.0 + absFloat(rankScore)
		}
		hits = append(hits, KeywordChunkHit{Chunk: chunk, Score: score})
	}
	return hits, rows.Err()
}

// SearchByLibraryFileIDs retains the historical API but now uses FTS5/BM25.
func (r *ChunkRepo) SearchByLibraryFileIDs(libraryFileIDs []string, queryText string, limit int) ([]models.DocumentChunk, error) {
	hits, err := r.SearchFTSByLibraryFileIDs(libraryFileIDs, queryText, limit)
	if err != nil {
		return nil, err
	}
	chunks := make([]models.DocumentChunk, len(hits))
	for index := range hits {
		chunks[index] = hits[index].Chunk
	}
	return chunks, nil
}

func (r *ChunkRepo) searchLexicalFallback(libraryFileIDs []string, queryText string, limit int) ([]KeywordChunkHit, error) {
	terms := lexicalTerms(queryText)
	if len(terms) == 0 {
		return nil, nil
	}
	batchSize := maxSQLiteParams - len(terms) - 2
	if batchSize < 1 {
		batchSize = 1
	}
	var hits []KeywordChunkHit
	for start := 0; start < len(libraryFileIDs); start += batchSize {
		end := start + batchSize
		if end > len(libraryFileIDs) {
			end = len(libraryFileIDs)
		}
		batch := libraryFileIDs[start:end]
		placeholders := strings.TrimRight(strings.Repeat("?,", len(batch)), ",")
		conditions := make([]string, len(terms))
		args := make([]any, 0, len(batch)+len(terms)+1)
		for _, id := range batch {
			args = append(args, id)
		}
		for index, term := range terms {
			conditions[index] = "LOWER(content) LIKE ?"
			args = append(args, "%"+strings.ToLower(term)+"%")
		}
		args = append(args, limit*3)
		query := fmt.Sprintf(`
            SELECT id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
                   chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
                   chunk_metadata_json, metadata_json, created_at
            FROM document_chunks
            WHERE library_file_id IN (%s) AND (%s)
            LIMIT ?`, placeholders, strings.Join(conditions, " OR "))
		rows, err := r.db.Query(query, args...)
		if err != nil {
			return nil, fmt.Errorf("search chunks fallback: %w", err)
		}
		chunks, err := scanChunks(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, chunk := range chunks {
			score := lexicalMatchScore(chunk.Content, terms)
			hits = append(hits, KeywordChunkHit{Chunk: chunk, Score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	for index := range hits {
		hits[index].Rank = index
	}
	return hits, nil
}

const maxSQLiteParams = 900

func (r *ChunkRepo) GetByIDs(ids []string) ([]models.DocumentChunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var allResults []models.DocumentChunk
	for start := 0; start < len(ids); start += maxSQLiteParams {
		end := start + maxSQLiteParams
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		placeholders := strings.TrimRight(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, len(batch))
		for index, id := range batch {
			args[index] = id
		}
		rows, err := r.db.Query(chunkSelect+fmt.Sprintf(` WHERE id IN (%s) ORDER BY chunk_index ASC`, placeholders), args...)
		if err != nil {
			return nil, fmt.Errorf("get chunks by ids: %w", err)
		}
		chunks, err := scanChunks(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, chunks...)
	}
	return allResults, nil
}

func (r *ChunkRepo) DeleteByAttachment(attachmentID string) error {
	_, err := r.db.Exec("DELETE FROM document_chunks WHERE attachment_id = ?", attachmentID)
	if err != nil {
		return fmt.Errorf("delete chunks by attachment: %w", err)
	}
	return nil
}

func (r *ChunkRepo) DeleteByConversation(conversationID string) error {
	_, err := r.db.Exec("DELETE FROM document_chunks WHERE conversation_id = ?", conversationID)
	if err != nil {
		return fmt.Errorf("delete chunks by conversation: %w", err)
	}
	return nil
}

func (r *ChunkRepo) DeleteByLibraryFileID(libraryFileID string) error {
	_, err := r.db.Exec("DELETE FROM document_chunks WHERE library_file_id = ?", libraryFileID)
	if err != nil {
		return fmt.Errorf("delete chunks by library file id: %w", err)
	}
	return nil
}

func (r *ChunkRepo) DistinctConversationIDsWithChunks() ([]string, error) {
	rows, err := r.db.Query(`SELECT DISTINCT conversation_id FROM document_chunks`)
	if err != nil {
		return nil, fmt.Errorf("distinct conversation ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ChunkRepo) CountByAttachment(attachmentID string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM document_chunks WHERE attachment_id = ?", attachmentID).Scan(&count)
	return count, err
}

const chunkSelect = `
SELECT id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
       chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
       chunk_metadata_json, metadata_json, created_at
FROM document_chunks`

func scanChunks(rows *sql.Rows) ([]models.DocumentChunk, error) {
	var chunks []models.DocumentChunk
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

type chunkRowScanner interface {
	Scan(dest ...any) error
}

func scanChunk(row chunkRowScanner) (models.DocumentChunk, error) {
	var chunk models.DocumentChunk
	var libraryFileID, scope, workspaceID, sourceType, sectionTitle, chunkMetaJSON sql.NullString
	var pageNumber sql.NullInt64
	if err := row.Scan(
		&chunk.ID, &chunk.AttachmentID, &chunk.ConversationID, &libraryFileID, &scope, &workspaceID, &sourceType,
		&chunk.ChunkIndex, &chunk.Content, &chunk.CharOffset, &chunk.CharLength, &chunk.TokenCount, &pageNumber, &sectionTitle,
		&chunkMetaJSON, &chunk.MetadataJSON, &chunk.CreatedAt,
	); err != nil {
		return models.DocumentChunk{}, fmt.Errorf("scan chunk: %w", err)
	}
	applyNullableChunkFields(&chunk, libraryFileID, scope, workspaceID, sourceType, pageNumber, sectionTitle, chunkMetaJSON)
	return chunk, nil
}

func scanChunkWithScore(row chunkRowScanner) (models.DocumentChunk, float64, error) {
	var chunk models.DocumentChunk
	var libraryFileID, scope, workspaceID, sourceType, sectionTitle, chunkMetaJSON sql.NullString
	var pageNumber sql.NullInt64
	var score float64
	if err := row.Scan(
		&chunk.ID, &chunk.AttachmentID, &chunk.ConversationID, &libraryFileID, &scope, &workspaceID, &sourceType,
		&chunk.ChunkIndex, &chunk.Content, &chunk.CharOffset, &chunk.CharLength, &chunk.TokenCount, &pageNumber, &sectionTitle,
		&chunkMetaJSON, &chunk.MetadataJSON, &chunk.CreatedAt, &score,
	); err != nil {
		return models.DocumentChunk{}, 0, fmt.Errorf("scan chunk with score: %w", err)
	}
	applyNullableChunkFields(&chunk, libraryFileID, scope, workspaceID, sourceType, pageNumber, sectionTitle, chunkMetaJSON)
	return chunk, score, nil
}

func applyNullableChunkFields(chunk *models.DocumentChunk, libraryFileID, scope, workspaceID, sourceType sql.NullString, pageNumber sql.NullInt64, sectionTitle, chunkMetaJSON sql.NullString) {
	if libraryFileID.Valid {
		chunk.LibraryFileID = &libraryFileID.String
	}
	if scope.Valid {
		chunk.Scope = &scope.String
	}
	if workspaceID.Valid {
		chunk.WorkspaceID = &workspaceID.String
	}
	if sourceType.Valid {
		chunk.SourceType = &sourceType.String
	}
	if pageNumber.Valid {
		value := int(pageNumber.Int64)
		chunk.PageNumber = &value
	}
	if sectionTitle.Valid {
		chunk.SectionTitle = &sectionTitle.String
	}
	if chunkMetaJSON.Valid {
		chunk.ChunkMetaJSON = chunkMetaJSON.String
	}
}

func buildFTSQuery(query string) string {
	terms := lexicalTerms(query)
	if len(terms) == 0 {
		return ""
	}
	parts := make([]string, len(terms))
	for index, term := range terms {
		term = strings.ReplaceAll(term, `"`, `""`)
		parts[index] = `"` + term + `"`
	}
	return strings.Join(parts, " OR ")
}

func lexicalTerms(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' || r == '.')
	})
	seen := make(map[string]struct{}, len(fields))
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, field)
		if len(terms) >= 16 {
			break
		}
	}
	return terms
}

func lexicalMatchScore(content string, terms []string) float64 {
	lower := strings.ToLower(content)
	matches := 0
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			matches++
		}
	}
	if len(terms) == 0 {
		return 0
	}
	return float64(matches) / float64(len(terms))
}

func isFTSUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table: document_chunks_fts") ||
		strings.Contains(message, "no such module: fts5") ||
		strings.Contains(message, "unable to use function bm25")
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func chunkMetaOrDefault(meta string) string {
	if strings.TrimSpace(meta) == "" {
		return "{}"
	}
	return meta
}

func chunkScopeOrDefault(scope *string) string {
	if scope == nil || strings.TrimSpace(*scope) == "" {
		return "conversation"
	}
	return *scope
}
