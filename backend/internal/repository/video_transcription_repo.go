package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// VideoTranscriptionRepo persists transcripts and timed segments.
type VideoTranscriptionRepo struct{ db *sql.DB }

func NewVideoTranscriptionRepo(db *sql.DB) *VideoTranscriptionRepo {
	return &VideoTranscriptionRepo{db: db}
}

func (r *VideoTranscriptionRepo) Create(item *models.VideoTranscript) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.Status == "" {
		item.Status = "queued"
	}
	if item.PrivacyJSON == "" {
		item.PrivacyJSON = "{}"
	}
	if item.MetadataJSON == "" {
		item.MetadataJSON = "{}"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = item.CreatedAt
	_, err := r.db.Exec(`INSERT INTO video_transcripts(id,project_id,asset_id,user_id,provider_profile_id,provider,model,status,language,translated_language,text,cost_usd,privacy_json,metadata_json,error,created_at,updated_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.ProjectID, item.AssetID, item.UserID, item.ProviderProfileID, item.Provider, item.Model, item.Status, item.Language, item.TranslatedLanguage, item.Text, item.CostUSD, item.PrivacyJSON, item.MetadataJSON, item.Error, item.CreatedAt, item.UpdatedAt, item.CompletedAt)
	if err != nil {
		return fmt.Errorf("create video transcript: %w", err)
	}
	return nil
}

func (r *VideoTranscriptionRepo) GetByID(id string) (*models.VideoTranscript, error) {
	row := r.db.QueryRow(transcriptSelect+` WHERE id=?`, id)
	item, err := scanTranscript(row)
	if err != nil || item == nil {
		return item, err
	}
	item.Segments, err = r.ListSegments(id)
	return item, err
}
func (r *VideoTranscriptionRepo) ListByProject(projectID string) ([]models.VideoTranscript, error) {
	rows, err := r.db.Query(transcriptSelect+` WHERE project_id=? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list video transcripts: %w", err)
	}
	defer rows.Close()
	items := []models.VideoTranscript{}
	for rows.Next() {
		item, scanErr := scanTranscript(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}
func (r *VideoTranscriptionRepo) MarkRunning(id string) error {
	_, err := r.db.Exec(`UPDATE video_transcripts SET status='running',error=NULL,updated_at=? WHERE id=? AND status='queued'`, time.Now().UTC(), id)
	return err
}
func (r *VideoTranscriptionRepo) MarkFailed(id, message string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`UPDATE video_transcripts SET status='failed',error=?,updated_at=?,completed_at=? WHERE id=?`, message, now, now, id)
	return err
}
func (r *VideoTranscriptionRepo) Complete(item *models.VideoTranscript, segments []models.VideoTranscriptSegment) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err = tx.Exec(`UPDATE video_transcripts SET status='completed',language=?,translated_language=?,text=?,cost_usd=?,metadata_json=?,error=NULL,updated_at=?,completed_at=? WHERE id=?`, item.Language, item.TranslatedLanguage, item.Text, item.CostUSD, item.MetadataJSON, now, now, item.ID); err != nil {
		return fmt.Errorf("complete transcript: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM video_transcript_segments WHERE transcript_id=?`, item.ID); err != nil {
		return err
	}
	for index, segment := range segments {
		if segment.ID == "" {
			segment.ID = uuid.NewString()
		}
		segment.TranscriptID = item.ID
		segment.SegmentIndex = index
		if _, err = tx.Exec(`INSERT INTO video_transcript_segments(id,transcript_id,segment_index,start_ms,end_ms,text,speaker,confidence,words_json) VALUES(?,?,?,?,?,?,?,?,?)`, segment.ID, segment.TranscriptID, segment.SegmentIndex, segment.StartMS, segment.EndMS, segment.Text, segment.Speaker, segment.Confidence, segment.WordsJSON); err != nil {
			return fmt.Errorf("insert transcript segment: %w", err)
		}
	}
	return tx.Commit()
}
func (r *VideoTranscriptionRepo) ListSegments(id string) ([]models.VideoTranscriptSegment, error) {
	rows, err := r.db.Query(`SELECT id,transcript_id,segment_index,start_ms,end_ms,text,speaker,confidence,words_json FROM video_transcript_segments WHERE transcript_id=? ORDER BY segment_index`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.VideoTranscriptSegment{}
	for rows.Next() {
		var item models.VideoTranscriptSegment
		var confidence sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.TranscriptID, &item.SegmentIndex, &item.StartMS, &item.EndMS, &item.Text, &item.Speaker, &confidence, &item.WordsJSON); err != nil {
			return nil, err
		}
		if confidence.Valid {
			item.Confidence = &confidence.Float64
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

const transcriptSelect = `SELECT id,project_id,asset_id,user_id,provider_profile_id,provider,model,status,language,translated_language,text,cost_usd,privacy_json,metadata_json,error,created_at,updated_at,completed_at FROM video_transcripts`

type transcriptScanner interface{ Scan(...any) error }

func scanTranscript(row transcriptScanner) (*models.VideoTranscript, error) {
	var item models.VideoTranscript
	var userID, translated, errorMessage sql.NullString
	var cost sql.NullFloat64
	var completed sql.NullTime
	err := row.Scan(&item.ID, &item.ProjectID, &item.AssetID, &userID, &item.ProviderProfileID, &item.Provider, &item.Model, &item.Status, &item.Language, &translated, &item.Text, &cost, &item.PrivacyJSON, &item.MetadataJSON, &errorMessage, &item.CreatedAt, &item.UpdatedAt, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan video transcript: %w", err)
	}
	if userID.Valid {
		item.UserID = &userID.String
	}
	if translated.Valid {
		item.TranslatedLanguage = translated.String
	}
	if cost.Valid {
		item.CostUSD = &cost.Float64
	}
	if errorMessage.Valid {
		item.Error = &errorMessage.String
	}
	if completed.Valid {
		item.CompletedAt = &completed.Time
	}
	return &item, nil
}
