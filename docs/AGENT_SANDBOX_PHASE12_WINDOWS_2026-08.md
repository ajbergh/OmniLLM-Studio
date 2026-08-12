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
2. **Process-tree confinement** — a Windows Job Object contains descendants and uses `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` so teardown kills the process tree.
3. **Filesystem confinement** — writable workspace authority is expressed through Windows ACLs and the restricted identity; access outside granted roots must fail in native tests.
4. **No ambient secrets** — existing sanitized environment and sandbox credential-rejection rules remain authoritative.
5. **Fail closed** — `required` mode must not claim native Windows confinement until the selected launch path actually uses these controls.
6. **Truthful capabilities** — runtime capability bits change only after the corresponding native implementation and tests exist.

## Workstreams

| Slice | Scope | Status | Evidence / exit criterion |
|---|---|---|---|
| 12A | Native security primitives | **IN PROGRESS** | PR #127 adds restricted token, kill-on-close Job Object, Restricted Code ACL merge helpers, and dedicated `windows-latest` tests. Exit requires the native CI job plus normal Quality/Security gates to pass. |
| 12B | First-party protocol-v2 Windows runtime | NOT STARTED | Implement Windows `NewLocalRuntime`, session lifecycle/cancellation, scratch/workspace ACL setup, bounded exec/output, and truthful capabilities. Native worker tests required. |
| 12C | Persistent stdio MCP/plugin confinement | NOT STARTED | Integrate restricted-token + Job Object confinement into the shared extension process seam without breaking streaming stdio. `auto` may select native Windows confinement only when genuinely available; `required` must fail closed otherwise. |
| 12D | Adversarial Windows assurance | NOT STARTED | Descendant escape/teardown, ACL escape, symlink/junction/reparse behavior, secret inheritance, cancellation, and mode-policy tests on Windows. |
| 12E | Documentation and completion | NOT STARTED | Update runtime/operator docs and the primary roadmap with final PR/commit/native validation evidence. |

## Phase 12A implementation notes

PR #127 adds Windows-only primitives in `backend/internal/sandbox/windows_security_windows.go`:

- a restricted primary token derived from the current process token using `CreateRestrictedToken`, `DISABLE_MAX_PRIVILEGE`, and the well-known Restricted Code SID;
- a Job Object configured with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`;
- a DACL merge helper that grants the Restricted Code SID explicit access while preserving the existing identity DACL, so restricted-token filesystem access must satisfy both checks.

Native tests in `backend/internal/sandbox/windows_security_windows_test.go` currently prove:

- the derived token is reported by Windows as restricted;
- the Job Object has `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` set;
- under restricted-token impersonation, writes succeed inside an explicitly writable ACL-scoped directory and fail in its traversal-only parent.

The Quality Gate adds a dedicated `Sandbox confinement primitives (Windows)` job so this evidence is visible and cannot be inferred from cross-compilation.

## Integration constraints discovered during design

- The existing `CommandRunner` returns `*exec.Cmd`. Assigning a process to a Job Object only after `cmd.Start()` creates an avoidable escape window, so Phase 12C must not use a post-start assignment and call that complete.
- The Windows integration should bind the process to confinement at creation time, or use a small trusted launcher that performs restricted-token creation and Job Object membership before untrusted extension code can run.
- `backend/internal/sandbox/local_runtime_other.go` still rejects all non-Linux first-party runtimes. Phase 12B must add a Windows implementation rather than merely changing extension policy.
- `sandboxd` is currently an operator-run worker rather than a desktop/server-packaged worker. Packaging remains tracked under the broader first-party runtime/deployment work and is not silently redefined as Phase 12 completion.

## Validation policy

Phase 12 requires the normal repository Quality Gate and Security Scan plus the dedicated Windows native confinement job. Cross-compilation alone is insufficient. No control is marked complete from API usage or build success without behavior-level Windows evidence.

## Progress log

- **2026-08-12:** Phase 12 branch created from `main` `2a8709db`.
- **2026-08-12:** Added Windows restricted-token, Job Object, and Restricted Code ACL primitives plus native tests.
- **2026-08-12:** Added a dedicated `windows-latest` Quality Gate job for explicit confinement evidence.
- **2026-08-12 — PR #127:** Opened focused Phase 12A foundation PR; native validation pending.
