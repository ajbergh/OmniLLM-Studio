package repository

import (
	"database/sql"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// WorkspaceMemberRepo handles CRUD operations for workspace memberships.
type WorkspaceMemberRepo struct {
	db *sql.DB
}

// NewWorkspaceMemberRepo creates a new WorkspaceMemberRepo.
func NewWorkspaceMemberRepo(db *sql.DB) *WorkspaceMemberRepo {
	return &WorkspaceMemberRepo{db: db}
}

// ListByWorkspace returns all members of a workspace.
func (r *WorkspaceMemberRepo) ListByWorkspace(workspaceID string) ([]models.WorkspaceMember, error) {
	rows, err := r.db.Query(
		`SELECT workspace_id, user_id, role, joined_at
		 FROM workspace_members WHERE workspace_id = ? ORDER BY joined_at ASC`, workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	defer rows.Close()

	var members []models.WorkspaceMember
	for rows.Next() {
		var m models.WorkspaceMember
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan workspace member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// ListByUser returns all workspaces a user is a member of.
func (r *WorkspaceMemberRepo) ListByUser(userID string) ([]models.WorkspaceMember, error) {
	rows, err := r.db.Query(
		`SELECT workspace_id, user_id, role, joined_at
		 FROM workspace_members WHERE user_id = ? ORDER BY joined_at ASC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user memberships: %w", err)
	}
	defer rows.Close()

	var members []models.WorkspaceMember
	for rows.Next() {
		var m models.WorkspaceMember
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan workspace member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// IsMember checks if a user is a member of a workspace.
func (r *WorkspaceMemberRepo) IsMember(workspaceID, userID string) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM workspace_members WHERE workspace_id = ? AND user_id = ?",
		workspaceID, userID,
	).Scan(&count)
	return count > 0, err
}

// GetMembership retrieves a specific membership.
func (r *WorkspaceMemberRepo) GetMembership(workspaceID, userID string) (*models.WorkspaceMember, error) {
	m := &models.WorkspaceMember{}
	err := r.db.QueryRow(
		`SELECT workspace_id, user_id, role, joined_at
		 FROM workspace_members WHERE workspace_id = ? AND user_id = ?`,
		workspaceID, userID,
	).Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return m, nil
}

// Add creates a workspace membership.
func (r *WorkspaceMemberRepo) Add(workspaceID, userID, role string) error {
	_, err := r.db.Exec(
		`INSERT OR IGNORE INTO workspace_members (workspace_id, user_id, role)
		 VALUES (?, ?, ?)`,
		workspaceID, userID, role,
	)
	return err
}

// UpdateRole changes a member's role in a workspace.
func (r *WorkspaceMemberRepo) UpdateRole(workspaceID, userID, role string) error {
	_, err := r.db.Exec(
		`UPDATE workspace_members SET role = ? WHERE workspace_id = ? AND user_id = ?`,
		role, workspaceID, userID,
	)
	return err
}

// Remove deletes a workspace membership.
func (r *WorkspaceMemberRepo) Remove(workspaceID, userID string) error {
	_, err := r.db.Exec(
		"DELETE FROM workspace_members WHERE workspace_id = ? AND user_id = ?",
		workspaceID, userID,
	)
	return err
}
