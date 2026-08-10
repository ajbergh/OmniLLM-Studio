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
			registration_method TEXT NOT NULL DEFAULT 'preregistered',
			client_issuer TEXT NOT NULL DEFAULT '',
			required_scope TEXT NOT NULL DEFAULT '',
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
		ClientIssuer:            "https://auth.example.com",
		TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretBasic,
		RegistrationMethod:      models.MCPOAuthRegistrationPreregistered,
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
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientSecret: &secret, ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretPost, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretPost, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
		t.Fatal(err)
	}
	runtime, err := repo.GetRuntime("server-1")
	if err != nil || runtime.ClientSecret != secret {
		t.Fatalf("nil secret update should preserve secret: %#v %v", runtime, err)
	}
	empty := ""
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientSecret: &empty, ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
		t.Fatal(err)
	}
	runtime, err = repo.GetRuntime("server-1")
	if err != nil || runtime.ClientSecret != "" {
		t.Fatalf("explicit empty secret should clear secret: %#v %v", runtime, err)
	}
}

func TestMCPOAuthRepoVerifiesPreregisteredClientIssuer(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindClientIssuer("server-1", "https://issuer.example"); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindClientIssuer("server-1", "https://other.example"); err == nil {
		t.Fatal("expected issuer mismatch to be rejected")
	}
	status, err := repo.Status("server-1", "http://127.0.0.1/callback")
	if err != nil || status.ClientIssuer != "https://issuer.example" || status.RegistrationMethod != models.MCPOAuthRegistrationPreregistered {
		t.Fatalf("unexpected binding status: %#v %v", status, err)
	}
}

func TestMCPOAuthRepoRejectsMissingPreregisteredIssuerAndSecret(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err == nil || !strings.Contains(err.Error(), "client_issuer") {
		t.Fatalf("expected missing issuer rejection, got %v", err)
	}
	if err := repo.ConfigureClient("server-2", models.ConfigureMCPOAuthInput{ClientID: "client", ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodClientSecretBasic, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err == nil || !strings.Contains(err.Error(), "client_secret") {
		t.Fatalf("expected missing secret rejection, got %v", err)
	}
}

func TestMCPOAuthRepoDoesNotBindCIMDClientToIssuer(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "https://client.example/metadata.json", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationCIMD}); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindClientIssuer("server-1", "https://issuer-a.example"); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindClientIssuer("server-1", "https://issuer-b.example"); err != nil {
		t.Fatalf("CIMD should remain issuer-portable: %v", err)
	}
}

func TestMCPOAuthRepoClearDynamicClientOnlyDeletesDCR(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	if err := repo.ConfigureDynamicClient("dcr-server", "https://issuer.example", "dynamic-client"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureClient("preregistered-server", models.ConfigureMCPOAuthInput{ClientID: "client", ClientIssuer: "https://issuer.example", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
		t.Fatal(err)
	}
	cleared, err := repo.ClearDynamicClient("dcr-server")
	if err != nil || !cleared {
		t.Fatalf("clear dynamic client: cleared=%v err=%v", cleared, err)
	}
	if runtime, err := repo.GetRuntime("dcr-server"); err != nil || runtime != nil {
		t.Fatalf("dynamic client still present: %#v %v", runtime, err)
	}
	cleared, err = repo.ClearDynamicClient("preregistered-server")
	if err != nil || cleared {
		t.Fatalf("preregistered client must not be cleared: cleared=%v err=%v", cleared, err)
	}
	if runtime, err := repo.GetRuntime("preregistered-server"); err != nil || runtime == nil {
		t.Fatalf("preregistered client was removed: %#v %v", runtime, err)
	}
}

func TestMCPOAuthRepoDynamicClientAndRequiredScope(t *testing.T) {
	repo, database := newMCPOAuthTestRepo(t)
	defer database.Close()
	if err := repo.ConfigureDynamicClient("server-1", "https://issuer.example", "dynamic-client"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRequiredScope("server-1", "files.read files.write"); err != nil {
		t.Fatal(err)
	}
	status, err := repo.Status("server-1", "http://127.0.0.1/callback")
	if err != nil {
		t.Fatal(err)
	}
	if status.RegistrationMethod != models.MCPOAuthRegistrationDCR || status.ClientIssuer != "https://issuer.example" || status.RequiredScope != "files.read files.write" {
		t.Fatalf("unexpected dynamic OAuth status: %#v", status)
	}
	if err := repo.SaveTokens("server-1", "access", "", "Bearer", "files.read files.write", nil); err != nil {
		t.Fatal(err)
	}
	status, err = repo.Status("server-1", "http://127.0.0.1/callback")
	if err != nil || status.RequiredScope != "" {
		t.Fatalf("successful grant should clear required scope: %#v %v", status, err)
	}
}
