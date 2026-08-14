package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenEnablesForeignKeysOnEveryConnection(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "foreign-keys.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	connections := make([]*sql.Conn, 0, 4)
	defer func() {
		for _, connection := range connections {
			connection.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		connection, err := database.Conn(context.Background())
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		connections = append(connections, connection)
		var enabled int
		if err := connection.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&enabled); err != nil {
			t.Fatalf("query connection %d: %v", i, err)
		}
		if enabled != 1 {
			t.Fatalf("connection %d foreign_keys=%d, want 1", i, enabled)
		}
	}

	if _, err := connections[0].ExecContext(context.Background(), `
		INSERT INTO messages (id, conversation_id, role, content)
		VALUES ('orphan', 'missing', 'user', 'blocked')
	`); err == nil {
		t.Fatal("expected an orphan insert to be rejected")
	}
}

func TestMigrateRepairsLegacyActionableOrphans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-orphans.db")
	legacy := openLegacyDatabase(t, path)
	if err := Migrate(legacy); err != nil {
		t.Fatalf("initial legacy migrate: %v", err)
	}
	if _, err := legacy.Exec(`
		INSERT INTO conversations (id, title, user_id) VALUES ('kept', 'Kept', 'missing-user');
		INSERT INTO messages (id, conversation_id, role, content) VALUES ('orphan-message', 'missing-conversation', 'user', 'remove');
		INSERT INTO attachments (id, conversation_id, message_id, type, storage_path)
		VALUES ('attachment', 'kept', 'missing-message', 'file', 'fixture');
	`); err != nil {
		t.Fatalf("seed legacy orphans: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatalf("admit legacy db: %v", err)
	}

	var conversationUser, attachmentMessage sql.NullString
	if err := database.QueryRow("SELECT user_id FROM conversations WHERE id = 'kept'").Scan(&conversationUser); err != nil {
		t.Fatalf("query conversation: %v", err)
	}
	if conversationUser.Valid {
		t.Fatalf("conversation user was not anonymized: %q", conversationUser.String)
	}
	if err := database.QueryRow("SELECT message_id FROM attachments WHERE id = 'attachment'").Scan(&attachmentMessage); err != nil {
		t.Fatalf("query attachment: %v", err)
	}
	if attachmentMessage.Valid {
		t.Fatalf("attachment message was not cleared: %q", attachmentMessage.String)
	}
	var orphanMessages int
	if err := database.QueryRow("SELECT COUNT(*) FROM messages WHERE id = 'orphan-message'").Scan(&orphanMessages); err != nil {
		t.Fatalf("query orphan message: %v", err)
	}
	if orphanMessages != 0 {
		t.Fatalf("orphan cascade child count=%d, want 0", orphanMessages)
	}
	violations, err := ForeignKeyViolations(database)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("remaining violations: %+v", violations)
	}
}

func TestMigrateRejectsAmbiguousLegacyOrphan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ambiguous-orphan.db")
	legacy := openLegacyDatabase(t, path)
	if err := Migrate(legacy); err != nil {
		t.Fatalf("initial legacy migrate: %v", err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE integrity_parent (id TEXT PRIMARY KEY);
		CREATE TABLE integrity_child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES integrity_parent(id)
		);
		INSERT INTO integrity_child (id, parent_id) VALUES ('child', 'missing');
	`); err != nil {
		t.Fatalf("seed ambiguous orphan: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer database.Close()
	err = Migrate(database)
	if err == nil {
		t.Fatal("expected ambiguous orphan to fail admission")
	}
	if !strings.Contains(err.Error(), "integrity_child") || !strings.Contains(err.Error(), "integrity_parent") {
		t.Fatalf("error does not identify the violated relationship: %v", err)
	}
}

func TestV22MigrationPreservesAgentStepsWithForeignKeysEnabled(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "v22.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(migrationSchemaVersions + migrationConversations + migrationMessages + migrationAgentRuns + migrationAgentSteps); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO schema_versions (version, name) VALUES (21, 'fixture');
		INSERT INTO conversations (id, title) VALUES ('conversation', 'Fixture');
		INSERT INTO agent_runs (id, conversation_id, goal) VALUES ('run', 'conversation', 'test');
		INSERT INTO agent_steps (id, run_id, step_index, type, description)
		VALUES ('step', 'run', 0, 'think', 'preserve');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	migration := Migration{Version: 22, Name: "agent_runs_awaiting_approval", SQL: migrationAgentRunsAwaitingApproval}
	if err := runVersionedMigration(database, migration); err != nil {
		t.Fatalf("run V22: %v", err)
	}
	var steps int
	if err := database.QueryRow("SELECT COUNT(*) FROM agent_steps WHERE id = 'step'").Scan(&steps); err != nil {
		t.Fatalf("query agent step: %v", err)
	}
	if steps != 1 {
		t.Fatalf("agent step count=%d, want 1", steps)
	}
	var enabled int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys=%d after V22, want 1", enabled)
	}
}

func openLegacyDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		t.Fatalf("ping legacy db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
