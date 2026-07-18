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

// SessionRepo handles CRUD operations for sessions. The sessions.token column
// stores only a SHA-256 digest; plaintext bearer tokens are returned once to the
// caller and are never persisted.
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo creates a new SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session using a one-way token digest.
func (r *SessionRepo) Create(userID, token string, expiresAt time.Time) (*models.Session, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	digest := sessionTokenDigest(token)

	_, err := r.db.Exec(
		`INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, digest, expiresAt, now,
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

// GetByToken retrieves a session by hashing the presented bearer token.
func (r *SessionRepo) GetByToken(token string) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(
		`SELECT id, user_id, expires_at, created_at
		 FROM sessions WHERE token = ?`, sessionTokenDigest(token),
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	return s, nil
}

// DeleteByToken removes a session by hashing the presented bearer token.
func (r *SessionRepo) DeleteByToken(token string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE token = ?", sessionTokenDigest(token))
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

func sessionTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
