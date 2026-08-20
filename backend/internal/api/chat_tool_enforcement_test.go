package api

import (
	"strings"
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
	if e.toolChoiceForRound(0, true, "openai") != nil {
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

	choice := e.toolChoiceForRound(0, true, "openai")
	if choice == nil || choice.Mode != llm.ToolChoiceSpecific || choice.Name != "web_search" {
		t.Fatalf("round 0 must force the named tool, got %#v", choice)
	}
	if got := e.toolChoiceForRound(1, true, "openai"); got != nil {
		t.Errorf("round 1 must be unconstrained, got %#v", got)
	}
	if !e.providerEnforced {
		t.Error("providerEnforced must record that the constraint was sent")
	}
}

// TestToolEnforcementProviderEnforcedTracksAllowlist guards a metadata lie: the
// LLM layer drops tool_choice for providers off its allowlist, so recording
// enforcement unconditionally made the UI claim the provider had been asked to
// require the tool and answered anyway — when the request never carried the
// constraint.
func TestToolEnforcementProviderEnforcedTracksAllowlist(t *testing.T) {
	selection, _ := parseTurnToolSelection("specific", []string{"web_search"}, "web_search")

	supported := newToolEnforcement(selection)
	if choice := supported.toolChoiceForRound(0, true, "openai"); choice == nil {
		t.Fatal("a supported provider must receive the constraint")
	}
	if !supported.providerEnforced {
		t.Error("providerEnforced must be true for an allowlisted provider")
	}

	unsupported := newToolEnforcement(selection)
	_ = unsupported.toolChoiceForRound(0, true, "ollama")
	if unsupported.providerEnforced {
		t.Error("providerEnforced must be false when the field is dropped in transport")
	}

	meta := map[string]interface{}{}
	unsupported.applyTo(meta)
	if meta[metaToolEnforced] != false {
		t.Errorf("%s = %v, want false", metaToolEnforced, meta[metaToolEnforced])
	}
}

func TestToolEnforcementSkipsWhenNoToolsAdvertised(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", nil, "")
	e := newToolEnforcement(selection)
	// tool_choice with an empty catalog is a provider error on several backends.
	if got := e.toolChoiceForRound(0, false, "openai"); got != nil {
		t.Errorf("no tools advertised means no tool_choice, got %#v", got)
	}
}

func TestToolEnforcementRequiredAnyTool(t *testing.T) {
	selection, _ := parseTurnToolSelection("required", nil, "")
	e := newToolEnforcement(selection)
	choice := e.toolChoiceForRound(0, true, "openai")
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
	if got := e.toolChoiceForRound(0, true, "openai"); got != nil {
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
	_ = e.toolChoiceForRound(0, true, "openai")

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

// TestRetrievalFailureDoesNotForceASearchRetry pins the reasoning behind
// removing the escalation: after a preflight fails, forcing web_search would run
// the same plan against the same provider that just failed, while consuming the
// one round where the model could reach for the tool that can actually answer.
func TestRetrievalFailureDoesNotForceASearchRetry(t *testing.T) {
	text := readMessageHandlerSource(t)
	if strings.Contains(text, `requireTool("web_search")`) {
		t.Error("a failed retrieval must not force a web_search retry; it starves MCP and plugin tools")
	}
	// The honest hedge must still be applied.
	if !strings.Contains(text, "degradedAnswerDirective") {
		t.Error("a failed retrieval must still constrain the answer")
	}
}

func TestFilterOutBrowserTools(t *testing.T) {
	var nav, shot, calc llm.Tool
	nav.Function.Name = "browser_navigate"
	shot.Function.Name = "browser_screenshot"
	calc.Function.Name = "calculator"

	filtered := filterOutBrowserTools([]llm.Tool{nav, calc, shot})
	if len(filtered) != 1 || filtered[0].Function.Name != "calculator" {
		t.Errorf("browser tools must be withheld from the non-streaming catalog, got %#v", filtered)
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
