package repository

import (
	"database/sql"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type ImageReferenceRepo struct {
	db *sql.DB
}

func NewImageReferenceRepo(db *sql.DB) *ImageReferenceRepo {
	return &ImageReferenceRepo{db: db}
}

func (r *ImageReferenceRepo) Create(ref *models.ImageReference) error {
	if ref.ID == "" {
		ref.ID = uuid.New().String()
	}
	_, err := r.db.Exec(`
		INSERT INTO image_references (id, node_id, attachment_id, ref_role, sort_order)
		VALUES (?, ?, ?, ?, ?)`,
		ref.ID, ref.NodeID, ref.AttachmentID, ref.RefRole, ref.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("create image reference: %w", err)
	}
	return nil
}

func (r *ImageReferenceRepo) ListByNode(nodeID string) ([]models.ImageReference, error) {
	rows, err := r.db.Query(`
		SELECT id, node_id, attachment_id, ref_role, sort_order
		FROM image_references WHERE node_id = ? ORDER BY sort_order ASC`, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list references: %w", err)
	}
	defer rows.Close()

	var refs []models.ImageReference
	for rows.Next() {
		var ref models.ImageReference
		if err := rows.Scan(&ref.ID, &ref.NodeID, &ref.AttachmentID, &ref.RefRole, &ref.SortOrder); err != nil {
			return nil, fmt.Errorf("scan reference: %w", err)
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *ImageReferenceRepo) DeleteByNode(nodeID string) error {
	_, err := r.db.Exec(`DELETE FROM image_references WHERE node_id = ?`, nodeID)
	if err != nil {
		return fmt.Errorf("delete references: %w", err)
	}
	return nil
}
