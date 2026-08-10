package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSearch exposes compact request-scoped tool metadata for large catalogs.
type ToolSearch struct{ registry *Registry }

func NewToolSearch(registry *Registry) *ToolSearch { return &ToolSearch{registry: registry} }

func (t *ToolSearch) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "tool_search",
		Description:      "Search the available tool catalog by capability and return compact matching metadata without loading every tool schema.",
		Category:         "orchestration",
		Enabled:          t != nil && t.registry != nil,
		ReadOnly:         true,
		SupportsParallel: true,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","minLength":1},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["query"],"additionalProperties":false}`),
	}
}

func (t *ToolSearch) Validate(args json.RawMessage) error {
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if in.Limit < 0 || in.Limit > 20 {
		return fmt.Errorf("limit must be between 1 and 20")
	}
	return nil
}

func (t *ToolSearch) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t == nil || t.registry == nil {
		return nil, fmt.Errorf("tool registry unavailable")
	}
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &in)
	if in.Limit == 0 {
		in.Limit = 8
	}
	terms := strings.Fields(strings.ToLower(in.Query))
	defs := t.registry.SelectForContext(ctx, terms, in.Limit+2)
	type item struct {
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Category    string    `json:"category"`
		Risk        RiskLevel `json:"risk"`
		ReadOnly    bool      `json:"read_only"`
	}
	out := make([]item, 0, in.Limit)
	for _, def := range defs {
		if def.Name == "tool_search" || def.Name == "tool_invoke" {
			continue
		}
		out = append(out, item{Name: def.Name, Description: def.Description, Category: def.Category, Risk: def.Risk, ReadOnly: def.ReadOnly})
		if len(out) >= in.Limit {
			break
		}
	}
	data, _ := json.Marshal(out)
	return &ToolResult{Content: string(data), Structured: data, Metadata: map[string]interface{}{"matches": len(out)}}, nil
}

// ToolInvoke is the generic execution companion for deferred discovery. The child
// invocation re-enters Executor, so its own policy/approval/restrictions remain authoritative.
type ToolInvoke struct{ executor *Executor }

func NewToolInvoke(executor *Executor) *ToolInvoke { return &ToolInvoke{executor: executor} }

func (t *ToolInvoke) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "tool_invoke",
		Description:      "Invoke one registered tool by name after discovering it with tool_search. The child tool still enforces its normal policy, approval, timeout, and request restrictions.",
		Category:         "orchestration",
		Enabled:          t != nil && t.executor != nil,
		Risk:             RiskHigh,
		SideEffecting:    true,
		ReadOnly:         false,
		SupportsParallel: false,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","minLength":1},"arguments":{"type":"object"}},"required":["name","arguments"],"additionalProperties":false}`),
	}
}

func (t *ToolInvoke) Validate(args json.RawMessage) error {
	var in struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return err
	}
	if in.Name == "" {
		return fmt.Errorf("name is required")
	}
	if in.Name == "tool_invoke" {
		return fmt.Errorf("recursive tool_invoke is not allowed")
	}
	return nil
}

func (t *ToolInvoke) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t == nil || t.executor == nil {
		return nil, fmt.Errorf("tool executor unavailable")
	}
	var in struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	_ = json.Unmarshal(args, &in)
	if !ToolAllowedByContext(ctx, in.Name) {
		return &ToolResult{Content: fmt.Sprintf("tool %q is excluded by the current request restriction", in.Name), IsError: true, Metadata: map[string]interface{}{"error_code": "TOOL_RESTRICTED"}}, nil
	}
	encoded, _ := json.Marshal(in.Arguments)
	child := t.executor.Execute(ctx, ToolCall{ID: "tool-invoke-child", Name: in.Name, Arguments: encoded})
	if child == nil {
		return nil, fmt.Errorf("child tool returned no result")
	}
	structured, _ := json.Marshal(child)
	copyResult := *child
	copyResult.Structured = structured
	return &copyResult, nil
}
