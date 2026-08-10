package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	_ "modernc.org/sqlite"
)

func newMCPOAuthTestRepo(t *testing.T) (*MCPOAuthRepo, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.Exec(`
		CREATE TABLE mcp_oauth_credentials (
			server_id TEXT PRIMARY KEY,
			client_id TEXT NOT NULL DEFAULT '',
			client_secret_enc TEXT NOT NULL DEFAULT '',
			token_endpoint_auth_method TEXT NOT NULL DEFAULT 'none',
			access_token_enc TEXT NOT NULL DEFAULT '',
			refresh_token_enc TEXT NOT NULL DEFAULT '',
			token_type TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT '',
			expires_at DATETIME,
			authorization_server TEXT NOT NULL DEFAULT '',
			authorization_endpoint TEXT NOT NULL DEFAULT '',
			token_endpoint TEXT NOT NULL DEFAULT '',
			resource_metadata_url TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		_ = database.Close()
		t.Fatalf("create OAuth table: %v", err)
	}
	return NewMCPOAuthRepo(database), database
}

func TestMCPOAuthRepoEncryptsSecretsAndTokens(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	secret := "client-secret-value"
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{
		ClientID:                "client-1",
		ClientSecret:            &secret,
		TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretBasic,
	}); err != nil {
		t.Fatalf("configure client: %v", err)
	}
	if err := repo.SaveDiscovery("server-1", "https://auth.example.com", "https://auth.example.com/authorize", "https://auth.example.com/token", "https://mcp.example.com/.well-known/oauth-protected-resource"); err != nil {
		t.Fatalf("save discovery: %v", err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	if err := repo.SaveTokens("server-1", "access-token-value", "refresh-token-value", "Bearer", "tools.read", &expires); err != nil {
		t.Fatalf("save tokens: %v", err)
	}

	var secretEnc, accessEnc, refreshEnc string
	if err := database.QueryRow(`SELECT client_secret_enc, access_token_enc, refresh_token_enc FROM mcp_oauth_credentials WHERE server_id='server-1'`).Scan(&secretEnc, &accessEnc, &refreshEnc); err != nil {
		t.Fatalf("read encrypted values: %v", err)
	}
	for name, value := range map[string]string{"client": secretEnc, "access": accessEnc, "refresh": refreshEnc} {
		if value == "" || strings.Contains(value, "token-value") || value == secret {
			t.Fatalf("%s secret was not encrypted: %q", name, value)
		}
	}

	runtime, err := repo.GetRuntime("server-1")
	if err != nil {
		t.Fatalf("get runtime: %v", err)
	}
	if runtime == nil || runtime.ClientSecret != secret || runtime.AccessToken != "access-token-value" || runtime.RefreshToken != "refresh-token-value" {
		t.Fatalf("unexpected decrypted runtime credential: %#v", runtime)
	}
	status, err := repo.Status("server-1", "http://127.0.0.1:8080/v1/mcp/oauth/callback")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Configured || !status.Connected || !status.HasClientSecret || !status.HasRefreshToken || status.ClientID != "client-1" {
		t.Fatalf("unexpected OAuth status: %#v", status)
	}
}

func TestMCPOAuthRepoPreservesOrClearsClientSecretExplicitly(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	secret := "secret"
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientSecret: &secret, TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretPost}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretPost}); err != nil {
		t.Fatal(err)
	}
	runtime, err := repo.GetRuntime("server-1")
	if err != nil || runtime.ClientSecret != secret {
		t.Fatalf("nil secret update should preserve secret: %#v %v", runtime, err)
	}
	empty := ""
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientSecret: &empty, TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone}); err != nil {
		t.Fatal(err)
	}
	runtime, err = repo.GetRuntime("server-1")
	if err != nil || runtime.ClientSecret != "" {
		t.Fatalf("explicit empty secret should clear secret: %#v %v", runtime, err)
	}
}
