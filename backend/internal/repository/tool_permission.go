package repository

import (
	"database/sql"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// ToolPermissionRepo manages per-tool access policies in the DB.
type ToolPermissionRepo struct {
	db *sql.DB
}

// NewToolPermissionRepo creates a ToolPermissionRepo.
func NewToolPermissionRepo(db *sql.DB) *ToolPermissionRepo {
	return &ToolPermissionRepo{db: db}
}

// Get returns the permission for a single tool. If none exists the zero-value
// ToolPermission is returned (which means "allow" by default).
func (r *ToolPermissionRepo) Get(toolName string) (*models.ToolPermission, error) {
	row := r.db.QueryRow(
		"SELECT tool_name, policy, updated_at FROM tool_permissions WHERE tool_name = ?",
		toolName,
	)
	var p models.ToolPermission
	if err := row.Scan(&p.ToolName, &p.Policy, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// List returns all tool permissions.
func (r *ToolPermissionRepo) List() ([]models.ToolPermission, error) {
	rows, err := r.db.Query(
		"SELECT tool_name, policy, updated_at FROM tool_permissions ORDER BY tool_name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []models.ToolPermission
	for rows.Next() {
		var p models.ToolPermission
		if err := rows.Scan(&p.ToolName, &p.Policy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// Upsert sets the permission policy for a tool, creating or updating the row.
func (r *ToolPermissionRepo) Upsert(toolName, policy string) error {
	_, err := r.db.Exec(`
		INSERT INTO tool_permissions (tool_name, policy, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(tool_name) DO UPDATE SET policy = excluded.policy, updated_at = excluded.updated_at
	`, toolName, policy)
	return err
}

// Delete removes a tool permission, reverting to the default (allow).
func (r *ToolPermissionRepo) Delete(toolName string) error {
	_, err := r.db.Exec("DELETE FROM tool_permissions WHERE tool_name = ?", toolName)
	return err
}

// PolicyResolver returns a PermissionResolver function suitable for the
// tools.Executor.  It queries the DB for each tool name.
func (r *ToolPermissionRepo) PolicyResolver() func(string) string {
	return func(toolName string) string {
		p, err := r.Get(toolName)
		if err != nil || p == nil {
			return "" // default allow
		}
		return p.Policy
	}
}
