package models

import "time"

// OpenAPIServer describes a governed OpenAPI-backed tool server. API credentials
// are never returned by the management API; HasAPIKey indicates configuration.
type OpenAPIServer struct {
	ID                  string            `json:"id"`
	OwnerUserID         string            `json:"owner_user_id"`
	Name                string            `json:"name"`
	BaseURL             string            `json:"base_url"`
	SpecJSON            string            `json:"spec_json"`
	Enabled             bool              `json:"enabled"`
	AllowPrivateNetwork bool              `json:"allow_private_network"`
	AuthHeader          string            `json:"auth_header,omitempty"`
	AuthPrefix          string            `json:"auth_prefix,omitempty"`
	HasAPIKey           bool              `json:"has_api_key"`
	Tools               []OpenAPIToolInfo `json:"tools,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// OpenAPIToolInfo is compact metadata for a generated operation tool.
type OpenAPIToolInfo struct {
	Name        string `json:"name"`
	OperationID string `json:"operation_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Policy      string `json:"policy,omitempty"`
}

// OpenAPIServerRuntime adds decrypted credentials for backend-only execution.
type OpenAPIServerRuntime struct {
	OpenAPIServer
	APIKey string `json:"-"`
}
