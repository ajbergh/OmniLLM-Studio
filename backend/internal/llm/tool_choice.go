package llm

import "strings"

// ToolChoiceMode constrains whether, and which, tool the model must call.
//
// Before this existed there was no provider-level way to require a tool call:
// "required" mode was a sentence in the system prompt and a filter on which tool
// definitions were advertised. Models routinely answered from memory anyway,
// which is how current-information turns produced confident ungrounded answers
// while the backend believed a tool had been requested.
type ToolChoiceMode string

const (
	// ToolChoiceAuto lets the model decide. Equivalent to omitting the field.
	ToolChoiceAuto ToolChoiceMode = "auto"
	// ToolChoiceNone forbids tool calls.
	ToolChoiceNone ToolChoiceMode = "none"
	// ToolChoiceRequired demands at least one tool call before a final answer.
	ToolChoiceRequired ToolChoiceMode = "required"
	// ToolChoiceSpecific demands a named tool.
	ToolChoiceSpecific ToolChoiceMode = "specific"
)

// ToolChoice is the provider-neutral form. It is translated per provider at
// serialization time rather than stored in wire format, so callers do not need
// to know which dialect the active provider speaks.
type ToolChoice struct {
	Mode ToolChoiceMode `json:"mode"`
	// Name is the tool that must be called when Mode is ToolChoiceSpecific.
	Name string `json:"name,omitempty"`
}

// RequireTool builds a ToolChoice demanding one named tool.
func RequireTool(name string) *ToolChoice {
	name = strings.TrimSpace(name)
	if name == "" {
		return &ToolChoice{Mode: ToolChoiceRequired}
	}
	return &ToolChoice{Mode: ToolChoiceSpecific, Name: name}
}

// RequireAnyTool builds a ToolChoice demanding at least one tool call.
func RequireAnyTool() *ToolChoice {
	return &ToolChoice{Mode: ToolChoiceRequired}
}

// toolChoiceProviders lists provider types known to honor the OpenAI-compatible
// tool_choice field.
//
// The list is an allowlist rather than a denylist on purpose. A provider that
// rejects an unknown field returns 400 and breaks the whole turn, which is a
// far worse failure than falling back to advisory behavior — and the tool loop
// verifies afterwards that the required tool actually ran, so an omitted field
// degrades gracefully rather than silently.
var toolChoiceProviders = map[string]bool{
	"openai":     true,
	"anthropic":  true,
	"openrouter": true,
	"groq":       true,
	"together":   true,
	"mistral":    true,
	"gemini":     true,
}

// SupportsToolChoice reports whether a provider type accepts tool_choice.
func SupportsToolChoice(providerType string) bool {
	return toolChoiceProviders[strings.ToLower(strings.TrimSpace(providerType))]
}

// wireValue renders the OpenAI-compatible tool_choice value for a provider.
//
// The second return value is false when the field must be omitted: either the
// mode is advisory (auto), or the provider is not on the allowlist.
func (t *ToolChoice) wireValue(providerType string) (interface{}, bool) {
	if t == nil {
		return nil, false
	}
	if !SupportsToolChoice(providerType) {
		return nil, false
	}
	switch t.Mode {
	case ToolChoiceNone:
		return "none", true
	case ToolChoiceRequired:
		return "required", true
	case ToolChoiceSpecific:
		if strings.TrimSpace(t.Name) == "" {
			// A specific choice with no name is meaningless; fall back to
			// requiring some tool rather than sending an invalid body.
			return "required", true
		}
		return map[string]interface{}{
			"type":     "function",
			"function": map[string]interface{}{"name": t.Name},
		}, true
	default:
		// auto and anything unrecognized: omit and let the model decide.
		return nil, false
	}
}

// applyToolChoice sets tool_choice on an outgoing OpenAI-compatible body.
//
// tool_choice is only meaningful alongside a tool catalog: sending "required"
// with no tools is a provider error on several backends.
func applyToolChoice(body map[string]interface{}, req ChatRequest, providerType string) {
	if req.ToolChoice == nil || len(req.Tools) == 0 {
		return
	}
	if value, ok := req.ToolChoice.wireValue(providerType); ok {
		body["tool_choice"] = value
	}
}
