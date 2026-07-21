package tools

import "testing"

func TestParallelStepSafeRejectsSideEffectsUnknownAndNonParallelCalls(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(planningTool{name: "read_parallel", readOnly: true, supportsParallel: true})
	registry.MustRegister(planningTool{name: "read_serial", readOnly: true, supportsParallel: false})
	registry.MustRegister(planningTool{name: "write", sideEffecting: true})
	executor := NewExecutor(registry, nil, 0)

	if !executor.parallelStepSafe([]ToolCall{{Name: "read_parallel"}, {Name: "read_parallel"}}) {
		t.Fatal("expected two explicitly parallel-safe reads to pass revalidation")
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
