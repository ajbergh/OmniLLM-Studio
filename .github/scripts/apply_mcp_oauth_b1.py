from pathlib import Path


def replace_once(path: str, old: str, new: str, label: str) -> None:
    target = Path(path)
    source = target.read_text()
    if old not in source:
        raise SystemExit(f"{label} marker missing in {path}")
    target.write_text(source.replace(old, new, 1))


# V49: bind preregistered OAuth credentials to their issuer and persist the
# selected registration method. CIMD client IDs are portable and intentionally
# do not acquire issuer binding.
db = Path("backend/internal/db/db.go")
source = db.read_text()
source = source.replace(
    '\t\t{Version: 48, Name: "mcp_oauth_credentials", SQL: migrationMCPOAuthCredentials},\n\t}\n',
    '\t\t{Version: 48, Name: "mcp_oauth_credentials", SQL: migrationMCPOAuthCredentials},\n\t\t{Version: 49, Name: "mcp_oauth_registration_binding", SQL: migrationMCPOAuthRegistrationBinding},\n\t}\n',
    1,
)
if 'const migrationMCPOAuthRegistrationBinding' not in source:
    source += '''

// V49: record how the OAuth client ID is established and, for preregistered
// clients, which authorization-server issuer owns those credentials.
const migrationMCPOAuthRegistrationBinding = `
ALTER TABLE mcp_oauth_credentials ADD COLUMN registration_method TEXT NOT NULL DEFAULT 'preregistered';
ALTER TABLE mcp_oauth_credentials ADD COLUMN client_issuer TEXT NOT NULL DEFAULT '';
`
'''
db.write_text(source)

migration_test = Path("backend/internal/db/agent_runtime_migration_test.go")
source = migration_test.read_text()
source = source.replace('if version != 48 {\n\t\tt.Fatalf("expected schema version 48, got %d", version)\n\t}', 'if version != 49 {\n\t\tt.Fatalf("expected schema version 49, got %d", version)\n\t}', 1)
migration_test.write_text(source)

Path("backend/internal/db/mcp_oauth_registration_migration_test.go").write_text(r'''package db

import (
    "testing"
)

func TestMigrationV49AddsOAuthRegistrationBinding(t *testing.T) {
    database := openMigrationTestDB(t)
    defer database.Close()
    if err := Migrate(database); err != nil {
        t.Fatalf("migrate: %v", err)
    }
    var registrationMethod, clientIssuer string
    if err := database.QueryRow(`SELECT registration_method, client_issuer FROM mcp_oauth_credentials LIMIT 1`).Scan(&registrationMethod, &clientIssuer); err == nil {
        // No rows is the expected normal case; successful scan is also fine.
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
    if !found["registration_method"] || !found["client_issuer"] {
        t.Fatalf("V49 columns missing: %#v", found)
    }
}
''')

# Public models expose registration method and issuer binding without secrets.
Path("backend/internal/models/mcp_oauth.go").write_text(r'''package models

import "time"

const (
    MCPOAuthAuthMethodNone              = "none"
    MCPOAuthAuthMethodClientSecretBasic = "client_secret_basic"
    MCPOAuthAuthMethodClientSecretPost  = "client_secret_post"

    MCPOAuthRegistrationPreregistered = "preregistered"
    MCPOAuthRegistrationCIMD          = "cimd"
)

// MCPOAuthStatus is the non-secret management view of one MCP OAuth connection.
type MCPOAuthStatus struct {
    ServerID                 string     `json:"server_id"`
    Configured               bool       `json:"configured"`
    Connected                bool       `json:"connected"`
    ClientID                 string     `json:"client_id,omitempty"`
    RegistrationMethod       string     `json:"registration_method,omitempty"`
    ClientIssuer             string     `json:"client_issuer,omitempty"`
    HasClientSecret          bool       `json:"has_client_secret"`
    HasRefreshToken          bool       `json:"has_refresh_token"`
    TokenEndpointAuthMethod  string     `json:"token_endpoint_auth_method,omitempty"`
    Scope                    string     `json:"scope,omitempty"`
    ExpiresAt                *time.Time `json:"expires_at,omitempty"`
    AuthorizationServer      string     `json:"authorization_server,omitempty"`
    AuthorizationEndpoint    string     `json:"authorization_endpoint,omitempty"`
    TokenEndpoint            string     `json:"token_endpoint,omitempty"`
    ResourceMetadataURL      string     `json:"resource_metadata_url,omitempty"`
    RedirectURI              string     `json:"redirect_uri,omitempty"`
}

// ConfigureMCPOAuthInput stores OAuth client information. A nil client_secret
// preserves an existing encrypted secret; an explicit empty string clears it.
// CIMD clients use an HTTPS metadata-document URL as client_id and method none.
type ConfigureMCPOAuthInput struct {
    ClientID                string  `json:"client_id"`
    ClientSecret            *string `json:"client_secret,omitempty"`
    TokenEndpointAuthMethod string  `json:"token_endpoint_auth_method"`
    RegistrationMethod      string  `json:"registration_method,omitempty"`
}

// MCPOAuthAuthorizationStart contains browser URL and non-secret discovery data.
type MCPOAuthAuthorizationStart struct {
    AuthorizationURL    string `json:"authorization_url"`
    AuthorizationServer string `json:"authorization_server"`
    RegistrationMethod string `json:"registration_method"`
    Scope               string `json:"scope,omitempty"`
    RedirectURI         string `json:"redirect_uri"`
}
''')

# Repository: registration method + explicit issuer binding for preregistered clients.
repo = Path("backend/internal/repository/mcp_oauth.go")
source = repo.read_text()
source = source.replace(
    '\tResourceMetadataURL     string\n}',
    '\tResourceMetadataURL     string\n\tRegistrationMethod      string\n\tClientIssuer            string\n}',
    1,
)
source = source.replace(
    'func validMCPOAuthMethod(value string) bool {',
    '''func validMCPOAuthRegistrationMethod(value string) bool {
\tswitch value {
\tcase models.MCPOAuthRegistrationPreregistered, models.MCPOAuthRegistrationCIMD:
\t\treturn true
\tdefault:
\t\treturn false
\t}
}

func validMCPOAuthMethod(value string) bool {''',
    1,
)
source = source.replace(
    '\tif !validMCPOAuthMethod(method) {\n\t\treturn fmt.Errorf("unsupported token_endpoint_auth_method %q", method)\n\t}\n',
    '''\tif !validMCPOAuthMethod(method) {
\t\treturn fmt.Errorf("unsupported token_endpoint_auth_method %q", method)
\t}
\tregistrationMethod := strings.TrimSpace(input.RegistrationMethod)
\tif registrationMethod == "" {
\t\tregistrationMethod = models.MCPOAuthRegistrationPreregistered
\t}
\tif !validMCPOAuthRegistrationMethod(registrationMethod) {
\t\treturn fmt.Errorf("unsupported OAuth registration_method %q", registrationMethod)
\t}
''',
    1,
)
# Existing issuer is preserved only when configuration identity is unchanged.
old_secret_block = '''\tsecretEnc := ""
\tif input.ClientSecret == nil {
\t\tvar existing sql.NullString
\t\terr := r.db.QueryRow(`SELECT client_secret_enc FROM mcp_oauth_credentials WHERE server_id = ?`, serverID).Scan(&existing)
\t\tif err != nil && err != sql.ErrNoRows {
\t\t\treturn fmt.Errorf("read existing MCP OAuth client secret: %w", err)
\t\t}
\t\tif existing.Valid {
\t\t\tsecretEnc = existing.String
\t\t}
\t} else if *input.ClientSecret != "" {
'''
new_secret_block = '''\tsecretEnc := ""
\texistingClientID, existingMethod, existingRegistration, existingIssuer := "", "", "", ""
\tvar existingSecret sql.NullString
\texistingErr := r.db.QueryRow(`SELECT client_id, client_secret_enc, token_endpoint_auth_method, registration_method, client_issuer FROM mcp_oauth_credentials WHERE server_id = ?`, serverID).Scan(&existingClientID, &existingSecret, &existingMethod, &existingRegistration, &existingIssuer)
\tif existingErr != nil && existingErr != sql.ErrNoRows {
\t\treturn fmt.Errorf("read existing MCP OAuth client: %w", existingErr)
\t}
\tif input.ClientSecret == nil {
\t\tif existingSecret.Valid {
\t\t\tsecretEnc = existingSecret.String
\t\t}
\t} else if *input.ClientSecret != "" {
'''
if old_secret_block not in source:
    raise SystemExit("repository secret marker missing")
source = source.replace(old_secret_block, new_secret_block, 1)
# Add binding decision before upsert.
marker = '\t_, err := r.db.Exec(`\n\t\tINSERT INTO mcp_oauth_credentials ('
if marker not in source:
    raise SystemExit("repository upsert marker missing")
binding = '''\tconfigurationChanged := existingErr == sql.ErrNoRows || existingClientID != clientID || existingMethod != method || existingRegistration != registrationMethod || input.ClientSecret != nil
\tclientIssuer := existingIssuer
\tif configurationChanged || registrationMethod == models.MCPOAuthRegistrationCIMD {
\t\tclientIssuer = ""
\t}

'''
source = source.replace(marker, binding + marker, 1)
source = source.replace(
    'authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, updated_at\n\t\t) VALUES (?, ?, ?, ?, \'\', \'\', \'\', \'\', NULL, \'\', \'\', \'\', \'\', CURRENT_TIMESTAMP)',
    'authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer, updated_at\n\t\t) VALUES (?, ?, ?, ?, \'\', \'\', \'\', \'\', NULL, \'\', \'\', \'\', \'\', ?, ?, CURRENT_TIMESTAMP)',
    1,
)
source = source.replace(
    '\t\t\ttoken_endpoint_auth_method = excluded.token_endpoint_auth_method,\n\t\t\taccess_token_enc = \'\', refresh_token_enc = \'\', token_type = \'\', scope = \'\', expires_at = NULL,\n\t\t\tupdated_at = CURRENT_TIMESTAMP\n\t`, serverID, clientID, secretEnc, method)',
    '\t\t\ttoken_endpoint_auth_method = excluded.token_endpoint_auth_method,\n\t\t\tregistration_method = excluded.registration_method, client_issuer = excluded.client_issuer,\n\t\t\taccess_token_enc = \'\', refresh_token_enc = \'\', token_type = \'\', scope = \'\', expires_at = NULL,\n\t\t\tupdated_at = CURRENT_TIMESTAMP\n\t`, serverID, clientID, secretEnc, method, registrationMethod, clientIssuer)',
    1,
)
source = source.replace(
    '\t\t\tauthorization_server, authorization_endpoint, token_endpoint, resource_metadata_url\n\t\tFROM mcp_oauth_credentials WHERE server_id = ?',
    '\t\t\tauthorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer\n\t\tFROM mcp_oauth_credentials WHERE server_id = ?',
    1,
)
source = source.replace(
    '\t\t&item.AuthorizationServer, &item.AuthorizationEndpoint, &item.TokenEndpoint, &item.ResourceMetadataURL,\n\t)',
    '\t\t&item.AuthorizationServer, &item.AuthorizationEndpoint, &item.TokenEndpoint, &item.ResourceMetadataURL, &item.RegistrationMethod, &item.ClientIssuer,\n\t)',
    1,
)
source = source.replace(
    '\tstatus.ClientID = credential.ClientID\n',
    '\tstatus.ClientID = credential.ClientID\n\tstatus.RegistrationMethod = credential.RegistrationMethod\n\tstatus.ClientIssuer = credential.ClientIssuer\n',
    1,
)
# Add issuer binding method before SaveDiscovery.
save_marker = '// SaveDiscovery records non-secret endpoints discovered from RFC9728 / RFC8414\n'
if save_marker not in source:
    raise SystemExit("SaveDiscovery marker missing")
bind_method = r'''// BindClientIssuer enforces authorization-server ownership for preregistered
// credentials. CIMD client IDs are self-hosted and portable across issuers.
func (r *MCPOAuthRepo) BindClientIssuer(serverID, issuer string) error {
    issuer = strings.TrimSpace(issuer)
    credential, err := r.GetRuntime(serverID)
    if err != nil {
        return err
    }
    if credential == nil {
        return fmt.Errorf("MCP OAuth client is not configured")
    }
    if credential.RegistrationMethod == models.MCPOAuthRegistrationCIMD {
        return nil
    }
    if credential.ClientIssuer != "" && credential.ClientIssuer != issuer {
        return fmt.Errorf("OAuth client credentials are bound to authorization server %q, not %q", credential.ClientIssuer, issuer)
    }
    if credential.ClientIssuer == issuer {
        return nil
    }
    if _, err := r.db.Exec(`UPDATE mcp_oauth_credentials SET client_issuer = ?, updated_at = CURRENT_TIMESTAMP WHERE server_id = ?`, issuer, serverID); err != nil {
        return fmt.Errorf("bind MCP OAuth client issuer: %w", err)
    }
    return nil
}

'''
source = source.replace(save_marker, bind_method + save_marker, 1)
repo.write_text(source)

# OAuth runtime: current 2026-07-28 metadata capabilities, CIMD validation,
# strict issuer comparison, prereg issuer binding, and RFC9207 callback validation.
oauth = Path("backend/internal/mcpclient/oauth.go")
source = oauth.read_text()
source = source.replace(
    '\tTokenEndpointAuthMethods      []string `json:"token_endpoint_auth_methods_supported"`\n}',
    '\tTokenEndpointAuthMethods                    []string `json:"token_endpoint_auth_methods_supported"`\n\tClientIDMetadataDocumentSupported              bool     `json:"client_id_metadata_document_supported"`\n\tRegistrationEndpoint                           string   `json:"registration_endpoint"`\n\tAuthorizationResponseIssParameterSupported     bool     `json:"authorization_response_iss_parameter_supported"`\n}',
    1,
)
source = source.replace(
    '\tScope                   string\n\tExpiresAt               time.Time\n}',
    '\tScope                   string\n\tExpectedIssuer          string\n\tIssuerParameterRequired bool\n\tExpiresAt               time.Time\n}',
    1,
)
# Configure validates CIMD shape and public-client behavior.
configure_marker = '''\tif method != models.MCPOAuthAuthMethodNone && input.ClientSecret != nil && strings.TrimSpace(*input.ClientSecret) == "" {
\t\treturn fmt.Errorf("client_secret is required for %s", method)
\t}
\treturn s.credentials.ConfigureClient(serverID, input)
}
'''
configure_replacement = '''\tif method != models.MCPOAuthAuthMethodNone && input.ClientSecret != nil && strings.TrimSpace(*input.ClientSecret) == "" {
\t\treturn fmt.Errorf("client_secret is required for %s", method)
\t}
\tregistrationMethod := strings.TrimSpace(input.RegistrationMethod)
\tif registrationMethod == "" {
\t\tregistrationMethod = models.MCPOAuthRegistrationPreregistered
\t\tinput.RegistrationMethod = registrationMethod
\t}
\tif registrationMethod == models.MCPOAuthRegistrationCIMD {
\t\tif err := validateCIMDClientID(input.ClientID); err != nil {
\t\t\treturn err
\t\t}
\t\tif method != models.MCPOAuthAuthMethodNone {
\t\t\treturn fmt.Errorf("CIMD currently requires token_endpoint_auth_method none")
\t\t}
\t\tif input.ClientSecret != nil && strings.TrimSpace(*input.ClientSecret) != "" {
\t\t\treturn fmt.Errorf("CIMD public clients do not use a stored client secret")
\t\t}
\t}
\treturn s.credentials.ConfigureClient(serverID, input)
}

func validateCIMDClientID(raw string) error {
\tparsed, err := url.Parse(strings.TrimSpace(raw))
\tif err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || strings.Trim(parsed.Path, "/") == "" {
\t\treturn fmt.Errorf("CIMD client_id must be an HTTPS metadata-document URL with a path and no query, fragment, or userinfo")
\t}
\treturn nil
}
'''
if configure_marker not in source:
    raise SystemExit("OAuth Configure end marker missing")
source = source.replace(configure_marker, configure_replacement, 1)
# Strict issuer value from PRM; no trailing-slash normalization for RFC8414/RFC9207 identity.
source = source.replace('authorizationServer := normalizeIssuer(resourceMetadata.AuthorizationServers[0])', 'authorizationServer := strings.TrimSpace(resourceMetadata.AuthorizationServers[0])', 1)
source = source.replace('if normalizeIssuer(authMetadata.Issuer) != authorizationServer {', 'if authMetadata.Issuer != authorizationServer {', 1)
# Insert registration checks after auth method check.
method_check = '''\tif !oauthMethodSupported(credential.TokenEndpointAuthMethod, authMetadata.TokenEndpointAuthMethods) {
\t\treturn models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not support token auth method %q", credential.TokenEndpointAuthMethod)
\t}
'''
registration_checks = method_check + '''\tregistrationMethod := credential.RegistrationMethod
\tif registrationMethod == "" {
\t\tregistrationMethod = models.MCPOAuthRegistrationPreregistered
\t}
\tif registrationMethod == models.MCPOAuthRegistrationCIMD {
\t\tif !authMetadata.ClientIDMetadataDocumentSupported {
\t\t\treturn models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not advertise Client ID Metadata Document support")
\t\t}
\t\tif err := validateCIMDClientID(credential.ClientID); err != nil {
\t\t\treturn models.MCPOAuthAuthorizationStart{}, err
\t\t}
\t} else if err := s.credentials.BindClientIssuer(serverID, authMetadata.Issuer); err != nil {
\t\treturn models.MCPOAuthAuthorizationStart{}, err
\t}
'''
if method_check not in source:
    raise SystemExit("OAuth method support marker missing")
source = source.replace(method_check, registration_checks, 1)
source = source.replace(
    '\t\tScope:                   scope,\n\t\tExpiresAt:               time.Now().UTC().Add(mcpOAuthStateTTL),',
    '\t\tScope:                   scope,\n\t\tExpectedIssuer:          authMetadata.Issuer,\n\t\tIssuerParameterRequired: authMetadata.AuthorizationResponseIssParameterSupported,\n\t\tExpiresAt:               time.Now().UTC().Add(mcpOAuthStateTTL),',
    1,
)
source = source.replace(
    '\t\tAuthorizationServer: authorizationServer,\n\t\tScope:               scope,',
    '\t\tAuthorizationServer: authorizationServer,\n\t\tRegistrationMethod: registrationMethod,\n\t\tScope:               scope,',
    1,
)
# CompleteAuthorization now validates RFC9207 before code redemption.
source = source.replace(
    'func (s *OAuthService) CompleteAuthorization(ctx context.Context, state, code string) (string, error) {',
    'func (s *OAuthService) CompleteAuthorization(ctx context.Context, state, code, issuer string) (string, error) {',
    1,
)
consume_block = '''\ts.mu.Lock()
\tpending, ok := s.states[state]
\tdelete(s.states, state)
\ts.mu.Unlock()
\tif !ok || time.Now().UTC().After(pending.ExpiresAt) {
\t\treturn "", fmt.Errorf("OAuth state is invalid or expired")
\t}
'''
consume_replacement = consume_block + '''\tif err := validateAuthorizationResponseIssuer(pending, issuer); err != nil {
\t\treturn "", err
\t}
'''
if consume_block not in source:
    raise SystemExit("OAuth pending state marker missing")
source = source.replace(consume_block, consume_replacement, 1)
# Replace RejectAuthorization with issuer-aware version and helper.
old_reject = '''// RejectAuthorization consumes a pending state after an authorization-server error.
func (s *OAuthService) RejectAuthorization(state string) {
\tstate = strings.TrimSpace(state)
\tif state == "" {
\t\treturn
\t}
\ts.mu.Lock()
\tdelete(s.states, state)
\ts.mu.Unlock()
}
'''
new_reject = '''func validateAuthorizationResponseIssuer(pending oauthPendingState, issuer string) error {
\tissuer = strings.TrimSpace(issuer)
\tif issuer == "" {
\t\tif pending.IssuerParameterRequired {
\t\t\treturn fmt.Errorf("authorization response is missing required issuer")
\t\t}
\t\treturn nil
\t}
\tif issuer != pending.ExpectedIssuer {
\t\treturn fmt.Errorf("authorization response issuer mismatch")
\t}
\treturn nil
}

// RejectAuthorization consumes a pending state after an authorization-server
// error, validating RFC9207 issuer identity before callers display error detail.
func (s *OAuthService) RejectAuthorization(state, issuer string) error {
\tstate = strings.TrimSpace(state)
\tif state == "" {
\t\treturn fmt.Errorf("OAuth state is invalid or expired")
\t}
\ts.mu.Lock()
\tpending, ok := s.states[state]
\tdelete(s.states, state)
\ts.mu.Unlock()
\tif !ok || time.Now().UTC().After(pending.ExpiresAt) {
\t\treturn fmt.Errorf("OAuth state is invalid or expired")
\t}
\treturn validateAuthorizationResponseIssuer(pending, issuer)
}
'''
if old_reject not in source:
    raise SystemExit("OAuth reject marker missing")
source = source.replace(old_reject, new_reject, 1)
oauth.write_text(source)

# Callback validates `iss` on success and errors before surfacing AS details.
handler = Path("backend/internal/api/mcp_oauth_handler.go")
source = handler.read_text()
source = source.replace(
    '''\tif oauthError := strings.TrimSpace(r.URL.Query().Get("error")); oauthError != "" {
\t\th.oauth.RejectAuthorization(state)
\t\tdescription := strings.TrimSpace(r.URL.Query().Get("error_description"))
''',
    '''\tissuer := strings.TrimSpace(r.URL.Query().Get("iss"))
\tif oauthError := strings.TrimSpace(r.URL.Query().Get("error")); oauthError != "" {
\t\tif err := h.oauth.RejectAuthorization(state, issuer); err != nil {
\t\t\th.writeCallbackPage(w, false, "Authorization response validation failed.")
\t\t\treturn
\t\t}
\t\tdescription := strings.TrimSpace(r.URL.Query().Get("error_description"))
''',
    1,
)
source = source.replace(
    'serverID, err := h.oauth.CompleteAuthorization(r.Context(), state, r.URL.Query().Get("code"))',
    'serverID, err := h.oauth.CompleteAuthorization(r.Context(), state, r.URL.Query().Get("code"), issuer)',
    1,
)
handler.write_text(source)

# Repository tests use the V49 shape and verify issuer binding/CIMD portability.
repo_test = Path("backend/internal/repository/mcp_oauth_test.go")
source = repo_test.read_text()
source = source.replace(
    '\t\t\tresource_metadata_url TEXT NOT NULL DEFAULT \'\',\n\t\t\tupdated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP',
    '\t\t\tresource_metadata_url TEXT NOT NULL DEFAULT \'\',\n\t\t\tregistration_method TEXT NOT NULL DEFAULT \'preregistered\',\n\t\t\tclient_issuer TEXT NOT NULL DEFAULT \'\',\n\t\t\tupdated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP',
    1,
)
source += r'''

func TestMCPOAuthRepoBindsPreregisteredClientToIssuer(t *testing.T) {
    repo, database := newMCPOAuthTestRepo(t)
    defer database.Close()
    if err := repo.ConfigureClient("server-1", models.ConfigureMCPOAuthInput{ClientID: "client", TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone, RegistrationMethod: models.MCPOAuthRegistrationPreregistered}); err != nil {
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
'''
repo_test.write_text(source)

# OAuth unit tests for CIMD and RFC9207 simple-string issuer semantics.
oauth_test = Path("backend/internal/mcpclient/oauth_test.go")
source = oauth_test.read_text()
source += r'''

func TestValidateCIMDClientID(t *testing.T) {
    if err := validateCIMDClientID("https://client.example/oauth/metadata.json"); err != nil {
        t.Fatalf("valid CIMD client ID rejected: %v", err)
    }
    for _, raw := range []string{"http://client.example/metadata.json", "https://client.example", "https://client.example/metadata.json?x=1", "https://client.example/metadata.json#fragment"} {
        if err := validateCIMDClientID(raw); err == nil {
            t.Fatalf("invalid CIMD client ID accepted: %q", raw)
        }
    }
}

func TestAuthorizationResponseIssuerValidation(t *testing.T) {
    pending := oauthPendingState{ExpectedIssuer: "https://auth.example/tenant", IssuerParameterRequired: true}
    if err := validateAuthorizationResponseIssuer(pending, "https://auth.example/tenant"); err != nil {
        t.Fatalf("matching issuer rejected: %v", err)
    }
    if err := validateAuthorizationResponseIssuer(pending, ""); err == nil {
        t.Fatal("required issuer omission accepted")
    }
    if err := validateAuthorizationResponseIssuer(pending, "https://auth.example/tenant/"); err == nil {
        t.Fatal("RFC9207 simple-string mismatch accepted")
    }
    pending.IssuerParameterRequired = false
    if err := validateAuthorizationResponseIssuer(pending, ""); err != nil {
        t.Fatalf("optional absent issuer should be accepted: %v", err)
    }
}
'''
oauth_test.write_text(source)

# Frontend API models.
Path("frontend/src/mcpOAuthApi.ts").write_text(r'''import { getAuthToken, resolveApiUrl } from './api';

export type MCPOAuthAuthMethod = 'none' | 'client_secret_basic' | 'client_secret_post';
export type MCPOAuthRegistrationMethod = 'preregistered' | 'cimd';

export interface MCPOAuthStatus {
  server_id: string;
  configured: boolean;
  connected: boolean;
  client_id?: string;
  registration_method?: MCPOAuthRegistrationMethod;
  client_issuer?: string;
  has_client_secret: boolean;
  has_refresh_token: boolean;
  token_endpoint_auth_method?: MCPOAuthAuthMethod;
  scope?: string;
  expires_at?: string;
  authorization_server?: string;
  authorization_endpoint?: string;
  token_endpoint?: string;
  resource_metadata_url?: string;
  redirect_uri?: string;
}

export interface ConfigureMCPOAuthInput {
  client_id: string;
  client_secret?: string;
  token_endpoint_auth_method: MCPOAuthAuthMethod;
  registration_method: MCPOAuthRegistrationMethod;
}

export interface MCPOAuthAuthorizationStart {
  authorization_url: string;
  authorization_server: string;
  registration_method: MCPOAuthRegistrationMethod;
  scope?: string;
  redirect_uri: string;
}

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(resolveApiUrl(path), {
    ...init,
    credentials: 'include',
    headers: { ...authHeaders(), ...(init?.headers || {}) },
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
  return body as T;
}

export const mcpOAuthApi = {
  status: (serverId: string) => request<MCPOAuthStatus>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`),
  configure: (serverId: string, input: ConfigureMCPOAuthInput) =>
    request<MCPOAuthStatus>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  start: (serverId: string) =>
    request<MCPOAuthAuthorizationStart>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth/start`, { method: 'POST' }),
  disconnect: (serverId: string) =>
    request<void>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`, { method: 'DELETE' }),
};
''')

# Settings UI: explicit preregistered vs CIMD strategy.
panel = Path("frontend/src/components/MCPAuthorizationPanel.tsx")
source = panel.read_text()
source = source.replace(
    "import { mcpOAuthApi, type MCPOAuthAuthMethod, type MCPOAuthStatus } from '../mcpOAuthApi';",
    "import { mcpOAuthApi, type MCPOAuthAuthMethod, type MCPOAuthRegistrationMethod, type MCPOAuthStatus } from '../mcpOAuthApi';",
    1,
)
source = source.replace(
    "  const [authMethod, setAuthMethod] = useState<MCPOAuthAuthMethod>('none');\n",
    "  const [authMethod, setAuthMethod] = useState<MCPOAuthAuthMethod>('none');\n  const [registrationMethod, setRegistrationMethod] = useState<MCPOAuthRegistrationMethod>('preregistered');\n",
    1,
)
source = source.replace(
    "      setAuthMethod(next.token_endpoint_auth_method || 'none');\n",
    "      setAuthMethod(next.token_endpoint_auth_method || 'none');\n      setRegistrationMethod(next.registration_method || 'preregistered');\n",
    1,
)
source = source.replace(
    "      token_endpoint_auth_method: authMethod,\n",
    "      token_endpoint_auth_method: registrationMethod === 'cimd' ? 'none' : authMethod,\n      registration_method: registrationMethod,\n",
    1,
)
source = source.replace(
    "    if (clientSecret !== '') payload.client_secret = clientSecret;\n",
    "    if (registrationMethod === 'preregistered' && clientSecret !== '') payload.client_secret = clientSecret;\n",
    1,
)
# Replace first form grid with registration selector and conditional controls.
old_grid = '''      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">Preregistered client ID</span>
          <input value={clientId} onChange={(event) => setClientId(event.target.value)} placeholder="oauth-client-id" className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
        </label>
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">Token endpoint authentication</span>
          <select value={authMethod} onChange={(event) => setAuthMethod(event.target.value as MCPOAuthAuthMethod)} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text">
            <option value="none">Public client (none)</option>
            <option value="client_secret_basic">client_secret_basic</option>
            <option value="client_secret_post">client_secret_post</option>
          </select>
        </label>
      </div>

      <label className="mt-3 block">
        <span className="mb-1 block text-[10px] font-medium text-text-muted">Client secret {status?.has_client_secret ? '(stored encrypted; leave blank to keep)' : '(optional for public clients)'}</span>
        <input type="password" value={clientSecret} onChange={(event) => setClientSecret(event.target.value)} autoComplete="new-password" className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
      </label>
'''
new_grid = '''      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">Client registration</span>
          <select value={registrationMethod} onChange={(event) => { const next = event.target.value as MCPOAuthRegistrationMethod; setRegistrationMethod(next); if (next === 'cimd') { setAuthMethod('none'); setClientSecret(''); } }} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text">
            <option value="preregistered">Preregistered client</option>
            <option value="cimd">Client ID Metadata Document (CIMD)</option>
          </select>
        </label>
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">{registrationMethod === 'cimd' ? 'Client metadata document URL' : 'Preregistered client ID'}</span>
          <input value={clientId} onChange={(event) => setClientId(event.target.value)} placeholder={registrationMethod === 'cimd' ? 'https://client.example/oauth/metadata.json' : 'oauth-client-id'} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
        </label>
      </div>

      {registrationMethod === 'cimd' ? (
        <div className="mt-3 rounded-xl border border-sky-500/20 bg-sky-500/5 p-3 text-[10px] leading-relaxed text-text-muted">
          CIMD is the preferred MCP registration method when client and authorization server have no prior relationship. The URL must be a stable public HTTPS document with a path; its <span className="font-mono">client_id</span> must exactly match the URL and include this redirect URI in <span className="font-mono">redirect_uris</span>.
        </div>
      ) : (
        <>
          <label className="mt-3 block">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Token endpoint authentication</span>
            <select value={authMethod} onChange={(event) => setAuthMethod(event.target.value as MCPOAuthAuthMethod)} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text">
              <option value="none">Public client (none)</option>
              <option value="client_secret_basic">client_secret_basic</option>
              <option value="client_secret_post">client_secret_post</option>
            </select>
          </label>
          <label className="mt-3 block">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Client secret {status?.has_client_secret ? '(stored encrypted; leave blank to keep)' : '(optional for public clients)'}</span>
            <input type="password" value={clientSecret} onChange={(event) => setClientSecret(event.target.value)} autoComplete="new-password" className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
          </label>
        </>
      )}
'''
if old_grid not in source:
    raise SystemExit("frontend OAuth form marker missing")
source = source.replace(old_grid, new_grid, 1)
# Clear-secret payload requires registration_method.
source = source.replace(
    "        token_endpoint_auth_method: authMethod,\n      });",
    "        token_endpoint_auth_method: authMethod,\n        registration_method: 'preregistered',\n      });",
    1,
)
# Show method and issuer binding in status details.
source = source.replace(
    '<div className="break-all"><span className="font-medium text-text-secondary">Authorization server:</span> {status.authorization_server}</div>',
    '<div className="break-all"><span className="font-medium text-text-secondary">Authorization server:</span> {status.authorization_server}</div>\n          <div className="mt-1"><span className="font-medium text-text-secondary">Registration:</span> {status.registration_method === \'cimd\' ? \'Client ID Metadata Document\' : \'Preregistered client\'}</div>\n          {status.client_issuer && <div className="mt-1 break-all"><span className="font-medium text-text-secondary">Client issuer binding:</span> {status.client_issuer}</div>}',
    1,
)
panel.write_text(source)

# Documentation is updated from Phase A follow-on language to current Phase B1.
doc = Path("docs/MCP_OAUTH_2026-08.md")
source = doc.read_text()
source = source.replace(
    'OmniLLM-Studio supports standards-aligned OAuth authorization for remote HTTP MCP servers using a preregistered OAuth client.',
    'OmniLLM-Studio supports standards-aligned OAuth authorization for remote HTTP MCP servers using preregistered OAuth clients or Client ID Metadata Documents (CIMD).',
    1,
)
source = source.replace(
    '## Preregistered clients\n\nPhase A intentionally supports preregistered client IDs rather than guessing dynamic-registration behavior. Configure the client ID and token endpoint authentication method in **Settings → MCP → OAuth 2.1 authorization**. Confidential clients may store a client secret; it is encrypted and never returned by the API.',
    '''## Client registration\n\nThe current MCP specification prioritizes preregistered credentials when available, then Client ID Metadata Documents (CIMD). DCR is deprecated and remains a later fallback for older authorization servers. Configure the registration method in **Settings → MCP → OAuth 2.1 authorization**.\n\nPreregistered credentials are bound to the exact authorization-server issuer that first validates them. If Protected Resource Metadata later points to a different issuer, Omni rejects reuse instead of silently sending credentials to the new server. CIMD client IDs are HTTPS metadata-document URLs and remain issuer-portable by design.''',
    1,
)
source = source.replace(
    '3. Verify the discovered issuer and require PKCE `S256`.',
    '3. Verify the discovered issuer with exact RFC 8414 comparison, require PKCE `S256`, and validate RFC 9207 `iss` on authorization responses when present/advertised.',
    1,
)
source = source.replace(
    '## Deliberate follow-on work\n\nClient ID Metadata Documents / dynamic client registration and incremental authorization-scope UX are separate follow-on phases. Keeping them separate makes the initial authorization boundary easier to review and test.',
    '## Deliberate follow-on work\n\nDeprecated Dynamic Client Registration fallback and persistent incremental authorization-scope step-up UX remain Phase B2. They are kept separate from issuer/CIMD correctness so the registration trust boundary stays reviewable.',
    1,
)
doc.write_text(source)
