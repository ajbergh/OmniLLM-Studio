# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and security constraints live in:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md`

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens/merges, an enforcement limitation changes, or CI evidence changes.

## Recovery resolution — 2026-08-12

A live `main` reconciliation found that the earlier stacked sandbox PRs had not actually landed even though prior progress summaries described the implementation as merged. Only the architecture/tracker foundation and subprocess-hardening work were on `main`. Unrelated Image Studio work subsequently advanced `main`, leaving the original sandbox stack conflict-prone.

Recovery PR **#118** rebuilt the cumulative sandbox implementation directly on the then-current `main`, repaired integration defects discovered during manual audit, passed the full repository validation set, and was squash-merged as **`a216323e512fbecb1aa0c7c14df866f85ef76eb0`**.

Validated #118 evidence:

- backend canonical formatting;
- `go vet ./...`;
- full backend tests;
- race detector;
- frontend lint, unit tests, and build;
- Windows plugin lifecycle tests;
- Windows desktop binding contract tests;
- full Chromium Playwright smoke suite;
- Helm lint/render checks;
- Go and JavaScript/TypeScript CodeQL;
- dependency vulnerability audit;
- frontend and backend Linux amd64/arm64 container builds.

Historical stacked PRs #101, #104, #105, #107, #108, #109, #110, and #111 were closed as superseded after #118 merged. They are historical implementation slices only and must not be merged independently.

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
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | **COMPLETE** | merged PR #118 | Broker-issued `sbx_` IDs, exact owner/TTL checks, capability negotiation, authenticated HTTP runtime, bounded protocol, and artifact-ID trust model are validated and on `main`. |
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | merged implementation in #118 | Bubblewrap/rootfs runtime and authenticated `sandboxd` worker are on `main`; trusted mounts and runtime TTL cleanup are repaired. Exit still requires a native Bubblewrap isolation fixture/packaging, resource quotas, and enforceable egress where claimed. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **COMPLETE** | merged PR #99 | Ambient `os.Environ()` inheritance removed; shared sanitized runner and secret-leak regression tests are on `main`. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | **COMPLETE** | merged PR #118 | Legacy unauthenticated `/v1/execute` path retired; both tools use owner-bound Broker sessions; restricted Python has no host-Python fallback. |
| 5 | Workspace registry + RO/RW-no-delete/RW grants + durable journal | P1 | **IN PROGRESS** | merged implementation in #118 | Opaque owner-scoped grants, canonical roots, state-bound atomic mutations, before/after hashes, bounded snapshots, and reverts are on `main`. Residual path-component TOCTOU/rename-swap assurance remains before declaring filesystem confinement complete. |
| 6 | Workspace list/search/read/write/apply-patch/delete/revert tools | P1 | **IN PROGRESS** | merged implementation in #118 | Governed tool family is on `main`, defaults high-risk mutations to Ask, hides host roots, and uses the journaled filesystem layer. Completion tracks Phase 5 containment assurance. |
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | merged implementation in #118 | Explicit argv terminal execution, owner-bound sessions, read-only project mounts, runtime TTL cleanup, wall/output limits, and cancellation are on `main`. Memory/CPU/PID/disk limits remain intentionally false until enforced. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | merged implementation in #118 | Owner-bound `sng_` grants, operator domain/port policy, terminal grant consumption, and explicit `network_allowlist` capability fail-close are on `main`. First-party Linux destination-enforced egress remains unimplemented. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | merged implementation in #118 | Host-side opaque `sch_` handles, owner/TTL checks, and arbitrary-sandbox credential/auth-agent/proxy environment rejection are on `main`. Service-specific credential-broker consumers remain. |
| 10 | Full local plugin + stdio MCP OS sandbox migration | P1 | **IN PROGRESS** | `agent/sandbox-extension-confinement-main-20260812` | Fresh post-#118 branch adds persistent extension confinement policy without rewriting streaming MCP/plugin lifecycles. Linux can use Bubblewrap/rootfs in `auto`/`required` modes; non-Linux `required` fails closed. Exit: fresh PR, full CI/security/container validation, then merge. Windows/macOS native confinement remains Phases 12/13. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | NOT STARTED | after Phase 10 | UI/API must expose workspace grants, runtime state/capabilities, network grants, approvals, execution state, recent changes, and reversible changes without exposing host roots. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted identity/token, Job Object/process-tree confinement, ACL-scoped workspace, and Windows-native CI evidence required. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and macOS-native CI evidence required. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, no arbitrary tenant execution in API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99, #118, Phase 10 onward | Coverage includes ambient-secret leakage, owner/session replay, capability fail-close, trusted mount resolution, path traversal/symlink checks, stale mutations/reverts, destination grants, runtime cleanup, and extension confinement policy modes. Expand on every phase. |

## Current execution order

1. Open and validate the fresh Phase 10 PR from `agent/sandbox-extension-confinement-main-20260812`.
2. Close stale pre-recovery Phase 10 PR #117 after the fresh PR exists.
3. Merge Phase 10 only after Quality Gate, Security Scan, Windows compatibility tests, full Chromium smoke, Helm, and applicable container checks pass.
4. Implement Phase 11 API/desktop Settings UX on its own branch from the then-current `main`.
5. Implement/validate Windows and macOS confinement independently; native evidence is mandatory before either phase becomes complete.
6. Add durable tasks, dedicated server/Kubernetes workers, and multi-agent worktree isolation as separate P2 slices.
7. Continue adversarial containment work, including workspace path-component TOCTOU/rename-swap testing and remediation.

## Enforcement notes on current `main`

### Control plane

- Tool Executor Allow/Ask/Deny remains distinct from sandbox technical confinement.
- Broker session IDs and network/credential handles are references, not authorization.
- Reusable handles are scoped to the application owner context and have TTLs.
- Runtime capability negotiation is fail-closed; accepting a field is not evidence a runtime enforces the requested control.
- Sandbox-dependent tools remain registered but unavailable until the authenticated Broker is composed; high-risk side-effecting tools default to `ask`.

### Filesystem

- Model tools never receive configured host roots.
- Workspace-relative paths reject absolute/traversal paths, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is not approximated inside an arbitrary POSIX shell; terminal access narrows that grant to read-only.
- `terminal_exec` requests read-only project mounts in the current release, keeping source writes in the journaled workspace-tool path.
- A residual TOCTOU class remains if an independently writable parent path component is swapped between validation and mutation; this is tracked under Phase 5/17 rather than overstated as solved.

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

Currently represented as enforceable on `main`:

- OS/process namespace isolation;
- filesystem isolation through an operator-provided read-only rootfs and explicit workspace/scratch mounts;
- network namespace isolation/no-network mode;
- process-tree/session confinement;
- runtime session TTL cleanup;
- wall-time and stdout/stderr limits.

Still intentionally **not advertised** until implemented and natively validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota;
- destination allowlist egress.

## Phase 10 design notes

The Phase 10 branch preserves the existing long-lived stdio streaming lifecycle and changes only the shared process-construction seam.

- `OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off` controls policy; default is `auto`.
- Linux `auto` uses Bubblewrap only when `OMNILLM_SANDBOX_ROOTFS` is configured; otherwise it preserves the current sanitized host boundary.
- Linux `required` fails closed without configured rootfs/Bubblewrap.
- Windows/macOS `auto` remain on the sanitized boundary until Phases 12/13; `required` fails closed rather than overclaiming native isolation.
- `off` is an explicit sanitized-host compatibility mode; ambient backend secrets remain stripped.
- When native extension confinement is active, credential-sensitive explicit environment entries are rejected by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is the narrow operator compatibility override until service-specific broker consumers exist.
- Linux confinement uses an immutable rootfs, private namespaces/session, private tmp/home, read-only extension/working-directory exposure, cleared environment, and no network.

Phase 10 is not `COMPLETE` merely because Linux confinement exists: cross-platform native confinement and persistent destination-scoped egress are tracked separately and must not be inferred from the Linux implementation.

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

Applicable PRs must pass the repository-defined gates before merge. The current contributor contract includes backend formatting/vet/test/race validation, frontend lint/unit/build, Windows plugin lifecycle coverage, Windows desktop bindings, the full Chromium smoke suite, Helm/deployment checks, Security Scan, and container validation where applicable.

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

A phase is `COMPLETE` only when its stated enforcement properties are implemented, validated, and—when the phase is intended for the default branch—actually merged to `main`. Partial, feature-gated, compatibility, platform-limited, or audit-known behavior remains `IN PROGRESS` or `BLOCKED` rather than being overstated.
