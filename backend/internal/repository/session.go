package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// SessionRepo handles CRUD operations for sessions.
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo creates a new SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session.
func (r *SessionRepo) Create(userID, token string, expiresAt time.Time) (*models.Session, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := r.db.Exec(
		`INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, token, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &models.Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// GetByToken retrieves a session by its token value.
func (r *SessionRepo) GetByToken(token string) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(
		`SELECT id, user_id, token, expires_at, created_at
		 FROM sessions WHERE token = ?`, token,
	).Scan(&s.ID, &s.UserID, &s.Token, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	return s, nil
}

// DeleteByToken removes a session by token.
func (r *SessionRepo) DeleteByToken(token string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// DeleteByUserID removes all sessions for a user.
func (r *SessionRepo) DeleteByUserID(userID string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// DeleteExpired removes all expired sessions.
func (r *SessionRepo) DeleteExpired() (int64, error) {
	result, err := r.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
