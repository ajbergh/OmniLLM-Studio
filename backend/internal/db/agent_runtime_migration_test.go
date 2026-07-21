package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentRuntimeMigration(t *testing.T) {
	t.Setenv("OMNILLM_MASTER_KEY", strings.Repeat("0", 64))
	database, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	version, err := SchemaVersion(database)
	if err != nil {
		t.Fatal(err)
	}
	if version != 43 {
		t.Fatalf("expected schema version 43, got %d", version)
	}
	for _, table := range []string{
		"tool_invocations", "agent_events", "agent_jobs", "scheduled_tasks", "memories", "app_connections",
		"video_transcripts", "video_transcript_segments",
	} {
		var name string
		if err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("runtime table %s missing: %v", table, err)
		}
	}
}
