package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// MessageEmbeddingRepo manages message embeddings in the database.
type MessageEmbeddingRepo struct {
	db *sql.DB
}

// NewMessageEmbeddingRepo creates a new MessageEmbeddingRepo.
func NewMessageEmbeddingRepo(db *sql.DB) *MessageEmbeddingRepo {
	return &MessageEmbeddingRepo{db: db}
}

// Upsert inserts or replaces a single message embedding.
func (r *MessageEmbeddingRepo) Upsert(messageID string, embedding []float32, model string, dims int) error {
	_, err := r.db.Exec(`
		INSERT OR REPLACE INTO message_embeddings (message_id, embedding, model, dimensions)
		VALUES (?, ?, ?, ?)`,
		messageID, float32sToBytes(embedding), model, dims)
	if err != nil {
		return fmt.Errorf("upsert message embedding: %w", err)
	}
	return nil
}

// UpsertBatch inserts or replaces multiple message embeddings in a single transaction.
func (r *MessageEmbeddingRepo) UpsertBatch(embeddings []models.MessageEmbedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO message_embeddings (message_id, embedding, model, dimensions)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, e := range embeddings {
		if _, err := stmt.Exec(e.MessageID, float32sToBytes(e.Embedding), e.Model, e.Dimensions); err != nil {
			return fmt.Errorf("exec upsert message embedding %s: %w", e.MessageID, err)
		}
	}
	return tx.Commit()
}

// GetByMessageIDs returns a map of messageID → embedding for the given IDs,
// batching to stay within SQLite's parameter limit.
func (r *MessageEmbeddingRepo) GetByMessageIDs(ids []string) (map[string][]float32, error) {
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
			"SELECT message_id, embedding FROM message_embeddings WHERE message_id IN (%s)", placeholders), args...)
		if err != nil {
			return nil, fmt.Errorf("get message embeddings by ids: %w", err)
		}
		for rows.Next() {
			var msgID string
			var blob []byte
			if err := rows.Scan(&msgID, &blob); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan message embedding: %w", err)
			}
			result[msgID] = bytesToFloat32s(blob)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

// MessageEmbeddingRow holds a full message embedding record for search operations.
type MessageEmbeddingRow struct {
	MessageID  string
	Embedding  []float32
	Model      string
	Dimensions int
}

// GetAll returns all message embeddings (used for similarity search across all messages).
func (r *MessageEmbeddingRepo) GetAll() ([]MessageEmbeddingRow, error) {
	rows, err := r.db.Query("SELECT message_id, embedding, model, dimensions FROM message_embeddings")
	if err != nil {
		return nil, fmt.Errorf("get all message embeddings: %w", err)
	}
	defer rows.Close()

	var result []MessageEmbeddingRow
	for rows.Next() {
		var row MessageEmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.MessageID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan message embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetAllByConversationKind returns all message embeddings for a specific conversation kind.
func (r *MessageEmbeddingRepo) GetAllByConversationKind(kind string) ([]MessageEmbeddingRow, error) {
	rows, err := r.db.Query(`
		SELECT me.message_id, me.embedding, me.model, me.dimensions
		FROM message_embeddings me
		JOIN messages m ON m.id = me.message_id
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.kind = ?`, kind)
	if err != nil {
		return nil, fmt.Errorf("get message embeddings by conversation kind: %w", err)
	}
	defer rows.Close()

	var result []MessageEmbeddingRow
	for rows.Next() {
		var row MessageEmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.MessageID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan message embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetByConversation returns all message embeddings for a given conversation.
func (r *MessageEmbeddingRepo) GetByConversation(conversationID string) ([]MessageEmbeddingRow, error) {
	rows, err := r.db.Query(`
		SELECT me.message_id, me.embedding, me.model, me.dimensions
		FROM message_embeddings me
		JOIN messages m ON m.id = me.message_id
		WHERE m.conversation_id = ?`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get message embeddings for conversation: %w", err)
	}
	defer rows.Close()

	var result []MessageEmbeddingRow
	for rows.Next() {
		var row MessageEmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.MessageID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan message embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetByUser returns all message embeddings for conversations owned by a given user.
func (r *MessageEmbeddingRepo) GetByUser(userID string) ([]MessageEmbeddingRow, error) {
	rows, err := r.db.Query(`
		SELECT me.message_id, me.embedding, me.model, me.dimensions
		FROM message_embeddings me
		JOIN messages m ON m.id = me.message_id
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("get message embeddings for user: %w", err)
	}
	defer rows.Close()

	var result []MessageEmbeddingRow
	for rows.Next() {
		var row MessageEmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.MessageID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan message embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetByUserAndConversationKind returns all message embeddings for a user's conversations of a specific kind.
func (r *MessageEmbeddingRepo) GetByUserAndConversationKind(userID, kind string) ([]MessageEmbeddingRow, error) {
	rows, err := r.db.Query(`
		SELECT me.message_id, me.embedding, me.model, me.dimensions
		FROM message_embeddings me
		JOIN messages m ON m.id = me.message_id
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.user_id = ? AND c.kind = ?`, userID, kind)
	if err != nil {
		return nil, fmt.Errorf("get message embeddings for user and conversation kind: %w", err)
	}
	defer rows.Close()

	var result []MessageEmbeddingRow
	for rows.Next() {
		var row MessageEmbeddingRow
		var blob []byte
		if err := rows.Scan(&row.MessageID, &blob, &row.Model, &row.Dimensions); err != nil {
			return nil, fmt.Errorf("scan message embedding row: %w", err)
		}
		row.Embedding = bytesToFloat32s(blob)
		result = append(result, row)
	}
	return result, rows.Err()
}

// Delete removes a message embedding.
func (r *MessageEmbeddingRepo) Delete(messageID string) error {
	_, err := r.db.Exec("DELETE FROM message_embeddings WHERE message_id = ?", messageID)
	if err != nil {
		return fmt.Errorf("delete message embedding: %w", err)
	}
	return nil
}

// DeleteByConversation removes all message embeddings for a conversation.
func (r *MessageEmbeddingRepo) DeleteByConversation(conversationID string) error {
	_, err := r.db.Exec(`
		DELETE FROM message_embeddings WHERE message_id IN (
			SELECT id FROM messages WHERE conversation_id = ?
		)`, conversationID)
	if err != nil {
		return fmt.Errorf("delete message embeddings by conversation: %w", err)
	}
	return nil
}

// CountEmbedded returns the number of messages that have embeddings.
func (r *MessageEmbeddingRepo) CountEmbedded() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM message_embeddings").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count embedded messages: %w", err)
	}
	return count, nil
}

// CountTotal returns the total number of messages (for reindex progress).
func (r *MessageEmbeddingRepo) CountTotal() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count total messages: %w", err)
	}
	return count, nil
}
