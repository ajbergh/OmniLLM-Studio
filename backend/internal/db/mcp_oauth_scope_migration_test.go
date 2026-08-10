package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationV50AddsRequiredOAuthScope(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE mcp_servers (id TEXT PRIMARY KEY);`); err != nil {
		t.Fatal(err)
	}
	for version, migration := range []string{migrationMCPOAuthCredentials, migrationMCPOAuthRegistrationBinding, migrationMCPOAuthIncrementalScope} {
		if _, err := database.Exec(migration); err != nil {
			t.Fatalf("apply OAuth migration %d: %v", version+48, err)
		}
	}
	rows, err := database.Query(`PRAGMA table_info(mcp_oauth_credentials)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "required_scope" {
			found = true
		}
	}
	if !found {
		t.Fatal("required_scope column missing after V50")
	}
}
