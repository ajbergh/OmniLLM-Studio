package tools

import (
	"context"
	"fmt"
)

// ExecutePlan runs a previously constructed execution plan and returns results
// in the same order as the calls represented by the plan. Parallel steps use
// ExecuteBatch; sequential steps execute one call at a time.
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
		if step.Parallel && len(step.Calls) > 1 {
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

func (e *Executor) cancelledBeforeStart(ctx context.Context, call ToolCall) *ToolResult {
	metadata := map[string]interface{}{
		"cancelled":  true,
		"retryable": false,
		"phase":      "before_start",
	}
	return e.failure(ctx, call, fmt.Sprintf("tool %q was cancelled before execution", call.Name), ToolEventFailed, metadata)
}
