package api

import (
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

// Metadata keys describing whether a turn's tool requirement was honored.
const (
	metaToolRequired    = "tool_required"
	metaToolEnforced    = "tool_enforced"
	metaToolUnfulfilled = "tool_requirement_unfulfilled"
)

// toolEnforcement tracks a turn's tool requirement across the tool loop.
//
// "Required" tool mode used to be advisory in every sense: a filter on which
// definitions were advertised, plus a sentence in the system prompt. Nothing
// asked the provider to force the call, and nothing checked afterwards that it
// had happened. A model that ignored the instruction produced an ordinary
// answer, and the backend recorded the turn as if the tool had run.
//
// Enforcement now has two layers, because neither is sufficient alone:
//
//  1. Provider-level. ToolChoice is sent on the first round so a supporting
//     provider refuses to answer without calling the tool.
//  2. Post-hoc. Providers that ignore tool_choice — or are not on the allowlist
//     at all — are caught by observing the calls the loop actually made. Content
//     has already streamed by then and cannot be retracted, so the outcome is
//     recorded in metadata for the UI rather than silently accepted.
type toolEnforcement struct {
	// requiredTool is the tool that must run. Empty with active set means "any
	// tool will do".
	requiredTool string
	// active is false when the turn has no tool requirement.
	active bool
	// satisfied becomes true once the requirement is met.
	satisfied bool
	// providerEnforced records whether the provider was asked to force the call.
	providerEnforced bool
}

// newToolEnforcement derives the requirement from the turn's tool selection.
func newToolEnforcement(selection turnToolSelection) toolEnforcement {
	switch selection.Mode {
	case turnToolModeRequired, turnToolModeSpecific:
		return toolEnforcement{
			requiredTool: strings.TrimSpace(selection.RequiredTool),
			active:       true,
		}
	default:
		return toolEnforcement{}
	}
}

// toolChoiceForRound returns the provider-level constraint for one loop round.
//
// Only the first round is forced. Later rounds must be free to produce the final
// answer, or the loop would never terminate: a provider held at "required" keeps
// calling tools until the round limit.
func (e *toolEnforcement) toolChoiceForRound(round int, hasTools bool, providerType string) *llm.ToolChoice {
	if !e.active || e.satisfied || round != 0 || !hasTools {
		return nil
	}
	// Record enforcement only when the provider actually accepts tool_choice.
	// The LLM layer drops the field for providers off its allowlist, so setting
	// this unconditionally made the metadata claim the provider had been asked to
	// require the tool and answered anyway — when the request never carried the
	// constraint at all.
	e.providerEnforced = llm.SupportsToolChoice(providerType)
	if e.requiredTool != "" {
		return llm.RequireTool(e.requiredTool)
	}
	return llm.RequireAnyTool()
}

// observe records the calls made in one round.
func (e *toolEnforcement) observe(calls []llm.ToolCall) {
	if !e.active || e.satisfied {
		return
	}
	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			continue
		}
		if e.requiredTool == "" || name == e.requiredTool {
			e.satisfied = true
			return
		}
	}
}

// unfulfilled reports a turn that required a tool and did not get one.
func (e toolEnforcement) unfulfilled() bool {
	return e.active && !e.satisfied
}

// applyTo records the outcome on outgoing metadata so the UI can mark an answer
// that was supposed to be tool-backed but is not.
func (e toolEnforcement) applyTo(meta map[string]interface{}) {
	if meta == nil || !e.active {
		return
	}
	if e.requiredTool != "" {
		meta[metaToolRequired] = e.requiredTool
	} else {
		meta[metaToolRequired] = true
	}
	meta[metaToolEnforced] = e.providerEnforced
	if e.unfulfilled() {
		meta[metaToolUnfulfilled] = true
	}
}

// unfulfilledToolDirective is appended before the final answer when a required
// tool never ran. It cannot undo streamed content, so it is used on the retry
// attempt that happens before any content is emitted.
func unfulfilledToolDirective(tool string) string {
	target := "an available tool"
	if tool != "" {
		target = "the " + tool + " tool"
	}
	return "TOOL REQUIREMENT NOT MET: you answered without calling " + target +
		". Call it now. If you genuinely cannot, say explicitly that you could not use " + target +
		" and that the answer is therefore unverified. Do not present unverified claims as fact."
}
