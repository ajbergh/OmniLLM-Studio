package llm

import (
	"encoding/json"
	"testing"
)

func TestToolChoiceWireValue(t *testing.T) {
	cases := []struct {
		name     string
		choice   *ToolChoice
		provider string
		wantSet  bool
		wantJSON string
	}{
		{"auto is omitted", &ToolChoice{Mode: ToolChoiceAuto}, "openai", false, ""},
		{"nil is omitted", nil, "openai", false, ""},
		{"none", &ToolChoice{Mode: ToolChoiceNone}, "openai", true, `"none"`},
		{"required", &ToolChoice{Mode: ToolChoiceRequired}, "openai", true, `"required"`},
		{
			"specific", RequireTool("web_search"), "anthropic", true,
			`{"function":{"name":"web_search"},"type":"function"}`,
		},
		{
			"specific with empty name degrades to required",
			&ToolChoice{Mode: ToolChoiceSpecific}, "openai", true, `"required"`,
		},
		// Ollama is deliberately off the allowlist: a provider that rejects an
		// unknown field would break the whole turn, and the tool loop verifies
		// afterwards that the required tool actually ran.
		{"unsupported provider omits", RequireTool("web_search"), "ollama", false, ""},
		{"unknown provider omits", RequireTool("web_search"), "some-local-thing", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := tc.choice.wireValue(tc.provider)
			if ok != tc.wantSet {
				t.Fatalf("wireValue set = %v, want %v", ok, tc.wantSet)
			}
			if !ok {
				return
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != tc.wantJSON {
				t.Errorf("wireValue = %s, want %s", encoded, tc.wantJSON)
			}
		})
	}
}

func TestApplyToolChoiceRequiresToolCatalog(t *testing.T) {
	// "required" with no tools is a provider error on several backends.
	body := map[string]interface{}{}
	applyToolChoice(body, ChatRequest{ToolChoice: RequireAnyTool()}, "openai")
	if _, present := body["tool_choice"]; present {
		t.Error("tool_choice must not be sent without a tool catalog")
	}

	body = map[string]interface{}{}
	applyToolChoice(body, ChatRequest{
		ToolChoice: RequireAnyTool(),
		Tools:      []Tool{{Type: "function"}},
	}, "openai")
	if body["tool_choice"] != "required" {
		t.Errorf("tool_choice = %v, want required", body["tool_choice"])
	}
}

func TestRequireToolHelpers(t *testing.T) {
	if got := RequireTool("  web_search  "); got.Mode != ToolChoiceSpecific || got.Name != "web_search" {
		t.Errorf("RequireTool trimmed = %#v", got)
	}
	if got := RequireTool("   "); got.Mode != ToolChoiceRequired {
		t.Errorf("a blank name must degrade to required, got %#v", got)
	}
	if got := RequireAnyTool(); got.Mode != ToolChoiceRequired || got.Name != "" {
		t.Errorf("RequireAnyTool = %#v", got)
	}
}

func TestSupportsToolChoice(t *testing.T) {
	for _, p := range []string{"openai", "OpenAI", " anthropic ", "gemini", "openrouter"} {
		if !SupportsToolChoice(p) {
			t.Errorf("SupportsToolChoice(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"ollama", "lmstudio", "", "openai-compatible"} {
		if SupportsToolChoice(p) {
			t.Errorf("SupportsToolChoice(%q) = true, want false", p)
		}
	}
}
