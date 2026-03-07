package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// AttachmentRepo handles attachment persistence.
type AttachmentRepo struct {
	db *sql.DB
}

// NewAttachmentRepo creates a new AttachmentRepo.
func NewAttachmentRepo(db *sql.DB) *AttachmentRepo {
	return &AttachmentRepo{db: db}
}

// Create inserts a new attachment record.
func (r *AttachmentRepo) Create(a *models.Attachment) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(`
		INSERT INTO attachments (id, conversation_id, message_id, type, mime_type, storage_path, bytes, width, height, created_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ConversationID, a.MessageID, a.Type, a.MimeType, a.StoragePath, a.Bytes,
		a.Width, a.Height, a.CreatedAt, a.MetadataJSON,
	)
	if err != nil {
		return fmt.Errorf("create attachment: %w", err)
	}
	return nil
}

// GetByID returns an attachment by ID.
func (r *AttachmentRepo) GetByID(id string) (*models.Attachment, error) {
	a := &models.Attachment{}
	err := r.db.QueryRow(`
		SELECT id, conversation_id, message_id, type, mime_type, storage_path, bytes, width, height, created_at, metadata_json
		FROM attachments WHERE id = ?`, id,
	).Scan(&a.ID, &a.ConversationID, &a.MessageID, &a.Type, &a.MimeType, &a.StoragePath, &a.Bytes,
		&a.Width, &a.Height, &a.CreatedAt, &a.MetadataJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}
	return a, nil
}

// ListByConversation returns all attachments for a conversation.
func (r *AttachmentRepo) ListByConversation(conversationID string) ([]models.Attachment, error) {
	rows, err := r.db.Query(`
		SELECT id, conversation_id, message_id, type, mime_type, storage_path, bytes, width, height, created_at, metadata_json
		FROM attachments WHERE conversation_id = ? ORDER BY created_at ASC`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var out []models.Attachment
	for rows.Next() {
		var a models.Attachment
		if err := rows.Scan(&a.ID, &a.ConversationID, &a.MessageID, &a.Type, &a.MimeType, &a.StoragePath, &a.Bytes,
			&a.Width, &a.Height, &a.CreatedAt, &a.MetadataJSON); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Delete removes an attachment record by ID (caller is responsible for file cleanup).
func (r *AttachmentRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM attachments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

// LinkToMessage sets the message_id on an attachment.
func (r *AttachmentRepo) LinkToMessage(attachmentID, messageID string) error {
	_, err := r.db.Exec("UPDATE attachments SET message_id = ? WHERE id = ?", messageID, attachmentID)
	if err != nil {
		return fmt.Errorf("link attachment to message: %w", err)
	}
	return nil
}
