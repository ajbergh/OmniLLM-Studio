# Sandbox Runtime — Current Platform Status (August 2026)

This document records the current first-party sandbox and persistent-extension behavior after Windows Phase 12. It supplements the older `SANDBOX_RUNTIME.md` historical snapshot.

## Shared protocol-v2 boundary

The backend Broker owns public sandbox sessions and talks to an authenticated protocol-v2 worker. The model does not call worker endpoints directly.

Runtime configuration uses:

```text
OMNILLM_SANDBOX_URL=http://127.0.0.1:8090
OMNILLM_SANDBOX_TOKEN=<long-random-service-token>
```

The service token is backend/runtime state, not model-facing data. Plain HTTP is limited to loopback endpoints; non-loopback runtime URLs require HTTPS and redirects are rejected.

The first-party worker is `backend/cmd/sandboxd`.

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

The Linux first-party runtime does not advertise destination allowlisting, memory quota, CPU quota, PID quota, or physical disk quota enforcement.

## Windows first-party runtime

Windows 10+ uses native AppContainer plus Job Objects.

Current enforced controls:

- unique ephemeral AppContainer profile/package SID per runtime session;
- AppContainer security capabilities applied at process creation;
- zero AppContainer network capabilities;
- Job Object membership applied at process creation;
- explicit inherited stdio handle list;
- process-tree teardown on root completion, cancellation, timeout, or session destruction;
- runtime-owned minimal environment rather than ambient backend inheritance;
- bounded wall time, stdout, and stderr;
- retryable AppContainer profile cleanup.

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

### macOS

Native extension confinement is not implemented yet. `required` fails closed; `auto` retains compatibility behavior until Phase 13.

## Extension environment policy

Compatibility-mode processes use a sanitized host environment rather than inheriting the backend environment wholesale.

Native Linux and Windows extension confinement rejects credential-sensitive explicit environment values by default. `OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true` is a narrow transitional compatibility override, not the preferred secret-delivery architecture.

## Network policy

Owner-bound destination grants exist in the Broker, but authorization is separate from runtime enforcement.

The first-party Linux and Windows runtimes currently remain no-network and report `network_allowlist=false`. Destination-scoped egress remains open Phase 8 work.

## Resource-policy limits

The first-party runtime does not currently claim enforcement of:

- memory quota;
- CPU quota;
- PID/process-count quota;
- physical disk quota.

Those capability bits must remain false until implementation and native validation exist.

## Explicit cancellation caveat

Issue #151 tracks a protocol-v2 correctness defect: synchronous `Exec` currently returns its internally generated execution ID only after completion, so an external caller cannot yet address `Cancel(runtime_id, execution_id)` while the execution is running.

Context cancellation and session `Destroy` do terminate active process trees and are natively tested. Issue #151 is an API-contract defect, not a Windows OS-confinement gap.

## Validation record

Windows Phase 12 completed through:

- PR #127 — native security primitives;
- PR #128 — first-party Windows protocol-v2 AppContainer runtime;
- PR #139 — persistent Windows stdio MCP/plugin AppContainer confinement;
- PR #149 — direct adversarial Windows assurance.

PR #149 final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed Quality Gate, Security Scan, native Windows sandbox/plugin/desktop checks, backend format/vet/tests/race, Chromium, frontend, Helm, dependency audit, both CodeQL lanes, and frontend/backend `linux/amd64` plus `linux/arm64` container builds before squash merge as `65bf1cd807b9cd94a2e7b62e653c9057366c6e8b`.

Cross-compilation alone is never considered platform-confinement evidence.
