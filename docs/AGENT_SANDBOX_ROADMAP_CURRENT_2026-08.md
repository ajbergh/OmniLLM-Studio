# Agent Sandbox Parity Program — Current Roadmap (August 2026)

> **Status:** ACTIVE
>
> **Checkpoint — 2026-08-19:** Windows Phase 12 and macOS Phase 13 are complete. Cross-platform workspace hardening, cumulative Linux CPU enforcement, and the durable-task core have advanced materially. PR #231 merged as `c5adbe417b105d9fd3ce0f2229cca30ad8ec4a91`; PR #232 merged as `1d719861d0d8c1feec2250860ba9af13e3ef7c68`; PR #233 merged as `fe066d016e538d90b33bca314f4f64356fbd7fdf`.

## Program invariants

- Arbitrary model-directed processes do not execute inside the primary backend process.
- Owner scope is revalidated on sandbox operations; IDs are references, not authorization.
- Models do not receive physical host paths for workspace mounts.
- Ambient backend secrets are absent from arbitrary sandboxes and natively confined extensions.
- Network is denied by default and widened only when a runtime can enforce the requested policy.
- Descendants inherit confinement and are terminated on cancellation/session teardown only where the runtime truthfully advertises process-tree isolation.
- Capability reporting is limited to controls actually enforced and proven.
- Guarded Git state/digest protections remain authoritative for reviewed publication workflows.
- Multi-user deployments do not run arbitrary tenant code in the primary API process/container.
- Durable retries fail closed by default; interrupted side-effecting work is never silently replayed.

## Current phase status

| Phase | Scope | Status | Current evidence / next exit |
|---|---|---|---|
| 0 | Architecture, threat model, durable roadmap | **COMPLETE** | Core design documents are on `main`. |
| 1 | Protocol v2 + owner-bound sessions | **COMPLETE** | Broker sessions, ownership/TTL checks, authenticated worker protocol, bounded results, and capability negotiation are merged. |
| 2 | First-party runtime abstraction + Linux execution plane | **IN PROGRESS** | Bubblewrap/rootfs runtime and `sandboxd` exist. Linux delegated cgroup-v2 PID/memory enforcement is merged; cumulative CPU enforcement merged in #232. Dedicated deployment packaging remains active. |
| 3 | Immediate stdio MCP/plugin subprocess hardening | **COMPLETE** | Ambient environment inheritance removed and managed-process confinement policies are active. |
| 4 | Broker-backed `code_execute` + `python_analysis` | **COMPLETE** | Owner-bound execution; restricted Python has no unrestricted host fallback. |
| 5 | Workspace registry + grants + durable journal | **IN PROGRESS** | Linux descriptor-relative reads/mutations/search and durable identity are merged; later Darwin/Windows hardening slices have advanced. Continue only where current platform-native identity/path races remain. |
| 6 | Governed workspace tools | **IN PROGRESS** | Read/write/patch/delete/revert/search exist with platform-native hardening advancing through the current merge train. |
| 7 | `terminal_exec` + cancellation + resource controls | **IN PROGRESS** | Explicit cancellation is merged. Windows PID/memory and Linux PID/memory are enforced. #232 adds cumulative Linux CPU accounting/enforcement evidence while capability promotion remains fail-closed until the complete public contract is proven. Disk remains unsupported. |
| 8 | Network broker + destination approvals | **IN PROGRESS** | Browser egress has its own guarded boundary; arbitrary first-party sandboxes remain network-none unless destination enforcement is independently proven. |
| 9 | Credential broker | **IN PROGRESS** | Opaque owner/TTL handles and raw-secret environment rejection exist; service-specific consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy | **COMPLETE** | Linux Bubblewrap, Windows AppContainer, and macOS Seatbelt persistent-extension confinement are implemented with `auto|required|off` policy. |
| 11 | Desktop sandbox/workspace UX | **COMPLETE** | Workspace grants, review APIs, Settings UX, and protected loopback grant flow are implemented. |
| 12 | Windows native confinement backend | **COMPLETE** | Protocol-v2 and persistent-extension AppContainer/Job confinement with adversarial evidence. |
| 13 | macOS native confinement backend | **COMPLETE** | Seatbelt runtime and persistent extensions are merged; `process_tree_isolation=false` remains truthful for deliberately detached descendants. |
| 14 | Durable sandbox-backed agent tasks | **IN PROGRESS** | #233 merged durable queue/recovery/executor core: immutable intent, owner scope, leases, attempt identities, runtime association, bounded results, and fail-closed retry/recovery. Next exit is application lifecycle composition and startup/shutdown proof. |
| 15 | Server/Kubernetes sandbox workers | **IN PROGRESS** | Dedicated worker packaging/deployment slice is prepared for normalization after Phase 14 lifecycle composition. Target boundary remains isolated worker identity/pods, strict security context, network policy, Secret-backed auth, and explicit operator-owned cgroup delegation. |
| 16 | Multi-agent isolated worktrees/workspaces | **IN PROGRESS** | Isolated snapshot/worktree implementation is prepared behind Phase 15. Promotion must remain digest-bound and governed; sandbox-visible workspaces never receive `.git` authority. |
| 17 | Adversarial assurance suite | **IN PROGRESS** | Native Windows/macOS/Linux workspace/quota/runtime lanes plus browser egress and worker-container assurance run on applicable sandbox heads. Exact-head evidence remains mandatory. |

## Resource-control truth

Enforced where applicable: OS/filesystem/no-network isolation, TTL cleanup, wall-time/output bounds, platform-specific teardown, Windows Job Object process-count and aggregate committed-memory limits, Linux delegated cgroup-v2 `pids.max`, Linux `memory.max` with swap disabled for bounded-memory executions, and cumulative Linux CPU accounting/enforcement from #232.

Broker admission continues to reject non-zero resource requests when the selected runtime does not advertise the matching capability. Capability truth is stricter than implementation presence: a hidden/tested primitive does not justify a public capability bit until the complete runtime contract is proven on the exact head.

`DiskLimit` remains false. No platform has yet proven a hard pre-write storage boundary matching the application `disk_bytes` contract.

Darwin process-tree isolation remains false because a deliberately detached descendant may outlive ordinary process-group teardown even though Seatbelt confinement still applies.

## Durable-task contract

PR #233 established the Phase 14 persistence and recovery foundation:

- immutable serialized create/exec intent;
- owner scope persisted with each task;
- queue leases with independent lease tokens;
- immutable attempt identity and caller-known execution IDs;
- durable Broker session/runtime association before execution;
- bounded terminal result/error persistence;
- retry policy defaults to `never`;
- retries are permitted only for explicitly idempotent work after the previous runtime association has been proven destroyed;
- recovery failures stop admission rather than risking overlapping side effects.

The next lifecycle slice must reuse the process-wide `sandbox.DefaultBroker()` and the application's existing SQLite connection. It must not create a second runtime authority or database connection. Startup recovery must complete before background claiming, worker terminal failure must be observable by the composition root, and shutdown must cancel/wait for in-flight cleanup before API runtime/database teardown.

## Phase 15 target deployment boundary

The dedicated server/Kubernetes worker remains separate from the primary API privilege surface. Required posture:

- dedicated worker image/process identity;
- non-root execution;
- read-only root filesystem where practical;
- `allowPrivilegeEscalation=false`;
- all Linux capabilities dropped;
- no service-account token automount;
- only required API-to-worker ingress;
- default-deny worker egress except explicitly required control/data paths;
- Secret-backed worker authentication;
- cgroup delegation supplied explicitly by the operator/runtime environment, never manufactured by the chart through broad privilege.

## Phase 16 target workspace boundary

Concurrent agent work must use isolated owner-scoped writable workspaces derived from immutable reviewed base material. Promotion/reconciliation must remain explicit and digest-bound. Checkout hooks/filters must not execute while materializing trusted bases, and `.git` authority must remain outside the arbitrary sandbox-visible filesystem.

## Remaining priority work

1. Merge Phase 14 lifecycle composition after exact-head Quality, Security, container, desktop/server, and sandbox assurance passes.
2. Normalize and validate the Phase 15 dedicated Kubernetes worker slice onto that resulting `main`; merge only after the chart and runtime boundary are proven.
3. Normalize and validate Phase 16 isolated concurrent workspaces/worktrees after Phase 15.
4. Continue Phase 8/9 destination-enforced arbitrary-sandbox egress and service-specific credential consumers.
5. Complete any remaining platform-specific governed-workspace identity/path-race hardening and teardown gaps.
6. Design a real hard physical-storage boundary before any `disk_limit` capability is exposed.
7. Promote `cpu_limit` only on runtimes whose complete cumulative CPU contract, admission behavior, descendant accounting, cancellation, and native evidence are all proven. Keep unsupported capability bits false.
8. Continue Phase 17 adversarial assurance with every runtime/platform/deployment change.

## Validation discipline

A sandbox phase is complete only when platform-native negative tests exist, capability claims match enforcement, unsupported controls are explicit and fail closed, and the exact merge head passes every applicable repository gate. Previous green results do not substitute for validation after ancestry or deployment-boundary changes.
