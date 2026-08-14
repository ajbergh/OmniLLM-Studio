> **Archived — superseded.** This pre-Phase-12 design snapshot is retained for historical rationale. Use [the current sandbox architecture](../../AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md) and [MASTER_PLAN.md](../../MASTER_PLAN.md) for current implementation and outstanding work.

# Agent Sandbox Architecture

## Purpose

This document defines the intended execution architecture for untrusted or model-directed local processes in OmniLLM-Studio. It is the implementation companion to `AGENT_SANDBOX_ROADMAP_2026-08.md` and `AGENT_SANDBOX_THREAT_MODEL.md`.

The architecture preserves the current tool Executor as the authorization/control plane while introducing a sandbox Broker and sandbox runtime as the execution/data plane.

## Current-state boundary

The existing runtime has several separate execution models:

- `code_execute` calls an operator-configured external `/v1/execute` service;
- `python_analysis` launches a restricted Python subprocess in a temporary directory;
- local plugins launch ordinary child processes;
- stdio MCP launches ordinary child processes;
- headless Chromium has its own strong specialized sandbox/session/SSRF controls;
- guarded Git tools use application-owned repository IDs, path containment, explicit mutation gates, and reviewed state digests.

The target state converges arbitrary subprocess execution without weakening the specialized controls that already exist.

## Component model

```text
                    +-------------------------+
                    | Chat / Agent / Task     |
                    +------------+------------+
                                 |
                                 v
                    +-------------------------+
                    | Tool Executor           |
                    | policy / approval /     |
                    | scope / events / limits |
                    +------------+------------+
                                 |
                                 v
                    +-------------------------+
                    | Sandbox Broker          |
                    | ownership / capability  |
                    | workspace / network /   |
                    | credentials / artifacts |
                    +------------+------------+
                                 |
                     authenticated control IPC
                                 |
                                 v
                    +-------------------------+
                    | Sandbox Runtime         |
                    | platform confinement    |
                    | process tree / limits   |
                    +------------+------------+
                                 |
                                 v
                 arbitrary or extension subprocesses
```

### Tool Executor

The existing `internal/tools.Executor` remains authoritative for:

- tool availability;
- global/scoped Allow / Ask / Deny policy;
- request/per-turn restrictions;
- approval;
- argument validation;
- timeout selection;
- lifecycle events;
- result limits;
- idempotency for side effects.

The sandbox does not replace this policy layer. It enforces technical containment after authorization.

### Sandbox Broker

Proposed package:

```text
backend/internal/sandbox/
```

Responsibilities:

- create application-owned sandbox sessions;
- bind sessions to authenticated invocation scope;
- resolve opaque workspace IDs to operator/application-approved roots;
- derive mount access mode from policy;
- derive network policy from hard deployment policy plus request approval;
- derive resource limits;
- create a sanitized environment;
- broker credentials/capabilities without exposing raw secrets;
- validate/promote artifacts;
- persist/emit sandbox lifecycle metadata;
- route execution to the selected runtime.

The Broker must not directly use `os/exec` for arbitrary model-controlled programs.

### Sandbox Runtime

A runtime executes one sandbox policy using platform controls.

Conceptual interface:

```go
type Runtime interface {
    Create(context.Context, Spec) (Session, error)
    Exec(context.Context, SessionID, ExecRequest) (ExecResult, error)
    Cancel(context.Context, SessionID, ExecutionID) error
    Status(context.Context, SessionID) (Status, error)
    Destroy(context.Context, SessionID) error
}
```

Runtime implementations may be:

- first-party local Linux/Windows/macOS desktop backends;
- container/gVisor/Kata/microVM workers for server deployments;
- authenticated remote workers that implement the same protocol.

## Policy model

### Ownership

Every sandbox is bound to:

```text
user_id
workspace_id (optional for pure scratch workloads)
conversation_id (optional)
message_id (optional)
agent_run_id (optional)
task_id (optional)
```

A sandbox ID alone is never authorization.

### Workspace access

Models receive opaque workspace IDs. The backend maps those IDs to physical roots.

Access modes:

```text
read_only
read_write_no_delete
read_write
```

Scratch space is distinct from mounted user/project workspace and may always be ephemeral read/write within quota.

`read_write_no_delete` must reject explicit delete and replacement patterns that amount to delete/recreate when the runtime can detect them. Application-level workspace tools additionally enforce operation semantics and maintain a journal.

### Network

Network modes:

```text
none
allowlist
approval_required
```

The Broker intersects requested access with deployment hard-deny rules. User approval can narrow or temporarily widen within operator-permitted bounds but cannot bypass a deployment hard deny.

Destination validation must cover hostname, resolved address, port/protocol, private/loopback/link-local/metadata/reserved ranges, redirects where applicable, and rebinding-resistant connection behavior.

### Environment

Default environment is constructed, not inherited.

Safe baseline examples:

```text
LANG / LC_* (controlled)
PATH (runtime-owned executable roots)
HOME (sandbox-local)
TMPDIR / TEMP / TMP (sandbox-local)
TERM (when terminal semantics require it)
```

Application/provider secrets are excluded unless exposed through a dedicated broker mechanism.

### Resource limits

A `ResourceLimits` policy should include, where supported:

```text
wall_time
cpu_time
memory_bytes
pids/processes
disk_bytes
file_count
stdout_bytes
stderr_bytes
artifact_bytes
```

The runtime reports which requested controls are actually enforced. A production policy may require specific controls and fail closed if the selected runtime cannot provide them.

## Sandbox protocol v2

The v2 protocol represents lifecycle rather than a one-shot code blob.

Recommended operations:

```text
POST   /v2/sandboxes
POST   /v2/sandboxes/{id}/exec
POST   /v2/sandboxes/{id}/cancel
GET    /v2/sandboxes/{id}/status
GET    /v2/sandboxes/{id}/artifacts
GET    /v2/sandboxes/{id}/changes
DELETE /v2/sandboxes/{id}
```

### Create request

Application-derived fields include:

- owner scope token/identity;
- workspace mount descriptors by opaque ID;
- access mode;
- network policy;
- resource limits;
- runtime profile;
- environment allowlist/value set;
- TTL.

The model does not supply physical host paths, worker endpoints, runtime credentials, or privileged runtime flags.

### Exec request

Contains:

- runtime/language/argv or command representation;
- sandbox-relative working directory;
- stdin/input payload when applicable;
- per-exec timeout bounded by sandbox policy;
- optional expected workspace generation/journal version for state-sensitive operations.

### Result

Contains bounded:

- stdout;
- stderr;
- exit code;
- execution ID;
- timing;
- resource usage where available;
- enforcement metadata;
- artifact IDs;
- workspace change journal references.

Artifact results never rely on arbitrary worker-provided URLs as the trust boundary.

## Process runner migration

Before every platform sandbox is complete, local subprocess creation should converge behind a runner interface.

Proposed package responsibility:

```go
type ProcessRunner interface {
    Start(context.Context, ProcessSpec) (Process, error)
}
```

Initial uses:

- stdio MCP;
- plugins;
- restricted Python.

The default runner must construct a sanitized environment. A sandbox-backed runner then replaces direct process launch without changing higher-level MCP/plugin lifecycle logic.

## Plugin and MCP capability manifests

Local extensions may request capabilities such as:

```yaml
permissions:
  filesystem:
    - workspace: plugin_data
      mode: read_write
  network:
    mode: allowlist
    domains:
      - api.example.com
  environment:
    keys:
      - EXAMPLE_NON_SECRET_SETTING
```

Requests are declarative. Effective access is the intersection of manifest request, operator configuration, scoped user policy, and runtime capability.

MCP server configuration should similarly distinguish configured variables from inherited environment; ambient inheritance is not part of the contract.

## Workspace change journal

Application-level workspace write tools record deterministic mutation metadata. Recommended record fields:

```text
id
workspace_id
user_id
conversation_id
agent_run_id
task_id
sandbox_id
execution_id
relative_path
operation
before_exists
before_sha256
after_exists
after_sha256
created_at
```

Optional stored before/after content or reverse patches may be used for bounded revert when size/type rules allow it.

The journal is not a replacement for Git. Git repositories continue to use the existing status/diff/index/worktree digest protections for stage/commit/publication.

## Git workflow integration

The sandbox may modify a repository worktree when the user granted workspace write access. It should generally not receive hosted Git credentials.

Preferred workflow:

```text
sandbox: edit + build + test
        |
        v
Omni Git read tools: status/diff
        |
        v
approval
        |
        v
Omni guarded Git mutations: stage/commit
        |
        v
Omni guarded remote/GitHub tools: push/PR/review
```

This keeps authentication and stale-state binding outside arbitrary shell execution.

## Desktop and server profiles

### Desktop local

Goals:

- no dependency on Docker for normal use;
- OS-native enforcement;
- low startup overhead;
- explicit workspace folder grants;
- network off by default;
- clear user-visible sandbox status.

### Headless/server

Goals:

- no arbitrary tenant execution in primary API process/container;
- dedicated worker identity;
- authenticated Broker-to-worker channel;
- tenant-bound ephemeral workspace storage;
- strict quotas;
- default-deny network policy;
- worker recycling after task/session boundaries.

### Kubernetes

Recommended worker security baseline:

- non-root UID/GID;
- read-only root filesystem where feasible;
- `allowPrivilegeEscalation: false`;
- dropped Linux capabilities;
- seccomp runtime default or stricter profile;
- resource requests/limits plus PID control where supported;
- ephemeral volumes;
- NetworkPolicy;
- no arbitrary `hostPath` mounts in multi-user mode.

## Platform enforcement

### Linux

Preferred first implementation because server deployments and CI can validate it most directly.

Evaluate established confinement helpers rather than hand-rolling every namespace operation. Required properties include mount/user/PID/network isolation as available, no-new-privileges, explicit bind mounts, read-only runtime, cgroup-based quotas where available, seccomp, and process-tree teardown.

### Windows

Required properties:

- restricted process identity/token;
- Job Object lifetime/process-tree control;
- controlled inheritable handles;
- explicit environment;
- ACL-scoped workspace/scratch directories;
- network restriction strategy appropriate to packaged desktop execution;
- cleanup of descendant processes and temporary workspace state.

### macOS

Required properties:

- OS-enforced process/file/network confinement suitable for distributed desktop builds;
- explicit workspace grants;
- child-process inheritance;
- controlled environment;
- deterministic cleanup.

## Observability

Sandbox lifecycle and execution should emit structured events compatible with existing tool/agent event pipelines:

```text
sandbox_created
sandbox_exec_started
sandbox_exec_progress
sandbox_network_approval_required
sandbox_exec_completed
sandbox_exec_failed
sandbox_cancelled
sandbox_destroyed
workspace_changed
artifact_promoted
```

Do not expose raw host paths, secrets, or unsafe command diagnostics to ordinary client logs.

## Compatibility and fail-closed policy

Configuration should distinguish:

- sandbox optional (developer compatibility mode);
- sandbox required for arbitrary code;
- sandbox required for local extensions;
- sandbox required globally for all eligible subprocesses.

If a deployment requires sandboxing and the runtime cannot satisfy the required enforcement capabilities, the affected tool/extension must remain disabled rather than silently falling back to unrestricted host execution.

## Testing requirements

The runtime contract requires unit tests for ownership/policy composition and platform-native integration tests for actual isolation. Cross-compiling a platform backend is not evidence that containment works on that platform.

See `AGENT_SANDBOX_THREAT_MODEL.md` for adversarial acceptance cases.
