package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// TemplateRepo handles prompt template CRUD.
type TemplateRepo struct {
	db *sql.DB
}

// NewTemplateRepo creates a new TemplateRepo.
func NewTemplateRepo(db *sql.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

// List returns all prompt templates optionally filtered by category.
func (r *TemplateRepo) List(category string) ([]models.PromptTemplate, error) {
	query := `SELECT id, name, description, category, template_body, variables,
		is_system, sort_order, created_at, updated_at
		FROM prompt_templates`
	var args []interface{}

	if category != "" {
		query += " WHERE category = ?"
		args = append(args, category)
	}
	query += " ORDER BY sort_order ASC, name ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []models.PromptTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// GetByID retrieves a prompt template by ID.
func (r *TemplateRepo) GetByID(id string) (*models.PromptTemplate, error) {
	row := r.db.QueryRow(`SELECT id, name, description, category, template_body, variables,
		is_system, sort_order, created_at, updated_at
		FROM prompt_templates WHERE id = ?`, id)

	var t models.PromptTemplate
	var varsJSON string
	var isSystem int
	if err := row.Scan(
		&t.ID, &t.Name, &t.Description, &t.Category, &t.TemplateBody, &varsJSON,
		&isSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get template: %w", err)
	}
	t.IsSystem = isSystem != 0
	if err := json.Unmarshal([]byte(varsJSON), &t.Variables); err != nil {
		t.Variables = []models.TemplateVariable{}
	}
	return &t, nil
}

// CreateTemplateInput holds data for creating a new template.
type CreateTemplateInput struct {
	Name         string                    `json:"name"`
	Description  string                    `json:"description"`
	Category     string                    `json:"category"`
	TemplateBody string                    `json:"template_body"`
	Variables    []models.TemplateVariable `json:"variables"`
	IsSystem     bool                      `json:"is_system"`
	SortOrder    int                       `json:"sort_order"`
}

// Create inserts a new prompt template.
func (r *TemplateRepo) Create(input CreateTemplateInput) (*models.PromptTemplate, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	varsJSON, err := json.Marshal(input.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	isSystem := 0
	if input.IsSystem {
		isSystem = 1
	}

	_, err = r.db.Exec(`
		INSERT INTO prompt_templates (id, name, description, category, template_body, variables, is_system, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, input.Name, input.Description, input.Category, input.TemplateBody, string(varsJSON), isSystem, input.SortOrder, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}

	return r.GetByID(id)
}

// UpdateTemplateInput holds fields for updating a template.
type UpdateTemplateInput struct {
	Name         *string                    `json:"name,omitempty"`
	Description  *string                    `json:"description,omitempty"`
	Category     *string                    `json:"category,omitempty"`
	TemplateBody *string                    `json:"template_body,omitempty"`
	Variables    *[]models.TemplateVariable `json:"variables,omitempty"`
	SortOrder    *int                       `json:"sort_order,omitempty"`
}

// Update modifies an existing prompt template (only user-created templates).
func (r *TemplateRepo) Update(id string, input UpdateTemplateInput) (*models.PromptTemplate, error) {
	existing, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.Category != nil {
		existing.Category = *input.Category
	}
	if input.TemplateBody != nil {
		existing.TemplateBody = *input.TemplateBody
	}
	if input.Variables != nil {
		existing.Variables = *input.Variables
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}
	existing.UpdatedAt = time.Now().UTC()

	varsJSON, err := json.Marshal(existing.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal variables: %w", err)
	}

	_, err = r.db.Exec(`
		UPDATE prompt_templates SET name = ?, description = ?, category = ?, template_body = ?,
			variables = ?, sort_order = ?, updated_at = ?
		WHERE id = ?
	`, existing.Name, existing.Description, existing.Category, existing.TemplateBody,
		string(varsJSON), existing.SortOrder, existing.UpdatedAt, id)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}

	return existing, nil
}

// Delete removes a prompt template. Only user-created templates can be deleted
// (is_system = 0). Returns true if a row was removed.
func (r *TemplateRepo) Delete(id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM prompt_templates WHERE id = ? AND is_system = 0", id)
	if err != nil {
		return false, fmt.Errorf("delete template: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Count returns the total number of templates.
func (r *TemplateRepo) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM prompt_templates").Scan(&count)
	return count, err
}

// scanTemplate scans a single template row from the given Rows.
func scanTemplate(rows *sql.Rows) (models.PromptTemplate, error) {
	var t models.PromptTemplate
	var varsJSON string
	var isSystem int
	if err := rows.Scan(
		&t.ID, &t.Name, &t.Description, &t.Category, &t.TemplateBody, &varsJSON,
		&isSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return t, fmt.Errorf("scan template: %w", err)
	}
	t.IsSystem = isSystem != 0
	if err := json.Unmarshal([]byte(varsJSON), &t.Variables); err != nil {
		t.Variables = []models.TemplateVariable{}
	}
	return t, nil
}
