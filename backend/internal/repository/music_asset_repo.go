package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type MusicAssetRepo struct {
	db *sql.DB
}

func NewMusicAssetRepo(db *sql.DB) *MusicAssetRepo {
	return &MusicAssetRepo{db: db}
}

func (r *MusicAssetRepo) Create(a *models.MusicAsset) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Kind == "" {
		a.Kind = "music"
	}
	if a.MetadataJSON == "" {
		a.MetadataJSON = "{}"
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO music_assets (
			id, session_id, generation_id, kind, file_name, file_path, mime_type, size_bytes,
			duration_ms, sample_rate_hz, channels, provider, model, metadata_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SessionID, a.GenerationID, a.Kind, a.FileName, a.FilePath, a.MimeType, a.SizeBytes,
		a.DurationMS, a.SampleRateHz, a.Channels, a.Provider, a.Model, a.MetadataJSON, a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create music asset: %w", err)
	}
	return nil
}

func (r *MusicAssetRepo) GetByID(id string) (*models.MusicAsset, error) {
	var a models.MusicAsset
	err := r.db.QueryRow(`
		SELECT id, session_id, generation_id, kind, file_name, file_path, mime_type, size_bytes,
		       duration_ms, sample_rate_hz, channels, provider, model, metadata_json, created_at
		FROM music_assets WHERE id = ?`, id,
	).Scan(
		&a.ID, &a.SessionID, &a.GenerationID, &a.Kind, &a.FileName, &a.FilePath, &a.MimeType, &a.SizeBytes,
		&a.DurationMS, &a.SampleRateHz, &a.Channels, &a.Provider, &a.Model, &a.MetadataJSON, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get music asset: %w", err)
	}
	return &a, nil
}

func (r *MusicAssetRepo) GetByGeneration(generationID string) (*models.MusicAsset, error) {
	var a models.MusicAsset
	err := r.db.QueryRow(`
		SELECT id, session_id, generation_id, kind, file_name, file_path, mime_type, size_bytes,
		       duration_ms, sample_rate_hz, channels, provider, model, metadata_json, created_at
		FROM music_assets WHERE generation_id = ? ORDER BY created_at DESC LIMIT 1`, generationID,
	).Scan(
		&a.ID, &a.SessionID, &a.GenerationID, &a.Kind, &a.FileName, &a.FilePath, &a.MimeType, &a.SizeBytes,
		&a.DurationMS, &a.SampleRateHz, &a.Channels, &a.Provider, &a.Model, &a.MetadataJSON, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get music asset by generation: %w", err)
	}
	return &a, nil
}

func (r *MusicAssetRepo) ListBySession(sessionID string) ([]models.MusicAsset, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, generation_id, kind, file_name, file_path, mime_type, size_bytes,
		       duration_ms, sample_rate_hz, channels, provider, model, metadata_json, created_at
		FROM music_assets WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list music assets: %w", err)
	}
	defer rows.Close()

	var assets []models.MusicAsset
	for rows.Next() {
		var a models.MusicAsset
		if err := rows.Scan(
			&a.ID, &a.SessionID, &a.GenerationID, &a.Kind, &a.FileName, &a.FilePath, &a.MimeType, &a.SizeBytes,
			&a.DurationMS, &a.SampleRateHz, &a.Channels, &a.Provider, &a.Model, &a.MetadataJSON, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan music asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func (r *MusicAssetRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM music_assets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete music asset: %w", err)
	}
	return nil
}
