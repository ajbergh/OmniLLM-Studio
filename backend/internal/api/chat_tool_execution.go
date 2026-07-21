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
// use the generic ordered runtime. A single browser-managed call keeps the entire
// round on the existing sequential handler path so browser state and provider
// result order continue to be governed by one implementation.
func (e chatToolExecution) genericRuntimeEligible() bool {
	if len(e.RuntimeCalls) == 0 || len(e.Plan) == 0 {
		return false
	}
	for _, call := range e.RuntimeCalls {
		if strings.HasPrefix(call.Name, "browser_") {
			return false
		}
	}
	return true
}

// executeChatToolStep executes exactly one planned step. Chat Studio must apply
// its per-turn result-context budget before starting the next step. This retains
// the existing behavior that later sequential or side-effecting calls do not run
// after the result limit is reached, while planner-approved read-only calls in a
// single parallel step may execute concurrently.
func executeChatToolStep(ctx context.Context, executor *tools.Executor, step tools.ExecutionStep) []*tools.ToolResult {
	if executor == nil || len(step.Calls) == 0 {
		return nil
	}
	return executor.ExecutePlan(ctx, []tools.ExecutionStep{step})
}
