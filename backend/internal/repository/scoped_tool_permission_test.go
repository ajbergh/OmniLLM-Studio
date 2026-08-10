package repository

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newScopedPolicyRepo(t *testing.T) *ScopedToolPermissionRepo {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE tool_permission_scopes(scope_type TEXT NOT NULL,scope_id TEXT NOT NULL,tool_name TEXT NOT NULL,policy TEXT NOT NULL,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(scope_type,scope_id,tool_name));`)
	if err != nil {
		t.Fatal(err)
	}
	return NewScopedToolPermissionRepo(db)
}

func TestScopedPolicyIsMonotonic(t *testing.T) {
	repo := newScopedPolicyRepo(t)
	if err := repo.Upsert("user", "u1", "search", "ask"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert("workspace", "w1", "search", "allow"); err != nil {
		t.Fatal(err)
	}
	if got := repo.Resolve("u1", "w1", "c1", "search", "allow"); got != "ask" {
		t.Fatalf("workspace allow widened user ask: %s", got)
	}
	if err := repo.Upsert("conversation", "c1", "search", "deny"); err != nil {
		t.Fatal(err)
	}
	if got := repo.Resolve("u1", "w1", "c1", "search", "allow"); got != "deny" {
		t.Fatalf("conversation deny not authoritative: %s", got)
	}
	if got := repo.Resolve("u1", "w1", "c1", "search", "ask"); got != "deny" {
		t.Fatalf("child scope widened base ask: %s", got)
	}
}

func TestScopedPolicyLookupFailureFailsClosed(t *testing.T) {
	repo := newScopedPolicyRepo(t)
	_ = repo.db.Close()
	if got := repo.Resolve("u", "", "", "search", "allow"); got != "deny" {
		t.Fatalf("lookup failure=%s, want deny", got)
	}
}
