# Chat Tool Parallel Orchestration — 2026-07-21

## Purpose

This branch continues the Chat Studio tool reliability program after PRs #32, #33, and #34. The current slice defines the execution-planning contract required before Chat Studio can safely execute multiple model-requested tools concurrently.

## Safety contract

Tool calls remain in model order. The planner may combine calls into a parallel execution step only when every contiguous call in that step is:

- registered;
- enabled;
- explicitly read-only;
- not side-effecting;
- explicitly marked `SupportsParallel`.

Unknown, disabled, side-effecting, or non-parallel tools are emitted as single sequential steps.

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

## Current implementation

`backend/internal/tools/execution_plan.go` introduces:

- `ExecutionStep`
- `BuildExecutionPlan(registry, calls)`

The function produces deterministic ordered steps and is independent from provider-specific tool-call wire formats. Provider calls are normalized before reaching this layer by the conformance work merged in PR #33.

## Test coverage

The branch covers:

- batching two contiguous parallel-safe reads;
- preserving a side-effect boundary between read batches;
- keeping unknown tools sequential;
- keeping read-only tools without `SupportsParallel` sequential;
- keeping a single parallel-safe tool as a sequential singleton because concurrency provides no benefit.

## Next integration step

The next commit should adapt the Chat Studio model-tool loop to execute `ExecutionStep` values:

- use `Executor.ExecuteBatch` only for `Parallel == true` steps;
- execute singleton steps through `Executor.Execute`;
- append tool results back to model history in original call order regardless of completion order;
- propagate request cancellation into the complete batch;
- emit individual lifecycle events for every tool call;
- preserve result-context limits and browser navigation limits.

GitHub Actions remain temporarily non-blocking due to the repository-owner budget suspension.