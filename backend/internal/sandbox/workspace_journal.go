package sandbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxRevertSnapshotBytes = 2 << 20

// WorkspaceChange records one filesystem mutation without exposing a host path.
type WorkspaceChange struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id,omitempty"`
	AgentRunID     string    `json:"agent_run_id,omitempty"`
	TaskID         string    `json:"task_id,omitempty"`
	SandboxID      string    `json:"sandbox_id,omitempty"`
	ExecutionID    string    `json:"execution_id,omitempty"`
	RelativePath   string    `json:"relative_path"`
	Operation      string    `json:"operation"`
	BeforeExists   bool      `json:"before_exists"`
	BeforeSHA256   string    `json:"before_sha256,omitempty"`
	AfterExists    bool      `json:"after_exists"`
	AfterSHA256    string    `json:"after_sha256,omitempty"`
	Revertable     bool      `json:"revertable"`
	CreatedAt      time.Time `json:"created_at"`
}

// FileState is a bounded snapshot used for journaling and stale-state checks.
type FileState struct {
	Exists     bool
	SHA256     string
	Mode       os.FileMode
	Content    []byte
	Revertable bool
}

// CaptureFileState returns a safe bounded snapshot for one regular file. Missing
// files are represented as Exists=false and are revertable as a delete of a
// subsequently created file. Symlinks and non-regular files are not treated as
// automatically revertable by this journal.
func CaptureFileState(path string) (FileState, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return FileState{Exists: false, Revertable: true}, nil
	}
	if err != nil {
		return FileState{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return FileState{Exists: true, Mode: info.Mode(), Revertable: false}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return FileState{}, err
	}
	defer file.Close()
	hash := sha256.New()
	content := make([]byte, 0, minInt64(info.Size(), maxRevertSnapshotBytes))
	buffer := make([]byte, 64*1024)
	var total int64
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
			if total < maxRevertSnapshotBytes {
				remaining := maxRevertSnapshotBytes - total
				copyN := int64(n)
				if copyN > remaining {
					copyN = remaining
				}
				content = append(content, buffer[:copyN]...)
			}
			total += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return FileState{}, readErr
		}
	}
	return FileState{
		Exists:     true,
		SHA256:     hex.EncodeToString(hash.Sum(nil)),
		Mode:       info.Mode().Perm(),
		Content:    content,
		Revertable: info.Size() <= maxRevertSnapshotBytes,
	}, nil
}

// RecordWorkspaceChange persists a change after the caller has captured before
// and after state. The record is ownership scoped and omits physical host paths.
func (r *WorkspaceRegistry) RecordWorkspaceChange(ctx context.Context, owner OwnerScope, workspaceID, relativePath, operation, sandboxID, executionID string, before, after FileState) (*WorkspaceChange, error) {
	if owner.UserID == "" {
		return nil, fmt.Errorf("workspace change owner is required")
	}
	if _, err := r.Get(owner.UserID, workspaceID); err != nil {
		return nil, err
	}
	relativePath = strings.TrimSpace(filepath.ToSlash(relativePath))
	if relativePath == "" || strings.HasPrefix(relativePath, "/") || relativePath == ".." || strings.HasPrefix(relativePath, "../") {
		return nil, fmt.Errorf("invalid workspace-relative path")
	}
	operation = strings.TrimSpace(operation)
	switch operation {
	case "create", "write", "delete", "revert":
	default:
		return nil, fmt.Errorf("unsupported workspace change operation %q", operation)
	}
	id := "wch_" + uuid.NewString()
	now := time.Now().UTC()
	revertable := before.Revertable
	var beforeContent []byte
	var beforeMode any
	if before.Revertable && before.Exists {
		beforeContent = append([]byte(nil), before.Content...)
		beforeMode = int64(before.Mode.Perm())
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO sandbox_workspace_changes (
    id, workspace_id, user_id, conversation_id, agent_run_id, task_id,
    sandbox_id, execution_id, relative_path, operation,
    before_exists, before_sha256, after_exists, after_sha256,
    before_content, before_mode, revertable, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, owner.UserID, nullableString(owner.ConversationID), nullableString(owner.AgentRunID), nullableString(owner.TaskID),
		nullableString(sandboxID), nullableString(executionID), relativePath, operation,
		boolInt(before.Exists), nullableString(before.SHA256), boolInt(after.Exists), nullableString(after.SHA256),
		beforeContent, beforeMode, boolInt(revertable), now)
	if err != nil {
		return nil, fmt.Errorf("record workspace change: %w", err)
	}
	return &WorkspaceChange{
		ID: id, WorkspaceID: workspaceID, UserID: owner.UserID,
		ConversationID: owner.ConversationID, AgentRunID: owner.AgentRunID, TaskID: owner.TaskID,
		SandboxID: sandboxID, ExecutionID: executionID, RelativePath: relativePath, Operation: operation,
		BeforeExists: before.Exists, BeforeSHA256: before.SHA256, AfterExists: after.Exists, AfterSHA256: after.SHA256,
		Revertable: revertable, CreatedAt: now,
	}, nil
}

// ListWorkspaceChanges returns bounded newest-first owner-scoped history.
func (r *WorkspaceRegistry) ListWorkspaceChanges(ctx context.Context, ownerUserID, workspaceID string, limit int) ([]WorkspaceChange, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, workspace_id, user_id,
COALESCE(conversation_id,''), COALESCE(agent_run_id,''), COALESCE(task_id,''),
COALESCE(sandbox_id,''), COALESCE(execution_id,''), relative_path, operation,
before_exists, COALESCE(before_sha256,''), after_exists, COALESCE(after_sha256,''), revertable, created_at
FROM sandbox_workspace_changes
WHERE user_id=? AND workspace_id=?
ORDER BY created_at DESC, id DESC LIMIT ?`, ownerUserID, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceChange
	for rows.Next() {
		var change WorkspaceChange
		var beforeExists, afterExists, revertable int
		if err := rows.Scan(&change.ID, &change.WorkspaceID, &change.UserID,
			&change.ConversationID, &change.AgentRunID, &change.TaskID,
			&change.SandboxID, &change.ExecutionID, &change.RelativePath, &change.Operation,
			&beforeExists, &change.BeforeSHA256, &afterExists, &change.AfterSHA256, &revertable, &change.CreatedAt); err != nil {
			return nil, err
		}
		change.BeforeExists = beforeExists != 0
		change.AfterExists = afterExists != 0
		change.Revertable = revertable != 0
		out = append(out, change)
	}
	return out, rows.Err()
}

// loadChangeForRevert returns internal snapshot data only after exact ownership
// checks. It is intentionally not a model-facing API.
func (r *WorkspaceRegistry) loadChangeForRevert(ctx context.Context, ownerUserID, changeID string) (WorkspaceChange, []byte, os.FileMode, error) {
	var change WorkspaceChange
	var beforeExists, afterExists, revertable int
	var beforeContent []byte
	var beforeMode sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, workspace_id, user_id,
COALESCE(conversation_id,''), COALESCE(agent_run_id,''), COALESCE(task_id,''),
COALESCE(sandbox_id,''), COALESCE(execution_id,''), relative_path, operation,
before_exists, COALESCE(before_sha256,''), after_exists, COALESCE(after_sha256,''),
before_content, before_mode, revertable, created_at
FROM sandbox_workspace_changes WHERE id=? AND user_id=?`, changeID, ownerUserID).
		Scan(&change.ID, &change.WorkspaceID, &change.UserID,
			&change.ConversationID, &change.AgentRunID, &change.TaskID,
			&change.SandboxID, &change.ExecutionID, &change.RelativePath, &change.Operation,
			&beforeExists, &change.BeforeSHA256, &afterExists, &change.AfterSHA256,
			&beforeContent, &beforeMode, &revertable, &change.CreatedAt)
	if err == sql.ErrNoRows {
		return WorkspaceChange{}, nil, 0, fmt.Errorf("workspace change not found")
	}
	if err != nil {
		return WorkspaceChange{}, nil, 0, err
	}
	change.BeforeExists = beforeExists != 0
	change.AfterExists = afterExists != 0
	change.Revertable = revertable != 0
	mode := os.FileMode(0)
	if beforeMode.Valid {
		mode = os.FileMode(beforeMode.Int64)
	}
	return change, beforeContent, mode, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
