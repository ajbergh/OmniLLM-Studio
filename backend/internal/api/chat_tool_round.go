package api

import (
	"context"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// chatToolRoundOutcome contains the ordered, provider-facing results produced by
// one complete generic tool-call round. Processed preserves the model's original
// call order even when a planner-approved step executes concurrently.
type chatToolRoundOutcome struct {
	Processed    []chatToolStepResult
	UsedChars    int
	LimitReached bool
}

// executeGenericChatToolRound executes a policy-aware plan one step at a time.
// The result-context budget is checked between steps so a completed step may be
// recorded while later sequential or side-effecting steps remain unexecuted.
// Every unstarted call still receives one TOOL_RESULT_LIMIT result/message so the
// provider history remains protocol-complete.
func executeGenericChatToolRound(
	ctx context.Context,
	executor *tools.Executor,
	plan []tools.ExecutionStep,
	usedChars int,
	maxChars int,
) chatToolRoundOutcome {
	outcome := chatToolRoundOutcome{UsedChars: usedChars}
	if len(plan) == 0 {
		return outcome
	}

	if maxChars <= usedChars {
		outcome.LimitReached = true
		for _, step := range plan {
			outcome.Processed = append(outcome.Processed, skippedChatToolResults(step.Calls)...)
		}
		return outcome
	}

	for stepIndex, step := range plan {
		results := executeChatToolStep(ctx, executor, step)
		processed, nextUsedChars, limited := processChatToolStepResults(
			step.Calls,
			results,
			outcome.UsedChars,
			maxChars,
		)
		outcome.Processed = append(outcome.Processed, processed...)
		outcome.UsedChars = nextUsedChars
		outcome.LimitReached = limited

		if !limited {
			continue
		}

		for _, remainingStep := range plan[stepIndex+1:] {
			outcome.Processed = append(outcome.Processed, skippedChatToolResults(remainingStep.Calls)...)
		}
		break
	}

	return outcome
}
