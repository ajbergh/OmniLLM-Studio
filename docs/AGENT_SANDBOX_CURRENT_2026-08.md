# Agent Sandbox — Current Documentation Index (August 2026)

Use these files for the current sandbox implementation state after Windows Phase 12, macOS Phase 13, cross-platform governed-workspace hardening, cumulative Linux CPU enforcement, and the active durable-task/worker-isolation program:

- `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` — authoritative phase status, merged lineage, open enforcement gaps, and execution order.
- `SANDBOX_RUNTIME_CURRENT_2026-08.md` — current runtime and persistent-extension operator behavior.
- `SANDBOX_QUOTA_EGRESS_HARDENING_DESIGN_2026-08.md` — resource-quota and forced-egress implementation contract.
- `AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md` — implemented Broker/runtime/managed-process architecture and platform boundaries.
- `AGENT_SANDBOX_PHASE13_MACOS_2026-08.md` — macOS native-confinement implementation and validation record.
- `AGENT_SANDBOX_PHASE13D_MACOS_ASSURANCE_2026-08.md` — macOS adversarial-assurance evidence and explicit teardown limitation.
- `AGENT_SANDBOX_THREAT_MODEL.md` — threat model and adversarial acceptance principles.

Completed historical records remain under `archive/`. The versioned current-state documents, this index, and `MASTER_PLAN.md` are authoritative for current status and outstanding work.

## Current checkpoint — 2026-08-19

The sandbox program has advanced beyond the earlier Linux-PID checkpoint.

- Windows native confinement is complete through PR #149; explicit execution cancellation is complete through PR #155.
- macOS Phase 13 is complete through PR #166. Seatbelt confinement remains truthful about the detached-descendant teardown limitation: `process_tree_isolation=false`.
- Windows process-count and aggregate-memory enforcement are merged; Linux delegated cgroup-v2 process-count and strict aggregate-memory enforcement are merged.
- Linux governed-workspace descriptor-relative reads, mutations, durable root identity, and search are merged. Darwin and Windows governed-workspace hardening subsequently advanced on `main`; current roadmap details are authoritative.
- PR #231 merged as `c5adbe417b105d9fd3ce0f2229cca30ad8ec4a91`, completing the Windows governed-workspace hardening slice in the active merge train.
- PR #232 merged as `1d719861d0d8c1feec2250860ba9af13e3ef7c68`, adding cumulative Linux CPU-budget enforcement with delegated cgroup-v2 native evidence. Public capability reporting remains fail-closed: `cpu_limit` is not promoted where the full runtime contract is not proven.
- PR #233 merged as `fe066d016e538d90b33bca314f4f64356fbd7fdf`, adding the durable sandbox-task queue, immutable task intent, lease/attempt identity, runtime association, recovery, and fail-closed retry semantics.

## Active phases

### Phase 14 — durable sandbox-backed agent tasks

**IN PROGRESS.** The durable queue/recovery/executor core is on `main` through #233. The next slice composes a single process-wide durable worker into server and desktop lifecycles, performs startup recovery before accepting work, observes worker failure, and shuts the worker down before API runtime/database teardown. Sandbox-disabled processes remain a no-op.

### Phase 15 — dedicated server/Kubernetes workers

**IN PROGRESS.** Dedicated sandbox-worker packaging and isolated Kubernetes deployment remain the next deployment boundary. The target posture is non-root, read-only root filesystem, no privilege escalation, all capabilities dropped, no service-account token automount, API-only ingress, default-deny worker egress, Secret-backed authentication, and explicit operator-owned cgroup delegation.

### Phase 16 — isolated concurrent agent workspaces

**IN PROGRESS.** The queued implementation uses owner-scoped Git commit snapshots, immutable base materialization without checkout hooks/filters, digest-bound review, and guarded promotion. `.git` authority remains outside the sandbox-visible writable workspace.

### Phase 17 — adversarial assurance

**IN PROGRESS.** Every platform/runtime slice continues to require exact-head native negative evidence plus repository-wide Quality, Security, container, and applicable sandbox-assurance gates.

## Remaining priority work

1. Land lifecycle composition for the durable task worker and validate startup recovery plus graceful shutdown on server and Wails desktop.
2. Normalize and validate the dedicated Kubernetes worker slice on the resulting `main`.
3. Normalize and validate isolated concurrent workspaces/worktrees after the worker deployment slice.
4. Keep unsupported capability bits false. In particular, do not advertise a CPU or disk limit without the exact runtime semantics and native evidence required by the public contract.
5. Continue destination-enforced arbitrary-sandbox egress and service-specific credential-broker consumers.
6. Continue platform-specific governed-workspace and teardown hardening where any pathname/identity/process-tree limitation remains.

## Validation rule

A sandbox phase is complete only when enforcement exists at the actual OS/runtime boundary, negative tests prove the boundary, capability reporting matches what is enforced, unsupported controls fail closed, and the exact merge head passes every applicable repository gate.
