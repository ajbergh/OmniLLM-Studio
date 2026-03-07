package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// MessageRepo handles message persistence.
type MessageRepo struct {
	db *sql.DB
}

// NewMessageRepo creates a new MessageRepo.
func NewMessageRepo(db *sql.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// ListByConversation returns all messages for a conversation in chronological order.
func (r *MessageRepo) ListByConversation(conversationID string) ([]models.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, created_at,
		       provider, model, token_input, token_output, latency_ms, metadata_json,
		       branch_id, parent_message_id, user_id
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt,
			&m.Provider, &m.Model, &m.TokenInput, &m.TokenOutput, &m.LatencyMs, &m.MetadataJSON,
			&m.BranchID, &m.ParentMessageID, &m.UserID,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ListByBranch returns messages for a specific branch of a conversation.
// It loads all 'main' branch messages up to the fork point, then all messages
// in the specified branch after the fork.
func (r *MessageRepo) ListByBranch(conversationID, branchID string) ([]models.Message, error) {
	if branchID == "" || branchID == "main" {
		// For main branch, only return main messages
		query := `
			SELECT id, conversation_id, role, content, created_at,
			       provider, model, token_input, token_output, latency_ms, metadata_json,
			       branch_id, parent_message_id, user_id
			FROM messages
			WHERE conversation_id = ? AND branch_id = 'main'
			ORDER BY created_at ASC
		`
		rows, err := r.db.Query(query, conversationID)
		if err != nil {
			return nil, fmt.Errorf("list branch messages: %w", err)
		}
		defer rows.Close()
		return scanMessages(rows)
	}

	// For non-main branches: get main messages up to fork point + branch messages after
	query := `
		SELECT id, conversation_id, role, content, created_at,
		       provider, model, token_input, token_output, latency_ms, metadata_json,
		       branch_id, parent_message_id, user_id
		FROM messages
		WHERE conversation_id = ?
		  AND (
		    (branch_id = 'main' AND created_at <= (
		      SELECT m2.created_at FROM messages m2
		      JOIN conversation_branches cb ON cb.fork_message_id = m2.id
		      WHERE cb.id = ?
		    ))
		    OR branch_id = ?
		  )
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, conversationID, branchID, branchID)
	if err != nil {
		return nil, fmt.Errorf("list branch messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// scanMessages scans message rows into a slice.
func scanMessages(rows *sql.Rows) ([]models.Message, error) {
	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt,
			&m.Provider, &m.Model, &m.TokenInput, &m.TokenOutput, &m.LatencyMs, &m.MetadataJSON,
			&m.BranchID, &m.ParentMessageID, &m.UserID,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// Create inserts a new message.
func (r *MessageRepo) Create(msg *models.Message) (*models.Message, error) {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	if msg.MetadataJSON == "" {
		msg.MetadataJSON = "{}"
	}

	if msg.BranchID == "" {
		msg.BranchID = "main"
	}

	_, err := r.db.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, created_at,
		                      provider, model, token_input, token_output, latency_ms, metadata_json,
		                      branch_id, parent_message_id, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.ConversationID, msg.Role, msg.Content, msg.CreatedAt,
		msg.Provider, msg.Model, msg.TokenInput, msg.TokenOutput, msg.LatencyMs, msg.MetadataJSON,
		msg.BranchID, msg.ParentMessageID, msg.UserID)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	return msg, nil
}

// GetByID retrieves a single message by ID.
func (r *MessageRepo) GetByID(id string) (*models.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, created_at,
		       provider, model, token_input, token_output, latency_ms, metadata_json,
		       branch_id, parent_message_id, user_id
		FROM messages WHERE id = ?
	`
	var m models.Message
	err := r.db.QueryRow(query, id).Scan(
		&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt,
		&m.Provider, &m.Model, &m.TokenInput, &m.TokenOutput, &m.LatencyMs, &m.MetadataJSON,
		&m.BranchID, &m.ParentMessageID, &m.UserID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return &m, nil
}

// Delete removes a message, scoped to its conversation for safety.
func (r *MessageRepo) Delete(conversationID, id string) error {
	result, err := r.db.Exec("DELETE FROM messages WHERE id = ? AND conversation_id = ?", id, conversationID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("message not found or does not belong to conversation")
	}
	return nil
}

// UpdateContent changes a message's content.
func (r *MessageRepo) UpdateContent(id, content string) error {
	_, err := r.db.Exec("UPDATE messages SET content = ? WHERE id = ?", content, id)
	return err
}

// DeleteFromMessageOnward removes the given message and everything after it
// in the same conversation (by rowid order). Useful for regeneration.
// Returns an error if the target message is not found in the given conversation.
func (r *MessageRepo) DeleteFromMessageOnward(conversationID, messageID string) error {
	result, err := r.db.Exec(`
		DELETE FROM messages
		WHERE conversation_id = ?
		  AND rowid >= (SELECT rowid FROM messages WHERE id = ? AND conversation_id = ?)
	`, conversationID, messageID, conversationID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("message not found or does not belong to conversation")
	}
	return nil
}
