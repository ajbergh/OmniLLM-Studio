# Agent Sandbox Parity Program — Current Roadmap (August 2026)

> **Status:** ACTIVE
>
> **Checkpoint (2026-08-19):** Linux governed-workspace TOCTOU hardening is complete through #182. Darwin durable root identity (#183) and descriptor-relative governed reads/search/mutations (#210, merged as `dadb0372`) are complete. Dedicated sandbox-worker image packaging is complete in #214. CI Playwright package-manager quiescence hardening merged in #226 (`ffbf7971`). The remaining reviewed sandbox slices have been rebuilt directly on current `main`: Windows governed-workspace hardening #231, cumulative CPU enforcement #232, durable leased sandbox tasks #233, Kubernetes worker deployment #234, and isolated agent worktrees #235. These rebuilt PRs must pass their own exact-head gates before merge; closed stacked drafts #211/#215/#216/#217/#221/#223 are historical only.

## Program invariants

- Arbitrary model-directed processes do not execute inside the primary backend process.
- Owner scope is revalidated on sandbox operations; IDs are references, not authorization.
- Models do not receive physical host paths for workspace mounts.
- Ambient backend secrets are absent from arbitrary sandboxes and natively confined extensions.
- Network is denied by default and widened only when a runtime can enforce the requested policy.
- Descendants inherit confinement and are terminated on cancellation/session teardown where the runtime truthfully advertises process-tree isolation.
- Capability reporting is limited to controls actually enforced and proven.
- Guarded Git state/digest protections remain authoritative for reviewed publication workflows.
- Multi-user deployments do not run arbitrary tenant code in the primary API process/container.
- Restart recovery must never replay side effects while a predecessor runtime may still be alive.

## Current phase status

| Phase | Scope | Status | Current evidence / next exit |
|---|---|---|---|
| 0 | Architecture, threat model, durable roadmap | **COMPLETE** | Core design documents are on `main`. |
| 1 | Protocol v2 + owner-bound sessions | **COMPLETE** | Broker sessions, ownership/TTL checks, authenticated worker protocol, bounded results, and capability negotiation merged in #118. |
| 2 | First-party runtime abstraction + Linux execution plane | **IN PROGRESS** | Bubblewrap/rootfs runtime and `sandboxd` exist; #173/#174 provide delegated PID/memory enforcement; #214 packages the dedicated worker image. CPU enforcement is rebuilt in #232 but capability promotion remains gated. |
| 3 | Immediate stdio MCP/plugin subprocess hardening | **COMPLETE** | Ambient environment inheritance removed in #99. |
| 4 | Broker-backed `code_execute` + `python_analysis` | **COMPLETE** | Owner-bound execution; restricted Python has no unrestricted host fallback. |
| 5 | Workspace registry + grants + durable journal | **IN PROGRESS — FINAL PLATFORM SLICE** | Linux identity/read/search/mutation hardening is merged through #182. Darwin root identity (#183) and descriptor-relative operations (#210) are merged. Rebuilt Windows native identity/path-race hardening is #231. |
| 6 | Governed workspace tools | **IN PROGRESS — FINAL PLATFORM SLICE** | Read/write/patch/delete/revert/search tools exist. Linux and Darwin operations are bound to opened filesystem objects. #231 provides the Windows equivalent and must pass exact-head native assurance before this cross-platform item can close. |
| 7 | `terminal_exec` + cancellation + resource controls | **IN PROGRESS** | Explicit cancellation merged in #155; fail-closed quota admission in #170; Windows PID/memory #171/#172; Linux PID/memory #173/#174. Cumulative CPU primitives are rebuilt in #232. Disk accounting and macOS resource quotas remain open. |
| 8 | Network broker + destination approvals | **IN PROGRESS** | Owner-bound grants exist; first-party Linux/Windows/Darwin arbitrary runtimes remain no-network because destination-enforced egress is not implemented. |
| 9 | Credential broker | **IN PROGRESS** | Opaque owner/TTL handles and raw-secret environment rejection exist; service-specific consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy | **COMPLETE** | `auto|required|off` and shared managed-process seam are implemented; Linux uses Bubblewrap, Windows AppContainer, macOS Seatbelt. |
| 11 | Desktop sandbox/workspace UX | **COMPLETE** | Workspace grants, review APIs, Settings UX, and loopback grant hardening merged in #125. |
| 12 | Windows native confinement backend | **COMPLETE** | #127, #128, #139, and #149 provide protocol-v2 and persistent-extension AppContainer/Job confinement with native adversarial evidence. |
| 13 | macOS native confinement backend | **COMPLETE** | 13A Seatbelt (#159), 13B runtime (#162), 13C persistent extensions (#164), and 13D adversarial assurance (#166) are merged. Detached-descendant limitation remains explicit. |
| 14 | Durable sandbox-backed agent tasks | **IN PROGRESS / REBUILT** | Reviewed queue/recovery/executor work from closed #215 has been rebuilt directly on current `main` as #233. Lifecycle integration from closed #223 follows only after the durable contract merges. |
| 15 | Server/Kubernetes sandbox workers | **IN PROGRESS / REBUILT** | Worker image packaging merged in #214. Reviewed isolated Helm workload/network policy from closed #216 is rebuilt as #234. Durable worker lifecycle remains dependent on Phase 14. |
| 16 | Multi-agent isolated worktrees/workspaces | **IN PROGRESS / REBUILT** | Reviewed owner-scoped immutable-base/writable-snapshot worktree primitive from closed #217 is rebuilt as #235. Promotion remains digest-bound and does not bypass guarded Git publication controls. |
| 17 | Adversarial assurance suite | **IN PROGRESS** | Browser egress (#168), Windows and Linux quota negatives, Linux workspace race assurance, Darwin root/path-race assurance, and rebuilt Windows workspace assurance are part of the active matrix. Every new enforcement claim requires native negative evidence. |

## Completed platform lineage

### Windows Phase 12

- **12A / #127** — unique per-sandbox Windows authority, restricted-token primitives, Job Objects, DACL helpers, and cross-sandbox denial.
- **12B / #128** — first-party Windows AppContainer protocol-v2 runtime.
- **12C / #139** — persistent stdio MCP/plugin AppContainer confinement.
- **12D / #149** — direct adversarial Windows assurance.

### Cross-platform cancellation

**#155** made execution cancellation addressable before `Exec`: callers can provide a canonical execution reference; omitted IDs use the shared generator; active duplicates fail closed; HTTP and `sandboxd` preserve the reference; platform runtimes use it as the cancellation key.

### macOS Phase 13

- **13A / #159** — Seatbelt primitive, canonicalized roots, default network deny, explicit write-root proof.
- **13B / #162** — first-party Darwin local runtime.
- **13C / #164** — persistent stdio MCP/plugin Seatbelt confinement with `auto|required|off` semantics.
- **13D / #166** — adversarial detached process/session, path/symlink/rename, cross-runtime authority, and persistent-extension assurance.

## Governed-workspace filesystem status

### Linux — complete for current governed tools

- **#175:** file content opens traverse from an opened root using descriptor-relative `O_NOFOLLOW` operations and validate the final descriptor with `fstat`.
- **#176:** write/delete/revert pin the root and final parent directory, capture and mutate relative to that descriptor lineage, and use the same lineage for verification/rollback.
- **#177:** registered roots persist device+inode; fresh operations fail closed if the path now refers to a different object; legacy pathname-only grants require trusted re-registration.
- **#182:** search enumeration recurses from pinned directory descriptors, opens candidates without following symlinks, and reads from the same validated descriptor; global early-stop is preserved.

### Darwin — complete for current governed tools

- **#183:** registered roots persist device+inode from a no-follow opened directory and reject root replacement/legacy grants.
- **#210 (`dadb0372`):** governed reads, search, mutations, and rollback use no-follow descriptor-relative operations and pinned directory identities. Native macOS race/symlink/root-replacement assurance is included.

### Windows — rebuilt, validation active

**#231** is the clean current-main rebuild of the reviewed Windows slice. It:

- persists workspace root identity as volume serial + file ID from `GetFileInformationByHandle`;
- opens roots/directories with `FILE_FLAG_OPEN_REPARSE_POINT` and rejects directory reparse points;
- validates opened file handles before governed reads consume bytes;
- treats search enumeration names as untrusted hints and reopens candidates through the hardened read boundary;
- pins mutation parent handles and replaces via native `NtSetInformationFile(FileRenameInformation)` relative to the opened parent;
- deletes through the opened target handle using `FileDispositionInfo`;
- proves root replacement, legacy grant, reparse candidate, renamed-parent mutation, search-root replacement, and global early-stop behavior on Windows.

Phase 5/6 cross-platform filesystem hardening closes only after #231 passes its rebuilt exact-head matrix and merges.

## Resource controls

Currently enforced where applicable: OS/filesystem/no-network isolation, TTL cleanup, wall-time bounds, stdout/stderr bounds, platform-specific process teardown, Windows Job Object process-count and aggregate committed-memory limits, Linux delegated cgroup-v2 `pids.max`, and aggregate Linux `memory.max` with `memory.swap.max=0`.

Broker admission rejects a non-zero resource request if the selected runtime does not advertise the matching capability. Capability truthfulness is therefore part of the security boundary rather than presentation metadata.

### CPU

Rebuilt **#232** preserves the reviewed cumulative CPU contract work:

- Linux enables the delegated cgroup-v2 `cpu` controller additively when available.
- `cpu.stat usage_usec` is the aggregate process-tree accounting source.
- `cpu.max` is used only as an overshoot bound; it is not the application CPU-time contract.
- Whole execution cgroups can be terminated with `cgroup.kill`.
- A cumulative monitor and final accounting close sampling-window escapes.
- Windows Job Object aggregate user+kernel accounting and whole-Job termination primitives are staged with native tests.

`CPULimit` remains false until an exact validated runtime path is intentionally promoted. Darwin CPU remains false because detached-descendant accounting is unresolved. Physical `DiskLimit` remains false everywhere pending a separate enforceable design.

## Durable execution and worker recovery

Rebuilt **#233** defines the Phase 14 durable contract:

- immutable owner-scoped create/exec specifications;
- lease owner/token/expiry authority and immutable execution-attempt records;
- durable Broker session and underlying runtime association before side-effecting execution;
- bounded terminal result/error storage;
- retry default `never`, with replay available only for explicitly idempotent work;
- expired work with a recorded runtime is not claimable until that exact runtime is destroyed;
- cleanup failure stops claiming rather than allowing overlap;
- executor runs through `sandbox.Broker` only, preallocates execution identity, renews leases, cancels on lease loss, and destroys the sandbox before terminal success.

After #233 merges, rebuild the lifecycle-only delta from closed #223 directly on the new `main`: startup recovery before new claims, process-unique worker identity, server/Wails composition using the existing process-wide Broker and SQLite handle, and shutdown ordering that drains/cancels sandbox work before database teardown.

## Kubernetes worker deployment

Rebuilt **#234** preserves the reviewed Phase 15 Helm slice:

- separate worker Deployment and ServiceAccount;
- no service-account token automount;
- non-root UID/GID, RuntimeDefault seccomp, read-only root filesystem, no privilege escalation, all capabilities dropped;
- bounded scratch `emptyDir`;
- internal ClusterIP only;
- ingress only from backend pods and worker egress denied by default;
- bearer token from an existing Secret shared with backend wiring;
- no fabricated cgroup delegation or quota capability claims.

The dedicated `Kubernetes Sandbox Worker Assurance` workflow must remain green before merge.

## Isolated agent worktrees

Rebuilt **#235** preserves the reviewed Phase 16 primitive:

- trusted generation of worktree IDs/physical paths and owner scope including user + agent-run identity;
- base revisions resolved to immutable commit IDs with go-git;
- base and writable snapshots materialized from commit blobs without repository checkout hooks, filters, credential helpers, or `.git` authority;
- unsafe symlinks/file modes fail closed;
- review uses a bounded binary `git diff --no-index` between immutable base and writable snapshot;
- normalized patch output is SHA-256 bound;
- promotion recomputes the digest, requires target cleanliness, and uses guarded `git apply --check` / `git apply`;
- promotion does not stage, commit, fetch, push, or merge, so existing guarded Git mutation/publication controls remain authoritative.

## Network

First-party arbitrary Linux, Windows, and Darwin runtimes remain no-network. Destination-scoped allowlisted egress is not implemented, so `network_allowlist` remains false. Browser-native egress assurance from #168 validates the browser perimeter only; it is not a substitute for arbitrary sandbox socket enforcement.

The next Phase 8 design must resist DNS rebinding/resolution drift, redirects, proxy delegation, private/link-local/loopback address escape, IPv4/IPv6 ambiguity, and connection reuse that violates the approved destination set.

## Credentials

Arbitrary sandbox environments reject credential-bearing keys and dangerous auth/proxy delegation. Opaque credential handles are owner/TTL scoped. Service-specific broker consumers remain open; raw provider credentials must not be injected into arbitrary sandbox environments as a shortcut.

## Persistent extensions

- Linux `auto`: Bubblewrap when a sandbox rootfs is configured; `required` fails closed if confinement is unavailable.
- Windows `auto`: native AppContainer when available; `required` fails closed; `off` is explicit sanitized-host compatibility.
- macOS: native Seatbelt confinement after #164; `required` fails closed if unavailable; `off` remains explicit compatibility.
- Native confinement rejects credential-sensitive explicit environment values by default; the transitional secret-env override remains operator controlled.

## Darwin process-tree limitation

Darwin uses process-group cancellation for ordinary descendants and deliberately reports `process_tree_isolation=false`. Phase 13D proves deliberately detached descendants remain Seatbelt-confined but can outlive ordinary process-group cancellation. Do not promote this capability without a stronger native teardown primitive and matching adversarial proof.

## Execution order

1. Complete exact-head validation and merge **#231** (Windows governed-workspace identity/path-race hardening).
2. Normalize **#232** onto the resulting `main` if required, rerun its exact-head matrix, and merge only with CPU capability reporting still fail-closed unless promotion is independently proven.
3. Continue Phase 8/9: destination-enforced arbitrary-sandbox egress and service-specific credential consumers. Physical-disk accounting remains a separate resource-control design.
4. Normalize and merge **#233** (durable leased task contract), then rebuild only the lifecycle delta from closed #223 on that merged contract.
5. Normalize/validate **#234** (isolated Kubernetes worker) and **#235** (owner-scoped isolated worktrees) on the then-current `main` and merge independently.
6. Continue Phase 17 adversarial assurance with every platform/runtime/capability change.

## Validation discipline

A sandbox phase is complete only when platform-native negative tests exist, capability claims match enforcement, unsupported controls remain explicit, and the exact merge candidate passes all applicable repository checks. A previously green stacked branch is evidence for design review, not permission to merge stale history; rebuilt current-main heads must earn their own validation.
