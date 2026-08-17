package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type VideoTimelineRepo struct {
	db *sql.DB
}

func NewVideoTimelineRepo(db *sql.DB) *VideoTimelineRepo {
	return &VideoTimelineRepo{db: db}
}

func (r *VideoTimelineRepo) Create(t *models.VideoTimeline) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Name == "" {
		t.Name = "Main Timeline"
	}
	if t.TimelineJSON == "" {
		t.TimelineJSON = "{}"
	}
	t.ContentSHA256 = timelineContentSHA256(t.TimelineJSON)
	if t.Revision <= 0 {
		t.Revision = 1
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin create video timeline: %w", err)
	}
	defer tx.Rollback()
	if t.Active {
		if _, err := tx.Exec(`UPDATE video_timelines SET active = 0 WHERE project_id = ?`, t.ProjectID); err != nil {
			return fmt.Errorf("deactivate video timelines: %w", err)
		}
	}
	active := 0
	if t.Active {
		active = 1
	}
	_, err = tx.Exec(`
		INSERT INTO video_timelines (
			id, project_id, name, active, timeline_json, duration_ms, revision, content_sha256, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Name, active, t.TimelineJSON, t.DurationMS, t.Revision, t.ContentSHA256, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create video timeline: %w", err)
	}
	if t.Active {
		if _, err := tx.Exec(`UPDATE video_projects SET active_timeline_id = ?, duration_ms = ?, updated_at = ? WHERE id = ?`, t.ID, t.DurationMS, t.UpdatedAt, t.ProjectID); err != nil {
			return fmt.Errorf("update active video timeline: %w", err)
		}
	}
	return tx.Commit()
}

func (r *VideoTimelineRepo) GetByID(id string) (*models.VideoTimeline, error) {
	row := r.db.QueryRow(videoTimelineSelectSQL+` WHERE id = ?`, id)
	return scanVideoTimeline(row)
}

func (r *VideoTimelineRepo) GetActiveByProject(projectID string) (*models.VideoTimeline, error) {
	row := r.db.QueryRow(videoTimelineSelectSQL+` WHERE project_id = ? AND active = 1 ORDER BY updated_at DESC LIMIT 1`, projectID)
	return scanVideoTimeline(row)
}

func (r *VideoTimelineRepo) Save(t *models.VideoTimeline) error {
	if t.ID == "" {
		return r.Create(t)
	}
	if t.Name == "" {
		t.Name = "Main Timeline"
	}
	if t.TimelineJSON == "" {
		t.TimelineJSON = "{}"
	}
	t.ContentSHA256 = timelineContentSHA256(t.TimelineJSON)
	t.UpdatedAt = time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save video timeline: %w", err)
	}
	defer tx.Rollback()
	if t.Active {
		if _, err := tx.Exec(`UPDATE video_timelines SET active = 0 WHERE project_id = ? AND id <> ?`, t.ProjectID, t.ID); err != nil {
			return fmt.Errorf("deactivate video timelines: %w", err)
		}
	}
	active := 0
	if t.Active {
		active = 1
	}
	err = tx.QueryRow(`
		UPDATE video_timelines
		SET name = ?, active = ?, timeline_json = ?, duration_ms = ?,
		    revision = revision + 1, content_sha256 = ?, updated_at = ?
		WHERE id = ? AND project_id = ?
		RETURNING revision`,
		t.Name, active, t.TimelineJSON, t.DurationMS, t.ContentSHA256, t.UpdatedAt, t.ID, t.ProjectID,
	).Scan(&t.Revision)
	if err != nil {
		return fmt.Errorf("save video timeline: %w", err)
	}
	if t.Active {
		if _, err := tx.Exec(`UPDATE video_projects SET active_timeline_id = ?, duration_ms = ?, updated_at = ? WHERE id = ?`, t.ID, t.DurationMS, t.UpdatedAt, t.ProjectID); err != nil {
			return fmt.Errorf("update video project active timeline: %w", err)
		}
	}
	return tx.Commit()
}

func (r *VideoTimelineRepo) Delete(id string) error {
	if _, err := r.db.Exec(`DELETE FROM video_timelines WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete video timeline: %w", err)
	}
	return nil
}

const videoTimelineSelectSQL = `
	SELECT id, project_id, name, active, timeline_json, duration_ms, revision, content_sha256, created_at, updated_at
	FROM video_timelines`

func scanVideoTimeline(row rowScanner) (*models.VideoTimeline, error) {
	var t models.VideoTimeline
	var active int
	err := row.Scan(&t.ID, &t.ProjectID, &t.Name, &active, &t.TimelineJSON, &t.DurationMS, &t.Revision, &t.ContentSHA256, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video timeline: %w", err)
	}
	t.Active = active == 1
	return &t, nil
}

func timelineContentSHA256(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
