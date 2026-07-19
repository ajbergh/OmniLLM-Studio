// Package apps provides a user-facing governed catalog over MCP connections.
package apps

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Capability describes a user-visible class of connected-app behavior.
type Capability string

const (
	CapabilitySearch Capability = "search"
	CapabilityFetch  Capability = "fetch"
	CapabilitySync   Capability = "sync"
	CapabilityCreate Capability = "create"
	CapabilityUpdate Capability = "update"
	CapabilityDelete Capability = "delete"
)

// Definition describes an installable app family. Runtime actions are supplied
// by its linked MCP server and remain governed by per-tool policies.
type Definition struct {
	Key             string       `json:"key"`
	DisplayName     string       `json:"display_name"`
	Description     string       `json:"description"`
	Category        string       `json:"category"`
	ConnectionTypes []string     `json:"connection_types"`
	Capabilities    []Capability `json:"capabilities"`
	AllowedScopes   []string     `json:"allowed_scopes"`
	WriteScopes     []string     `json:"write_scopes,omitempty"`
}

// Connection maps a user/workspace app to an existing MCP server.
type Connection struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id,omitempty"`
	WorkspaceID    string          `json:"workspace_id,omitempty"`
	AppKey         string          `json:"app_key"`
	DisplayName    string          `json:"display_name"`
	ConnectionType string          `json:"connection_type"`
	ServerID       string          `json:"server_id,omitempty"`
	Scopes         []string        `json:"scopes"`
	Status         string          `json:"status"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Service struct {
	db      *sql.DB
	catalog map[string]Definition
}

func NewService(db *sql.DB) *Service {
	definitions := builtinDefinitions()
	catalog := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		catalog[definition.Key] = definition
	}
	return &Service{db: db, catalog: catalog}
}

func builtinDefinitions() []Definition {
	return []Definition{
		{Key: "github", DisplayName: "GitHub", Description: "Repositories, issues, pull requests, reviews, and governed repository actions.", Category: "development", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate, CapabilityUpdate}, AllowedScopes: []string{"repo:read", "issues:read", "issues:write", "pull_requests:read", "pull_requests:write", "contents:read", "contents:write"}, WriteScopes: []string{"issues:write", "pull_requests:write", "contents:write"}},
		{Key: "google_workspace", DisplayName: "Google Workspace", Description: "Drive, Gmail, Calendar, and Contacts through a configured MCP server.", Category: "productivity", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate, CapabilityUpdate}, AllowedScopes: []string{"drive:read", "drive:write", "gmail:read", "gmail:send", "calendar:read", "calendar:write", "contacts:read"}, WriteScopes: []string{"drive:write", "gmail:send", "calendar:write"}},
		{Key: "microsoft_365", DisplayName: "Microsoft 365", Description: "OneDrive, SharePoint, Outlook, Calendar, and Contacts through a configured MCP server.", Category: "productivity", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate, CapabilityUpdate}, AllowedScopes: []string{"files:read", "files:write", "mail:read", "mail:send", "calendar:read", "calendar:write", "contacts:read"}, WriteScopes: []string{"files:write", "mail:send", "calendar:write"}},
		{Key: "slack", DisplayName: "Slack", Description: "Search channels and messages, then perform explicitly approved messaging actions.", Category: "communication", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate}, AllowedScopes: []string{"messages:read", "messages:write", "channels:read", "users:read"}, WriteScopes: []string{"messages:write"}},
		{Key: "notion", DisplayName: "Notion", Description: "Search, read, create, and update pages and databases.", Category: "knowledge", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate, CapabilityUpdate}, AllowedScopes: []string{"content:read", "content:write", "databases:read", "databases:write"}, WriteScopes: []string{"content:write", "databases:write"}},
		{Key: "custom_mcp", DisplayName: "Custom MCP App", Description: "Govern any configured MCP server as a user-visible connected app.", Category: "custom", ConnectionTypes: []string{"mcp"}, Capabilities: []Capability{CapabilitySearch, CapabilityFetch, CapabilityCreate, CapabilityUpdate, CapabilityDelete}, AllowedScopes: []string{"read", "write", "delete"}, WriteScopes: []string{"write", "delete"}},
	}
}

func (s *Service) Catalog() []Definition {
	out := make([]Definition, 0, len(s.catalog))
	for _, definition := range s.catalog {
		out = append(out, definition)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out
}

func (s *Service) Definition(key string) (Definition, bool) {
	definition, ok := s.catalog[strings.ToLower(strings.TrimSpace(key))]
	return definition, ok
}

func (s *Service) ConnectMCP(userID, workspaceID, appKey, displayName, serverID string, scopes []string, metadata json.RawMessage) (*Connection, error) {
	if userID == "" || strings.TrimSpace(serverID) == "" {
		return nil, fmt.Errorf("user_id and server_id are required")
	}
	definition, ok := s.Definition(appKey)
	if !ok {
		return nil, fmt.Errorf("unknown app %q", appKey)
	}
	if displayName = strings.TrimSpace(displayName); displayName == "" {
		displayName = definition.DisplayName
	}
	var serverExists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mcp_servers WHERE id = ?`, serverID).Scan(&serverExists); err != nil {
		return nil, fmt.Errorf("verify MCP server: %w", err)
	}
	if serverExists == 0 {
		return nil, fmt.Errorf("MCP server not found")
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(metadata) {
		return nil, fmt.Errorf("metadata must be valid JSON")
	}
	allowed := make(map[string]bool, len(definition.AllowedScopes))
	for _, scope := range definition.AllowedScopes {
		allowed[scope] = true
	}
	normalizedScopes := make([]string, 0, len(scopes))
	seen := map[string]bool{}
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" || seen[scope] {
			continue
		}
		if !allowed[scope] {
			return nil, fmt.Errorf("scope %q is not declared by app %s", scope, definition.Key)
		}
		seen[scope] = true
		normalizedScopes = append(normalizedScopes, scope)
	}
	sort.Strings(normalizedScopes)
	scopesJSON, _ := json.Marshal(normalizedScopes)
	now := time.Now().UTC()
	connection := &Connection{
		ID: uuid.NewString(), UserID: userID, WorkspaceID: workspaceID, AppKey: definition.Key,
		DisplayName: displayName, ConnectionType: "mcp", ServerID: serverID,
		Scopes: normalizedScopes, Status: "configured", Metadata: metadata, CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.db.Exec(`
		INSERT INTO app_connections (
			id, user_id, workspace_id, app_key, display_name, connection_type, server_id,
			scopes_json, status, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'mcp', ?, ?, 'configured', ?, ?, ?)
	`, connection.ID, connection.UserID, connection.WorkspaceID, connection.AppKey,
		connection.DisplayName, connection.ServerID, string(scopesJSON), string(connection.Metadata),
		connection.CreatedAt, connection.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("connect app: %w", err)
	}
	return connection, nil
}

func (s *Service) List(userID, workspaceID string) ([]Connection, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, workspace_id, app_key, display_name, connection_type, server_id,
			scopes_json, status, metadata_json, created_at, updated_at
		FROM app_connections
		WHERE user_id = ? AND (? = '' OR workspace_id = '' OR workspace_id = ?)
		ORDER BY display_name ASC
	`, userID, workspaceID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list app connections: %w", err)
	}
	defer rows.Close()
	out := make([]Connection, 0)
	for rows.Next() {
		var connection Connection
		var scopes, metadata string
		if err := rows.Scan(&connection.ID, &connection.UserID, &connection.WorkspaceID, &connection.AppKey,
			&connection.DisplayName, &connection.ConnectionType, &connection.ServerID,
			&scopes, &connection.Status, &metadata, &connection.CreatedAt, &connection.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan app connection: %w", err)
		}
		_ = json.Unmarshal([]byte(scopes), &connection.Scopes)
		connection.Metadata = json.RawMessage(metadata)
		out = append(out, connection)
	}
	return out, rows.Err()
}

func (s *Service) Delete(id, userID string) error {
	result, err := s.db.Exec(`DELETE FROM app_connections WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("disconnect app: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("app connection not found")
	}
	return nil
}
