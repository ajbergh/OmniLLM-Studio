package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGitHubAppConnectionsMigrationCreatesOwnerScopedCredentialTable(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(migrationGitHubAppConnections); err != nil {
		t.Fatalf("apply GitHub App migration: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO github_app_connections (
			owner_id, github_user_id, github_login, access_token_enc, refresh_token_enc, token_type
		) VALUES ('local', 12345, 'octocat', 'cipher-a', 'cipher-r', 'bearer')
	`); err != nil {
		t.Fatalf("insert GitHub App connection: %v", err)
	}
	var ownerID, login, accessTokenEnc string
	var githubUserID int64
	if err := database.QueryRow(`
		SELECT owner_id, github_user_id, github_login, access_token_enc
		FROM github_app_connections WHERE owner_id = 'local'
	`).Scan(&ownerID, &githubUserID, &login, &accessTokenEnc); err != nil {
		t.Fatalf("read GitHub App connection: %v", err)
	}
	if ownerID != "local" || githubUserID != 12345 || login != "octocat" || accessTokenEnc != "cipher-a" {
		t.Fatalf("unexpected GitHub App row: owner=%q user=%d login=%q access=%q", ownerID, githubUserID, login, accessTokenEnc)
	}
	if _, err := database.Exec(`INSERT INTO github_app_connections (owner_id, github_user_id, github_login) VALUES ('local', 999, 'other')`); err == nil {
		t.Fatal("expected owner_id primary key to reject a second connection for the same owner")
	}
}
