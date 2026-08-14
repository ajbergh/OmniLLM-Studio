# Agent Sandbox Parity Program — Current Roadmap (August 2026)

> **Status:** ACTIVE
>
> **Checkpoint:** Windows Phase 12 is complete. Explicit execution cancellation is complete in PR #155. macOS Phase 13A is merged in PR #159 and Phase 13B is in final validation in PR #162.

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
| 2 | First-party runtime abstraction + Linux execution plane | **IN PROGRESS** | Bubblewrap/rootfs runtime and `sandboxd` exist. Packaging and remaining enforcement work continue. |
| 3 | Immediate stdio MCP/plugin subprocess hardening | **COMPLETE** | Ambient environment inheritance removed in #99. |
| 4 | Broker-backed `code_execute` + `python_analysis` | **COMPLETE** | Owner-bound execution; restricted Python has no unrestricted host fallback. |
| 5 | Workspace registry + grants + durable journal | **IN PROGRESS** | Owner-scoped grants and state-bound journaled mutations exist; broader path-component TOCTOU assurance remains. |
| 6 | Governed workspace tools | **IN PROGRESS** | Read/write/patch/delete/revert tools exist; completion tracks Phase 5 assurance. |
| 7 | `terminal_exec` + cancellation + resource controls | **IN PROGRESS** | Caller-known execution IDs and explicit cancellation merged in #155 with Linux/Windows/HTTP/`sandboxd` coverage. Wall/output bounds exist. Memory/CPU/PID/disk quotas remain open. |
| 8 | Network broker + destination approvals | **IN PROGRESS** | Owner-bound grants exist; first-party Linux/Windows/Darwin runtimes remain no-network because destination-enforced egress is not implemented. |
| 9 | Credential broker | **IN PROGRESS** | Opaque owner/TTL handles and raw-secret environment rejection exist; service-specific consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy | **COMPLETE** | `auto|required|off` and shared managed-process seam are implemented; Linux can use Bubblewrap and Windows uses AppContainer. macOS native implementation is Phase 13C. |
| 11 | Desktop sandbox/workspace UX | **COMPLETE** | Workspace grants, review APIs, Settings UX, and loopback grant hardening merged in #125. |
| 12 | Windows native confinement backend | **COMPLETE** | #127, #128, #139, and #149 provide protocol-v2 and persistent-extension AppContainer/Job confinement with native adversarial evidence. |
| 13 | macOS native confinement backend | **IN PROGRESS** | 13A Seatbelt primitive merged in #159. 13B first-party local runtime is implemented in #162 and has native runtime tests; final exact-head repository validation is required before merge. 13C persistent extensions and 13D adversarial assurance remain. |
| 14 | Durable sandbox-backed agent tasks | NOT STARTED | Persist sandbox/task association and recovery/scheduling semantics. |
| 15 | Server/Kubernetes sandbox workers | NOT STARTED | Separate worker identity/pods, quotas, hardened security context, and network policy. |
| 16 | Multi-agent isolated worktrees/workspaces | NOT STARTED | Independent writable workspaces with reviewed promotion/reconciliation. |
| 17 | Adversarial assurance suite | **IN PROGRESS** | Continuous negative/platform-native testing across all phases. |

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
- **13B / PR #162** — first-party Darwin local runtime. Read-only workspaces are copied into bounded per-session staging instead of exposing live host paths; the runtime uses explicit system/session read roots, session-only writes, default network deny, bounded wall/output execution, sanitized environment reconstruction, and caller-known cancellation. Native tests prove allowed staged reads, denied host reads/writes/network, environment isolation, output bounds, unsafe staging rejection, and ordinary descendant cancellation. Final exact-head repository validation is still required before merge.
- **13C** — next: native persistent stdio MCP/plugin Seatbelt confinement through `platformExtensionCommandContext` with `auto|required|off` semantics and native lifecycle/denial tests.
- **13D** — final macOS adversarial assurance: detached process/session attempts, path/symlink/rename pressure, cross-runtime authority reuse, cancellation/timeout/forced teardown, and persistent-extension equivalents.

## Open enforcement gaps

### Filesystem

Windows staged-copy flows reject reparse points/junctions, hard links, special files, traversal, and post-open source-handle escapes.

Darwin 13B staging rejects symbolic links, hard links, special files, traversal, source-identity changes, and size changes while copying. The Seatbelt runtime grants file contents only below explicit read roots; exact ancestor directories receive only the directory traversal access macOS requires to resolve those roots. Live host workspace roots are not granted to a staged read-only session.

Broader workspace-registry/path-component TOCTOU cases outside those staging flows remain under Phases 5 and 17.

### Resource controls

Enforced where applicable: OS/filesystem/no-network isolation, TTL cleanup, wall-time bounds, stdout/stderr bounds, and platform-specific process teardown.

Not yet advertised universally as enforced: memory, CPU, PID/process-count, and physical disk quotas. Darwin 13B reports these controls false.

### Network

First-party Linux, Windows, and Darwin 13B runtimes remain no-network. Destination-scoped allowlisted egress is not implemented, so `network_allowlist` remains false.

### Credentials

Arbitrary sandbox environments reject credential-bearing keys and dangerous auth/proxy delegation. Opaque handles are owner/TTL scoped. Darwin additionally rejects runtime-owned path/home/temp overrides and `DYLD_*` injection. Service-specific broker consumers remain open.

### Persistent extensions

- Linux `auto`: Bubblewrap when a sandbox rootfs is configured; otherwise compatibility behavior unless `required` is selected.
- Windows `auto`: native AppContainer when available; `required`: fail closed; `off`: explicit sanitized-host compatibility.
- macOS: native persistent extension confinement remains Phase 13C. Until then, `required` must continue to fail closed and `auto` must not claim native extension confinement.
- Native Linux/Windows confinement rejects credential-sensitive explicit environment values by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a transitional operator override.

### Process-tree isolation on Darwin

13B uses process-group cancellation for ordinary descendants and tests that behavior, but deliberately reports `process_tree_isolation=false`. A hostile descendant can attempt to create an independent process group/session; 13D must prove a stronger enforcement boundary or formally preserve this limitation before the capability may be advertised.

## Execution order

1. Complete and merge Phase 13B only after its exact final head passes native macOS assurance plus repository Quality, Security, and applicable container checks.
2. Implement Phase 13C persistent-extension Seatbelt confinement with native stdio lifecycle, host-file/network denial, environment, and descendant-teardown evidence.
3. Complete Phase 13D adversarial macOS assurance and reconcile runtime/operator docs.
4. Continue Phase 2/5/7/8/9 work: packaging, quotas, destination-enforced egress, broader TOCTOU assurance, and service-specific credential consumers.
5. Continue Phase 17 adversarial assurance with every platform/runtime change.

## Validation discipline

A sandbox phase is complete only when platform-native negative tests exist, capability claims match enforcement, unsupported controls are explicit, and the exact merge head passes applicable repository checks.

Windows Phase 12 met that bar through PRs #127, #128, #139, and #149. macOS Phase 13 must meet the same standard incrementally for arbitrary local execution (13B), persistent extensions (13C), and final adversarial assurance (13D).
