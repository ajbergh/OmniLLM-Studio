package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/githubauth"
)

// GitHubAppConnectionRepo persists one encrypted GitHub App user credential per
// OmniLLM owner. The owner may be an authenticated user ID or the stable local
// solo-mode owner ID.
type GitHubAppConnectionRepo struct{ db *sql.DB }

func NewGitHubAppConnectionRepo(db *sql.DB) *GitHubAppConnectionRepo {
	return &GitHubAppConnectionRepo{db: db}
}

// Get returns the decrypted credential for backend-only execution.
func (r *GitHubAppConnectionRepo) Get(ownerID string) (*githubauth.Credential, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner ID is required")
	}
	var credential githubauth.Credential
	var accessTokenEnc, refreshTokenEnc string
	var accessExpiresAt, refreshExpiresAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT github_user_id, github_login, access_token_enc, refresh_token_enc,
			token_type, scope, access_expires_at, refresh_expires_at
		FROM github_app_connections
		WHERE owner_id = ?
	`, ownerID).Scan(
		&credential.GitHubUserID,
		&credential.GitHubLogin,
		&accessTokenEnc,
		&refreshTokenEnc,
		&credential.TokenType,
		&credential.Scope,
		&accessExpiresAt,
		&refreshExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get GitHub App connection: %w", err)
	}
	accessToken, err := decryptOptional(accessTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt GitHub access token: %w", err)
	}
	refreshToken, err := decryptOptional(refreshTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt GitHub refresh token: %w", err)
	}
	credential.AccessToken = accessToken
	credential.RefreshToken = refreshToken
	credential.GitHubLogin = strings.TrimSpace(credential.GitHubLogin)
	credential.TokenType = strings.TrimSpace(credential.TokenType)
	credential.Scope = strings.TrimSpace(credential.Scope)
	if accessExpiresAt.Valid {
		value := accessExpiresAt.Time.UTC()
		credential.AccessExpiresAt = &value
	}
	if refreshExpiresAt.Valid {
		value := refreshExpiresAt.Time.UTC()
		credential.RefreshExpiresAt = &value
	}
	return &credential, nil
}

// Save encrypts token-bearing values before an owner-scoped upsert.
func (r *GitHubAppConnectionRepo) Save(ownerID string, credential githubauth.Credential) error {
	ownerID = strings.TrimSpace(ownerID)
	credential.GitHubLogin = strings.TrimSpace(credential.GitHubLogin)
	credential.TokenType = strings.TrimSpace(credential.TokenType)
	credential.Scope = strings.TrimSpace(credential.Scope)
	if ownerID == "" {
		return fmt.Errorf("owner ID is required")
	}
	if credential.GitHubUserID <= 0 || credential.GitHubLogin == "" {
		return fmt.Errorf("GitHub user identity is required")
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		return fmt.Errorf("GitHub access token is required")
	}
	accessTokenEnc, err := crypto.Encrypt(credential.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt GitHub access token: %w", err)
	}
	refreshTokenEnc := ""
	if credential.RefreshToken != "" {
		refreshTokenEnc, err = crypto.Encrypt(credential.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt GitHub refresh token: %w", err)
		}
	}
	_, err = r.db.Exec(`
		INSERT INTO github_app_connections (
			owner_id, github_user_id, github_login, access_token_enc, refresh_token_enc,
			token_type, scope, access_expires_at, refresh_expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(owner_id) DO UPDATE SET
			github_user_id = excluded.github_user_id,
			github_login = excluded.github_login,
			access_token_enc = excluded.access_token_enc,
			refresh_token_enc = excluded.refresh_token_enc,
			token_type = excluded.token_type,
			scope = excluded.scope,
			access_expires_at = excluded.access_expires_at,
			refresh_expires_at = excluded.refresh_expires_at,
			updated_at = CURRENT_TIMESTAMP
	`,
		ownerID,
		credential.GitHubUserID,
		credential.GitHubLogin,
		accessTokenEnc,
		refreshTokenEnc,
		credential.TokenType,
		credential.Scope,
		normalizeCredentialTime(credential.AccessExpiresAt),
		normalizeCredentialTime(credential.RefreshExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("save GitHub App connection: %w", err)
	}
	return nil
}

// Clear removes only the selected OmniLLM owner's local GitHub credential.
func (r *GitHubAppConnectionRepo) Clear(ownerID string) error {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return fmt.Errorf("owner ID is required")
	}
	if _, err := r.db.Exec(`DELETE FROM github_app_connections WHERE owner_id = ?`, ownerID); err != nil {
		return fmt.Errorf("clear GitHub App connection: %w", err)
	}
	return nil
}

func normalizeCredentialTime(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return value.UTC()
}
