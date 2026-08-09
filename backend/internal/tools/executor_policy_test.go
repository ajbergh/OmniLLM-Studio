package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type policyTestTool struct {
	definition ToolDefinition
	executed   *bool
}

func (t policyTestTool) Definition() ToolDefinition { return t.definition }
func (t policyTestTool) Validate(json.RawMessage) error { return nil }
func (t policyTestTool) Execute(context.Context, json.RawMessage) (*ToolResult, error) {
	if t.executed != nil {
		*t.executed = true
	}
	return &ToolResult{Content: "ok"}, nil
}

func newPolicyTestDefinition(name string, sideEffecting bool) ToolDefinition {
	return ToolDefinition{
		Name:          name,
		Description:   "Policy test tool",
		Parameters:    json.RawMessage(`{"type":"object"}`),
		Category:      "test",
		Enabled:       true,
		SideEffecting: sideEffecting,
		ReadOnly:      !sideEffecting,
	}
}

func TestEffectivePolicyUsesSafeDefinitionDefaults(t *testing.T) {
	readOnly := newPolicyTestDefinition("read_only", false)
	if got := EffectivePolicy(readOnly, ""); got != "allow" {
		t.Fatalf("read-only default policy = %q, want allow", got)
	}

	sideEffect := newPolicyTestDefinition("side_effect", true)
	if got := EffectivePolicy(sideEffect, ""); got != "ask" {
		t.Fatalf("side-effect default policy = %q, want ask", got)
	}

	highRisk := newPolicyTestDefinition("high_risk", false)
	highRisk.Risk = RiskHigh
	if got := EffectivePolicy(highRisk, ""); got != "ask" {
		t.Fatalf("high-risk default policy = %q, want ask", got)
	}

	if got := EffectivePolicy(sideEffect, "allow"); got != "allow" {
		t.Fatalf("explicit allow = %q, want allow", got)
	}
	if got := EffectivePolicy(readOnly, "deny"); got != "deny" {
		t.Fatalf("explicit deny = %q, want deny", got)
	}
}

func TestExecutorMissingSideEffectPolicyRequiresApproval(t *testing.T) {
	executed := false
	registry := NewRegistry()
	registry.MustRegister(policyTestTool{
		definition: newPolicyTestDefinition("side_effect", true),
		executed:   &executed,
	})
	executor := NewExecutor(registry, func(string) string { return "" }, 0)

	result := executor.Execute(context.Background(), ToolCall{
		ID:        "call-1",
		Name:      "side_effect",
		Arguments: json.RawMessage(`{}`),
	})

	if !result.IsError {
		t.Fatal("expected missing side-effect policy to require approval")
	}
	if executed {
		t.Fatal("side-effect tool executed without approval")
	}
	if result.Metadata[ApprovalStatusMetadataKey] != "required" {
		t.Fatalf("approval status = %v, want required", result.Metadata[ApprovalStatusMetadataKey])
	}
}

func TestExecutorPolicyReturnsDenyForUnknownTool(t *testing.T) {
	executor := NewExecutor(NewRegistry(), nil, 0)
	if got := executor.Policy("missing"); got != "deny" {
		t.Fatalf("unknown tool policy = %q, want deny", got)
	}
}

func TestExecutorRechecksDenyAfterApproval(t *testing.T) {
	executed := false
	policy := "ask"
	registry := NewRegistry()
	registry.MustRegister(policyTestTool{
		definition: newPolicyTestDefinition("mutable_policy", false),
		executed:   &executed,
	})
	executor := NewExecutor(registry, func(string) string { return policy }, 0)
	ctx := ContextWithApprovalHandler(context.Background(), func(context.Context, ApprovalRequest) (bool, error) {
		policy = "deny"
		return true, nil
	})

	result := executor.Execute(ctx, ToolCall{
		ID:        "call-2",
		Name:      "mutable_policy",
		Arguments: json.RawMessage(`{}`),
	})

	if !result.IsError || !strings.Contains(result.Content, "denied by policy") {
		t.Fatalf("result = %#v, want policy denial after approval", result)
	}
	if executed {
		t.Fatal("tool executed after policy changed to deny")
	}
}

func TestExecuteApprovedInvalidatedWhenToolTurnedOff(t *testing.T) {
	executed := false
	policy := "ask"
	registry := NewRegistry()
	registry.MustRegister(policyTestTool{
		definition: newPolicyTestDefinition("approved_then_denied", false),
		executed:   &executed,
	})
	executor := NewExecutor(registry, func(string) string { return policy }, 0)

	policy = "deny"
	result := executor.ExecuteApproved(context.Background(), ToolCall{
		ID:        "call-3",
		Name:      "approved_then_denied",
		Arguments: json.RawMessage(`{}`),
	})

	if !result.IsError || !strings.Contains(result.Content, "denied by policy") {
		t.Fatalf("result = %#v, want stale approval to be denied", result)
	}
	if result.Metadata[ApprovalStatusMetadataKey] != "invalidated" {
		t.Fatalf("approval status = %v, want invalidated", result.Metadata[ApprovalStatusMetadataKey])
	}
	if executed {
		t.Fatal("tool executed from stale approval after being turned off")
	}
}
