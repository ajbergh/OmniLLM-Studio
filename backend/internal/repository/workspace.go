package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// WorkspaceRepo handles workspace CRUD operations.
type WorkspaceRepo struct {
	db *sql.DB
}

// NewWorkspaceRepo creates a new WorkspaceRepo.
func NewWorkspaceRepo(db *sql.DB) *WorkspaceRepo {
	return &WorkspaceRepo{db: db}
}

// List returns all workspaces ordered by sort_order, then name.
func (r *WorkspaceRepo) List() ([]models.Workspace, error) {
	rows, err := r.db.Query(`
		SELECT id, name, description, color, icon, sort_order, created_at, updated_at
		FROM workspaces
		ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []models.Workspace
	for rows.Next() {
		var w models.Workspace
		if err := rows.Scan(
			&w.ID, &w.Name, &w.Description, &w.Color, &w.Icon,
			&w.SortOrder, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, rows.Err()
}

// GetByID retrieves a single workspace by ID.
func (r *WorkspaceRepo) GetByID(id string) (*models.Workspace, error) {
	var w models.Workspace
	err := r.db.QueryRow(`
		SELECT id, name, description, color, icon, sort_order, created_at, updated_at
		FROM workspaces WHERE id = ?`, id).Scan(
		&w.ID, &w.Name, &w.Description, &w.Color, &w.Icon,
		&w.SortOrder, &w.CreatedAt, &w.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return &w, nil
}

// CreateWorkspaceInput holds the fields for creating a workspace.
type CreateWorkspaceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
}

// Create inserts a new workspace.
func (r *WorkspaceRepo) Create(input CreateWorkspaceInput) (*models.Workspace, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	color := input.Color
	if color == "" {
		color = "#6366f1"
	}
	icon := input.Icon
	if icon == "" {
		icon = "folder"
	}

	_, err := r.db.Exec(`
		INSERT INTO workspaces (id, name, description, color, icon, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		id, input.Name, input.Description, color, icon, now, now)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return r.GetByID(id)
}

// UpdateWorkspaceInput holds the fields for updating a workspace.
type UpdateWorkspaceInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	SortOrder   *int    `json:"sort_order,omitempty"`
}

// Update modifies an existing workspace.
func (r *WorkspaceRepo) Update(id string, input UpdateWorkspaceInput) (*models.Workspace, error) {
	sets := []string{}
	args := []interface{}{}

	if input.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *input.Name)
	}
	if input.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *input.Description)
	}
	if input.Color != nil {
		sets = append(sets, "color = ?")
		args = append(args, *input.Color)
	}
	if input.Icon != nil {
		sets = append(sets, "icon = ?")
		args = append(args, *input.Icon)
	}
	if input.SortOrder != nil {
		sets = append(sets, "sort_order = ?")
		args = append(args, *input.SortOrder)
	}

	if len(sets) == 0 {
		return r.GetByID(id)
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)

	query := "UPDATE workspaces SET "
	for i, s := range sets {
		if i > 0 {
			query += ", "
		}
		query += s
	}
	query += " WHERE id = ?"

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	return r.GetByID(id)
}

// Delete removes a workspace. Conversations are un-assigned (workspace_id set to NULL on cascade).
func (r *WorkspaceRepo) Delete(id string) error {
	// Un-assign conversations first (ON DELETE SET NULL handles this via FK, but be explicit)
	_, err := r.db.Exec("UPDATE conversations SET workspace_id = NULL WHERE workspace_id = ?", id)
	if err != nil {
		return fmt.Errorf("un-assign conversations: %w", err)
	}
	// Un-assign templates
	_, err = r.db.Exec("UPDATE prompt_templates SET workspace_id = NULL WHERE workspace_id = ?", id)
	if err != nil {
		return fmt.Errorf("un-assign templates: %w", err)
	}
	_, err = r.db.Exec("DELETE FROM workspaces WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

// GetStats retrieves aggregate statistics for a workspace.
func (r *WorkspaceRepo) GetStats(id string) (*models.WorkspaceStats, error) {
	var stats models.WorkspaceStats

	// Single round-trip instead of three separate COUNT queries.
	err := r.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM conversations WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM messages m
			 JOIN conversations c ON c.id = m.conversation_id
			 WHERE c.workspace_id = ?),
			(SELECT COUNT(*) FROM prompt_templates WHERE workspace_id = ?)
	`, id, id, id).Scan(&stats.ConversationCount, &stats.MessageCount, &stats.TemplateCount)
	if err != nil {
		return nil, fmt.Errorf("get workspace stats: %w", err)
	}

	return &stats, nil
}
