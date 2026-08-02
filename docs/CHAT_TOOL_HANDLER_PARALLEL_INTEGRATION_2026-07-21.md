# Chat Tool Handler Parallel Integration — 2026-07-21

## Purpose

This branch begins the focused migration of `backend/internal/api/message_handler.go` from its current per-call sequential loop to the ordered, policy-aware execution runtime merged in PR #35.

The first change deliberately extracts and tests the integration contract before modifying the large stateful handler.

## Provider-to-runtime adapter

`backend/internal/api/chat_tool_execution.go` converts provider-normalized `llm.ToolCall` values into `tools.ToolCall` values while preserving:

- provider order;
- tool-call IDs;
- tool names;
- argument JSON;
- empty-argument normalization to `{}`.

It builds the plan through `Executor.BuildExecutionPlan`, so effective `allow`, `ask`, and `deny` policy remains part of the parallel-safety decision.

## Browser-managed fallback

Any round containing a `browser_*` call remains on the existing sequential handler path.

This is intentional because the current Chat handler owns browser-specific state that is not part of the generic executor contract:

- navigation-result caching;
- visited-URL tracking;
- the per-turn navigation count;
- browser progress events;
- browser-specific result sanitization;
- navigated URL metadata.

Keeping the complete mixed round sequential avoids splitting one provider tool-call list across two state owners and preserves exact result order.

## Stepwise execution and result budget

The adapter executes one `tools.ExecutionStep` at a time rather than executing the complete plan in advance.

This preserves the existing result-context behavior:

1. execute one sequential or planner-approved parallel step;
2. sanitize and append one result for every call in that step;
3. apply the per-turn result-context budget;
4. stop before beginning the next step when the budget is exhausted;
5. emit explicit `TOOL_RESULT_LIMIT` results and provider tool messages for every unstarted call in later steps.

Parallel steps contain only effective-policy `allow`, read-only, non-side-effecting tools. Therefore, multiple calls within a single step may already be running when one result exhausts the context budget, but no later sequential or side-effecting step begins.

## Result processing

`backend/internal/api/chat_tool_step_results.go` centralizes generic result handling:

- user-visible results use `safeToolResultForMetadata`;
- raw tool output remains available to the provider model for recovery and reasoning;
- the existing truncation suffix is preserved exactly;
- all call IDs receive exactly one provider `role=tool` message;
- calls skipped after the budget limit receive `TOOL_RESULT_LIMIT` metadata;
- skipped calls are never executed.

## Test coverage

The branch covers:

- provider-to-runtime order and argument preservation;
- empty argument normalization;
- browser-managed round fallback;
- policy-aware parallel plan boundaries;
- ordered results from one parallel step;
- empty/nil execution handling;
- result-order preservation;
- result-context truncation;
- later calls in a completed parallel step receiving a limit marker;
- safe user-visible error metadata;
- one result for missing executor output;
- one `TOOL_RESULT_LIMIT` result per unstarted call.

## Remaining call-site migration

The final handler edit should replace only the generic non-browser execution portion of the loop:

```text
execution := newChatToolExecution(h.toolExecutor, finalToolCalls)
if execution.genericRuntimeEligible() {
    for each ordered step:
        executeChatToolStep(...)
        processChatToolStepResults(...)
        emit tool_result SSE and append tool messages
        if limit reached:
            append skippedChatToolResults for all remaining steps
            stop
} else {
    retain the existing browser-aware sequential loop
}
```

The migration must continue attaching the same provider, invocation scope, inline approval, progress, and event-sink context before each step.

GitHub Actions remain temporarily non-blocking due to repository-owner budget constraints. This branch is being manually source-reviewed until automated validation can be restored.