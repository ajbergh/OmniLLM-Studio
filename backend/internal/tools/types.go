package tools

import (
	"context"
	"encoding/json"
)

// RiskLevel describes the potential impact of a tool invocation.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// ToolExample provides a compact example that can be shown to planners and users.
type ToolExample struct {
	Description string          `json:"description,omitempty"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
}

// ToolDefinition describes a registered tool's metadata and contract.
//
// The additional v2 fields are optional so existing built-in and MCP tools remain
// source-compatible while richer tools can advertise risk, output, execution,
// and parallelism characteristics to the orchestrator.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for arguments
	Category    string          `json:"category"`   // "search", "compute", "fetch", etc.
	Enabled     bool            `json:"enabled"`

	Version             string          `json:"version,omitempty"`
	OutputSchema        json.RawMessage `json:"output_schema,omitempty"`
	Risk                RiskLevel       `json:"risk,omitempty"`
	ReadOnly            bool            `json:"read_only,omitempty"`
	SideEffecting       bool            `json:"side_effecting,omitempty"`
	RequiresNetwork     bool            `json:"requires_network,omitempty"`
	SupportsParallel    bool            `json:"supports_parallel,omitempty"`
	RequiresCredentials bool            `json:"requires_credentials,omitempty"`
	DefaultTimeoutMS    int             `json:"default_timeout_ms,omitempty"`
	MaxResultBytes      int             `json:"max_result_bytes,omitempty"`
	Examples            []ToolExample   `json:"examples,omitempty"`
}

// Normalized returns a definition with safe defaults for legacy tools.
func (d ToolDefinition) Normalized() ToolDefinition {
	if d.Version == "" {
		d.Version = "1"
	}
	if d.Risk == "" {
		if d.SideEffecting {
			d.Risk = RiskHigh
		} else {
			d.Risk = RiskLow
		}
	}
	if !d.SideEffecting && !d.ReadOnly {
		// Legacy tools were predominantly read-only. Preserve that behavior while
		// requiring new side-effecting tools to opt in explicitly.
		d.ReadOnly = true
	}
	return d
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

// InvocationScope identifies the user-visible context in which a tool runs.
type InvocationScope struct {
	UserID         string `json:"user_id,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
}

// ApprovalRequest describes a tool call waiting on an "ask" policy decision.
type ApprovalRequest struct {
	ApprovalID  string          `json:"approval_id,omitempty"`
	ToolCallID  string          `json:"tool_call_id"`
	ToolName    string          `json:"tool_name"`
	Description string          `json:"description"`
	Arguments   json.RawMessage `json:"arguments"`
	Scope       InvocationScope `json:"scope,omitempty"`
	Risk        RiskLevel       `json:"risk,omitempty"`
	ReadOnly    bool            `json:"read_only,omitempty"`
}

// ApprovalHandler returns true to allow the tool call to proceed.
type ApprovalHandler func(ctx context.Context, req ApprovalRequest) (bool, error)

// ToolArtifact references an output produced by a tool invocation.
type ToolArtifact struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
}

// ToolResult holds the output of a tool execution.
type ToolResult struct {
	ToolCallID string                 `json:"tool_call_id"`
	Content    string                 `json:"content"`
	IsError    bool                   `json:"is_error"`
	Structured json.RawMessage        `json:"structured,omitempty"`
	Artifacts  []ToolArtifact         `json:"artifacts,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
