package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type ImageSessionRepo struct {
	db *sql.DB
}

func NewImageSessionRepo(db *sql.DB) *ImageSessionRepo {
	return &ImageSessionRepo{db: db}
}

func (r *ImageSessionRepo) Create(conversationID, title string) (*models.ImageSession, error) {
	s := &models.ImageSession{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Title:          title,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	_, err := r.db.Exec(`
		INSERT INTO image_sessions (id, conversation_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		s.ID, s.ConversationID, s.Title, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create image session: %w", err)
	}
	return s, nil
}

func (r *ImageSessionRepo) GetByID(id string) (*models.ImageSession, error) {
	var s models.ImageSession
	var activeNodeID sql.NullString
	err := r.db.QueryRow(`
		SELECT id, conversation_id, title, active_node_id, created_at, updated_at
		FROM image_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.ConversationID, &s.Title, &activeNodeID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get image session: %w", err)
	}
	if activeNodeID.Valid {
		s.ActiveNodeID = &activeNodeID.String
	}
	return &s, nil
}

func (r *ImageSessionRepo) ListByConversation(conversationID string) ([]models.ImageSession, error) {
	rows, err := r.db.Query(`
		SELECT id, conversation_id, title, active_node_id, created_at, updated_at
		FROM image_sessions WHERE conversation_id = ? ORDER BY created_at DESC`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list image sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.ImageSession
	for rows.Next() {
		var s models.ImageSession
		var activeNodeID sql.NullString
		if err := rows.Scan(&s.ID, &s.ConversationID, &s.Title, &activeNodeID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan image session: %w", err)
		}
		if activeNodeID.Valid {
			s.ActiveNodeID = &activeNodeID.String
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *ImageSessionRepo) UpdateActiveNode(sessionID, nodeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		UPDATE image_sessions SET active_node_id = ?, updated_at = ? WHERE id = ?`,
		nodeID, now, sessionID,
	)
	if err != nil {
		return fmt.Errorf("update active node: %w", err)
	}
	return nil
}

func (r *ImageSessionRepo) UpdateTitle(sessionID, title string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		UPDATE image_sessions SET title = ?, updated_at = ? WHERE id = ?`,
		title, now, sessionID,
	)
	if err != nil {
		return fmt.Errorf("update session title: %w", err)
	}
	return nil
}

func (r *ImageSessionRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM image_sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete image session: %w", err)
	}
	return nil
}

// ListAllForUser returns all image sessions across all conversations owned by the given user.
// When userID is empty (solo mode), returns all sessions without user filtering.
func (r *ImageSessionRepo) ListAllForUser(userID string) ([]models.ImageSession, error) {
	query := `
		SELECT s.id, s.conversation_id, s.title, s.active_node_id, s.created_at, s.updated_at
		FROM image_sessions s
		JOIN conversations c ON c.id = s.conversation_id`
	var args []interface{}
	if userID != "" {
		query += ` WHERE c.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY s.updated_at DESC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all image sessions for user: %w", err)
	}
	defer rows.Close()

	var sessions []models.ImageSession
	for rows.Next() {
		var s models.ImageSession
		var activeNodeID sql.NullString
		if err := rows.Scan(&s.ID, &s.ConversationID, &s.Title, &activeNodeID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan image session: %w", err)
		}
		if activeNodeID.Valid {
			s.ActiveNodeID = &activeNodeID.String
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
