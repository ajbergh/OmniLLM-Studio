> **Archived — completed corrective integration.** Retained for SSE ordering and browser fallback rationale.

# Chat Tool Handler Parallel Integration — 2026-07-21

## Status

The Chat Studio handler migration is complete as of the corrective PR created on 2026-08-02.

PR #36 originally merged the provider/runtime adapter and result-processing helpers without wiring them into `backend/internal/api/message_handler.go`. The corrective implementation activates those helpers in the streaming Chat Studio tool loop and adds a regression test that fails if the callsite is removed.

## Provider-to-runtime adapter

`backend/internal/api/chat_tool_execution.go` converts provider-normalized `llm.ToolCall` values into `tools.ToolCall` values while preserving:

- provider order;
- tool-call IDs;
- tool names;
- argument JSON;
- empty-argument normalization to `{}`.

It builds the plan through `Executor.BuildExecutionPlan`, so effective `allow`, `ask`, and `deny` policy remains part of the parallel-safety decision.

## Active handler integration

`backend/internal/api/message_handler.go` now creates a `chatToolExecution` for each provider tool-call round.

Complete non-browser rounds use the ordered runtime when `genericRuntimeEligible()` returns true. The handler attaches the same execution context used by the legacy path:

- provider type;
- progress callback;
- user, workspace, conversation, and message invocation scope;
- inline approval broker;
- lifecycle-event SSE sink.

The handler emits one `tool_result` event and appends one provider `role=tool` message for every call, preserving the model's original call order even when a read-only step executes concurrently.

## Parallel SSE safety

Planner-approved parallel steps execute tool workers concurrently. `http.ResponseWriter` and `http.Flusher` are not safe for concurrent writes, so `backend/internal/api/chat_tool_sse.go` provides a request-local serialized lifecycle-event sink.

The sink holds one mutex across the complete `event:`/`data:` frame and flush operation. This prevents concurrent queued, started, progress, completed, failed, timed-out, or cancelled events from interleaving and corrupting the Chat Studio stream. The legacy browser-aware sequential path remains unchanged.

## Browser-managed fallback

Any round containing a `browser_*` call remains on the existing sequential handler path.

This is intentional because the Chat handler owns browser-specific state that is not part of the generic executor contract:

- navigation-result caching;
- visited-URL tracking;
- the per-turn navigation count;
- browser progress events;
- browser-specific result sanitization;
- navigated URL metadata.

Keeping the complete mixed round sequential avoids splitting one provider tool-call list across two state owners and preserves exact result order.

## Stepwise execution and result budget

`backend/internal/api/chat_tool_round.go` executes one `tools.ExecutionStep` at a time rather than executing the complete plan in advance.

This preserves the existing result-context behavior:

1. execute one sequential or planner-approved parallel step;
2. sanitize and append one result for every call in that step;
3. apply the per-turn result-context budget;
4. stop before beginning the next step when the budget is exhausted;
5. emit explicit `TOOL_RESULT_LIMIT` results and provider tool messages for every unstarted call in later steps.

Parallel steps contain only effective-policy `allow`, read-only, non-side-effecting tools. Multiple calls within one approved parallel step may already be running when one result exhausts the context budget, but no later sequential or side-effecting step begins.

## Result processing

`backend/internal/api/chat_tool_step_results.go` centralizes generic result handling:

- user-visible results use `safeToolResultForMetadata`;
- raw tool output remains available to the provider model for recovery and reasoning;
- the existing truncation suffix is preserved exactly;
- all call IDs receive exactly one provider `role=tool` message;
- calls skipped after the budget limit receive `TOOL_RESULT_LIMIT` metadata;
- skipped calls are never executed.

## Test coverage

Coverage now includes:

- provider-to-runtime order and argument preservation;
- valid tool-schema and argument JSON fixtures;
- empty argument normalization;
- browser-managed round fallback;
- policy-aware parallel plan boundaries;
- ordered results from a concurrent read-only step;
- result-context truncation;
- prevention of a later side effect after the budget is exhausted;
- all calls skipped when the budget is already exhausted;
- safe user-visible error metadata;
- one result for missing executor output;
- one `TOOL_RESULT_LIMIT` result per unstarted call;
- source-level verification that `message_handler.go` invokes the ordered runtime and retains the browser fallback;
- concurrent lifecycle-event writes producing complete, independently decodable SSE frames.

## Validation

The corrective branch was formatted with `gofmt` and passed:

```text
go test ./internal/tools ./internal/api
go test ./...
```

The full backend test used a temporary `cmd/desktop/frontend_dist/index.html` fixture to satisfy the Wails embed directive on the clean runner. The fixture was not committed.

Normal repository Actions status gates remain non-blocking while the repository-owner budget suspension is in effect.
