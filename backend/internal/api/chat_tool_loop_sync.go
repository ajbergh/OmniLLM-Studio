package api

import (
	"context"
	"log"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const (
	syncMaxToolLoops           = 6
	syncMaxToolResultCharsTurn = 150000
)

// filterOutBrowserTools removes browser automation from a tool catalog.
//
// The generic tool round deliberately refuses browser calls
// (chatToolExecution.genericRuntimeEligible returns false for them) because the
// streaming loop routes them through a browser-aware sequential path that owns
// provider-type context, the per-turn navigation cap, the visited-URL cache, and
// result sanitization. The non-streaming loop has no progress channel to drive
// them from, so they are withheld rather than given a weaker second path.
func filterOutBrowserTools(toolsList []llm.Tool) []llm.Tool {
	filtered := make([]llm.Tool, 0, len(toolsList))
	for _, tool := range toolsList {
		if strings.HasPrefix(tool.Function.Name, "browser_") {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

// syncToolLoopOutcome carries everything the non-streaming handler needs to
// finish a turn after tool rounds have run.
type syncToolLoopOutcome struct {
	Content   string
	Thinking  string
	Provider  string
	Model     string
	TokenIn   *int
	TokenOut  *int
	Cost      *float64
	ToolCalls []llm.ToolCall
	Results   []tools.ToolResult
	// Citations are provider-native grounding sources accumulated across rounds.
	Citations []llm.Citation
	// LimitReached records that the turn stopped on the result-context or
	// round budget rather than because the model was finished.
	LimitReached bool
}

// runSyncToolLoop executes tool rounds for a non-streaming turn.
//
// Non-streaming chat previously had no tool support whatsoever:
// selectChatToolsForContext was only ever called from the streaming handler, so
// llmReq.Tools was never set and ChatComplete could not produce a tool call. The
// consequence for current-information turns was that the orchestrator answered
// and returned, and nothing could run afterwards — a request to "find the latest
// prices and calculate the total" got the prices and no total.
//
// The loop is deliberately smaller than the streaming one: no SSE progress, no
// browser-navigation budget, and a lower round cap, because a non-streaming
// caller is waiting on a single response and long tool chains belong on the
// streaming path.
func (h *MessageHandler) runSyncToolLoop(
	ctx context.Context,
	llmReq llm.ChatRequest,
	providerType string,
	enforcement *toolEnforcement,
	scope tools.InvocationScope,
) (syncToolLoopOutcome, error) {
	var outcome syncToolLoopOutcome
	usedChars := 0

	for round := 0; round < syncMaxToolLoops; round++ {
		// Only the opening round is constrained; a provider held at
		// tool_choice=required never produces a final answer.
		llmReq.ToolChoice = enforcement.toolChoiceForRound(round, len(llmReq.Tools) > 0, providerType)

		resp, err := h.llmSvc.ChatComplete(ctx, llmReq)
		if err != nil {
			return outcome, err
		}

		outcome.Provider = resp.Provider
		outcome.Model = resp.Model
		outcome.TokenIn = addTokenCount(outcome.TokenIn, resp.TokenInput)
		outcome.TokenOut = addTokenCount(outcome.TokenOut, resp.TokenOutput)
		// Cost is per request, like tokens. Overwriting would report the last
		// round's cost against the whole turn's token count.
		outcome.Cost = addCost(outcome.Cost, resp.Cost)
		if resp.Thinking != "" {
			outcome.Thinking += resp.Thinking
		}
		outcome.Citations = llm.MergeCitations(outcome.Citations, resp.Citations)

		if len(resp.ToolCalls) == 0 {
			outcome.Content = resp.Content
			return outcome, nil
		}

		calls := llm.NormalizeToolCalls(providerType, resp.ToolCalls)
		enforcement.observe(calls)
		outcome.ToolCalls = append(outcome.ToolCalls, calls...)

		llmReq.Messages = append(llmReq.Messages, llm.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: calls,
		})

		toolCtx := tools.ContextWithInvocationScope(ctx, scope)
		execution := newChatToolExecutionForContext(toolCtx, h.toolExecutor, calls)
		roundOutcome := executeGenericChatToolRound(
			toolCtx,
			h.toolExecutor,
			execution.Plan,
			usedChars,
			syncMaxToolResultCharsTurn,
		)
		usedChars = roundOutcome.UsedChars

		for _, item := range roundOutcome.Processed {
			outcome.Results = append(outcome.Results, item.MetadataResult)
			llmReq.Messages = append(llmReq.Messages, item.Message)
		}

		lastRound := round == syncMaxToolLoops-1
		if roundOutcome.LimitReached || lastRound {
			outcome.LimitReached = true
			reason := "The maximum number of tool-call rounds was reached."
			if roundOutcome.LimitReached {
				reason = "The tool result context budget was reached."
			}
			content, err := h.completeSyncFinalAnswer(ctx, llmReq, reason, &outcome)
			if err != nil {
				return outcome, err
			}
			outcome.Content = content
			return outcome, nil
		}
	}

	return outcome, nil
}

// completeSyncFinalAnswer asks for a final answer with tools withdrawn.
func (h *MessageHandler) completeSyncFinalAnswer(
	ctx context.Context,
	llmReq llm.ChatRequest,
	reason string,
	outcome *syncToolLoopOutcome,
) (string, error) {
	llmReq.Tools = nil
	llmReq.ToolChoice = nil
	llmReq.Messages = append(llmReq.Messages, llm.ChatMessage{
		Role: "system",
		Content: reason +
			" Do not call any more tools. Give the best possible final answer using the tool results already present in the conversation.",
	})
	resp, err := h.llmSvc.ChatComplete(ctx, llmReq)
	if err != nil {
		return "", err
	}
	outcome.Provider = resp.Provider
	outcome.Model = resp.Model
	outcome.TokenIn = addTokenCount(outcome.TokenIn, resp.TokenInput)
	outcome.TokenOut = addTokenCount(outcome.TokenOut, resp.TokenOutput)
	outcome.Cost = addCost(outcome.Cost, resp.Cost)
	if resp.Thinking != "" {
		outcome.Thinking += resp.Thinking
	}
	outcome.Citations = llm.MergeCitations(outcome.Citations, resp.Citations)
	return resp.Content, nil
}

// addCost accumulates optional per-request costs across rounds.
func addCost(total *float64, next *float64) *float64 {
	if next == nil {
		return total
	}
	if total == nil {
		value := *next
		return &value
	}
	sum := *total + *next
	return &sum
}

// addTokenCount accumulates optional token counts across rounds. Tokens are
// per-request, so a multi-round turn costs their sum; reporting only the last
// round would understate the turn.
func addTokenCount(total *int, next *int) *int {
	if next == nil {
		return total
	}
	if total == nil {
		value := *next
		return &value
	}
	sum := *total + *next
	return &sum
}

// logSyncToolLoopFailure records a tool-loop error without leaking provider
// detail to the client.
func logSyncToolLoopFailure(err error) {
	log.Printf("ERROR: non-streaming tool loop: %v", err)
}
