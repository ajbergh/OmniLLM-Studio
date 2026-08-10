package models

import "time"

// ScopedToolPermission narrows tool policy at a user, workspace, or conversation scope.
// Scoped policy is monotonic: a child scope may tighten Allow -> Ask -> Deny but never widen it.
type ScopedToolPermission struct {
	ScopeType string    `json:"scope_type"`
	ScopeID   string    `json:"scope_id"`
	ToolName  string    `json:"tool_name"`
	Policy    string    `json:"policy"`
	UpdatedAt time.Time `json:"updated_at"`
}
