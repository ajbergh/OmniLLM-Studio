package sandbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func ensureSandboxTaskAssociationSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("sandbox task database is required")
	}
	rows, err := db.Query(`PRAGMA table_info(sandbox_tasks)`)
	if err != nil {
		return fmt.Errorf("inspect sandbox task schema: %w", err)
	}
	defer rows.Close()
	hasSessionID := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan sandbox task schema: %w", err)
		}
		if strings.EqualFold(name, "sandbox_session_id") {
			hasSessionID = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasSessionID {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE sandbox_tasks ADD COLUMN sandbox_session_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add sandbox session association: %w", err)
	}
	return nil
}

// BindRuntimeAssociation persists both the Broker-facing sandbox session ID and
// the underlying runtime ID before arbitrary code executes. Both identities are
// required because a control-plane restart loses the Broker's in-memory mapping
// while a remote worker runtime may remain alive.
func (q *SandboxTaskQueue) BindRuntimeAssociation(ctx context.Context, taskID, workerID, leaseToken, attemptID, sandboxSessionID, runtimeID string) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("sandbox task queue is unavailable")
	}
	if err := ensureSandboxTaskAssociationSchema(q.db); err != nil {
		return err
	}
	sandboxSessionID = strings.TrimSpace(sandboxSessionID)
	runtimeID = strings.TrimSpace(runtimeID)
	if sandboxSessionID == "" || runtimeID == "" {
		return fmt.Errorf("sandbox session and runtime ids are required")
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := q.now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks
		SET sandbox_session_id = ?, runtime_id = ?, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND lease_token = ? AND status = 'running'
	`, sandboxSessionID, runtimeID, now, taskID, workerID, leaseToken)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task lease is no longer owned by this worker")
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE sandbox_task_attempts
		SET runtime_id = ?
		WHERE id = ? AND task_id = ? AND worker_id = ? AND status = 'running'
	`, runtimeID, attemptID, taskID, workerID)
	if err != nil {
		return err
	}
	rows, _ = result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task attempt is no longer active")
	}
	return tx.Commit()
}

type expiredSandboxRuntime struct {
	TaskID           string
	Owner            OwnerScope
	SandboxSessionID string
	RuntimeID        string
}

// CleanupExpiredRuntimeAssociations destroys every known runtime whose task
// lease has expired before Claim is allowed to requeue or fail that task. If
// any cleanup cannot be confirmed, this method returns an error and the caller
// must not claim additional work; that prevents a retry from overlapping an
// unproven old process tree.
func (q *SandboxTaskQueue) CleanupExpiredRuntimeAssociations(ctx context.Context, broker *Broker) error {
	if q == nil || q.db == nil || broker == nil {
		return fmt.Errorf("sandbox task recovery is not configured")
	}
	if err := ensureSandboxTaskAssociationSchema(q.db); err != nil {
		return err
	}
	now := q.now().UTC()
	rows, err := q.db.QueryContext(ctx, `
		SELECT id, user_id, workspace_id, conversation_id, agent_run_id, task_scope_id,
			sandbox_session_id, runtime_id
		FROM sandbox_tasks
		WHERE status IN ('leased','running')
		  AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?
		  AND sandbox_session_id <> '' AND runtime_id <> ''
		ORDER BY created_at ASC, id ASC
	`, now)
	if err != nil {
		return fmt.Errorf("list expired sandbox runtime associations: %w", err)
	}
	defer rows.Close()
	var expired []expiredSandboxRuntime
	for rows.Next() {
		var item expiredSandboxRuntime
		if err := rows.Scan(
			&item.TaskID,
			&item.Owner.UserID,
			&item.Owner.WorkspaceID,
			&item.Owner.ConversationID,
			&item.Owner.AgentRunID,
			&item.Owner.TaskID,
			&item.SandboxSessionID,
			&item.RuntimeID,
		); err != nil {
			return fmt.Errorf("scan expired sandbox runtime association: %w", err)
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range expired {
		if err := broker.DestroyRecordedRuntime(ctx, item.Owner, item.SandboxSessionID, item.RuntimeID); err != nil {
			return fmt.Errorf("destroy expired sandbox runtime for task %s: %w", item.TaskID, err)
		}
		result, err := q.db.ExecContext(ctx, `
			UPDATE sandbox_tasks SET sandbox_session_id = '', runtime_id = '', updated_at = ?
			WHERE id = ? AND status IN ('leased','running')
			  AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?
			  AND sandbox_session_id = ? AND runtime_id = ?
		`, time.Now().UTC(), item.TaskID, now, item.SandboxSessionID, item.RuntimeID)
		if err != nil {
			return fmt.Errorf("clear expired sandbox runtime association: %w", err)
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return fmt.Errorf("sandbox task recovery association changed concurrently")
		}
	}
	return nil
}
