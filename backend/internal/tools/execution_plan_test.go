package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type planningTool struct {
	name            string
	readOnly        bool
	sideEffecting   bool
	supportsParallel bool
}

func (t planningTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             t.name,
		Description:      "planning test tool",
		Parameters:       json.RawMessage(`{"type":"object"}`),
		Category:         "test",
		Enabled:          true,
		ReadOnly:         t.readOnly,
		SideEffecting:    t.sideEffecting,
		SupportsParallel: t.supportsParallel,
	}
}
func (t planningTool) Validate(json.RawMessage) error { return nil }
func (t planningTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	return &ToolResult{Content: t.name}, nil
}

func TestBuildExecutionPlanPreservesSideEffectBoundaries(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "read_a", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "read_b", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "write", sideEffecting: true})
	registry.MustRegister(planningTool{name: "read_c", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "read_d", readOnly: true, supportsParallel: true})

	calls := []ToolCall{{ID: "1", Name: "read_a"}, {ID: "2", Name: "read_b"}, {ID: "3", Name: "write"}, {ID: "4", Name: "read_c"}, {ID: "5", Name: "read_d"}}
	plan := BuildExecutionPlan(registry, calls)
	if len(plan) != 3 {
		t.Fatalf("plan length = %d, want 3: %#v", len(plan), plan)
	}
	if !plan[0].Parallel || len(plan[0].Calls) != 2 || plan[0].Calls[0].Name != "read_a" || plan[0].Calls[1].Name != "read_b" {
		t.Fatalf("first step = %#v", plan[0])
	}
	if plan[1].Parallel || len(plan[1].Calls) != 1 || plan[1].Calls[0].Name != "write" {
		t.Fatalf("write step = %#v", plan[1])
	}
	if !plan[2].Parallel || len(plan[2].Calls) != 2 || plan[2].Calls[0].Name != "read_c" || plan[2].Calls[1].Name != "read_d" {
		t.Fatalf("last step = %#v", plan[2])
	}
}

func TestBuildExecutionPlanKeepsUnknownAndNonParallelSequential(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "read_parallel", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "read_serial", readOnly: true, supportsParallel: false})

	calls := []ToolCall{{ID: "1", Name: "read_parallel"}, {ID: "2", Name: "unknown"}, {ID: "3", Name: "read_serial"}}
	plan := BuildExecutionPlan(registry, calls)
	if len(plan) != 3 {
		t.Fatalf("plan length = %d, want 3: %#v", len(plan), plan)
	}
	for i, step := range plan {
		if step.Parallel || len(step.Calls) != 1 {
			t.Fatalf("step %d should be sequential singleton: %#v", i, step)
		}
	}
}

func TestBuildExecutionPlanSingleParallelSafeCallStaysSequentialStep(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "read_parallel", readOnly: true, supportsParallel: true})
	plan := BuildExecutionPlan(registry, []ToolCall{{ID: "1", Name: "read_parallel"}})
	if len(plan) != 1 || plan[0].Parallel || len(plan[0].Calls) != 1 {
		t.Fatalf("single-call plan = %#v", plan)
	}
}
