package mcpclient

import "encoding/json"

const (
	// ProtocolVersion is the latest MCP protocol revision supported by this client.
	ProtocolVersion = "2025-06-18"

	defaultRequestTimeout = 30
	maxRPCMessageBytes    = 4 << 20
	maxToolContentBytes   = 100 << 10
)

// Tool is the MCP server tool definition shape used by tools/list.
type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Annotations  json.RawMessage `json:"annotations,omitempty"`
}

type initializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ServerInfo      implementation  `json:"serverInfo,omitempty"`
}

type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type listToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolResult is the MCP tools/call result shape.
type CallToolResult struct {
	Content           []map[string]interface{} `json:"content"`
	StructuredContent map[string]interface{}   `json:"structuredContent,omitempty"`
	IsError           bool                     `json:"isError,omitempty"`
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Data) > 0 {
		return e.Message + ": " + string(e.Data)
	}
	return e.Message
}

type pendingResponse struct {
	result json.RawMessage
	err    error
}
