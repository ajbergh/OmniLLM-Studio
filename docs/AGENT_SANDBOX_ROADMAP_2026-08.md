# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and operator documentation:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md`

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens or merges, an enforcement limitation changes, or validation evidence changes.

## Current checkpoint — 2026-08-12

The authoritative default branch used for the Phase 11 replay was `main` at **`054a235ac5269572573adb70f92a2e5dbb16a944`**. That commit includes the GitHub Settings/repository work from PR #123.

Sandbox lineage now has two validated integration milestones:

- **PR #118**, squash-merged as `a216323e512fbecb1aa0c7c14df866f85ef76eb0`, recovered the cumulative sandbox implementation onto the then-current `main` and repaired integration defects found during manual audit.
- **PR #119**, squash-merged as `dd91b246736451fafc498659fa582ff605e1bf16`, added persistent extension confinement policy for local plugins and stdio MCP without rewriting their streaming lifecycle.

Both #118 and #119 passed the applicable repository validation set before merge, including backend format/vet/tests/race, frontend lint/unit/build, Windows compatibility coverage, full Chromium Playwright smoke, Helm checks, Security Scan, and Linux multi-architecture container validation.

Historical stacked PRs #101, #104, #105, #107, #108, #109, #110, and #111 were closed as superseded after #118 merged. Stale Phase 11 PR **#121** was closed after its reviewed implementation was replayed cleanly onto current `main` as **PR #125**.

## Non-negotiable invariants

1. Arbitrary model-generated processes never execute in the OmniLLM backend process.
2. Sandboxed/local extension processes do not inherit ambient backend secrets or the backend environment by default.
3. Models never supply physical host paths for sandbox mounts; application-owned workspace IDs are used instead.
4. Sandbox IDs are application-issued references and every operation revalidates user/workspace/conversation/run ownership.
5. Filesystem access is explicit: `read_only`, `read_write_no_delete`, or `read_write`.
6. Network is denied by default and may be widened only within operator policy and an enforceable runtime.
7. Descendants inherit sandbox restrictions and cancellation destroys the execution process tree.
8. Runtimes report controls they actually enforce; required-but-unavailable controls fail closed.
9. Raw provider/GitHub/master/session/browser/SSH/cloud credentials are not injected into arbitrary sandboxes.
10. Existing reviewed Git state/digest protections remain authoritative for stage/commit/remote publication.
11. Local plugins and stdio MCP use the same confinement policy principles as arbitrary code/terminal execution.
12. Multi-user deployments never run arbitrary tenant code in the primary API process/container.

## Roadmap status

Status values: `NOT STARTED`, `IN PROGRESS`, `COMPLETE`, `BLOCKED`.

| Phase | Scope | Priority | Status | Branch / PR | Current evidence / next exit criterion |
|---|---|---:|---|---|---|
| 0 | Architecture, threat model, durable roadmap | P0 | **COMPLETE** | merged PR #98 | Architecture, threat model, and tracker are on `main`. |
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | **COMPLETE** | merged PR #118 | Broker-issued `sbx_` IDs, exact owner/TTL checks, authenticated protocol-v2 runtime, capability negotiation, bounded protocol, and artifact-ID trust model are on `main`. |
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | implementation merged in #118 | Bubblewrap/rootfs runtime, authenticated `sandboxd`, trusted mounts, and runtime TTL cleanup are on `main`. Exit still requires native isolation fixtures/packaging, resource quotas, and enforceable egress where claimed. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **COMPLETE** | merged PR #99 | Ambient `os.Environ()` inheritance removed; shared sanitized runner and secret-leak regression tests are on `main`. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | **COMPLETE** | merged PR #118 | Legacy unauthenticated execution path retired; both tools use owner-bound Broker sessions; restricted Python has no host-Python fallback. |
| 5 | Workspace registry + RO/RW-no-delete/RW grants + durable journal | P1 | **IN PROGRESS** | implementation merged in #118 | Opaque owner-scoped grants, canonical roots, state-bound atomic mutations, before/after hashes, bounded snapshots, and reverts are on `main`. Residual path-component TOCTOU/rename-swap assurance remains. |
| 6 | Workspace list/search/read/write/apply-patch/delete/revert tools | P1 | **IN PROGRESS** | implementation merged in #118 | Governed tool family is on `main`, high-risk mutations default to Ask, host roots stay hidden, and mutations use the journaled filesystem layer. Completion tracks Phase 5 containment assurance. |
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | implementation merged in #118 | Explicit argv execution, owner-bound sessions, read-only project mounts, TTL cleanup, wall/output limits, and cancellation are on `main`. Memory/CPU/PID/disk limits remain intentionally false until enforced. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | implementation merged in #118 | Owner-bound `sng_` grants, operator domain/port policy, grant consumption, and `network_allowlist` capability fail-close are on `main`. First-party destination-enforced egress remains unimplemented. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | implementation merged in #118 | Host-side opaque `sch_` handles, owner/TTL checks, and arbitrary-sandbox credential/auth-agent/proxy environment rejection are on `main`. Service-specific credential consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy migration | P1 | **COMPLETE** | merged PR #119 (`dd91b246`) | Shared process-construction seam now supports `auto|required|off`; Linux can use Bubblewrap/rootfs, required mode fails closed, and Windows/macOS preserve the sanitized compatibility boundary until native backends land. Full validation passed before merge. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | **IN PROGRESS** | `agent/sandbox-settings-ux-current-main-20260812` / PR #125 | Safe status/workspace/change APIs, Wails folder picker, opaque grant management, capability truth, and review-only journal history are implemented. Exit: full CI/security/container validation and merge to `main`. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted identity/token, Job Object/process-tree confinement, ACL-scoped workspace, and Windows-native isolation evidence required. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and macOS-native isolation evidence required. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, and no arbitrary tenant execution in the API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99, #118, #119, #125 onward | Expand negative coverage with every phase; native confinement cannot be declared complete from cross-compilation alone. |

## Phase 11 — active implementation details

PR **#125** is a clean replay from current `main`; the intervening GitHub Settings work touched none of the eight Phase 11 implementation files.

Implemented surfaces:

- authenticated `/v1/sandbox/status` reporting actual Broker/runtime capabilities and extension policy without returning rootfs paths, runtime URLs, tokens, or secrets;
- owner-scoped workspace listing that returns opaque workspace ID, access mode, and timestamps only;
- owner-scoped recent change review returning relative path, operation, before/after existence and hashes, revertability, and timestamp only;
- an explicit safe change-history DTO that does **not** serialize internal user, conversation, agent-run, task, sandbox, or execution identifiers;
- host-path grant creation gated by admin authorization, `OMNILLM_SANDBOX_ALLOW_PATH_GRANTS=true`, and the actual socket peer being loopback;
- grant revocation that removes the database authorization only and never deletes workspace files;
- Wails-native folder selection for desktop builds;
- desktop-only default enablement of the path-grant flow, using the existing protected per-launch loopback capability URL;
- an Agent Sandbox panel embedded in the existing Tools diagnostics/settings surface;
- individual capability badges so namespace isolation is not confused with destination allowlist enforcement;
- ephemeral frontend handling of the selected physical path: after an opaque grant is created, the path is cleared from component state;
- review-only display of reversible changes. No direct HTTP revert bypass is added; actual revert remains behind the governed workspace tool/approval path.

Server/web deployments do **not** gain a generic remote filesystem picker. Operators must explicitly enable host-path grants, and even then creation is restricted to loopback plus admin authorization.

## Current execution order

1. Validate PR #125 against the repository's full applicable gate set.
2. Fix any code, formatting, test, desktop-binding, security, browser, Helm, or container failures on #125 itself.
3. Merge #125 only after all required checks are green; then verify the exact `main` merge commit and mark Phase 11 complete in this tracker.
4. Implement and natively validate Windows confinement (Phase 12) and macOS confinement (Phase 13) independently.
5. Continue Phase 2/5/7/8/9 enforcement work rather than overclaiming completion: resource quotas, destination-enforced egress, workspace TOCTOU assurance, and service-specific credential consumers remain open.
6. Add durable tasks, dedicated server/Kubernetes workers, and multi-agent worktree isolation as separate P2 slices.
7. Keep the adversarial assurance suite continuous across every phase.

## Open enforcement gaps

### Filesystem

- Model tools do not receive configured host roots.
- Workspace-relative paths reject absolute/traversal paths, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is narrowed to read-only for arbitrary POSIX shell access rather than approximated unsafely.
- `terminal_exec` currently requests read-only project mounts so source writes remain in the journaled workspace-tool path.
- **Still open:** a residual path-component TOCTOU class exists if an independently writable parent is swapped between validation and mutation. Phase 5/17 remain incomplete until this is natively addressed/tested.

### Runtime resources

Currently represented as enforceable by the first-party Linux runtime:

- OS/process namespace isolation;
- filesystem isolation through read-only rootfs plus explicit trusted mounts;
- no-network namespace isolation;
- process-tree/session confinement;
- session TTL cleanup;
- wall-time and stdout/stderr limits.

Still intentionally **not advertised** until implemented and validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota.

### Network

- Default is no network.
- Network authorization requires operator destination policy plus an owner-bound high-risk grant.
- IP literals and localhost are rejected from the grant surface.
- A runtime must separately advertise destination-allowlist enforcement; namespace isolation alone is not equivalent.
- **Still open:** the first-party Linux runtime does not yet enforce destination-scoped egress and therefore remains no-network even when a destination grant exists.

### Credentials

- Arbitrary sandbox environments reject credential-bearing keys, SSH/Git auth delegation, cloud credential-file pointers, and proxy variables.
- Opaque credential handles carry no secret values and are ownership/TTL scoped.
- Existing guarded Git/GitHub operations remain host-side.
- **Still open:** service-specific credential broker consumers are needed before arbitrary sandbox tasks can use narrowly delegated external-service credentials.

### Persistent extensions

Phase 10 established the policy boundary:

- `OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off`;
- Linux `auto` uses Bubblewrap only when a sandbox rootfs is configured;
- `required` fails closed when native confinement is unavailable;
- Windows/macOS `auto` retain the sanitized host compatibility boundary until Phases 12/13;
- ambient backend secrets remain stripped in compatibility mode;
- native Linux extension confinement clears the environment and rejects credential-sensitive explicit environment by default;
- `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a narrow transitional operator override, not the desired long-term credential path.

## Security acceptance categories

Every relevant phase expands negative tests for:

- traversal, absolute paths, symlink/junction/reparse/hard-link and rename escape;
- orphan/daemon descendants, fork/process abuse, cancellation escape;
- CPU/memory/disk/file-count/output exhaustion;
- localhost/private/link-local/metadata/DNS-rebinding/proxy/network bypass;
- backend/provider/GitHub/master/session/browser/SSH/cloud credential access;
- cross-user/workspace/conversation/run/sandbox/artifact/grant references;
- artifact path/MIME/size/hash attacks;
- Git publication bypass around reviewed-state preconditions;
- prompt/tool-result instructions attempting to alter policy.

## Validation policy

Applicable PRs must pass the repository-defined gates before merge. Current gates include:

```bash
cd backend
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm run lint
npm run test:unit
npm run build

cd ..
npx playwright test --project=chromium
```

Repository CI additionally covers Windows plugin lifecycle behavior, Windows desktop binding contracts, Helm/deployment validation, Security Scan/CodeQL/dependency audit, and applicable Linux amd64/arm64 container builds.

A phase is `COMPLETE` only when its stated enforcement properties are implemented, validated, and—when intended for the default branch—actually merged to `main`. Partial, feature-gated, compatibility, platform-limited, or audit-known behavior remains `IN PROGRESS` or `BLOCKED`.

## Progress log

- **2026-08-12 — #118:** recovered the cumulative sandbox stack onto current `main`, repaired runtime/Broker/tool integration defects, passed the full gate set, and merged as `a216323e`.
- **2026-08-12 — #119:** added persistent extension confinement policy, passed the full gate set, and merged as `dd91b246`.
- **2026-08-12 — #121:** closed without merge as stale after `main` advanced.
- **2026-08-12 — #125:** replayed Phase 11 from `054a235a`; manual API audit additionally narrowed workspace change history to a dedicated safe DTO. Validation in progress.
