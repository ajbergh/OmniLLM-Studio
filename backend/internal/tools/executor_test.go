package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeTool struct {
	executed *bool
}

func (t fakeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "fake_tool",
		Description: "Fake tool",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Category:    "test",
		Enabled:     true,
	}
}

func (t fakeTool) Validate(json.RawMessage) error {
	return nil
}

func (t fakeTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	if t.executed != nil {
		*t.executed = true
	}
	return &ToolResult{Content: "ok"}, nil
}

func TestExecutorAskRequiresApprovalWithoutHandler(t *testing.T) {
	executed := false
	registry := NewRegistry()
	registry.MustRegister(fakeTool{executed: &executed})
	executor := NewExecutor(registry, func(string) string { return "ask" }, 0)

	result := executor.Execute(context.Background(), ToolCall{Name: "fake_tool", Arguments: json.RawMessage(`{}`)})

	if !result.IsError {
		t.Fatal("expected ask policy without handler to return an error")
	}
	if executed {
		t.Fatal("tool should not execute before approval")
	}
	if result.Metadata[ApprovalStatusMetadataKey] != "required" {
		t.Fatalf("approval status = %v, want required", result.Metadata[ApprovalStatusMetadataKey])
	}
}

func TestExecutorAskRunsAfterApproval(t *testing.T) {
	executed := false
	registry := NewRegistry()
	registry.MustRegister(fakeTool{executed: &executed})
	executor := NewExecutor(registry, func(string) string { return "ask" }, 0)
	ctx := ContextWithApprovalHandler(context.Background(), func(_ context.Context, req ApprovalRequest) (bool, error) {
		if req.ToolName != "fake_tool" {
			t.Fatalf("approval tool name = %q, want fake_tool", req.ToolName)
		}
		return true, nil
	})

	result := executor.Execute(ctx, ToolCall{Name: "fake_tool", Arguments: json.RawMessage(`{}`)})

	if result.IsError {
		t.Fatalf("expected approved tool to run, got error: %s", result.Content)
	}
	if !executed {
		t.Fatal("tool should execute after approval")
	}
}

func TestExecutorAskRejectsAfterApprovalDenial(t *testing.T) {
	executed := false
	registry := NewRegistry()
	registry.MustRegister(fakeTool{executed: &executed})
	executor := NewExecutor(registry, func(string) string { return "ask" }, 0)
	ctx := ContextWithApprovalHandler(context.Background(), func(context.Context, ApprovalRequest) (bool, error) {
		return false, nil
	})

	result := executor.Execute(ctx, ToolCall{Name: "fake_tool", Arguments: json.RawMessage(`{}`)})

	if !result.IsError {
		t.Fatal("expected rejected approval to return an error")
	}
	if executed {
		t.Fatal("tool should not execute after rejection")
	}
	if result.Metadata[ApprovalStatusMetadataKey] != "rejected" {
		t.Fatalf("approval status = %v, want rejected", result.Metadata[ApprovalStatusMetadataKey])
	}
}

func TestExecutorAskHandlerError(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(fakeTool{})
	executor := NewExecutor(registry, func(string) string { return "ask" }, 0)
	ctx := ContextWithApprovalHandler(context.Background(), func(context.Context, ApprovalRequest) (bool, error) {
		return false, errors.New("approval unavailable")
	})

	result := executor.Execute(ctx, ToolCall{Name: "fake_tool", Arguments: json.RawMessage(`{}`)})

	if !result.IsError {
		t.Fatal("expected approval handler error to return an error")
	}
	if result.Metadata[ApprovalStatusMetadataKey] != "error" {
		t.Fatalf("approval status = %v, want error", result.Metadata[ApprovalStatusMetadataKey])
	}
}
