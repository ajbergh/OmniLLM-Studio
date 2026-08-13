# Agent Sandbox Phase 12 — Windows Native Confinement

> **Status:** IN PROGRESS
>
> **Started:** 2026-08-12
>
> **Active branch:** `agent/sandbox-windows-extensions-12c-clean-20260813`
>
> **Active PR:** #137
>
> **Latest merged Phase 12 checkpoint:** 12B PR #128 squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502` after exact-final-head validation on `282fbc0fc366c3791f31b7e1d841250971b0b980`.

This file is the detailed durable tracker for Phase 12 of `AGENT_SANDBOX_ROADMAP_2026-08.md`. Phase 12 is not complete until both the first-party protocol-v2 Windows runtime and persistent local extension processes use native Windows confinement with Windows-native test evidence, followed by the planned adversarial assurance pass.

## Required security properties

1. **Restricted identity/token** — arbitrary processes execute under an AppContainer/lowbox or equivalently restricted Windows identity rather than the backend's unrestricted security context.
2. **Per-sandbox identity** — ACL-scoped resources use an application-issued unique restricting SID or unique AppContainer package SID so authority cannot be reused by another sandbox under the same user.
3. **Process-tree confinement** — a Windows Job Object contains descendants and uses `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`; process creation must bind Job membership before untrusted code executes.
4. **Filesystem confinement** — arbitrary process access is limited to sandbox-owned resources or explicitly staged/granted resources. Host workspace ACLs must not be widened as a convenience shortcut.
5. **No ambient secrets** — existing sandbox environment validation remains authoritative and arbitrary processes do not inherit the backend environment.
6. **No network by default** — restricted tokens and ACLs alone are insufficient. A Windows runtime may advertise `NetworkIsolation=true` only when an AppContainer/equivalent default-deny network boundary is natively proven.
7. **Fail closed** — unsupported mount/network/interpreter behavior is rejected rather than approximated with weaker host execution.
8. **Truthful capabilities** — runtime capability bits change only after implementation plus Windows-native behavior evidence.

## Workstreams

| Slice | Scope | Status | Evidence / exit criterion |
|---|---|---|---|
| 12A | Native security primitives | **COMPLETE** | PR #127 passed hardened Windows-native restricted-token, per-sandbox SID, cross-sandbox ACL denial, Job Object, full Quality/Security, and multi-architecture container validation; squash-merged as `c68ba013d3ad41ff2044646733d38ab981b3dc87`. |
| 12B | First-party protocol-v2 Windows runtime | **COMPLETE** | PR #128 final head `282fbc0f` passed the native Windows confinement suite, backend format/vet/tests/race, Chromium, frontend, Helm, Security, Windows plugin/desktop compatibility, and applicable container builds; squash-merged as `43c1c42b`. |
| 12C | Persistent stdio MCP/plugin confinement | **IN PROGRESS** | Clean PR #137 is replayed directly on current `main` and introduces a managed persistent-process seam plus a Windows AppContainer/Job-backed stdio runner. Earlier draft #134 passed the real Windows plugin lifecycle and existing native Windows sandbox suite; #137 exact-final-head full validation is required before merge. |
| 12D | Adversarial Windows assurance | NOT STARTED | Expand direct extension/runtime evidence for descendant escape/teardown, cross-sandbox authority reuse, reparse/hard-link/rename behavior, network escape, secret inheritance, cancellation, staging edge cases, and mode-policy behavior. |
| 12E | Documentation and completion | NOT STARTED | Finalize operator/runtime docs and mark Phase 12 complete only after 12C is merged and 12D evidence closes the remaining Windows assurance gaps. |

## Phase 12A — merged foundation

PR #127 established the reusable Windows primitives:

- cryptographically random per-sandbox restricting SIDs;
- restricted primary-token creation with `CreateRestrictedToken`;
- kill-on-close Job Objects;
- SID-scoped DACL helpers that preserve the normal identity access check;
- native tests proving one sandbox cannot reuse another sandbox's ACL authority.

The first #127 implementation used the well-known Restricted Code SID. Manual review caught that the identity was reusable by other restricted processes under the same Windows user. It was replaced before merge with a unique application-issued restricting SID and cross-sandbox denial coverage. Final head `8b491a80de9543afcd259d5d5959794bb4a61eaa` passed the full applicable gate set before squash merge as `c68ba013`.

## Phase 12B — merged first-party Windows runtime

PR #128 added the first Windows implementation of the protocol-v2 `Runtime` interface.

### Runtime boundary

- requires Windows 10 or newer and feature-detects the required stable AppContainer APIs;
- creates an ephemeral AppContainer profile per runtime session;
- launches arbitrary processes with `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` and zero network capabilities;
- attaches a kill-on-close Job Object through `PROC_THREAD_ATTRIBUTE_JOB_LIST` at process creation rather than assigning after start;
- constrains inherited handles to stdin/stdout/stderr with `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`;
- terminates remaining Job descendants when the root command exits, cancels, or times out;
- uses a minimal runtime-owned environment and validates every explicit environment entry with the sandbox secret policy;
- enforces wall-time plus bounded stdout/stderr without advertising CPU/memory/PID/disk quotas.

The AppContainer package SID is the 12B process/filesystem/network identity. The Phase 12A random restricting-SID primitive remains available for explicitly SID-scoped resources, but it is not layered on top of the AppContainer process token because doing so would also require explicit read/execute grants for every executable and DLL loaded by the child.

### Workspace policy

PR #128 intentionally does not alter user workspace ACLs.

- no mount creates an ephemeral writable workspace inside the AppContainer profile;
- one `read_only` mount is copied into the AppContainer profile and exposed read-only;
- `read_write` and `read_write_no_delete` arbitrary-process mounts fail closed;
- staging is limited to 20,000 entries / 256 MiB, rejects reparse points, multiply-linked files and special files, validates destination containment, and closes each source handle immediately;
- after every source file is opened, `GetFinalPathNameByHandle` must resolve that opened handle beneath the canonical source root before bytes are copied;
- the staged workspace uses a protected DACL: current backend user, LocalSystem, and Administrators retain cleanup authority; the AppContainer package SID receives only read/traverse/execute authority;
- `home` and `tmp` are sandbox-owned writable directories.

The copy/staging limit is an admission/operational bound only. `DiskLimit` remains `false` because 12B does not enforce a physical runtime disk quota.

### Final native evidence

Final head `282fbc0fc366c3791f31b7e1d841250971b0b980` passed:

- the dedicated Windows confinement suite, including AppContainer token state, read-only staged workspace, unrelated-host-file read/write denial, ambient-secret absence, loopback denial, descendant teardown, active-`Destroy` synchronization, and retryable cleanup failure handling;
- `cmd/sandboxd` Windows compilation/testing;
- backend formatting, vet, unit/integration tests, and race detector;
- Windows plugin lifecycle and desktop bindings;
- frontend lint/unit/build and full Chromium Playwright smoke;
- Helm validation;
- dependency audit plus Go and JavaScript/TypeScript CodeQL;
- applicable Linux multi-architecture frontend/backend container builds.

PR #128 then squash-merged as `43c1c42bebf245ded6722d742fcb3ea0a71b4502`.

## Phase 12C — active persistent extension confinement

PR #137 moves persistent stdio MCP/plugin processes from the sanitized Windows compatibility boundary to native AppContainer confinement while preserving their existing streaming JSON-RPC lifecycle. It is the clean successor to development draft #134 and was replayed directly onto `main` `54a9a1849340f2f9c3ca1a393b979c5e739fc2fe` before final validation.

### Managed process seam

`backend/internal/sandbox/process.go` now defines a narrow `CommandProcess` lifecycle surface:

- `StdinPipe`, `StdoutPipe`, `StderrPipe`;
- `Start` and `Wait`;
- `Kill` for process-tree termination.

The host and Linux Bubblewrap implementations remain adapters around `*exec.Cmd`. MCP already consumed only the pipe/start/wait subset. Plugin forced shutdown now calls `CommandProcess.Kill()` rather than reaching through `cmd.Process`, which lets Windows terminate the whole Job Object without rewriting protocol code.

### Windows extension launch boundary

The Windows platform adapter:

- feature-detects Windows 10+ AppContainer APIs;
- in `required` mode, fails closed if native confinement is unavailable;
- in `auto` mode, uses native Windows confinement when available and falls back only when the OS/API backend itself is unavailable;
- validates explicit extension environment values with the existing credential-sensitive policy before native launch;
- creates a unique ephemeral AppContainer profile for each persistent process;
- stages non-System32 command directories read-only into the profile rather than widening ACLs on the original host path;
- stages an explicit/inferred working directory read-only when needed;
- remaps absolute arguments only when they resolve beneath staged roots and rejects unrelated absolute host arguments;
- gives the child runtime-owned `home`/`tmp` and a minimal Windows environment rather than ambient backend state;
- supplies AppContainer security capabilities, Job membership, and the explicit stdio handle list in the same process-creation attribute list;
- launches `.cmd`/`.bat` through the System32 command processor while keeping the staged script path inside the AppContainer profile;
- terminates the Job on cancellation, forced plugin shutdown, and root-process completion so descendants cannot outlive the persistent extension lifecycle;
- performs immediate bounded profile cleanup and, if Windows temporarily blocks cleanup, keeps retrying the same idempotent cleanup on a delayed timer rather than abandoning an unreachable profile.

The staging limits currently reuse the 12B admission bounds. Complex command lines that hide host paths inside opaque string arguments are not rewritten; native AppContainer ACLs remain the enforcement boundary, and broader adversarial/path-shape assurance is tracked in 12D.

### Current 12C validation evidence

Development draft #134 provided useful behavior evidence even though its first Linux backend job stopped at formatting:

- the Windows plugin lifecycle job passed end-to-end through `NewHostCommandRunner()`, which now selects the native Windows extension backend in `auto` mode;
- the existing dedicated Windows sandbox package compiled the new Windows files and passed its native suite;
- frontend and Helm checks passed;
- the Linux failure was limited to non-canonical `gofmt` output in `process.go` and `extension_process_windows_lifecycle.go`.

Those formatting deltas were corrected with canonical `gofmt`, and the intended 11-file 12C state was replayed onto current `main` as clean commit `6480498052eca8036607b13021f5baf1d8e01a64` before PR #137 was opened. Diagnostic evidence from #134 does not replace #137 exact-final-head Quality, Security, Windows, Chromium/race, Helm, and applicable container validation.

## Integration constraints / remaining design work

- Python/Node runtime availability is not assumed merely because an interpreter is installed. Persistent extensions may stage a command bundle and related working-directory resources, but unsupported path shapes still fail closed rather than receiving broad host ACL grants.
- Explicit credential-sensitive extension environment values remain rejected under native confinement unless the existing narrow `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` operator override is set. The desired long-term path remains brokered/scoped credentials rather than raw environment secrets.
- Destination allowlisting is not implemented. Windows AppContainer execution remains no-network by default.
- Memory, CPU, process-count, and physical disk quotas remain unimplemented and unadvertised.
- `sandboxd` remains operator-run rather than automatically packaged into desktop/server distributions; packaging remains tracked under the broader first-party runtime/deployment work.
- The 12B/12C staged-copy path binds each opened source handle back to the canonical source root before copying. Broader workspace-registry/path-component TOCTOU assurance outside staged-copy flows remains tracked under Phases 5 and 17.
- 12D still needs direct adversarial extension evidence beyond the current end-to-end plugin lifecycle and shared native-primitives evidence.

## Validation policy

Phase 12 requires the normal repository Quality Gate and Security Scan plus Windows-native confinement/lifecycle jobs. Applicable container builds must also pass. No control becomes complete from API usage, cross-compilation, or capability flags alone.

## Progress log

- **2026-08-12 — PR #127:** opened Phase 12A native Windows primitives.
- **2026-08-12 — #127 audit:** replaced globally reusable Restricted Code SID ACL authority with unique per-sandbox SIDs and added cross-sandbox denial coverage.
- **2026-08-12 — #127:** final hardened head passed Quality, Security, native Windows, Chromium/race, Helm, and multi-architecture container validation; squash-merged as `c68ba013`.
- **2026-08-12 — PR #128:** opened first-party Windows AppContainer runtime implementation.
- **2026-08-12 — #128 audit:** closed the staging handle/rename escape and made failed profile cleanup retryable rather than unreachable.
- **2026-08-13 — #128 final validation:** exact head `282fbc0f` passed the full Quality, Security, Windows-native, Chromium/race, Helm, and applicable container gates; squash-merged as `43c1c42b`.
- **2026-08-13 — #134:** opened development draft for native persistent Windows extension confinement; initial Windows plugin/native jobs passed and Linux formatting differences were corrected.
- **2026-08-13 — #137:** clean-replayed the intended 12C state onto current `main` and opened the authoritative validation PR. Exact-final-head full validation remains pending.
