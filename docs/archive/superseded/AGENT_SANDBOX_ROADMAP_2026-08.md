> **Archived — superseded.** This aggregate roadmap predates completed Windows Phase 12 and macOS Phases 13A–13C. Use [the current roadmap](../../AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md) and [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and operator documentation:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md`
- `docs/AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md`

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens or merges, an enforcement limitation changes, or validation evidence changes.

## Current checkpoint — 2026-08-12

At the latest Phase 12B integration checkpoint, the authoritative default branch was `main` at **`788df6e0944a1e2608203e79cd0d29e44eeb0875`**. The most recent merged sandbox milestone remains Phase 12A PR #127, squash-merged as **`c68ba013d3ad41ff2044646733d38ab981b3dc87`**; `main` has subsequently advanced with disjoint Image Studio and GitHub merge-policy work.

Sandbox lineage now has four validated integration milestones:

- **PR #118**, squash-merged as `a216323e512fbecb1aa0c7c14df866f85ef76eb0`, recovered the cumulative sandbox implementation onto the then-current `main` and repaired integration defects found during manual audit.
- **PR #119**, squash-merged as `dd91b246736451fafc498659fa582ff605e1bf16`, added persistent extension confinement policy for local plugins and stdio MCP without rewriting their streaming lifecycle.
- **PR #125**, squash-merged as `87727495bfa51dc12b2e00a7b9317039e4fd0ca9`, added the desktop sandbox/workspace Settings experience, safe owner-scoped review APIs, native workspace selection, and direct-loopback path-grant hardening.
- **PR #127**, squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`, added Windows-native restricted-token/per-sandbox-SID primitives, kill-on-close Job Objects, ACL helpers, and behavior-level `windows-latest` confinement evidence.

#118, #119, #125, and #127 each passed the applicable repository validation set before merge, including backend format/vet/tests/race, frontend lint/unit/build, Windows coverage, full Chromium Playwright smoke, Helm checks, Security Scan, and applicable Linux multi-architecture container validation.

Phase 12B is active in **PR #128** on branch `agent/sandbox-windows-runtime-12b-20260812`, clean-replayed directly from `main` `788df6e0` after the default branch advanced. Exact-final-head validation remains required before merge.

Historical stacked PRs #101, #104, #105, #107, #108, #109, #110, and #111 were closed as superseded after #118 merged. Stale Phase 11 PR #121 was closed after its reviewed implementation was replayed cleanly as #125.

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
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | implementation merged in #118; Windows work #128 | Explicit argv execution, owner-bound sessions, read-only project mounts, TTL cleanup, wall/output limits, and cancellation are on `main` for Linux. Memory/CPU/PID/disk limits remain intentionally false. PR #128 adds a Windows-native execution plane without widening those quota claims. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | implementation merged in #118 | Owner-bound `sng_` grants, operator domain/port policy, grant consumption, and `network_allowlist` capability fail-close are on `main`. First-party destination-enforced egress remains unimplemented; Windows #128 is intentionally no-network only. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | implementation merged in #118 | Host-side opaque `sch_` handles, owner/TTL checks, and arbitrary-sandbox credential/auth-agent/proxy environment rejection are on `main`. Service-specific credential consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy migration | P1 | **COMPLETE** | merged PR #119 (`dd91b246`) | Shared process-construction seam supports `auto|required|off`; Linux can use Bubblewrap/rootfs, required mode fails closed, and Windows/macOS preserve the sanitized compatibility boundary until native extension backends land. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | **COMPLETE** | merged PR #125 (`87727495`) | Safe status/workspace/change APIs, Wails folder picker, opaque grant management, capability truth, review-only journal history, direct-loopback grant hardening, `/v1` client routing, and full CI/security/container validation are on `main`. |
| 12 | Windows native confinement backend | P1 | **IN PROGRESS** | #127 merged; PR #128 active | 12A native token/SID/Job/ACL primitives are on `main`. 12B adds first-party AppContainer `NewLocalRuntime`, creation-time Job membership, explicit handles, default-deny network, RO staging, and behavior-level native tests. `fa22da80` passed the complete dedicated Windows confinement suite; exact-final-head full repository validation is pending after canonical formatting/replay. Completion still requires 12C extension confinement plus 12D adversarial evidence. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and macOS-native isolation evidence required. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, and no arbitrary tenant execution in the API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99, #118, #119, #125, #127, #128 onward | Expand negative coverage with every phase; native confinement cannot be declared complete from cross-compilation alone. |

## Phase 11 — merged implementation details

PR #125 was replayed from then-current `main`, validated on final head `1789fcd582be46be679ac07965002c7f4e960095`, and squash-merged as `87727495bfa51dc12b2e00a7b9317039e4fd0ca9`.

Implemented surfaces include authenticated safe sandbox status/workspace/change APIs, safe DTOs, direct-loopback path-grant hardening, Wails folder selection, Agent Sandbox Settings UI, truthful capability badges, ephemeral physical-path handling, governed change review, and `/v1/sandbox/...` frontend routing.

Server/web deployments do not gain a generic remote filesystem picker. Operators must explicitly enable host-path grants, and creation remains direct-loopback-only.

Final #125 validation evidence included backend format/vet/tests/race, frontend lint/unit/build, Windows plugin/desktop checks, full Chromium Playwright, Helm, Go and JavaScript/TypeScript CodeQL, dependency audit, and frontend/backend Linux amd64/arm64 container builds.

## Phase 12 — Windows native confinement

Detailed status lives in `docs/AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md`.

### Phase 12A — merged #127

Implemented and natively proved:

- per-sandbox restricting SID generation;
- restricted primary-token creation;
- kill-on-close Job Objects;
- SID-scoped DACL helpers;
- cross-sandbox ACL denial under the same Windows account.

Manual review caught and fixed a first implementation that used the globally reusable Restricted Code SID. Final hardened head `8b491a80de9543afcd259d5d5959794bb4a61eaa` passed the full applicable gate set and merged as `c68ba013`.

### Phase 12B — active #128

PR #128 proposes the first Windows `NewLocalRuntime` implementation using stable AppContainer/process-creation mechanisms:

- one AppContainer profile/package SID per runtime session;
- zero AppContainer network capabilities, so the runtime remains no-network;
- Job Object membership and inherited-handle restriction at child creation time;
- Job teardown on root completion/cancel/timeout;
- minimal non-ambient environment;
- bounded wall/output limits;
- ephemeral writable sandbox workspace or bounded staged `read_only` host workspace;
- protected staged-workspace DACL and no host workspace ACL mutation;
- fail-closed writable arbitrary-process mounts;
- post-open `GetFinalPathNameByHandle` containment verification before staged bytes are copied;
- native child assertions for AppContainer token state, read-only workspace, unrelated-host-file read/write denial, ambient-secret absence, loopback denial, and host-source immutability;
- native descendant teardown and active-`Destroy` synchronization coverage;
- `cmd/sandboxd` compilation/testing in the dedicated Windows sandbox job.

Head `fa22da808423b9cd47652d74b639f8eda2d052aa` passed the complete dedicated Windows confinement suite plus Windows plugin lifecycle, frontend/Helm, Security Scan, and applicable container builds. The full Quality Gate still failed because the new Windows source was not canonical `gofmt` output. Canonical formatting was then applied and the intended PR state was clean-replayed onto `main` `788df6e0`; that earlier native pass is diagnostic evidence only, and the exact final head must pass every gate before merge.

Python and JavaScript language shortcuts remain fail-closed in this Windows slice until an AppContainer-readable interpreter/package design is natively validated. This is an explicit feature-completion gap, not a hidden host fallback.

## Current execution order

1. Finish and natively validate Phase 12B PR #128 on its exact final head; merge only after full Quality, Security, Windows, Chromium/race, Helm, and applicable container checks pass.
2. Start Phase 12C from post-#128 `main`: confine persistent stdio MCP/plugin processes on Windows without a pre-confinement execution window or streaming regression.
3. Expand Phase 12D adversarial Windows evidence: descendant teardown, cross-sandbox authority reuse, reparse/hard-link/rename escape, network bypass, secret inheritance, and cancellation.
4. Mark Phase 12 complete only after first-party runtime and persistent extension surfaces both have native Windows evidence.
5. Implement and natively validate macOS confinement independently in Phase 13.
6. Continue Phase 2/5/7/8/9 enforcement work: runtime packaging, quotas, destination-enforced egress, workspace TOCTOU assurance, and service-specific credential consumers remain open.
7. Add durable tasks, dedicated server/Kubernetes workers, and multi-agent worktree isolation as separate P2 slices.
8. Keep the adversarial assurance suite continuous across every phase.

## Open enforcement gaps

### Filesystem

- Model tools do not receive configured host roots.
- Workspace-relative paths reject absolute/traversal paths, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is narrowed to read-only for arbitrary POSIX shell access rather than approximated unsafely.
- `terminal_exec` requests read-only project mounts so source writes remain in the journaled workspace-tool path.
- Windows #128 stages read-only workspace input into AppContainer-owned storage rather than widening ACLs on the original host workspace; writable arbitrary-process mounts remain fail-closed.
- Windows #128 also binds every opened staging source handle back to the canonical source root with `GetFinalPathNameByHandle` before copying, closing the specific checked-path/reparse-or-rename staging escape identified during review.
- **Still open:** the broader Phase 5 workspace-registry/path-component TOCTOU class outside the 12B staged-copy flow remains when independently writable namespace components can change between validation and use. Phase 5/17 remain incomplete until those paths are natively addressed/tested.

### Runtime resources

Currently represented as enforceable by the merged first-party Linux runtime:

- OS/process namespace isolation;
- filesystem isolation through read-only rootfs plus explicit trusted mounts;
- no-network namespace isolation;
- process-tree/session confinement;
- session TTL cleanup;
- wall-time and stdout/stderr limits.

PR #128 proposes equivalent Windows claims only where native AppContainer/Job behavior is tested: OS/filesystem/network/process-tree isolation plus wall/output/cancellation. It does not advertise resource quotas.

Still intentionally not advertised until implemented and validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota.

### Network

- Default is no network.
- Network authorization requires operator destination policy plus an owner-bound high-risk grant.
- IP literals and localhost are rejected from the grant surface.
- A runtime must separately advertise destination-allowlist enforcement; isolation alone is not equivalent to allowlisting.
- The first-party Linux runtime does not yet enforce destination-scoped egress and remains no-network even when a destination grant exists.
- Windows #128 is likewise intentionally no-network; destination allowlisting remains false/unimplemented.

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
- Windows/macOS `auto` retain the sanitized host compatibility boundary until their native extension backends land;
- ambient backend secrets remain stripped in compatibility mode;
- native Linux extension confinement clears the environment and rejects credential-sensitive explicit environment by default;
- `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a narrow transitional operator override, not the desired long-term credential path.

Phase 12B does not change this Windows extension behavior. That migration is Phase 12C.

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

Repository CI additionally covers Windows plugin lifecycle, Windows desktop bindings, dedicated Windows sandbox confinement behavior, Helm/deployment validation, Security Scan/CodeQL/dependency audit, and applicable Linux amd64/arm64 container builds.

A phase is `COMPLETE` only when its stated enforcement properties are implemented, validated, and—when intended for the default branch—actually merged to `main`. Partial, feature-gated, compatibility, platform-limited, or audit-known behavior remains `IN PROGRESS` or `BLOCKED`.

## Progress log

- **2026-08-12 — #118:** recovered the cumulative sandbox stack onto current `main`, repaired runtime/Broker/tool integration defects, passed the full gate set, and merged as `a216323e`.
- **2026-08-12 — #119:** added persistent extension confinement policy, passed the full gate set, and merged as `dd91b246`.
- **2026-08-12 — #121:** closed without merge as stale after `main` advanced.
- **2026-08-12 — #125:** replayed Phase 11 from current `main`; manual audit fixed safe change-history serialization and forwarded-address loopback spoofing. Playwright then exposed a missing `/v1` frontend route prefix; it was fixed and final validation passed before merge as `87727495`.
- **2026-08-12 — #127:** added native Windows SID/token/Job/ACL primitives. Manual audit replaced globally reusable Restricted Code SID authority with per-sandbox SIDs; hardened native/full gates passed and #127 merged as `c68ba013`.
- **2026-08-12 — #128:** opened first-party Windows AppContainer runtime. Head `fa22da80` passed the complete dedicated Windows confinement suite, plugin lifecycle, frontend/Helm, Security, and containers; Quality failed only on canonical formatting. The Windows files were then canonicalized and the intended nine-file PR state was clean-replayed from `main` `788df6e0`; exact-final-head validation remains pending.
