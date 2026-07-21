package tools

import (
	"context"
	"errors"

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
	ToolEventCancelled        ToolEventType = "tool_cancelled"
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
	inherited, _ := ctx.Value(invocationScopeContextKey{}).(InvocationScope)
	if scope.UserID == "" {
		scope.UserID = inherited.UserID
	}
	if scope.WorkspaceID == "" {
		scope.WorkspaceID = inherited.WorkspaceID
	}
	if scope.ConversationID == "" {
		scope.ConversationID = inherited.ConversationID
	}
	if scope.MessageID == "" {
		scope.MessageID = inherited.MessageID
	}
	if scope.RunID == "" {
		scope.RunID = inherited.RunID
	}
	if scope.UserID == "" {
		scope.UserID = auth.ScopeUserIDFromContext(ctx)
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
		scope.UserID = auth.ScopeUserIDFromContext(ctx)
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

// normalizeContextTerminalEvent maps a generic failed event to the terminal
// request state when the parent context has already been cancelled or expired.
// Normalization happens before metrics, audit persistence, and live request sinks
// observe the event, keeping all three surfaces consistent.
func normalizeContextTerminalEvent(ctx context.Context, event ToolEvent) ToolEvent {
	if event.Type != ToolEventFailed || ctx == nil {
		return event
	}
	if event.Data == nil {
		event.Data = map[string]interface{}{}
	}
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		event.Type = ToolEventCancelled
		event.Data["cancelled"] = true
		event.Data["retryable"] = false
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		event.Type = ToolEventTimedOut
		event.Data["retryable"] = false
	}
	return event
}

func emitEvent(ctx context.Context, event ToolEvent) {
	event = normalizeContextTerminalEvent(ctx, event)
	recordGlobalToolMetric(event)
	emitGlobalEvent(event)
	if sink, ok := ctx.Value(eventSinkContextKey{}).(EventSink); ok && sink != nil {
		sink(event)
	}
}
