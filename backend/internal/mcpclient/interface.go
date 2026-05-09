package mcpclient

import (
	"context"
	"encoding/json"
)

// MCPClient is the common interface for all MCP transport implementations.
// Both *Client (stdio) and *HTTPClient (Streamable HTTP) satisfy this interface.
type MCPClient interface {
	// Start initialises the connection to the MCP server and performs the MCP
	// initialization handshake.
	Start(ctx context.Context) error

	// Stop tears down the connection.  Implementations should make a
	// best-effort attempt and must not block indefinitely.
	Stop(ctx context.Context) error

	// ListTools queries the server for all tools it exposes, following
	// pagination cursors automatically.
	ListTools(ctx context.Context) ([]Tool, error)

	// CallTool invokes a named tool on the MCP server.
	CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error)
}
