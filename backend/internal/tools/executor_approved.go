package tools

import (
	"context"
	"fmt"
	"time"
)

// ExecuteApproved executes a call whose exact arguments were explicitly
// approved by the user. It bypasses only the policy prompt; enabled-state,
// argument validation, timeout, result limits, and lifecycle events still apply.
func (e *Executor) ExecuteApproved(ctx context.Context, call ToolCall) *ToolResult {
	scope := InvocationScopeFromContext(ctx)
	tool, ok := e.registry.Get(call.Name)
	if !ok {
		return e.failure(ctx, call, fmt.Sprintf("unknown tool: %s", call.Name), ToolEventFailed, nil)
	}
	definition := tool.Definition().Normalized()
	if !definition.Enabled {
		return e.failure(ctx, call, fmt.Sprintf("tool %q is disabled", call.Name), ToolEventFailed, nil)
	}
	if err := tool.Validate(call.Arguments); err != nil {
		return e.failure(ctx, call, fmt.Sprintf("invalid approved arguments for %q: %v", call.Name, err), ToolEventFailed, nil)
	}
	timeout := e.timeout
	if definition.DefaultTimeoutMS > 0 {
		toolTimeout := time.Duration(definition.DefaultTimeoutMS) * time.Millisecond
		if timeout <= 0 || toolTimeout < timeout {
			timeout = toolTimeout
		}
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	emitEvent(ctx, ToolEvent{
		Type: ToolEventStarted, ToolCallID: call.ID, ToolName: call.Name, Scope: scope,
		Data: map[string]interface{}{"approved": true, "arguments": string(call.Arguments)},
	})
	started := time.Now()
	result, err := tool.Execute(execCtx, call.Arguments)
	durationMS := time.Since(started).Milliseconds()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return e.failure(ctx, call, fmt.Sprintf("tool %q timed out after %s", call.Name, timeout), ToolEventTimedOut, map[string]interface{}{"duration_ms": durationMS})
		}
		return e.failure(ctx, call, fmt.Sprintf("tool %q failed: %v", call.Name, err), ToolEventFailed, map[string]interface{}{"duration_ms": durationMS})
	}
	if result == nil {
		return e.failure(ctx, call, fmt.Sprintf("tool %q returned no result", call.Name), ToolEventFailed, map[string]interface{}{"duration_ms": durationMS})
	}
	result.ToolCallID = call.ID
	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}
	result.Metadata["duration_ms"] = durationMS
	result.Metadata["tool_version"] = definition.Version
	result.Metadata["approval_status"] = "approved"
	if definition.MaxResultBytes > 0 && len(result.Content) > definition.MaxResultBytes {
		originalBytes := len(result.Content)
		result.Content = result.Content[:definition.MaxResultBytes] + "\n\n[tool result truncated]"
		result.Metadata["truncated"] = true
		result.Metadata["original_bytes"] = originalBytes
	}
	emitEvent(ctx, ToolEvent{
		Type: ToolEventCompleted, ToolCallID: call.ID, ToolName: call.Name, Scope: scope,
		Data: map[string]interface{}{"approved": true, "duration_ms": durationMS, "result_bytes": len(result.Content), "is_error": result.IsError},
	})
	return result
}
