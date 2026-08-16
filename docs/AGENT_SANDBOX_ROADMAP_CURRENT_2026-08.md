# Agent Sandbox Parity Program — Current Roadmap (August 2026)

> **Status:** ACTIVE
>
> **Checkpoint:** Windows Phase 12 is complete. Explicit execution cancellation is complete in PR #155. macOS Phase 13 is complete through Phase 13D, merged in PR #166 as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17`. Native browser egress assurance merged in PR #168, Broker resource admission fails closed after PR #170, Windows process-count quotas merged in PR #171, Windows aggregate memory enforcement merged in PR #172 as `bc9eb6f204db9dcb2c6fb3670262ef8d0c58cb3f`, Linux cgroup-v2 PID enforcement merged in PR #173 as `981691a0058efd7a061cc892d3f43f1edf4d22e3`, strict aggregate Linux memory enforcement merged in PR #174 as `c8adc3cd4e8492b57ff4b05450e84a46f409edbc`, Linux descriptor-relative workspace content reads merged in PR #175 as `925e258539bc16ad4bf96c1d1c6d78f49dd64ada`, and PR #176 now has focused native Ubuntu evidence for descriptor-relative Linux workspace mutation transactions under parent rename/symlink replacement pressure pending final documentation-inclusive validation.

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

## Current phase status

| Phase | Scope | Status | Current evidence / next exit |
|---|---|---|---|
| 0 | Architecture, threat model, durable roadmap | **COMPLETE** | Core design documents are on `main`. |
| 1 | Protocol v2 + owner-bound sessions | **COMPLETE** | Broker sessions, ownership/TTL checks, authenticated worker protocol, bounded results, and capability negotiation merged in #118. |
| 2 | First-party runtime abstraction + Linux execution plane | **IN PROGRESS** | Bubblewrap/rootfs runtime and `sandboxd` exist. #173 merged delegated cgroup-v2 PID enforcement and #174 merged aggregate memory enforcement. Packaging remains open. |
| 3 | Immediate stdio MCP/plugin subprocess hardening | **COMPLETE** | Ambient environment inheritance removed in #99. |
| 4 | Broker-backed `code_execute` + `python_analysis` | **COMPLETE** | Owner-bound execution; restricted Python has no unrestricted host fallback. |
| 5 | Workspace registry + grants + durable journal | **IN PROGRESS** | Owner-scoped grants and state-bound journaled mutations exist. #175 binds Linux read/search content opens to pinned directory FDs; #176 binds Linux write/delete/revert state capture, mutation, verification, and rollback to one pinned parent-directory identity. Registered-root identity replacement, search enumeration races, and non-Linux descriptor-relative operations remain open. |
| 6 | Governed workspace tools | **IN PROGRESS** | Read/write/patch/delete/revert tools exist. Linux read/search content is protected by #175; Linux write/delete/revert transactions gain the #176 descriptor-relative mutation boundary. Completion still tracks root-identity and remaining cross-platform Phase 5 assurance. |
| 7 | `terminal_exec` + cancellation + resource controls | **IN PROGRESS** | Caller-known execution IDs and explicit cancellation merged in #155. Broker quota requests fail closed when unsupported (#170). Windows PID/memory quotas merged in #171/#172; Linux PID/memory quotas merged in #173/#174. macOS resource quotas, CPU semantics, and physical-disk quotas remain open. |
| 8 | Network broker + destination approvals | **IN PROGRESS** | Owner-bound grants exist; first-party Linux/Windows/Darwin runtimes remain no-network because destination-enforced egress is not implemented. |
| 9 | Credential broker | **IN PROGRESS** | Opaque owner/TTL handles and raw-secret environment rejection exist; service-specific consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy | **COMPLETE** | `auto|required|off` and shared managed-process seam are implemented; Linux uses Bubblewrap, Windows uses AppContainer, and macOS uses Seatbelt after #164. |
| 11 | Desktop sandbox/workspace UX | **COMPLETE** | Workspace grants, review APIs, Settings UX, and loopback grant hardening merged in #125. |
| 12 | Windows native confinement backend | **COMPLETE** | #127, #128, #139, and #149 provide protocol-v2 and persistent-extension AppContainer/Job confinement with native adversarial evidence. |
| 13 | macOS native confinement backend | **COMPLETE** | 13A Seatbelt primitive (#159), 13B first-party local runtime (#162), 13C persistent extensions (#164), and 13D adversarial assurance (#166) are merged. The proven detached-descendant limitation remains explicit. |
| 14 | Durable sandbox-backed agent tasks | NOT STARTED | Persist sandbox/task association and recovery/scheduling semantics. |
| 15 | Server/Kubernetes sandbox workers | NOT STARTED | Separate worker identity/pods, quotas, hardened security context, cgroup delegation, and network policy. |
| 16 | Multi-agent isolated worktrees/workspaces | NOT STARTED | Independent writable workspaces with reviewed promotion/reconciliation. |
| 17 | Adversarial assurance suite | **IN PROGRESS** | Continuous negative/platform-native testing across all phases; browser-native egress assurance is covered by #168, Windows quota controls have native negative evidence, Linux cgroup-v2 quota assurance covers PID/memory, and the Ubuntu workspace path-race lane now exercises both #175 content reads and #176 mutation transactions ten times per run. |

## Windows Phase 12 lineage

- **12A / PR #127** — unique per-sandbox Windows authority, restricted-token primitives, Job Objects, DACL helpers, and cross-sandbox denial; merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.
- **12B / PR #128** — first-party Windows AppContainer protocol-v2 runtime; merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.
- **12C / PR #139** — persistent stdio MCP/plugin AppContainer confinement; merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.
- **12D / PR #149** — direct adversarial Windows assurance; merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

PR #149's exact final head passed Quality Gate, Security Scan, native Windows sandbox/plugin/desktop jobs, backend format/vet/tests/race, Chromium, frontend, Helm, dependency audit, both CodeQL lanes, and frontend/backend `linux/amd64` + `linux/arm64` container builds.

## Cross-platform cancellation lineage

**PR #155** fixed the protocol-v2 cancellation addressability defect. Callers can provide a canonical execution reference before `Exec`; omitted IDs are generated through the shared helper; active duplicates fail closed; HTTP and `sandboxd` preserve the reference; Linux and Windows runtimes use it as the cancellation key. The issue tracked as #151 is therefore no longer an open roadmap item.

## macOS Phase 13 lineage

- **13A / PR #159** — fixed `/usr/bin/sandbox-exec` Seatbelt primitive, canonicalized policy roots, default network deny, explicit write-root proof, and native `macos-latest` evidence; merged as `ce7d880ab39402671a6f39407ea9319418089de4`.
- **13B / PR #162** — first-party Darwin local runtime, merged as `840b00bb6d2b74d1a88eb1fd910d06dab64118a2` after native and repository validation.
- **13C / PR #164** — native persistent stdio MCP/plugin Seatbelt confinement with `auto|required|off` semantics and native lifecycle/denial tests, merged as `44f410793a70444963ec1eecb989b15df159b5f1`.
- **13D / PR #166** — final macOS adversarial assurance: detached process/session attempts, path/symlink/rename pressure, cross-runtime authority reuse, and persistent-extension equivalents; merged as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17` after native and repository validation.

## Open enforcement gaps

### Filesystem

Windows staged-copy flows reject reparse points/junctions, hard links, special files, traversal, and post-open source-handle escapes.

Darwin 13B staging rejects symbolic links, hard links, special files, traversal, source-identity changes, and size changes while copying. The Seatbelt runtime grants file contents only below explicit read roots; exact ancestor directories receive only the directory traversal access macOS requires to resolve those roots. Live host workspace roots are not granted to a staged read-only session.

On Linux workspace tools, #175 replaces pathname re-open for file content with descriptor-relative traversal from an already-open workspace root. Each parent is opened with `O_NOFOLLOW|O_DIRECTORY`, the final file is opened with `O_NOFOLLOW`, and the opened inode is validated with `fstat`; search candidates discovered by `WalkDir` are reopened through the same read boundary before bytes are trusted.

#176 extends descriptor-relative identity through governed mutations. `WriteFile`, delete, and revert now acquire a platform mutation target; on Linux this target pins the workspace root and final parent directory, captures the final file with `openat(O_NOFOLLOW)` plus `fstat`, creates replacement files inside the pinned parent, commits them with `renameat`, deletes with `unlinkat`, and uses the same pinned parent for after-state capture and journal-failure rollback. Native negative coverage renames the granted parent, replaces its old pathname with a symlink to an outside directory, and proves write/delete/restore continue to affect only the pinned detached directory while the outside file remains unchanged. The combined Ubuntu workspace lane runs read and mutation path-race tests ten times.

This does **not** close all workspace TOCTOU work. The registry stores the canonical root pathname but not a durable filesystem identity, so replacement of the registered workspace root before a new operation can transfer the pathname to a different directory. `WalkDir` candidate enumeration remains pathname-based, and non-Linux governed workspace operations still use their prior pathname behavior. These remain Phase 5/17 work.

### Resource controls

Enforced where applicable: OS/filesystem/no-network isolation, TTL cleanup, wall-time bounds, stdout/stderr bounds, platform-specific process teardown, Windows Job Object process-count limits, Windows aggregate Job committed-memory limits, Linux delegated cgroup-v2 `pids.max`, and aggregate Linux `memory.max` with cgroup swap disabled by `memory.swap.max=0` after #174.

Broker admission rejects non-zero memory, CPU, process-count, or disk limits when the selected runtime does not advertise the matching capability. Windows advertises `pid_limit=true` and `memory_limit=true`. Linux dynamically advertises `pid_limit`/`memory_limit` only for controllers available at the configured delegated cgroup-v2 boundary. A non-zero quota fails closed when the matching controller is unavailable. Linux executions are atomically born in their execution cgroup with `CLONE_INTO_CGROUP`; PID and memory limits therefore apply to descendants inherited into the same cgroup before untrusted code executes.

The cgroup-v2 `pids` controller limits tasks, not only distinct process IDs, so threads also consume the configured `resources.max_processes` ceiling. For memory, `memory.max` is the hard resident/accounted memory limit while swap authority is separate; #174 sets `memory.swap.max=0` for positive `resources.memory_bytes` so anonymous pages cannot use swap to exceed the application-level byte ceiling. Native evidence requires `memory.events` to record OOM enforcement under descendant pressure.

CPU remains deferred until the cumulative `cpu_time_ms` contract is reconciled with cgroup/Job Object CPU primitives; physical-disk accounting remains a separate design problem. All macOS resource quota capabilities remain false.

### Network

First-party Linux, Windows, and Darwin runtimes remain no-network. Destination-scoped allowlisted egress is not implemented, so `network_allowlist` remains false.

The browser perimeter is separately validated by native Chromium adversarial coverage after PR #168; that does not constitute arbitrary-sandbox socket egress enforcement.

### Credentials

Arbitrary sandbox environments reject credential-bearing keys and dangerous auth/proxy delegation. Opaque handles are owner/TTL scoped. Darwin additionally rejects runtime-owned path/home/temp overrides and `DYLD_*` injection. Service-specific broker consumers remain open.

### Persistent extensions

- Linux `auto`: Bubblewrap when a sandbox rootfs is configured; otherwise compatibility behavior unless `required` is selected.
- Windows `auto`: native AppContainer when available; `required`: fail closed; `off`: explicit sanitized-host compatibility.
- macOS: native persistent extension confinement is active after #164; `required` fails closed if the native primitive is unavailable and `off` remains the explicit sanitized-host compatibility path.
- Native Linux/Windows/Darwin confinement rejects credential-sensitive explicit environment values by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a transitional operator override.

### Process-tree isolation on Darwin

Darwin uses process-group cancellation for ordinary descendants and deliberately reports `process_tree_isolation=false`. 13D proves that a deliberately detached descendant remains Seatbelt-confined but may outlive ordinary process-group cancellation, so the capability remains false until a stronger native teardown primitive is implemented and proven.

## Execution order

1. Complete and merge Linux workspace mutation path-race hardening (#176) only after its documentation-inclusive exact head passes the combined Ubuntu workspace assurance and repository-wide gates.
2. Persist and verify exact granted-root filesystem identity so replacement of a registered root pathname cannot silently transfer workspace authority; keep this a separate reviewable schema/runtime slice.
3. Address remaining search-enumeration and non-Linux workspace path-race assurance without weakening existing state/digest and rollback semantics.
4. Resolve the aggregate/cumulative CPU-time semantic contract before enabling `cpu_limit` on any platform; design physical-disk accounting separately.
5. Continue Phase 8/9 work: destination-enforced egress and service-specific credential consumers.
6. Continue Phase 17 adversarial assurance with every platform/runtime change.
7. After the intended deployment’s required quota/egress hardening is complete, advance durable sandbox-backed tasks and isolated workers.

## Validation discipline

A sandbox phase is complete only when platform-native negative tests exist, capability claims match enforcement, unsupported controls are explicit, and the exact merge head passes applicable repository checks.

Windows Phase 12 met that bar through PRs #127, #128, #139, and #149. macOS Phase 13 met the same standard incrementally for arbitrary local execution (13B), persistent extensions (13C), and final adversarial assurance (13D). PR #171 merged as `11dfab99e73fe414e45cc44b0f33d4c80789295a`; PR #172 merged as `bc9eb6f204db9dcb2c6fb3670262ef8d0c58cb3f`; PR #173 passed its exact-head native/repository matrix and merged as `981691a0058efd7a061cc892d3f43f1edf4d22e3`; PR #174 passed its exact-head native/repository matrix and merged as `c8adc3cd4e8492b57ff4b05450e84a46f409edbc`; PR #175 passed its exact-head native/repository matrix and merged as `925e258539bc16ad4bf96c1d1c6d78f49dd64ada`. PR #176 has focused native Ubuntu workspace-operation evidence green on implementation head `74ee4d02282fd7c642c8fc002eae79556db6dfdf`; its documentation-inclusive final head must pass the same applicable gates before merge.
