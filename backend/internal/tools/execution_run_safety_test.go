package tools

import "testing"

func TestParallelStepSafeRejectsSideEffectsUnknownNonParallelAndApprovalCalls(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "read_parallel", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "read_serial", readOnly: true, supportsParallel: false})
	registry.MustRegister(planningTool{name: "write", sideEffecting: true})
	policy := func(name string) string {
		if name == "read_parallel" {
			return "allow"
		}
		return "ask"
	}
	executor := NewExecutor(registry, policy, 0)

	if !executor.parallelStepSafe([]ToolCall{{Name: "read_parallel"}, {Name: "read_parallel"}}) {
		t.Fatal("expected two explicitly parallel-safe allowed reads to pass revalidation")
	}
	cases := [][]ToolCall{
		{{Name: "read_parallel"}},
		{{Name: "read_parallel"}, {Name: "read_serial"}},
		{{Name: "read_parallel"}, {Name: "write"}},
		{{Name: "read_parallel"}, {Name: "unknown"}},
	}
	for _, calls := range cases {
		if executor.parallelStepSafe(calls) {
			t.Fatalf("unsafe calls passed parallel revalidation: %#v", calls)
		}
	}
}

func TestExecutorBuildExecutionPlanKeepsAskAndDenyPoliciesSequential(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "allowed_read", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "approval_read", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "denied_read", readOnly: true, supportsParallel: true})
	executor := NewExecutor(registry, func(name string) string {
		switch name {
		case "approval_read":
			return "ask"
		case "denied_read":
			return "deny"
		default:
			return "allow"
		}
	}, 0)

	plan := executor.BuildExecutionPlan([]ToolCall{
		{ID: "1", Name: "allowed_read"},
		{ID: "2", Name: "approval_read"},
		{ID: "3", Name: "denied_read"},
		{ID: "4", Name: "allowed_read"},
	})
	if len(plan) != 4 {
		t.Fatalf("plan length = %d, want 4: %#v", len(plan), plan)
	}
	for i, step := range plan {
		if step.Parallel || len(step.Calls) != 1 {
			t.Fatalf("step %d should remain sequential: %#v", i, step)
		}
	}
}
