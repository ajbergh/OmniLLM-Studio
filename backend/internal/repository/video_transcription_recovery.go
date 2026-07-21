package repository

import (
	"fmt"
	"time"
)

// FailInterrupted marks queued/running transcription jobs as failed during
// startup recovery. Provider requests cannot be resumed safely because the
// remote operation identifier is not part of the provider-neutral contract.
func (r *VideoTranscriptionRepo) FailInterrupted(message string) (int64, error) {
	now := time.Now().UTC()
	result, err := r.db.Exec(`
		UPDATE video_transcripts
		SET status = 'failed', error = ?, updated_at = ?, completed_at = ?
		WHERE status IN ('queued', 'running')
	`, message, now, now)
	if err != nil {
		return 0, fmt.Errorf("fail interrupted video transcriptions: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count interrupted video transcriptions: %w", err)
	}
	return count, nil
}
