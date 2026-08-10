package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMCPHTTPPrivateNetworkMigrationPreservesExistingHTTPServers(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`
		CREATE TABLE mcp_servers (
			id TEXT PRIMARY KEY,
			transport TEXT NOT NULL DEFAULT 'stdio'
		);
		INSERT INTO mcp_servers (id, transport) VALUES ('existing-http', 'http'), ('existing-stdio', 'stdio');
	`); err != nil {
		t.Fatalf("prepare legacy MCP schema: %v", err)
	}
	if _, err := database.Exec(migrationMCPHTTPPrivateNetwork); err != nil {
		t.Fatalf("apply MCP HTTP security migration: %v", err)
	}

	var httpAllowed, stdioAllowed int
	if err := database.QueryRow(`SELECT allow_private_network FROM mcp_servers WHERE id='existing-http'`).Scan(&httpAllowed); err != nil {
		t.Fatalf("read migrated HTTP server: %v", err)
	}
	if err := database.QueryRow(`SELECT allow_private_network FROM mcp_servers WHERE id='existing-stdio'`).Scan(&stdioAllowed); err != nil {
		t.Fatalf("read migrated stdio server: %v", err)
	}
	if httpAllowed != 1 {
		t.Fatalf("existing HTTP allow_private_network = %d, want 1 for upgrade compatibility", httpAllowed)
	}
	if stdioAllowed != 0 {
		t.Fatalf("existing stdio allow_private_network = %d, want 0", stdioAllowed)
	}
}
