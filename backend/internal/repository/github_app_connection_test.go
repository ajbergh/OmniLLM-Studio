package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	_ "modernc.org/sqlite"
)

func newGitHubAppConnectionTestRepo(t *testing.T) (*GitHubAppConnectionRepo, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := database.Exec(`
		CREATE TABLE github_app_connections (
			owner_id TEXT PRIMARY KEY,
			github_user_id INTEGER NOT NULL,
			github_login TEXT NOT NULL DEFAULT '',
			access_token_enc TEXT NOT NULL DEFAULT '',
			refresh_token_enc TEXT NOT NULL DEFAULT '',
			token_type TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT '',
			access_expires_at DATETIME,
			refresh_expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		database.Close()
		t.Fatalf("create schema: %v", err)
	}
	return NewGitHubAppConnectionRepo(database), database
}

func TestGitHubAppConnectionRepoEncryptsTokensAndRoundTrips(t *testing.T) {
	repo, database := newGitHubAppConnectionTestRepo(t)
	defer database.Close()
	now := time.Now().UTC().Truncate(time.Second)
	accessExpiry := now.Add(8 * time.Hour)
	refreshExpiry := now.Add(180 * 24 * time.Hour)
	credential := githubauth.Credential{
		AccessToken:      "ghu_access_plaintext",
		RefreshToken:     "ghr_refresh_plaintext",
		TokenType:        "bearer",
		Scope:            "repo",
		AccessExpiresAt:  &accessExpiry,
		RefreshExpiresAt: &refreshExpiry,
		GitHubUserID:     12345,
		GitHubLogin:      "octocat",
	}
	if err := repo.Save("owner-a", credential); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	var accessTokenEnc, refreshTokenEnc string
	if err := database.QueryRow(`SELECT access_token_enc, refresh_token_enc FROM github_app_connections WHERE owner_id = 'owner-a'`).Scan(&accessTokenEnc, &refreshTokenEnc); err != nil {
		t.Fatalf("read encrypted columns: %v", err)
	}
	if accessTokenEnc == credential.AccessToken || refreshTokenEnc == credential.RefreshToken || strings.Contains(accessTokenEnc, "ghu_access") || strings.Contains(refreshTokenEnc, "ghr_refresh") {
		t.Fatalf("plaintext GitHub token material persisted: access=%q refresh=%q", accessTokenEnc, refreshTokenEnc)
	}
	got, err := repo.Get("owner-a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || got.AccessToken != credential.AccessToken || got.RefreshToken != credential.RefreshToken || got.GitHubUserID != 12345 || got.GitHubLogin != "octocat" || got.TokenType != "bearer" || got.Scope != "repo" {
		t.Fatalf("unexpected round trip: %#v", got)
	}
	if got.AccessExpiresAt == nil || got.RefreshExpiresAt == nil || !got.AccessExpiresAt.Equal(accessExpiry) || !got.RefreshExpiresAt.Equal(refreshExpiry) {
		t.Fatalf("unexpected expiry round trip: %#v", got)
	}
}

func TestGitHubAppConnectionRepoIsolatesOwnersAndClear(t *testing.T) {
	repo, database := newGitHubAppConnectionTestRepo(t)
	defer database.Close()
	for owner, login := range map[string]string{"owner-a": "octocat", "owner-b": "hubot"} {
		if err := repo.Save(owner, githubauth.Credential{AccessToken: "token-" + owner, GitHubUserID: 100, GitHubLogin: login, TokenType: "bearer"}); err != nil {
			t.Fatalf("save %s: %v", owner, err)
		}
	}
	if err := repo.Clear("owner-a"); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	first, err := repo.Get("owner-a")
	if err != nil || first != nil {
		t.Fatalf("owner-a after clear = %#v, %v", first, err)
	}
	second, err := repo.Get("owner-b")
	if err != nil || second == nil || second.GitHubLogin != "hubot" {
		t.Fatalf("owner-b was affected by clear: %#v, %v", second, err)
	}
}

func TestGitHubAppConnectionRepoValidatesIdentityAndOwner(t *testing.T) {
	repo, database := newGitHubAppConnectionTestRepo(t)
	defer database.Close()
	valid := githubauth.Credential{AccessToken: "token", GitHubUserID: 1, GitHubLogin: "octocat", TokenType: "bearer"}
	if err := repo.Save("", valid); err == nil {
		t.Fatal("Save() accepted empty owner")
	}
	if err := repo.Save("owner", githubauth.Credential{AccessToken: "token", GitHubLogin: "octocat"}); err == nil {
		t.Fatal("Save() accepted missing GitHub user ID")
	}
	if err := repo.Save("owner", githubauth.Credential{GitHubUserID: 1, GitHubLogin: "octocat"}); err == nil {
		t.Fatal("Save() accepted missing access token")
	}
	if _, err := repo.Get(""); err == nil {
		t.Fatal("Get() accepted empty owner")
	}
	if err := repo.Clear(""); err == nil {
		t.Fatal("Clear() accepted empty owner")
	}
}
