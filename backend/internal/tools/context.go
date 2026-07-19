package tools

import (
	"context"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/go-chi/chi/v5"
)

type invocationScopeContextKey struct{}
type eventSinkContextKey struct{}

type ToolEventType string

const (
	ToolEventQueued           ToolEventType = "tool_queued"
	ToolEventApprovalRequired ToolEventType = "tool_approval_required"
	ToolEventApprovalResolved ToolEventType = "tool_approval_resolved"
	ToolEventStarted          ToolEventType = "tool_started"
	ToolEventProgress         ToolEventType = "tool_progress"
	ToolEventCompleted        ToolEventType = "tool_completed"
	ToolEventFailed           ToolEventType = "tool_failed"
	ToolEventTimedOut         ToolEventType = "tool_timed_out"
)

type ToolEvent struct {
	Type       ToolEventType          `json:"type"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	Scope      InvocationScope        `json:"scope,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

type EventSink func(event ToolEvent)

func ContextWithInvocationScope(ctx context.Context, scope InvocationScope) context.Context {
	if scope.UserID == "" {
		scope.UserID = auth.UserIDFromContext(ctx)
	}
	if scope.ConversationID == "" {
		scope.ConversationID = chi.URLParamFromCtx(ctx, "conversationId")
	}
	return context.WithValue(ctx, invocationScopeContextKey{}, scope)
}

// InvocationScopeFromContext returns explicit invocation metadata and inherits
// the authenticated user and Chi conversation route when available.
func InvocationScopeFromContext(ctx context.Context) InvocationScope {
	scope, _ := ctx.Value(invocationScopeContextKey{}).(InvocationScope)
	if scope.UserID == "" {
		scope.UserID = auth.UserIDFromContext(ctx)
	}
	if scope.ConversationID == "" {
		scope.ConversationID = chi.URLParamFromCtx(ctx, "conversationId")
	}
	return scope
}

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
