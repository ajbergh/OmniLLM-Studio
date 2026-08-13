# Agent Sandbox Phase 12 — Windows Native Confinement

> **Status:** COMPLETE
>
> **Started:** 2026-08-12
>
> **Completed:** 2026-08-13
>
> **Final Phase 12 checkpoint:** Phase 12D PR #149 squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b` after exact-final-head validation on `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb`.

This file is the detailed durable record for Phase 12 of `AGENT_SANDBOX_ROADMAP_2026-08.md`. The first-party protocol-v2 Windows runtime and persistent local extension processes now both use native Windows AppContainer/Job confinement with Windows-native adversarial evidence. Phase 12 is complete; broader sandbox-program work remains separately tracked in the main roadmap.

The completion claim is intentionally scoped to **Windows native confinement**. It does not claim that resource quotas, destination-scoped egress, general workspace TOCTOU hardening, packaged worker deployment, Windows interpreter availability, or every protocol-v2 lifecycle API is complete.

## Required security properties

1. **Restricted identity/token** — arbitrary processes execute under an AppContainer/lowbox or equivalently restricted Windows identity rather than the backend's unrestricted security context.
2. **Per-sandbox identity** — ACL-scoped resources use an application-issued unique restricting SID or unique AppContainer package SID so authority cannot be reused by another sandbox under the same user.
3. **Process-tree confinement** — a Windows Job Object contains descendants and uses `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`; process creation binds Job membership before untrusted code executes.
4. **Filesystem confinement** — arbitrary process access is limited to sandbox-owned resources or explicitly staged/granted resources. Host workspace ACLs are not widened as a convenience shortcut.
5. **No ambient secrets** — sandboxed processes do not inherit the backend environment, and credential-sensitive explicit environment entries remain governed separately.
6. **No network by default** — Windows runtime/extension execution uses AppContainer with zero network capabilities and native denial evidence.
7. **Fail closed** — unsupported mount/network/interpreter/path behavior is rejected rather than approximated with weaker host execution.
8. **Truthful capabilities** — runtime capability bits claim only controls implemented and natively validated.

## Workstreams

| Slice | Scope | Status | Final evidence |
|---|---|---|---|
| 12A | Native security primitives | **COMPLETE** | PR #127 passed hardened Windows-native restricted-token, per-sandbox SID, cross-sandbox ACL denial, Job Object, full Quality/Security, and multi-architecture container validation; squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`. |
| 12B | First-party protocol-v2 Windows runtime | **COMPLETE** | PR #128 final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed native Windows confinement, backend format/vet/tests/race, Chromium, frontend, Helm, Security, Windows plugin/desktop compatibility, and applicable container builds; squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`. |
| 12C | Persistent stdio MCP/plugin confinement | **COMPLETE** | PR #139 final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed Quality, Security, Windows sandbox/plugin/desktop, Chromium/race, frontend/Helm, and applicable container validation; squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`. |
| 12D | Adversarial Windows assurance | **COMPLETE** | PR #149 final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed native adversarial Windows tests plus complete Quality, Security, race, Playwright, Windows compatibility, Helm, and multi-architecture container validation; squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`. |
| 12E | Documentation and completion | **COMPLETE** | Durable tracker/runtime documentation reconciled after #149; Phase 12 completion is scoped to Windows native confinement while residual cross-program gaps remain explicit below. |

## Phase 12A — native Windows security primitives

PR #127 established reusable Windows primitives:

- cryptographically random per-sandbox restricting SIDs;
- restricted primary-token creation with `CreateRestrictedToken`;
- kill-on-close Job Objects;
- SID-scoped DACL helpers that preserve the normal identity access check;
- native tests proving one sandbox cannot reuse another sandbox's ACL authority.

Manual review caught a first implementation that used the well-known Restricted Code SID. That authority was reusable by other restricted processes under the same Windows user, so it was replaced before merge with a unique application-issued restricting SID and cross-sandbox denial coverage. Final head `8b491a80de9543afcd259d5d5959794bb4a61eaa` passed the applicable gate set before #127 squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.

## Phase 12B — first-party protocol-v2 Windows runtime

PR #128 added the Windows implementation of the protocol-v2 `Runtime` interface.

### Runtime boundary

- requires Windows 10 or newer and feature-detects required stable AppContainer APIs;
- creates an ephemeral AppContainer profile per runtime session;
- launches arbitrary processes with `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` and zero network capabilities;
- attaches a kill-on-close Job Object through `PROC_THREAD_ATTRIBUTE_JOB_LIST` at process creation;
- constrains inherited handles to stdin/stdout/stderr with `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`;
- terminates remaining Job descendants when the root command exits, the execution context cancels, the command times out, or the runtime session is destroyed;
- uses a minimal runtime-owned environment and validates explicit environment entries with the sandbox secret policy;
- enforces wall time plus bounded stdout/stderr without advertising CPU/memory/PID/disk quotas.

The AppContainer package SID is the 12B process/filesystem/network identity. The Phase 12A random restricting-SID primitive remains available for explicitly SID-scoped resources, but it is not layered on top of the AppContainer process token because that would also require explicit read/execute grants for every executable and DLL loaded by the child.

### Workspace policy

PR #128 intentionally does not alter user workspace ACLs.

- no mount creates an ephemeral writable workspace inside the AppContainer profile;
- one `read_only` mount is copied into the AppContainer profile and exposed read-only;
- `read_write` and `read_write_no_delete` arbitrary-process mounts fail closed;
- staging is bounded to 20,000 entries / 256 MiB, rejects reparse points, multiply-linked files, and special files, validates destination containment, and closes each source handle immediately;
- after every source file is opened, `GetFinalPathNameByHandle` must resolve that opened handle beneath the canonical source root before bytes are copied;
- the staged workspace uses a protected DACL: the backend user, LocalSystem, and Administrators retain cleanup authority while the AppContainer package SID receives read/traverse/execute authority;
- `home` and `tmp` are sandbox-owned writable directories.

The copy/staging limit is an admission bound only. `DiskLimit` remains `false` because no physical runtime disk quota is enforced.

### Final 12B evidence

Final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed:

- dedicated Windows confinement tests for AppContainer token state, read-only staged workspace, unrelated-host-file denial, ambient-secret absence, loopback denial, descendant teardown, active `Destroy` synchronization, and retryable cleanup failure handling;
- `cmd/sandboxd` Windows compilation/testing;
- backend formatting, vet, unit/integration tests, and race detector;
- Windows plugin lifecycle and desktop bindings;
- frontend lint/unit/build and full Chromium Playwright;
- Helm validation;
- dependency audit plus Go and JavaScript/TypeScript CodeQL;
- applicable Linux multi-architecture frontend/backend container builds.

PR #128 then squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.

## Phase 12C — persistent stdio MCP/plugin confinement

PR #139 moved persistent Windows stdio MCP/plugin processes from the sanitized compatibility path to native AppContainer confinement while preserving their streaming JSON-RPC lifecycle.

### Managed process seam

`backend/internal/sandbox/process.go` defines the narrow `CommandProcess` lifecycle used by host, Linux, and Windows extension processes:

- `StdinPipe`, `StdoutPipe`, `StderrPipe`;
- `Start` and `Wait`;
- `Kill` for process-tree termination.

MCP continues to use the pipe/start/wait subset. Plugin forced shutdown calls `CommandProcess.Kill()` rather than reaching through `cmd.Process`, which lets the Windows backend terminate the whole Job Object without rewriting protocol code.

### Native Windows extension launch boundary

The Windows adapter:

- feature-detects Windows 10+ AppContainer APIs;
- in `required` mode, fails closed if native confinement is unavailable;
- in `auto` mode, uses native Windows confinement when available and falls back only when the native backend itself is unavailable;
- in `off` mode, deliberately selects the sanitized host compatibility boundary;
- validates explicit extension environment values with the existing credential-sensitive policy before native launch;
- creates a unique ephemeral AppContainer profile for each persistent process;
- stages non-System32 command directories read-only rather than widening ACLs on original host paths;
- stages an explicit/inferred working directory read-only when needed;
- remaps absolute arguments only when they resolve beneath staged roots and rejects unrelated absolute host arguments;
- gives the child runtime-owned `home`/`tmp` and a minimal Windows environment rather than ambient backend state;
- supplies AppContainer security capabilities, Job membership, and the explicit stdio handle list in the same creation-time attribute list;
- launches `.cmd`/`.bat` through the System32 command processor while keeping the staged script path inside the AppContainer profile;
- terminates the Job on cancellation, forced plugin shutdown, and root-process completion;
- performs immediate bounded profile cleanup and retries transient profile-removal failures.

Development PRs #134 and #137 were deliberately closed without merge after exposing formatting and unrelated-workflow-diff problems. Corrected PR #139 final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed the full gate set and squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.

## Phase 12D — adversarial Windows assurance

PR #149 converted the remaining Windows completion claims into native negative evidence. The detailed matrix remains in `AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md`.

The final suite proves or exercises:

- unique per-extension AppContainer identities and denial of cross-profile read/write authority;
- read-only staged extension bundles with writable sandbox-owned home/tmp;
- denial of unrelated host-file read/write;
- ambient backend-secret absence;
- default-deny loopback/network behavior;
- descendant Job teardown after root exit;
- context cancellation terminating the full extension Job process tree;
- hard-link staging rejection;
- junction/reparse-point staging rejection;
- fail-closed unrelated absolute argument handling;
- credential-sensitive explicit environment rejection in `required` mode;
- `auto` and `required` selecting the native Windows backend while `off` selects the sanitized compatibility boundary.

Final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed the native Windows sandbox/adversarial job and the complete repository Quality, Security, Windows plugin/desktop, backend race, Chromium Playwright, Helm, dependency/CodeQL, and applicable multi-architecture container gates before #149 squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

## Phase 12E — completion decision

Phase 12 is now `COMPLETE` because the Windows-specific exit criteria are met across the reusable primitives, first-party runtime, persistent extension surface, and dedicated adversarial assurance suite.

This does **not** close the following broader roadmap items:

- memory, CPU, process-count, and physical disk quotas remain unimplemented/unadvertised (Phase 7 / broader runtime work);
- destination-scoped network allowlisting remains unimplemented; the first-party Windows runtime and Windows native extension backend remain intentionally no-network (Phase 8);
- broader workspace-registry/path-component TOCTOU assurance outside the staged-copy path remains open (Phases 5 and 17);
- service-specific credential broker consumers remain open (Phase 9);
- `sandboxd` packaging/deployment remains broader Phase 2/15 work;
- Python/JavaScript shortcuts in the Windows arbitrary-process runtime remain fail-closed until an AppContainer-readable interpreter/package design is natively validated;
- explicit execution-ID cancellation addressability is tracked separately as issue #151. Context cancellation, timeout, session `Destroy`, and Windows Job teardown work and have native evidence, but the current synchronous protocol does not reveal an execution ID early enough for a caller to use the separate explicit `Cancel(executionID)` endpoint while `Exec` is active.

Issue #151 belongs to the protocol/runtime lifecycle track and does not invalidate the Windows confinement guarantees proved in Phase 12. It must nevertheless remain visible until repaired.

## Validation policy going forward

Windows confinement regressions remain part of the normal repository gate set. Relevant PRs continue to run:

- backend formatting, vet, unit/integration tests, and race detector;
- dedicated `windows-latest` sandbox confinement/adversarial tests;
- Windows plugin lifecycle and desktop bindings;
- frontend lint/unit/build and Chromium Playwright;
- Helm/deployment checks;
- dependency audit and Go/JavaScript-TypeScript CodeQL;
- applicable frontend/backend multi-architecture container builds.

No later refactor should widen a capability claim merely because a Windows API is present. Native behavior evidence remains authoritative.

## Progress log

- **2026-08-12 — #127:** Phase 12A native Windows SID/token/Job/ACL primitives; manual audit replaced globally reusable Restricted Code SID authority with per-sandbox SIDs; hardened gates passed and #127 merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`.
- **2026-08-13 — #128:** Phase 12B first-party Windows AppContainer runtime; exact final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed full Quality/Security/native Windows/Chromium/container validation and merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.
- **2026-08-13 — #134 / #137:** Phase 12C development/replay PRs intentionally closed without merge after formatting and unrelated workflow-diff findings.
- **2026-08-13 — #139:** corrected Phase 12C final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed full validation and merged as `69590078223f85f4a6eb5c64aa24959aafa10835`.
- **2026-08-13 — #149:** Phase 12D exact final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed native adversarial Windows assurance plus complete Quality/Security/Playwright/race/container validation and merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.
- **2026-08-13 — #151:** separate protocol lifecycle issue opened for explicit execution-ID cancellation addressability; not a Windows confinement regression.
- **2026-08-13 — 12E:** durable operator/roadmap documentation reconciled and Windows native confinement Phase 12 marked `COMPLETE` with residual cross-program gaps kept explicit.
