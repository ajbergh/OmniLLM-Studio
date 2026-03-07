package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type ImageNodeAssetRepo struct {
	db *sql.DB
}

func NewImageNodeAssetRepo(db *sql.DB) *ImageNodeAssetRepo {
	return &ImageNodeAssetRepo{db: db}
}

func (r *ImageNodeAssetRepo) Create(asset *models.ImageNodeAsset) error {
	if asset.ID == "" {
		asset.ID = uuid.New().String()
	}
	if asset.CreatedAt == "" {
		asset.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.db.Exec(`
		INSERT INTO image_node_assets (id, node_id, attachment_id, variant_index, is_selected, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		asset.ID, asset.NodeID, asset.AttachmentID, asset.VariantIndex, boolToInt(asset.IsSelected), asset.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create image node asset: %w", err)
	}
	return nil
}

func (r *ImageNodeAssetRepo) ListByNode(nodeID string) ([]models.ImageNodeAsset, error) {
	rows, err := r.db.Query(`
		SELECT id, node_id, attachment_id, variant_index, is_selected, created_at
		FROM image_node_assets WHERE node_id = ? ORDER BY variant_index ASC`, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list node assets: %w", err)
	}
	defer rows.Close()

	var assets []models.ImageNodeAsset
	for rows.Next() {
		var a models.ImageNodeAsset
		var selected int
		if err := rows.Scan(&a.ID, &a.NodeID, &a.AttachmentID, &a.VariantIndex, &selected, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan node asset: %w", err)
		}
		a.IsSelected = selected != 0
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// ListBySession returns all assets for a session, optionally filtered by node operation_type.
func (r *ImageNodeAssetRepo) ListBySession(sessionID string, opTypes []string, sortDesc bool) ([]models.ImageNodeAsset, error) {
	query := `
		SELECT a.id, a.node_id, a.attachment_id, a.variant_index, a.is_selected, a.created_at
		FROM image_node_assets a
		JOIN image_nodes n ON a.node_id = n.id
		WHERE n.session_id = ?`

	args := []interface{}{sessionID}
	if len(opTypes) > 0 {
		placeholders := ""
		for i, ot := range opTypes {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, ot)
		}
		query += " AND n.operation_type IN (" + placeholders + ")"
	}

	if sortDesc {
		query += " ORDER BY a.created_at DESC"
	} else {
		query += " ORDER BY a.created_at ASC"
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list session assets: %w", err)
	}
	defer rows.Close()

	var assets []models.ImageNodeAsset
	for rows.Next() {
		var a models.ImageNodeAsset
		var selected int
		if err := rows.Scan(&a.ID, &a.NodeID, &a.AttachmentID, &a.VariantIndex, &selected, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan session asset: %w", err)
		}
		a.IsSelected = selected != 0
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func (r *ImageNodeAssetRepo) GetByID(id string) (*models.ImageNodeAsset, error) {
	var a models.ImageNodeAsset
	var selected int
	err := r.db.QueryRow(`
		SELECT id, node_id, attachment_id, variant_index, is_selected, created_at
		FROM image_node_assets WHERE id = ?`, id,
	).Scan(&a.ID, &a.NodeID, &a.AttachmentID, &a.VariantIndex, &selected, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get image node asset: %w", err)
	}
	a.IsSelected = selected != 0
	return &a, nil
}

func (r *ImageNodeAssetRepo) SetSelected(nodeID, assetID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE image_node_assets SET is_selected = 0 WHERE node_id = ?`, nodeID); err != nil {
		return fmt.Errorf("deselect all: %w", err)
	}
	if _, err := tx.Exec(`UPDATE image_node_assets SET is_selected = 1 WHERE id = ? AND node_id = ?`, assetID, nodeID); err != nil {
		return fmt.Errorf("select asset: %w", err)
	}
	return tx.Commit()
}

func (r *ImageNodeAssetRepo) DeleteByNode(nodeID string) error {
	_, err := r.db.Exec(`DELETE FROM image_node_assets WHERE node_id = ?`, nodeID)
	if err != nil {
		return fmt.Errorf("delete node assets: %w", err)
	}
	return nil
}

func (r *ImageNodeAssetRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM image_node_assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
