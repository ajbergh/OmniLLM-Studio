package tools

import (
	"context"
	"encoding/json"
)

// ToolDefinition describes a registered tool's metadata and schema.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for arguments
	Category    string          `json:"category"`   // "search", "compute", "fetch", etc.
	Enabled     bool            `json:"enabled"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Definition returns the tool's metadata and schema.
	Definition() ToolDefinition
	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error)
	// Validate checks whether the given arguments are valid for this tool.
	Validate(args json.RawMessage) error
}

// ToolCall represents a request to invoke a tool.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ApprovalRequest describes a tool call waiting on an "ask" policy decision.
type ApprovalRequest struct {
	ToolCallID  string          `json:"tool_call_id"`
	ToolName    string          `json:"tool_name"`
	Description string          `json:"description"`
	Arguments   json.RawMessage `json:"arguments"`
}

// ApprovalHandler returns true to allow the tool call to proceed.
type ApprovalHandler func(ctx context.Context, req ApprovalRequest) (bool, error)

// ToolResult holds the output of a tool execution.
type ToolResult struct {
	ToolCallID string                 `json:"tool_call_id"`
	Content    string                 `json:"content"`
	IsError    bool                   `json:"is_error"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
