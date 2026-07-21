# Chat Tool Parallel Orchestration — 2026-07-21

## Purpose

This branch continues the Chat Studio tool reliability program after PRs #32, #33, and #34. It establishes the provider-neutral planning and execution runtime required before the large Chat Studio model/tool loop can safely adopt concurrent tool execution.

## Safety contract

Tool calls remain in model order. The planner may combine calls into a parallel execution step only when every contiguous call in that step is:

- registered;
- enabled;
- explicitly read-only;
- not side-effecting;
- explicitly marked `SupportsParallel`;
- currently permitted with effective policy `allow`.

Unknown, disabled, side-effecting, approval-gated (`ask`), denied, or non-parallel tools are emitted as single sequential steps.

Most importantly, the planner never moves a read-only call across a side-effecting or otherwise sequential call. For example:

```text
read A, read B, write C, read D, read E
```

becomes:

```text
parallel(read A, read B)
sequential(write C)
parallel(read D, read E)
```

It does not become one global read batch followed by writes, because that would alter the model's requested execution semantics.

## Implemented runtime

`backend/internal/tools/execution_plan.go` provides:

- `ExecutionStep`;
- `BuildExecutionPlan(registry, calls)` for definition-only planning;
- `Executor.BuildExecutionPlan(calls)` for runtime policy-aware planning.

`backend/internal/tools/execution_run.go` provides:

- `Executor.ExecutePlan(ctx, steps)`;
- ordered result collection across sequential and parallel steps;
- `Executor.ExecuteBatch` use only for planner-approved parallel steps;
- executor-side revalidation of every caller-supplied parallel step;
- current-policy revalidation immediately before batching;
- automatic sequential fallback when a step contains an unknown, disabled, side-effecting, approval-gated, denied, or non-parallel tool;
- pre-step cancellation checks that prevent pending side-effecting tools from starting after the request is cancelled;
- one terminal result per skipped tool call ID.

Provider calls are normalized before reaching this layer by the conformance work merged in PR #33.

## Cancellation lifecycle

A failed terminal event emitted from a cancelled request context is normalized to `tool_cancelled`. A parent deadline is normalized to `tool_timed_out`.

The normalized event is used consistently by:

- live request event sinks and Chat Studio SSE;
- runtime tool metrics;
- the durable `tool_invocations` audit recorder.

Cancelled invocations now receive a durable completion timestamp rather than remaining in an apparently in-progress state.

## Test coverage

The branch covers:

- batching two contiguous parallel-safe reads;
- preserving a side-effect boundary between read batches;
- keeping unknown tools sequential;
- keeping read-only tools without `SupportsParallel` sequential;
- keeping approval-gated reads sequential;
- keeping a single parallel-safe tool as a sequential singleton;
- proving two planned reads actually begin concurrently;
- preserving result order even when execution is concurrent;
- preventing a side-effecting tool from starting after parent cancellation;
- emitting `tool_cancelled` for the skipped call;
- rejecting unsafe caller-supplied parallel steps at execution time.

## Deliberately deferred Chat handler migration

The existing `backend/internal/api/message_handler.go` loop still invokes each tool sequentially. Migrating that large stateful loop should be a separate focused change using this runtime API rather than embedding new concurrency logic directly in the handler.

That migration must preserve:

- browser navigation caching and per-turn navigation limits;
- inline approval pauses;
- per-tool lifecycle SSE events;
- provider-normalized tool-call ordering;
- one tool result for every call ID;
- result-context truncation and final-answer fallback;
- cancellation of the complete request batch.

GitHub Actions remain temporarily non-blocking due to the repository-owner budget suspension.