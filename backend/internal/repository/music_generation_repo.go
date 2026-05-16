package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type MusicGenerationRepo struct {
	db *sql.DB
}

func NewMusicGenerationRepo(db *sql.DB) *MusicGenerationRepo {
	return &MusicGenerationRepo{db: db}
}

func (r *MusicGenerationRepo) Create(g *models.MusicGeneration) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}
	if g.Status == "" {
		g.Status = "pending"
	}
	if g.MetadataJSON == "" {
		g.MetadataJSON = "{}"
	}
	_, err := r.db.Exec(`
		INSERT INTO music_generations (
			id, session_id, parent_id, title, status, provider, model, prompt, assembled_prompt,
			lyrics, structure, error, upstream_request_id, usage_json, cost_usd, duration_ms,
			input_chars, output_bytes, metadata_json, created_at, updated_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.SessionID, g.ParentID, g.Title, g.Status, g.Provider, g.Model, g.Prompt, g.AssembledPrompt,
		g.Lyrics, g.Structure, g.Error, g.UpstreamReqID, g.UsageJSON, g.CostUSD, g.DurationMS,
		g.InputChars, g.OutputBytes, g.MetadataJSON, g.CreatedAt, g.UpdatedAt, g.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create music generation: %w", err)
	}
	return nil
}

func (r *MusicGenerationRepo) GetByID(id string) (*models.MusicGeneration, error) {
	row := r.db.QueryRow(`
		SELECT id, session_id, parent_id, title, status, provider, model, prompt, assembled_prompt,
		       lyrics, structure, error, upstream_request_id, usage_json, cost_usd, duration_ms,
		       input_chars, output_bytes, metadata_json, created_at, updated_at, completed_at
		FROM music_generations WHERE id = ?`, id)
	return scanMusicGeneration(row)
}

func (r *MusicGenerationRepo) ListBySession(sessionID string) ([]models.MusicGeneration, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, parent_id, title, status, provider, model, prompt, assembled_prompt,
		       lyrics, structure, error, upstream_request_id, usage_json, cost_usd, duration_ms,
		       input_chars, output_bytes, metadata_json, created_at, updated_at, completed_at
		FROM music_generations
		WHERE session_id = ?
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list music generations: %w", err)
	}
	defer rows.Close()

	var generations []models.MusicGeneration
	for rows.Next() {
		g, err := scanMusicGeneration(rows)
		if err != nil {
			return nil, err
		}
		generations = append(generations, *g)
	}
	return generations, rows.Err()
}

func (r *MusicGenerationRepo) MarkRunning(id string) error {
	return r.updateStatus(id, "running", nil)
}

func (r *MusicGenerationRepo) MarkFailed(id, message string) error {
	return r.updateStatus(id, "failed", &message)
}

func (r *MusicGenerationRepo) MarkCompleted(generationID string, result MusicGenerationCompletion) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE music_generations
		SET status = 'completed',
		    lyrics = ?,
		    structure = ?,
		    error = NULL,
		    upstream_request_id = ?,
		    usage_json = ?,
		    cost_usd = ?,
		    duration_ms = ?,
		    output_bytes = ?,
		    metadata_json = ?,
		    updated_at = ?,
		    completed_at = ?
		WHERE id = ?`,
		result.Lyrics, result.Structure, result.UpstreamReqID, result.UsageJSON, result.CostUSD,
		result.DurationMS, result.OutputBytes, result.MetadataJSON, now, now, generationID,
	)
	if err != nil {
		return fmt.Errorf("complete music generation: %w", err)
	}
	return nil
}

func (r *MusicGenerationRepo) updateStatus(id, status string, message *string) error {
	_, err := r.db.Exec(`
		UPDATE music_generations SET status = ?, error = ?, updated_at = ? WHERE id = ?`,
		status, message, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update music generation status: %w", err)
	}
	return nil
}

func (r *MusicGenerationRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM music_generations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete music generation: %w", err)
	}
	return nil
}

type MusicGenerationCompletion struct {
	Lyrics        string
	Structure     string
	UpstreamReqID *string
	UsageJSON     *string
	CostUSD       *float64
	DurationMS    *int64
	OutputBytes   int64
	MetadataJSON  string
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanMusicGeneration(row rowScanner) (*models.MusicGeneration, error) {
	var g models.MusicGeneration
	var parentID, errMsg, reqID, usageJSON sql.NullString
	var cost sql.NullFloat64
	var duration sql.NullInt64
	var completedAt sql.NullTime
	err := row.Scan(
		&g.ID, &g.SessionID, &parentID, &g.Title, &g.Status, &g.Provider, &g.Model, &g.Prompt, &g.AssembledPrompt,
		&g.Lyrics, &g.Structure, &errMsg, &reqID, &usageJSON, &cost, &duration,
		&g.InputChars, &g.OutputBytes, &g.MetadataJSON, &g.CreatedAt, &g.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan music generation: %w", err)
	}
	if parentID.Valid {
		g.ParentID = &parentID.String
	}
	if errMsg.Valid {
		g.Error = &errMsg.String
	}
	if reqID.Valid {
		g.UpstreamReqID = &reqID.String
	}
	if usageJSON.Valid {
		g.UsageJSON = &usageJSON.String
	}
	if cost.Valid {
		g.CostUSD = &cost.Float64
	}
	if duration.Valid {
		g.DurationMS = &duration.Int64
	}
	if completedAt.Valid {
		g.CompletedAt = &completedAt.Time
	}
	return &g, nil
}
