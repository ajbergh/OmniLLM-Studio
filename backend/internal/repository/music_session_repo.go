package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type MusicSessionRepo struct {
	db *sql.DB
}

func NewMusicSessionRepo(db *sql.DB) *MusicSessionRepo {
	return &MusicSessionRepo{db: db}
}

func (r *MusicSessionRepo) Create(userID, title, provider, model string) (*models.MusicSession, error) {
	now := time.Now().UTC()
	s := &models.MusicSession{
		ID:              uuid.New().String(),
		Title:           title,
		DefaultProvider: provider,
		DefaultModel:    model,
		MetadataJSON:    "{}",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	var uid *string
	if userID != "" {
		uid = &userID
		s.UserID = uid
	}
	_, err := r.db.Exec(`
		INSERT INTO music_sessions (id, user_id, title, default_provider, default_model, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, uid, s.Title, s.DefaultProvider, s.DefaultModel, s.MetadataJSON, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create music session: %w", err)
	}
	return s, nil
}

func (r *MusicSessionRepo) GetByID(id string) (*models.MusicSession, error) {
	var s models.MusicSession
	var userID, activeGenerationID sql.NullString
	err := r.db.QueryRow(`
		SELECT id, user_id, title, active_generation_id, default_provider, default_model,
		       metadata_json, created_at, updated_at
		FROM music_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &userID, &s.Title, &activeGenerationID, &s.DefaultProvider, &s.DefaultModel, &s.MetadataJSON, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get music session: %w", err)
	}
	if userID.Valid {
		s.UserID = &userID.String
	}
	if activeGenerationID.Valid {
		s.ActiveGenerationID = &activeGenerationID.String
	}
	return &s, nil
}

func (r *MusicSessionRepo) ListForUser(userID string) ([]models.MusicSession, error) {
	query := `
		SELECT id, user_id, title, active_generation_id, default_provider, default_model,
		       metadata_json, created_at, updated_at
		FROM music_sessions`
	args := []interface{}{}
	if userID != "" {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list music sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.MusicSession
	for rows.Next() {
		var s models.MusicSession
		var uid, activeGenerationID sql.NullString
		if err := rows.Scan(&s.ID, &uid, &s.Title, &activeGenerationID, &s.DefaultProvider, &s.DefaultModel, &s.MetadataJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan music session: %w", err)
		}
		if uid.Valid {
			s.UserID = &uid.String
		}
		if activeGenerationID.Valid {
			s.ActiveGenerationID = &activeGenerationID.String
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *MusicSessionRepo) Update(id, title, provider, model string) (*models.MusicSession, error) {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		UPDATE music_sessions
		SET title = COALESCE(NULLIF(?, ''), title),
		    default_provider = COALESCE(NULLIF(?, ''), default_provider),
		    default_model = COALESCE(NULLIF(?, ''), default_model),
		    updated_at = ?
		WHERE id = ?`,
		title, provider, model, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update music session: %w", err)
	}
	return r.GetByID(id)
}

func (r *MusicSessionRepo) UpdateActiveGeneration(sessionID, generationID string) error {
	_, err := r.db.Exec(`
		UPDATE music_sessions SET active_generation_id = ?, updated_at = ? WHERE id = ?`,
		generationID, time.Now().UTC(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("update active music generation: %w", err)
	}
	return nil
}

func (r *MusicSessionRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM music_sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete music session: %w", err)
	}
	return nil
}

func (r *MusicSessionRepo) BelongsToUser(sessionID, userID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var found int
	err := r.db.QueryRow(`SELECT 1 FROM music_sessions WHERE id = ? AND user_id = ?`, sessionID, userID).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check music session owner: %w", err)
	}
	return true, nil
}
