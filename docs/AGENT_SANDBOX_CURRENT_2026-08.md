# Agent Sandbox — Current Documentation Index (August 2026)

Use these files for the current sandbox implementation state after Windows Phase 12, completed macOS Phase 13, and the active cross-platform hardening, durable-task, worker-isolation, and multi-agent workspace program:

- `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` — active phase status, open enforcement gaps, and execution order.
- `SANDBOX_RUNTIME_CURRENT_2026-08.md` — current runtime and persistent-extension operator behavior.
- `SANDBOX_QUOTA_EGRESS_HARDENING_DESIGN_2026-08.md` — approved resource-quota and forced-egress implementation contract.
- `AGENT_SANDBOX_ARCHITECTURE_CURRENT_2026-08.md` — implemented Broker/runtime/managed-process architecture and platform boundaries.
- `AGENT_SANDBOX_PHASE13_MACOS_2026-08.md` — macOS native-confinement implementation and validation record.
- `AGENT_SANDBOX_PHASE13D_MACOS_ASSURANCE_2026-08.md` — macOS adversarial-assurance evidence and explicit teardown limitation.
- `AGENT_SANDBOX_THREAT_MODEL.md` — threat model and adversarial acceptance principles.

Completed Windows Phase 12 records and older aggregate snapshots are retained under `archive/`. The versioned current-state documents, this index, and `MASTER_PLAN.md` are authoritative for current status and outstanding work.

## Current checkpoint — 2026-08-19

Windows native confinement is complete through PR #149, merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`. Explicit execution cancellation addressability is complete in PR #155 (`35b5533d0556532a762aaa27522a9be4029f1fee`). macOS Phase 13 completed through PR #166 (`d52ab16f6f1cdc14bd7762ccb13d16964d665b17`), including native Seatbelt runtime/extension confinement and adversarial evidence while preserving the truthful detached-descendant `process_tree_isolation=false` limitation.

The quota and governed-workspace program has advanced beyond the earlier PID-only checkpoint. Windows process-count and aggregate committed-memory enforcement are merged (#171/#172). Linux delegated cgroup-v2 process-count and strict aggregate memory enforcement are merged (#173/#174). Linux governed workspace reads, mutations, durable root identity, and descriptor-relative search are merged (#175/#176/#177/#182). Darwin durable registered-root identity is merged through #183.

### Active workspace hardening

- **PR #210 — Darwin governed workspace operations:** descriptor-relative/no-follow reads, search, and mutation transactions. Its reviewed implementation head passed the full repository matrix; because `main` advanced afterward, no-code head `46b772f2` is revalidating the same implementation against current `main` before merge.
- **PR #211 — Windows governed workspace identity/path races:** native volume/file identity, reparse rejection, opened-object validation, pinned-parent mutation, and native NT relative rename. Native Windows confinement evidence is green on the stacked implementation head. It remains stacked on #210 and will be normalized onto `main` after #210 merges.

### Network and credential boundary

- **PR #213 — brokered egress/credentials:** the reviewed implementation head passed all gates; no-code head `7fb04952` is revalidating it against the current `main` merge base. First-party arbitrary runtimes still remain network-none unless a separately enforceable destination boundary is selected.

### Phase 14 — durable sandbox-backed agent tasks

Phase 14 is now **IN PROGRESS**, not “not started.”

- **PR #215 — durable queue/recovery/executor core:** exact head `70d3d46a` is green across Quality Gate, Security Scan, container builds, and applicable sandbox assurance. It persists immutable create/exec intent, owner scope, lease tokens, attempt identities, Broker/runtime association, bounded results, and fail-closed recovery. Retry defaults to `never`; only explicitly idempotent work may receive a fresh attempt after the prior runtime association has been proven destroyed.
- **PR #223 — application lifecycle composition (stacked on #215):** reuses the process-wide authenticated `sandbox.DefaultBroker()` and application SQLite connection; performs expired-runtime recovery synchronously before accepting work; runs the durable claim loop in server and Wails lifecycles; and cancels/waits for in-flight sandbox cleanup before API runtime/database teardown. Sandbox-disabled processes remain a no-op and no second Broker/runtime/database authority is created.

### Phase 15 — dedicated server/Kubernetes workers

Phase 15 is **IN PROGRESS**.

- **PR #214:** packages `cmd/sandboxd` as a dedicated non-root worker image with Bubblewrap and a separate immutable execution rootfs rather than expanding the API image privilege surface.
- **PR #216 (stacked on #214):** adds an isolated Kubernetes worker Deployment/ServiceAccount, no token automount, non-root/read-only/no-escalation/all-capabilities-dropped security context, API-only ingress, default-deny worker egress, Secret-backed worker authentication, and automatic backend worker URL/token wiring. Cgroup delegation remains an explicit operator responsibility rather than chart-manufactured privilege.

### Phase 16 — isolated concurrent agent workspaces

- **PR #217:** implements owner-scoped Git commit snapshots with trusted metadata outside the sandbox-visible tree, immutable base materialization without checkout hooks/filters, digest-bound review, and guarded promotion. It does not expose `.git` authority or bypass existing guarded stage/commit/publication controls.

### CPU and disk enforcement

- **Draft PR #221:** implements staged cumulative process-tree CPU enforcement work. Linux uses aggregate cgroup-v2 `cpu.stat usage_usec`, whole-cgroup termination, a rate ceiling only to bound sampling overshoot, final accounting, descendant fan-out evidence, and an end-to-end Bubblewrap test. Public Linux `LocalRuntime.Create` still rejects a non-zero CPU request while `CPULimit=false`; test-only injection exercises the hidden enforcement path until exact-head native assurance justifies promotion.
- Windows CPU work in #221 currently includes aggregate Job Object user+kernel accounting, a cumulative monitor, whole-Job termination, and descendant-pressure native test scaffolding. Windows AppContainer runtime wiring and direct-runtime CPU admission hardening remain unfinished, so `CPULimit=false` is still required.
- Darwin aggregate CPU accounting remains unresolved because detached descendants are not yet covered by a defensible process-tree accounting primitive.
- `DiskLimit` remains false everywhere. No platform has yet proven a hard pre-write storage boundary matching the application `disk_bytes` contract.

### CI reliability

PR #219 merged the bounded/retried Linux package and Playwright bootstrap used by the shared Quality workflow. Security-sensitive sandbox PRs are being revalidated against the current merge base rather than waived when `main` advances.

## Remaining priority work

1. Finish current-main validation and merge #210, then normalize/revalidate #211.
2. Finish current-main validation for #213 and merge if every exact-head gate remains green.
3. Land #215's durable core, then normalize #223 and prove server/desktop startup recovery and graceful shutdown on its final `main`-based head.
4. Complete #214/#216 worker packaging and Kubernetes isolation validation, including target-cluster evidence for any intentionally delegated cgroup capabilities.
5. Use #221 native results to finish Linux CPU capability promotion only if the exact cumulative-tree contract is proven; complete Windows AppContainer CPU admission/runtime wiring separately. Keep unsupported capability bits false.
6. Complete Phase 16 validation/promotion boundaries in #217 and continue Phase 17 adversarial assurance with every runtime/platform change.
7. Design a real hard storage boundary before any `disk_limit` capability is exposed.
