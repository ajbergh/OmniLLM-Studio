package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type transientError struct{ message string }

func (e transientError) Error() string   { return e.message }
func (e transientError) Retryable() bool { return true }

type retryTool struct {
	attempts *int
}

func (t retryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "retry_tool",
		Description: "Retryable read-only tool",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Category:    "test",
		Enabled:     true,
		ReadOnly:    true,
	}
}

func (t retryTool) Validate(json.RawMessage) error { return nil }
func (t retryTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	(*t.attempts)++
	if *t.attempts == 1 {
		return nil, transientError{message: "temporary"}
	}
	return &ToolResult{Content: "ok"}, nil
}

type sideEffectTool struct {
	attempts *int
}

func (t sideEffectTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:          "side_effect_tool",
		Description:   "Side-effecting tool",
		Parameters:    json.RawMessage(`{"type":"object"}`),
		Category:      "test",
		Enabled:       true,
		SideEffecting: true,
		ReadOnly:      false,
	}
}

func (t sideEffectTool) Validate(json.RawMessage) error { return nil }
func (t sideEffectTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	(*t.attempts)++
	return &ToolResult{Content: "created"}, nil
}

func TestIsRetryableExecutionError(t *testing.T) {
	if !IsRetryableExecutionError(transientError{message: "temporary"}) {
		t.Fatal("expected RetryableError to be retryable")
	}
	if IsRetryableExecutionError(errors.New("permanent")) {
		t.Fatal("plain error should not be retryable")
	}
}

func TestExecutorRetriesReadOnlyTransientFailure(t *testing.T) {
	attempts := 0
	registry := NewRegistry()
	registry.MustRegister(retryTool{attempts: &attempts})
	executor := NewExecutor(registry, nil, 0)

	result := executor.Execute(context.Background(), ToolCall{ID: "retry-call", Name: "retry_tool", Arguments: json.RawMessage(`{}`)})
	if result.IsError {
		t.Fatalf("expected retry to succeed, got %q", result.Content)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if got := result.Metadata["attempt_count"]; got != 2 {
		t.Fatalf("attempt_count metadata = %#v, want 2", got)
	}
}

func TestExecutorDoesNotReplaySideEffectingCall(t *testing.T) {
	attempts := 0
	registry := NewRegistry()
	registry.MustRegister(sideEffectTool{attempts: &attempts})
	executor := NewExecutor(registry, nil, 0)
	call := ToolCall{ID: "stable-side-effect-call", Name: "side_effect_tool", Arguments: json.RawMessage(`{"value":1}`)}

	first := executor.Execute(context.Background(), call)
	second := executor.Execute(context.Background(), call)
	if first.IsError || second.IsError {
		t.Fatalf("expected successful results: first=%v second=%v", first.IsError, second.IsError)
	}
	if attempts != 1 {
		t.Fatalf("side-effecting execution count = %d, want 1", attempts)
	}
	if replay, _ := second.Metadata["idempotent_replay"].(bool); !replay {
		t.Fatalf("second result metadata = %#v, want idempotent_replay=true", second.Metadata)
	}
}
