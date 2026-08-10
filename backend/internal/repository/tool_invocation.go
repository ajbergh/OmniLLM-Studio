package repository

import (
	"database/sql"
	"fmt"
	"strings"
)

// ToolInvocationSummary is the privacy-safe operational view of one durable
// tool invocation. Arguments, result payloads, error text, and user identifiers
// are intentionally excluded from this model.
type ToolInvocationSummary struct {
	ID             string `json:"id"`
	ToolCallID     string `json:"tool_call_id"`
	ToolName       string `json:"tool_name"`
	Status         string `json:"status"`
	ApprovalStatus string `json:"approval_status"`
	ConversationID string `json:"conversation_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	DurationMS     int64  `json:"duration_ms"`
	ResultBytes    int64  `json:"result_bytes"`
	RetryCount     int64  `json:"retry_count"`
	CreatedAt      string `json:"created_at"`
	StartedAt      string `json:"started_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
}

// ToolInvocationListOptions controls the bounded diagnostics query.
type ToolInvocationListOptions struct {
	Limit    int
	ToolName string
	Status   string
}

// ToolInvocationRepo reads durable tool invocation audit records without
// exposing payload-bearing columns.
type ToolInvocationRepo struct{ db *sql.DB }

func NewToolInvocationRepo(db *sql.DB) *ToolInvocationRepo {
	return &ToolInvocationRepo{db: db}
}

// ListForUser returns newest-first operational summaries for the authenticated
// user scope. Solo mode naturally passes the empty user ID used by the runtime.
func (r *ToolInvocationRepo) ListForUser(userID string, options ToolInvocationListOptions) ([]ToolInvocationSummary, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("tool invocation repository is not configured")
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := `
		SELECT id, tool_call_id, tool_name, status, approval_status,
			conversation_id, run_id, duration_ms, result_bytes, retry_count,
			CAST(created_at AS TEXT), COALESCE(CAST(started_at AS TEXT), ''), COALESCE(CAST(completed_at AS TEXT), '')
		FROM tool_invocations
		WHERE user_id = ?`
	args := []interface{}{userID}
	if toolName := strings.TrimSpace(options.ToolName); toolName != "" {
		query += " AND tool_name = ?"
		args = append(args, toolName)
	}
	if status := strings.TrimSpace(options.Status); status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tool invocations: %w", err)
	}
	defer rows.Close()

	items := make([]ToolInvocationSummary, 0, limit)
	for rows.Next() {
		var item ToolInvocationSummary
		if err := rows.Scan(
			&item.ID,
			&item.ToolCallID,
			&item.ToolName,
			&item.Status,
			&item.ApprovalStatus,
			&item.ConversationID,
			&item.RunID,
			&item.DurationMS,
			&item.ResultBytes,
			&item.RetryCount,
			&item.CreatedAt,
			&item.StartedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tool invocation: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tool invocations: %w", err)
	}
	return items, nil
}
