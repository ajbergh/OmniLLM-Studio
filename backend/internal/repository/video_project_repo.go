package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type VideoProjectRepo struct {
	db *sql.DB
}

func NewVideoProjectRepo(db *sql.DB) *VideoProjectRepo {
	return &VideoProjectRepo{db: db}
}

func (r *VideoProjectRepo) Create(userID, title, provider, model string, width, height, fps int, aspectRatio string) (*models.VideoProject, error) {
	now := time.Now().UTC()
	project := &models.VideoProject{
		ID:           uuid.New().String(),
		Title:        title,
		Width:        width,
		Height:       height,
		FPS:          fps,
		AspectRatio:  aspectRatio,
		MetadataJSON: "{}",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	var uid *string
	if userID != "" {
		uid = &userID
		project.UserID = uid
	}
	if provider != "" {
		project.DefaultProvider = &provider
	}
	if model != "" {
		project.DefaultModel = &model
	}
	_, err := r.db.Exec(`
		INSERT INTO video_projects (
			id, user_id, title, default_provider, default_model, width, height, fps,
			duration_ms, aspect_ratio, metadata_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.ID, uid, project.Title, project.DefaultProvider, project.DefaultModel,
		project.Width, project.Height, project.FPS, project.DurationMS, project.AspectRatio,
		project.MetadataJSON, project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create video project: %w", err)
	}
	return project, nil
}

func (r *VideoProjectRepo) GetByID(id string) (*models.VideoProject, error) {
	row := r.db.QueryRow(`
		SELECT id, user_id, title, active_timeline_id, default_provider, default_model,
		       width, height, fps, duration_ms, aspect_ratio, metadata_json, created_at, updated_at
		FROM video_projects WHERE id = ?`, id)
	return scanVideoProject(row)
}

func (r *VideoProjectRepo) ListForUser(userID string) ([]models.VideoProject, error) {
	query := `
		SELECT id, user_id, title, active_timeline_id, default_provider, default_model,
		       width, height, fps, duration_ms, aspect_ratio, metadata_json, created_at, updated_at
		FROM video_projects`
	args := []interface{}{}
	if userID != "" {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list video projects: %w", err)
	}
	defer rows.Close()

	var projects []models.VideoProject
	for rows.Next() {
		project, err := scanVideoProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *project)
	}
	return projects, rows.Err()
}

func (r *VideoProjectRepo) Update(id, title, provider, model string, width, height, fps int, aspectRatio string) (*models.VideoProject, error) {
	_, err := r.db.Exec(`
		UPDATE video_projects
		SET title = COALESCE(NULLIF(?, ''), title),
		    default_provider = COALESCE(NULLIF(?, ''), default_provider),
		    default_model = COALESCE(NULLIF(?, ''), default_model),
		    width = CASE WHEN ? > 0 THEN ? ELSE width END,
		    height = CASE WHEN ? > 0 THEN ? ELSE height END,
		    fps = CASE WHEN ? > 0 THEN ? ELSE fps END,
		    aspect_ratio = COALESCE(NULLIF(?, ''), aspect_ratio),
		    updated_at = ?
		WHERE id = ?`,
		title, provider, model,
		width, width, height, height, fps, fps, aspectRatio,
		time.Now().UTC(), id,
	)
	if err != nil {
		return nil, fmt.Errorf("update video project: %w", err)
	}
	return r.GetByID(id)
}

func (r *VideoProjectRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM video_projects WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete video project: %w", err)
	}
	return nil
}

func (r *VideoProjectRepo) BelongsToUser(projectID, userID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var found int
	err := r.db.QueryRow(`SELECT 1 FROM video_projects WHERE id = ? AND user_id = ?`, projectID, userID).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check video project owner: %w", err)
	}
	return true, nil
}

func scanVideoProject(row rowScanner) (*models.VideoProject, error) {
	var project models.VideoProject
	var userID, timelineID, provider, model sql.NullString
	err := row.Scan(
		&project.ID, &userID, &project.Title, &timelineID, &provider, &model,
		&project.Width, &project.Height, &project.FPS, &project.DurationMS,
		&project.AspectRatio, &project.MetadataJSON, &project.CreatedAt, &project.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video project: %w", err)
	}
	if userID.Valid {
		project.UserID = &userID.String
	}
	if timelineID.Valid {
		project.ActiveTimelineID = &timelineID.String
	}
	if provider.Valid {
		project.DefaultProvider = &provider.String
	}
	if model.Valid {
		project.DefaultModel = &model.String
	}
	return &project, nil
}
