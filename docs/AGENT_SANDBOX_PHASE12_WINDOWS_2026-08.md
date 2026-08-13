# Agent Sandbox Phase 12 — Windows Native Confinement

> **Status:** IN PROGRESS
>
> **Started:** 2026-08-12
>
> **Branch:** `agent/sandbox-windows-confinement-20260812`
>
> **PR:** #127
>
> **Base:** `main` at `2a8709dba42bdd23b5a08fab1c4124a8603ae3ee`

This file is the detailed durable tracker for Phase 12 of `AGENT_SANDBOX_ROADMAP_2026-08.md`. Phase 12 is not complete until both the first-party protocol-v2 Windows runtime and persistent local extension processes use native Windows confinement with Windows-native test evidence.

## Required security properties

1. **Restricted identity/token** — sandboxed processes run under a restricted access token rather than the backend's unrestricted process token.
2. **Per-sandbox identity** — filesystem authority is bound to an application-issued unique restricting SID; a grant to one sandbox cannot be reused by another restricted process under the same user.
3. **Process-tree confinement** — a Windows Job Object contains descendants and uses `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` so teardown kills the process tree.
4. **Filesystem confinement** — writable workspace authority is expressed through Windows ACLs and the sandbox-specific restricting SID; access outside granted roots must fail in native tests.
5. **No ambient secrets** — existing sanitized environment and sandbox credential-rejection rules remain authoritative.
6. **No network by default** — restricted tokens and ACLs alone do not satisfy network isolation; the Windows execution runtime must use AppContainer or an equivalent enforceable no-network boundary before advertising `NetworkIsolation=true`.
7. **Fail closed** — `required` mode must not claim native Windows confinement until the selected launch path actually uses these controls.
8. **Truthful capabilities** — runtime capability bits change only after the corresponding native implementation and tests exist.

## Workstreams

| Slice | Scope | Status | Evidence / exit criterion |
|---|---|---|---|
| 12A | Native security primitives | **IN PROGRESS** | PR #127 adds unique per-sandbox restricting SIDs, restricted tokens, kill-on-close Job Objects, SID-scoped DACL merge helpers, and dedicated `windows-latest` tests. Exit requires the hardened native CI job plus normal Quality/Security/container gates to pass. |
| 12B | First-party protocol-v2 Windows runtime | NOT STARTED | Implement Windows `NewLocalRuntime`, session lifecycle/cancellation, scratch/workspace isolation, default-deny network enforcement, bounded exec/output, and truthful capabilities. Native worker tests required. |
| 12C | Persistent stdio MCP/plugin confinement | NOT STARTED | Integrate Windows sandbox identity + process-tree + filesystem/network confinement into the shared extension process seam without breaking streaming stdio. `auto` may select native Windows confinement only when genuinely available; `required` must fail closed otherwise. |
| 12D | Adversarial Windows assurance | NOT STARTED | Descendant escape/teardown, cross-sandbox ACL reuse, symlink/junction/reparse behavior, network escape, secret inheritance, cancellation, and mode-policy tests on Windows. |
| 12E | Documentation and completion | NOT STARTED | Update runtime/operator docs and the primary roadmap with final PR/commit/native validation evidence. |

## Phase 12A implementation notes

PR #127 adds Windows-only primitives in `backend/internal/sandbox/windows_security_windows.go`:

- a cryptographically random, unregistered SID for each sandbox restricting identity;
- a restricted primary token derived from the current process token using `CreateRestrictedToken` and `DISABLE_MAX_PRIVILEGE`, with exactly that sandbox-specific SID in the restricting SID list;
- a Job Object configured with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`;
- a generic SID-scoped DACL merge helper that preserves the existing identity DACL, so filesystem access must satisfy both the normal user check and the sandbox-specific restricting-SID check.

The initial proof used the well-known Restricted Code SID for ACL authority. Manual review caught that this identity would be shared by other restricted processes owned by the same Windows user. The branch was hardened before merge to use a unique restricting SID per sandbox instead.

Native tests in `backend/internal/sandbox/windows_security_windows_test.go` prove:

- the derived token is reported by Windows as restricted;
- independently generated sandbox SIDs differ;
- the Job Object has `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` set;
- sandbox A can write inside its explicitly writable ACL-scoped directory but not its traversal-only parent;
- sandbox B, under the same Windows user but a different restricting SID, cannot reuse sandbox A's writable ACL.

The Quality Gate adds a dedicated `Sandbox confinement primitives (Windows)` job so this evidence is visible and cannot be inferred from cross-compilation.

## Phase 12B target architecture

The target Windows runtime must combine the proven identity/process primitives with a real no-network and filesystem/process boundary. AppContainer is the preferred stable Windows security model because it is default-deny for network and filesystem access outside granted resources. Windows 11 also exposes an experimental composable process-sandbox API with AppContainer and bound-filesystem policies; that API may be feature-detected, but Phase 12 must not make an experimental Windows 11-only API a silent hard dependency or weaken containment when it is unavailable.

Process creation must bind confinement before untrusted code can execute. Windows supports assigning a child to Job Objects through process-creation attributes on supported versions; a design that starts unrestricted code and only later calls `AssignProcessToJobObject` is not acceptable Phase 12 completion.

## Integration constraints discovered during design

- The existing `CommandRunner` returns `*exec.Cmd`. Assigning a process to a Job Object only after `cmd.Start()` creates an avoidable escape window, so Phase 12C must not use a post-start assignment and call that complete.
- The Windows integration should bind the process to confinement at creation time, or use a small trusted launcher that performs sandbox identity/AppContainer setup and Job Object membership before untrusted extension code can run.
- `backend/internal/sandbox/local_runtime_other.go` still rejects all non-Linux first-party runtimes. Phase 12B must add a Windows implementation rather than merely changing extension policy.
- `sandboxd` is currently an operator-run worker rather than a desktop/server-packaged worker. Packaging remains tracked under the broader first-party runtime/deployment work and is not silently redefined as Phase 12 completion.

## Validation policy

Phase 12 requires the normal repository Quality Gate and Security Scan plus the dedicated Windows native confinement job. Cross-compilation alone is insufficient. No control is marked complete from API usage or build success without behavior-level Windows evidence.

## Progress log

- **2026-08-12:** Phase 12 branch created from `main` `2a8709db`.
- **2026-08-12:** Added Windows restricted-token, Job Object, and SID-scoped ACL primitives plus native tests.
- **2026-08-12:** Added a dedicated `windows-latest` Quality Gate job for explicit confinement evidence.
- **2026-08-12 — PR #127:** Opened focused Phase 12A foundation PR.
- **2026-08-12 — #127 audit:** Native proof initially passed, then manual security review identified globally reusable Restricted Code SID ACL authority. Replaced it with unique per-sandbox restricting SIDs and added cross-sandbox ACL denial coverage; final-head validation restarted.
