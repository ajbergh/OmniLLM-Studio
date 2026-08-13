# Agent Sandbox Architecture

## Purpose

This document defines the execution architecture for untrusted or model-directed local processes in OmniLLM-Studio. It is the implementation companion to `AGENT_SANDBOX_ROADMAP_2026-08.md` and `AGENT_SANDBOX_THREAT_MODEL.md`.

The architecture preserves the tool Executor as the authorization/control plane while using the sandbox Broker and platform runtime as the execution/data plane. Specialized surfaces such as headless Chromium and guarded Git retain their own additional controls rather than being flattened into generic shell access.

## Current-state boundary — August 2026

The current implementation has converged the major arbitrary-process paths behind shared sandbox/confinement concepts:

- `code_execute` creates an owner-bound protocol-v2 Broker session and executes through the selected sandbox runtime;
- `python_analysis` keeps its restricted AST/builtin policy but routes execution through the Broker rather than falling back to unrestricted host Python;
- the first-party Linux runtime uses Bubblewrap/rootfs confinement;
- the first-party Windows runtime uses AppContainer plus creation-time Job Object/handle restrictions;
- local plugins and stdio MCP construct processes through the shared extension runner; Linux can use Bubblewrap and Windows uses native AppContainer/Job confinement when available;
- macOS native confinement is not yet implemented and remains Phase 13 work;
- headless Chromium keeps its own browser sandbox/session/SSRF controls;
- guarded Git/GitHub tools remain host-side and use application-owned repository/remote IDs, explicit mutation gates, approvals, and reviewed-state digests.

The architecture is still incomplete in several cross-platform areas: resource quotas, destination-enforced egress, broader workspace path-component TOCTOU assurance, service-specific credential consumers, worker packaging/deployment, macOS native confinement, and durable task scheduling. Issue #151 separately tracks the explicit execution-ID cancellation addressability gap in the synchronous protocol lifecycle.

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

Implemented package:

```text
backend/internal/sandbox/
```

Responsibilities include:

- create application-owned sandbox sessions;
- bind sessions to authenticated invocation scope;
- resolve opaque workspace IDs to application/operator-approved roots;
- derive mount access mode from policy;
- derive network policy from deployment hard rules plus approved grants;
- derive resource limits;
- construct/supply sanitized environment policy;
- broker credential references without exposing raw secrets;
- negotiate required runtime capabilities;
- persist/emit sandbox lifecycle metadata through higher-level callers;
- route execution to the selected runtime.

The Broker does not directly use `os/exec` for arbitrary model-controlled programs.

### Sandbox Runtime

A runtime executes one sandbox policy using platform controls. The implemented interface is conceptually:

```go
type Runtime interface {
    Capabilities() RuntimeCapabilities
    Create(context.Context, RuntimeCreateRequest) (runtimeID string, err error)
    Exec(context.Context, runtimeID string, request ExecRequest) (*ExecResult, error)
    Cancel(context.Context, runtimeID, executionID string) error
    Status(context.Context, runtimeID string) (*Status, error)
    Destroy(context.Context, runtimeID string) error
}
```

Implemented/target runtime classes include:

- first-party local Linux and Windows desktop/worker backends;
- future macOS native backend;
- future container/gVisor/Kata/microVM workers for server deployments;
- authenticated remote workers that implement the protocol-v2 contract.

Runtime capability flags are evidence-backed claims, not configuration wishes. If the Broker requires a control that a runtime cannot enforce, session creation fails closed.

## Policy model

### Ownership

Every sandbox is bound to application invocation scope including:

```text
user_id
workspace_id (optional for pure scratch workloads)
conversation_id (optional)
message_id (optional)
agent_run_id (optional)
task_id (optional)
```

A sandbox ID alone is never authorization. Broker operations revalidate the full owner scope and session TTL.

### Workspace access

Models receive opaque workspace IDs. The backend maps those IDs to physical roots.

Access modes are:

```text
read_only
read_write_no_delete
read_write
```

Scratch space is distinct from mounted user/project workspace and is ephemeral read/write inside the sandbox boundary.

Application-level workspace tools enforce mutation semantics and maintain a durable journal. Arbitrary shell/runtime access may narrow requested permissions where the OS primitive cannot safely provide the requested semantic. Examples:

- Linux `read_write_no_delete` is narrowed to read-only for arbitrary shell execution rather than approximated unsafely;
- current `terminal_exec` intentionally requests a read-only project mount;
- Windows arbitrary-process project input is staged read-only into AppContainer-owned storage; writable project mounts fail closed rather than widening host ACLs.

### Network

Network policy modes are:

```text
none
allowlist
approval_required
```

The Broker intersects requested access with deployment hard-deny rules. Approval can authorize within operator-permitted bounds but cannot bypass a hard deny.

Destination-scoped authorization and destination-scoped enforcement are intentionally separate. A runtime must advertise `network_allowlist=true` before an approved destination grant can be used. Current first-party Linux and Windows runtimes advertise no destination allowlist and remain no-network.

A future enforcing egress path must cover hostname, resolved address, port/protocol, private/loopback/link-local/metadata/reserved ranges, redirects where applicable, and rebinding-resistant connection behavior.

### Environment

Default sandbox environment is constructed, not inherited.

Safe baseline examples include controlled values for:

```text
LANG / LC_*
PATH (runtime-owned executable roots)
HOME (sandbox-local)
TMPDIR / TEMP / TMP (sandbox-local)
TERM (when terminal semantics require it)
```

Application/provider secrets are excluded unless exposed through a dedicated broker mechanism. Credential-bearing variables, credential-file pointers, SSH/Git authentication delegation, and proxy variables are rejected for arbitrary sandbox execution.

Persistent extension compatibility mode still accepts explicitly configured extension environment values while stripping ambient backend environment. Native extension confinement rejects credential-sensitive explicit values by default unless the narrow transitional operator override is enabled.

### Resource limits

`ResourceLimits` represents controls such as:

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

The runtime reports which requested controls are actually enforced. Today the first-party runtimes enforce wall/output bounds and process-tree/session lifetime controls, but do not advertise memory, CPU, PID-count, or physical disk quotas. Those remain explicit roadmap gaps rather than aspirational capability flags.

## Sandbox protocol v2

Protocol v2 represents lifecycle rather than a one-shot code blob.

The current authenticated worker operations are:

```text
GET    /v2/capabilities
POST   /v2/sandboxes
POST   /v2/sandboxes/{runtime_id}/exec
POST   /v2/sandboxes/{runtime_id}/cancel
GET    /v2/sandboxes/{runtime_id}/status
DELETE /v2/sandboxes/{runtime_id}
```

Artifact promotion/change inspection remain application-level responsibilities and future worker extensions rather than arbitrary worker URLs.

### Create request

Application-derived fields include:

- owner scope identity;
- workspace mount descriptors by opaque ID;
- access mode;
- network policy;
- resource limits;
- runtime profile;
- environment allowlist/value set;
- TTL;
- required enforcement capabilities.

The model does not supply physical host paths, worker endpoints, runtime credentials, or privileged runtime flags.

### Exec request

Contains:

- language/code or explicit executable/argv representation;
- sandbox-relative working directory;
- stdin/input payload when applicable;
- per-exec timeout bounded by sandbox/session policy;
- explicit environment values subject to sandbox environment validation.

### Exec result

Contains bounded:

- stdout;
- stderr;
- exit code;
- execution ID;
- timing;
- resource/enforcement metadata where available;
- artifact metadata where supported.

Artifact results never rely on arbitrary worker-provided URLs as the trust boundary.

### Explicit cancellation gap — issue #151

Context cancellation, timeout, session destruction, and platform process-tree teardown work. The separate execution-ID cancellation contract is not fully addressable by a synchronous caller today: Linux/Windows runtimes generate the execution ID inside `Exec`, while the caller receives it only in the completed result. The `/cancel` operation requires that ID.

Issue #151 tracks the preferred small repair: make a canonical execution reference caller-known at start time, preserve it across Broker/HTTP/worker boundaries, reject malformed or duplicate active IDs, and prove cancellation of an active execution without weakening owner checks.

This lifecycle defect does not invalidate the OS confinement properties proved by Windows Phase 12.

## Process runner migration

Persistent extension process creation has converged behind `backend/internal/sandbox` process-construction/lifecycle seams.

Current uses include:

- stdio MCP;
- local plugins;
- platform extension confinement adapters.

The host compatibility runner constructs a sanitized environment. Linux and Windows native extension runners apply OS confinement without rewriting higher-level JSON-RPC streaming logic.

Restricted Python analysis no longer needs a direct host subprocess runner for model execution; it uses the Broker/runtime path and fails closed when the selected runtime cannot provide a confined interpreter.

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

MCP server configuration similarly distinguishes configured variables from inherited environment; ambient inheritance is not part of the contract.

## Workspace change journal

Application-level workspace write tools record deterministic mutation metadata including:

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

Optional bounded before/after content or reverse patches may support revert when size/type rules permit it.

The journal is not a replacement for Git. Git repositories continue to use existing status/diff/index/worktree digest protections for stage/commit/publication.

## Git workflow integration

Arbitrary sandboxes generally should not receive hosted Git credentials. The preferred workflow remains:

```text
sandbox / workspace tools: edit + build + test
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
Omni guarded remote/GitHub tools: push/PR/review/merge
```

This keeps authentication, authorization, and stale-state binding outside arbitrary shell execution.

## Desktop and server profiles

### Desktop local

Goals/properties:

- no Docker dependency for normal local use;
- OS-native enforcement where implemented;
- low startup overhead;
- explicit workspace folder grants;
- network off by default;
- user-visible sandbox/runtime capability status.

Linux and Windows now have first-party local confinement implementations. macOS remains pending Phase 13.

### Headless/server

Target properties:

- no arbitrary tenant execution in primary API process/container;
- dedicated worker identity;
- authenticated Broker-to-worker channel;
- tenant-bound ephemeral workspace storage;
- strict quotas;
- default-deny network policy;
- worker recycling after task/session boundaries.

This remains Phase 15/deployment work; do not use the API container as an arbitrary execution shortcut.

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

### Linux — implemented, broader limits open

The first-party runtime uses Bubblewrap with an operator-configured read-only rootfs, explicit trusted mounts, private namespaces, no network, process-tree/session cleanup, and bounded wall/output behavior. Persistent extension confinement can use the same rootfs boundary.

Still open: packaged worker distribution, memory/CPU/PID/disk quotas, and destination-enforced egress.

### Windows — native confinement complete

Windows native confinement Phase 12 is complete and includes:

- AppContainer restricted identity with unique per-session/per-extension package SIDs;
- creation-time kill-on-close Job Object process-tree control;
- explicit inherited stdio handles only;
- constructed/minimal environment;
- read-only staged project/extension input rather than host ACL widening;
- post-open canonical source-handle validation plus hard-link/reparse rejection;
- zero network capabilities/default-deny loopback evidence;
- bounded wall/output behavior;
- retryable temporary AppContainer profile cleanup;
- adversarial cross-profile, host-file, secret, network, path-shape, policy-mode, and Job teardown evidence.

The Windows runtime does not advertise memory/CPU/PID/disk quotas or destination allowlisting. Python/JavaScript shortcuts for arbitrary code execution remain fail-closed until a confined interpreter/package layout is natively validated. Issue #151 remains a protocol lifecycle gap outside the Windows confinement completion claim.

### macOS — pending Phase 13

Required properties include:

- OS-enforced process/file/network confinement suitable for distributed desktop builds;
- explicit workspace grants;
- child-process inheritance;
- controlled environment;
- deterministic cleanup;
- native behavior evidence rather than cross-compilation-only confidence.

Until this lands, macOS extension `auto` mode retains the sanitized host boundary and `required` fails closed.

## Observability

Sandbox lifecycle/execution should emit structured events compatible with existing tool/agent event pipelines, such as:

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

Configuration distinguishes compatibility from required confinement. If a deployment requires sandboxing and the selected runtime cannot satisfy required enforcement capabilities, the affected tool/extension remains disabled rather than silently falling back to unrestricted host execution.

Persistent extensions use `OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off`. `off` is an explicit operator compatibility choice, not a runtime failure fallback.

## Testing requirements

The runtime contract requires unit tests for ownership/policy composition and platform-native integration/adversarial tests for actual isolation. Cross-compiling a platform backend is not evidence that containment works on that platform.

Windows Phase 12 establishes the pattern: native tests must prove identity, filesystem, network, process-tree, environment, staging/path-shape, cleanup, and policy-mode behavior before declaring a platform confinement phase complete.

See `AGENT_SANDBOX_THREAT_MODEL.md` for the continuing adversarial acceptance categories and `AGENT_SANDBOX_ROADMAP_2026-08.md` for remaining program phases.
