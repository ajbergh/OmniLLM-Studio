package mcpclient

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

var (
	ErrMCPOAuthNotConfigured = errors.New("MCP OAuth is not configured")
	ErrMCPOAuthRequired      = errors.New("MCP OAuth authorization is required")
)

const (
	mcpOAuthStateTTL        = 10 * time.Minute
	mcpOAuthMetadataMaxBody = 1 << 20
	mcpOAuthRefreshSkew     = 60 * time.Second
)

type protectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

type authorizationServerMetadata struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	ScopesSupported               []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethods      []string `json:"token_endpoint_auth_methods_supported"`
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int64  `json:"expires_in"`
}

type oauthPendingState struct {
	ServerID                string
	UserID                  string
	CodeVerifier            string
	RedirectURI             string
	ResourceURI             string
	TokenEndpoint           string
	ClientID                string
	TokenEndpointAuthMethod string
	Scope                   string
	ExpiresAt               time.Time
}

// BearerTokenProvider supplies an MCP-scoped access token, refreshing it when
// needed. Empty token with nil error means OAuth is not configured for the server.
type BearerTokenProvider interface {
	AccessToken(ctx context.Context, serverID string) (string, error)
}

// OAuthService owns OAuth discovery, PKCE state, token exchange, refresh, and
// encrypted token persistence for remote MCP HTTP servers.
type OAuthService struct {
	servers     *repository.MCPServerRepo
	credentials *repository.MCPOAuthRepo
	redirectURI string

	mu     sync.Mutex
	states map[string]oauthPendingState
}

func NewOAuthService(servers *repository.MCPServerRepo, credentials *repository.MCPOAuthRepo, redirectURI string) (*OAuthService, error) {
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:8080/v1/mcp/oauth/callback"
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHostname(parsed.Hostname()))) {
		return nil, fmt.Errorf("MCP OAuth redirect URI must be HTTPS or localhost HTTP")
	}
	return &OAuthService{servers: servers, credentials: credentials, redirectURI: redirectURI, states: map[string]oauthPendingState{}}, nil
}

func isLoopbackHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func randomURLSafe(bytesCount int) (string, error) {
	buffer := make([]byte, bytesCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func codeChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func canonicalResourceURI(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid MCP resource URI")
	}
	parsed.Fragment = ""
	parsed.User = nil
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func normalizeIssuer(raw string) string { return strings.TrimRight(strings.TrimSpace(raw), "/") }

func (s *OAuthService) Status(serverID string) (models.MCPOAuthStatus, error) {
	return s.credentials.Status(serverID, s.redirectURI)
}

func (s *OAuthService) Configure(serverID string, input models.ConfigureMCPOAuthInput) error {
	server, err := s.servers.GetRuntimeByID(serverID)
	if err != nil {
		return err
	}
	if server == nil || server.Transport != "http" || server.URL == nil {
		return fmt.Errorf("OAuth requires an HTTP MCP server")
	}
	if strings.TrimSpace(input.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}
	method := strings.TrimSpace(input.TokenEndpointAuthMethod)
	if method == "" {
		method = models.MCPOAuthAuthMethodNone
		input.TokenEndpointAuthMethod = method
	}
	if method != models.MCPOAuthAuthMethodNone && input.ClientSecret != nil && strings.TrimSpace(*input.ClientSecret) == "" {
		return fmt.Errorf("client_secret is required for %s", method)
	}
	return s.credentials.ConfigureClient(serverID, input)
}

// StartAuthorization discovers OAuth metadata, creates PKCE/state material, and
// returns the authorization URL to open in the user's browser.
func (s *OAuthService) StartAuthorization(ctx context.Context, serverID, userID string) (models.MCPOAuthAuthorizationStart, error) {
	server, err := s.servers.GetRuntimeByID(serverID)
	if err != nil {
		return models.MCPOAuthAuthorizationStart{}, err
	}
	if server == nil || server.Transport != "http" || server.URL == nil {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("OAuth requires an HTTP MCP server")
	}
	credential, err := s.credentials.GetRuntime(serverID)
	if err != nil {
		return models.MCPOAuthAuthorizationStart{}, err
	}
	if credential == nil || strings.TrimSpace(credential.ClientID) == "" {
		return models.MCPOAuthAuthorizationStart{}, ErrMCPOAuthNotConfigured
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
	authorizationServer := normalizeIssuer(resourceMetadata.AuthorizationServers[0])
	authMetadata, err := discoverAuthorizationServer(ctx, *server, authorizationServer)
	if err != nil {
		return models.MCPOAuthAuthorizationStart{}, err
	}
	if normalizeIssuer(authMetadata.Issuer) != authorizationServer {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization metadata issuer mismatch")
	}
	if !containsString(authMetadata.CodeChallengeMethodsSupported, "S256") {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not advertise PKCE S256")
	}
	if !oauthMethodSupported(credential.TokenEndpointAuthMethod, authMetadata.TokenEndpointAuthMethods) {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization server does not support token auth method %q", credential.TokenEndpointAuthMethod)
	}
	if err := validateOAuthEndpoint(authMetadata.AuthorizationEndpoint); err != nil {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("authorization endpoint: %w", err)
	}
	if err := validateOAuthEndpoint(authMetadata.TokenEndpoint); err != nil {
		return models.MCPOAuthAuthorizationStart{}, fmt.Errorf("token endpoint: %w", err)
	}

	scopes := challengedScopes
	if len(scopes) == 0 {
		scopes = resourceMetadata.ScopesSupported
	}
	if len(scopes) == 0 {
		scopes = authMetadata.ScopesSupported
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
		ExpiresAt:               time.Now().UTC().Add(mcpOAuthStateTTL),
	}
	s.mu.Unlock()

	if err := s.credentials.SaveDiscovery(serverID, authorizationServer, authMetadata.AuthorizationEndpoint, authMetadata.TokenEndpoint, resourceMetadataURL); err != nil {
		return models.MCPOAuthAuthorizationStart{}, err
	}
	return models.MCPOAuthAuthorizationStart{
		AuthorizationURL:    authURL.String(),
		AuthorizationServer: authorizationServer,
		Scope:               scope,
		RedirectURI:         s.redirectURI,
	}, nil
}

// CompleteAuthorization validates one-time state and exchanges the authorization
// code for resource-bound tokens.
func (s *OAuthService) CompleteAuthorization(ctx context.Context, state, code string) (string, error) {
	state = strings.TrimSpace(state)
	code = strings.TrimSpace(code)
	if state == "" || code == "" {
		return "", fmt.Errorf("state and code are required")
	}

	s.mu.Lock()
	pending, ok := s.states[state]
	delete(s.states, state)
	s.mu.Unlock()
	if !ok || time.Now().UTC().After(pending.ExpiresAt) {
		return "", fmt.Errorf("OAuth state is invalid or expired")
	}

	server, err := s.servers.GetRuntimeByID(pending.ServerID)
	if err != nil || server == nil {
		return "", fmt.Errorf("MCP server is no longer available")
	}
	credential, err := s.credentials.GetRuntime(pending.ServerID)
	if err != nil {
		return "", err
	}
	if credential == nil || credential.ClientID != pending.ClientID || credential.TokenEndpointAuthMethod != pending.TokenEndpointAuthMethod {
		return "", fmt.Errorf("MCP OAuth client configuration changed during authorization")
	}

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", pending.RedirectURI)
	values.Set("client_id", pending.ClientID)
	values.Set("code_verifier", pending.CodeVerifier)
	values.Set("resource", pending.ResourceURI)
	token, err := s.exchangeToken(ctx, *server, pending.TokenEndpoint, pending.TokenEndpointAuthMethod, credential.ClientSecret, values)
	if err != nil {
		return "", err
	}
	expiresAt := tokenExpiry(token.ExpiresIn)
	if token.Scope == "" {
		token.Scope = pending.Scope
	}
	if err := s.credentials.SaveTokens(pending.ServerID, token.AccessToken, token.RefreshToken, token.TokenType, token.Scope, expiresAt); err != nil {
		return "", err
	}
	return pending.ServerID, nil
}

// AccessToken returns a valid resource-bound access token, refreshing it before
// expiry when a refresh token is available.
func (s *OAuthService) AccessToken(ctx context.Context, serverID string) (string, error) {
	credential, err := s.credentials.GetRuntime(serverID)
	if err != nil {
		return "", err
	}
	if credential == nil || strings.TrimSpace(credential.ClientID) == "" {
		return "", nil
	}
	if credential.AccessToken != "" && (credential.ExpiresAt == nil || credential.ExpiresAt.After(time.Now().UTC().Add(mcpOAuthRefreshSkew))) {
		return credential.AccessToken, nil
	}
	if credential.RefreshToken == "" {
		return "", ErrMCPOAuthRequired
	}
	server, err := s.servers.GetRuntimeByID(serverID)
	if err != nil || server == nil || server.URL == nil {
		return "", fmt.Errorf("MCP server is unavailable for token refresh")
	}
	if credential.TokenEndpoint == "" {
		return "", ErrMCPOAuthRequired
	}
	resourceURI, err := canonicalResourceURI(*server.URL)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", credential.RefreshToken)
	values.Set("client_id", credential.ClientID)
	values.Set("resource", resourceURI)
	if credential.Scope != "" {
		values.Set("scope", credential.Scope)
	}
	token, err := s.exchangeToken(ctx, *server, credential.TokenEndpoint, credential.TokenEndpointAuthMethod, credential.ClientSecret, values)
	if err != nil {
		return "", err
	}
	if token.Scope == "" {
		token.Scope = credential.Scope
	}
	if err := s.credentials.SaveTokens(serverID, token.AccessToken, token.RefreshToken, token.TokenType, token.Scope, tokenExpiry(token.ExpiresIn)); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// RejectAuthorization consumes a pending state after an authorization-server error.
func (s *OAuthService) RejectAuthorization(state string) {
	state = strings.TrimSpace(state)
	if state == "" {
		return
	}
	s.mu.Lock()
	delete(s.states, state)
	s.mu.Unlock()
}

func (s *OAuthService) Disconnect(serverID string) error { return s.credentials.ClearTokens(serverID) }

func (s *OAuthService) exchangeToken(ctx context.Context, server models.MCPServer, endpoint, method, clientSecret string, values url.Values) (oauthTokenResponse, error) {
	if err := validateOAuthEndpoint(endpoint); err != nil {
		return oauthTokenResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthTokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	switch method {
	case "", models.MCPOAuthAuthMethodNone:
	case models.MCPOAuthAuthMethodClientSecretBasic:
		if clientSecret == "" {
			return oauthTokenResponse{}, fmt.Errorf("OAuth client secret is required")
		}
		request.SetBasicAuth(values.Get("client_id"), clientSecret)
		values.Del("client_id")
		request.Body = io.NopCloser(strings.NewReader(values.Encode()))
		request.ContentLength = int64(len(values.Encode()))
	case models.MCPOAuthAuthMethodClientSecretPost:
		if clientSecret == "" {
			return oauthTokenResponse{}, fmt.Errorf("OAuth client secret is required")
		}
		values.Set("client_secret", clientSecret)
		request.Body = io.NopCloser(strings.NewReader(values.Encode()))
		request.ContentLength = int64(len(values.Encode()))
	default:
		return oauthTokenResponse{}, fmt.Errorf("unsupported token endpoint auth method %q", method)
	}

	response, err := newHTTPTransportClient(server).Do(request)
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("OAuth token exchange: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, mcpOAuthMetadataMaxBody+1))
	if err != nil {
		return oauthTokenResponse{}, err
	}
	if len(body) > mcpOAuthMetadataMaxBody {
		return oauthTokenResponse{}, fmt.Errorf("OAuth token response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oauthTokenResponse{}, fmt.Errorf("OAuth token endpoint returned HTTP %d", response.StatusCode)
	}
	var token oauthTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return oauthTokenResponse{}, fmt.Errorf("decode OAuth token response: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return oauthTokenResponse{}, fmt.Errorf("OAuth token response did not include access_token")
	}
	if token.TokenType != "" && !strings.EqualFold(token.TokenType, "Bearer") {
		return oauthTokenResponse{}, fmt.Errorf("unsupported OAuth token type %q", token.TokenType)
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return token, nil
}

func tokenExpiry(expiresIn int64) *time.Time {
	if expiresIn <= 0 {
		return nil
	}
	value := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	return &value
}

func validateOAuthEndpoint(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("OAuth endpoint must be an HTTPS URL without credentials or fragments")
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func oauthMethodSupported(method string, advertised []string) bool {
	if method == "" {
		method = models.MCPOAuthAuthMethodNone
	}
	if len(advertised) == 0 {
		return method == models.MCPOAuthAuthMethodNone
	}
	return containsString(advertised, method)
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, field := range strings.Fields(value) {
			if field != "" && !seen[field] {
				seen[field] = true
				out = append(out, field)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *OAuthService) pruneExpiredStatesLocked() {
	now := time.Now().UTC()
	for key, pending := range s.states {
		if now.After(pending.ExpiresAt) {
			delete(s.states, key)
		}
	}
}

func discoverProtectedResource(ctx context.Context, server models.MCPServer, resourceURI string) (protectedResourceMetadata, string, []string, error) {
	client := newHTTPTransportClient(server)
	metadataURL := ""
	var challengedScopes []string
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURI, nil)
	if err == nil {
		request.Header.Set("Accept", "application/json")
		if response, requestErr := client.Do(request); requestErr == nil {
			if response.StatusCode == http.StatusUnauthorized {
				metadataURL, challengedScopes = parseBearerChallenge(response.Header.Get("WWW-Authenticate"))
			}
			response.Body.Close()
		}
	}

	candidates := []string{}
	if metadataURL != "" {
		candidates = append(candidates, metadataURL)
	}
	fallback, err := protectedResourceMetadataCandidates(resourceURI)
	if err != nil {
		return protectedResourceMetadata{}, "", nil, err
	}
	candidates = append(candidates, fallback...)
	for _, candidate := range dedupeStrings(candidates) {
		var metadata protectedResourceMetadata
		if err := fetchProtectedResourceJSON(ctx, client, candidate, &metadata); err != nil {
			continue
		}
		if metadata.Resource != "" {
			canonicalMetadataResource, err := canonicalResourceURI(metadata.Resource)
			if err != nil || canonicalMetadataResource != resourceURI {
				return protectedResourceMetadata{}, "", nil, fmt.Errorf("protected resource metadata resource mismatch")
			}
		}
		return metadata, candidate, challengedScopes, nil
	}
	return protectedResourceMetadata{}, "", nil, fmt.Errorf("unable to discover MCP protected resource metadata")
}

func protectedResourceMetadataCandidates(resourceURI string) ([]string, error) {
	parsed, err := url.Parse(resourceURI)
	if err != nil {
		return nil, err
	}
	origin := parsed.Scheme + "://" + parsed.Host
	path := strings.TrimPrefix(parsed.EscapedPath(), "/")
	candidates := []string{}
	if path != "" {
		candidates = append(candidates, origin+"/.well-known/oauth-protected-resource/"+path)
	}
	candidates = append(candidates, origin+"/.well-known/oauth-protected-resource")
	return candidates, nil
}

func discoverAuthorizationServer(ctx context.Context, server models.MCPServer, issuer string) (authorizationServerMetadata, error) {
	candidates, err := authorizationMetadataCandidates(issuer)
	if err != nil {
		return authorizationServerMetadata{}, err
	}
	client := newHTTPTransportClient(server)
	for _, candidate := range candidates {
		var metadata authorizationServerMetadata
		if err := fetchOAuthJSON(ctx, client, candidate, &metadata); err != nil {
			continue
		}
		if metadata.Issuer == "" || metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" {
			continue
		}
		return metadata, nil
	}
	return authorizationServerMetadata{}, fmt.Errorf("unable to discover OAuth authorization server metadata")
}

func authorizationMetadataCandidates(issuer string) ([]string, error) {
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("authorization server issuer must be HTTPS")
	}
	origin := parsed.Scheme + "://" + parsed.Host
	issuerPath := strings.Trim(parsed.EscapedPath(), "/")
	candidates := []string{}
	if issuerPath != "" {
		candidates = append(candidates,
			origin+"/.well-known/oauth-authorization-server/"+issuerPath,
			origin+"/.well-known/openid-configuration/"+issuerPath,
			strings.TrimRight(issuer, "/")+"/.well-known/openid-configuration",
		)
	} else {
		candidates = append(candidates,
			origin+"/.well-known/oauth-authorization-server",
			origin+"/.well-known/openid-configuration",
		)
	}
	return dedupeStrings(candidates), nil
}

func fetchProtectedResourceJSON(ctx context.Context, client *http.Client, endpoint string, out interface{}) error {
	if err := ValidateHTTPServerURL(endpoint); err != nil {
		return err
	}
	return fetchOAuthJSONWithValidation(ctx, client, endpoint, out, false)
}

func fetchOAuthJSON(ctx context.Context, client *http.Client, endpoint string, out interface{}) error {
	return fetchOAuthJSONWithValidation(ctx, client, endpoint, out, true)
}

func fetchOAuthJSONWithValidation(ctx context.Context, client *http.Client, endpoint string, out interface{}, requireHTTPS bool) error {
	if requireHTTPS {
		if err := validateOAuthEndpoint(endpoint); err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("metadata endpoint returned HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, mcpOAuthMetadataMaxBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(body) > mcpOAuthMetadataMaxBody {
		return fmt.Errorf("metadata response is too large")
	}
	return json.Unmarshal(body, out)
}

func parseBearerChallenge(header string) (string, []string) {
	if !strings.Contains(strings.ToLower(header), "bearer") {
		return "", nil
	}
	return challengeParameter(header, "resource_metadata"), strings.Fields(challengeParameter(header, "scope"))
}

func challengeParameter(header, name string) string {
	lower := strings.ToLower(header)
	needle := strings.ToLower(name) + "="
	index := strings.Index(lower, needle)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(header[index+len(needle):])
	if strings.HasPrefix(value, "\"") {
		value = value[1:]
		if end := strings.Index(value, "\""); end >= 0 {
			return value[:end]
		}
		return ""
	}
	if end := strings.IndexAny(value, ", "); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
