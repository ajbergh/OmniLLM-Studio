package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// chatToolExecution adapts provider-normalized LLM tool calls to the generic
// tools runtime without losing provider call ordering, IDs, names, or arguments.
// Browser-managed calls remain on the existing Chat Studio path because that
// path owns navigation caching, per-turn limits, URL tracking, and browser result
// sanitization that are not part of the generic Executor contract.
type chatToolExecution struct {
	ProviderCalls []llm.ToolCall
	RuntimeCalls  []tools.ToolCall
	Plan          []tools.ExecutionStep
}

func newChatToolExecution(executor *tools.Executor, calls []llm.ToolCall) chatToolExecution {
	providerCalls := append([]llm.ToolCall(nil), calls...)
	runtimeCalls := make([]tools.ToolCall, len(providerCalls))
	for i, call := range providerCalls {
		arguments := json.RawMessage(call.Function.Arguments)
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		runtimeCalls[i] = tools.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: arguments,
		}
	}

	var plan []tools.ExecutionStep
	if executor != nil {
		plan = executor.BuildExecutionPlan(runtimeCalls)
	}
	return chatToolExecution{
		ProviderCalls: providerCalls,
		RuntimeCalls:  runtimeCalls,
		Plan:          plan,
	}
}

// genericRuntimeEligible reports whether a complete provider tool-call round can
// execute through the generic ordered runtime. A single browser-managed call
// keeps the entire round sequential so result ordering and browser state remain
// governed by the existing handler implementation.
func (e chatToolExecution) genericRuntimeEligible() bool {
	if len(e.RuntimeCalls) == 0 {
		return false
	}
	for _, call := range e.RuntimeCalls {
		if strings.HasPrefix(call.Name, "browser_") {
			return false
		}
	}
	return true
}

// executeGenericChatToolRound executes an eligible round through the reviewed
// policy-aware planner/runtime introduced by the parallel orchestration work.
// The returned result slice has the same order and cardinality as calls.
func executeGenericChatToolRound(ctx context.Context, executor *tools.Executor, calls []llm.ToolCall) ([]*tools.ToolResult, bool) {
	if executor == nil {
		return nil, false
	}
	execution := newChatToolExecution(executor, calls)
	if !execution.genericRuntimeEligible() {
		return nil, false
	}
	return executor.ExecutePlan(ctx, execution.Plan), true
}
