package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/apps"
)

// AppCatalogTool lists supported app families and declared scopes.
type AppCatalogTool struct{ service *apps.Service }

func NewAppCatalogTool(service *apps.Service) *AppCatalogTool {
	return &AppCatalogTool{service: service}
}

func (t *AppCatalogTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "app_catalog", Description: "List governed connected-app families, capabilities, and declared scopes.",
		Category: "apps", Enabled: t.service != nil, Version: "1", Risk: RiskLow,
		ReadOnly: true, SupportsParallel: true, DefaultTimeoutMS: 3000, MaxResultBytes: 65536,
		Parameters:   json.RawMessage(`{"type":"object","properties":{"app_key":{"type":"string"}}}`),
		OutputSchema: json.RawMessage(`{"type":"array"}`),
	}
}

func (t *AppCatalogTool) Validate(raw json.RawMessage) error {
	var args map[string]interface{}
	return json.Unmarshal(raw, &args)
}

func (t *AppCatalogTool) Execute(_ context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		AppKey string `json:"app_key"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.AppKey != "" {
		definition, ok := t.service.Definition(args.AppKey)
		if !ok {
			return nil, fmt.Errorf("unknown app %q", args.AppKey)
		}
		encoded, _ := json.Marshal(definition)
		return &ToolResult{Content: fmt.Sprintf("%s: %s\nCapabilities: %v\nAllowed scopes: %v", definition.DisplayName, definition.Description, definition.Capabilities, definition.AllowedScopes), Structured: encoded}, nil
	}
	catalog := t.service.Catalog()
	encoded, _ := json.Marshal(catalog)
	var builder strings.Builder
	for _, definition := range catalog {
		builder.WriteString(fmt.Sprintf("- %s (%s): %s\n", definition.DisplayName, definition.Key, definition.Description))
	}
	return &ToolResult{Content: strings.TrimSpace(builder.String()), Structured: encoded, Metadata: map[string]interface{}{"count": len(catalog)}}, nil
}

// AppConnectionsTool lists current user's app-to-MCP mappings.
type AppConnectionsTool struct{ service *apps.Service }

func NewAppConnectionsTool(service *apps.Service) *AppConnectionsTool {
	return &AppConnectionsTool{service: service}
}

func (t *AppConnectionsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "app_connections", Description: "List connected apps visible to the current user and workspace. App actions are exposed as governed MCP tools.",
		Category: "apps", Enabled: t.service != nil, Version: "1", Risk: RiskLow,
		ReadOnly: true, SupportsParallel: true, DefaultTimeoutMS: 3000, MaxResultBytes: 65536,
		Parameters: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"array"}`),
	}
}

func (t *AppConnectionsTool) Validate(raw json.RawMessage) error {
	var value map[string]interface{}
	return json.Unmarshal(raw, &value)
}

func (t *AppConnectionsTool) Execute(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	scope := InvocationScopeFromContext(ctx)
	connections, err := t.service.List(scope.UserID, scope.WorkspaceID)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(connections)
	if len(connections) == 0 {
		return &ToolResult{Content: "No connected apps in the current scope.", Structured: encoded}, nil
	}
	var builder strings.Builder
	for _, connection := range connections {
		builder.WriteString(fmt.Sprintf("- %s (%s), scopes: %s, MCP server: %s\n", connection.DisplayName, connection.AppKey, strings.Join(connection.Scopes, ", "), connection.ServerID))
	}
	return &ToolResult{Content: strings.TrimSpace(builder.String()), Structured: encoded, Metadata: map[string]interface{}{"count": len(connections)}}, nil
}

// AppConnectMCPTool registers an existing MCP server as a governed app.
type AppConnectMCPTool struct{ service *apps.Service }

func NewAppConnectMCPTool(service *apps.Service) *AppConnectMCPTool {
	return &AppConnectMCPTool{service: service}
}

func (t *AppConnectMCPTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "app_connect_mcp", Description: "Map an existing configured MCP server to a governed connected-app definition and declared scopes.",
		Category: "apps", Enabled: t.service != nil, Version: "1", Risk: RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 16384,
		Parameters: json.RawMessage(`{
			"type":"object","required":["app_key","server_id","scopes"],
			"properties":{
				"app_key":{"type":"string"},"display_name":{"type":"string"},
				"server_id":{"type":"string"},"workspace_id":{"type":"string"},
				"scopes":{"type":"array","items":{"type":"string"}},
				"metadata":{"type":"object"}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

type appConnectArgs struct {
	AppKey      string          `json:"app_key"`
	DisplayName string          `json:"display_name"`
	ServerID    string          `json:"server_id"`
	WorkspaceID string          `json:"workspace_id"`
	Scopes      []string        `json:"scopes"`
	Metadata    json.RawMessage `json:"metadata"`
}

func (t *AppConnectMCPTool) Validate(raw json.RawMessage) error {
	var args appConnectArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.AppKey) == "" || strings.TrimSpace(args.ServerID) == "" {
		return fmt.Errorf("app_key and server_id are required")
	}
	if len(args.Scopes) == 0 {
		return fmt.Errorf("at least one declared scope is required")
	}
	return nil
}

func (t *AppConnectMCPTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args appConnectArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	scope := InvocationScopeFromContext(ctx)
	workspaceID := args.WorkspaceID
	if workspaceID == "" {
		workspaceID = scope.WorkspaceID
	}
	connection, err := t.service.ConnectMCP(scope.UserID, workspaceID, args.AppKey, args.DisplayName, args.ServerID, args.Scopes, args.Metadata)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(connection)
	return &ToolResult{Content: fmt.Sprintf("Connected %s through MCP server %s. Write-capable MCP tools remain subject to their individual approval policies.", connection.DisplayName, connection.ServerID), Structured: encoded, Metadata: map[string]interface{}{"connection_id": connection.ID}}, nil
}

// AppDisconnectTool removes an app mapping but does not delete the MCP server.
type AppDisconnectTool struct{ service *apps.Service }

func NewAppDisconnectTool(service *apps.Service) *AppDisconnectTool {
	return &AppDisconnectTool{service: service}
}

func (t *AppDisconnectTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "app_disconnect", Description: "Remove a connected-app mapping. The underlying MCP server remains configured until an administrator removes it.",
		Category: "apps", Enabled: t.service != nil, Version: "1", Risk: RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 8192,
		Parameters: json.RawMessage(`{"type":"object","required":["connection_id"],"properties":{"connection_id":{"type":"string"}}}`),
	}
}

func (t *AppDisconnectTool) Validate(raw json.RawMessage) error {
	var args struct {
		ConnectionID string `json:"connection_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.ConnectionID) == "" {
		return fmt.Errorf("connection_id is required")
	}
	return nil
}

func (t *AppDisconnectTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		ConnectionID string `json:"connection_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if err := t.service.Delete(args.ConnectionID, InvocationScopeFromContext(ctx).UserID); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(map[string]interface{}{"connection_id": args.ConnectionID, "disconnected": true})
	return &ToolResult{Content: "Disconnected app mapping: " + args.ConnectionID, Structured: encoded}, nil
}
