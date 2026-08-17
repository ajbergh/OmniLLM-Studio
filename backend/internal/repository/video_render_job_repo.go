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
	if j.MetadataJSON == "" {
		j.MetadataJSON = "{}"
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO video_render_jobs (
			id, project_id, timeline_id, snapshot_id, status, progress, settings_json,
			output_asset_id, error, metadata_json, created_at, started_at, completed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.ProjectID, j.TimelineID, j.SnapshotID, j.Status, j.Progress, j.SettingsJSON,
		j.OutputAssetID, j.Error, j.MetadataJSON, j.CreatedAt, j.StartedAt, j.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create video render job: %w", err)
	}
	return nil
}

// CreateWithSnapshot atomically persists an immutable render snapshot, its
// asset lease rows, and the queued job that references it.
func (r *VideoRenderJobRepo) CreateWithSnapshot(j *models.VideoRenderJob, snapshot *models.VideoRenderSnapshot, assets []models.VideoRenderSnapshotAsset) error {
	if j == nil || snapshot == nil {
		return fmt.Errorf("render job and snapshot are required")
	}
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	if snapshot.ID == "" {
		snapshot.ID = uuid.New().String()
	}
	if j.Status == "" {
		j.Status = "queued"
	}
	if j.SettingsJSON == "" {
		j.SettingsJSON = "{}"
	}
	if j.MetadataJSON == "" {
		j.MetadataJSON = "{}"
	}
	now := time.Now().UTC()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = now
	}
	j.SnapshotID = &snapshot.ID
	j.TimelineRevision = snapshot.TimelineRevision
	j.TimelineSHA256 = snapshot.TimelineSHA256
	j.AssetManifestSHA256 = snapshot.AssetManifestSHA256
	j.Renderer = snapshot.Renderer
	j.RendererVersion = snapshot.RendererVersion
	j.RenderContractVersion = snapshot.RenderContractVersion
	j.RenderSourceMode = "immutable_snapshot"
	j.ExactSourceAvailable = true

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin create video render snapshot: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		INSERT INTO video_render_snapshots (
			id, project_id, timeline_id, timeline_revision, timeline_json, timeline_sha256,
			asset_manifest_json, asset_manifest_sha256, settings_json,
			render_contract_version, renderer, renderer_version, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.ProjectID, snapshot.TimelineID, snapshot.TimelineRevision,
		snapshot.TimelineJSON, snapshot.TimelineSHA256, snapshot.AssetManifestJSON,
		snapshot.AssetManifestSHA256, snapshot.SettingsJSON, snapshot.RenderContractVersion,
		snapshot.Renderer, snapshot.RendererVersion, snapshot.CreatedAt,
	); err != nil {
		return fmt.Errorf("create video render snapshot: %w", err)
	}
	for _, asset := range assets {
		if _, err := tx.Exec(`
			INSERT INTO video_render_snapshot_assets (snapshot_id, asset_id, file_sha256, size_bytes)
			VALUES (?, ?, ?, ?)`, snapshot.ID, asset.AssetID, asset.FileSHA256, asset.SizeBytes); err != nil {
			return fmt.Errorf("create video render snapshot asset: %w", err)
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO video_render_jobs (
			id, project_id, timeline_id, snapshot_id, status, progress, settings_json,
			output_asset_id, error, metadata_json, created_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.ProjectID, j.TimelineID, j.SnapshotID, j.Status, j.Progress, j.SettingsJSON,
		j.OutputAssetID, j.Error, j.MetadataJSON, j.CreatedAt, j.StartedAt, j.CompletedAt,
	); err != nil {
		return fmt.Errorf("create video render job: %w", err)
	}
	return tx.Commit()
}

func (r *VideoRenderJobRepo) GetByID(id string) (*models.VideoRenderJob, error) {
	row := r.db.QueryRow(videoRenderJobSelectSQL+` WHERE j.id = ?`, id)
	return scanVideoRenderJob(row)
}

func (r *VideoRenderJobRepo) GetSnapshot(id string) (*models.VideoRenderSnapshot, error) {
	row := r.db.QueryRow(`
		SELECT id, project_id, timeline_id, timeline_revision, timeline_json, timeline_sha256,
		       asset_manifest_json, asset_manifest_sha256, settings_json,
		       render_contract_version, renderer, renderer_version, created_at
		FROM video_render_snapshots WHERE id = ?`, id)
	var snapshot models.VideoRenderSnapshot
	if err := row.Scan(
		&snapshot.ID, &snapshot.ProjectID, &snapshot.TimelineID, &snapshot.TimelineRevision,
		&snapshot.TimelineJSON, &snapshot.TimelineSHA256, &snapshot.AssetManifestJSON,
		&snapshot.AssetManifestSHA256, &snapshot.SettingsJSON, &snapshot.RenderContractVersion,
		&snapshot.Renderer, &snapshot.RendererVersion, &snapshot.CreatedAt,
	); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("get video render snapshot: %w", err)
	}
	return &snapshot, nil
}

// HasActiveAssetReference reports whether queued/running render snapshots are
// leasing an asset's bytes. Deletion must check this before removing the file.
func (r *VideoRenderJobRepo) HasActiveAssetReference(assetID string) (bool, error) {
	var exists int
	err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM video_render_snapshot_assets sa
			JOIN video_render_jobs j ON j.snapshot_id = sa.snapshot_id
			WHERE sa.asset_id = ? AND j.status IN ('queued','running')
		)`, assetID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check active video render asset reference: %w", err)
	}
	return exists == 1, nil
}

// Delete removes a render job record. Output assets are independent rows and
// survive the job's deletion.
func (r *VideoRenderJobRepo) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete video render job: %w", err)
	}
	defer tx.Rollback()
	var snapshotID sql.NullString
	if err := tx.QueryRow(`SELECT snapshot_id FROM video_render_jobs WHERE id = ?`, id).Scan(&snapshotID); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("query video render snapshot for delete: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM video_render_jobs WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete video render job: %w", err)
	}
	if snapshotID.Valid {
		if _, err := tx.Exec(`DELETE FROM video_render_snapshots WHERE id = ?`, snapshotID.String); err != nil {
			return fmt.Errorf("delete video render snapshot: %w", err)
		}
	}
	return tx.Commit()
}

func (r *VideoRenderJobRepo) ListByProject(projectID string) ([]models.VideoRenderJob, error) {
	rows, err := r.db.Query(videoRenderJobSelectSQL+` WHERE j.project_id = ? ORDER BY j.created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list video render jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]models.VideoRenderJob, 0)
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

// SetMetadata stores render diagnostics (FFmpeg command, stderr, probe info).
func (r *VideoRenderJobRepo) SetMetadata(id, metadataJSON string) error {
	if metadataJSON == "" {
		metadataJSON = "{}"
	}
	if _, err := r.db.Exec(`UPDATE video_render_jobs SET metadata_json = ? WHERE id = ?`, metadataJSON, id); err != nil {
		return fmt.Errorf("set video render job metadata: %w", err)
	}
	return nil
}

// ListActive returns jobs still queued or running (used for restart recovery).
func (r *VideoRenderJobRepo) ListActive() ([]models.VideoRenderJob, error) {
	rows, err := r.db.Query(videoRenderJobSelectSQL + ` WHERE j.status IN ('queued','running') ORDER BY j.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list active video render jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]models.VideoRenderJob, 0)
	for rows.Next() {
		job, err := scanVideoRenderJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

const videoRenderJobSelectSQL = `
	SELECT j.id, j.project_id, j.timeline_id, j.snapshot_id,
	       s.timeline_revision, s.timeline_sha256, s.asset_manifest_sha256,
	       s.renderer, s.renderer_version, s.render_contract_version,
	       j.status, j.progress, j.settings_json,
	       j.output_asset_id, j.error, j.metadata_json, j.created_at, j.started_at, j.completed_at
	FROM video_render_jobs j
	LEFT JOIN video_render_snapshots s ON s.id = j.snapshot_id`

func scanVideoRenderJob(row rowScanner) (*models.VideoRenderJob, error) {
	var j models.VideoRenderJob
	var snapshotID, timelineSHA256, assetManifestSHA256, renderer, rendererVersion sql.NullString
	var timelineRevision sql.NullInt64
	var renderContractVersion sql.NullInt64
	var outputAssetID, errMsg sql.NullString
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.ProjectID, &j.TimelineID, &snapshotID,
		&timelineRevision, &timelineSHA256, &assetManifestSHA256,
		&renderer, &rendererVersion, &renderContractVersion,
		&j.Status, &j.Progress, &j.SettingsJSON,
		&outputAssetID, &errMsg, &j.MetadataJSON, &j.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video render job: %w", err)
	}
	if snapshotID.Valid {
		j.SnapshotID = &snapshotID.String
		j.RenderSourceMode = "immutable_snapshot"
		j.ExactSourceAvailable = true
	} else {
		j.RenderSourceMode = "legacy_mutable_source"
		j.ExactSourceAvailable = false
	}
	if timelineRevision.Valid {
		j.TimelineRevision = timelineRevision.Int64
	}
	if timelineSHA256.Valid {
		j.TimelineSHA256 = timelineSHA256.String
	}
	if assetManifestSHA256.Valid {
		j.AssetManifestSHA256 = assetManifestSHA256.String
	}
	if renderer.Valid {
		j.Renderer = renderer.String
	}
	if rendererVersion.Valid {
		j.RendererVersion = rendererVersion.String
	}
	if renderContractVersion.Valid {
		j.RenderContractVersion = int(renderContractVersion.Int64)
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
