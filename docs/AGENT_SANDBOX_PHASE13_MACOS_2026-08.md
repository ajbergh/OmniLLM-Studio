# Agent Sandbox Phase 13 — macOS Native Confinement — August 2026

> **Status:** IN PROGRESS — 13A merged; 13B implementation under native validation
>
> Phase 13 is intentionally split into native-evidence slices. Controls are advertised only after the first-party implementation enforces them and native macOS CI proves the behavior.

## Objective

Bring macOS to the same truthful confinement standard used for Linux and Windows. Missing primitives and unsupported requested controls fail closed. A normal process-tree behavior test is not enough to advertise adversarial process-tree isolation; that remains a later assurance item.

## Phase 13A — Seatbelt foundation — merged in PR #159

Phase 13A established the fixed native launcher boundary:

```text
/usr/bin/sandbox-exec
```

It proved on `macos-latest` that a default-deny Seatbelt profile can:

- permit writes beneath an explicit canonicalized root;
- deny writes outside that root;
- deny loopback network access when no network operation is granted;
- reject invalid/non-directory policy roots;
- launch through a fixed system path with the sanitized subprocess environment.

13A intentionally allowed broad host reads for compatibility and therefore did not register a Darwin `NewLocalRuntime` or advertise filesystem isolation.

## Phase 13B — first-party local runtime — implementation in progress

Phase 13B wires Darwin into the protocol-v2 local runtime instead of the unsupported-platform stub.

### Runtime lifecycle

The Darwin runtime:

- requires `/usr/bin/sandbox-exec`; absence fails closed;
- creates a canonicalized per-session root below `LocalRuntimeConfig.ScratchRoot`;
- creates separate `workspace`, `home`, and `tmp` session roots;
- supports no workspace mount or one trusted `read_only` workspace mount;
- removes the per-session staging tree on destroy after registered executions finish cancellation teardown;
- preserves the existing TTL cleanup model.

### Workspace boundary

A trusted read-only workspace is **copied** into the session staging tree before model-directed execution. The live host workspace path is never granted to the child.

The staging pass is bounded to:

- at most 20,000 regular files;
- at most 256 MiB total staged bytes.

It fails closed on:

- symbolic links;
- hard links;
- non-regular files;
- source identity changes while copying;
- source size changes while copying;
- traversal outside the canonical source root.

Executable bits are preserved only as read/execute permissions on the staged copy. Non-executable files are staged read-only.

Writable/read-write host workspace mounts remain unsupported in this slice.

### Runtime Seatbelt profile

Unlike the 13A primitive profile, the 13B runtime profile does **not** grant host-wide `file-read*`.

Read access is limited to canonicalized session roots plus existing system/runtime roots needed for normal executable and dynamic-loader startup:

- `/System`
- `/usr`
- `/bin`
- `/sbin`
- `/dev`
- `/private/etc`
- Homebrew/MacPorts roots when present (`/opt/homebrew`, `/opt/local`)

Writes are limited to:

- `/dev/null`;
- the session `home` root;
- the session `tmp` root;
- the session workspace only when it is an ephemeral no-mount workspace.

A staged read-only workspace is not included in the Seatbelt write roots.

Network remains denied because the profile grants no network operations. Destination allowlists remain unsupported.

### Command and environment boundary

Executable resolution uses a fixed runtime path and allows only:

- executables inside the staged/ephemeral session workspace; or
- executables resolving below approved system runtime roots.

Absolute host executables outside those roots fail closed before launch.

The runtime environment is rebuilt from a small runtime-owned set (`PATH`, `HOME`, `TMPDIR`, `LANG`) plus validated explicit non-sensitive overrides. Credential/proxy-sensitive variables continue to be rejected through the shared sandbox environment policy, and Darwin additionally rejects runtime-owned path/home/temp overrides and all `DYLD_*` injection variables.

Ambient backend secrets are not inherited.

### Execution, limits, and cancellation

13B integrates the caller-known execution-reference contract merged in PR #155:

- supplied canonical execution IDs are preserved;
- omitted IDs are generated through the shared helper;
- duplicate active IDs fail closed;
- `Cancel(runtimeID, executionID)` addresses the active execution;
- finished/unknown IDs fail closed.

Execution uses:

- default 30-second wall time, narrowed by session/request limits;
- bounded stdout/stderr (default 1 MiB each, or lower configured limits);
- process-group cancellation for ordinary descendants;
- explicit teardown wait before session deletion.

Normal descendant cancellation is tested, but `ProcessTreeIsolation` remains **false** because a hostile descendant may attempt to detach into an independent process group/session. Phase 13D must resolve or formally constrain that case before the capability can be advertised.

### Truthful capabilities in 13B

The proposed Darwin runtime reports:

- `os_isolation = true`
- `filesystem_isolation = true`
- `network_isolation = true`
- `network_allowlist = false`
- `process_tree_isolation = false`
- `memory_limit = false`
- `cpu_limit = false`
- `pid_limit = false`
- `disk_limit = false`

Any `CreateRequest.Requirements` demanding one of the unsupported controls therefore fails closed through the shared capability check.

### Native 13B assurance

A dedicated `macOS Sandbox Runtime Assurance` workflow runs on `macos-latest` and executes both the 13A Seatbelt primitive suite and the 13B `TestDarwinLocalRuntime*` suite.

The 13B native suite covers:

- truthful capability reporting;
- allowed staged-workspace reads;
- read-only workspace write denial;
- ungranted host-file read denial;
- loopback network denial;
- ephemeral workspace write/read persistence within one session;
- stdout/stderr bounding;
- ambient secret non-inheritance;
- credential-sensitive, `DYLD_*`, and runtime-owned environment rejection;
- caller-known cancellation and ordinary descendant termination;
- rejection of symlink and hard-link staging;
- rejection of writable workspace mounts;
- rejection of unapproved host executables.

**13B is not complete until the exact final PR head passes this native suite plus the repository Quality Gate, Security Scan, and applicable container validation.**

## Phase 13C — persistent extension confinement

Still required:

- Darwin-specific `platformExtensionCommandContext` implementation;
- `required` uses Seatbelt or fails closed;
- `auto` selects the native backend when prerequisites are satisfied;
- no pre-confinement child execution window;
- sanitized/minimal environment and credential-sensitive explicit environment rejection;
- stdio lifecycle compatibility for MCP and local plugins;
- native descendant termination plus host-file/network denial evidence.

## Phase 13D — adversarial assurance and completion review

Still required:

- path-component/symlink/rename escape attempts around session roots;
- workspace-source mutation races beyond the current copy-time identity checks;
- writable-root aliasing/canonicalization attacks;
- cross-runtime authority reuse attempts;
- detached process/session escape attempts;
- cancellation, timeout, and forced teardown under adversarial descendants;
- persistent-extension equivalents;
- exact final-head Quality, Security, native macOS, race/Playwright, Helm, and applicable container validation.

Phase 13 may be marked complete only after both arbitrary sandbox execution and persistent extension processes have native macOS confinement evidence and the final capability report matches that evidence.

## Known platform constraint

`/usr/bin/sandbox-exec` is a platform prerequisite, not a capability OmniLLM can emulate safely. The implementation must continue to fail closed if a future macOS release or deployment image removes or disables it. Native CI is therefore part of the ongoing compatibility contract rather than a one-time discovery check.
