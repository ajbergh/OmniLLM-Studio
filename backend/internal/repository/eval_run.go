package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// EvalRunRepo handles CRUD operations for evaluation runs.
type EvalRunRepo struct {
	db *sql.DB
}

// NewEvalRunRepo creates a new EvalRunRepo.
func NewEvalRunRepo(db *sql.DB) *EvalRunRepo {
	return &EvalRunRepo{db: db}
}

// Create inserts a new eval run record.
func (r *EvalRunRepo) Create(run *models.EvalRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	run.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(
		"INSERT INTO eval_runs (id, suite_name, provider, model, total_score, results_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		run.ID, run.SuiteName, run.Provider, run.Model, run.TotalScore, run.ResultsJSON, run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create eval run: %w", err)
	}
	return nil
}

// GetByID returns an eval run by ID.
func (r *EvalRunRepo) GetByID(id string) (*models.EvalRun, error) {
	var run models.EvalRun
	err := r.db.QueryRow(
		"SELECT id, suite_name, provider, model, total_score, results_json, created_at FROM eval_runs WHERE id = ?",
		id,
	).Scan(&run.ID, &run.SuiteName, &run.Provider, &run.Model, &run.TotalScore, &run.ResultsJSON, &run.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get eval run %q: %w", id, err)
	}
	return &run, nil
}

// List returns eval runs, optionally filtered by suite name.
func (r *EvalRunRepo) List(suiteName string) ([]models.EvalRun, error) {
	var rows *sql.Rows
	var err error

	if suiteName != "" {
		rows, err = r.db.Query(
			"SELECT id, suite_name, provider, model, total_score, results_json, created_at FROM eval_runs WHERE suite_name = ? ORDER BY created_at DESC",
			suiteName,
		)
	} else {
		rows, err = r.db.Query(
			"SELECT id, suite_name, provider, model, total_score, results_json, created_at FROM eval_runs ORDER BY created_at DESC",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list eval runs: %w", err)
	}
	defer rows.Close()

	runs := make([]models.EvalRun, 0)
	for rows.Next() {
		var run models.EvalRun
		if err := rows.Scan(&run.ID, &run.SuiteName, &run.Provider, &run.Model, &run.TotalScore, &run.ResultsJSON, &run.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan eval run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// Delete removes an eval run record.
func (r *EvalRunRepo) Delete(id string) error {
	res, err := r.db.Exec("DELETE FROM eval_runs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete eval run %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("eval run %q not found", id)
	}
	return nil
}
