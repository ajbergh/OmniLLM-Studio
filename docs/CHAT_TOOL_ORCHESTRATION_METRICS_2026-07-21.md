# Chat Tool Orchestration, Cancellation, and Metrics — 2026-07-21

## Scope

This branch continues the Chat Studio reliability work merged through PRs #32 and #33. It focuses on two operational gaps that remain after provider conformance and replay safety:

1. cancellation must never be treated as a transient retryable failure;
2. operators and users need privacy-safe runtime visibility into tool success, failure, timeout, cancellation, retry, and latency behavior.

## Cancellation and retry semantics

`tools.IsRetryableExecutionError` now treats both `context.Canceled` and `context.DeadlineExceeded` as terminal for the current invocation, including wrapped errors. This prevents a canceled read-only tool from being replayed by the executor merely because an underlying transport also implements retry semantics.

The existing executor already derives each tool execution context from the parent request context with a bounded timeout. As a result, cancellation of the Chat Studio request propagates into the tool implementation. The new reliability tests verify that a canceled read-only call executes once and is not retried.

A `tool_cancelled` lifecycle event type is also defined for explicit cancellation reporting as orchestration paths adopt that terminal classification.

## Runtime metrics

Tool lifecycle events feed an in-process aggregate collector. The collector records only privacy-safe operational data:

- call count;
- success count;
- failure count;
- timeout count;
- cancellation count;
- retry count;
- total execution duration;
- most recent execution duration;
- most recent terminal event type.

Arguments, result content, artifacts, and user identifiers are not exposed in metric summaries.

The durable `tool_invocations` audit table remains the source for historical event-level records. The new runtime metrics intentionally reset when the process restarts and are intended for live diagnostics rather than durable analytics.

## Multi-user isolation

Runtime aggregates are internally keyed by authenticated user scope plus tool name. The authenticated tool endpoint exposes only `ToolMetricsSnapshotForUser(userID)` rather than the process-wide snapshot.

Request:

```json
{
  "action": "metrics"
}
```

Response shape:

```json
{
  "scope": "user",
  "tools": [
    {
      "tool_name": "calculator",
      "calls": 8,
      "successes": 7,
      "failures": 0,
      "timeouts": 1,
      "cancellations": 0,
      "retries": 1,
      "total_duration_ms": 94,
      "last_duration_ms": 30,
      "last_event": "tool_timed_out"
    }
  ]
}
```

Solo mode naturally uses the empty user scope and therefore continues to return the local process user's metrics.

## Validation coverage added

The branch adds tests for:

- retryable transient failures still retrying once for read-only tools;
- plain permanent failures remaining non-retryable;
- direct and wrapped cancellation remaining non-retryable;
- deadline errors remaining non-retryable;
- canceled read-only tool calls executing only once;
- metrics aggregation across success, timeout, retry, and failure events;
- explicit cancellation metric accounting;
- user-scoped metric isolation.

## Follow-on work

The next orchestration slice should wire explicit `tool_cancelled` terminal events into executor failure classification, add realistic parallel-tool versus ordered-side-effect orchestration fixtures, and evaluate whether live runtime metrics should also be surfaced in the Chat Studio diagnostics UI.
## Tool diagnostics UI — 2026-08-10

The live metrics and durable audit records are now surfaced in **Settings → Tools → Tool Diagnostics**. The authenticated endpoint combines the existing per-user in-process metric snapshot with a bounded newest-first view of `tool_invocations`.

The diagnostics contract is deliberately privacy-safe: it exposes tool name, lifecycle status, approval status, latency, result byte count, retry count, and timestamps, but never returns stored arguments, result bodies, error text, or user identifiers. Durable queries are constrained to the authenticated user scope; solo mode uses the stable local owner ID.

This closes the diagnostics-UI follow-on identified in the original orchestration metrics work. Future observability work should build on this endpoint rather than exposing the payload-bearing audit columns.
