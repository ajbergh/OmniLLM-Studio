package tools

// ExecutionStep describes one ordered unit of tool execution. Parallel is true
// only when every call in the step is explicitly read-only, non-side-effecting,
// and marked SupportsParallel by its registered definition.
type ExecutionStep struct {
	Parallel bool       `json:"parallel"`
	Calls    []ToolCall `json:"calls"`
}

// BuildExecutionPlan preserves the model's tool-call order while coalescing
// contiguous definition-level parallel-safe calls into a single batch. It is
// useful for tests and callers without an Executor. Runtime orchestration should
// prefer Executor.BuildExecutionPlan so persisted allow/ask/deny policy is also
// part of the safety decision.
func BuildExecutionPlan(registry *Registry, calls []ToolCall) []ExecutionStep {
	return buildExecutionPlan(calls, func(call ToolCall) bool {
		tool, ok := registry.Get(call.Name)
		if !ok {
			return false
		}
		return definitionParallelSafe(tool.Definition().Normalized())
	})
}

// BuildExecutionPlan creates a policy-aware runtime plan. Approval-gated and
// denied tools always remain sequential, even when their definitions advertise
// read-only parallel support.
func (e *Executor) BuildExecutionPlan(calls []ToolCall) []ExecutionStep {
	return buildExecutionPlan(calls, func(call ToolCall) bool {
		if e.Policy(call.Name) != "allow" {
			return false
		}
		definition, ok := e.Definition(call.Name)
		return ok && definitionParallelSafe(definition)
	})
}

func buildExecutionPlan(calls []ToolCall, parallelSafe func(ToolCall) bool) []ExecutionStep {
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
		if parallelSafe(call) {
			parallelBatch = append(parallelBatch, call)
			continue
		}
		flushParallel()
		steps = append(steps, ExecutionStep{Calls: []ToolCall{call}})
	}
	flushParallel()
	return steps
}

func definitionParallelSafe(definition ToolDefinition) bool {
	return definition.Enabled && definition.ReadOnly && !definition.SideEffecting && definition.SupportsParallel
}
