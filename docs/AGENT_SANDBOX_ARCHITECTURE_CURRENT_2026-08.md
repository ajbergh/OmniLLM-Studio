# Agent Sandbox Architecture — Current State (August 2026)

This document records the implemented execution architecture after Windows Phase 12 and macOS Phases 13A–13C. It supplements the earlier `AGENT_SANDBOX_ARCHITECTURE.md` design snapshot.

## Trust boundaries

OmniLLM-Studio separates policy, ownership, and OS enforcement:

```text
Chat / Agent / Tool request
        |
        v
Tool Executor policy + approval
        |
        v
Sandbox Broker
  - owner scope
  - workspace grants
  - TTL
  - network/credential grant references
  - runtime capability requirements
        |
        v
Authenticated protocol-v2 runtime
  - platform-native process isolation
  - filesystem boundary
  - network boundary
  - process-tree teardown
        |
        v
Untrusted process
```

Persistent local plugins and stdio MCP use a parallel process-construction path rather than the protocol-v2 request/response execution API, but they share the same policy principles and platform confinement primitives.

## Broker responsibilities

The application-owned Broker remains authoritative for:

- public `sbx_...` session IDs;
- authenticated owner/workspace/conversation/run scope;
- runtime-private ID mapping;
- workspace grant resolution;
- capability requirements;
- TTL and lifecycle coordination;
- network and credential grant references;
- artifact metadata trust boundaries.

Runtime IDs and execution IDs are references, not authorization. Physical workspace roots remain backend/runtime state.

## Protocol-v2 runtime boundary

The runtime owns technical enforcement for approved arbitrary execution:

- process isolation;
- filesystem isolation;
- network isolation;
- process-tree lifetime;
- environment construction;
- wall/output bounds;
- platform-specific cleanup.

Capability negotiation is fail closed. A runtime must not report a control that it cannot enforce.

Current first-party platform implementations:

- **Linux:** Bubblewrap with an operator-prepared read-only rootfs.
- **Windows 10+:** AppContainer plus Job Objects.
- **macOS:** Seatbelt-backed local runtime plus persistent-extension confinement. Phase 13D adversarial assurance completed in PR #166; its deliberately detached-descendant limitation keeps `process_tree_isolation=false`.

## Persistent extension process seam

Local plugins and stdio MCP keep streaming stdin/stdout JSON-RPC but no longer require callers to manipulate `*exec.Cmd` directly.

The shared managed process lifecycle is:

```text
CommandProcess
  StdinPipe()
  StdoutPipe()
  StderrPipe()
  Start()
  Wait()
  Kill()
```

Platform adapters implement that contract:

- sanitized host compatibility adapter;
- Linux Bubblewrap adapter;
- Windows AppContainer/Job adapter.

This preserves MCP/plugin protocol code while allowing forced shutdown to terminate the platform-confined process tree rather than only the root process.

## Extension policy modes

`OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off` is the operator policy boundary.

### Linux

- `auto` uses Bubblewrap when a sandbox rootfs is configured;
- `required` fails closed when native confinement is unavailable;
- `off` uses sanitized-host compatibility.

### Windows

- `auto` uses native AppContainer confinement when available;
- `required` fails closed when native confinement cannot be provided;
- `off` uses sanitized-host compatibility.

### macOS

The fixed `/usr/bin/sandbox-exec` Seatbelt backend now confines both first-party local runtime executions and persistent stdio MCP/plugin processes. In `auto` and `required` modes, native confinement is selected when the platform primitive is available; `required` fails closed if it is unavailable, while `off` remains the explicit sanitized-host compatibility mode. The per-process profile supplies explicit system/command/working-directory read roots, runtime-owned home/tmp write roots, reconstructed environment, and default network denial. Process-group teardown is proven for ordinary descendants but not intentionally detached sessions, so no stronger process-tree claim is made.

## Windows enforcement design

### Identity

Each protocol-v2 Windows runtime session receives a unique ephemeral AppContainer profile/package SID. Persistent natively confined extensions likewise receive unique ephemeral AppContainer profiles.

Phase 12A also provides unique application-issued restricting-SID primitives for resources that need explicit SID-scoped authority. The AppContainer package SID is the process/filesystem/network identity used by the Phase 12B/12C execution paths.

### Process creation

Windows child creation applies confinement before untrusted code runs:

- `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` attaches AppContainer security capabilities;
- `PROC_THREAD_ATTRIBUTE_JOB_LIST` binds a kill-on-close Job Object during process creation;
- `PROC_THREAD_ATTRIBUTE_HANDLE_LIST` restricts inherited handles to intended stdio handles.

There is no start-unrestricted/post-assign Job window in the implemented Windows paths.

### Filesystem

The Windows design avoids widening ACLs on original user workspace/extension directories.

Protocol-v2 project access:

- no project mount -> AppContainer-owned writable scratch;
- one `read_only` project mount -> bounded copy into AppContainer-owned staged storage;
- writable arbitrary-process project mounts -> fail closed.

Persistent extension access:

- non-System32 command bundles are staged read-only;
- required working-directory content may be staged read-only;
- absolute arguments are remapped only when they resolve beneath staged roots;
- unrelated absolute host paths fail closed.

Staging rejects:

- reparse points and junctions;
- multiply-linked files;
- special files;
- traversal/destination escapes;
- post-open source-handle paths that no longer resolve under the canonical source root.

AppContainer-owned home/tmp remain writable.

### Network

Windows AppContainer processes are launched with zero network capabilities. The first-party Windows runtime and persistent native extension path are therefore no-network by default.

The Broker's destination-grant model does not itself create egress enforcement. `network_allowlist` remains false until a destination-scoped enforcement mechanism is implemented.

### Environment and credentials

Native Windows children receive a runtime-owned minimal environment. Ambient backend environment is not copied wholesale.

Credential-sensitive explicit extension environment values are rejected under native confinement by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` remains a narrow compatibility override rather than the intended long-term credential architecture.

### Lifecycle

Job teardown is used for:

- root completion;
- context cancellation;
- timeout;
- forced plugin shutdown;
- runtime session destruction.

Profile cleanup is idempotent/retryable when Windows temporarily blocks deletion.

## Linux enforcement design

The Linux first-party runtime uses Bubblewrap with:

- read-only configured rootfs;
- private namespaces/session;
- no-network namespace;
- explicit trusted workspace mounts;
- ephemeral scratch;
- process-tree/session teardown;
- wall/output bounds.

Persistent Linux extensions use the same Bubblewrap approach when configured.

Destination-scoped egress and memory/CPU/PID/disk quotas remain outside the currently advertised first-party capability set.

## Workspace mutation model

Arbitrary process access and application-level workspace mutation remain deliberately distinct.

Preferred source-change workflow:

```text
sandbox: inspect / build / test with bounded project access
        |
        v
Omni workspace mutation tools
  - owner-scoped relative paths
  - journal
  - before/after hashes
  - stale-state binding
        |
        v
Omni Git read tools: status/diff
        |
        v
approval
        |
        v
Guarded Git stage/commit/publication
```

This preserves reviewed Git state/digest protections and avoids giving arbitrary shell execution broad mutation/publication authority.

## Desktop and server profiles

### Desktop

The desktop target supports native folder selection and owner-scoped workspace grants. Model-facing APIs expose opaque grant IDs, not physical host roots.

Path-grant creation is admin-gated, operator-enabled, and direct-loopback only.

### Headless/server

The primary API process must not become an arbitrary tenant execution container. Dedicated worker identity and hardened worker deployment remain Phase 15 work.

### Kubernetes

Future dedicated workers should use non-root identity, read-only root filesystem where feasible, no privilege escalation, dropped capabilities, seccomp/runtime-default or stricter policy, resource limits, ephemeral storage, NetworkPolicy, and no arbitrary multi-user `hostPath` mounts.

## Current assurance evidence

Windows Phase 12 native evidence covers:

- AppContainer token state;
- read-only staged content and writable sandbox-owned home;
- unrelated host-file denial;
- cross-profile authority denial;
- ambient-secret absence;
- loopback/network denial;
- descendant teardown after root exit and context cancellation;
- hard-link and junction/reparse rejection;
- unrelated absolute-argument fail-close;
- sensitive environment fail-close;
- `auto|required|off` backend selection.

Final Phase 12D head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed the full repository Quality/Security and applicable container gates before PR #149 merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

## Explicitly open architecture work

- Issue #151: make explicit protocol-v2 execution cancellation addressable while synchronous `Exec` is running.
- Resource quotas: memory, CPU, PID/process-count, physical disk.
- Destination-scoped egress enforcement.
- Broader workspace-registry/path-component TOCTOU assurance.
- Service-specific credential broker consumers.
- macOS adversarial assurance and a disposition for intentionally detached descendants.
- Dedicated server/Kubernetes workers.
- Durable sandbox-backed tasks and multi-agent worktree isolation.

These gaps do not invalidate the completed Windows confinement boundary; they remain separate roadmap items and must fail closed where a required control is unavailable.
