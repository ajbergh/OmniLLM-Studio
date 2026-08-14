> **Archived — completed.** Caller-known execution references and exact cancellation merged in PR #155. Durable task scheduling remains separately tracked in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Sandbox Execution Cancellation Contract — August 2026

Issue #151 makes protocol-v2 explicit cancellation addressable without changing `Exec` into an asynchronous task API.

## Caller-known execution references

Application code that may need to cancel a synchronous execution can allocate a canonical execution reference before starting the call:

```go
executionID := sandbox.NewExecutionID()
request.ExecutionID = executionID
```

The same reference is carried through Broker, local/HTTP runtime, and `sandboxd`. A concurrent control path can then call `Cancel` with that ID while `Exec` remains blocked.

Callers that do not need explicit in-flight cancellation may omit the field. Broker/local runtimes continue to allocate a canonical ID automatically for compatibility.

## Validation and lifecycle rules

- Execution IDs use the canonical `exec_<uuid>` form.
- Whitespace-padded and malformed IDs are rejected.
- A caller-supplied ID is preserved end to end.
- A runtime result with a different ID is rejected by Broker; HTTP runtime also rejects a mismatched supplied ID.
- Duplicate **active** IDs fail closed and do not overwrite the original execution's cancellation entry.
- Cancellation is owner/session scoped by Broker before it reaches the runtime.
- Once an execution finishes and unregisters, that execution ID is no longer cancellable.
- Context cancellation and session `Destroy` remain valid process-tree teardown mechanisms in addition to explicit execution-ID cancellation.

## Platform evidence

The regression suite covers Broker allocation/preservation, HTTP and `sandboxd` pass-through, Linux local-runtime cancellation, and Windows AppContainer cancellation. The Windows-native test starts a real AppContainer execution with a pre-known ID, rejects a duplicate active ID, cancels the original execution while it is running, and verifies the finished ID is no longer registered.

This change does not add durable task scheduling, pause/resume, or asynchronous execution status APIs; those remain separate roadmap work.
