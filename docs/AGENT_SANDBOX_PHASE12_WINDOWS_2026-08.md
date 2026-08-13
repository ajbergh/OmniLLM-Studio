# Agent Sandbox Phase 12 — Windows Native Confinement

> **Status:** IN PROGRESS
>
> **Started:** 2026-08-12
>
> **Active branch:** `agent/sandbox-windows-runtime-12b-20260812`
>
> **Active PR:** #128
>
> **12B integration checkpoint:** replayed directly onto `main` at `788df6e0944a1e2608203e79cd0d29e44eeb0875`; exact-final-head validation remains required before merge.

This file is the detailed durable tracker for Phase 12 of `AGENT_SANDBOX_ROADMAP_2026-08.md`. Phase 12 is not complete until both the first-party protocol-v2 Windows runtime and persistent local extension processes use native Windows confinement with Windows-native test evidence.

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
| 12B | First-party protocol-v2 Windows runtime | **IN PROGRESS** | PR #128 adds Windows AppContainer `NewLocalRuntime`, creation-time Job membership, explicit inherited handles, no-network AppContainer capabilities, bounded execution, RO workspace staging, and native end-to-end isolation tests. `fa22da80` passed the dedicated Windows confinement job; exact-final-head full repository validation is still required after canonical formatting/replay. |
| 12C | Persistent stdio MCP/plugin confinement | NOT STARTED | Integrate Windows identity + AppContainer/filesystem/network + process-tree confinement into the shared persistent extension seam without breaking streaming stdio. `auto` may select native Windows confinement only when genuinely available; `required` must fail closed otherwise. |
| 12D | Adversarial Windows assurance | NOT STARTED | Descendant escape/teardown, cross-sandbox authority reuse, reparse/hard-link/rename behavior, network escape, secret inheritance, cancellation, and mode-policy tests on Windows. |
| 12E | Documentation and completion | NOT STARTED | Update runtime/operator docs and the primary roadmap with final PR/commit/native validation evidence only after both runtime surfaces are natively validated. |

## Phase 12A — merged foundation

PR #127 established the reusable Windows primitives:

- cryptographically random per-sandbox restricting SIDs;
- restricted primary-token creation with `CreateRestrictedToken`;
- kill-on-close Job Objects;
- SID-scoped DACL helpers that preserve the normal identity access check;
- native tests proving one sandbox cannot reuse another sandbox's ACL authority.

The first #127 implementation used the well-known Restricted Code SID. Manual review caught that the identity was reusable by other restricted processes under the same Windows user. It was replaced before merge with a unique application-issued restricting SID and cross-sandbox denial coverage. Final head `8b491a80de9543afcd259d5d5959794bb4a61eaa` then passed Quality Gate, Security Scan, and applicable container validation before squash merge as `c68ba013`.

## Phase 12B — active implementation

PR #128 adds the first Windows implementation of the existing protocol-v2 `Runtime` interface.

### Runtime boundary

- requires Windows 10 or newer and feature-detects the required stable AppContainer APIs;
- creates an ephemeral AppContainer profile per runtime session;
- launches arbitrary processes with `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` and **zero network capabilities**;
- attaches a kill-on-close Job Object through `PROC_THREAD_ATTRIBUTE_JOB_LIST` at process creation rather than assigning after start;
- constrains inherited handles to stdin/stdout/stderr with `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`;
- terminates remaining Job descendants when the root command exits, cancels, or times out;
- uses a minimal runtime-owned environment and validates every explicit environment entry with the existing sandbox secret policy;
- enforces wall-time plus bounded stdout/stderr without advertising CPU/memory/PID/disk quotas.

The AppContainer package SID is the 12B process/filesystem/network identity. The Phase 12A random restricting-SID primitive remains useful for explicitly SID-scoped resources, but it is **not** applied as a process-wide restricting SID in 12B because that would also require the unique SID to be granted read/execute authority on every executable and DLL loaded by the child.

### Workspace policy

PR #128 intentionally does not alter user workspace ACLs.

- no mount creates an ephemeral writable workspace inside the AppContainer profile;
- one `read_only` mount is copied into the AppContainer profile and exposed read-only;
- `read_write` and `read_write_no_delete` arbitrary-process mounts fail closed;
- staging is limited to 20,000 entries / 256 MiB, rejects reparse points, multiply-linked files and special files, validates destination containment, and closes each source handle immediately;
- after every source file is opened, `GetFinalPathNameByHandle` must resolve that opened handle beneath the canonical source root before any bytes are copied, preventing a checked-path/reparse-or-rename swap from staging an unrelated host file;
- the staged workspace uses a protected DACL: current backend user, LocalSystem, and Administrators retain cleanup authority; the AppContainer package SID receives only read/traverse/execute authority;
- `home` and `tmp` are sandbox-owned writable directories.

The copy/staging limit is an admission/operational bound only. `DiskLimit` remains `false` because 12B does not yet enforce a physical runtime disk quota.

### Command support

- explicit command mode may execute a staged workspace executable or a System32 command basename;
- shell code maps to System32 `cmd.exe`;
- absolute/volume command paths and workspace traversal are rejected;
- Python and JavaScript language shortcuts remain fail-closed until an AppContainer-readable interpreter/package strategy is implemented and natively tested.

### Native acceptance evidence required

`backend/internal/sandbox/local_runtime_windows_test.go` stages the Windows Go test executable itself and runs that copy inside the runtime. The native suite requires behavior-level evidence that:

- `TokenIsAppContainer` is true;
- the staged workspace cannot be written;
- an unrelated host file outside the AppContainer profile cannot be read or written;
- an ambient parent secret is absent;
- a connection to a parent loopback listener is denied;
- the original host source workspace remains unchanged;
- a descendant spawned by the root process is terminated with the Job when the root execution completes and cannot write a delayed marker;
- `Destroy` cancels an active execution, waits for process/pipe teardown, removes the AppContainer profile data, and makes the session unreachable before returning;
- `cmd/sandboxd` compiles/tests on `windows-latest`, not only `internal/sandbox`.

The same Windows job also continues the Phase 12A primitive tests. Cross-compilation alone is not completion evidence.

On head `fa22da808423b9cd47652d74b639f8eda2d052aa`, the dedicated Windows confinement job passed all of those runtime assertions, plugin lifecycle passed, frontend/Helm checks passed, Security Scan passed, and applicable container builds passed. The repository Quality Gate still failed because the Linux backend formatting check found the new Windows files were not canonical `gofmt` output. The files were subsequently canonicalized and PR #128 was clean-replayed onto current `main`; therefore the earlier native pass is useful diagnostic evidence but does **not** replace exact-final-head validation.

## Integration constraints / remaining design work

- Persistent local extensions still use the compatibility runner on Windows; 12C must preserve streaming stdin/stdout while applying confinement **before** extension code executes.
- Python/Node runtime availability inside AppContainer is not assumed from host installation. A future interpreter solution must be explicit, package-readable, and tested.
- Read/write arbitrary-process access remains intentionally unavailable. Journaled workspace tools remain the trusted mutation path.
- Destination allowlisting is not implemented. Windows 12B is no-network only.
- Memory, CPU, process-count, and physical disk quotas remain unimplemented and unadvertised.
- `sandboxd` remains operator-run rather than automatically packaged into desktop/server distributions; packaging remains tracked under the broader first-party runtime/deployment work.
- The 12B staged-copy path now binds each opened source handle back to the canonical source root before copying, closing the specific staging reparse/rename escape identified during review. Broader workspace-registry/path-component TOCTOU assurance outside this staged-copy flow remains tracked under Phases 5 and 17.

## Validation policy

Phase 12 requires the normal repository Quality Gate and Security Scan plus the dedicated Windows native confinement job. Applicable container builds must also pass. No control becomes complete from API usage, cross-compilation, or capability flags alone.

## Progress log

- **2026-08-12 — PR #127:** opened Phase 12A native Windows primitives.
- **2026-08-12 — #127 audit:** replaced globally reusable Restricted Code SID ACL authority with unique per-sandbox SIDs and added cross-sandbox denial coverage.
- **2026-08-12 — #127:** final hardened head passed Quality, Security, native Windows, Chromium/race, Helm, and multi-architecture container validation; squash-merged as `c68ba013`.
- **2026-08-12 — 12B:** fresh branch created from post-#127 `main`. When Image Studio advanced `main` to `1cc50515`, 12B was replayed as one clean commit on that exact base before PR creation.
- **2026-08-12 — PR #128:** opened first-party Windows AppContainer runtime implementation.
- **2026-08-12 — #128 native validation:** head `fa22da80` passed the complete dedicated Windows confinement suite, plugin lifecycle, frontend/Helm, Security, and containers; Quality failed only because the new Windows files needed canonical `gofmt` formatting.
- **2026-08-12 — #128 integration refresh:** canonical formatting was applied and the intended nine-file PR state was clean-replayed directly onto `main` `788df6e0`; exact-final-head full validation remains pending.
