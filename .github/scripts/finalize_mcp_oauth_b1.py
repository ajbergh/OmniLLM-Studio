from pathlib import Path

Path("backend/internal/db/mcp_oauth_registration_migration_test.go").write_text(r'''package db

import (
    "database/sql"
    "testing"

    _ "modernc.org/sqlite"
)

func TestMigrationV49AddsOAuthRegistrationBinding(t *testing.T) {
    database, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("open sqlite: %v", err)
    }
    defer database.Close()

    if _, err := database.Exec(`CREATE TABLE mcp_servers (id TEXT PRIMARY KEY);`); err != nil {
        t.Fatalf("prepare MCP server schema: %v", err)
    }
    if _, err := database.Exec(migrationMCPOAuthCredentials); err != nil {
        t.Fatalf("apply V48 OAuth migration: %v", err)
    }
    if _, err := database.Exec(migrationMCPOAuthRegistrationBinding); err != nil {
        t.Fatalf("apply V49 OAuth registration migration: %v", err)
    }

    rows, err := database.Query(`PRAGMA table_info(mcp_oauth_credentials)`)
    if err != nil {
        t.Fatal(err)
    }
    defer rows.Close()

    found := map[string]bool{}
    for rows.Next() {
        var cid int
        var name, typ string
        var notnull int
        var defaultValue interface{}
        var pk int
        if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
            t.Fatal(err)
        }
        if name == "registration_method" || name == "client_issuer" {
            found[name] = true
        }
    }
    if err := rows.Err(); err != nil {
        t.Fatal(err)
    }
    if !found["registration_method"] || !found["client_issuer"] {
        t.Fatalf("V49 columns missing: %#v", found)
    }

    if _, err := database.Exec(`INSERT INTO mcp_oauth_credentials (server_id, client_id) VALUES ('server-1', 'client-1')`); err == nil {
        // The parent row is absent and FK enforcement is off in normal Omni runtime;
        // this insert is not part of the column contract under test.
    }
}
''')
