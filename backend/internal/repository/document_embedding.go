package repository

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// EmbeddingRow represents a row from the document_embeddings table with its chunk ID.
type EmbeddingRow struct {
	ChunkID    string
	Embedding  []float32
	Model      string
	Dimensions int
}

// EmbeddingRepo handles document embedding persistence.
type EmbeddingRepo struct {
	db *sql.DB
}

// NewEmbeddingRepo creates a new EmbeddingRepo.
func NewEmbeddingRepo(db *sql.DB) *EmbeddingRepo {
	return &EmbeddingRepo{db: db}
}

// Upsert inserts or replaces an embedding for a chunk.
func (r *EmbeddingRepo) Upsert(chunkID string, embedding []float32, model string, dims int) error {
	blob := float32sToBytes(embedding)
	_, err := r.db.Exec(`
		INSERT INTO document_embeddings (chunk_id, embedding, model, dimensions, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET embedding = excluded.embedding, model = excluded.model, dimensions = excluded.dimensions, created_at = excluded.created_at`,
		chunkID, blob, model, dims, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}
	return nil
}

// UpsertBatch inserts or replaces embeddings for multiple chunks in a transaction.
func (r *EmbeddingRepo) UpsertBatch(embeddings []models.DocumentEmbedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO document_embeddings (chunk_id, embedding, model, dimensions, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET embedding = excluded.embedding, model = excluded.model, dimensions = excluded.dimensions, created_at = excluded.created_at`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, e := range embeddings {
		blob := float32sToBytes(e.Embedding)
		if _, err := stmt.Exec(e.ChunkID, blob, e.Model, e.Dimensions, now); err != nil {
			return fmt.Errorf("upsert embedding for chunk %s: %w", e.ChunkID, err)
		}
	}

	return tx.Commit()
}

// GetByChunkIDs returns embeddings for the given chunk IDs, batching to stay
// within SQLite's parameter limit.
func (r *EmbeddingRepo) GetByChunkIDs(ids []string) (map[string][]float32, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	result := make(map[string][]float32, len(ids))
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

		rows, err := r.db.Query(fmt.Sprintf(
			"SELECT chunk_id, embedding FROM document_embeddings WHERE chunk_id IN (%s)", placeholders), args...)
		if err != nil {
			return nil, fmt.Errorf("get embeddings by chunk ids: %w", err)
		}
		for rows.Next() {
			var chunkID string
			var blob []byte
			if err := rows.Scan(&chunkID, &blob); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan embedding: %w", err)
			}
			result[chunkID] = bytesToFloat32s(blob)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

// GetAllForConversation returns all embeddings for chunks belonging to a conversation.
func (r *EmbeddingRepo) GetAllForConversation(conversationID string) ([]EmbeddingRow, error) {
	rows, err := r.db.Query(`
		SELECT de.chunk_id, de.embedding, de.model, de.dimensions
		FROM document_embeddings de
		JOIN document_chunks dc ON dc.id = de.chunk_id
		WHERE dc.conversation_id = ?`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get embeddings for conversation: %w", err)
	}
	defer rows.Close()

	var result []EmbeddingRow
	for rows.Next() {
		var row EmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.ChunkID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// DeleteByAttachment removes embeddings for all chunks of an attachment.
func (r *EmbeddingRepo) DeleteByAttachment(attachmentID string) error {
	_, err := r.db.Exec(`
		DELETE FROM document_embeddings WHERE chunk_id IN (
			SELECT id FROM document_chunks WHERE attachment_id = ?
		)`, attachmentID)
	if err != nil {
		return fmt.Errorf("delete embeddings by attachment: %w", err)
	}
	return nil
}

// DeleteByConversation removes embeddings for all chunks of a conversation.
func (r *EmbeddingRepo) DeleteByConversation(conversationID string) error {
	_, err := r.db.Exec(`
		DELETE FROM document_embeddings WHERE chunk_id IN (
			SELECT id FROM document_chunks WHERE conversation_id = ?
		)`, conversationID)
	if err != nil {
		return fmt.Errorf("delete embeddings by conversation: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Byte serialization helpers for float32 arrays
// ---------------------------------------------------------------------------

func float32sToBytes(fs []float32) []byte {
	buf := make([]byte, len(fs)*4)
	for i, f := range fs {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToFloat32s(b []byte) []float32 {
	n := len(b) / 4
	fs := make([]float32, n)
	for i := 0; i < n; i++ {
		fs[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return fs
}
