// Package tasks provides local one-time, recurring, and condition-watch agent tasks.
package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

const (
	KindOneTime   = "one_time"
	KindInterval  = "interval"
	KindCondition = "condition"

	StatusActive  = "active"
	StatusPaused  = "paused"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

const MinimumInterval = time.Hour

// Task is a durable scheduled agent instruction.
type Task struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id,omitempty"`
	ConversationID  string    `json:"conversation_id"`
	Title           string    `json:"title"`
	Prompt          string    `json:"prompt"`
	Profile         string    `json:"profile"`
	Timezone        string    `json:"timezone"`
	ScheduleKind    string    `json:"schedule_kind"`
	NextRunAt       time.Time `json:"next_run_at"`
	IntervalSeconds int64     `json:"interval_seconds"`
	Status          string    `json:"status"`
	LastRunID       string    `json:"last_run_id,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateRequest creates one task. Times are absolute RFC3339 values; clients
// resolve user-friendly recurrences before calling the backend.
type CreateRequest struct {
	UserID          string
	ConversationID  string
	Title           string
	Prompt          string
	Profile         agent.RunProfile
	Timezone        string
	ScheduleKind    string
	NextRunAt       time.Time
	IntervalSeconds int64
}

// Scheduler polls SQLite for due tasks and executes them through Agent Runner.
type Scheduler struct {
	db       *sql.DB
	runner   *agent.Runner
	convos   *repository.ConversationRepo
	messages *repository.MessageRepo

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewScheduler(db *sql.DB, runner *agent.Runner, convos *repository.ConversationRepo, messages *repository.MessageRepo) (*Scheduler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{db: db, runner: runner, convos: convos, messages: messages, ctx: ctx, cancel: cancel}
	if _, err := db.Exec(`UPDATE scheduled_tasks SET status = 'active', last_error = 'application restarted during task execution', updated_at = ? WHERE status = 'running'`, time.Now().UTC()); err != nil {
		cancel()
		return nil, fmt.Errorf("recover scheduled tasks: %w", err)
	}
	s.wg.Add(1)
	go s.loop()
	return s, nil
}

func (s *Scheduler) Create(req CreateRequest) (*Task, error) {
	if s.runner == nil {
		return nil, fmt.Errorf("agent runner unavailable")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.UserID == "" || req.ConversationID == "" || req.Prompt == "" {
		return nil, fmt.Errorf("user_id, conversation_id, and prompt are required")
	}
	if req.Title == "" {
		req.Title = "Scheduled agent task"
	}
	if len(req.Title) > 200 || len(req.Prompt) > 20000 {
		return nil, fmt.Errorf("task title or prompt is too long")
	}
	if req.Profile == "" {
		req.Profile = agent.ProfileAgent
	}
	if req.Profile != agent.ProfileAgent && req.Profile != agent.ProfileResearch && req.Profile != agent.ProfileChat {
		return nil, fmt.Errorf("profile must be chat, research, or agent")
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone %q", req.Timezone)
	}
	switch req.ScheduleKind {
	case KindOneTime:
		req.IntervalSeconds = 0
	case KindInterval, KindCondition:
		if time.Duration(req.IntervalSeconds)*time.Second < MinimumInterval {
			return nil, fmt.Errorf("recurring tasks must run no more frequently than once per hour")
		}
	default:
		return nil, fmt.Errorf("schedule_kind must be one_time, interval, or condition")
	}
	if req.NextRunAt.IsZero() {
		return nil, fmt.Errorf("next_run_at is required")
	}
	now := time.Now().UTC()
	task := &Task{
		ID: uuid.NewString(), UserID: req.UserID, ConversationID: req.ConversationID,
		Title: req.Title, Prompt: req.Prompt, Profile: string(req.Profile), Timezone: req.Timezone,
		ScheduleKind: req.ScheduleKind, NextRunAt: req.NextRunAt.UTC(), IntervalSeconds: req.IntervalSeconds,
		Status: StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.db.Exec(`
		INSERT INTO scheduled_tasks (
			id, user_id, conversation_id, title, prompt, profile, timezone, schedule_kind,
			next_run_at, interval_seconds, status, last_run_id, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', '', '', ?, ?)
	`, task.ID, task.UserID, task.ConversationID, task.Title, task.Prompt, task.Profile,
		task.Timezone, task.ScheduleKind, task.NextRunAt, task.IntervalSeconds, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create scheduled task: %w", err)
	}
	return task, nil
}

func (s *Scheduler) Get(id, userID string) (*Task, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, conversation_id, title, prompt, profile, timezone, schedule_kind,
			next_run_at, interval_seconds, status, last_run_id, last_error, created_at, updated_at
		FROM scheduled_tasks WHERE id = ? AND user_id = ?
	`, id, userID)
	return scanTask(row)
}

func (s *Scheduler) List(userID string, limit int) ([]Task, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, conversation_id, title, prompt, profile, timezone, schedule_kind,
			next_run_at, interval_seconds, status, last_run_id, last_error, created_at, updated_at
		FROM scheduled_tasks WHERE user_id = ? ORDER BY created_at DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list scheduled tasks: %w", err)
	}
	defer rows.Close()
	out := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *task)
	}
	return out, rows.Err()
}

func (s *Scheduler) SetStatus(id, userID, status string) error {
	if status != StatusActive && status != StatusPaused {
		return fmt.Errorf("status must be active or paused")
	}
	result, err := s.db.Exec(`UPDATE scheduled_tasks SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?`, status, time.Now().UTC(), id, userID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("scheduled task not found")
	}
	return nil
}

func (s *Scheduler) Delete(id, userID string) error {
	result, err := s.db.Exec(`DELETE FROM scheduled_tasks WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("scheduled task not found")
	}
	return nil
}

func (s *Scheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.dispatchDue()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) dispatchDue() {
	rows, err := s.db.Query(`
		SELECT id, user_id, conversation_id, title, prompt, profile, timezone, schedule_kind,
			next_run_at, interval_seconds, status, last_run_id, last_error, created_at, updated_at
		FROM scheduled_tasks WHERE status = 'active' AND next_run_at <= ? ORDER BY next_run_at ASC LIMIT 10
	`, time.Now().UTC())
	if err != nil {
		log.Printf("[tasks] query due: %v", err)
		return
	}
	var due []Task
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			log.Printf("[tasks] scan due: %v", scanErr)
			continue
		}
		due = append(due, *task)
	}
	rows.Close()
	for i := range due {
		task := due[i]
		result, err := s.db.Exec(`UPDATE scheduled_tasks SET status = 'running', updated_at = ? WHERE id = ? AND status = 'active'`, time.Now().UTC(), task.ID)
		if err != nil {
			continue
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.execute(task)
		}()
	}
}

func (s *Scheduler) execute(task Task) {
	convo, err := s.convos.GetByID(task.ConversationID)
	if err != nil || convo == nil {
		s.finish(task, "", fmt.Errorf("conversation not found"))
		return
	}
	messages, err := s.messages.ListByConversation(task.ConversationID)
	if err != nil {
		s.finish(task, "", err)
		return
	}
	history := make([]llm.ChatMessage, 0, len(messages))
	for _, message := range messages {
		history = append(history, llm.ChatMessage{Role: message.Role, Content: message.Content})
	}
	provider, model := "", ""
	if convo.DefaultProvider != nil {
		provider = *convo.DefaultProvider
	}
	if convo.DefaultModel != nil {
		model = *convo.DefaultModel
	}
	prompt := task.Prompt
	if task.ScheduleKind == KindCondition {
		prompt = "Check this condition using current evidence. Only produce a user-facing notification when the condition is met; otherwise state that no notification is required: " + task.Prompt
	}
	run, runErr := s.runner.StartRunWithOptions(s.ctx, task.ConversationID, prompt, provider, model, history, agent.RunOptions{Profile: agent.RunProfile(task.Profile)}, nil)
	runID := ""
	if run != nil {
		runID = run.ID
	}
	s.finish(task, runID, runErr)
}

func (s *Scheduler) finish(task Task, runID string, runErr error) {
	now := time.Now().UTC()
	lastError := ""
	if runErr != nil {
		lastError = runErr.Error()
	}
	status := StatusDone
	nextRun := task.NextRunAt
	if task.ScheduleKind == KindInterval || task.ScheduleKind == KindCondition {
		status = StatusActive
		interval := time.Duration(task.IntervalSeconds) * time.Second
		nextRun = task.NextRunAt.Add(interval)
		for !nextRun.After(now) {
			nextRun = nextRun.Add(interval)
		}
	} else if runErr != nil {
		status = StatusFailed
	}
	_, err := s.db.Exec(`
		UPDATE scheduled_tasks SET status = ?, next_run_at = ?, last_run_id = ?, last_error = ?, updated_at = ? WHERE id = ?
	`, status, nextRun, runID, lastError, now, task.ID)
	if err != nil {
		log.Printf("[tasks] finish %s: %v", task.ID, err)
	}
}

func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type scanner interface{ Scan(dest ...interface{}) error }

func scanTask(row scanner) (*Task, error) {
	var task Task
	if err := row.Scan(&task.ID, &task.UserID, &task.ConversationID, &task.Title, &task.Prompt,
		&task.Profile, &task.Timezone, &task.ScheduleKind, &task.NextRunAt, &task.IntervalSeconds,
		&task.Status, &task.LastRunID, &task.LastError, &task.CreatedAt, &task.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan scheduled task: %w", err)
	}
	return &task, nil
}
