package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type ImageNodeRepo struct {
	db *sql.DB
}

func NewImageNodeRepo(db *sql.DB) *ImageNodeRepo {
	return &ImageNodeRepo{db: db}
}

func (r *ImageNodeRepo) Create(node *models.ImageNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.CreatedAt == "" {
		node.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.db.Exec(`
		INSERT INTO image_nodes (id, session_id, parent_node_id, operation_type, instruction, provider, model, seed, params_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.SessionID, node.ParentNodeID, node.OperationType,
		node.Instruction, node.Provider, node.Model, node.Seed, node.ParamsJSON, node.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create image node: %w", err)
	}
	return nil
}

func (r *ImageNodeRepo) GetByID(id string) (*models.ImageNode, error) {
	var n models.ImageNode
	var parentNodeID sql.NullString
	var seed sql.NullInt64
	var paramsJSON sql.NullString
	err := r.db.QueryRow(`
		SELECT id, session_id, parent_node_id, operation_type, instruction, provider, model, seed, params_json, created_at
		FROM image_nodes WHERE id = ?`, id,
	).Scan(&n.ID, &n.SessionID, &parentNodeID, &n.OperationType, &n.Instruction,
		&n.Provider, &n.Model, &seed, &paramsJSON, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get image node: %w", err)
	}
	if parentNodeID.Valid {
		n.ParentNodeID = &parentNodeID.String
	}
	if seed.Valid {
		s := int(seed.Int64)
		n.Seed = &s
	}
	if paramsJSON.Valid {
		n.ParamsJSON = &paramsJSON.String
	}
	return &n, nil
}

func (r *ImageNodeRepo) ListBySession(sessionID string) ([]models.ImageNode, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, parent_node_id, operation_type, instruction, provider, model, seed, params_json, created_at
		FROM image_nodes WHERE session_id = ? ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list image nodes: %w", err)
	}
	defer rows.Close()

	var nodes []models.ImageNode
	for rows.Next() {
		var n models.ImageNode
		var parentNodeID sql.NullString
		var seed sql.NullInt64
		var paramsJSON sql.NullString
		if err := rows.Scan(&n.ID, &n.SessionID, &parentNodeID, &n.OperationType, &n.Instruction,
			&n.Provider, &n.Model, &seed, &paramsJSON, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan image node: %w", err)
		}
		if parentNodeID.Valid {
			n.ParentNodeID = &parentNodeID.String
		}
		if seed.Valid {
			s := int(seed.Int64)
			n.Seed = &s
		}
		if paramsJSON.Valid {
			n.ParamsJSON = &paramsJSON.String
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *ImageNodeRepo) GetChildren(nodeID string) ([]models.ImageNode, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, parent_node_id, operation_type, instruction, provider, model, seed, params_json, created_at
		FROM image_nodes WHERE parent_node_id = ? ORDER BY created_at ASC`, nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	defer rows.Close()

	var nodes []models.ImageNode
	for rows.Next() {
		var n models.ImageNode
		var parentNodeID sql.NullString
		var seed sql.NullInt64
		var paramsJSON sql.NullString
		if err := rows.Scan(&n.ID, &n.SessionID, &parentNodeID, &n.OperationType, &n.Instruction,
			&n.Provider, &n.Model, &seed, &paramsJSON, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan child node: %w", err)
		}
		if parentNodeID.Valid {
			n.ParentNodeID = &parentNodeID.String
		}
		if seed.Valid {
			s := int(seed.Int64)
			n.Seed = &s
		}
		if paramsJSON.Valid {
			n.ParamsJSON = &paramsJSON.String
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
