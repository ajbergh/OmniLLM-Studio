package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMCPOAuthCredentialsMigrationCreatesCascadeTable(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE mcp_servers (id TEXT PRIMARY KEY);
		INSERT INTO mcp_servers (id) VALUES ('server-1');
	`); err != nil {
		t.Fatalf("prepare MCP schema: %v", err)
	}
	if _, err := database.Exec(migrationMCPOAuthCredentials); err != nil {
		t.Fatalf("apply MCP OAuth migration: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO mcp_oauth_credentials (server_id, client_id) VALUES ('server-1', 'client-1')`); err != nil {
		t.Fatalf("insert OAuth credential: %v", err)
	}
	if _, err := database.Exec(`DELETE FROM mcp_servers WHERE id='server-1'`); err != nil {
		t.Fatalf("delete MCP server: %v", err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM mcp_oauth_credentials`).Scan(&count); err != nil {
		t.Fatalf("count OAuth credentials: %v", err)
	}
	if count != 0 {
		t.Fatalf("OAuth credential count = %d, want 0 after server cascade delete", count)
	}
}
