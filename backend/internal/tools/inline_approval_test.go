package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type inlineApprovalTestTool struct{ executed chan string }

func (t *inlineApprovalTestTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:          "write_test",
		Description:   "writes a test value",
		Parameters:    json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`),
		Enabled:       true,
		SideEffecting: true,
	}
}
func (t *inlineApprovalTestTool) Validate(args json.RawMessage) error { return nil }
func (t *inlineApprovalTestTool) Execute(_ context.Context, args json.RawMessage) (*ToolResult, error) {
	var payload struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(args, &payload)
	t.executed <- payload.Value
	return &ToolResult{Content: payload.Value}, nil
}

func TestInlineApprovalResumesSameExecutorCallWithEditedArguments(t *testing.T) {
	registry := NewRegistry()
	tool := &inlineApprovalTestTool{executed: make(chan string, 1)}
	registry.MustRegister(tool)
	executor := NewExecutor(registry, func(string) string { return "ask" }, time.Second)
	ctx, cancel := context.WithTimeout(ContextWithInlineApproval(context.Background()), 2*time.Second)
	defer cancel()

	resultCh := make(chan *ToolResult, 1)
	go func() {
		resultCh <- executor.Execute(ctx, ToolCall{ID: "call-1", Name: "write_test", Arguments: json.RawMessage(`{"value":"before"}`)})
	}()

	var approval PendingApproval
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := executor.ApprovalBroker().List(InvocationScope{})
		if len(pending) > 0 {
			approval = pending[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if approval.ID == "" {
		t.Fatal("approval was not created")
	}
	if approval.Request.ContinuationMode != "inline" {
		t.Fatalf("continuation mode = %q", approval.Request.ContinuationMode)
	}
	if err := executor.ApprovalBroker().Resolve(approval.ID, true, json.RawMessage(`{"value":"after"}`)); err != nil {
		t.Fatalf("resolve approval: %v", err)
	}

	select {
	case result := <-resultCh:
		if result == nil || result.IsError || result.Content != "after" {
			t.Fatalf("unexpected result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("executor did not resume")
	}
	select {
	case value := <-tool.executed:
		if value != "after" {
			t.Fatalf("tool executed with %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("tool did not execute")
	}
}
