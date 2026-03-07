package agent

import (
	"encoding/json"
	"testing"
	"time"
)

// --------------- IsTerminalRunStatus ---------------

func TestIsTerminalRunStatus(t *testing.T) {
	tests := []struct {
		status   string
		terminal bool
	}{
		{RunStatusCompleted, true},
		{RunStatusFailed, true},
		{RunStatusCancelled, true},
		{RunStatusRunning, false},
		{RunStatusPlanning, false},
		{RunStatusAwaitingApproval, false},
		{RunStatusPaused, false},
		{"unknown", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := IsTerminalRunStatus(tc.status); got != tc.terminal {
			t.Errorf("IsTerminalRunStatus(%q) = %v, want %v", tc.status, got, tc.terminal)
		}
	}
}

// --------------- ValidStepTypes ---------------

func TestValidStepTypes(t *testing.T) {
	expected := []string{StepTypeThink, StepTypeToolCall, StepTypeApproval, StepTypeMessage}
	for _, st := range expected {
		if !ValidStepTypes[st] {
			t.Errorf("ValidStepTypes[%q] should be true", st)
		}
	}
	invalid := []string{"invalid", "execute", "", "THINK"}
	for _, st := range invalid {
		if ValidStepTypes[st] {
			t.Errorf("ValidStepTypes[%q] should be false", st)
		}
	}
}

// --------------- DefaultRunnerConfig ---------------

func TestDefaultRunnerConfig(t *testing.T) {
	cfg := DefaultRunnerConfig()
	if cfg.MaxSteps != DefaultMaxSteps {
		t.Errorf("MaxSteps = %d, want %d", cfg.MaxSteps, DefaultMaxSteps)
	}
	if cfg.MaxDuration != DefaultMaxDuration {
		t.Errorf("MaxDuration = %v, want %v", cfg.MaxDuration, DefaultMaxDuration)
	}
}

func TestDefaultRunnerConfigValues(t *testing.T) {
	if DefaultMaxSteps != 20 {
		t.Errorf("DefaultMaxSteps = %d, want 20", DefaultMaxSteps)
	}
	if DefaultMaxDuration != 10*time.Minute {
		t.Errorf("DefaultMaxDuration = %v, want 10m", DefaultMaxDuration)
	}
}

// --------------- ParsePlan ---------------

func TestParsePlanBasic(t *testing.T) {
	raw := `[{"type":"think","description":"Analyze"},{"type":"message","description":"Reply"}]`
	steps, err := ParsePlan(raw)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("len(steps) = %d, want 2", len(steps))
	}
	if steps[0].Type != StepTypeThink {
		t.Errorf("steps[0].Type = %q, want %q", steps[0].Type, StepTypeThink)
	}
	if steps[0].Description != "Analyze" {
		t.Errorf("steps[0].Description = %q, want %q", steps[0].Description, "Analyze")
	}
	if steps[1].Type != StepTypeMessage {
		t.Errorf("steps[1].Type = %q, want %q", steps[1].Type, StepTypeMessage)
	}
}

func TestParsePlanWithToolCall(t *testing.T) {
	raw := `[{"type":"tool_call","description":"Search","tool_name":"web_search","input_json":{"query":"test"}}]`
	steps, err := ParsePlan(raw)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("len(steps) = %d, want 1", len(steps))
	}
	if steps[0].ToolName != "web_search" {
		t.Errorf("ToolName = %q, want %q", steps[0].ToolName, "web_search")
	}
	// InputJSON should be valid json.RawMessage
	var obj map[string]string
	if err := json.Unmarshal(steps[0].InputJSON, &obj); err != nil {
		t.Fatalf("Unmarshal InputJSON: %v", err)
	}
	if obj["query"] != "test" {
		t.Errorf("InputJSON query = %q, want %q", obj["query"], "test")
	}
}

func TestParsePlanEmpty(t *testing.T) {
	steps, err := ParsePlan("[]")
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("len(steps) = %d, want 0", len(steps))
	}
}

func TestParsePlanInvalidJSON(t *testing.T) {
	_, err := ParsePlan("not json at all")
	if err == nil {
		t.Error("ParsePlan with invalid JSON should return error")
	}
}

func TestParsePlanNotArray(t *testing.T) {
	_, err := ParsePlan(`{"type":"think"}`)
	if err == nil {
		t.Error("ParsePlan with non-array JSON should return error")
	}
}

// --------------- EncodePlan ---------------

func TestEncodePlan(t *testing.T) {
	steps := []PlanStep{
		{Type: StepTypeThink, Description: "Think about it"},
		{Type: StepTypeMessage, Description: "Reply"},
	}
	encoded, err := EncodePlan(steps)
	if err != nil {
		t.Fatalf("EncodePlan: %v", err)
	}
	// Verify round-trip
	decoded, err := ParsePlan(encoded)
	if err != nil {
		t.Fatalf("ParsePlan(encoded): %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("round-trip len = %d, want 2", len(decoded))
	}
	if decoded[0].Type != StepTypeThink || decoded[0].Description != "Think about it" {
		t.Errorf("round-trip step 0 mismatch: %+v", decoded[0])
	}
	if decoded[1].Type != StepTypeMessage || decoded[1].Description != "Reply" {
		t.Errorf("round-trip step 1 mismatch: %+v", decoded[1])
	}
}

func TestEncodePlanWithInputJSON(t *testing.T) {
	steps := []PlanStep{
		{
			Type:        StepTypeToolCall,
			Description: "Search",
			ToolName:    "search",
			InputJSON:   json.RawMessage(`{"q":"hello"}`),
		},
	}
	encoded, err := EncodePlan(steps)
	if err != nil {
		t.Fatalf("EncodePlan: %v", err)
	}
	decoded, err := ParsePlan(encoded)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if decoded[0].ToolName != "search" {
		t.Errorf("ToolName = %q, want %q", decoded[0].ToolName, "search")
	}
	var obj map[string]string
	if err := json.Unmarshal(decoded[0].InputJSON, &obj); err != nil {
		t.Fatalf("Unmarshal InputJSON: %v", err)
	}
	if obj["q"] != "hello" {
		t.Errorf("InputJSON q = %q, want %q", obj["q"], "hello")
	}
}

func TestEncodePlanEmpty(t *testing.T) {
	encoded, err := EncodePlan([]PlanStep{})
	if err != nil {
		t.Fatalf("EncodePlan: %v", err)
	}
	if encoded != "[]" {
		t.Errorf("encoded = %q, want %q", encoded, "[]")
	}
}

// --------------- PlanStep JSON marshalling ---------------

func TestPlanStepMarshalOmitsEmpty(t *testing.T) {
	step := PlanStep{Type: StepTypeThink, Description: "Reason"}
	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if _, ok := m["tool_name"]; ok {
		t.Error("tool_name should be omitted when empty")
	}
	if _, ok := m["input_json"]; ok {
		t.Error("input_json should be omitted when nil")
	}
}
