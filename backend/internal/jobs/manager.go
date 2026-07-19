// Package jobs provides durable local asynchronous work for agent tools.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusPaused    = "paused"
)

// Scope identifies the owner and UI context for a job.
type Scope struct {
	UserID         string `json:"user_id,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// Job is the durable public state of asynchronous work.
type Job struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	UserID         string          `json:"user_id,omitempty"`
	WorkspaceID    string          `json:"workspace_id,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Status         string          `json:"status"`
	Progress       float64         `json:"progress"`
	Stage          string          `json:"stage,omitempty"`
	Request        json.RawMessage `json:"request,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
}

// Progress updates the persisted stage and normalized progress fraction 0..1.
type Progress func(stage string, fraction float64)

// Work executes a job and returns a JSON-serializable result.
type Work func(ctx context.Context, progress Progress) (interface{}, error)

type liveJob struct {
	cancel context.CancelFunc
}

// Manager owns durable job records and volatile cancellation contexts.
type Manager struct {
	db *sql.DB

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	live   map[string]*liveJob
}

func NewManager(db *sql.DB) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{db: db, ctx: ctx, cancel: cancel, live: make(map[string]*liveJob)}
	if _, err := db.Exec(`
		UPDATE agent_jobs
		SET status = 'failed', error_message = 'application restarted before job completed',
			updated_at = ?, completed_at = ?
		WHERE status IN ('queued', 'running')
	`, time.Now().UTC(), time.Now().UTC()); err != nil {
		cancel()
		return nil, fmt.Errorf("recover interrupted jobs: %w", err)
	}
	return manager, nil
}

// Start persists the request before launching work. The returned job can be
// safely presented to an LLM immediately.
func (m *Manager) Start(kind string, scope Scope, request interface{}, work Work) (*Job, error) {
	if work == nil {
		return nil, fmt.Errorf("job work function is required")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode job request: %w", err)
	}
	now := time.Now().UTC()
	job := &Job{
		ID: uuid.NewString(), Kind: kind, UserID: scope.UserID,
		WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
		Status: StatusQueued, Progress: 0, Stage: "queued",
		Request: requestJSON, Result: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	if _, err := m.db.Exec(`
		INSERT INTO agent_jobs (
			id, kind, user_id, workspace_id, conversation_id, status, progress, stage,
			request_json, result_json, error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '{}', '', ?, ?)
	`, job.ID, job.Kind, job.UserID, job.WorkspaceID, job.ConversationID,
		job.Status, job.Progress, job.Stage, string(job.Request), job.CreatedAt, job.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert agent job: %w", err)
	}

	jobCtx, cancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	m.live[job.ID] = &liveJob{cancel: cancel}
	m.mu.Unlock()
	m.wg.Add(1)
	go m.run(jobCtx, job.ID, work)
	return job, nil
}

func (m *Manager) run(ctx context.Context, jobID string, work Work) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		delete(m.live, jobID)
		m.mu.Unlock()
	}()
	now := time.Now().UTC()
	if _, err := m.db.Exec(`UPDATE agent_jobs SET status = 'running', stage = 'started', started_at = ?, updated_at = ? WHERE id = ?`, now, now, jobID); err != nil {
		log.Printf("[jobs] mark running %s: %v", jobID, err)
	}
	progress := func(stage string, fraction float64) {
		if fraction < 0 {
			fraction = 0
		}
		if fraction > 1 {
			fraction = 1
		}
		if _, err := m.db.Exec(`UPDATE agent_jobs SET stage = ?, progress = ?, updated_at = ? WHERE id = ? AND status = 'running'`, stage, fraction, time.Now().UTC(), jobID); err != nil {
			log.Printf("[jobs] progress %s: %v", jobID, err)
		}
	}
	result, err := work(ctx, progress)
	completedAt := time.Now().UTC()
	if err != nil {
		status := StatusFailed
		message := err.Error()
		if ctx.Err() == context.Canceled {
			status = StatusCancelled
			message = "job cancelled"
		}
		if _, updateErr := m.db.Exec(`
			UPDATE agent_jobs SET status = ?, stage = ?, error_message = ?, updated_at = ?, completed_at = ? WHERE id = ?
		`, status, status, message, completedAt, completedAt, jobID); updateErr != nil {
			log.Printf("[jobs] mark failed %s: %v", jobID, updateErr)
		}
		return
	}
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		resultJSON = []byte(`{"error":"job result could not be encoded"}`)
	}
	if _, err := m.db.Exec(`
		UPDATE agent_jobs SET status = 'completed', stage = 'completed', progress = 1,
			result_json = ?, error_message = '', updated_at = ?, completed_at = ? WHERE id = ?
	`, string(resultJSON), completedAt, completedAt, jobID); err != nil {
		log.Printf("[jobs] mark completed %s: %v", jobID, err)
	}
}

func (m *Manager) Get(id string) (*Job, error) {
	row := m.db.QueryRow(`
		SELECT id, kind, user_id, workspace_id, conversation_id, status, progress, stage,
			request_json, result_json, error_message, created_at, updated_at, started_at, completed_at
		FROM agent_jobs WHERE id = ?
	`, id)
	return scanJob(row)
}

func (m *Manager) List(scope Scope, limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := m.db.Query(`
		SELECT id, kind, user_id, workspace_id, conversation_id, status, progress, stage,
			request_json, result_json, error_message, created_at, updated_at, started_at, completed_at
		FROM agent_jobs
		WHERE (? = '' OR user_id = ?) AND (? = '' OR workspace_id = ?) AND (? = '' OR conversation_id = ?)
		ORDER BY created_at DESC LIMIT ?
	`, scope.UserID, scope.UserID, scope.WorkspaceID, scope.WorkspaceID, scope.ConversationID, scope.ConversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent jobs: %w", err)
	}
	defer rows.Close()
	out := make([]Job, 0)
	for rows.Next() {
		job, err := scanJobRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, rows.Err()
}

func (m *Manager) Cancel(id, userID string) error {
	job, err := m.Get(id)
	if err != nil || job == nil {
		return fmt.Errorf("job not found")
	}
	if userID != "" && job.UserID != "" && job.UserID != userID {
		return fmt.Errorf("job not found")
	}
	if job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled {
		return fmt.Errorf("job cannot be cancelled (status: %s)", job.Status)
	}
	m.mu.Lock()
	live := m.live[id]
	m.mu.Unlock()
	if live != nil {
		live.cancel()
	}
	now := time.Now().UTC()
	_, err = m.db.Exec(`UPDATE agent_jobs SET status = 'cancelled', stage = 'cancelled', error_message = 'job cancelled', updated_at = ?, completed_at = ? WHERE id = ?`, now, now, id)
	return err
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.cancel()
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanJob(row scanner) (*Job, error) {
	var job Job
	var request, result string
	if err := row.Scan(&job.ID, &job.Kind, &job.UserID, &job.WorkspaceID, &job.ConversationID,
		&job.Status, &job.Progress, &job.Stage, &request, &result, &job.ErrorMessage,
		&job.CreatedAt, &job.UpdatedAt, &job.StartedAt, &job.CompletedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan agent job: %w", err)
	}
	job.Request = json.RawMessage(request)
	job.Result = json.RawMessage(result)
	return &job, nil
}

func scanJobRows(rows *sql.Rows) (*Job, error) { return scanJob(rows) }
