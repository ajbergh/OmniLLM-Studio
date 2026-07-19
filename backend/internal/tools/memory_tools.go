package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	memorysvc "github.com/ajbergh/omnillm-studio/internal/memory"
)

// MemorySearchTool retrieves only explicitly stored memories visible to the
// current user/workspace/conversation.
type MemorySearchTool struct{ service *memorysvc.Service }

func NewMemorySearchTool(service *memorysvc.Service) *MemorySearchTool {
	return &MemorySearchTool{service: service}
}

func (t *MemorySearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "memory_search", Description: "Search explicit user-controlled memories and project decisions visible in the current scope.",
		Category: "memory", Enabled: t.service != nil, Version: "1", Risk: RiskLow,
		ReadOnly: true, SupportsParallel: true, DefaultTimeoutMS: 3000, MaxResultBytes: 65536,
		Parameters:   json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}}}`),
		OutputSchema: json.RawMessage(`{"type":"array"}`),
	}
}

func (t *MemorySearchTool) Validate(raw json.RawMessage) error {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if args.Limit < 0 || args.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	return nil
}

func (t *MemorySearchTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	scope := InvocationScopeFromContext(ctx)
	items, err := t.service.List(memorysvc.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, args.Query, args.Limit)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(items)
	if len(items) == 0 {
		return &ToolResult{Content: "No matching saved memories.", Structured: encoded, Metadata: map[string]interface{}{"count": 0}}, nil
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("- [%s] %s (memory_id: %s)\n", item.Kind, item.Content, item.ID))
	}
	return &ToolResult{Content: strings.TrimSpace(builder.String()), Structured: encoded, Metadata: map[string]interface{}{"count": len(items)}}, nil
}

// MemorySaveTool stores one explicit memory and requires approval by default.
type MemorySaveTool struct{ service *memorysvc.Service }

func NewMemorySaveTool(service *memorysvc.Service) *MemorySaveTool {
	return &MemorySaveTool{service: service}
}

func (t *MemorySaveTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "memory_save", Description: "Save an explicit preference, project decision, workspace fact, conversation note, or temporary memory. Credentials and sensitive identifiers are rejected.",
		Category: "memory", Enabled: t.service != nil, Version: "1", Risk: RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 3000, MaxResultBytes: 16384,
		Parameters: json.RawMessage(`{
			"type":"object","required":["kind","content"],
			"properties":{
				"kind":{"type":"string","enum":["preference","workspace_knowledge","project_decision","conversation","temporary"]},
				"content":{"type":"string","minLength":1,"maxLength":4000},
				"source_message_id":{"type":"string"},
				"expires_at":{"type":"string","description":"Optional RFC3339 expiration"}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

func (t *MemorySaveTool) Validate(raw json.RawMessage) error {
	var args struct {
		Kind      string `json:"kind"`
		Content   string `json:"content"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Kind) == "" || strings.TrimSpace(args.Content) == "" {
		return fmt.Errorf("kind and content are required")
	}
	if len(args.Content) > 4000 {
		return fmt.Errorf("content exceeds 4000 characters")
	}
	if args.ExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, args.ExpiresAt); err != nil {
			return fmt.Errorf("expires_at must be RFC3339")
		}
	}
	return nil
}

func (t *MemorySaveTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		Kind            string `json:"kind"`
		Content         string `json:"content"`
		SourceMessageID string `json:"source_message_id"`
		ExpiresAt       string `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	var expiresAt *time.Time
	if args.ExpiresAt != "" {
		parsed, _ := time.Parse(time.RFC3339, args.ExpiresAt)
		expiresAt = &parsed
	}
	scope := InvocationScopeFromContext(ctx)
	item, err := t.service.Save(memorysvc.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, args.Kind, args.Content, args.SourceMessageID, expiresAt)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(item)
	return &ToolResult{Content: fmt.Sprintf("Saved %s memory %s.", item.Kind, item.ID), Structured: encoded, Metadata: map[string]interface{}{"memory_id": item.ID}}, nil
}

// MemoryDeleteTool permanently deletes one owned memory.
type MemoryDeleteTool struct{ service *memorysvc.Service }

func NewMemoryDeleteTool(service *memorysvc.Service) *MemoryDeleteTool {
	return &MemoryDeleteTool{service: service}
}

func (t *MemoryDeleteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "memory_delete", Description: "Permanently delete one saved memory owned by the current user.",
		Category: "memory", Enabled: t.service != nil, Version: "1", Risk: RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 3000, MaxResultBytes: 8192,
		Parameters: json.RawMessage(`{"type":"object","required":["memory_id"],"properties":{"memory_id":{"type":"string"}}}`),
	}
}

func (t *MemoryDeleteTool) Validate(raw json.RawMessage) error {
	var args struct {
		MemoryID string `json:"memory_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.MemoryID) == "" {
		return fmt.Errorf("memory_id is required")
	}
	return nil
}

func (t *MemoryDeleteTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		MemoryID string `json:"memory_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if err := t.service.Delete(args.MemoryID, InvocationScopeFromContext(ctx).UserID); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(map[string]interface{}{"memory_id": args.MemoryID, "deleted": true})
	return &ToolResult{Content: "Memory deleted: " + args.MemoryID, Structured: encoded}, nil
}
