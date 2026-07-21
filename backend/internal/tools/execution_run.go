package tools

import (
	"context"
	"fmt"
)

// ExecutePlan runs a previously constructed execution plan and returns results
// in the same order as the calls represented by the plan. Parallel steps use
// ExecuteBatch only after the executor independently revalidates every call;
// sequential steps execute one call at a time.
//
// If the parent context is already cancelled before a step begins, the executor
// does not invoke any remaining tools. Instead it emits terminal cancellation
// results for each remaining call so callers can preserve one response per tool
// call ID without risking side effects after cancellation.
func (e *Executor) ExecutePlan(ctx context.Context, steps []ExecutionStep) []*ToolResult {
	totalCalls := 0
	for _, step := range steps {
		totalCalls += len(step.Calls)
	}
	results := make([]*ToolResult, 0, totalCalls)

	for _, step := range steps {
		if len(step.Calls) == 0 {
			continue
		}
		if ctx.Err() != nil {
			for _, call := range step.Calls {
				results = append(results, e.cancelledBeforeStart(ctx, call))
			}
			continue
		}
		if step.Parallel && e.parallelStepSafe(step.Calls) {
			results = append(results, e.ExecuteBatch(ctx, step.Calls)...)
			continue
		}
		for _, call := range step.Calls {
			if ctx.Err() != nil {
				results = append(results, e.cancelledBeforeStart(ctx, call))
				continue
			}
			results = append(results, e.Execute(ctx, call))
		}
	}
	return results
}

func (e *Executor) parallelStepSafe(calls []ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	for _, call := range calls {
		tool, ok := e.registry.Get(call.Name)
		if !ok {
			return false
		}
		definition := tool.Definition().Normalized()
		if !definition.Enabled || !definition.ReadOnly || definition.SideEffecting || !definition.SupportsParallel {
			return false
		}
	}
	return true
}

func (e *Executor) cancelledBeforeStart(ctx context.Context, call ToolCall) *ToolResult {
	metadata := map[string]interface{}{
		"cancelled":  true,
		"retryable": false,
		"phase":      "before_start",
	}
	return e.failure(ctx, call, fmt.Sprintf("tool %q was cancelled before execution", call.Name), ToolEventFailed, metadata)
}
