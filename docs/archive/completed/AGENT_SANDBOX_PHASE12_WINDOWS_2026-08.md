> **Archived — completed.** Windows Phase 12 closed through PR #149. The current cross-platform plan is [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Agent Sandbox Phase 12 — Windows Native Confinement

> **Status:** COMPLETE
>
> **Started:** 2026-08-12
>
> **Completed through:** Phase 12D PR #149, squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`
>
> **Documentation closeout:** Phase 12E on branch `docs/sandbox-phase12-completion-20260813`

This file is the durable Windows-specific tracker for Phase 12 of `AGENT_SANDBOX_ROADMAP_2026-08.md`. The implementation and native assurance slices are complete. Phase 12E reconciles operator/runtime documentation with the merged behavior; it does not add new Windows privilege or capability claims.

## Completion summary

| Slice | Scope | Status | Final evidence |
|---|---|---|---|
| 12A | Native Windows security primitives | **COMPLETE** | PR #127; unique per-sandbox SID authority, restricted-token primitives, Job Objects, DACL helpers, and cross-sandbox denial; squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`. |
| 12B | First-party protocol-v2 Windows runtime | **COMPLETE** | PR #128 final head `282fbc0fc366c3791f31b7e1d841250971b0b980`; exact-head Quality, Security, Windows-native, Chromium/race, Helm, and applicable container validation; squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`. |
| 12C | Persistent stdio MCP/plugin confinement | **COMPLETE** | PR #139 final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977`; Quality, Security, applicable containers, Windows lifecycle/native jobs, and no unresolved review threads; squash-merged as `69590078223f85f4a6eb5c64aa24959aafa10835`. |
| 12D | Adversarial Windows assurance | **COMPLETE** | PR #149 final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb`; Quality, Security, Windows adversarial/native, Chromium/race, desktop/plugin, and linux/amd64+linux/arm64 container validation; squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`. |
| 12E | Documentation reconciliation | **COMPLETE when this closeout PR merges** | Reconciles this tracker, the primary roadmap, runtime/operator docs, and architecture wording with the merged 12A–12D behavior. |

## Enforced Windows runtime boundary

The first-party protocol-v2 Windows runtime is implemented with Windows AppContainer and Job Objects.

Confirmed behavior:

- Windows 10+ feature detection for the required AppContainer APIs;
- a unique ephemeral AppContainer profile/package SID per runtime session;
- `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` applied at process creation;
- zero AppContainer network capabilities, so the runtime is no-network by default;
- Job Object membership applied at process creation through `PROC_THREAD_ATTRIBUTE_JOB_LIST`, avoiding a start-unrestricted/post-assign window;
- explicit inherited handle list limited to stdin/stdout/stderr;
- process-tree teardown on root completion, cancellation, timeout, or session destruction;
- runtime-owned minimal environment rather than ambient backend inheritance;
- wall-time and stdout/stderr bounds;
- retryable profile cleanup when Windows temporarily blocks deletion.

The runtime intentionally does **not** advertise destination allowlisting, memory quota, CPU quota, PID/process-count quota, or physical disk quota enforcement.

## Workspace and filesystem policy

Windows arbitrary-process execution does not widen ACLs on user workspace roots.

- No project mount produces an AppContainer-owned ephemeral writable workspace.
- One `read_only` project mount may be staged into AppContainer-owned storage.
- Arbitrary-process `read_write` and `read_write_no_delete` project mounts fail closed.
- Staging is bounded to 20,000 entries and 256 MiB.
- Reparse points, junctions, multiply-linked files, special files, traversal, and destination escapes are rejected.
- After each source file is opened, `GetFinalPathNameByHandle` must still resolve that exact handle under the canonical source root before bytes are copied.
- The staged workspace receives a protected read-only DACL for the AppContainer package SID; backend user, LocalSystem, and Administrators retain cleanup authority.
- AppContainer-owned home/tmp remain writable.

These controls close the staged-copy reparse/rename/hard-link class directly exercised by Phase 12 tests. Broader workspace-registry/path-component TOCTOU assurance outside these staging flows remains tracked under Phases 5 and 17.

## Persistent plugin and stdio MCP confinement

Phase 12C moved persistent local extensions behind the shared managed-process boundary without changing their JSON-RPC streaming protocols.

`CommandProcess` supplies the lifecycle needed by MCP/plugins:

- `StdinPipe`;
- `StdoutPipe`;
- `StderrPipe`;
- `Start`;
- `Wait`;
- `Kill`.

On Windows:

- `OMNILLM_EXTENSION_SANDBOX_MODE=auto` selects native AppContainer confinement when the backend is available;
- `required` fails closed if native confinement cannot be provided;
- `off` is the explicit sanitized-host compatibility override;
- each persistent extension receives a unique ephemeral AppContainer profile;
- non-System32 command bundles and required working-directory content are staged read-only instead of granting the child access to the original host directory;
- absolute arguments are remapped only when they resolve beneath staged command/workspace roots; unrelated absolute host arguments fail closed;
- `.cmd`/`.bat` launch through System32 `cmd.exe` while the script remains staged inside the AppContainer profile;
- home/tmp are AppContainer-owned and writable;
- ambient backend environment is absent;
- credential-sensitive explicit environment values remain rejected under native confinement unless the narrow operator override `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is intentionally enabled;
- cancellation and forced shutdown terminate the Job-backed process tree;
- transient profile cleanup failure is retried rather than abandoning an unreachable profile.

## Native adversarial assurance

Phase 12D added direct `windows-latest` evidence beyond ordinary lifecycle success.

PR #149 proves:

- the child token is actually AppContainer;
- a confined extension cannot write its staged command bundle;
- AppContainer-owned home is writable;
- unrelated host-file reads and writes are denied;
- one concurrently running extension cannot reuse another extension profile's filesystem authority;
- an ambient `OMNILLM_MASTER_KEY` is absent;
- a parent loopback listener is unreachable under the no-network policy;
- root exit kills a spawned descendant, verified with a host-held process handle;
- context cancellation returns `context.Canceled` and kills the descendant process tree;
- workspace staging rejects Windows hard links and junction/reparse points;
- unrelated absolute extension arguments fail closed;
- sensitive explicit environment values fail closed in `required` mode;
- `auto` and `required` select `windowsExtensionProcess`, while `off` selects the sanitized host adapter.

The dedicated Windows job also continues to run the Phase 12A/12B runtime/primitives suite and `cmd/sandboxd` Windows compilation/testing.

## Validation record

### PR #128 — first-party runtime

Final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed:

- Windows AppContainer/runtime confinement;
- active-destroy synchronization and retryable cleanup regression;
- backend formatting, vet, unit/integration tests, and race detector;
- Windows plugin lifecycle and desktop compatibility;
- frontend lint/unit/build;
- Chromium Playwright;
- Helm;
- dependency audit and Go/JavaScript-TypeScript CodeQL;
- applicable multi-architecture container builds.

### PR #139 — persistent extensions

Final head `f8076939c7bb27a3938a0a91d89c9b2ec444b977` passed Quality Gate, Security Scan, applicable container validation, Windows plugin/native jobs, and review-thread inspection before merge.

### PR #149 — adversarial assurance

Final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed:

- complete Quality Gate, including formatting/vet/tests/race, Windows sandbox/plugin/desktop, frontend, Helm, and Chromium;
- complete Security Scan, including dependency audit and both CodeQL lanes;
- frontend and backend container builds for `linux/amd64` and `linux/arm64` plus Helm container validation;
- no unresolved review threads.

PR #149 squash-merged as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

## Explicitly open work outside Phase 12 completion

Phase 12 completion does not imply the wider sandbox program is complete.

Still open:

- **Issue #151:** protocol-v2 explicit `Cancel(runtimeID, executionID)` is not addressable while synchronous `Exec` is running because the execution ID is currently returned only when `Exec` completes. Context cancellation and session `Destroy` work; this is a separate API-contract defect.
- **Phase 2:** packaging/deployment polish for first-party workers and remaining runtime enforcement work.
- **Phases 5/17:** broader workspace-registry/path-component TOCTOU assurance outside the Windows staging flow.
- **Phase 7:** memory/CPU/PID/disk quotas are not implemented or advertised.
- **Phase 8:** destination-enforced egress is not implemented; first-party Windows remains no-network rather than allowlisted-network.
- **Phase 9:** service-specific brokered credential consumers remain.
- **Phase 13:** macOS native confinement remains independent future work.
- **Phase 15:** dedicated hardened server/Kubernetes sandbox workers remain future work.

Python and JavaScript shortcuts in the first-party Windows protocol-v2 runtime also remain fail-closed until an AppContainer-readable interpreter/package design is intentionally implemented and natively tested. Persistent extensions may stage their own command bundles subject to the stricter extension path policy; that does not make arbitrary host interpreters generally available to protocol-v2 code execution.

## Completion decision

Phase 12's stated objective is satisfied: both first-party protocol-v2 execution and persistent local plugin/stdio MCP execution now have OS-enforced Windows confinement with behavior-level native Windows evidence, including adversarial negative tests.

No remaining item above is required to claim the **Windows confinement backend** exists. Each remains tracked under its own roadmap phase or issue and must fail closed where the corresponding control is unavailable.
