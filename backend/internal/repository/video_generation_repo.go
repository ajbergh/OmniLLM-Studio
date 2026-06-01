package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type VideoGenerationRepo struct {
	db *sql.DB
}

func NewVideoGenerationRepo(db *sql.DB) *VideoGenerationRepo {
	return &VideoGenerationRepo{db: db}
}

func (r *VideoGenerationRepo) Create(g *models.VideoGeneration) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	if g.Status == "" {
		g.Status = "pending"
	}
	if g.SettingsJSON == "" {
		g.SettingsJSON = "{}"
	}
	if g.InputAssetIDsJSON == "" {
		g.InputAssetIDsJSON = "[]"
	}
	if g.InputAssetsJSON == "" {
		g.InputAssetsJSON = "[]"
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO video_generations (
			id, project_id, parent_id, status, provider, model, prompt, enhanced_prompt,
			negative_prompt, settings_json, input_asset_ids_json, input_assets_json, output_asset_id,
			upstream_job_id, upstream_request_id, usage_json, cost_usd, error, created_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.ProjectID, g.ParentID, g.Status, g.Provider, g.Model, g.Prompt, g.EnhancedPrompt,
		g.NegativePrompt, g.SettingsJSON, g.InputAssetIDsJSON, g.InputAssetsJSON, g.OutputAssetID,
		g.UpstreamJobID, g.UpstreamReqID, g.UsageJSON, g.CostUSD, g.Error, g.CreatedAt, g.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create video generation: %w", err)
	}
	return nil
}

func (r *VideoGenerationRepo) GetByID(id string) (*models.VideoGeneration, error) {
	row := r.db.QueryRow(videoGenerationSelectSQL+` WHERE id = ?`, id)
	return scanVideoGeneration(row)
}

func (r *VideoGenerationRepo) ListByProject(projectID string) ([]models.VideoGeneration, error) {
	rows, err := r.db.Query(videoGenerationSelectSQL+` WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list video generations: %w", err)
	}
	defer rows.Close()

	generations := make([]models.VideoGeneration, 0)
	for rows.Next() {
		generation, err := scanVideoGeneration(rows)
		if err != nil {
			return nil, err
		}
		generations = append(generations, *generation)
	}
	return generations, rows.Err()
}

func (r *VideoGenerationRepo) MarkRunning(id string) error {
	return r.updateStatus(id, "running", nil, false)
}

func (r *VideoGenerationRepo) MarkFailed(id, message string) error {
	return r.updateStatus(id, "failed", &message, true)
}

func (r *VideoGenerationRepo) MarkCancelled(id string) error {
	message := "cancelled"
	return r.updateStatus(id, "cancelled", &message, true)
}

func (r *VideoGenerationRepo) MarkCompleted(generationID string, result VideoGenerationCompletion) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE video_generations
		SET status = 'completed',
		    error = NULL,
		    output_asset_id = ?,
		    upstream_job_id = ?,
		    upstream_request_id = ?,
		    usage_json = ?,
		    cost_usd = ?,
		    completed_at = ?
		WHERE id = ?`,
		result.OutputAssetID, result.UpstreamJobID, result.UpstreamReqID,
		result.UsageJSON, result.CostUSD, now, generationID,
	)
	if err != nil {
		return fmt.Errorf("complete video generation: %w", err)
	}
	return nil
}

func (r *VideoGenerationRepo) updateStatus(id, status string, message *string, completed bool) error {
	var completedAt interface{}
	if completed {
		completedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		UPDATE video_generations SET status = ?, error = ?, completed_at = COALESCE(?, completed_at) WHERE id = ?`,
		status, message, completedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update video generation status: %w", err)
	}
	return nil
}

func (r *VideoGenerationRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM video_generations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete video generation: %w", err)
	}
	return nil
}

// SetUpstreamJobID stores the provider operation name on a pending generation so
// the async poll goroutine can be recovered after a restart.
func (r *VideoGenerationRepo) SetUpstreamJobID(id, jobID string) error {
	_, err := r.db.Exec(`UPDATE video_generations SET upstream_job_id = ? WHERE id = ?`, jobID, id)
	if err != nil {
		return fmt.Errorf("set upstream_job_id: %w", err)
	}
	return nil
}

// ListActiveWithUpstreamJob returns all running/pending generations that have an
// upstream_job_id set.  Used at startup to resume background poll goroutines.
func (r *VideoGenerationRepo) ListActiveWithUpstreamJob() ([]models.VideoGeneration, error) {
	rows, err := r.db.Query(
		videoGenerationSelectSQL + ` WHERE status IN ('pending','running') AND upstream_job_id IS NOT NULL AND upstream_job_id != ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active video generations: %w", err)
	}
	defer rows.Close()
	var result []models.VideoGeneration
	for rows.Next() {
		g, err := scanVideoGeneration(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *g)
	}
	return result, rows.Err()
}

type VideoGenerationCompletion struct {
	OutputAssetID string
	UpstreamJobID *string
	UpstreamReqID *string
	UsageJSON     *string
	CostUSD       *float64
}

const videoGenerationSelectSQL = `
	SELECT id, project_id, parent_id, status, provider, model, prompt, enhanced_prompt,
	       negative_prompt, settings_json, input_asset_ids_json, input_assets_json, output_asset_id,
	       upstream_job_id, upstream_request_id, usage_json, cost_usd, error, created_at, completed_at
	FROM video_generations`

func scanVideoGeneration(row rowScanner) (*models.VideoGeneration, error) {
	var g models.VideoGeneration
	var parentID, enhancedPrompt, negativePrompt, outputAssetID, jobID, reqID, usageJSON, errMsg sql.NullString
	var cost sql.NullFloat64
	var completedAt sql.NullTime
	err := row.Scan(
		&g.ID, &g.ProjectID, &parentID, &g.Status, &g.Provider, &g.Model, &g.Prompt, &enhancedPrompt,
		&negativePrompt, &g.SettingsJSON, &g.InputAssetIDsJSON, &g.InputAssetsJSON, &outputAssetID,
		&jobID, &reqID, &usageJSON, &cost, &errMsg, &g.CreatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video generation: %w", err)
	}
	if parentID.Valid {
		g.ParentID = &parentID.String
	}
	if enhancedPrompt.Valid {
		g.EnhancedPrompt = &enhancedPrompt.String
	}
	if negativePrompt.Valid {
		g.NegativePrompt = &negativePrompt.String
	}
	if outputAssetID.Valid {
		g.OutputAssetID = &outputAssetID.String
	}
	if jobID.Valid {
		g.UpstreamJobID = &jobID.String
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
	if errMsg.Valid {
		g.Error = &errMsg.String
	}
	if completedAt.Valid {
		g.CompletedAt = &completedAt.Time
	}
	return &g, nil
}
