package db

import (
	"database/sql"
	"fmt"
)

// GitHub App user-access credentials live in a dedicated owner-scoped table.
// Token-bearing values are encrypted by the repository layer before storage.
const migrationGitHubAppConnections = `
CREATE TABLE IF NOT EXISTS github_app_connections (
	owner_id TEXT PRIMARY KEY,
	github_user_id INTEGER NOT NULL,
	github_login TEXT NOT NULL DEFAULT '',
	access_token_enc TEXT NOT NULL DEFAULT '',
	refresh_token_enc TEXT NOT NULL DEFAULT '',
	token_type TEXT NOT NULL DEFAULT '',
	scope TEXT NOT NULL DEFAULT '',
	access_expires_at DATETIME,
	refresh_expires_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_github_app_connections_user
ON github_app_connections(github_user_id);
`

// EnsureGitHubAppConnectionsSchema creates the dedicated GitHub credential
// persistence surface idempotently. G2 keeps this isolated from the generic
// settings table so no settings API can enumerate credential ciphertext.
func EnsureGitHubAppConnectionsSchema(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}
	if _, err := database.Exec(migrationGitHubAppConnections); err != nil {
		return fmt.Errorf("ensure GitHub App connection schema: %w", err)
	}
	return nil
}
