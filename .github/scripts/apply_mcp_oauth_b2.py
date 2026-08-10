from pathlib import Path


def replace_once(path: str, old: str, new: str, label: str) -> None:
    target = Path(path)
    source = target.read_text()
    if old not in source:
        raise SystemExit(f"{label} marker missing in {path}")
    target.write_text(source.replace(old, new, 1))


# ---------------------------------------------------------------------------
# V50: persist an incremental authorization scope challenge until a successful
# authorization-code exchange grants the requested scope set.
# ---------------------------------------------------------------------------
db = Path("backend/internal/db/db.go")
source = db.read_text()
source = source.replace(
    '\t\t{Version: 49, Name: "mcp_oauth_registration_binding", SQL: migrationMCPOAuthRegistrationBinding},\n\t}\n',
    '\t\t{Version: 49, Name: "mcp_oauth_registration_binding", SQL: migrationMCPOAuthRegistrationBinding},\n\t\t{Version: 50, Name: "mcp_oauth_incremental_scope", SQL: migrationMCPOAuthIncrementalScope},\n\t}\n',
    1,
)
if 'const migrationMCPOAuthIncrementalScope' not in source:
    source += '''

// V50: persist authorization step-up requirements discovered from
// WWW-Authenticate insufficient_scope challenges.
const migrationMCPOAuthIncrementalScope = `
ALTER TABLE mcp_oauth_credentials ADD COLUMN required_scope TEXT NOT NULL DEFAULT '';
`
'''
db.write_text(source)

replace_once(
    "backend/internal/db/agent_runtime_migration_test.go",
    'if version != 49 {\n\t\tt.Fatalf("expected schema version 49, got %d", version)\n\t}',
    'if version != 50 {\n\t\tt.Fatalf("expected schema version 50, got %d", version)\n\t}',
    "schema version 50",
)

Path("backend/internal/db/mcp_oauth_scope_migration_test.go").write_text(r'''package db

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
''')

# ---------------------------------------------------------------------------
# Models: DCR is a persisted backend-generated registration method; required
# scope is exposed but never token material.
# ---------------------------------------------------------------------------
models = Path("backend/internal/models/mcp_oauth.go")
source = models.read_text()
source = source.replace(
    '\tMCPOAuthRegistrationPreregistered = "preregistered"\n\tMCPOAuthRegistrationCIMD          = "cimd"\n',
    '\tMCPOAuthRegistrationPreregistered = "preregistered"\n\tMCPOAuthRegistrationCIMD          = "cimd"\n\tMCPOAuthRegistrationDCR           = "dcr"\n',
    1,
)
source = source.replace(
    '\tScope                   string     `json:"scope,omitempty"`\n',
    '\tScope                   string     `json:"scope,omitempty"`\n\tRequiredScope           string     `json:"required_scope,omitempty"`\n',
    1,
)
models.write_text(source)

# ---------------------------------------------------------------------------
# Repository: persist DCR-generated public client IDs per issuer and the
# step-up scope union. Successful token writes clear the pending step-up.
# ---------------------------------------------------------------------------
repo = Path("backend/internal/repository/mcp_oauth.go")
source = repo.read_text()
source = source.replace('\tScope                   string\n', '\tScope                   string\n\tRequiredScope           string\n', 1)
source = source.replace(
    'case models.MCPOAuthRegistrationPreregistered, models.MCPOAuthRegistrationCIMD:',
    'case models.MCPOAuthRegistrationPreregistered, models.MCPOAuthRegistrationCIMD, models.MCPOAuthRegistrationDCR:',
    1,
)
source = source.replace(
    'authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer\n\t\tFROM mcp_oauth_credentials WHERE server_id = ?',
    'authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer, required_scope\n\t\tFROM mcp_oauth_credentials WHERE server_id = ?',
    1,
)
source = source.replace(
    '&item.AuthorizationServer, &item.AuthorizationEndpoint, &item.TokenEndpoint, &item.ResourceMetadataURL, &item.RegistrationMethod, &item.ClientIssuer,\n\t)',
    '&item.AuthorizationServer, &item.AuthorizationEndpoint, &item.TokenEndpoint, &item.ResourceMetadataURL, &item.RegistrationMethod, &item.ClientIssuer, &item.RequiredScope,\n\t)',
    1,
)
source = source.replace('\tstatus.Scope = credential.Scope\n', '\tstatus.Scope = credential.Scope\n\tstatus.RequiredScope = credential.RequiredScope\n', 1)
# User-driven client configuration always resets a stale step-up requirement.
source = source.replace(
    "access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', expires_at = NULL,\n\t\t\tupdated_at = CURRENT_TIMESTAMP",
    "access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', required_scope = '', expires_at = NULL,\n\t\t\tupdated_at = CURRENT_TIMESTAMP",
    1,
)
# Add backend-only dynamic registration persistence before decryptOptional.
marker = 'func decryptOptional(value string) (string, error) {'
if marker not in source:
    raise SystemExit('repository decrypt marker missing')
dcr_repo = r'''// ConfigureDynamicClient stores an automatically registered public DCR client.
// DCR credentials are always bound to the exact authorization-server issuer.
func (r *MCPOAuthRepo) ConfigureDynamicClient(serverID, issuer, clientID string) error {
    issuer = strings.TrimSpace(issuer)
    clientID = strings.TrimSpace(clientID)
    if issuer == "" || clientID == "" {
        return fmt.Errorf("issuer and client_id are required for dynamic registration")
    }
    _, err := r.db.Exec(`
        INSERT INTO mcp_oauth_credentials (
            server_id, client_id, client_secret_enc, token_endpoint_auth_method,
            access_token_enc, refresh_token_enc, token_type, scope, expires_at,
            authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url,
            registration_method, client_issuer, required_scope, updated_at
        ) VALUES (?, ?, '', ?, '', '', '', '', NULL, '', '', '', '', ?, ?, '', CURRENT_TIMESTAMP)
        ON CONFLICT(server_id) DO UPDATE SET
            client_id = excluded.client_id,
            client_secret_enc = '',
            token_endpoint_auth_method = excluded.token_endpoint_auth_method,
            registration_method = excluded.registration_method,
            client_issuer = excluded.client_issuer,
            access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', required_scope = '', expires_at = NULL,
            updated_at = CURRENT_TIMESTAMP
    `, serverID, clientID, models.MCPOAuthAuthMethodNone, models.MCPOAuthRegistrationDCR, issuer)
    if err != nil {
        return fmt.Errorf("save dynamically registered MCP OAuth client: %w", err)
    }
    return nil
}

'''
source = source.replace(marker, dcr_repo + marker, 1)
# SaveTokens clears required_scope on successful grant/refresh.
source = source.replace(
    'SET access_token_enc = ?, token_type = ?, scope = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP',
    "SET access_token_enc = ?, token_type = ?, scope = ?, required_scope = '', expires_at = ?, updated_at = CURRENT_TIMESTAMP",
    1,
)
source = source.replace(
    'SET access_token_enc = ?, refresh_token_enc = ?, token_type = ?, scope = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP',
    "SET access_token_enc = ?, refresh_token_enc = ?, token_type = ?, scope = ?, required_scope = '', expires_at = ?, updated_at = CURRENT_TIMESTAMP",
    1,
)
# ClearTokens also clears any pending step-up requirement.
source = source.replace(
    "SET access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', expires_at = NULL, updated_at = CURRENT_TIMESTAMP",
    "SET access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', required_scope = '', expires_at = NULL, updated_at = CURRENT_TIMESTAMP",
    1,
)
# Persistent step-up setter.
source += r'''

// SaveRequiredScope persists the complete scope set needed for the next
// authorization-code step-up. Callers pass a normalized union.
func (r *MCPOAuthRepo) SaveRequiredScope(serverID, scope string) error {
    if _, err := r.db.Exec(`UPDATE mcp_oauth_credentials SET required_scope = ?, updated_at = CURRENT_TIMESTAMP WHERE server_id = ?`, strings.TrimSpace(scope), serverID); err != nil {
        return fmt.Errorf("save MCP OAuth required scope: %w", err)
    }
    return nil
}
'''
repo.write_text(source)

# Repository test schema follows V50 and verifies DCR/step-up persistence.
repo_test = Path("backend/internal/repository/mcp_oauth_test.go")
source = repo_test.read_text()
source = source.replace(
    "\t\t\tclient_issuer TEXT NOT NULL DEFAULT '',\n\t\t\tupdated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
    "\t\t\tclient_issuer TEXT NOT NULL DEFAULT '',\n\t\t\trequired_scope TEXT NOT NULL DEFAULT '',\n\t\t\tupdated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
    1,
)
source += r'''

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
'''
repo_test.write_text(source)

# ---------------------------------------------------------------------------
# OAuth service: discover first, then automatically use DCR only as the
# deprecated fallback when no client is configured. Persist and union step-up
# challenges with the previously granted scope before reauthorization.
# ---------------------------------------------------------------------------
oauth = Path("backend/internal/mcpclient/oauth.go")
source = oauth.read_text()
source = source.replace(
    'ErrMCPOAuthRequired      = errors.New("MCP OAuth authorization is required")\n',
    'ErrMCPOAuthRequired          = errors.New("MCP OAuth authorization is required")\n\tErrMCPOAuthInsufficientScope = errors.New("MCP OAuth additional authorization scope is required")\n',
    1,
)
# User configuration cannot claim the backend-generated DCR method.
config_guard = '''\tif registrationMethod == models.MCPOAuthRegistrationCIMD {
'''
if config_guard not in source:
    raise SystemExit('OAuth registration guard marker missing')
source = source.replace(config_guard, '''\tif registrationMethod == models.MCPOAuthRegistrationDCR {
\t\treturn fmt.Errorf("dynamic OAuth registration is managed automatically")
\t}
\tif registrationMethod == models.MCPOAuthRegistrationCIMD {
''', 1)
# Replace StartAuthorization in full to permit discovery before client choice.
start = source.index('func (s *OAuthService) StartAuthorization(')
end = source.index('// CompleteAuthorization', start)
new_start = r'''func (s *OAuthService) StartAuthorization(ctx context.Context, serverID, userID string) (models.MCPOAuthAuthorizationStart, error) {
    server, err := s.servers.GetRuntimeByID(serverID)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    if server == nil || server.Transport != "http" || server.URL == nil {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("OAuth requires an HTTP MCP server")
    }

    resourceURI, err := canonicalResourceURI(*server.URL)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    resourceMetadata, resourceMetadataURL, challengedScopes, err := discoverProtectedResource(ctx, *server, resourceURI)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    if len(resourceMetadata.AuthorizationServers) == 0 {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("protected resource metadata did not advertise an authorization server")
    }
    authorizationServer := strings.TrimSpace(resourceMetadata.AuthorizationServers[0])
    authMetadata, err := discoverAuthorizationServer(ctx, *server, authorizationServer)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    if authMetadata.Issuer != authorizationServer {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization metadata issuer mismatch")
    }
    if !containsString(authMetadata.CodeChallengeMethodsSupported, "S256") {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not advertise PKCE S256")
    }

    credential, err := s.credentials.GetRuntime(serverID)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    if credential == nil || strings.TrimSpace(credential.ClientID) == "" {
        if strings.TrimSpace(authMetadata.RegistrationEndpoint) == "" {
            return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("%w: configure a preregistered/CIMD client; authorization server does not advertise legacy DCR", ErrMCPOAuthNotConfigured)
        }
        if err := s.registerDynamicClient(ctx, *server, authMetadata, serverID); err != nil {
            return models.MCPOAuthAuthorizationStart{}, err
        }
        credential, err = s.credentials.GetRuntime(serverID)
        if err != nil || credential == nil {
            return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("load dynamically registered OAuth client: %w", err)
        }
    }

    if !oauthMethodSupported(credential.TokenEndpointAuthMethod, authMetadata.TokenEndpointAuthMethods) {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not support token auth method %q", credential.TokenEndpointAuthMethod)
    }
    registrationMethod := credential.RegistrationMethod
    if registrationMethod == "" {
        registrationMethod = models.MCPOAuthRegistrationPreregistered
    }
    if registrationMethod == models.MCPOAuthRegistrationCIMD {
        if !authMetadata.ClientIDMetadataDocumentSupported {
            return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not advertise Client ID Metadata Document support")
        }
        if err := validateCIMDClientID(credential.ClientID); err != nil {
            return models.MCPOAuthAuthorizationStart{}, err
        }
    } else if err := s.credentials.BindClientIssuer(serverID, authMetadata.Issuer); err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    if err := validateOAuthEndpoint(authMetadata.AuthorizationEndpoint); err != nil {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization endpoint: %w", err)
    }
    if err := validateOAuthEndpoint(authMetadata.TokenEndpoint); err != nil {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("token endpoint: %w", err)
    }

    scopes := append([]string{}, challengedScopes...)
    if credential.RequiredScope != "" {
        scopes = append(scopes, strings.Fields(credential.Scope)...)
        scopes = append(scopes, strings.Fields(credential.RequiredScope)...)
    }
    if len(scopes) == 0 {
        scopes = append(scopes, resourceMetadata.ScopesSupported...)
    }
    if len(scopes) == 0 {
        scopes = append(scopes, authMetadata.ScopesSupported...)
    }
    scopes = uniqueSortedStrings(scopes)
    scope := strings.Join(scopes, " ")

    state, err := randomURLSafe(32)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("generate OAuth state: %w", err)
    }
    verifier, err := randomURLSafe(48)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("generate PKCE verifier: %w", err)
    }

    authURL, err := url.Parse(authMetadata.AuthorizationEndpoint)
    if err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    query := authURL.Query()
    query.Set("response_type", "code")
    query.Set("client_id", credential.ClientID)
    query.Set("redirect_uri", s.redirectURI)
    query.Set("state", state)
    query.Set("code_challenge", codeChallenge(verifier))
    query.Set("code_challenge_method", "S256")
    query.Set("resource", resourceURI)
    if scope != "" {
        query.Set("scope", scope)
    }
    authURL.RawQuery = query.Encode()

    s.mu.Lock()
    s.pruneExpiredStatesLocked()
    s.states[state] = oauthPendingState{
        ServerID:                serverID,
        UserID:                  userID,
        CodeVerifier:            verifier,
        RedirectURI:             s.redirectURI,
        ResourceURI:             resourceURI,
        TokenEndpoint:           authMetadata.TokenEndpoint,
        ClientID:                credential.ClientID,
        TokenEndpointAuthMethod: credential.TokenEndpointAuthMethod,
        Scope:                   scope,
        ExpectedIssuer:          authMetadata.Issuer,
        IssuerParameterRequired: authMetadata.AuthorizationResponseIssParameterSupported,
        ExpiresAt:               time.Now().UTC().Add(mcpOAuthStateTTL),
    }
    s.mu.Unlock()

    if err := s.credentials.SaveDiscovery(serverID, authorizationServer, authMetadata.AuthorizationEndpoint, authMetadata.TokenEndpoint, resourceMetadataURL); err != nil {
        return models.MCPOAuthAuthorizationStart{}, err
    }
    return models.MCPOAuthAuthorizationStart{
        AuthorizationURL:    authURL.String(),
        AuthorizationServer: authorizationServer,
        RegistrationMethod:  registrationMethod,
        Scope:               scope,
        RedirectURI:         s.redirectURI,
    }, nil
}

'''
source = source[:start] + new_start + source[end:]
# DCR wire helpers before CompleteAuthorization.
insert = source.index('// CompleteAuthorization')
dcr_helpers = r'''type dynamicClientRegistrationRequest struct {
    ClientName              string   `json:"client_name"`
    RedirectURIs            []string `json:"redirect_uris"`
    GrantTypes              []string `json:"grant_types"`
    ResponseTypes           []string `json:"response_types"`
    TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
    ApplicationType         string   `json:"application_type"`
}

type dynamicClientRegistrationResponse struct {
    ClientID                string `json:"client_id"`
    TokenEndpointAuthMethod string `json:"token_endpoint_auth_method"`
}

func oauthApplicationType(redirectURI string) string {
    parsed, err := url.Parse(redirectURI)
    if err == nil && isLoopbackHostname(parsed.Hostname()) {
        return "native"
    }
    return "web"
}

func (s *OAuthService) registerDynamicClient(ctx context.Context, server models.MCPServer, metadata authorizationServerMetadata, serverID string) error {
    endpoint := strings.TrimSpace(metadata.RegistrationEndpoint)
    if endpoint == "" {
        return fmt.Errorf("authorization server does not advertise a dynamic registration endpoint")
    }
    response, err := registerDynamicPublicClient(ctx, newHTTPTransportClient(server), endpoint, s.redirectURI)
    if err != nil {
        return err
    }
    return s.credentials.ConfigureDynamicClient(serverID, metadata.Issuer, response.ClientID)
}

func registerDynamicPublicClient(ctx context.Context, client *http.Client, endpoint, redirectURI string) (dynamicClientRegistrationResponse, error) {
    if err := validateOAuthEndpoint(endpoint); err != nil {
        return dynamicClientRegistrationResponse{}, fmt.Errorf("dynamic registration endpoint: %w", err)
    }
    payload := dynamicClientRegistrationRequest{
        ClientName:              "OmniLLM Studio",
        RedirectURIs:            []string{redirectURI},
        GrantTypes:              []string{"authorization_code", "refresh_token"},
        ResponseTypes:           []string{"code"},
        TokenEndpointAuthMethod: models.MCPOAuthAuthMethodNone,
        ApplicationType:         oauthApplicationType(redirectURI),
    }
    body, err := json.Marshal(payload)
    if err != nil {
        return dynamicClientRegistrationResponse{}, err
    }
    request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
    if err != nil {
        return dynamicClientRegistrationResponse{}, err
    }
    request.Header.Set("Content-Type", "application/json")
    request.Header.Set("Accept", "application/json")
    response, err := client.Do(request)
    if err != nil {
        return dynamicClientRegistrationResponse{}, fmt.Errorf("dynamic OAuth client registration: %w", err)
    }
    defer response.Body.Close()
    limited, err := io.ReadAll(io.LimitReader(response.Body, mcpOAuthMetadataMaxBody+1))
    if err != nil {
        return dynamicClientRegistrationResponse{}, err
    }
    if len(limited) > mcpOAuthMetadataMaxBody {
        return dynamicClientRegistrationResponse{}, fmt.Errorf("dynamic registration response is too large")
    }
    if response.StatusCode < 200 || response.StatusCode >= 300 {
        return dynamicClientRegistrationResponse{}, fmt.Errorf("dynamic registration endpoint returned HTTP %d", response.StatusCode)
    }
    var result dynamicClientRegistrationResponse
    if err := json.Unmarshal(limited, &result); err != nil {
        return result, fmt.Errorf("decode dynamic registration response: %w", err)
    }
    result.ClientID = strings.TrimSpace(result.ClientID)
    if result.ClientID == "" {
        return result, fmt.Errorf("dynamic registration response did not include client_id")
    }
    method := strings.TrimSpace(result.TokenEndpointAuthMethod)
    if method == "" {
        method = models.MCPOAuthAuthMethodNone
    }
    if method != models.MCPOAuthAuthMethodNone {
        return result, fmt.Errorf("dynamic registration returned unsupported token auth method %q; Omni requests a public client", method)
    }
    result.TokenEndpointAuthMethod = method
    return result, nil
}

'''
source = source[:insert] + dcr_helpers + source[insert:]
# Add persistent challenge recorder before AccessToken.
marker = '// AccessToken returns a valid resource-bound access token, refreshing it before\n'
if marker not in source:
    raise SystemExit('AccessToken marker missing')
recorder = r'''// RecordScopeChallenge persists the union of previously granted/requested scopes
// and the newly challenged scopes for an explicit authorization-code step-up.
func (s *OAuthService) RecordScopeChallenge(serverID string, scopes []string) error {
    credential, err := s.credentials.GetRuntime(serverID)
    if err != nil {
        return err
    }
    if credential == nil || credential.ClientID == "" {
        return ErrMCPOAuthNotConfigured
    }
    combined := append([]string{}, strings.Fields(credential.Scope)...)
    combined = append(combined, strings.Fields(credential.RequiredScope)...)
    combined = append(combined, scopes...)
    required := strings.Join(uniqueSortedStrings(combined), " ")
    if required == "" {
        return fmt.Errorf("insufficient_scope challenge did not include a scope")
    }
    return s.credentials.SaveRequiredScope(serverID, required)
}

'''
source = source.replace(marker, recorder + marker, 1)
oauth.write_text(source)

# OAuth tests: DCR request contract and application type.
oauth_test = Path("backend/internal/mcpclient/oauth_test.go")
source = oauth_test.read_text()
# Ensure httptest/http/json imports are available using a small import expansion.
source = source.replace('"net/url"\n', '"encoding/json"\n\t"net/http"\n\t"net/http/httptest"\n\t"net/url"\n', 1)
source += r'''

func TestDynamicPublicClientRegistrationContract(t *testing.T) {
    var received dynamicClientRegistrationRequest
    server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            t.Fatalf("method = %s", r.Method)
        }
        if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
            t.Fatal(err)
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"client_id":"dynamic-123","token_endpoint_auth_method":"none"}`))
    }))
    defer server.Close()

    result, err := registerDynamicPublicClient(context.Background(), server.Client(), server.URL, "http://127.0.0.1:54321/v1/mcp/oauth/callback")
    if err != nil {
        t.Fatalf("register dynamic public client: %v", err)
    }
    if result.ClientID != "dynamic-123" || received.TokenEndpointAuthMethod != "none" || received.ApplicationType != "native" {
        t.Fatalf("unexpected DCR contract: result=%#v request=%#v", result, received)
    }
    if len(received.RedirectURIs) != 1 || received.RedirectURIs[0] != "http://127.0.0.1:54321/v1/mcp/oauth/callback" {
        t.Fatalf("redirect URIs = %#v", received.RedirectURIs)
    }
}

func TestOAuthApplicationTypeUsesWebForHTTPSCallback(t *testing.T) {
    if got := oauthApplicationType("https://chat.example/v1/mcp/oauth/callback"); got != "web" {
        t.Fatalf("application type = %q, want web", got)
    }
}
'''
oauth_test.write_text(source)

# ---------------------------------------------------------------------------
# Streamable HTTP: record 403 Bearer insufficient_scope challenges and return a
# typed actionable error. No retry loop is attempted before user authorization.
# ---------------------------------------------------------------------------
http_client = Path("backend/internal/mcpclient/http_client.go")
source = http_client.read_text()
# Challenge handler before parseResponse.
marker = '// parseResponse reads the HTTP response body as either plain JSON or an SSE\n'
if marker not in source:
    raise SystemExit('HTTP parseResponse marker missing')
challenge_helper = r'''func (c *HTTPClient) handleOAuthScopeChallenge(response *http.Response) error {
    if response.StatusCode != http.StatusForbidden || c.tokenProvider == nil {
        return nil
    }
    challenge := response.Header.Get("WWW-Authenticate")
    if !strings.EqualFold(challengeParameter(challenge, "error"), "insufficient_scope") {
        return nil
    }
    _, scopes := parseBearerChallenge(challenge)
    scopes = uniqueSortedStrings(scopes)
    if len(scopes) == 0 {
        return nil
    }
    recorder, ok := c.tokenProvider.(interface {
        RecordScopeChallenge(serverID string, scopes []string) error
    })
    if ok {
        if err := recorder.RecordScopeChallenge(c.server.ID, scopes); err != nil {
            return fmt.Errorf("%w: persist scope challenge: %v", ErrMCPOAuthInsufficientScope, err)
        }
    }
    return fmt.Errorf("%w: %s", ErrMCPOAuthInsufficientScope, strings.Join(scopes, " "))
}

'''
source = source.replace(marker, challenge_helper + marker, 1)
# Insert challenge handling at all three non-success paths.
source = source.replace(
    '\tif httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {\n\t\tbody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))',
    '\tif httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {\n\t\tif err := c.handleOAuthScopeChallenge(httpResp); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tbody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))',
    1,
)
source = source.replace(
    '\tif httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {\n\t\tsnip, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))',
    '\tif httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {\n\t\tif err := c.handleOAuthScopeChallenge(httpResp); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tsnip, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))',
    1,
)
source = source.replace(
    '\tif httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {\n\t\treturn nil\n\t}\n\treturn fmt.Errorf("%s: unexpected HTTP %d", method, httpResp.StatusCode)',
    '\tif httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {\n\t\treturn nil\n\t}\n\tif err := c.handleOAuthScopeChallenge(httpResp); err != nil {\n\t\treturn err\n\t}\n\treturn fmt.Errorf("%s: unexpected HTTP %d", method, httpResp.StatusCode)',
    1,
)
http_client.write_text(source)

# Challenge test uses a token provider that also records scope requirements.
http_test = Path("backend/internal/mcpclient/http_oauth_test.go")
source = http_test.read_text()
source += r'''

type recordingBearerProvider struct {
    token  string
    scopes []string
}

func (p *recordingBearerProvider) AccessToken(_ context.Context, _ string) (string, error) { return p.token, nil }
func (p *recordingBearerProvider) RecordScopeChallenge(_ string, scopes []string) error {
    p.scopes = append([]string{}, scopes...)
    return nil
}

func TestHTTPClientRecordsInsufficientScopeChallenge(t *testing.T) {
    provider := &recordingBearerProvider{token: "access"}
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req rpcRequest
        if r.Method == http.MethodDelete {
            w.WriteHeader(http.StatusOK)
            return
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            t.Fatal(err)
        }
        if req.ID == 0 {
            w.WriteHeader(http.StatusAccepted)
            return
        }
        if req.Method == "initialize" {
            w.Header().Set("Content-Type", "application/json")
            writeJSONRPCResult(w, req.ID, map[string]interface{}{
                "protocolVersion": ProtocolVersion,
                "serverInfo": map[string]interface{}{"name": "scope-test", "version": "1"},
                "capabilities": map[string]interface{}{},
            })
            return
        }
        w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="files.write files.read"`)
        w.WriteHeader(http.StatusForbidden)
    }))
    defer server.Close()

    config := testMCPServer(server.URL)
    client := NewHTTPClientWithTokenProvider(config, provider)
    if err := client.Start(context.Background()); err != nil {
        t.Fatalf("start: %v", err)
    }
    _, err := client.ListTools(context.Background())
    if err == nil || !strings.Contains(err.Error(), ErrMCPOAuthInsufficientScope.Error()) {
        t.Fatalf("expected insufficient-scope error, got %v", err)
    }
    if strings.Join(provider.scopes, " ") != "files.read files.write" {
        t.Fatalf("recorded scopes = %#v", provider.scopes)
    }
}
'''
# Add encoding/json import if absent.
if '"encoding/json"' not in source:
    source = source.replace('import (\n', 'import (\n\t"encoding/json"\n', 1)
http_test.write_text(source)

# ---------------------------------------------------------------------------
# Frontend/API: status includes pending scope; blank prereg client permits
# automatic DCR fallback. DCR-generated IDs are shown as status, not rewritten
# into prereg configuration on reconnect.
# ---------------------------------------------------------------------------
api = Path("frontend/src/mcpOAuthApi.ts")
source = api.read_text()
source = source.replace("export type MCPOAuthRegistrationMethod = 'preregistered' | 'cimd';", "export type MCPOAuthRegistrationMethod = 'preregistered' | 'cimd' | 'dcr';", 1)
source = source.replace('  scope?: string;\n', '  scope?: string;\n  required_scope?: string;\n', 1)
api.write_text(source)

panel = Path("frontend/src/components/MCPAuthorizationPanel.tsx")
source = panel.read_text()
source = source.replace(
    "      setClientId(next.client_id || '');\n      setAuthMethod(next.token_endpoint_auth_method || 'none');\n      setRegistrationMethod(next.registration_method || 'preregistered');",
    "      const generatedDCR = next.registration_method === 'dcr';\n      setClientId(generatedDCR ? '' : (next.client_id || ''));\n      setAuthMethod(next.token_endpoint_auth_method || 'none');\n      setRegistrationMethod(next.registration_method === 'cimd' ? 'cimd' : 'preregistered');",
    1,
)
# Connect may skip save when prereg fields are blank, allowing server-advertised DCR.
old_connect = '''    try {
      await saveConfiguration();
      const start = await mcpOAuthApi.start(server.id);
'''
new_connect = '''    try {
      const steppingUp = Boolean(status?.required_scope);
      if (clientId.trim()) {
        await saveConfiguration();
      } else if (registrationMethod === 'cimd') {
        throw new Error('A Client ID Metadata Document URL is required');
      }
      const start = await mcpOAuthApi.start(server.id);
'''
if old_connect not in source:
    raise SystemExit('frontend connect marker missing')
source = source.replace(old_connect, new_connect, 1)
source = source.replace(
    '        if (next?.connected || attempts >= 60) {',
    '        if ((next?.connected && (!steppingUp || !next.required_scope)) || attempts >= 60) {',
    1,
)
source = source.replace(
    "            toast.success('MCP OAuth connected');",
    "            toast.success(steppingUp ? 'MCP OAuth permissions updated' : 'MCP OAuth connected');",
    1,
)
# Add DCR fallback guidance after CIMD/prereg form.
marker = '      <div className="mt-3 rounded-xl border border-border bg-surface/50 p-3 text-[10px] leading-relaxed text-text-muted">\n        <div><span className="font-medium text-text-secondary">Redirect URI:</span>'
if marker not in source:
    raise SystemExit('frontend redirect marker missing')
dcr_note = '''      {registrationMethod === 'preregistered' && !clientId.trim() && status?.registration_method !== 'dcr' && (
        <div className="mt-3 rounded-xl border border-amber-500/20 bg-amber-500/5 p-3 text-[10px] leading-relaxed text-text-muted">
          No client ID entered. Connect can fall back to legacy Dynamic Client Registration only when the authorization server advertises a registration endpoint. DCR is retained for backward compatibility; preregistration or CIMD is preferred.
        </div>
      )}

'''
source = source.replace(marker, dcr_note + marker, 1)
# Registration status label supports DCR.
source = source.replace(
    "{status.registration_method === 'cimd' ? 'Client ID Metadata Document' : 'Preregistered client'}",
    "{status.registration_method === 'cimd' ? 'Client ID Metadata Document' : status.registration_method === 'dcr' ? 'Dynamic registration (legacy DCR)' : 'Preregistered client'}",
    1,
)
# Add explicit scope-step-up alert before action row.
action_marker = '      <div className="mt-4 flex flex-wrap items-center gap-2">\n'
if action_marker not in source:
    raise SystemExit('frontend action marker missing')
scope_alert = '''      {status?.required_scope && (
        <div className="mt-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-amber-200">
          <div className="font-semibold">Additional OAuth permission required</div>
          <div className="mt-1 break-all font-mono text-[10px]">{status.required_scope}</div>
          <div className="mt-1 text-[10px] text-text-muted">Granting this step-up starts a new PKCE authorization flow and unions the challenged scopes with the scopes already granted/requested.</div>
        </div>
      )}

'''
source = source.replace(action_marker, scope_alert + action_marker, 1)
source = source.replace(
    'disabled={busy || !clientId.trim()} onClick={() => void connect()}',
    'disabled={busy || (registrationMethod === \'cimd\' && !clientId.trim())} onClick={() => void connect()}',
    1,
)
source = source.replace(
    "{status?.connected ? 'Reconnect OAuth' : 'Connect OAuth'}",
    "{status?.required_scope ? 'Grant additional scopes' : status?.connected ? 'Reconnect OAuth' : status?.registration_method === 'dcr' ? 'Reconnect OAuth' : 'Connect OAuth'}",
    1,
)
panel.write_text(source)

# ---------------------------------------------------------------------------
# Documentation: B2 behavior and explicit no-silent-retry policy.
# ---------------------------------------------------------------------------
doc = Path("docs/MCP_OAUTH_2026-08.md")
source = doc.read_text()
source = source.replace(
    'The current MCP specification prioritizes preregistered credentials when available, then Client ID Metadata Documents (CIMD). DCR is deprecated and remains a later fallback for older authorization servers.',
    'The current MCP specification prioritizes preregistered credentials when available, then Client ID Metadata Documents (CIMD). Omni also supports Dynamic Client Registration only as the deprecated fallback when no client is configured and the authorization server advertises `registration_endpoint`. Automatic DCR requests a public PKCE client (`token_endpoint_auth_method=none`) and binds the returned client ID to the exact issuer.',
    1,
)
source = source.replace(
    '## Deliberate follow-on work\n\nDeprecated Dynamic Client Registration fallback and persistent incremental authorization-scope step-up UX remain Phase B2. They are kept separate from issuer/CIMD correctness so the registration trust boundary stays reviewable.',
    '''## Incremental authorization step-up\n\nWhen a remote MCP operation returns `403` with `WWW-Authenticate: Bearer error="insufficient_scope"`, Omni persists the normalized union of the granted/requested scopes and the newly challenged scopes. Settings shows the pending scope set and offers **Grant additional scopes**, which starts a fresh resource-bound PKCE authorization flow. A successful token grant clears the pending requirement.\n\nOmni does not silently replay a failed tool call after browser authorization; the original action remains failed and should be retried by the user/agent after authorization completes. This avoids hidden side effects and retry loops across an interactive consent boundary.''',
    1,
)
doc.write_text(source)
