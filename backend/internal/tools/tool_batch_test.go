package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type batchTestTool struct {
	name  string
	calls *int
}

func (t batchTestTool) Definition() ToolDefinition {
	return ToolDefinition{Name: t.name, Description: "batch test", Category: "test", Enabled: true, ReadOnly: true, Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (t batchTestTool) Validate(json.RawMessage) error { return nil }
func (t batchTestTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	*t.calls++
	return &ToolResult{Content: "ok"}, nil
}

func TestToolBatchUsesExecutorAndRequestRestriction(t *testing.T) {
	registry := NewRegistry()
	allowedCalls := 0
	blockedCalls := 0
	registry.MustRegister(batchTestTool{name: "allowed_child", calls: &allowedCalls})
	registry.MustRegister(batchTestTool{name: "blocked_child", calls: &blockedCalls})
	executor := NewExecutor(registry, func(string) string { return "allow" }, 0)
	batch := NewToolBatch(executor)
	ctx := ContextWithToolRestriction(context.Background(), []string{"tool_batch", "allowed_child"})
	args := json.RawMessage(`{"calls":[{"name":"allowed_child","arguments":{}},{"name":"blocked_child","arguments":{}}],"continue_on_error":true}`)
	result, err := batch.Execute(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	if allowedCalls != 1 || blockedCalls != 0 {
		t.Fatalf("calls allowed=%d blocked=%d", allowedCalls, blockedCalls)
	}
	if !result.IsError || !strings.Contains(result.Content, "excluded by the current request restriction") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestToolBatchRejectsRecursion(t *testing.T) {
	batch := NewToolBatch(NewExecutor(NewRegistry(), nil, 0))
	if err := batch.Validate(json.RawMessage(`{"calls":[{"name":"tool_batch","arguments":{}}]}`)); err == nil {
		t.Fatal("nested tool_batch must be rejected")
	}
}

func TestExecutionRestrictionIntersects(t *testing.T) {
	ctx := ContextWithToolRestriction(context.Background(), []string{"a", "b"})
	ctx = ContextWithToolRestriction(ctx, []string{"b", "c"})
	if ToolAllowedByContext(ctx, "a") || !ToolAllowedByContext(ctx, "b") || ToolAllowedByContext(ctx, "c") {
		t.Fatal("nested restrictions must intersect")
	}
}
