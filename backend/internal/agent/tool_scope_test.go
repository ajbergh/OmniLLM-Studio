package agent

import (
	"context"
	"testing"
)

func TestAssistantToolScopeIsRestrictive(t *testing.T) {
	ctx := ContextWithAllowedTools(context.Background(), []string{"calculator", "web_search"})
	if !toolAllowedByContext(ctx, "calculator") || !toolAllowedByContext(ctx, "web_search") {
		t.Fatal("allowed tools should remain available")
	}
	if toolAllowedByContext(ctx, "sports_lookup") {
		t.Fatal("tool outside profile allowlist should be rejected")
	}
}

func TestEmptyAssistantToolScopeIsBackwardCompatible(t *testing.T) {
	ctx := ContextWithAllowedTools(context.Background(), nil)
	if !toolAllowedByContext(ctx, "sports_lookup") {
		t.Fatal("empty profile tool list should remain unrestricted")
	}
}
