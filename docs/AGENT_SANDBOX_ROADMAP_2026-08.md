# Agent Sandbox Parity Program — August 2026

> **Status:** ACTIVE
>
> **Program goal:** Evolve OmniLLM-Studio from tool-governed local execution into a first-class, OS-enforced agent workspace runtime with the practical safety and workflow properties expected from modern coding/desktop agent products.

Detailed design and security constraints live in:

- `docs/AGENT_SANDBOX_ARCHITECTURE.md`
- `docs/AGENT_SANDBOX_THREAT_MODEL.md`
- `docs/SANDBOX_RUNTIME.md` (introduced in the code-runtime convergence stack)

This file is the durable implementation tracker. Update it whenever a phase changes status, a PR opens/merges, an enforcement limitation changes, or CI evidence changes.

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
| 1 | Sandbox Protocol v2 + backend-issued ownership-bound sessions | P0 | **IN PROGRESS** | PR #101 | Broker-issued `sbx_` IDs, exact owner/TTL checks, capability negotiation, authenticated HTTP runtime, bounded protocol, artifact-ID trust model. Merge/CI stack collapse remains. |
| 2 | First-party runtime abstraction + Linux execution plane | P1 | **IN PROGRESS** | PR #104 | Bubblewrap/rootfs runtime and authenticated `sandboxd` worker implemented. Native isolation fixture, packaging, and later quota/egress controls remain. |
| 3 | Immediate subprocess hardening for stdio MCP/plugins | P0 | **IN PROGRESS** | PR #99 | Ambient `os.Environ()` inheritance removed; shared sanitized runner and secret-leak regression tests implemented. Implementation head previously passed Quality/Security/Container; merge-stack collapse remains. |
| 4 | Route `code_execute` + `python_analysis` through Broker | P1 | **IN PROGRESS** | PR #105 | Legacy unauthenticated `/v1/execute` path retired in stack; `code_execute` uses owner-bound Broker sessions; restricted Python has no host-Python fallback. |
| 5 | Workspace registry + RO/RW-no-delete/RW grants + durable journal | P1 | **IN PROGRESS** | PR #107 | Owner-scoped opaque filesystem grants, canonical roots, before/after hashes, bounded revert snapshots, state-checked revert primitives implemented. |
| 6 | Workspace list/search/read/write/apply-patch/delete/revert tools | P1 | **IN PROGRESS** | PR #108 | Model-facing tools use only opaque workspace IDs/relative paths; mutation tools are high-risk, atomic, state-bound, and journaled. |
| 7 | `terminal_exec` + cancellation + runtime resource controls | P1 | **IN PROGRESS** | PR #109 | High-risk explicit argv terminal tool uses Broker with no network and optional read-only workspace mount. Wall/output limits implemented; memory/CPU/PID/disk flags remain false until enforced. |
| 8 | Network broker + destination approvals | P1 | **IN PROGRESS** | PR #110 | Owner-bound `sng_` grants, operator domain/port policy, runtime `network_allowlist` capability, fail-closed terminal consumption. First-party Linux egress allowlist enforcement remains. |
| 9 | Credential broker + raw-secret environment rejection | P1 | **IN PROGRESS** | PR #111 | Host-side opaque `sch_` credential handles and owner/TTL checks implemented; arbitrary sandbox env rejects credential/auth-agent/proxy escape keys. Service-specific broker consumers remain. |
| 10 | Full local plugin + stdio MCP OS sandbox migration | P1 | **IN PROGRESS** | next branch | Sanitized host execution exists from Phase 3; persistent streaming subprocess confinement is the next implementation slice. |
| 11 | Desktop workspace/sandbox UX + change review | P1 | NOT STARTED | TBD | UI must expose workspace grants, runtime state, network grants, approvals, execution state, and reversible changes. |
| 12 | Windows native confinement backend | P1 | NOT STARTED | TBD | Restricted identity/token, Job Object/process-tree confinement, ACL-scoped workspace, and Windows-native CI evidence required. |
| 13 | macOS native confinement backend | P1 | NOT STARTED | TBD | OS-enforced file/network/process confinement and macOS-native CI evidence required. |
| 14 | Durable sandbox-backed agent tasks | P2 | NOT STARTED | TBD | Persist sandbox/task association and support pause/resume/cancel/checkpoint/recovery/scheduling without weakening ownership. |
| 15 | Server/Kubernetes sandbox workers | P2 | NOT STARTED | TBD | Separate worker identity/pods, quotas, hardened security context, network policy, no arbitrary tenant execution in API pod. |
| 16 | Multi-agent isolated worktrees/workspaces | P2 | NOT STARTED | TBD | Independent writable workspaces/worktrees with reviewed promotion/reconciliation. |
| 17 | Adversarial sandbox assurance suite | Continuous | **IN PROGRESS** | #99 onward | Coverage already includes environment-secret leakage, owner/session replay, path traversal/symlink checks, stale mutations/reverts, destination grants, and capability fail-close; expand on every phase. |

## Active implementation stack

The program is intentionally decomposed into small security-reviewable PRs. Current logical order:

1. #98 — architecture/threat model/tracker — **merged**
2. #99 — local subprocess environment hardening
3. #101 — protocol-v2 Broker/control plane
4. #104 — first-party Linux Bubblewrap worker
5. #105 — code/restricted-Python Broker convergence
6. #107 — filesystem workspace registry/journal
7. #108 — governed coding workspace tools
8. #109 — isolated terminal with read-only project mounts
9. #110 — owner-bound destination network grants
10. #111 — host-side credential broker and secret-environment rejection
11. persistent plugin/MCP OS confinement
12. desktop UX and platform-specific native runtimes
13. durable/remote workers and multi-agent isolation

Retarget each child PR to `main` after its parent squash-merges. When retargeting, synchronize this tracker from current `main` so a child branch never reverts newer progress documentation.

## Implemented enforcement notes

### Control plane

- Tool Executor Allow/Ask/Deny remains distinct from sandbox technical confinement.
- Broker session IDs and network/credential handles are references, not authorization.
- Every reusable handle is scoped to the exact application owner context and has a TTL.
- Runtime capability negotiation is fail-closed; a worker cannot satisfy a requirement merely by accepting a field.

### Filesystem

- Model tools never receive configured host roots.
- Workspace relative paths reject absolute/traversal, symlink components, and direct `.git` access.
- Small-file mutations are atomic and journaled with before/after hashes.
- Patch/delete/revert flows are stale-state bound.
- `read_write_no_delete` is not approximated inside an arbitrary POSIX shell; terminal access narrows that grant to read-only.
- `terminal_exec` currently requests read-only project mounts even for broader stored grants, keeping source writes in the journaled workspace tool path.

### Network

- Default is no network.
- Network authorization requires an operator destination policy plus a high-risk owner-bound grant.
- IP literals and localhost are rejected from the grant surface.
- A runtime must separately advertise true destination allowlist enforcement; generic namespace isolation is not considered equivalent.
- The first-party Bubblewrap runtime therefore remains no-network until a real egress enforcement mechanism lands.

### Credentials

- Arbitrary sandbox environments reject credential-bearing keys, SSH/Git auth delegation, cloud credential file pointers, and proxy variables.
- Credential handles carry no secret values and cannot be redeemed by a generic model tool.
- Existing guarded Git/GitHub operations remain host-side, aligning with the credential-broker model.

### Linux runtime

Currently advertised as enforced:

- OS/process namespace isolation;
- filesystem isolation through an operator-provided read-only rootfs and explicit workspace/scratch mounts;
- network namespace isolation/no-network mode;
- process-tree/session confinement;
- wall-time and stdout/stderr limits.

Still intentionally **not advertised** until implemented and validated:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota;
- destination allowlist egress.

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

Applicable PRs must pass the repository gates before merge:

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

A phase is `COMPLETE` only when its stated enforcement properties are implemented and validated. Partial, feature-gated, compatibility, or platform-limited behavior remains `IN PROGRESS` or `BLOCKED` rather than being overstated.
