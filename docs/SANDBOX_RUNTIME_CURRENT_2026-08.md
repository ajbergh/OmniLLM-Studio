# Sandbox Runtime — Current Platform Status (August 2026)

This document records the current first-party sandbox and persistent-extension behavior after Windows Phase 12 and completed macOS Phase 13. It supplements the older `SANDBOX_RUNTIME.md` historical snapshot.

## Shared protocol-v2 boundary

The backend Broker owns public sandbox sessions and talks to an authenticated protocol-v2 worker. The model does not call worker endpoints directly.

Runtime configuration uses:

```text
OMNILLM_SANDBOX_URL=http://127.0.0.1:8090
OMNILLM_SANDBOX_TOKEN=<long-random-service-token>
```

The service token is backend/runtime state, not model-facing data. Plain HTTP is limited to loopback endpoints; non-loopback runtime URLs require HTTPS and redirects are rejected.

The first-party worker is `backend/cmd/sandboxd`.

Caller-known canonical execution IDs are supported end-to-end. A caller may choose the execution reference before dispatch, active duplicate IDs fail closed, and `Cancel(runtime_id, execution_id)` addresses the exact active execution. This contract was completed in PR #155.

## Linux first-party runtime

Linux uses Bubblewrap plus an operator-prepared read-only root filesystem configured with `OMNILLM_SANDBOX_ROOTFS`.

Current enforced controls:

- OS/process namespace isolation;
- read-only runtime filesystem plus explicit trusted workspace/scratch mounts;
- no-network namespace isolation;
- process-tree/session teardown;
- session TTL cleanup;
- wall-time bounds;
- stdout/stderr bounds.

The Linux first-party runtime does not advertise destination allowlisting, memory quota, CPU quota, PID quota, or physical disk quota enforcement. The approved next quota slice is cgroup-v2 capability detection followed by per-execution PID and memory controllers; unsupported or undelegated hosts must remain fail closed.

## Windows first-party runtime

Windows 10+ uses native AppContainer plus Job Objects.

Current enforced controls:

- unique ephemeral AppContainer profile/package SID per runtime session;
- AppContainer security capabilities applied at process creation;
- zero AppContainer network capabilities;
- Job Object membership applied at process creation;
- process-count quota enforcement with `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` when `resources.max_processes` is non-zero;
- aggregate process-tree committed-memory enforcement with `JOB_OBJECT_LIMIT_JOB_MEMORY` when `resources.memory_bytes` is non-zero;
- explicit inherited stdio handle list;
- process-tree teardown on root completion, cancellation, timeout, or session destruction;
- runtime-owned minimal environment rather than ambient backend inheritance;
- bounded wall time, stdout, and stderr;
- retryable AppContainer profile cleanup.

The Windows runtime advertises `pid_limit=true` after PR #171 proved `MaxProcesses=1` prevents a nested child process from running. PR #172 adds `memory_limit=true` only after native `windows-latest` evidence proved that a descendant starts inside the same Job and a 512 MiB `VirtualAlloc(MEM_RESERVE|MEM_COMMIT)` request is synchronously denied under a 256 MiB aggregate Job memory ceiling. CPU, physical-disk, and destination-allowlist capability bits remain false.

### Windows workspace policy

- No project mount creates an AppContainer-owned writable scratch workspace.
- One `read_only` project workspace may be staged into AppContainer-owned storage.
- Arbitrary-process `read_write` and `read_write_no_delete` project mounts fail closed.
- Staging is bounded to 20,000 entries and 256 MiB.
- Reparse points/junctions, hard links, special files, traversal, and destination escapes are rejected.
- After a source file is opened, the final handle path must still resolve beneath the canonical source root before bytes are copied.
- The staged workspace is read-only to the AppContainer package SID; AppContainer-owned home/tmp remain writable.

### Windows language scope

Explicit command execution and shell code are supported. Shell code uses System32 `cmd.exe`.

Protocol-v2 Python and JavaScript shortcuts remain fail closed until an AppContainer-readable interpreter/package design is intentionally implemented and natively tested.

## macOS first-party runtime

macOS Phase 13 uses the fixed system `/usr/bin/sandbox-exec` launcher and explicit Seatbelt profiles.

Phase 13A proved the native primitive. Phase 13B, merged in PR #162 as `840b00bb6d2b74d1a88eb1fd910d06dab64118a2`, adds the first-party protocol-v2 Darwin local runtime with:

- per-session canonical workspace/home/tmp roots;
- no mount or one trusted `read_only` workspace mount;
- bounded read-only workspace staging (20,000 files / 256 MiB) instead of granting the live host workspace path;
- rejection of symbolic links, hard links, special files, traversal, and copy-time source identity/size changes;
- explicit system/session file-read roots rather than host-wide reads;
- write access only to runtime-owned home/tmp and the ephemeral workspace when no host workspace is mounted;
- zero network operations in the Seatbelt profile;
- a fixed executable search path and rejection of arbitrary model-selected host executables outside approved runtime roots;
- reconstructed minimal environment with sensitive/proxy keys, runtime-owned path/home/temp keys, and `DYLD_*` injection rejected;
- bounded wall time/stdout/stderr;
- caller-known execution IDs and explicit cancellation;
- process-group teardown for ordinary descendants.

The Darwin runtime truthfully reports `process_tree_isolation=false`. Phase 13D adversarial evidence confirms that a deliberately `setsid`-detached descendant may outlive process-group cancellation while retaining Seatbelt filesystem/network confinement. Destination allowlists and memory/CPU/PID/disk quotas also remain unadvertised.

## Persistent plugins and stdio MCP

Persistent extensions keep their existing streaming JSON-RPC lifecycle behind the shared managed-process boundary.

Configuration:

```text
OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off
```

### Linux

- `auto` uses Bubblewrap when the sandbox rootfs is configured;
- `required` fails closed when native confinement cannot be supplied;
- `off` uses the sanitized host compatibility boundary.

### Windows

- `auto` selects native AppContainer confinement when available;
- `required` fails closed if native confinement cannot be supplied;
- `off` explicitly selects the sanitized host compatibility boundary;
- each native extension receives a unique ephemeral AppContainer profile;
- non-System32 command bundles and required working-directory content are staged read-only;
- unrelated absolute host arguments fail closed;
- `.cmd` and `.bat` launch through System32 `cmd.exe` while their staged script remains inside AppContainer storage;
- AppContainer-owned home/tmp remain writable;
- Job membership and stdio handle restriction exist before untrusted code runs;
- root completion, context cancellation, and forced shutdown terminate descendants;
- transient profile cleanup failures are retried.

### macOS — Phase 13C

PR #164 merged a Darwin `platformExtensionCommandContext` backed by the same fixed native Seatbelt primitive proven in 13A/13B, as `44f410793a70444963ec1eecb989b15df159b5f1`.

When `/usr/bin/sandbox-exec` is available:

- `auto` selects native Seatbelt confinement;
- `required` selects native confinement and fails closed if the primitive is unavailable;
- `off` remains the explicit sanitized-host compatibility path;
- each extension receives a unique runtime-owned home/tmp scratch root;
- the canonical extension command directory and optional canonical working directory are read-only;
- system/runtime read roots are explicit and network operations are not granted;
- writes are limited to the per-extension home/tmp roots plus `/dev/null`;
- the child environment is reconstructed from a fixed path/home/tmp/lang base plus validated explicit configuration;
- `PATH`, `HOME`, temp/shell authority, and all `DYLD_*` overrides are rejected in native mode;
- ambient backend secrets are not inherited;
- the existing `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` override remains a transitional explicit operator choice for configured secret-bearing values; it does not alter network denial;
- streaming stdin/stdout/stderr remains available through the shared `CommandProcess` lifecycle;
- context cancellation and explicit kill terminate the ordinary process group before scratch cleanup;
- scratch cleanup retains the original temporary path even if canonicalization fails, avoiding a cleanup leak on a failed launch.

As with the Phase 13B runtime, Phase 13C does not claim authoritative teardown for an adversarial descendant that successfully detaches into an independent process group/session.

PR #164's normalized exact head passed native `macos-latest` extension lifecycle/confinement tests and applicable repository gates before merge.

## Extension environment policy

Compatibility-mode processes use a sanitized host environment rather than inheriting the backend environment wholesale.

Native Linux, Windows, and Darwin extension confinement rejects credential-sensitive explicit environment values by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a narrow transitional compatibility override, not the preferred secret-delivery architecture.

## Network policy

Owner-bound destination grants exist in the Broker, but authorization is separate from runtime enforcement.

The first-party Linux, Windows, and Darwin runtimes currently remain no-network and report `network_allowlist=false`. Native persistent extensions likewise receive no network authority. Destination-scoped egress remains open Phase 8 work.

## Resource-policy limits

Resource capability reporting is platform-specific rather than universal.

- Windows: `pid_limit=true` and `memory_limit=true`; `resources.max_processes` and aggregate `resources.memory_bytes` are enforced by the pre-start Job Object for the root process and descendants.
- Linux: memory, CPU, PID/process-count, and physical-disk quota bits remain false pending cgroup-v2 delegated-controller work.
- macOS: memory, CPU, PID/process-count, and physical-disk quota bits remain false.
- All first-party runtimes: CPU and physical-disk quota enforcement remain open.

The Broker fails closed on non-zero memory, CPU, process-count, or disk quota requests whenever the selected runtime does not advertise the matching capability. Capability bits must remain false until implementation and native validation exist for that runtime.

## Validation record

Windows Phase 12 completed through:

- PR #127 — native security primitives;
- PR #128 — first-party Windows protocol-v2 AppContainer runtime;
- PR #139 — persistent Windows stdio MCP/plugin AppContainer confinement;
- PR #149 — direct adversarial Windows assurance.

PR #149 final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed Quality Gate, Security Scan, native Windows sandbox/plugin/desktop checks, backend format/vet/tests/race, Chromium, frontend, Helm, dependency audit, both CodeQL lanes, and frontend/backend `linux/amd64` plus `linux/arm64` container builds before squash merge as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

Browser native egress assurance merged in PR #168 as `76f4f4c55a8370fb036290daa8a4054f00be1232`. Broker fail-closed resource admission merged in PR #170 as `9a2db5bf34f51502b3872145057e21d62c9d1ed1`. Windows PID-limit enforcement merged in PR #171 as `11dfab99e73fe414e45cc44b0f33d4c80789295a` after the normalized final head passed native Windows and repository-wide gates. PR #172 adds aggregate Windows Job memory enforcement; native `windows-latest` Job configuration, zero/negative compatibility, and descendant over-limit evidence are green on implementation head `a73ef3939e909992cf5dab16422e101c7dd0011b`. The documentation-inclusive final head must still pass applicable gates before merge.

macOS Phase 13A merged in PR #159. Phase 13B passed native macOS runtime assurance and repository gates and merged in PR #162 as `840b00bb6d2b74d1a88eb1fd910d06dab64118a2`. Phase 13C merged in PR #164 as `44f410793a70444963ec1eecb989b15df159b5f1`. Phase 13D adversarial assurance passed its native and repository gates and merged in PR #166 as `d52ab16f6f1cdc14bd7762ccb13d16964d665b17`, closing Phase 13 without changing the truthful detached-process limitation.

Cross-compilation alone is never considered platform-confinement evidence.
