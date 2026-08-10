package models

import "time"

const (
	MCPOAuthAuthMethodNone              = "none"
	MCPOAuthAuthMethodClientSecretBasic = "client_secret_basic"
	MCPOAuthAuthMethodClientSecretPost  = "client_secret_post"
)

// MCPOAuthStatus is the non-secret management view of one MCP OAuth connection.
type MCPOAuthStatus struct {
	ServerID                string     `json:"server_id"`
	Configured              bool       `json:"configured"`
	Connected               bool       `json:"connected"`
	ClientID                string     `json:"client_id,omitempty"`
	HasClientSecret         bool       `json:"has_client_secret"`
	HasRefreshToken         bool       `json:"has_refresh_token"`
	TokenEndpointAuthMethod string     `json:"token_endpoint_auth_method,omitempty"`
	Scope                   string     `json:"scope,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	AuthorizationServer     string     `json:"authorization_server,omitempty"`
	AuthorizationEndpoint   string     `json:"authorization_endpoint,omitempty"`
	TokenEndpoint           string     `json:"token_endpoint,omitempty"`
	ResourceMetadataURL     string     `json:"resource_metadata_url,omitempty"`
	RedirectURI             string     `json:"redirect_uri,omitempty"`
}

// ConfigureMCPOAuthInput stores preregistered OAuth client information. A nil
// client_secret preserves an existing encrypted secret; an explicit empty string
// clears it. Secrets are never returned by management APIs.
type ConfigureMCPOAuthInput struct {
	ClientID                string  `json:"client_id"`
	ClientSecret            *string `json:"client_secret,omitempty"`
	TokenEndpointAuthMethod string  `json:"token_endpoint_auth_method"`
}

// MCPOAuthAuthorizationStart contains the browser URL and non-secret discovery
// information required to explain the authorization flow in the UI.
type MCPOAuthAuthorizationStart struct {
	AuthorizationURL    string `json:"authorization_url"`
	AuthorizationServer string `json:"authorization_server"`
	Scope               string `json:"scope,omitempty"`
	RedirectURI         string `json:"redirect_uri"`
}
