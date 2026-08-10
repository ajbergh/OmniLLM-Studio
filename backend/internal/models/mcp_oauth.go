package models

import "time"

const (
	MCPOAuthAuthMethodNone              = "none"
	MCPOAuthAuthMethodClientSecretBasic = "client_secret_basic"
	MCPOAuthAuthMethodClientSecretPost  = "client_secret_post"

	MCPOAuthRegistrationPreregistered = "preregistered"
	MCPOAuthRegistrationCIMD          = "cimd"
	MCPOAuthRegistrationDCR           = "dcr"
)

// MCPOAuthStatus is the non-secret management view of one MCP OAuth connection.
type MCPOAuthStatus struct {
	ServerID                string     `json:"server_id"`
	Configured              bool       `json:"configured"`
	Connected               bool       `json:"connected"`
	ClientID                string     `json:"client_id,omitempty"`
	RegistrationMethod      string     `json:"registration_method,omitempty"`
	ClientIssuer            string     `json:"client_issuer,omitempty"`
	HasClientSecret         bool       `json:"has_client_secret"`
	HasRefreshToken         bool       `json:"has_refresh_token"`
	TokenEndpointAuthMethod string     `json:"token_endpoint_auth_method,omitempty"`
	Scope                   string     `json:"scope,omitempty"`
	RequiredScope           string     `json:"required_scope,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	AuthorizationServer     string     `json:"authorization_server,omitempty"`
	AuthorizationEndpoint   string     `json:"authorization_endpoint,omitempty"`
	TokenEndpoint           string     `json:"token_endpoint,omitempty"`
	ResourceMetadataURL     string     `json:"resource_metadata_url,omitempty"`
	RedirectURI             string     `json:"redirect_uri,omitempty"`
}

// ConfigureMCPOAuthInput stores OAuth client information. A nil client_secret
// preserves an existing encrypted secret; an explicit empty string clears it.
// CIMD clients use an HTTPS metadata-document URL as client_id and method none.
type ConfigureMCPOAuthInput struct {
	ClientID                string  `json:"client_id"`
	ClientSecret            *string `json:"client_secret,omitempty"`
	ClientIssuer            string  `json:"client_issuer,omitempty"`
	TokenEndpointAuthMethod string  `json:"token_endpoint_auth_method"`
	RegistrationMethod      string  `json:"registration_method,omitempty"`
}

// MCPOAuthAuthorizationStart contains browser URL and non-secret discovery data.
type MCPOAuthAuthorizationStart struct {
	AuthorizationURL    string `json:"authorization_url"`
	AuthorizationServer string `json:"authorization_server"`
	RegistrationMethod  string `json:"registration_method"`
	Scope               string `json:"scope,omitempty"`
	RedirectURI         string `json:"redirect_uri"`
}
