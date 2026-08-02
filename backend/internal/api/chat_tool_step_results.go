package api

import (
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const toolResultContextLimitMessage = "Tool result context limit reached for this turn."
const toolResultContextTruncationSuffix = "\n\n[tool result truncated at the per-turn context limit]"
const toolSkippedAfterContextLimitMessage = "Tool was not executed because this turn reached the tool result context limit."

// chatToolStepResult contains the two representations Chat Studio needs after a
// generic tool call completes: a sanitized user-visible result and the provider
// tool message appended to the next model request.
type chatToolStepResult struct {
	ToolCallID     string
	ToolName       string
	MetadataResult tools.ToolResult
	Message        llm.ChatMessage
}

// processChatToolStepResults applies the existing Chat Studio result-context
// budget to one completed execution step. All calls in a parallel step have
// already run, so every call still receives exactly one provider tool message.
// Once the budget is exhausted, later messages in the same step receive the
// limit marker and the caller must not begin another execution step.
func processChatToolStepResults(calls []tools.ToolCall, results []*tools.ToolResult, usedChars, maxChars int) ([]chatToolStepResult, int, bool) {
	processed := make([]chatToolStepResult, 0, len(calls))
	limitReached := false

	for index, call := range calls {
		var result *tools.ToolResult
		if index < len(results) {
			result = results[index]
		}
		metadataResult := safeToolResultForMetadata(call.Name, result)

		toolOutput := ""
		if result != nil {
			toolOutput = result.Content
		}
		remaining := maxChars - usedChars
		switch {
		case limitReached || remaining <= 0:
			toolOutput = toolResultContextLimitMessage
			limitReached = true
		case len(toolOutput) > remaining:
			toolOutput = toolOutput[:remaining] + toolResultContextTruncationSuffix
			limitReached = true
		}
		usedChars += len(toolOutput)

		processed = append(processed, chatToolStepResult{
			ToolCallID:     call.ID,
			ToolName:       call.Name,
			MetadataResult: metadataResult,
			Message: llm.ChatMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: call.ID,
				Name:       call.Name,
			},
		})
	}

	return processed, usedChars, limitReached
}

// skippedChatToolResults creates terminal results for calls in execution steps
// that must not start after a previous step exhausted the per-turn context
// budget. It mirrors the existing sequential handler's TOOL_RESULT_LIMIT shape.
func skippedChatToolResults(calls []tools.ToolCall) []chatToolStepResult {
	processed := make([]chatToolStepResult, 0, len(calls))
	for _, call := range calls {
		metadataResult := tools.ToolResult{
			ToolCallID: call.ID,
			Content:    toolSkippedAfterContextLimitMessage,
			IsError:    true,
			Metadata: map[string]interface{}{
				"error_code": "TOOL_RESULT_LIMIT",
				"retryable":  false,
				"tool_name":  call.Name,
			},
		}
		processed = append(processed, chatToolStepResult{
			ToolCallID:     call.ID,
			ToolName:       call.Name,
			MetadataResult: metadataResult,
			Message: llm.ChatMessage{
				Role:       "tool",
				Content:    toolSkippedAfterContextLimitMessage,
				ToolCallID: call.ID,
				Name:       call.Name,
			},
		})
	}
	return processed
}
