package tools

import (
	"context"
	"testing"
)

func TestContextWithInvocationScopeMergesInheritedScope(t *testing.T) {
	base := ContextWithInvocationScope(context.Background(), InvocationScope{
		UserID: "user-1", WorkspaceID: "workspace-1", MessageID: "message-1",
	})
	ctx := ContextWithInvocationScope(base, InvocationScope{
		ConversationID: "conversation-1", RunID: "run-1",
	})
	scope := InvocationScopeFromContext(ctx)
	if scope.UserID != "user-1" || scope.WorkspaceID != "workspace-1" || scope.MessageID != "message-1" {
		t.Fatalf("inherited scope was lost: %+v", scope)
	}
	if scope.ConversationID != "conversation-1" || scope.RunID != "run-1" {
		t.Fatalf("overlay scope was not applied: %+v", scope)
	}
}
