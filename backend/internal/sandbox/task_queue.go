package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SandboxTaskQueued    = "queued"
	SandboxTaskLeased    = "leased"
	SandboxTaskRunning   = "running"
	SandboxTaskSucceeded = "succeeded"
	SandboxTaskFailed    = "failed"
	SandboxTaskCancelled = "cancelled"

	SandboxRetryNever      = "never"
	SandboxRetryIdempotent = "idempotent"
)

const (
	defaultSandboxTaskLease = 30 * time.Second
	maxSandboxTaskLease     = 5 * time.Minute
)

// SandboxTask is a durable execution intent. Create and Exec are persisted as
// immutable JSON so a worker never reconstructs authority from ambient state.
// RetryPolicy defaults to never: interrupted side-effecting work therefore
// fails closed rather than being silently replayed after a worker restart.
type SandboxTask struct {
	ID               string          `json:"id"`
	Owner            OwnerScope      `json:"owner"`
	Create           CreateRequest   `json:"create"`
	Exec             ExecRequest     `json:"exec"`
	RetryPolicy      string          `json:"retry_policy"`
	Status           string          `json:"status"`
	AttemptCount     int             `json:"attempt_count"`
	LeaseOwner       string          `json:"lease_owner,omitempty"`
	LeaseToken       string          `json:"lease_token,omitempty"`
	LeaseExpiresAt   *time.Time      `json:"lease_expires_at,omitempty"`
	SandboxSessionID string          `json:"sandbox_session_id,omitempty"`
	RuntimeID        string          `json:"runtime_id,omitempty"`
	ExecutionID      string          `json:"execution_id,omitempty"`
	Result           json.RawMessage `json:"result,omitempty"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

// SandboxTaskAttempt is immutable attempt evidence. Attempt records are created
// before runtime execution and finalized once; they are never reused for a
// subsequent retry.
type SandboxTaskAttempt struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	Attempt     int        `json:"attempt"`
	WorkerID    string     `json:"worker_id"`
	RuntimeID   string     `json:"runtime_id,omitempty"`
	ExecutionID string     `json:"execution_id,omitempty"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// SandboxTaskQueue stores durable execution intents plus lease and attempt
// state. It owns no goroutines and can be shared by desktop, server, or a future
// dedicated coordinator without changing the persistence contract.
type SandboxTaskQueue struct {
	db  *sql.DB
	now func() time.Time
}

func NewSandboxTaskQueue(db *sql.DB) (*SandboxTaskQueue, error) {
	if db == nil {
		return nil, fmt.Errorf("sandbox task database is required")
	}
	if err := EnsureSandboxTaskSchema(db); err != nil {
		return nil, err
	}
	return &SandboxTaskQueue{db: db, now: time.Now}, nil
}

// EnsureSandboxTaskSchema installs additive durable task tables. It is separate
// from scheduled_tasks because scheduled prompts and arbitrary sandbox
// executions have materially different restart/replay semantics.
func EnsureSandboxTaskSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("sandbox task database is required")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sandbox_tasks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			agent_run_id TEXT NOT NULL DEFAULT '',
			task_scope_id TEXT NOT NULL DEFAULT '',
			create_json TEXT NOT NULL,
			exec_json TEXT NOT NULL,
			retry_policy TEXT NOT NULL DEFAULT 'never',
			status TEXT NOT NULL DEFAULT 'queued',
			attempt_count INTEGER NOT NULL DEFAULT 0,
			lease_owner TEXT NOT NULL DEFAULT '',
			lease_token TEXT NOT NULL DEFAULT '',
			lease_expires_at DATETIME,
			sandbox_session_id TEXT NOT NULL DEFAULT '',
			runtime_id TEXT NOT NULL DEFAULT '',
			execution_id TEXT NOT NULL DEFAULT '',
			result_json TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sandbox_tasks_claim ON sandbox_tasks(status, lease_expires_at, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sandbox_tasks_owner ON sandbox_tasks(user_id, workspace_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS sandbox_task_attempts (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			worker_id TEXT NOT NULL,
			runtime_id TEXT NOT NULL DEFAULT '',
			execution_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			error_message TEXT NOT NULL DEFAULT '',
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			UNIQUE(task_id, attempt),
			FOREIGN KEY(task_id) REFERENCES sandbox_tasks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sandbox_task_attempts_task ON sandbox_task_attempts(task_id, attempt)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("ensure sandbox task schema: %w", err)
		}
	}
	return ensureSandboxTaskAssociationSchema(db)
}

func (q *SandboxTaskQueue) Enqueue(_ context.Context, owner OwnerScope, create CreateRequest, execReq ExecRequest, retryPolicy string) (*SandboxTask, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("sandbox task queue is unavailable")
	}
	if owner.Empty() || strings.TrimSpace(owner.UserID) == "" {
		return nil, fmt.Errorf("sandbox task owner is required")
	}
	if err := validateCreateRequest(create); err != nil {
		return nil, err
	}
	if err := validateExecRequest(execReq); err != nil {
		return nil, err
	}
	retryPolicy = strings.ToLower(strings.TrimSpace(retryPolicy))
	if retryPolicy == "" {
		retryPolicy = SandboxRetryNever
	}
	if retryPolicy != SandboxRetryNever && retryPolicy != SandboxRetryIdempotent {
		return nil, fmt.Errorf("sandbox task retry_policy must be never or idempotent")
	}
	// Attempt execution IDs are allocated at claim time. A caller-selected ID on
	// the immutable template must never be reused across retries.
	execReq.ExecutionID = ""
	createJSON, err := json.Marshal(create)
	if err != nil {
		return nil, fmt.Errorf("encode sandbox task create request: %w", err)
	}
	execJSON, err := json.Marshal(execReq)
	if err != nil {
		return nil, fmt.Errorf("encode sandbox task exec request: %w", err)
	}
	now := q.now().UTC()
	task := &SandboxTask{
		ID:          "sbt_" + uuid.NewString(),
		Owner:       owner,
		Create:      create,
		Exec:        execReq,
		RetryPolicy: retryPolicy,
		Status:      SandboxTaskQueued,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = q.db.Exec(`
		INSERT INTO sandbox_tasks (
			id, user_id, workspace_id, conversation_id, agent_run_id, task_scope_id,
			create_json, exec_json, retry_policy, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, owner.UserID, owner.WorkspaceID, owner.ConversationID, owner.AgentRunID, owner.TaskID,
		string(createJSON), string(execJSON), retryPolicy, SandboxTaskQueued, now, now)
	if err != nil {
		return nil, fmt.Errorf("enqueue sandbox task: %w", err)
	}
	return task, nil
}

// Claim leases one task for a worker. Expired leases are reconciled in the same
// transaction. The executor is responsible for calling
// CleanupExpiredRuntimeAssociations first; therefore a task with a known old
// runtime cannot be replayed until that runtime has been destroyed.
func (q *SandboxTaskQueue) Claim(ctx context.Context, workerID string, lease time.Duration) (*SandboxTask, *SandboxTaskAttempt, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("sandbox task queue is unavailable")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(workerID) > 128 {
		return nil, nil, fmt.Errorf("sandbox worker id is required")
	}
	if lease <= 0 {
		lease = defaultSandboxTaskLease
	}
	if lease > maxSandboxTaskLease {
		lease = maxSandboxTaskLease
	}
	now := q.now().UTC()
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE sandbox_task_attempts
		SET status = 'interrupted', error_message = 'worker lease expired', finished_at = ?
		WHERE status = 'running' AND task_id IN (
			SELECT id FROM sandbox_tasks
			WHERE status IN ('leased','running')
			  AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?
			  AND sandbox_session_id = '' AND runtime_id = ''
		)
	`, now, now); err != nil {
		return nil, nil, fmt.Errorf("record interrupted sandbox attempts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks
		SET status = CASE WHEN retry_policy = 'idempotent' THEN 'queued' ELSE 'failed' END,
			lease_owner = '', lease_token = '', lease_expires_at = NULL,
			error_message = CASE WHEN retry_policy = 'idempotent' THEN '' ELSE 'worker lease expired; automatic replay denied' END,
			completed_at = CASE WHEN retry_policy = 'idempotent' THEN NULL ELSE ? END,
			updated_at = ?
		WHERE status IN ('leased','running')
		  AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?
		  AND sandbox_session_id = '' AND runtime_id = ''
	`, now, now, now); err != nil {
		return nil, nil, fmt.Errorf("recover expired sandbox tasks: %w", err)
	}

	var taskID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM sandbox_tasks
		WHERE status = 'queued'
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				return nil, nil, err
			}
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("select sandbox task: %w", err)
	}

	leaseToken := "sbl_" + uuid.NewString()
	expires := now.Add(lease)
	claim, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks
		SET status = 'leased', lease_owner = ?, lease_token = ?, lease_expires_at = ?,
			attempt_count = attempt_count + 1, updated_at = ?, sandbox_session_id = '',
			runtime_id = '', execution_id = '', result_json = '', error_message = '', completed_at = NULL
		WHERE id = ? AND status = 'queued'
	`, workerID, leaseToken, expires, now, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("lease sandbox task: %w", err)
	}
	rows, _ := claim.RowsAffected()
	if rows != 1 {
		return nil, nil, fmt.Errorf("sandbox task claim lost concurrent race")
	}
	task, err := scanSandboxTaskTx(ctx, tx, taskID)
	if err != nil {
		return nil, nil, err
	}
	executionID := NewExecutionID()
	attempt := &SandboxTaskAttempt{
		ID:          "sba_" + uuid.NewString(),
		TaskID:      task.ID,
		Attempt:     task.AttemptCount,
		WorkerID:    workerID,
		ExecutionID: executionID,
		Status:      SandboxTaskRunning,
		StartedAt:   now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sandbox_task_attempts (
			id, task_id, attempt, worker_id, execution_id, status, started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, attempt.ID, attempt.TaskID, attempt.Attempt, attempt.WorkerID, attempt.ExecutionID, attempt.Status, attempt.StartedAt); err != nil {
		return nil, nil, fmt.Errorf("create sandbox task attempt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks SET status = 'running', execution_id = ?, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND lease_token = ? AND status = 'leased'
	`, executionID, now, task.ID, workerID, leaseToken); err != nil {
		return nil, nil, fmt.Errorf("activate sandbox task attempt: %w", err)
	}
	task.Status = SandboxTaskRunning
	task.LeaseOwner = workerID
	task.LeaseToken = leaseToken
	task.LeaseExpiresAt = &expires
	task.ExecutionID = executionID
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return task, attempt, nil
}

func (q *SandboxTaskQueue) Renew(ctx context.Context, taskID, workerID, leaseToken string, lease time.Duration) error {
	if lease <= 0 {
		lease = defaultSandboxTaskLease
	}
	if lease > maxSandboxTaskLease {
		lease = maxSandboxTaskLease
	}
	now := q.now().UTC()
	result, err := q.db.ExecContext(ctx, `
		UPDATE sandbox_tasks SET lease_expires_at = ?, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND lease_token = ? AND status = 'running'
	`, now.Add(lease), now, strings.TrimSpace(taskID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken))
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task lease is no longer owned by this worker")
	}
	return nil
}

// BindRuntime is retained for compatibility with queue callers that only need
// to bind an opaque runtime reference. New durable execution code should use
// BindRuntimeAssociation so both Broker session and runtime identities survive
// restart.
func (q *SandboxTaskQueue) BindRuntime(ctx context.Context, taskID, workerID, leaseToken, attemptID, runtimeID string) error {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return fmt.Errorf("sandbox runtime id is required")
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := q.now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks SET runtime_id = ?, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND lease_token = ? AND status = 'running'
	`, runtimeID, now, taskID, workerID, leaseToken)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task lease is no longer owned by this worker")
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE sandbox_task_attempts SET runtime_id = ?
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

func (q *SandboxTaskQueue) Complete(ctx context.Context, taskID, workerID, leaseToken, attemptID string, resultValue *ExecResult, runErr error) error {
	now := q.now().UTC()
	status := SandboxTaskSucceeded
	errorMessage := ""
	resultJSON := ""
	if runErr != nil {
		status = SandboxTaskFailed
		errorMessage = runErr.Error()
		if len(errorMessage) > 8192 {
			errorMessage = errorMessage[:8192]
		}
	} else if resultValue != nil {
		encoded, err := json.Marshal(resultValue)
		if err != nil {
			return fmt.Errorf("encode sandbox task result: %w", err)
		}
		if len(encoded) > 2<<20 {
			return fmt.Errorf("sandbox task result exceeds durable result limit")
		}
		resultJSON = string(encoded)
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE sandbox_tasks
		SET status = ?, result_json = ?, error_message = ?, lease_owner = '', lease_token = '',
			lease_expires_at = NULL, sandbox_session_id = '', runtime_id = '', completed_at = ?, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND lease_token = ? AND status = 'running'
	`, status, resultJSON, errorMessage, now, now, taskID, workerID, leaseToken)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task completion rejected because lease ownership changed")
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE sandbox_task_attempts
		SET status = ?, error_message = ?, finished_at = ?
		WHERE id = ? AND task_id = ? AND worker_id = ? AND status = 'running'
	`, status, errorMessage, now, attemptID, taskID, workerID)
	if err != nil {
		return err
	}
	rows, _ = result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("sandbox task attempt completion changed concurrently")
	}
	return tx.Commit()
}

func (q *SandboxTaskQueue) Get(ctx context.Context, taskID string, owner OwnerScope) (*SandboxTask, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("sandbox task queue is unavailable")
	}
	row := q.db.QueryRowContext(ctx, sandboxTaskSelect+` WHERE id = ? AND user_id = ?`, strings.TrimSpace(taskID), owner.UserID)
	task, err := scanSandboxTaskRow(row)
	if err != nil {
		return nil, err
	}
	if owner.WorkspaceID != "" && task.Owner.WorkspaceID != owner.WorkspaceID {
		return nil, fmt.Errorf("sandbox task is outside the current workspace")
	}
	return task, nil
}

const sandboxTaskSelect = `
	SELECT id, user_id, workspace_id, conversation_id, agent_run_id, task_scope_id,
		create_json, exec_json, retry_policy, status, attempt_count, lease_owner,
		lease_token, lease_expires_at, sandbox_session_id, runtime_id, execution_id,
		result_json, error_message, created_at, updated_at, completed_at
	FROM sandbox_tasks`

func scanSandboxTaskTx(ctx context.Context, tx *sql.Tx, taskID string) (*SandboxTask, error) {
	return scanSandboxTaskRow(tx.QueryRowContext(ctx, sandboxTaskSelect+` WHERE id = ?`, taskID))
}

type rowScanner interface {
	Scan(...any) error
}

func scanSandboxTaskRow(row rowScanner) (*SandboxTask, error) {
	var task SandboxTask
	var createJSON, execJSON, resultJSON string
	var leaseExpires, completed sql.NullTime
	if err := row.Scan(
		&task.ID, &task.Owner.UserID, &task.Owner.WorkspaceID, &task.Owner.ConversationID,
		&task.Owner.AgentRunID, &task.Owner.TaskID, &createJSON, &execJSON,
		&task.RetryPolicy, &task.Status, &task.AttemptCount, &task.LeaseOwner,
		&task.LeaseToken, &leaseExpires, &task.SandboxSessionID, &task.RuntimeID,
		&task.ExecutionID, &resultJSON, &task.ErrorMessage, &task.CreatedAt,
		&task.UpdatedAt, &completed,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(createJSON), &task.Create); err != nil {
		return nil, fmt.Errorf("decode sandbox task create request: %w", err)
	}
	if err := json.Unmarshal([]byte(execJSON), &task.Exec); err != nil {
		return nil, fmt.Errorf("decode sandbox task exec request: %w", err)
	}
	if resultJSON != "" {
		task.Result = json.RawMessage(resultJSON)
	}
	if leaseExpires.Valid {
		value := leaseExpires.Time.UTC()
		task.LeaseExpiresAt = &value
	}
	if completed.Valid {
		value := completed.Time.UTC()
		task.CompletedAt = &value
	}
	return &task, nil
}
