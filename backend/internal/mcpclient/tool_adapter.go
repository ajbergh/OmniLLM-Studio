// Package mcpclient provides the Model Context Protocol (MCP) client implementation.
// This file implements an adapter that bridges remote MCP tools to the local
// OmniLLM-Studio tool registry.
package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// AuditFunc records MCP events without coupling tool execution to repository details.
type AuditFunc func(event models.MCPAuditEvent)

// ToolAdapter wraps an MCP tool as an OmniLLM tool.
type ToolAdapter struct {
	serverID     string
	serverName   string
	internalName string
	tool         Tool
	client       MCPClient
	audit        AuditFunc
}

// NewToolAdapter creates a Tool implementation backed by an MCP server.
func NewToolAdapter(serverID, serverName, internalName string, tool Tool, client MCPClient, audit AuditFunc) *ToolAdapter {
	return &ToolAdapter{
		serverID:     serverID,
		serverName:   serverName,
		internalName: internalName,
		tool:         tool,
		client:       client,
		audit:        audit,
	}
}

type toolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

func parseToolAnnotations(raw json.RawMessage) toolAnnotations {
	var annotations toolAnnotations
	if len(bytes.TrimSpace(raw)) == 0 {
		return annotations
	}
	_ = json.Unmarshal(raw, &annotations)
	return annotations
}

// Definition returns the tool metadata exposed to OmniLLM.
func (a *ToolAdapter) Definition() tools.ToolDefinition {
	params := a.tool.InputSchema
	if len(bytes.TrimSpace(params)) == 0 {
		params = json.RawMessage(`{"type":"object"}`)
	}

	description := strings.TrimSpace(a.tool.Description)
	if description == "" && a.tool.Title != "" {
		description = a.tool.Title
	}
	if description == "" {
		description = fmt.Sprintf("MCP tool %s from server %s.", a.tool.Name, a.serverName)
	}

	annotations := parseToolAnnotations(a.tool.Annotations)
	readOnly := annotations.ReadOnlyHint != nil && *annotations.ReadOnlyHint
	// MCP annotations are hints. The MCP specification defaults readOnlyHint to
	// false, so an omitted or malformed annotation must be treated conservatively
	// as potentially side-effecting. This prevents automatic retries of unknown
	// remote write tools.
	sideEffecting := !readOnly
	risk := tools.RiskHigh
	if readOnly {
		risk = tools.RiskLow
	}

	return tools.ToolDefinition{
		Name:             a.internalName,
		Description:      description,
		Parameters:       params,
		OutputSchema:     a.tool.OutputSchema,
		Category:         "mcp",
		Enabled:          true,
		Version:          "1",
		Risk:             risk,
		ReadOnly:         readOnly,
		SideEffecting:    sideEffecting,
		RequiresNetwork:  true,
		SupportsParallel: readOnly,
	}
}

// Validate checks that arguments are a JSON object. Full JSON Schema validation
// is deferred to the MCP server for the MVP.
func (a *ToolAdapter) Validate(args json.RawMessage) error {
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return fmt.Errorf("invalid JSON object: %w", err)
	}
	if obj == nil {
		return fmt.Errorf("arguments must be a JSON object")
	}
	return nil
}

// Execute calls the MCP server and normalizes its response.
func (a *ToolAdapter) Execute(ctx context.Context, args json.RawMessage) (*tools.ToolResult, error) {
	start := time.Now()
	result, err := a.client.CallTool(ctx, a.tool.Name, args)
	duration := int(time.Since(start).Milliseconds())

	inputJSON := string(args)
	if inputJSON == "" {
		inputJSON = "{}"
	}

	if err != nil {
		if a.audit != nil {
			msg := err.Error()
			a.audit(models.MCPAuditEvent{
				ServerID:   a.serverID,
				EventType:  "error",
				ToolName:   &a.internalName,
				InputJSON:  &inputJSON,
				DurationMs: &duration,
				ErrorMsg:   &msg,
			})
		}
		return nil, err
	}

	normalized := NormalizeToolResult(result)
	if a.audit != nil {
		outputJSON := normalized.Content
		if len(outputJSON) > 4096 {
			outputJSON = outputJSON[:4096] + "\n[Audit output truncated]"
		}
		a.audit(models.MCPAuditEvent{
			ServerID:   a.serverID,
			EventType:  "tool_call",
			ToolName:   &a.internalName,
			InputJSON:  &inputJSON,
			OutputJSON: &outputJSON,
			DurationMs: &duration,
		})
	}

	return normalized, nil
}

// NormalizeToolResult converts MCP tool content blocks to the existing ToolResult shape.
func NormalizeToolResult(result *CallToolResult) *tools.ToolResult {
	if result == nil {
		return &tools.ToolResult{Content: "", IsError: true}
	}

	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		parts = append(parts, normalizeContentBlock(item))
	}
	content := strings.TrimSpace(strings.Join(nonEmpty(parts), "\n"))
	if content == "" && result.StructuredContent != nil {
		if data, err := json.Marshal(result.StructuredContent); err == nil {
			content = string(data)
		}
	}

	content = truncateContent(content, maxToolContentBytes)
	metadata := map[string]interface{}{
		"mcp_content_count": len(result.Content),
	}
	if result.StructuredContent != nil {
		metadata["structured_content"] = result.StructuredContent
	}

	return &tools.ToolResult{
		Content:  content,
		IsError:  result.IsError,
		Metadata: metadata,
	}
}

func normalizeContentBlock(item map[string]interface{}) string {
	contentType, _ := item["type"].(string)
	switch contentType {
	case "text":
		if text, ok := item["text"].(string); ok {
			return text
		}
	case "image":
		mimeType, _ := item["mimeType"].(string)
		if mimeType == "" {
			mimeType = "image"
		}
		return "[Image: " + mimeType + "]"
	case "audio":
		mimeType, _ := item["mimeType"].(string)
		if mimeType == "" {
			mimeType = "audio"
		}
		return "[Audio: " + mimeType + "]"
	case "resource_link":
		name, _ := item["name"].(string)
		uri, _ := item["uri"].(string)
		if name != "" && uri != "" {
			return "[Resource: " + name + " " + uri + "]"
		}
		if uri != "" {
			return "[Resource: " + uri + "]"
		}
	case "resource":
		if resource, ok := item["resource"].(map[string]interface{}); ok {
			if text, ok := resource["text"].(string); ok {
				return text
			}
			if uri, ok := resource["uri"].(string); ok {
				return "[Resource: " + uri + "]"
			}
			if mimeType, ok := resource["mimeType"].(string); ok {
				return "[Resource: " + mimeType + "]"
			}
	}

	data, err := json.Marshal(item)
	if err != nil {
		return ""
	}
	return string(data)
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func truncateContent(content string, maxBytes int) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	omitted := len(content) - maxBytes
	return content[:maxBytes] + fmt.Sprintf("\n[Truncated: %d bytes]", omitted)
}
