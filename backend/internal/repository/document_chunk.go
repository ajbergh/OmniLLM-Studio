package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ChunkRepo handles document chunk persistence.
type ChunkRepo struct {
	db *sql.DB
}

// NewChunkRepo creates a new ChunkRepo.
func NewChunkRepo(db *sql.DB) *ChunkRepo {
	return &ChunkRepo{db: db}
}

// Create inserts a single document chunk.
func (r *ChunkRepo) Create(c *models.DocumentChunk) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(`
		INSERT INTO document_chunks (id, attachment_id, conversation_id, chunk_index, content, char_offset, char_length, token_count, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AttachmentID, c.ConversationID, c.ChunkIndex, c.Content,
		c.CharOffset, c.CharLength, c.TokenCount, c.MetadataJSON, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create chunk: %w", err)
	}
	return nil
}

// CreateBatch inserts multiple document chunks in a single transaction.
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
		INSERT INTO document_chunks (id, attachment_id, conversation_id, chunk_index, content, char_offset, char_length, token_count, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for i := range chunks {
		c := &chunks[i]
		if c.ID == "" {
			c.ID = uuid.New().String()
		}
		c.CreatedAt = now
		if c.MetadataJSON == "" {
			c.MetadataJSON = "{}"
		}
		if _, err := stmt.Exec(
			c.ID, c.AttachmentID, c.ConversationID, c.ChunkIndex, c.Content,
			c.CharOffset, c.CharLength, c.TokenCount, c.MetadataJSON, c.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	return tx.Commit()
}

// ListByAttachment returns all chunks for a given attachment.
func (r *ChunkRepo) ListByAttachment(attachmentID string) ([]models.DocumentChunk, error) {
	rows, err := r.db.Query(`
		SELECT id, attachment_id, conversation_id, chunk_index, content, char_offset, char_length, token_count, metadata_json, created_at
		FROM document_chunks WHERE attachment_id = ? ORDER BY chunk_index ASC`, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("list chunks by attachment: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

// ListByConversation returns all chunks for a given conversation.
func (r *ChunkRepo) ListByConversation(conversationID string) ([]models.DocumentChunk, error) {
	rows, err := r.db.Query(`
		SELECT id, attachment_id, conversation_id, chunk_index, content, char_offset, char_length, token_count, metadata_json, created_at
		FROM document_chunks WHERE conversation_id = ? ORDER BY attachment_id, chunk_index ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list chunks by conversation: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

// maxSQLiteParams is the maximum number of parameters in a single query.
// SQLite default SQLITE_MAX_VARIABLE_NUMBER is 999; we use 900 for safety.
const maxSQLiteParams = 900

// GetByIDs returns chunks matching the given IDs, batching to stay within
// SQLite's parameter limit.
func (r *ChunkRepo) GetByIDs(ids []string) ([]models.DocumentChunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var allResults []models.DocumentChunk
	for i := 0; i < len(ids); i += maxSQLiteParams {
		end := i + maxSQLiteParams
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		placeholders := strings.Repeat("?,", len(batch))
		placeholders = placeholders[:len(placeholders)-1]

		args := make([]interface{}, len(batch))
		for j, id := range batch {
			args[j] = id
		}

		rows, err := r.db.Query(fmt.Sprintf(`
		SELECT id, attachment_id, conversation_id, chunk_index, content, char_offset, char_length, token_count, metadata_json, created_at
		FROM document_chunks WHERE id IN (%s) ORDER BY chunk_index ASC`, placeholders), args...)
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

// DeleteByAttachment removes all chunks for an attachment.
func (r *ChunkRepo) DeleteByAttachment(attachmentID string) error {
	_, err := r.db.Exec("DELETE FROM document_chunks WHERE attachment_id = ?", attachmentID)
	if err != nil {
		return fmt.Errorf("delete chunks by attachment: %w", err)
	}
	return nil
}

// DeleteByConversation removes all chunks for a conversation.
func (r *ChunkRepo) DeleteByConversation(conversationID string) error {
	_, err := r.db.Exec("DELETE FROM document_chunks WHERE conversation_id = ?", conversationID)
	if err != nil {
		return fmt.Errorf("delete chunks by conversation: %w", err)
	}
	return nil
}

// CountByAttachment returns the number of chunks for an attachment.
func (r *ChunkRepo) CountByAttachment(attachmentID string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM document_chunks WHERE attachment_id = ?", attachmentID).Scan(&count)
	return count, err
}

func scanChunks(rows *sql.Rows) ([]models.DocumentChunk, error) {
	var chunks []models.DocumentChunk
	for rows.Next() {
		var c models.DocumentChunk
		if err := rows.Scan(
			&c.ID, &c.AttachmentID, &c.ConversationID, &c.ChunkIndex, &c.Content,
			&c.CharOffset, &c.CharLength, &c.TokenCount, &c.MetadataJSON, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
