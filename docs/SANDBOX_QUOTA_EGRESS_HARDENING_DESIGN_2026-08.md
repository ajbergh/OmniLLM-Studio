# Sandbox quota and egress hardening design — August 2026

> **Status:** APPROVED IMPLEMENTATION CONTRACT
>
> This document executes the design step called out by `MASTER_PLAN.md` and `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`. It does not mark any quota or destination-scoped egress capability complete. Capability bits remain false until the platform-native enforcement and adversarial evidence below are merged and green on the platform that advertises them.

## Goals

1. Add resource controls one independently verifiable primitive at a time.
2. Keep `RuntimeCapabilities` truthful per runtime and platform.
3. Preserve default-deny networking until destination enforcement exists below untrusted code.
4. Reject requested controls at session creation when a runtime cannot enforce them.
5. Require native negative evidence before a capability bit changes from false to true.

## Non-goals

- No broad network enablement based only on Broker grants.
- No DNS-only allowlist checks followed by an independent connect-time lookup.
- No universal capability claim based on one operating system.
- No reliance on wall-time/output bounds as substitutes for memory, CPU, PID, or physical-disk quotas.
- No tenant arbitrary-code execution in the primary API process/container.

## Resource capability contract

A runtime may advertise a resource capability only when a non-zero request value is translated into an operating-system/runtime enforcement primitive before untrusted code can exceed the requested authority.

| Capability | Request field | Required semantics |
|---|---|---|
| `memory_limit` | `resources.memory_bytes` | Bound aggregate sandbox execution memory at or below the configured byte ceiling, including preventing separately governed swap authority from defeating that ceiling; child processes must not evade the same bound. |
| `cpu_limit` | `resources.cpu_time_ms` | Bound aggregate CPU consumption for the sandbox execution/process tree; wall time is not equivalent. |
| `pid_limit` | `resources.max_processes` | Bound the number of concurrently active processes in the sandbox execution tree. |
| `disk_limit` | `resources.disk_bytes` | Bound bytes attributable to sandbox-writable storage, including runtime-owned workspace/home/tmp and generated artifacts where applicable. |

Zero values mean no caller-requested quota for that resource; they do not weaken other default runtime controls.

## Platform plan

### Windows AppContainer runtime

**Primitive:** Job Objects already exist at process creation through `PROC_THREAD_ATTRIBUTE_JOB_LIST`.

Implementation order:

1. **PID limit first** — add `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` with `ActiveProcessLimit=resources.max_processes` on the same Job Object that already has `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`.
2. **Memory second** — use a Job Object aggregate memory limit (`JOB_OBJECT_LIMIT_JOB_MEMORY`) rather than a per-process-only limit so descendants share the ceiling.
3. **CPU third** — use Job Object CPU controls only after confirming semantics for the desired aggregate CPU-time contract and cancellation/error reporting.
4. **Disk later** — AppContainer filesystem confinement does not itself meter physical bytes; implement explicit runtime-owned writable-root accounting/preflight plus enforcement that cannot be bypassed by alternate writable locations.

Required native evidence for PID and memory:

- capability false before implementation and true only after the exact native test is present;
- creation succeeds when the matching requirement is requested and the quota is non-zero;
- a process-tree attempt beyond the PID ceiling fails while the root execution remains confined;
- an allocation beyond the memory ceiling cannot escape through a child process;
- cancellation/timeout still tears down the Job tree;
- an unrelated concurrent sandbox is unaffected;
- zero-valued quotas preserve existing behavior.

### Linux Bubblewrap runtime

**Primitive boundary:** Bubblewrap supplies namespace/filesystem/network confinement but is not a complete resource controller. Use a cgroup-v2 boundary owned by `sandboxd` (or an equivalent separately privileged worker boundary), not shell `ulimit` as the authoritative capability.

Implementation order:

1. detect writable/delegated cgroup-v2 support at runtime startup;
2. create one cgroup per sandbox execution before untrusted code starts;
3. map `max_processes` to `pids.max`;
4. for positive `memory_bytes`, map the byte ceiling to `memory.max`, set `memory.swap.max=0` so anonymous pages cannot escape through separately governed swap, and observe `memory.events` for OOM enforcement evidence;
5. define CPU semantics explicitly before mapping to `cpu.max`/accounting because `cpu_time_ms` is cumulative time while `cpu.max` is a rate/quota control;
6. account writable runtime roots for disk separately; cgroups do not provide a portable physical-disk-byte quota.

The PID and memory capabilities are independent. If the worker lacks a required delegated controller or strict memory/swap interface, the corresponding capability remains false and a non-zero request for that resource fails closed.

### macOS Seatbelt runtime

Seatbelt provides filesystem/network policy, not the required resource metering. Do not advertise memory/CPU/PID/disk controls merely because process-group teardown exists.

Implementation candidates must be proven individually. In particular:

- `setrlimit`/launch-time limits may be useful for process-local controls but do not automatically satisfy aggregate descendant semantics;
- the known detached-descendant limitation means PID/CPU/memory claims require evidence that the chosen control follows or independently constrains detached descendants;
- `process_tree_isolation` remains false until the existing detached-process limitation is actually removed.

No macOS resource capability bit changes under this design alone.

## Destination-scoped egress contract

Broker destination grants are authorization only. A runtime may advertise `network_allowlist=true` only when every outbound connection from arbitrary sandbox code is forced through an enforceable egress boundary.

The boundary must provide all of the following:

1. default deny when no destination grant exists;
2. HTTP, HTTPS/CONNECT, WebSocket, raw TCP where supported, and DNS behavior are explicitly covered rather than assumed;
3. hostname validation and connect use the same resolved address or an equivalent pinned-address mechanism;
4. every redirect/new connection is revalidated;
5. private, loopback, link-local, metadata, multicast, unspecified, reserved, and otherwise disallowed addresses fail closed;
6. proxy environment variables supplied by untrusted code cannot bypass the boundary;
7. direct socket egress cannot bypass an HTTP proxy;
8. destination grants remain owner/session scoped and expire with their authorization;
9. denial and bounded destination metadata are auditable without logging credentials or sensitive payloads.

### Recommended deployment boundary

For first-party server/Kubernetes workers, prefer a dedicated sandbox worker network namespace/pod plus an egress enforcement component whose network policy prevents direct alternate egress. Application-layer destination validation may run in a local broker/proxy, but infrastructure policy must ensure arbitrary code cannot route around it.

For local desktop runtimes, keep the current **no-network** posture until an OS-specific forced-egress primitive is proven. Do not weaken AppContainer, Bubblewrap, or Seatbelt network denial simply because the browser egress proxy exists; browser enforcement is not a sandbox-wide socket boundary.

## Capability reporting and admission

`RuntimeCapabilities` remains runtime-specific. Broker admission continues to reject `RuntimeRequirements` that the chosen runtime does not advertise.

Additional invariant for resource fields:

- when a caller supplies a non-zero quota for a resource that the runtime cannot enforce, runtime creation must fail closed even if the caller omitted the matching `RuntimeRequirements` bit. This prevents a request from silently degrading into an unenforced limit.

This invariant should be added before the first resource capability is enabled.

## Adversarial test matrix

Every new control needs both positive and negative tests.

- **Admission:** unsupported non-zero limit fails closed; supported limit is accepted.
- **Boundary:** exceed the configured quota and observe enforcement.
- **Descendants:** repeat the attempt from a child/grandchild process.
- **Isolation:** one sandbox cannot consume or mutate another sandbox's quota controller.
- **Lifecycle:** cancellation, timeout, TTL cleanup, and destroy remove the controller and descendants.
- **Race:** rapid create/exec/cancel/destroy does not leave an unbounded process/controller behind.
- **Capability truth:** status/capability output exactly matches the native control proven on that platform.
- **Memory evidence:** verify the configured hard memory limit and any required swap restriction, then require kernel accounting (`memory.events`) to show the over-limit descendant was constrained; an allocation merely failing for an unrelated reason is insufficient.
- **Egress:** DNS rebinding shape, redirect, proxy env, private-address destination, alternate port, WebSocket, and direct-socket bypass attempts all fail unless explicitly authorized and enforced.

## Implementation sequence

1. Add generic admission validation that rejects any non-zero memory/CPU/PID/disk request when the selected runtime capability is false.
2. Implement Windows PID limit using the existing pre-start Job Object boundary and add native negative tests.
3. Implement Windows aggregate memory limit and native child-process tests.
4. Add Linux cgroup-v2 capability detection and PID limit; then strict memory limit with `memory.max`, swap denial, and `memory.events` evidence.
5. Resolve CPU semantic mismatch before enabling `cpu_limit` anywhere.
6. Design physical-disk accounting/enforcement for runtime-owned writable roots.
7. Keep sandbox egress at `network=none` while designing a forced-egress worker boundary; implement destination-scoped egress only with both application and network bypass evidence.
8. Re-evaluate macOS resource primitives independently; do not inherit claims from Windows/Linux.

## Exit criteria

The sandbox hardening work may advance to durable sandbox-backed tasks only after the intended production worker platforms have native enforcement for the quotas required by that deployment, destination-scoped egress is either enforced or explicitly unavailable with no-network fail-closed behavior, and the Phase 17 adversarial suite continuously verifies the advertised claims.
