package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestScopedPolicyNarrowsExecutionAndDiscovery(t *testing.T) {
	calls := 0
	registry := NewRegistry()
	registry.MustRegister(batchTestTool{name: "scoped_tool", calls: &calls})
	executor := NewExecutor(registry, func(string) string { return "allow" }, 0)
	executor.SetScopedPermissionResolver(func(scope InvocationScope, name, base string) string {
		if scope.UserID == "blocked" && name == "scoped_tool" {
			return "deny"
		}
		return base
	})
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "blocked"})
	if got := executor.PolicyForContext(ctx, "scoped_tool"); got != "deny" {
		t.Fatalf("policy=%s", got)
	}
	if registry.IsAvailableForContext(ctx, "scoped_tool") {
		t.Fatal("scoped deny should hide tool from discovery")
	}
	result := executor.Execute(ctx, ToolCall{ID: "1", Name: "scoped_tool", Arguments: json.RawMessage(`{}`)})
	if !result.IsError || !strings.Contains(result.Content, "denied by policy") {
		t.Fatalf("unexpected result %#v", result)
	}
	if calls != 0 {
		t.Fatal("scoped denied tool executed")
	}
}

func TestToolSearchOmitsScopedDeniedCandidate(t *testing.T) {
	calls := 0
	registry := NewRegistry()
	registry.MustRegister(batchTestTool{name: "weather_alpha", calls: &calls})
	registry.MustRegister(batchTestTool{name: "weather_beta", calls: &calls})
	executor := NewExecutor(registry, func(string) string { return "allow" }, 0)
	executor.SetScopedPermissionResolver(func(scope InvocationScope, name, base string) string {
		if scope.WorkspaceID == "w1" && name == "weather_beta" {
			return "deny"
		}
		return base
	})
	search := NewToolSearch(registry)
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{WorkspaceID: "w1"})
	result, err := search.Execute(ctx, json.RawMessage(`{"query":"weather","limit":10}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "weather_alpha") || strings.Contains(result.Content, "weather_beta") {
		t.Fatalf("unexpected search result %s", result.Content)
	}
}
