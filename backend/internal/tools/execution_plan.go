package tools

// ExecutionStep describes one ordered unit of tool execution. Parallel is true
// only when every call in the step is explicitly read-only, non-side-effecting,
// and marked SupportsParallel by its registered definition.
type ExecutionStep struct {
	Parallel bool       `json:"parallel"`
	Calls    []ToolCall `json:"calls"`
}

// BuildExecutionPlan preserves the model's tool-call order while coalescing
// contiguous parallel-safe calls into a single batch. Unknown, disabled,
// side-effecting, or non-parallel tools always remain single sequential steps.
// Permission policy is enforced again by Executor.Execute; callers should avoid
// marking approval-gated tools SupportsParallel because an inline approval wait
// is inherently sequential. This deliberately does not move read-only calls
// across a side-effect boundary.
func BuildExecutionPlan(registry *Registry, calls []ToolCall) []ExecutionStep {
	steps := make([]ExecutionStep, 0, len(calls))
	parallelBatch := make([]ToolCall, 0)
	flushParallel := func() {
		if len(parallelBatch) == 0 {
			return
		}
		batch := append([]ToolCall(nil), parallelBatch...)
		steps = append(steps, ExecutionStep{Parallel: len(batch) > 1, Calls: batch})
		parallelBatch = parallelBatch[:0]
	}

	for _, call := range calls {
		tool, ok := registry.Get(call.Name)
		if !ok {
			flushParallel()
			steps = append(steps, ExecutionStep{Calls: []ToolCall{call}})
			continue
		}
		definition := tool.Definition().Normalized()
		parallelSafe := definition.Enabled && definition.ReadOnly && !definition.SideEffecting && definition.SupportsParallel
		if parallelSafe {
			parallelBatch = append(parallelBatch, call)
			continue
		}
		flushParallel()
		steps = append(steps, ExecutionStep{Calls: []ToolCall{call}})
	}
	flushParallel()
	return steps
}
