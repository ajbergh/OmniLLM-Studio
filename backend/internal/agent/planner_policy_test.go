package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type plannerPolicyTestTool struct {
	name string
}

func (t plannerPolicyTestTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        t.name,
		Description: "Planner policy test tool " + t.name,
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Category:    "planner-policy-test",
		Enabled:     true,
		ReadOnly:    true,
	}
}

func (t plannerPolicyTestTool) Validate(json.RawMessage) error { return nil }
func (t plannerPolicyTestTool) Execute(context.Context, json.RawMessage) (*tools.ToolResult, error) {
	return &tools.ToolResult{Content: "ok"}, nil
}

func TestPlannerDoesNotAdvertiseDeniedTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(plannerPolicyTestTool{name: "planner_allowed_tool"})
	registry.MustRegister(plannerPolicyTestTool{name: "planner_denied_tool"})
	_ = tools.NewExecutor(registry, func(name string) string {
		if name == "planner_denied_tool" {
			return "deny"
		}
		return "allow"
	}, 0)
	planner := NewPlanner(nil, registry)

	definitions := planner.selectTools("planner denied tool", nil)
	for _, definition := range definitions {
		if definition.Name == "planner_denied_tool" {
			t.Fatal("planner advertised a tool that Settings policy denied")
		}
	}
}

func TestPlannerRejectsHallucinatedDeniedToolStep(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(plannerPolicyTestTool{name: "planner_denied_tool"})
	_ = tools.NewExecutor(registry, func(name string) string {
		if name == "planner_denied_tool" {
			return "deny"
		}
		return "allow"
	}, 0)
	planner := NewPlanner(nil, registry)

	validated, errs := planner.validatePlan([]PlanStep{{
		ID:          "step-1",
		Type:        StepTypeToolCall,
		Description: "Run the denied tool",
		ToolName:    "planner_denied_tool",
		InputJSON:   json.RawMessage(`{}`),
	}})

	if len(errs) == 0 {
		t.Fatal("expected denied tool plan step to fail validation")
	}
	if !strings.Contains(strings.Join(errs, " "), "unavailable tool") {
		t.Fatalf("validation errors = %v, want unavailable tool error", errs)
	}
	for _, step := range validated {
		if step.Type == StepTypeToolCall && step.ToolName == "planner_denied_tool" {
			t.Fatal("validated plan retained a denied tool step")
		}
	}
}

func TestPlannerMarksAskPolicyAsRequiringApproval(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(plannerPolicyTestTool{name: "planner_ask_tool"})
	_ = tools.NewExecutor(registry, func(name string) string {
		if name == "planner_ask_tool" {
			return "ask"
		}
		return "allow"
	}, 0)
	planner := NewPlanner(nil, registry)

	validated, errs := planner.validatePlan([]PlanStep{{
		ID:          "step-1",
		Type:        StepTypeToolCall,
		Description: "Run the approval-gated tool",
		ToolName:    "planner_ask_tool",
		InputJSON:   json.RawMessage(`{}`),
	}})
	if len(errs) != 0 {
		t.Fatalf("validatePlan errors: %v", errs)
	}
	if len(validated) == 0 || !validated[0].RequiresApproval {
		t.Fatalf("validated step = %#v, want requires_approval=true", validated)
	}
}
