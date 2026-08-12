package db

import (
	"database/sql"
	"fmt"
)

// GitHub repository bindings map one OmniLLM owner and one preconfigured local
// repository ID to immutable GitHub repository identity metadata. No local paths
// or credentials are stored in this table.
const migrationGitHubRepositoryBindings = `
CREATE TABLE IF NOT EXISTS github_repository_bindings (
	owner_id TEXT NOT NULL,
	local_repository_id TEXT NOT NULL,
	github_user_id INTEGER NOT NULL,
	github_repository_id INTEGER NOT NULL,
	github_full_name TEXT NOT NULL,
	default_branch TEXT NOT NULL DEFAULT '',
	private INTEGER NOT NULL DEFAULT 0,
	fork INTEGER NOT NULL DEFAULT 0,
	archived INTEGER NOT NULL DEFAULT 0,
	disabled INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (owner_id, local_repository_id)
);
CREATE INDEX IF NOT EXISTS idx_github_repository_bindings_owner_repo
ON github_repository_bindings(owner_id, github_repository_id);
CREATE INDEX IF NOT EXISTS idx_github_repository_bindings_github_user
ON github_repository_bindings(github_user_id);
`

// EnsureGitHubRepositoryBindingsSchema creates the owner-scoped repository
// binding surface idempotently.
func EnsureGitHubRepositoryBindingsSchema(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}
	if _, err := database.Exec(migrationGitHubRepositoryBindings); err != nil {
		return fmt.Errorf("ensure GitHub repository binding schema: %w", err)
	}
	return nil
}
