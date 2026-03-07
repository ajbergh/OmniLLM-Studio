package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type ImageMaskRepo struct {
	db *sql.DB
}

func NewImageMaskRepo(db *sql.DB) *ImageMaskRepo {
	return &ImageMaskRepo{db: db}
}

func (r *ImageMaskRepo) Create(mask *models.ImageMask) error {
	if mask.ID == "" {
		mask.ID = uuid.New().String()
	}
	if mask.CreatedAt == "" {
		mask.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.db.Exec(`
		INSERT INTO image_masks (id, node_id, attachment_id, stroke_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		mask.ID, mask.NodeID, mask.AttachmentID, mask.StrokeJSON, mask.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create image mask: %w", err)
	}
	return nil
}

func (r *ImageMaskRepo) GetByNode(nodeID string) (*models.ImageMask, error) {
	var m models.ImageMask
	var strokeJSON sql.NullString
	err := r.db.QueryRow(`
		SELECT id, node_id, attachment_id, stroke_json, created_at
		FROM image_masks WHERE node_id = ? ORDER BY created_at DESC LIMIT 1`, nodeID,
	).Scan(&m.ID, &m.NodeID, &m.AttachmentID, &strokeJSON, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get image mask: %w", err)
	}
	if strokeJSON.Valid {
		m.StrokeJSON = &strokeJSON.String
	}
	return &m, nil
}

// ListBySession returns all masks for all nodes in a session.
func (r *ImageMaskRepo) ListBySession(sessionID string) ([]models.ImageMask, error) {
	rows, err := r.db.Query(`
		SELECT m.id, m.node_id, m.attachment_id, m.stroke_json, m.created_at
		FROM image_masks m
		JOIN image_nodes n ON n.id = m.node_id
		WHERE n.session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list session masks: %w", err)
	}
	defer rows.Close()

	var masks []models.ImageMask
	for rows.Next() {
		var m models.ImageMask
		var strokeJSON sql.NullString
		if err := rows.Scan(&m.ID, &m.NodeID, &m.AttachmentID, &strokeJSON, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan session mask: %w", err)
		}
		if strokeJSON.Valid {
			m.StrokeJSON = &strokeJSON.String
		}
		masks = append(masks, m)
	}
	return masks, rows.Err()
}
