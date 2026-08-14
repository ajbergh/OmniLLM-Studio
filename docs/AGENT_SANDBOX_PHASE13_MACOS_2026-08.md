# Agent Sandbox Phase 13 — macOS Native Confinement — August 2026

> **Status:** COMPLETE — 13A through 13D merged; final adversarial assurance landed in PR #166 as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17`
>
> Phase 13 is intentionally split into native-evidence slices. Controls are advertised only after the first-party implementation enforces them and native macOS CI proves the behavior.

## Objective

Bring macOS to the same truthful confinement standard used for Linux and Windows. Missing primitives and unsupported requested controls fail closed. A normal process-tree behavior test is not enough to advertise adversarial process-tree isolation; that remains an explicit limitation unless a stronger native teardown primitive is implemented and proven.

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

## Phase 13B — first-party local runtime — merged in PR #162

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

macOS path resolution also requires directory-data access on parent components. The runtime grants `file-read-data` and metadata only to the exact ancestor directories of approved roots; sibling file contents remain outside the `file-read*` subpath grants and are denied by native tests.

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

Normal descendant cancellation is tested, but `ProcessTreeIsolation` remains **false** because a hostile descendant may detach into an independent process group/session.

### Truthful capabilities in 13B

The Darwin runtime reports:

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

The dedicated `macOS Sandbox Runtime Assurance` workflow runs on `macos-latest` and covers truthful capability reporting, allowed staged reads, denied host reads/workspace writes/network, ephemeral workspace persistence, output bounds, ambient-secret non-inheritance, environment rejection, caller-known cancellation, ordinary descendant termination, unsafe staging rejection, mount policy, and executable policy.

PR #162's exact final head passed native macOS runtime assurance, Quality Gate, Security Scan, Helm validation, applicable frontend/backend container validation, and final review/diff checks before squash merge as `840b00bb6d2b74d1a88eb1fd910d06dab64118a2`.

## Phase 13C — persistent extension confinement — merged in PR #164

Phase 13C adds a Darwin-specific `platformExtensionCommandContext` without changing MCP/plugin callers or the shared `CommandProcess` streaming lifecycle.

### Policy composition

The existing operator policy remains authoritative:

```text
OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off
```

On Darwin:

- `required` uses the fixed `/usr/bin/sandbox-exec` native backend and fails closed if it is unavailable;
- `auto` selects the same native backend when Seatbelt is available;
- `off` remains the explicit sanitized-host compatibility boundary;
- unsupported platforms retain their existing fail-closed `required` behavior.

No child is launched before the Seatbelt profile is supplied to the fixed system launcher.

### Extension filesystem and network boundary

Each native persistent extension receives a unique runtime-owned `home` and `tmp` root.

Read access is limited to:

- explicit system/runtime roots already used by 13B;
- the canonical extension command directory;
- the canonical configured working directory when it differs from the command directory;
- the extension's own home/tmp roots.

Write access is limited to the unique home/tmp roots and `/dev/null`. Command and working-directory content stays read-only.

No network operation is granted. Loopback access is therefore denied along with external network access.

One extension's scratch/home authority is not added to another extension's profile. Native tests exercise cross-extension read denial.

### Environment boundary

13C reuses the existing native-extension sensitive-environment policy:

- credential/proxy-sensitive explicit values are rejected by default;
- `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` remains an explicit transitional operator override;
- ambient backend secrets are not inherited;
- Darwin additionally rejects confinement-owned `PATH`, `HOME`, temp/shell keys and all `DYLD_*` injection variables;
- runtime-owned `PATH`, `HOME`, `TMPDIR`, and `LANG` values are reconstructed before launch.

### Process lifecycle

The Darwin extension process preserves streaming stdin/stdout/stderr and uses a dedicated process group so ordinary descendants are terminated on context cancellation or explicit `Kill` before the scratch root is removed.

The scratch cleanup path is retained independently from its canonicalized Seatbelt path so a failure while resolving the newly created temporary root cannot orphan the original directory.

As with 13B, this ordinary descendant behavior does **not** justify a broader process-tree confinement claim against deliberately detached sessions.

### Native 13C assurance

A dedicated `macOS Extension Sandbox Assurance` workflow runs on `macos-latest` and covers:

- `auto` native selection and `required` behavior;
- persistent stdio request/response lifecycle;
- extension command-directory write denial;
- per-extension home write success;
- unrelated host read/write denial;
- cross-extension scratch read denial;
- ambient secret non-inheritance;
- loopback network denial;
- sensitive/`DYLD_*`/runtime-owned environment rejection;
- context cancellation and ordinary descendant teardown;
- continued explicit `off` compatibility behavior.

PR #164's exact final head passed native extension/runtime assurance, Quality Gate, Security Scan, Helm and container validation, and final review/diff checks before squash merge as `44f410793a70444963ec1eecb989b15df159b5f1`.

## Phase 13D — adversarial assurance and completion review — merged in PR #166

Phase 13D adds direct adversarial evidence for:

- deterministic workspace-source identity swaps between observation and open;
- writable-root symlink alias attempts against unrelated host paths;
- cross-runtime authority reuse attempts;
- deliberately detached runtime descendants;
- deliberately detached persistent-extension descendants;
- preservation of Seatbelt filesystem/network confinement after `setsid` detachment.

The first native 13D adversarial run passed on its stacked implementation head. It also demonstrated the truthful limitation: a deliberately detached descendant can outlive ordinary process-group cancellation/root kill while remaining Seatbelt-confined. Therefore `process_tree_isolation` must remain false unless a stronger native teardown mechanism is implemented and proven.

PR #166 passed its exact final-head repository/native gates and merged as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17`, closing Phase 13. The detached-process limitation remains documented and `process_tree_isolation` remains false.

## Known platform constraint

`/usr/bin/sandbox-exec` is a platform prerequisite, not a capability OmniLLM can emulate safely. The implementation must continue to fail closed if a future macOS release or deployment image removes or disables it. Native CI is therefore part of the ongoing compatibility contract rather than a one-time discovery check.
