# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and security constraints live in:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md`

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens/merges, an enforcement limitation changes, or CI evidence changes.

## Recovery note — 2026-08-12

A live `main` reconciliation found that the earlier stacked sandbox PRs had not actually landed even though prior progress summaries described the implementation as merged. Only the architecture/tracker foundation and subprocess-hardening work were on `main`. Unrelated Image Studio work subsequently advanced `main`, leaving the original sandbox stack conflict-prone.

PR **#118** (`agent/sandbox-recovery-current-main-20260812`) is now the authoritative recovery integration branch for Phases 1–2 and 4–9. It was rebuilt directly from the current `main` lineage and overlays only the cumulative sandbox implementation files, preserving unrelated Image Studio work and this current roadmap. The earlier stacked PRs #101, #104, #105, #107, #108, #109, #110, and #111 are retained only as historical implementation slices until #118 validates and merges; then they should be closed as superseded.

No phase represented by #118 is marked `COMPLETE` until repository CI/security gates pass and #118 is actually merged to `main`.

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
11. Local plugins and stdio MCP ultimately use the same OS-confinement principles as arbitrary code/terminal execution.
12. Multi-user deployments never run arbitrary tenant code in the primary API process/container.

## Roadmap status

Status values: `NOT STARTED`, `IN PROGRESS`, `COMPLETE`, `BLOCKED`.

| Phase | Scope | Priority | Status | Branch / PR | Current evidence / next exit criterion |
|---|---|---:|---|---|---|
| 0 | Architecture, threat model, durable roadmap | P0 | **COMPLETE** | merged PR #98 | Architecture, threat model, and tracker are on `main`; #98 passed Quality Gate and Security Scan before merge. |
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | **IN PROGRESS** | PR #118 (recovered from #101) | Broker-issued `sbx_` IDs, exact owner/TTL checks, capability negotiation, authenticated HTTP runtime, bounded protocol, and artifact-ID trust model are present in #118. Exit: all required gates pass and #118 merges. |
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | PR #118 (recovered from #104) | Bubblewrap/rootfs runtime and authenticated `sandboxd` worker are present. Native isolation fixture, packaging, quotas, and enforceable egress remain separate exit criteria; do not overclaim unsupported controls. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **COMPLETE** | merged PR #99 | Ambient `os.Environ()` inheritance removed; shared sanitized runner and secret-leak regression tests are on `main`. Quality, Security, and container gates passed before merge. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | **IN PROGRESS** | PR #118 (recovered from #105) | Legacy unauthenticated `/v1/execute` path retired; `code_execute` uses owner-bound Broker sessions; restricted Python has no host-Python fallback. Exit: #118 validation + merge. |
| 5 | Workspace registry + RO/RW-no-delete/RW grants + durable journal | P1 | **IN PROGRESS** | PR #118 (recovered from #107) | Owner-scoped opaque filesystem grants, canonical roots, before/after hashes, bounded revert snapshots, and state-checked revert primitives are present. Exit: #118 validation + merge. |
| 6 | Workspace list/search/read/write/apply-patch/delete/revert tools | P1 | **IN PROGRESS** | PR #118 (recovered from #108) | Model-facing tools use opaque workspace IDs/relative paths; mutation tools are high-risk, atomic, state-bound, and journaled. Exit: #118 validation + merge. |
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | PR #118 (recovered from #109) | High-risk explicit argv terminal tool uses Broker with no network and optional read-only workspace mount. Wall/output limits are implemented; memory/CPU/PID/disk flags remain false until enforced. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | PR #118 (recovered from #110) | Owner-bound `sng_` grants, operator domain/port policy, runtime `network_allowlist` capability, and fail-closed terminal consumption are present. First-party Linux destination-enforced egress remains unimplemented. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | PR #118 (recovered from #111) | Host-side opaque `sch_` credential handles and owner/TTL checks are present; arbitrary sandbox env rejects credential/auth-agent/proxy escape keys. Service-specific broker consumers remain. |
| 10 | Full local plugin + stdio MCP OS sandbox migration | P1 | NOT STARTED | new branch after #118 | Phase 3 sanitizes host subprocesses, but persistent streaming extension processes are not yet OS-confined on authoritative `main`. A prior #117 branch is stale and must not be treated as completed implementation. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | NOT STARTED | after Phase 10 | UI/API must expose workspace grants, runtime state/capabilities, network grants, approvals, execution state, recent changes, and reversible changes without exposing host roots. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted identity/token, Job Object/process-tree confinement, ACL-scoped workspace, and Windows-native CI evidence required. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and macOS-native CI evidence required. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, no arbitrary tenant execution in API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99 and #118 onward | Coverage includes environment-secret leakage and recovered tests for owner/session replay, path traversal/symlink checks, stale mutations/reverts, destination grants, and capability fail-close. Expand on every phase. |

## Current execution order

1. **Validate and merge #118** against the current `main` branch.
2. After #118 merges, verify its exact merge SHA and update this tracker on `main`.
3. Close #101, #104, #105, #107, #108, #109, #110, and #111 as **superseded by #118**; do not merge them independently afterward.
4. Rebuild Phase 10 on a fresh branch from the then-current `main`; do not reuse stale #117 as evidence of implementation.
5. Implement Phase 11 API/desktop Settings UX on its own PR.
6. Implement/validate Windows and macOS confinement independently; native evidence is mandatory before either phase becomes complete.
7. Add durable tasks, dedicated server/Kubernetes workers, and multi-agent worktree isolation as separate P2 slices.

## Recovered enforcement notes in #118

### Control plane

- Tool Executor Allow/Ask/Deny remains distinct from sandbox technical confinement.
- Broker session IDs and network/credential handles are references, not authorization.
- Reusable handles are scoped to the application owner context and have TTLs.
- Runtime capability negotiation is fail-closed; accepting a field is not evidence a runtime enforces the requested control.

### Filesystem

- Model tools never receive configured host roots.
- Workspace-relative paths reject absolute/traversal paths, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is not approximated inside an arbitrary POSIX shell; terminal access narrows that grant to read-only.
- `terminal_exec` requests read-only project mounts in the recovered release, keeping source writes in the journaled workspace-tool path.

### Network

- Default is no network.
- Network authorization requires an operator destination policy plus a high-risk owner-bound grant.
- IP literals and localhost are rejected from the grant surface.
- A runtime must separately advertise destination-allowlist enforcement; generic namespace isolation is not considered equivalent.
- The first-party Bubblewrap runtime therefore remains no-network until a real egress-enforcement mechanism lands.

### Credentials

- Arbitrary sandbox environments reject credential-bearing keys, SSH/Git auth delegation, cloud credential-file pointers, and proxy variables.
- Credential handles carry no secret values and cannot be redeemed by a generic model tool.
- Existing guarded Git/GitHub operations remain host-side, aligning with the credential-broker model.

### Linux runtime

Currently represented as enforceable by the recovered implementation:

- OS/process namespace isolation;
- filesystem isolation through an operator-provided read-only rootfs and explicit workspace/scratch mounts;
- network namespace isolation/no-network mode;
- process-tree/session confinement;
- wall-time and stdout/stderr limits.

Still intentionally **not advertised** until implemented and natively validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota;
- destination allowlist egress.

## Superseded-PR rule

The older stacked PRs remain open only until #118 proves the recovered cumulative implementation is healthy on current `main`. They are historical evidence, not merge candidates. Once #118 merges, close them with a note pointing to #118 and delete obsolete branches only after confirming no unique commits remain outside #118.

## Security acceptance categories

Every relevant phase expands negative tests for:

- traversal, absolute paths, symlink/junction/reparse/hard-link and rename escape;
- orphan/daemon descendants, fork/process abuse and cancellation escape;
- CPU/memory/disk/file-count/output exhaustion;
- localhost/private/link-local/metadata/DNS-rebinding/proxy/network bypass;
- backend/provider/GitHub/master/session/browser/SSH/cloud credential access;
- cross-user/workspace/conversation/run/sandbox/artifact/grant references;
- artifact path/MIME/size/hash attacks;
- Git publication bypass around reviewed state preconditions;
- prompt/tool-result instructions attempting to alter policy.

## Validation policy

Applicable PRs must pass the repository-defined gates before merge. The current contributor contract includes backend formatting/vet/test/race validation, frontend lint/unit/build, Windows plugin lifecycle coverage, the full Chromium smoke suite, Helm/deployment checks, Security Scan, and container validation where applicable.

Representative local commands remain:

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

Security-sensitive runtime/deployment work additionally requires Security Scan and applicable container/deployment checks. Platform confinement is not considered complete from cross-compilation alone; native isolation tests are required for each supported OS.

## Progress-update rule

A phase is `COMPLETE` only when its stated enforcement properties are implemented, validated, and—when the phase is intended for the default branch—actually merged to `main`. Partial, feature-gated, compatibility, recovered-but-unmerged, or platform-limited behavior remains `IN PROGRESS` or `BLOCKED` rather than being overstated.
