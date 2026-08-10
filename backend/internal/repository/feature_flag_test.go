package repository

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newFeatureFlagPolicyTestRepo(t *testing.T) *FeatureFlagRepo {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE feature_flags (
			key TEXT PRIMARY KEY,
			enabled INTEGER NOT NULL DEFAULT 0,
			metadata TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE tool_permissions (
			tool_name TEXT PRIMARY KEY,
			policy TEXT NOT NULL
		);
		INSERT INTO feature_flags (key, enabled, metadata)
		VALUES ('sports_lookup_enabled', 1, 'test');
	`); err != nil {
		t.Fatalf("create test schema: %v", err)
	}

	return NewFeatureFlagRepo(db)
}

func setSportsToolPolicy(t *testing.T, repo *FeatureFlagRepo, policy string) {
	t.Helper()
	if _, err := repo.db.Exec(
		"INSERT INTO tool_permissions (tool_name, policy) VALUES (?, ?)",
		"sports_lookup",
		policy,
	); err != nil {
		t.Fatalf("insert sports policy: %v", err)
	}
}

func TestFeatureFlagAsMapDisablesSportsPreflightWhenToolDenied(t *testing.T) {
	repo := newFeatureFlagPolicyTestRepo(t)
	setSportsToolPolicy(t, repo, "deny")

	flags, err := repo.AsMap()
	if err != nil {
		t.Fatalf("AsMap: %v", err)
	}
	if flags["sports_lookup_enabled"] {
		t.Fatal("sports_lookup_enabled = true, want false when sports_lookup policy is deny")
	}
}

func TestFeatureFlagAsMapDisablesSportsPreflightWhenToolRequiresApproval(t *testing.T) {
	repo := newFeatureFlagPolicyTestRepo(t)
	setSportsToolPolicy(t, repo, "ask")

	flags, err := repo.AsMap()
	if err != nil {
		t.Fatalf("AsMap: %v", err)
	}
	if flags["sports_lookup_enabled"] {
		t.Fatal("sports_lookup_enabled = true, want false when sports_lookup policy is ask")
	}
}

func TestFeatureFlagAsMapPreservesSportsFeatureFlagWhenToolAllowed(t *testing.T) {
	repo := newFeatureFlagPolicyTestRepo(t)
	setSportsToolPolicy(t, repo, "allow")

	flags, err := repo.AsMap()
	if err != nil {
		t.Fatalf("AsMap: %v", err)
	}
	if !flags["sports_lookup_enabled"] {
		t.Fatal("sports_lookup_enabled = false, want true when feature and tool policy are enabled")
	}

	if err := repo.Set("sports_lookup_enabled", false); err != nil {
		t.Fatalf("disable feature flag: %v", err)
	}
	flags, err = repo.AsMap()
	if err != nil {
		t.Fatalf("AsMap after feature disable: %v", err)
	}
	if flags["sports_lookup_enabled"] {
		t.Fatal("sports_lookup_enabled = true, want false when feature flag itself is disabled")
	}
}

func TestFeatureFlagAsMapPreservesLegacySportsDefaultWithoutToolPermission(t *testing.T) {
	repo := newFeatureFlagPolicyTestRepo(t)

	flags, err := repo.AsMap()
	if err != nil {
		t.Fatalf("AsMap: %v", err)
	}
	if !flags["sports_lookup_enabled"] {
		t.Fatal("sports_lookup_enabled = false, want legacy feature flag value without persisted tool permission")
	}
}
