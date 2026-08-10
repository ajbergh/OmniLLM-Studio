package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

// MCPOAuthCredential is the decrypted runtime view used only inside the backend.
type MCPOAuthCredential struct {
	ServerID                string
	ClientID                string
	ClientSecret            string
	AccessToken             string
	RefreshToken            string
	TokenType               string
	Scope                   string
	RequiredScope           string
	ExpiresAt               *time.Time
	TokenEndpointAuthMethod string
	AuthorizationServer     string
	AuthorizationEndpoint   string
	TokenEndpoint           string
	ResourceMetadataURL     string
	RegistrationMethod      string
	ClientIssuer            string
}

// MCPOAuthRepo persists preregistered OAuth client configuration and tokens.
type MCPOAuthRepo struct{ db *sql.DB }

func NewMCPOAuthRepo(db *sql.DB) *MCPOAuthRepo { return &MCPOAuthRepo{db: db} }

func validMCPOAuthRegistrationMethod(value string) bool {
	switch value {
	case models.MCPOAuthRegistrationPreregistered, models.MCPOAuthRegistrationCIMD, models.MCPOAuthRegistrationDCR:
		return true
	default:
		return false
	}
}

func validMCPOAuthMethod(value string) bool {
	switch value {
	case models.MCPOAuthAuthMethodNone, models.MCPOAuthAuthMethodClientSecretBasic, models.MCPOAuthAuthMethodClientSecretPost:
		return true
	default:
		return false
	}
}

// ConfigureClient creates or updates preregistered OAuth client settings. A
// client change invalidates previously issued tokens to avoid token/client mixups.
func (r *MCPOAuthRepo) ConfigureClient(serverID string, input models.ConfigureMCPOAuthInput) error {
	clientID := strings.TrimSpace(input.ClientID)
	if clientID == "" {
		return fmt.Errorf("client_id is required")
	}
	method := strings.TrimSpace(input.TokenEndpointAuthMethod)
	if method == "" {
		method = models.MCPOAuthAuthMethodNone
	}
	if !validMCPOAuthMethod(method) {
		return fmt.Errorf("unsupported token_endpoint_auth_method %q", method)
	}
	registrationMethod := strings.TrimSpace(input.RegistrationMethod)
	if registrationMethod == "" {
		registrationMethod = models.MCPOAuthRegistrationPreregistered
	}
	if !validMCPOAuthRegistrationMethod(registrationMethod) {
		return fmt.Errorf("unsupported OAuth registration_method %q", registrationMethod)
	}

	clientIssuer := strings.TrimSpace(input.ClientIssuer)
	switch registrationMethod {
	case models.MCPOAuthRegistrationPreregistered:
		if clientIssuer == "" {
			return fmt.Errorf("client_issuer is required for preregistered OAuth clients")
		}
	case models.MCPOAuthRegistrationCIMD:
		clientIssuer = ""
	case models.MCPOAuthRegistrationDCR:
		return fmt.Errorf("dynamic OAuth registration is managed automatically")
	}

	secretEnc := ""
	existingClientID, existingMethod, existingRegistration, existingIssuer := "", "", "", ""
	var existingSecret sql.NullString
	existingErr := r.db.QueryRow(`SELECT client_id, client_secret_enc, token_endpoint_auth_method, registration_method, client_issuer FROM mcp_oauth_credentials WHERE server_id = ?`, serverID).Scan(&existingClientID, &existingSecret, &existingMethod, &existingRegistration, &existingIssuer)
	if existingErr != nil && existingErr != sql.ErrNoRows {
		return fmt.Errorf("read existing MCP OAuth client: %w", existingErr)
	}
	if input.ClientSecret == nil {
		if existingErr == nil && existingClientID == clientID && existingMethod == method && existingRegistration == registrationMethod && existingIssuer == clientIssuer && existingSecret.Valid {
			secretEnc = existingSecret.String
		}
	} else if *input.ClientSecret != "" {
		var err error
		secretEnc, err = crypto.Encrypt(*input.ClientSecret)
		if err != nil {
			return fmt.Errorf("encrypt MCP OAuth client secret: %w", err)
		}
	}
	if method != models.MCPOAuthAuthMethodNone && strings.TrimSpace(secretEnc) == "" {
		return fmt.Errorf("client_secret is required for %s", method)
	}

	_, err := r.db.Exec(`
		INSERT INTO mcp_oauth_credentials (
			server_id, client_id, client_secret_enc, token_endpoint_auth_method,
			access_token_enc, refresh_token_enc, token_type, scope, expires_at,
			authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer, updated_at
		) VALUES (?, ?, ?, ?, '', '', '', '', NULL, '', '', '', '', ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(server_id) DO UPDATE SET
			client_id = excluded.client_id,
			client_secret_enc = excluded.client_secret_enc,
			token_endpoint_auth_method = excluded.token_endpoint_auth_method,
			registration_method = excluded.registration_method, client_issuer = excluded.client_issuer,
			access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', required_scope = '', expires_at = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, serverID, clientID, secretEnc, method, registrationMethod, clientIssuer)
	if err != nil {
		return fmt.Errorf("save MCP OAuth client: %w", err)
	}
	return nil
}

// ConfigureDynamicClient stores an automatically registered public DCR client.
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

func decryptOptional(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return crypto.Decrypt(value)
}

// GetRuntime returns decrypted OAuth material for backend-only execution.
func (r *MCPOAuthRepo) GetRuntime(serverID string) (*MCPOAuthCredential, error) {
	var item MCPOAuthCredential
	var clientSecretEnc, accessTokenEnc, refreshTokenEnc string
	var expiresAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT server_id, client_id, client_secret_enc, access_token_enc, refresh_token_enc,
			token_type, scope, expires_at, token_endpoint_auth_method,
			authorization_server, authorization_endpoint, token_endpoint, resource_metadata_url, registration_method, client_issuer, required_scope
		FROM mcp_oauth_credentials WHERE server_id = ?
	`, serverID).Scan(
		&item.ServerID, &item.ClientID, &clientSecretEnc, &accessTokenEnc, &refreshTokenEnc,
		&item.TokenType, &item.Scope, &expiresAt, &item.TokenEndpointAuthMethod,
		&item.AuthorizationServer, &item.AuthorizationEndpoint, &item.TokenEndpoint, &item.ResourceMetadataURL, &item.RegistrationMethod, &item.ClientIssuer, &item.RequiredScope,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP OAuth credential: %w", err)
	}
	if expiresAt.Valid {
		value := expiresAt.Time.UTC()
		item.ExpiresAt = &value
	}
	if item.ClientSecret, err = decryptOptional(clientSecretEnc); err != nil {
		return nil, fmt.Errorf("decrypt MCP OAuth client secret: %w", err)
	}
	if item.AccessToken, err = decryptOptional(accessTokenEnc); err != nil {
		return nil, fmt.Errorf("decrypt MCP OAuth access token: %w", err)
	}
	if item.RefreshToken, err = decryptOptional(refreshTokenEnc); err != nil {
		return nil, fmt.Errorf("decrypt MCP OAuth refresh token: %w", err)
	}
	return &item, nil
}

// Status returns the non-secret connection view.
func (r *MCPOAuthRepo) Status(serverID, redirectURI string) (models.MCPOAuthStatus, error) {
	credential, err := r.GetRuntime(serverID)
	if err != nil {
		return models.MCPOAuthStatus{}, err
	}
	status := models.MCPOAuthStatus{ServerID: serverID, RedirectURI: redirectURI}
	if credential == nil {
		return status, nil
	}
	status.Configured = strings.TrimSpace(credential.ClientID) != ""
	status.Connected = strings.TrimSpace(credential.AccessToken) != "" && (credential.ExpiresAt == nil || credential.ExpiresAt.After(time.Now().UTC()))
	status.ClientID = credential.ClientID
	status.RegistrationMethod = credential.RegistrationMethod
	status.ClientIssuer = credential.ClientIssuer
	status.HasClientSecret = credential.ClientSecret != ""
	status.HasRefreshToken = credential.RefreshToken != ""
	status.TokenEndpointAuthMethod = credential.TokenEndpointAuthMethod
	status.Scope = credential.Scope
	status.RequiredScope = credential.RequiredScope
	status.ExpiresAt = credential.ExpiresAt
	status.AuthorizationServer = credential.AuthorizationServer
	status.AuthorizationEndpoint = credential.AuthorizationEndpoint
	status.TokenEndpoint = credential.TokenEndpoint
	status.ResourceMetadataURL = credential.ResourceMetadataURL
	return status, nil
}

// BindClientIssuer verifies authorization-server ownership for preregistered
// and DCR credentials. CIMD client IDs are self-hosted and portable across issuers.
// The issuer is never learned on first use: preregistered credentials must carry
// an explicit trusted issuer and DCR credentials are bound when registered.
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
	if credential.ClientIssuer == "" {
		if credential.RegistrationMethod == models.MCPOAuthRegistrationDCR {
			return fmt.Errorf("dynamic OAuth client has no authorization-server issuer binding")
		}
		return fmt.Errorf("preregistered OAuth client must declare its authorization-server issuer")
	}
	if credential.ClientIssuer != issuer {
		return fmt.Errorf("OAuth client credentials are bound to authorization server %q, not %q", credential.ClientIssuer, issuer)
	}
	return nil
}

// SaveDiscovery records non-secret endpoints discovered from RFC9728 / RFC8414
// metadata so later refreshes do not depend on stale user-provided endpoints.
func (r *MCPOAuthRepo) SaveDiscovery(serverID, authorizationServer, authorizationEndpoint, tokenEndpoint, resourceMetadataURL string) error {
	_, err := r.db.Exec(`
		UPDATE mcp_oauth_credentials
		SET authorization_server = ?, authorization_endpoint = ?, token_endpoint = ?, resource_metadata_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ?
	`, authorizationServer, authorizationEndpoint, tokenEndpoint, resourceMetadataURL, serverID)
	if err != nil {
		return fmt.Errorf("save MCP OAuth discovery: %w", err)
	}
	return nil
}

// SaveTokens encrypts OAuth tokens at rest. An empty refresh token preserves the
// prior refresh token, matching common rotation behavior where refresh responses
// omit a replacement token.
func (r *MCPOAuthRepo) SaveTokens(serverID, accessToken, refreshToken, tokenType, scope string, expiresAt *time.Time) error {
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("access token is required")
	}
	accessEnc, err := crypto.Encrypt(accessToken)
	if err != nil {
		return fmt.Errorf("encrypt MCP OAuth access token: %w", err)
	}
	refreshEnc := ""
	if refreshToken != "" {
		refreshEnc, err = crypto.Encrypt(refreshToken)
		if err != nil {
			return fmt.Errorf("encrypt MCP OAuth refresh token: %w", err)
		}
	}
	if refreshToken == "" {
		_, err = r.db.Exec(`
			UPDATE mcp_oauth_credentials
			SET access_token_enc = ?, token_type = ?, scope = ?, required_scope = '', expires_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE server_id = ?
		`, accessEnc, tokenType, scope, expiresAt, serverID)
	} else {
		_, err = r.db.Exec(`
			UPDATE mcp_oauth_credentials
			SET access_token_enc = ?, refresh_token_enc = ?, token_type = ?, scope = ?, required_scope = '', expires_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE server_id = ?
		`, accessEnc, refreshEnc, tokenType, scope, expiresAt, serverID)
	}
	if err != nil {
		return fmt.Errorf("save MCP OAuth tokens: %w", err)
	}
	return nil
}

func (r *MCPOAuthRepo) ClearTokens(serverID string) error {
	_, err := r.db.Exec(`
		UPDATE mcp_oauth_credentials
		SET access_token_enc = '', refresh_token_enc = '', token_type = '', scope = '', required_scope = '', expires_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ?
	`, serverID)
	if err != nil {
		return fmt.Errorf("clear MCP OAuth tokens: %w", err)
	}
	return nil
}

// ClearDynamicClient removes a persisted legacy DCR registration so the next
// authorization attempt can register a fresh public client. Preregistered and
// CIMD configurations are never deleted through this recovery operation.
func (r *MCPOAuthRepo) ClearDynamicClient(serverID string) (bool, error) {
	result, err := r.db.Exec(`DELETE FROM mcp_oauth_credentials WHERE server_id = ? AND registration_method = ?`, serverID, models.MCPOAuthRegistrationDCR)
	if err != nil {
		return false, fmt.Errorf("clear dynamically registered MCP OAuth client: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read dynamic OAuth reset result: %w", err)
	}
	return rows > 0, nil
}

// SaveRequiredScope persists the complete scope set needed for the next
// authorization-code step-up. Callers pass a normalized union.
func (r *MCPOAuthRepo) SaveRequiredScope(serverID, scope string) error {
	if _, err := r.db.Exec(`UPDATE mcp_oauth_credentials SET required_scope = ?, updated_at = CURRENT_TIMESTAMP WHERE server_id = ?`, strings.TrimSpace(scope), serverID); err != nil {
		return fmt.Errorf("save MCP OAuth required scope: %w", err)
	}
	return nil
}
