package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type VideoAssetRepo struct {
	db *sql.DB
}

func NewVideoAssetRepo(db *sql.DB) *VideoAssetRepo {
	return &VideoAssetRepo{db: db}
}

func (r *VideoAssetRepo) Create(a *models.VideoAsset) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.SourceType == "" {
		a.SourceType = "generation"
	}
	if a.Kind == "" {
		a.Kind = "video"
	}
	if a.MetadataJSON == "" {
		a.MetadataJSON = "{}"
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO video_assets (
			id, project_id, source_type, source_studio, source_id, kind, file_name,
			file_path, mime_type, size_bytes, duration_ms, width, height, fps,
			thumbnail_path, waveform_path, provider, model, metadata_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, a.SourceType, a.SourceStudio, a.SourceID, a.Kind, a.FileName,
		a.FilePath, a.MimeType, a.SizeBytes, a.DurationMS, a.Width, a.Height, a.FPS,
		a.ThumbnailPath, a.WaveformPath, a.Provider, a.Model, a.MetadataJSON, a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create video asset: %w", err)
	}
	return nil
}

func (r *VideoAssetRepo) GetByID(id string) (*models.VideoAsset, error) {
	row := r.db.QueryRow(videoAssetSelectSQL+` WHERE id = ?`, id)
	return scanVideoAsset(row)
}

func (r *VideoAssetRepo) GetByGeneration(generationID string) (*models.VideoAsset, error) {
	row := r.db.QueryRow(videoAssetSelectSQL+`
		WHERE id = (
			SELECT output_asset_id FROM video_generations WHERE id = ?
		)`, generationID)
	return scanVideoAsset(row)
}

func (r *VideoAssetRepo) ListByProject(projectID string) ([]models.VideoAsset, error) {
	rows, err := r.db.Query(videoAssetSelectSQL+` WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list video assets: %w", err)
	}
	defer rows.Close()

	assets := make([]models.VideoAsset, 0)
	for rows.Next() {
		asset, err := scanVideoAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, *asset)
	}
	return assets, rows.Err()
}

// UpdateFileName renames the display file name of an asset (the file on disk
// keeps its storage name).
func (r *VideoAssetRepo) UpdateFileName(id, fileName string) error {
	if _, err := r.db.Exec(`UPDATE video_assets SET file_name = ? WHERE id = ?`, fileName, id); err != nil {
		return fmt.Errorf("rename video asset: %w", err)
	}
	return nil
}

// UpdateArtifacts records generated thumbnail/waveform paths (lazy backfill
// for assets ingested before artifact generation existed).
func (r *VideoAssetRepo) UpdateArtifacts(id string, thumbnailPath, waveformPath *string) error {
	if _, err := r.db.Exec(`UPDATE video_assets SET thumbnail_path = ?, waveform_path = ? WHERE id = ?`, thumbnailPath, waveformPath, id); err != nil {
		return fmt.Errorf("update video asset artifacts: %w", err)
	}
	return nil
}

func (r *VideoAssetRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM video_assets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete video asset: %w", err)
	}
	return nil
}

const videoAssetSelectSQL = `
	SELECT id, project_id, source_type, source_studio, source_id, kind, file_name,
	       file_path, mime_type, size_bytes, duration_ms, width, height, fps,
	       thumbnail_path, waveform_path, provider, model, metadata_json, created_at
	FROM video_assets`

func scanVideoAsset(row rowScanner) (*models.VideoAsset, error) {
	var a models.VideoAsset
	var projectID, sourceStudio, sourceID, thumbnailPath, waveformPath, provider, model sql.NullString
	var duration sql.NullInt64
	var width, height sql.NullInt64
	var fps sql.NullFloat64
	err := row.Scan(
		&a.ID, &projectID, &a.SourceType, &sourceStudio, &sourceID, &a.Kind, &a.FileName,
		&a.FilePath, &a.MimeType, &a.SizeBytes, &duration, &width, &height, &fps,
		&thumbnailPath, &waveformPath, &provider, &model, &a.MetadataJSON, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video asset: %w", err)
	}
	if projectID.Valid {
		a.ProjectID = &projectID.String
	}
	if sourceStudio.Valid {
		a.SourceStudio = &sourceStudio.String
	}
	if sourceID.Valid {
		a.SourceID = &sourceID.String
	}
	if duration.Valid {
		a.DurationMS = &duration.Int64
	}
	if width.Valid {
		v := int(width.Int64)
		a.Width = &v
	}
	if height.Valid {
		v := int(height.Int64)
		a.Height = &v
	}
	if fps.Valid {
		a.FPS = &fps.Float64
	}
	if thumbnailPath.Valid {
		a.ThumbnailPath = &thumbnailPath.String
	}
	if waveformPath.Valid {
		a.WaveformPath = &waveformPath.String
	}
	if provider.Valid {
		a.Provider = &provider.String
	}
	if model.Valid {
		a.Model = &model.String
	}
	return &a, nil
}
