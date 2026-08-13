# Agent Sandbox Parity Program — Current Roadmap (August 2026)

> **Status:** ACTIVE
>
> **Checkpoint:** Windows Phase 12 implementation and adversarial assurance are complete through PR #149. Phase 12E reconciles documentation only.

## Program invariants

- Arbitrary model-directed processes do not execute inside the primary backend process.
- Owner scope is revalidated on sandbox operations; IDs are references, not authorization.
- Models do not receive physical host paths for workspace mounts.
- Ambient backend secrets are absent from arbitrary sandboxes and natively confined extensions.
- Network is denied by default and widened only when a runtime can enforce the requested policy.
- Descendants inherit confinement and are terminated on cancellation/session teardown.
- Capability reporting is limited to controls actually enforced.
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
| 7 | `terminal_exec` + cancellation + resource controls | **IN PROGRESS** | Linux/Windows process-tree teardown and wall/output bounds exist. Memory/CPU/PID/disk quotas remain open. Issue #151 tracks explicit execution-ID cancellation addressability. |
| 8 | Network broker + destination approvals | **IN PROGRESS** | Owner-bound grants exist; first-party Linux/Windows remain no-network because destination-enforced egress is not implemented. |
| 9 | Credential broker | **IN PROGRESS** | Opaque owner/TTL handles and raw-secret environment rejection exist; service-specific consumers remain. |
| 10 | Local plugin + stdio MCP confinement policy | **COMPLETE** | `auto|required|off` and shared managed-process seam are implemented; Linux can use Bubblewrap and Windows uses AppContainer. macOS awaits Phase 13. |
| 11 | Desktop sandbox/workspace UX | **COMPLETE** | Workspace grants, review APIs, Settings UX, and loopback grant hardening merged in #125. |
| 12 | Windows native confinement backend | **COMPLETE** | #127, #128, #139, and #149 provide protocol-v2 and persistent-extension AppContainer/Job confinement with native adversarial evidence. |
| 13 | macOS native confinement backend | NOT STARTED | Requires OS-enforced file/network/process confinement plus macOS-native evidence. |
| 14 | Durable sandbox-backed agent tasks | NOT STARTED | Persist sandbox/task association and recovery/scheduling semantics. |
| 15 | Server/Kubernetes sandbox workers | NOT STARTED | Separate worker identity/pods, quotas, hardened security context, and network policy. |
| 16 | Multi-agent isolated worktrees/workspaces | NOT STARTED | Independent writable workspaces with reviewed promotion/reconciliation. |
| 17 | Adversarial assurance suite | **IN PROGRESS** | Continuous negative/platform-native testing across all phases. |

## Windows Phase 12 lineage

- **12A / PR #127** — unique per-sandbox Windows authority, restricted-token primitives, Job Objects, DACL helpers, and cross-sandbox denial; merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.
- **12B / PR #128** — first-party Windows AppContainer protocol-v2 runtime; final head `282fbc0fc366c3791f31b7e1d841250971b0b980`; merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.
- **12C / PR #139** — persistent stdio MCP/plugin AppContainer confinement; final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977`; merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.
- **12D / PR #149** — direct adversarial Windows assurance; final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb`; merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

PR #149's exact final head passed Quality Gate, Security Scan, native Windows sandbox/plugin/desktop jobs, backend format/vet/tests/race, Chromium, frontend, Helm, dependency audit, both CodeQL lanes, and frontend/backend `linux/amd64` + `linux/arm64` container builds.

## Open enforcement gaps

### Filesystem

Windows staged-copy flows reject reparse points/junctions, hard links, special files, traversal, and post-open source-handle escapes. Broader workspace-registry/path-component TOCTOU cases outside those staging flows remain under Phases 5 and 17.

### Resource controls

Enforced where applicable: OS/filesystem/no-network/process-tree isolation, TTL cleanup, wall-time bounds, and stdout/stderr bounds.

Not yet advertised as enforced: memory, CPU, PID/process-count, and physical disk quotas.

### Network

First-party Linux and Windows runtimes remain no-network. Destination-scoped allowlisted egress is not implemented, so `network_allowlist` remains false.

### Credentials

Arbitrary sandbox environments reject credential-bearing keys and dangerous auth/proxy delegation. Opaque handles are owner/TTL scoped. Service-specific broker consumers remain open.

### Persistent extensions

- Linux `auto`: Bubblewrap when a sandbox rootfs is configured; otherwise compatibility behavior unless `required` is selected.
- Windows `auto`: native AppContainer when available; `required`: fail closed; `off`: explicit sanitized-host compatibility.
- macOS: native extension confinement remains pending Phase 13.
- Native Linux/Windows confinement rejects credential-sensitive explicit environment values by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a transitional operator override.

### Explicit execution cancellation contract

Issue #151 remains open. Protocol v2 exposes `Cancel(runtimeID, executionID)`, but synchronous `Exec` currently returns its internally generated execution ID only after completion. Context cancellation and session `Destroy` remain effective and tested. This is an API-contract defect, not a Windows OS-confinement gap.

## Execution order

1. Land the Phase 12E documentation closeout.
2. Fix Issue #151 with a caller-known/start-time execution reference, duplicate-ID fail-close, and Linux/Windows/HTTP/`sandboxd` coverage.
3. Begin Phase 13 macOS native confinement independently.
4. Continue Phase 2/5/7/8/9 work: packaging, quotas, destination-enforced egress, broader TOCTOU assurance, and service-specific credential consumers.
5. Continue Phase 17 adversarial assurance with every platform/runtime change.

## Validation discipline

A sandbox phase is complete only when platform-native negative tests exist, capability claims match enforcement, unsupported controls are explicit, and the exact merge head passes applicable repository checks.

Phase 12 met that bar through PRs #127, #128, #139, and #149.
