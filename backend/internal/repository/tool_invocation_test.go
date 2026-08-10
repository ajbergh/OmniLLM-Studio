package repository

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newToolInvocationTestRepo(t *testing.T) (*ToolInvocationRepo, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := EnsureAgentRuntimeSchema(db); err != nil {
		_ = db.Close()
		t.Fatalf("ensure agent runtime schema: %v", err)
	}
	return NewToolInvocationRepo(db), db
}

func insertToolInvocationForTest(t *testing.T, db *sql.DB, id, userID, toolName, status string, minutesAgo int) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO tool_invocations (
			id, tool_call_id, tool_name, user_id, status, approval_status,
			conversation_id, run_id, arguments_json, result_json,
			duration_ms, result_bytes, retry_count, created_at
		) VALUES (?, ?, ?, ?, ?, 'approved', ?, ?, ?, ?, 25, 128, 1, datetime('now', ?))
	`, id, "call-"+id, toolName, userID, status, "conversation-"+id, "run-"+id,
		`{"secret":"must-not-be-returned"}`, `{"private":"result"}`, "-"+string(rune('0'+minutesAgo))+" minutes"); err != nil {
		t.Fatalf("insert invocation %s: %v", id, err)
	}
}

func TestToolInvocationRepoListForUserIsolatesAndFilters(t *testing.T) {
	repo, db := newToolInvocationTestRepo(t)
	defer db.Close()

	insertToolInvocationForTest(t, db, "one", "user-a", "calculator", "tool_completed", 3)
	insertToolInvocationForTest(t, db, "two", "user-a", "web_search", "tool_failed", 2)
	insertToolInvocationForTest(t, db, "three", "user-b", "calculator", "tool_completed", 1)

	items, err := repo.ListForUser("user-a", ToolInvocationListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list user-a: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("user-a item count = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.ID == "three" {
			t.Fatal("user-b invocation leaked into user-a diagnostics")
		}
	}

	failed, err := repo.ListForUser("user-a", ToolInvocationListOptions{ToolName: "web_search", Status: "tool_failed"})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != "two" {
		t.Fatalf("filtered result = %#v, want only invocation two", failed)
	}
}

func TestToolInvocationRepoCapsLimit(t *testing.T) {
	repo, db := newToolInvocationTestRepo(t)
	defer db.Close()

	for i := 0; i < 205; i++ {
		id := "invocation-" + string(rune(0x1000+i))
		insertToolInvocationForTest(t, db, id, "user-a", "calculator", "tool_completed", 1)
	}

	items, err := repo.ListForUser("user-a", ToolInvocationListOptions{Limit: 999})
	if err != nil {
		t.Fatalf("list capped invocations: %v", err)
	}
	if len(items) != 200 {
		t.Fatalf("capped item count = %d, want 200", len(items))
	}
}
