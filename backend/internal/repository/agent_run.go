package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// AgentRunRepo handles agent run persistence.
type AgentRunRepo struct {
	db *sql.DB
}

// NewAgentRunRepo creates a new AgentRunRepo.
func NewAgentRunRepo(db *sql.DB) *AgentRunRepo {
	return &AgentRunRepo{db: db}
}

// Create inserts a new agent run.
func (r *AgentRunRepo) Create(conversationID, goal string) (*models.AgentRun, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.db.Exec(`
		INSERT INTO agent_runs (id, conversation_id, status, goal, plan_json, result_summary, created_at, updated_at)
		VALUES (?, ?, 'planning', ?, '[]', '', ?, ?)
	`, id, conversationID, goal, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent run: %w", err)
	}

	return r.GetByID(id)
}

// GetByID retrieves an agent run by ID.
func (r *AgentRunRepo) GetByID(id string) (*models.AgentRun, error) {
	row := r.db.QueryRow(`
		SELECT id, conversation_id, status, goal, plan_json, result_summary, created_at, updated_at, completed_at
		FROM agent_runs WHERE id = ?
	`, id)

	var run models.AgentRun
	if err := row.Scan(
		&run.ID, &run.ConversationID, &run.Status, &run.Goal, &run.PlanJSON,
		&run.ResultSummary, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent run: %w", err)
	}
	return &run, nil
}

// ListByConversation returns all agent runs for a conversation.
func (r *AgentRunRepo) ListByConversation(conversationID string) ([]models.AgentRun, error) {
	rows, err := r.db.Query(`
		SELECT id, conversation_id, status, goal, plan_json, result_summary, created_at, updated_at, completed_at
		FROM agent_runs WHERE conversation_id = ? ORDER BY created_at DESC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	defer rows.Close()

	var runs []models.AgentRun
	for rows.Next() {
		var run models.AgentRun
		if err := rows.Scan(
			&run.ID, &run.ConversationID, &run.Status, &run.Goal, &run.PlanJSON,
			&run.ResultSummary, &run.CreatedAt, &run.UpdatedAt, &run.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// UpdateStatus updates the status of an agent run.
func (r *AgentRunRepo) UpdateStatus(id, status string) error {
	now := time.Now().UTC()
	var completedAt *time.Time
	if status == "completed" || status == "failed" || status == "cancelled" {
		completedAt = &now
	}
	_, err := r.db.Exec(`
		UPDATE agent_runs SET status = ?, updated_at = ?, completed_at = ? WHERE id = ?
	`, status, now, completedAt, id)
	if err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	return nil
}

// UpdatePlan updates the plan JSON.
func (r *AgentRunRepo) UpdatePlan(id, planJSON string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE agent_runs SET plan_json = ?, updated_at = ? WHERE id = ?
	`, planJSON, now, id)
	if err != nil {
		return fmt.Errorf("update agent run plan: %w", err)
	}
	return nil
}

// UpdateResult sets the result summary.
func (r *AgentRunRepo) UpdateResult(id, resultSummary string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE agent_runs SET result_summary = ?, updated_at = ? WHERE id = ?
	`, resultSummary, now, id)
	if err != nil {
		return fmt.Errorf("update agent run result: %w", err)
	}
	return nil
}
