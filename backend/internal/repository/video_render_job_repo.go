package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type VideoRenderJobRepo struct {
	db *sql.DB
}

func NewVideoRenderJobRepo(db *sql.DB) *VideoRenderJobRepo {
	return &VideoRenderJobRepo{db: db}
}

func (r *VideoRenderJobRepo) Create(j *models.VideoRenderJob) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	if j.Status == "" {
		j.Status = "queued"
	}
	if j.SettingsJSON == "" {
		j.SettingsJSON = "{}"
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO video_render_jobs (
			id, project_id, timeline_id, status, progress, settings_json,
			output_asset_id, error, created_at, started_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.ProjectID, j.TimelineID, j.Status, j.Progress, j.SettingsJSON,
		j.OutputAssetID, j.Error, j.CreatedAt, j.StartedAt, j.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create video render job: %w", err)
	}
	return nil
}

func (r *VideoRenderJobRepo) GetByID(id string) (*models.VideoRenderJob, error) {
	row := r.db.QueryRow(videoRenderJobSelectSQL+` WHERE id = ?`, id)
	return scanVideoRenderJob(row)
}

func (r *VideoRenderJobRepo) ListByProject(projectID string) ([]models.VideoRenderJob, error) {
	rows, err := r.db.Query(videoRenderJobSelectSQL+` WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list video render jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.VideoRenderJob
	for rows.Next() {
		job, err := scanVideoRenderJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (r *VideoRenderJobRepo) MarkRunning(id string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE video_render_jobs
		SET status = 'running', progress = CASE WHEN progress < 0.05 THEN 0.05 ELSE progress END, started_at = COALESCE(started_at, ?), error = NULL
		WHERE id = ? AND status <> 'cancelled'`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("start video render job: %w", err)
	}
	return nil
}

func (r *VideoRenderJobRepo) UpdateProgress(id string, progress float64) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	_, err := r.db.Exec(`UPDATE video_render_jobs SET progress = ? WHERE id = ? AND status IN ('queued','running')`, progress, id)
	if err != nil {
		return fmt.Errorf("update video render progress: %w", err)
	}
	return nil
}

func (r *VideoRenderJobRepo) MarkCompleted(id, outputAssetID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE video_render_jobs
		SET status = 'completed', progress = 1, output_asset_id = ?, error = NULL, completed_at = ?
		WHERE id = ? AND status IN ('queued','running')`,
		outputAssetID, now, id,
	)
	if err != nil {
		return fmt.Errorf("complete video render job: %w", err)
	}
	return nil
}

func (r *VideoRenderJobRepo) MarkFailed(id, message string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE video_render_jobs
		SET status = 'failed', error = ?, completed_at = ?
		WHERE id = ? AND status <> 'cancelled'`,
		message, now, id,
	)
	if err != nil {
		return fmt.Errorf("fail video render job: %w", err)
	}
	return nil
}

func (r *VideoRenderJobRepo) MarkCancelled(id string) error {
	message := "cancelled"
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE video_render_jobs
		SET status = 'cancelled', error = ?, completed_at = ?
		WHERE id = ? AND status IN ('queued','running')`,
		message, now, id,
	)
	if err != nil {
		return fmt.Errorf("cancel video render job: %w", err)
	}
	return nil
}

const videoRenderJobSelectSQL = `
	SELECT id, project_id, timeline_id, status, progress, settings_json,
	       output_asset_id, error, created_at, started_at, completed_at
	FROM video_render_jobs`

func scanVideoRenderJob(row rowScanner) (*models.VideoRenderJob, error) {
	var j models.VideoRenderJob
	var outputAssetID, errMsg sql.NullString
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.ProjectID, &j.TimelineID, &j.Status, &j.Progress, &j.SettingsJSON,
		&outputAssetID, &errMsg, &j.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video render job: %w", err)
	}
	if outputAssetID.Valid {
		j.OutputAssetID = &outputAssetID.String
	}
	if errMsg.Valid {
		j.Error = &errMsg.String
	}
	if startedAt.Valid {
		j.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}
