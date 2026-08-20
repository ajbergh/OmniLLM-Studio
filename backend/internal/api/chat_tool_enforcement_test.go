package api

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

func toolCallNamed(name string) llm.ToolCall {
	var call llm.ToolCall
	call.Function.Name = name
	return call
}

func TestToolEnforcementInactiveForAutoMode(t *testing.T) {
	selection, err := parseTurnToolSelection("auto", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	e := newToolEnforcement(selection)
	if e.active {
		t.Error("auto mode must not enforce anything")
	}
	if e.toolChoiceForRound(0, true) != nil {
		t.Error("auto mode must not constrain the provider")
	}
	if e.unfulfilled() {
		t.Error("an inactive requirement is never unfulfilled")
	}
	meta := map[string]interface{}{}
	e.applyTo(meta)
	if len(meta) != 0 {
		t.Errorf("inactive enforcement must not write metadata, got %v", meta)
	}
}

// TestToolEnforcementForcesFirstRoundOnly guards the termination property: a
// provider held at tool_choice=required never produces a final answer, so only
// the opening round may be constrained.
func TestToolEnforcementForcesFirstRoundOnly(t *testing.T) {
	selection, err := parseTurnToolSelection("specific", []string{"web_search"}, "web_search")
	if err != nil {
		t.Fatal(err)
	}
	e := newToolEnforcement(selection)

	choice := e.toolChoiceForRound(0, true)
	if choice == nil || choice.Mode != llm.ToolChoiceSpecific || choice.Name != "web_search" {
		t.Fatalf("round 0 must force the named tool, got %#v", choice)
	}
	if got := e.toolChoiceForRound(1, true); got != nil {
		t.Errorf("round 1 must be unconstrained, got %#v", got)
	}
	if !e.providerEnforced {
		t.Error("providerEnforced must record that the constraint was sent")
	}
}

func TestToolEnforcementSkipsWhenNoToolsAdvertised(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", nil, "")
	e := newToolEnforcement(selection)
	// tool_choice with an empty catalog is a provider error on several backends.
	if got := e.toolChoiceForRound(0, false); got != nil {
		t.Errorf("no tools advertised means no tool_choice, got %#v", got)
	}
}

func TestToolEnforcementRequiredAnyTool(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", nil, "")
	e := newToolEnforcement(selection)
	choice := e.toolChoiceForRound(0, true)
	if choice == nil || choice.Mode != llm.ToolChoiceRequired {
		t.Fatalf("required mode with no named tool must demand any tool, got %#v", choice)
	}

	e.observe([]llm.ToolCall{toolCallNamed("calculator")})
	if e.unfulfilled() {
		t.Error("any tool call satisfies an unnamed requirement")
	}
}

func TestToolEnforcementObserveMatchesNamedToolOnly(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", []string{"web_search", "calculator"}, "web_search")
	e := newToolEnforcement(selection)

	e.observe([]llm.ToolCall{toolCallNamed("calculator")})
	if !e.unfulfilled() {
		t.Error("a different tool must not satisfy a named requirement")
	}

	e.observe([]llm.ToolCall{toolCallNamed("web_search")})
	if e.unfulfilled() {
		t.Error("the named tool must satisfy the requirement")
	}
	// Once satisfied, later rounds must not be re-constrained.
	if got := e.toolChoiceForRound(0, true); got != nil {
		t.Errorf("a satisfied requirement must not re-force the provider, got %#v", got)
	}
}

func TestToolEnforcementObserveIgnoresBlankNames(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", nil, "")
	e := newToolEnforcement(selection)
	e.observe([]llm.ToolCall{toolCallNamed("   ")})
	if !e.unfulfilled() {
		t.Error("a call with no tool name must not satisfy the requirement")
	}
}

// TestToolEnforcementMetadataRecordsViolation is the post-hoc half of
// enforcement: content has already streamed by the time we know the tool was
// skipped, so the outcome has to reach the UI through metadata.
func TestToolEnforcementMetadataRecordsViolation(t *testing.T) {
	selection, _ := parseTurnToolSelection("specific", []string{"web_search"}, "web_search")
	e := newToolEnforcement(selection)
	_ = e.toolChoiceForRound(0, true)

	meta := map[string]interface{}{}
	e.applyTo(meta)
	if meta[metaToolRequired] != "web_search" {
		t.Errorf("%s = %v, want web_search", metaToolRequired, meta[metaToolRequired])
	}
	if meta[metaToolEnforced] != true {
		t.Errorf("%s should record that the provider was constrained", metaToolEnforced)
	}
	if meta[metaToolUnfulfilled] != true {
		t.Errorf("%s must be set when the tool never ran", metaToolUnfulfilled)
	}

	e.observe([]llm.ToolCall{toolCallNamed("web_search")})
	satisfied := map[string]interface{}{}
	e.applyTo(satisfied)
	if _, present := satisfied[metaToolUnfulfilled]; present {
		t.Error("a satisfied requirement must not be marked unfulfilled")
	}
}

func TestToolEnforcementRequireToolEscalation(t *testing.T) {
	selection, _ := parseTurnToolSelection("auto", nil, "")
	e := newToolEnforcement(selection).requireTool("web_search")
	if !e.active || e.requiredTool != "web_search" {
		t.Fatalf("escalation must activate the requirement, got %#v", e)
	}

	// A client-specified requirement wins: silently substituting a different tool
	// would be worse than not escalating.
	clientSelection, _ := parseTurnToolSelection("specific", []string{"calculator"}, "calculator")
	kept := newToolEnforcement(clientSelection).requireTool("web_search")
	if kept.requiredTool != "calculator" {
		t.Errorf("client requirement must win, got %q", kept.requiredTool)
	}

	// A blank name is a no-op rather than an accidental "any tool" requirement.
	unchanged := newToolEnforcement(selection).requireTool("  ")
	if unchanged.active {
		t.Error("escalating with a blank name must do nothing")
	}
}

func TestUnfulfilledToolDirective(t *testing.T) {
	named := unfulfilledToolDirective("web_search")
	if !containsAll(named, "web_search", "unverified") {
		t.Errorf("directive must name the tool and demand a hedge: %q", named)
	}
	anyTool := unfulfilledToolDirective("")
	if !containsAll(anyTool, "an available tool") {
		t.Errorf("unnamed directive should refer to any tool: %q", anyTool)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !containsAny(s, part) {
			return false
		}
	}
	return true
}
