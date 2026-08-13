# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and operator documentation:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md`
- `docs/AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md`
- `docs/AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md`

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens or merges, an enforcement limitation changes, or validation evidence changes.

## Current checkpoint — 2026-08-13

At the Phase 12E completion checkpoint, the authoritative default branch is `main` at **`65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`**. Phase 12D PR #149 is the latest merged sandbox milestone; its exact final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed the complete Quality Gate, Security Scan, native Windows confinement/adversarial suite, plugin/desktop compatibility, backend race, Chromium Playwright, Helm, and applicable multi-architecture container builds before squash merge.

Validated sandbox lineage now includes:

- **PR #118**, squash-merged as `a216323e512fbecb1aa0c7c14df866f85ef76eb0`, recovered the cumulative sandbox implementation onto then-current `main` and repaired integration defects found during manual audit.
- **PR #119**, squash-merged as `dd91b246736451fafc498659fa582ff605e1bf16`, added persistent extension confinement policy for local plugins and stdio MCP without rewriting their streaming lifecycle.
- **PR #125**, squash-merged as `87727495bfa51dc12b2e00a7b9317039e4fd0ca9`, added the desktop sandbox/workspace Settings experience, safe owner-scoped review APIs, native workspace selection, and direct-loopback path-grant hardening.
- **PR #127**, squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`, added Windows-native restricted-token/per-sandbox-SID primitives, kill-on-close Job Objects, ACL helpers, and behavior-level native confinement evidence.
- **PR #128**, squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`, added the first-party Windows AppContainer runtime with creation-time Job membership, default-deny network, bounded read-only staging, and native lifecycle/confinement evidence.
- **PR #139**, squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`, moved persistent Windows stdio MCP/plugin processes onto the native AppContainer/Job-backed confinement path while preserving their streaming lifecycle.
- **PR #149**, squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`, added the dedicated Phase 12D native Windows adversarial matrix and policy-mode evidence.

Historical stacked PRs #101, #104, #105, #107, #108, #109, #110, and #111 were closed as superseded after #118 merged. Stale Phase 11 PR #121 was closed after its reviewed implementation was replayed cleanly as #125. Phase 12C development PRs #134 and #137 were closed without merge after they exposed formatting and unrelated-workflow-diff issues; the corrected implementation merged through #139. Phase 12D diagnostic PR #148 was superseded by clean validated #149.

## Non-negotiable invariants

1. Arbitrary model-generated processes never execute in the OmniLLM backend process.
2. Sandboxed/local extension processes do not inherit ambient backend secrets or the backend environment by default.
3. Models never supply physical host paths for sandbox mounts; application-owned workspace IDs are used instead.
4. Sandbox IDs are application-issued references and every operation revalidates user/workspace/conversation/run ownership.
5. Filesystem access is explicit: `read_only`, `read_write_no_delete`, or `read_write`.
6. Network is denied by default and may be widened only within operator policy and an enforceable runtime.
7. Descendants inherit sandbox restrictions and cancellation/destruction tears down the execution process tree.
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
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | implementation merged in #118 | Bubblewrap/rootfs runtime, authenticated `sandboxd`, trusted mounts, and runtime TTL cleanup are on `main`. Exit still requires packaged/native worker distribution and the remaining resource/egress work tracked below. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **COMPLETE** | merged PR #99 | Ambient `os.Environ()` inheritance removed; shared sanitized runner and secret-leak regression tests are on `main`. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | **COMPLETE** | merged PR #118 | Legacy unauthenticated execution path retired; both tools use owner-bound Broker sessions; restricted Python has no host-Python fallback. |
| 5 | Workspace registry + RO/RW-no-delete/RW grants + durable journal | P1 | **IN PROGRESS** | implementation merged in #118 | Opaque owner-scoped grants, canonical roots, state-bound atomic mutations, before/after hashes, bounded snapshots, and reverts are on `main`. Residual path-component TOCTOU/rename-swap assurance remains outside the Windows staged-copy flow. |
| 6 | Workspace list/search/read/write/apply-patch/delete/revert tools | P1 | **IN PROGRESS** | implementation merged in #118 | Governed tool family is on `main`, high-risk mutations default to Ask, host roots stay hidden, and mutations use the journaled filesystem layer. Completion tracks Phase 5 containment assurance. |
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | implementation merged in #118; Windows runtime #128 merged; issue #151 open | Explicit argv execution, owner-bound sessions, read-only project mounts, TTL cleanup, wall/output limits, context cancellation, and process-tree teardown are on `main`. Memory/CPU/PID/disk quotas remain intentionally false. Issue #151 tracks the separate explicit execution-ID cancellation addressability defect. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | implementation merged in #118 | Owner-bound `sng_` grants, operator domain/port policy, grant consumption, and `network_allowlist` capability fail-close are on `main`. First-party destination-enforced egress remains unimplemented; current Linux/Windows first-party runtimes remain intentionally no-network. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | implementation merged in #118 | Host-side opaque `sch_` handles, owner/TTL checks, and arbitrary-sandbox credential/auth-agent/proxy environment rejection are on `main`. Service-specific credential consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy migration | P1 | **COMPLETE** | #119 + Windows #139 merged | Shared process-construction seam supports `auto|required|off`; Linux can use Bubblewrap/rootfs and Windows uses native AppContainer/Job confinement when available. Required mode fails closed. macOS still preserves the sanitized compatibility boundary until Phase 13. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | **COMPLETE** | merged PR #125 (`87727495`) | Safe status/workspace/change APIs, Wails folder picker, opaque grant management, capability truth, review-only journal history, direct-loopback grant hardening, `/v1` client routing, and full CI/security/container validation are on `main`. |
| 12 | Windows native confinement backend | P1 | **COMPLETE** | #127, #128, #139, #149 merged | Native security primitives, first-party AppContainer runtime, persistent extension confinement, and dedicated adversarial evidence are all merged and natively validated. Phase 12E documents the scoped completion claim; cross-program gaps remain in Phases 2/5/7/8/9/17. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement plus macOS-native behavior evidence required; do not infer parity from Linux/Windows implementations. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, and no arbitrary tenant execution in the API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99, #118, #119, #125, #127, #128, #139, #149 onward | Windows Phase 12D is complete, but adversarial assurance remains continuous across future platform/runtime/workspace/credential changes. |

## Phase 11 — merged desktop UX and review boundary

PR #125 was replayed from then-current `main`, validated on final head `1789fcd582be46be679ac07965002c7f4e960095`, and squash-merged as `87727495bfa51dc12b2e00a7b9317039e4fd0ca9`.

Implemented surfaces include authenticated safe sandbox status/workspace/change APIs, safe DTOs, direct-loopback path-grant hardening, Wails folder selection, Agent Sandbox Settings UI, truthful capability badges, ephemeral physical-path handling, governed change review, and `/v1/sandbox/...` frontend routing.

Server/web deployments do not gain a generic remote filesystem picker. Operators must explicitly enable host-path grants, and creation remains direct-loopback-only.

## Phase 12 — Windows native confinement: COMPLETE

Detailed history lives in `docs/AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md`; the dedicated adversarial matrix is recorded in `docs/AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md`.

### 12A — native primitives (#127)

Implemented and natively proved:

- per-sandbox restricting SID generation;
- restricted primary-token creation;
- kill-on-close Job Objects;
- SID-scoped DACL helpers;
- cross-sandbox ACL denial under the same Windows account.

Manual review caught and fixed a first implementation that used globally reusable Restricted Code SID authority. Final hardened head `8b491a80de9543afcd259d5d5959794bb4a61eaa` passed the full applicable gate set and #127 merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.

### 12B — first-party AppContainer runtime (#128)

The merged runtime provides:

- one AppContainer profile/package SID per runtime session;
- zero AppContainer network capabilities;
- Job Object membership and inherited-handle restriction at child creation time;
- Job teardown on root completion/context cancellation/timeout/session destruction;
- minimal non-ambient environment;
- bounded wall/output limits;
- ephemeral writable sandbox workspace or bounded staged `read_only` host workspace;
- protected staged-workspace DACL and no host workspace ACL mutation;
- fail-closed writable arbitrary-process mounts;
- post-open `GetFinalPathNameByHandle` containment verification before staged bytes are copied;
- native assertions for AppContainer token state, workspace/host denial, ambient-secret absence, loopback denial, descendant teardown, and cleanup behavior.

Exact final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed complete validation before #128 squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.

Python and JavaScript language shortcuts in the Windows arbitrary-process runtime remain fail-closed until an AppContainer-readable interpreter/package design is natively validated. This is a feature-completion gap, not a host fallback.

### 12C — persistent Windows extension confinement (#139)

Persistent stdio MCP/plugin processes now use the same native Windows confinement principles while preserving their streaming JSON-RPC lifecycle:

- shared managed `CommandProcess` seam;
- `auto|required|off` policy behavior;
- AppContainer security capabilities, Job membership, and explicit inherited stdio handles bound at process creation;
- minimal non-ambient environment plus credential-sensitive explicit-env validation;
- read-only staging for non-System32 command bundles and working directories rather than host ACL widening;
- bounded absolute-path remapping under staged roots with unrelated host paths rejected;
- process-tree termination on cancellation, forced shutdown, and root completion;
- retryable cleanup for transient AppContainer profile removal failures.

Corrected final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed full validation before #139 squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.

### 12D — adversarial Windows assurance (#149)

The dedicated native matrix proves/exercises:

- per-extension AppContainer identity and cross-profile authority isolation;
- read-only staged extension bundles with writable sandbox-owned home;
- denial of unrelated host-file read/write;
- ambient backend-secret absence;
- default-deny loopback/network behavior;
- descendant Job teardown after root exit;
- context cancellation terminating the full Job process tree;
- hard-link and junction/reparse-point staging rejection;
- fail-closed unrelated absolute argument handling;
- credential-sensitive explicit environment rejection in `required` mode;
- `auto` and `required` choosing native Windows confinement while `off` selects the sanitized host boundary.

Exact final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed all repository gates before #149 squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

### 12E — completion decision

Windows native confinement Phase 12 is complete. The completion claim is scoped to OS/filesystem/network-default-deny/process-tree confinement and the persistent extension migration. It intentionally does **not** absorb broader incomplete roadmap work.

Issue #151 was discovered during Phase 12D: the explicit `Cancel(executionID)` protocol surface is not addressable by a synchronous caller because runtime execution IDs are returned only after `Exec` completes. Context cancellation, timeout, `Destroy`, and Windows Job teardown are implemented and natively tested. #151 remains a Phase 7/protocol lifecycle defect to repair separately.

## Current execution order

1. **Repair issue #151** as a small protocol/runtime lifecycle slice: make an execution reference caller-known at start time, preserve ownership, reject duplicate/invalid IDs, and prove explicit active-execution cancellation across local/HTTP runtimes.
2. **Start Phase 13 macOS native confinement** independently; do not reuse Windows/Linux capability claims without macOS-native evidence.
3. Continue **Phase 2 / 5 / 7 / 8 / 9** completion work: packaged worker distribution, quotas, broader workspace TOCTOU assurance, destination-enforced egress, and service-specific credential consumers.
4. Add durable tasks, dedicated server/Kubernetes workers, and multi-agent worktree isolation as separate P2 slices.
5. Keep Phase 17 adversarial assurance continuous across every future confinement, workspace, credential, and deployment change.

## Open enforcement gaps

### Filesystem

- Model tools do not receive configured host roots.
- Workspace-relative paths reject absolute/traversal paths, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is narrowed to read-only for arbitrary POSIX shell access rather than approximated unsafely.
- `terminal_exec` requests read-only project mounts so source writes remain in the journaled workspace-tool path.
- Windows read-only runtime/extension input is staged into AppContainer-owned storage rather than widening original host ACLs.
- Windows staging rejects reparse points and multiply-linked files and verifies opened source handles remain beneath the canonical source root before copy.
- **Still open:** broader Phase 5 workspace-registry/path-component TOCTOU outside those staged-copy flows, especially where independently writable namespace components can change between validation and use.

### Runtime resources

Implemented and advertised only where enforceable:

- OS/process isolation;
- filesystem isolation;
- default no-network isolation;
- process-tree/session confinement;
- session TTL cleanup;
- wall-time and stdout/stderr limits.

Still intentionally unadvertised until implemented and validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota.

### Execution lifecycle

- context cancellation, timeout, session destruction, and process-tree teardown work on current runtimes;
- **issue #151 remains open:** explicit `Cancel(executionID)` cannot currently be addressed while synchronous `Exec` is active because the execution ID is generated/returned inside the runtime result rather than being known before execution starts.

### Network

- Default is no network.
- Network authorization requires operator destination policy plus an owner-bound high-risk grant.
- IP literals and localhost are rejected from the grant surface.
- A runtime must separately advertise destination-allowlist enforcement; isolation alone is not equivalent to allowlisting.
- First-party Linux and Windows runtimes do not yet enforce destination-scoped egress and remain no-network even when a destination grant exists.

### Credentials

- Arbitrary sandbox environments reject credential-bearing keys, SSH/Git auth delegation, cloud credential-file pointers, and proxy variables.
- Opaque credential handles carry no secret values and are ownership/TTL scoped.
- Existing guarded Git/GitHub operations remain host-side.
- **Still open:** service-specific credential broker consumers before arbitrary sandbox tasks can use narrowly delegated external-service credentials.

### Persistent extensions

`OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off` remains the policy boundary:

- Linux `auto` uses Bubblewrap when a sandbox rootfs is configured;
- Windows `auto` uses native AppContainer/Job confinement when supported;
- `required` fails closed when native confinement is unavailable;
- `off` explicitly selects the sanitized host compatibility boundary;
- macOS `auto` still uses the sanitized compatibility boundary pending Phase 13;
- ambient backend secrets remain stripped in compatibility mode;
- native Linux/Windows confinement uses minimal/non-ambient environments and rejects credential-sensitive explicit environment by default;
- `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a narrow transitional operator override, not the desired long-term credential path.

## Security acceptance categories

Every relevant future phase expands negative tests for:

- traversal, absolute paths, symlink/junction/reparse/hard-link and rename escape;
- orphan/daemon descendants, process abuse, cancellation escape;
- CPU/memory/disk/file-count/output exhaustion;
- localhost/private/link-local/metadata/DNS-rebinding/proxy/network bypass;
- backend/provider/GitHub/master/session/browser/SSH/cloud credential access;
- cross-user/workspace/conversation/run/sandbox/artifact/grant references;
- artifact path/MIME/size/hash attacks;
- Git publication bypass around reviewed-state preconditions;
- prompt/tool-result instructions attempting to alter policy.

## Validation policy

Applicable PRs must pass repository-defined gates before merge. Current core commands include:

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

Repository CI additionally covers Windows plugin lifecycle, Windows desktop bindings, dedicated Windows sandbox confinement/adversarial behavior, Helm/deployment validation, Security Scan/CodeQL/dependency audit, and applicable Linux amd64/arm64 container builds.

A phase is `COMPLETE` only when its stated enforcement properties are implemented, validated, and merged. Partial, feature-gated, compatibility, platform-limited, or audit-known behavior remains `IN PROGRESS` or is explicitly tracked outside the completed phase scope.

## Progress log

- **2026-08-12 — #118:** cumulative sandbox stack recovered onto current `main`, integration defects repaired, full gates passed, merged as `a216323e`.
- **2026-08-12 — #119:** persistent extension confinement policy added, full gates passed, merged as `dd91b246`.
- **2026-08-12 — #121:** closed without merge as stale after `main` advanced.
- **2026-08-12 — #125:** Phase 11 replay; safe history serialization, loopback hardening, and `/v1` route correction completed; full validation passed and merged as `87727495`.
- **2026-08-12 — #127:** Windows SID/token/Job/ACL primitives; reusable Restricted Code SID defect corrected; merged as `c68ba013` after native/full validation.
- **2026-08-13 — #128:** first-party Windows AppContainer runtime final head `282fbc0f...` passed full validation and merged as `43c1c42b...`.
- **2026-08-13 — #139:** persistent Windows extension confinement final head `f8076939...` passed full validation and merged as `69590078...`.
- **2026-08-13 — #149:** Phase 12D native adversarial assurance final head `8f4ee1b7...` passed full validation and merged as `65bf1cd8...`.
- **2026-08-13 — #151:** protocol lifecycle issue opened for explicit execution-ID cancellation addressability.
- **2026-08-13 — Phase 12E:** Windows native confinement marked `COMPLETE`; next engineering slice is #151, followed by Phase 13 macOS confinement and remaining cross-program P1 work.
