package tools

import "context"

type invocationScopeContextKey struct{}
type eventSinkContextKey struct{}

// ToolEventType identifies lifecycle events emitted by the executor.
type ToolEventType string

const (
	ToolEventQueued            ToolEventType = "tool_queued"
	ToolEventApprovalRequired  ToolEventType = "tool_approval_required"
	ToolEventApprovalResolved  ToolEventType = "tool_approval_resolved"
	ToolEventStarted           ToolEventType = "tool_started"
	ToolEventProgress          ToolEventType = "tool_progress"
	ToolEventCompleted         ToolEventType = "tool_completed"
	ToolEventFailed            ToolEventType = "tool_failed"
	ToolEventTimedOut          ToolEventType = "tool_timed_out"
)

// ToolEvent is the provider-independent event contract used by Chat and Agent modes.
type ToolEvent struct {
	Type       ToolEventType          `json:"type"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	Scope      InvocationScope        `json:"scope,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// EventSink receives executor lifecycle events. Implementations must return
// quickly; long-running persistence should be queued by the sink.
type EventSink func(event ToolEvent)

// ContextWithInvocationScope associates user/workspace/conversation/run metadata
// with a tool invocation without changing every tool's interface.
func ContextWithInvocationScope(ctx context.Context, scope InvocationScope) context.Context {
	return context.WithValue(ctx, invocationScopeContextKey{}, scope)
}

// InvocationScopeFromContext returns the scope attached to ctx, if any.
func InvocationScopeFromContext(ctx context.Context) InvocationScope {
	scope, _ := ctx.Value(invocationScopeContextKey{}).(InvocationScope)
	return scope
}

// ContextWithEventSink attaches a lifecycle event sink to an invocation.
func ContextWithEventSink(ctx context.Context, sink EventSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, eventSinkContextKey{}, sink)
}

func emitEvent(ctx context.Context, event ToolEvent) {
	if sink, ok := ctx.Value(eventSinkContextKey{}).(EventSink); ok && sink != nil {
		sink(event)
	}
}
