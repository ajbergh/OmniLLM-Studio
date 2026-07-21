package tools

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

type blockingParallelTool struct {
	name    string
	started chan<- string
	release <-chan struct{}
}

func (t blockingParallelTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             t.name,
		Description:      "parallel execution test tool",
		Parameters:       json.RawMessage(`{"type":"object"}`),
		Category:         "test",
		Enabled:          true,
		ReadOnly:         true,
		SupportsParallel: true,
	}
}

func (t blockingParallelTool) Validate(json.RawMessage) error { return nil }
func (t blockingParallelTool) Execute(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	select {
	case t.started <- t.name:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-t.release:
		return &ToolResult{Content: t.name}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type countedSideEffectTool struct {
	attempts *atomic.Int32
}

func (t countedSideEffectTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:          "counted_write",
		Description:   "side-effect cancellation test tool",
		Parameters:    json.RawMessage(`{"type":"object"}`),
		Category:      "test",
		Enabled:       true,
		ReadOnly:      false,
		SideEffecting: true,
	}
}

func (t countedSideEffectTool) Validate(json.RawMessage) error { return nil }
func (t countedSideEffectTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	t.attempts.Add(1)
	return &ToolResult{Content: "written"}, nil
}

func TestExecutePlanRunsParallelStepConcurrentlyAndPreservesResultOrder(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	registry := NewRegistry()
	registry.MustRegister(blockingParallelTool{name: "read_a", started: started, release: release})
	registry.MustRegister(blockingParallelTool{name: "read_b", started: started, release: release})
	executor := NewExecutor(registry, nil, time.Second)
	calls := []ToolCall{
		{ID: "call-a", Name: "read_a", Arguments: json.RawMessage(`{}`)},
		{ID: "call-b", Name: "read_b", Arguments: json.RawMessage(`{}`)},
	}
	plan := BuildExecutionPlan(registry, calls)

	resultsCh := make(chan []*ToolResult, 1)
	go func() {
		resultsCh <- executor.ExecutePlan(context.Background(), plan)
	}()

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatal("parallel tools did not both start before release")
		}
	}
	if !seen["read_a"] || !seen["read_b"] {
		t.Fatalf("started tools = %#v", seen)
	}
	close(release)

	select {
	case results := <-resultsCh:
		if len(results) != 2 {
			t.Fatalf("result count = %d, want 2", len(results))
		}
		if results[0].ToolCallID != "call-a" || results[0].Content != "read_a" {
			t.Fatalf("first result = %#v", results[0])
		}
		if results[1].ToolCallID != "call-b" || results[1].Content != "read_b" {
			t.Fatalf("second result = %#v", results[1])
		}
	case <-time.After(time.Second):
		t.Fatal("parallel execution did not complete")
	}
}

func TestExecutePlanDoesNotStartSideEffectAfterCancellation(t *testing.T) {
	var attempts atomic.Int32
	registry := NewRegistry()
	registry.MustRegister(countedSideEffectTool{attempts: &attempts})
	executor := NewExecutor(registry, nil, time.Second)
	plan := BuildExecutionPlan(registry, []ToolCall{{ID: "write-call", Name: "counted_write", Arguments: json.RawMessage(`{}`)}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []ToolEvent
	ctx = ContextWithEventSink(ctx, func(event ToolEvent) {
		events = append(events, event)
	})
	results := executor.ExecutePlan(ctx, plan)

	if attempts.Load() != 0 {
		t.Fatalf("side-effect execution count = %d, want 0", attempts.Load())
	}
	if len(results) != 1 || !results[0].IsError {
		t.Fatalf("cancelled results = %#v", results)
	}
	if len(events) != 1 || events[0].Type != ToolEventCancelled {
		t.Fatalf("cancellation events = %#v", events)
	}
	if phase, _ := results[0].Metadata["phase"].(string); phase != "before_start" {
		t.Fatalf("cancelled metadata = %#v", results[0].Metadata)
	}
}
