package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// BrowserSessionRepo manages persisted metadata for active browser sessions.
type BrowserSessionRepo struct {
	db *sql.DB
}

// NewBrowserSessionRepo creates a BrowserSessionRepo.
func NewBrowserSessionRepo(db *sql.DB) *BrowserSessionRepo {
	return &BrowserSessionRepo{db: db}
}

// Create inserts a browser session row.
func (r *BrowserSessionRepo) Create(session *models.BrowserSession) error {
	if session == nil {
		return fmt.Errorf("browser session is nil")
	}
	if session.Metadata == "" {
		session.Metadata = "{}"
	}
	_, err := r.db.Exec(`
		INSERT INTO browser_sessions (id, user_id, created_at, last_used_at, current_url, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.CreatedAt, session.LastUsedAt, session.CurrentURL, session.Metadata)
	if err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	return nil
}

// UpdateLastUsed updates last-used timestamp and current URL.
func (r *BrowserSessionRepo) UpdateLastUsed(id string, currentURL string) error {
	_, err := r.db.Exec(`
		UPDATE browser_sessions
		SET last_used_at = ?, current_url = ?
		WHERE id = ?
	`, time.Now().UTC(), currentURL, id)
	if err != nil {
		return fmt.Errorf("update browser session: %w", err)
	}
	return nil
}

// Delete removes a browser session row.
func (r *BrowserSessionRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM browser_sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete browser session: %w", err)
	}
	return nil
}

// DeleteAll removes all browser session rows. Used at startup because live
// CDP pages are in-memory and cannot survive a process restart.
func (r *BrowserSessionRepo) DeleteAll() error {
	if _, err := r.db.Exec("DELETE FROM browser_sessions"); err != nil {
		return fmt.Errorf("delete all browser sessions: %w", err)
	}
	return nil
}

// CleanupExpired deletes session rows older than the provided timestamp.
func (r *BrowserSessionRepo) CleanupExpired(before time.Time) error {
	_, err := r.db.Exec("DELETE FROM browser_sessions WHERE last_used_at < ?", before.UTC())
	if err != nil {
		return fmt.Errorf("cleanup expired browser sessions: %w", err)
	}
	return nil
}

// ListByUser returns persisted browser session metadata for a user.
func (r *BrowserSessionRepo) ListByUser(userID string) ([]models.BrowserSession, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, created_at, last_used_at, current_url, metadata
		FROM browser_sessions
		WHERE user_id = ?
		ORDER BY last_used_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list browser sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.BrowserSession
	for rows.Next() {
		var s models.BrowserSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastUsedAt, &s.CurrentURL, &s.Metadata); err != nil {
			return nil, fmt.Errorf("scan browser session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
