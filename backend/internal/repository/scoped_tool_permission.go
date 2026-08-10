package repository

import (
	"database/sql"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// ScopedToolPermissionRepo stores user/workspace/conversation policy overrides.
type ScopedToolPermissionRepo struct{ db *sql.DB }

func NewScopedToolPermissionRepo(db *sql.DB) *ScopedToolPermissionRepo {
	return &ScopedToolPermissionRepo{db: db}
}

func validScopedPolicy(policy string) bool {
	return policy == "allow" || policy == "ask" || policy == "deny"
}
func policyRank(policy string) int {
	switch policy {
	case "deny":
		return 2
	case "ask":
		return 1
	default:
		return 0
	}
}
func tighterPolicy(current, candidate string) string {
	if policyRank(candidate) > policyRank(current) {
		return candidate
	}
	return current
}

func (r *ScopedToolPermissionRepo) Upsert(scopeType, scopeID, toolName, policy string) error {
	if scopeType != "user" && scopeType != "workspace" && scopeType != "conversation" {
		return fmt.Errorf("scope_type must be user, workspace, or conversation")
	}
	if scopeID == "" || toolName == "" || !validScopedPolicy(policy) {
		return fmt.Errorf("scope_id, tool_name, and valid policy are required")
	}
	_, err := r.db.Exec(`INSERT INTO tool_permission_scopes (scope_type, scope_id, tool_name, policy, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(scope_type, scope_id, tool_name) DO UPDATE SET policy=excluded.policy, updated_at=excluded.updated_at`, scopeType, scopeID, toolName, policy)
	return err
}

func (r *ScopedToolPermissionRepo) Delete(scopeType, scopeID, toolName string) error {
	_, err := r.db.Exec(`DELETE FROM tool_permission_scopes WHERE scope_type=? AND scope_id=? AND tool_name=?`, scopeType, scopeID, toolName)
	return err
}

func (r *ScopedToolPermissionRepo) List(scopeType, scopeID string) ([]models.ScopedToolPermission, error) {
	rows, err := r.db.Query(`SELECT scope_type, scope_id, tool_name, policy, updated_at FROM tool_permission_scopes WHERE scope_type=? AND scope_id=? ORDER BY tool_name`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ScopedToolPermission
	for rows.Next() {
		var item models.ScopedToolPermission
		if err := rows.Scan(&item.ScopeType, &item.ScopeID, &item.ToolName, &item.Policy, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Resolve applies user -> workspace -> conversation restrictions monotonically.
func (r *ScopedToolPermissionRepo) Resolve(userID, workspaceID, conversationID, toolName, basePolicy string) string {
	policy := basePolicy
	for _, scope := range []struct{ kind, id string }{{"user", userID}, {"workspace", workspaceID}, {"conversation", conversationID}} {
		if scope.id == "" {
			continue
		}
		var candidate string
		err := r.db.QueryRow(`SELECT policy FROM tool_permission_scopes WHERE scope_type=? AND scope_id=? AND tool_name=?`, scope.kind, scope.id, toolName).Scan(&candidate)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return "deny"
		}
		if validScopedPolicy(candidate) {
			policy = tighterPolicy(policy, candidate)
		}
	}
	return policy
}
