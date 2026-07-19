package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AgentEventRecord is an append-only event used for audit and SSE replay.
type AgentEventRecord struct {
	ID        int64           `json:"id"`
	RunID     string          `json:"run_id"`
	StepID    string          `json:"step_id,omitempty"`
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

type AgentEventRepo struct{ db *sql.DB }

func NewAgentEventRepo(db *sql.DB) *AgentEventRepo { return &AgentEventRepo{db: db} }

func (r *AgentEventRepo) Append(runID, stepID, eventType string, data json.RawMessage) (int64, error) {
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	result, err := r.db.Exec(`
		INSERT INTO agent_events (run_id, step_id, event_type, data_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, runID, stepID, eventType, string(data), time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("append agent event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read agent event cursor: %w", err)
	}
	return id, nil
}

func (r *AgentEventRepo) ListAfter(runID string, afterID int64, limit int) ([]AgentEventRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	rows, err := r.db.Query(`
		SELECT id, run_id, step_id, event_type, data_json, created_at
		FROM agent_events
		WHERE run_id = ? AND id > ?
		ORDER BY id ASC
		LIMIT ?
	`, runID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent events: %w", err)
	}
	defer rows.Close()
	out := make([]AgentEventRecord, 0)
	for rows.Next() {
		var record AgentEventRecord
		var data string
		if err := rows.Scan(&record.ID, &record.RunID, &record.StepID, &record.EventType, &data, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent event: %w", err)
		}
		record.Data = json.RawMessage(data)
		out = append(out, record)
	}
	return out, rows.Err()
}

func (r *AgentEventRepo) DeleteByRun(runID string) error {
	_, err := r.db.Exec(`DELETE FROM agent_events WHERE run_id = ?`, runID)
	return err
}
