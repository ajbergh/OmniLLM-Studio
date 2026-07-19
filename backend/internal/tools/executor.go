// Package tools provides the dynamic tool registry and execution framework
// for OmniLLM-Studio, supporting both local built-in tools and remote MCP tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultTimeout is the default per-tool execution timeout.
const DefaultTimeout = 30 * time.Second

// PermissionResolver looks up the policy for a given tool.
// Return "allow", "deny", or "ask". A zero-value ("") is treated as "allow".
type PermissionResolver func(toolName string) string

type approvalContextKey struct{}

const ApprovalStatusMetadataKey = "approval_status"
const ApprovalIDMetadataKey = "approval_id"

// ContextWithApprovalHandler attaches a per-request approval handler used for
// tools with "ask" policy. Agent mode uses this to integrate approval with its
// persisted run state; ordinary chat creates a non-blocking pending approval.
func ContextWithApprovalHandler(ctx context.Context, handler ApprovalHandler) context.Context {
	if handler == nil {
		return ctx
	}
	return context.WithValue(ctx, approvalContextKey{}, handler)
}

// Executor orchestrates tool lookup, policy checks, approvals, validation,
// lifecycle events, timeouts, and result limits.
type Executor struct {
	registry    *Registry
	permissions PermissionResolver
	timeout     time.Duration
	approvals   *ApprovalBroker
}

// NewExecutor creates an Executor with the given registry and permission
// resolver. If timeout is 0 the DefaultTimeout is used.
func NewExecutor(registry *Registry, permissions PermissionResolver, timeout time.Duration) *Executor {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &Executor{
		registry:    registry,
		permissions: permissions,
		timeout:     timeout,
		approvals:   NewApprovalBroker(15 * time.Minute),
	}
}

// ApprovalBroker exposes the broker used by API handlers to list and resolve
// pending ordinary-chat approvals.
func (e *Executor) ApprovalBroker() *ApprovalBroker { return e.approvals }

// Execute runs a single tool call. It validates permissions and arguments, then
// executes the tool within the configured timeout.
func (e *Executor) Execute(ctx context.Context, call ToolCall) *ToolResult {
	scope := InvocationScopeFromContext(ctx)
	emitEvent(ctx, ToolEvent{Type: ToolEventQueued, ToolCallID: call.ID, ToolName: call.Name, Scope: scope})

	tool, ok := e.registry.Get(call.Name)
	if !ok {
		return e.failure(ctx, call, fmt.Sprintf("unknown tool: %s", call.Name), ToolEventFailed, nil)
	}
	def := tool.Definition().Normalized()
	if !def.Enabled {
		return e.failure(ctx, call, fmt.Sprintf("tool %q is disabled", call.Name), ToolEventFailed, nil)
	}

	if e.permissions != nil {
		switch e.permissions(call.Name) {
		case "deny":
			return e.failure(ctx, call, fmt.Sprintf("tool %q is denied by policy", call.Name), ToolEventFailed, nil)
		case "ask":
			req := ApprovalRequest{
				ToolCallID:  call.ID,
				ToolName:    call.Name,
				Description: def.Description,
				Arguments:   call.Arguments,
				Scope:       scope,
				Risk:        def.Risk,
				ReadOnly:    def.ReadOnly,
			}
			if handler, _ := ctx.Value(approvalContextKey{}).(ApprovalHandler); handler != nil {
				approved, err := handler(ctx, req)
				if err != nil {
					return e.failure(ctx, call, fmt.Sprintf("tool %q approval failed: %v", call.Name, err), ToolEventFailed, map[string]interface{}{ApprovalStatusMetadataKey: "error"})
				}
				if !approved {
					return e.failure(ctx, call, fmt.Sprintf("tool %q was rejected by the user", call.Name), ToolEventFailed, map[string]interface{}{ApprovalStatusMetadataKey: "rejected"})
				}
			} else {
				pending := e.approvals.CreatePending(req)
				metadata := map[string]interface{}{
					ApprovalStatusMetadataKey: "required",
					ApprovalIDMetadataKey:     pending.ID,
					"tool_name":               call.Name,
					"arguments":               string(call.Arguments),
					"risk":                    def.Risk,
					"read_only":               def.ReadOnly,
				}
				emitEvent(ctx, ToolEvent{
					Type:       ToolEventApprovalRequired,
					ToolCallID: call.ID,
					ToolName:   call.Name,
					Scope:      scope,
					Data:       metadata,
				})
				return &ToolResult{
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool %q requires user approval (approval_id: %s)", call.Name, pending.ID),
					IsError:    true,
					Metadata:   metadata,
				}
			}
		}
	}

	if err := tool.Validate(call.Arguments); err != nil {
		return e.failure(ctx, call, fmt.Sprintf("invalid arguments for %q: %v", call.Name, err), ToolEventFailed, nil)
	}

	timeout := e.timeout
	if def.DefaultTimeoutMS > 0 {
		toolTimeout := time.Duration(def.DefaultTimeoutMS) * time.Millisecond
		if timeout <= 0 || toolTimeout < timeout {
			timeout = toolTimeout
		}
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	emitEvent(ctx, ToolEvent{
		Type:       ToolEventStarted,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Scope:      scope,
		Data: map[string]interface{}{
			"arguments": string(call.Arguments),
			"risk":      def.Risk,
			"read_only": def.ReadOnly,
		},
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
	result.Metadata["tool_version"] = def.Version
	if def.MaxResultBytes > 0 && len(result.Content) > def.MaxResultBytes {
		originalBytes := len(result.Content)
		result.Content = result.Content[:def.MaxResultBytes] + "\n\n[tool result truncated]"
		result.Metadata["truncated"] = true
		result.Metadata["original_bytes"] = originalBytes
	}

	emitEvent(ctx, ToolEvent{
		Type:       ToolEventCompleted,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Scope:      scope,
		Data: map[string]interface{}{
			"duration_ms": durationMS,
			"is_error":    result.IsError,
			"result_bytes": len(result.Content),
		},
	})
	return result
}

func (e *Executor) failure(ctx context.Context, call ToolCall, content string, eventType ToolEventType, metadata map[string]interface{}) *ToolResult {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	emitEvent(ctx, ToolEvent{
		Type:       eventType,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Scope:      InvocationScopeFromContext(ctx),
		Data:       metadata,
	})
	return &ToolResult{ToolCallID: call.ID, Content: content, IsError: true, Metadata: metadata}
}

// ExecuteBatch runs multiple tool calls concurrently and returns results in the
// same order as the input calls. The orchestrator should call this only for
// definitions that advertise SupportsParallel and have no dependency edges.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCall) []*ToolResult {
	results := make([]*ToolResult, len(calls))
	var wg sync.WaitGroup
	wg.Add(len(calls))
	for i, call := range calls {
		go func(idx int, c ToolCall) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, c)
		}(i, call)
	}
	wg.Wait()
	return results
}

// ExecuteJSON is a convenience wrapper that accepts a raw JSON tool call,
// unmarshals it, executes, and returns the result.
func (e *Executor) ExecuteJSON(ctx context.Context, raw json.RawMessage) *ToolResult {
	var call ToolCall
	if err := json.Unmarshal(raw, &call); err != nil {
		log.Printf("[tools/executor] failed to unmarshal tool call: %v", err)
		return &ToolResult{Content: fmt.Sprintf("malformed tool call: %v", err), IsError: true}
	}
	return e.Execute(ctx, call)
}
