package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// AgentStepRepo handles agent step persistence.
type AgentStepRepo struct {
	db *sql.DB
}

// NewAgentStepRepo creates a new AgentStepRepo.
func NewAgentStepRepo(db *sql.DB) *AgentStepRepo {
	return &AgentStepRepo{db: db}
}

// Create inserts a new agent step.
func (r *AgentStepRepo) Create(runID string, stepIndex int, stepType, description string) (*models.AgentStep, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.db.Exec(`
		INSERT INTO agent_steps (id, run_id, step_index, type, description, status, input_json, output_json, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', '{}', '{}', ?)
	`, id, runID, stepIndex, stepType, description, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent step: %w", err)
	}

	return r.GetByID(id)
}

// CreateBatch inserts multiple steps in a single transaction.
func (r *AgentStepRepo) CreateBatch(steps []models.AgentStep) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO agent_steps (id, run_id, step_index, type, description, status, input_json, output_json, tool_name, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, s := range steps {
		id := s.ID
		if id == "" {
			id = uuid.New().String()
		}
		input := s.InputJSON
		if input == "" {
			input = "{}"
		}
		output := s.OutputJSON
		if output == "" {
			output = "{}"
		}
		_, err := stmt.Exec(id, s.RunID, s.StepIndex, s.Type, s.Description, input, output, s.ToolName, now)
		if err != nil {
			return fmt.Errorf("insert step %d: %w", s.StepIndex, err)
		}
	}

	return tx.Commit()
}

// GetByID retrieves an agent step by ID.
func (r *AgentStepRepo) GetByID(id string) (*models.AgentStep, error) {
	row := r.db.QueryRow(`
		SELECT id, run_id, step_index, type, description, status, input_json, output_json,
			tool_name, message_id, duration_ms, created_at, completed_at
		FROM agent_steps WHERE id = ?
	`, id)
	return scanStep(row)
}

// ListByRun returns all steps for an agent run.
func (r *AgentStepRepo) ListByRun(runID string) ([]models.AgentStep, error) {
	rows, err := r.db.Query(`
		SELECT id, run_id, step_index, type, description, status, input_json, output_json,
			tool_name, message_id, duration_ms, created_at, completed_at
		FROM agent_steps WHERE run_id = ? ORDER BY step_index ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list agent steps: %w", err)
	}
	defer rows.Close()

	var steps []models.AgentStep
	for rows.Next() {
		var s models.AgentStep
		if err := rows.Scan(
			&s.ID, &s.RunID, &s.StepIndex, &s.Type, &s.Description, &s.Status,
			&s.InputJSON, &s.OutputJSON, &s.ToolName, &s.MessageID, &s.DurationMs,
			&s.CreatedAt, &s.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent step: %w", err)
		}
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

// UpdateStatus updates the status of a step.
func (r *AgentStepRepo) UpdateStatus(id, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" || status == "skipped" {
		now := time.Now().UTC()
		completedAt = &now
	}
	_, err := r.db.Exec(`
		UPDATE agent_steps SET status = ?, completed_at = ? WHERE id = ?
	`, status, completedAt, id)
	return err
}

// UpdateOutput sets the output JSON and duration for a step.
func (r *AgentStepRepo) UpdateOutput(id, outputJSON string, durationMs int) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE agent_steps SET output_json = ?, duration_ms = ?, status = 'completed', completed_at = ? WHERE id = ?
	`, outputJSON, durationMs, now, id)
	return err
}

// UpdateMessageID links a step to a message.
func (r *AgentStepRepo) UpdateMessageID(stepID, messageID string) error {
	_, err := r.db.Exec(`UPDATE agent_steps SET message_id = ? WHERE id = ?`, messageID, stepID)
	return err
}

// scanStep scans a single step from a Row.
func scanStep(row *sql.Row) (*models.AgentStep, error) {
	var s models.AgentStep
	if err := row.Scan(
		&s.ID, &s.RunID, &s.StepIndex, &s.Type, &s.Description, &s.Status,
		&s.InputJSON, &s.OutputJSON, &s.ToolName, &s.MessageID, &s.DurationMs,
		&s.CreatedAt, &s.CompletedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan step: %w", err)
	}
	return &s, nil
}
