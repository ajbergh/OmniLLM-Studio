package repository

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newToolPermissionTestRepo(t *testing.T) (*ToolPermissionRepo, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE tool_permissions (
			tool_name TEXT PRIMARY KEY,
			policy TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		_ = db.Close()
		t.Fatalf("create tool_permissions: %v", err)
	}
	return NewToolPermissionRepo(db), db
}

func TestToolPermissionPolicyResolverDistinguishesMissingRowFromLookupError(t *testing.T) {
	repo, db := newToolPermissionTestRepo(t)
	resolver := repo.PolicyResolver()

	if got := resolver("missing_tool"); got != "" {
		t.Fatalf("missing policy = %q, want empty for definition-based default", got)
	}

	if err := repo.Upsert("ask_tool", "ask"); err != nil {
		t.Fatalf("upsert ask policy: %v", err)
	}
	if got := resolver("ask_tool"); got != "ask" {
		t.Fatalf("stored policy = %q, want ask", got)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if got := resolver("ask_tool"); got != "deny" {
		t.Fatalf("lookup error policy = %q, want deny", got)
	}
}
