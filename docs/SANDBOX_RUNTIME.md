# Sandbox Runtime

OmniLLM-Studio's agent/code sandbox uses an application-owned Broker and authenticated protocol-v2 execution worker. This runtime is separate from the Chromium browser sandbox and from tool policy: the existing Tool Executor decides whether a capability is allowed/Ask/denied, while the sandbox runtime enforces what an approved process can technically access.

The durable implementation program and threat model are tracked in:

- `AGENT_SANDBOX_ROADMAP_2026-08.md`
- `AGENT_SANDBOX_ARCHITECTURE.md`
- `AGENT_SANDBOX_THREAT_MODEL.md`

## Protocol-v2 configuration

The backend runtime endpoint is configured with:

```text
OMNILLM_SANDBOX_URL=http://127.0.0.1:8090
OMNILLM_SANDBOX_TOKEN=<long-random-service-token>
```

`OMNILLM_SANDBOX_TOKEN` is mandatory. It is an application/service credential and must not be placed in model prompts, tool arguments, frontend state, logs, or sandbox child environments.

During the migration from the historical one-shot code-sandbox adapter, `OMNILLM_CODE_SANDBOX_URL` remains accepted as a URL compatibility alias. It no longer means the old unauthenticated `/v1/execute` contract: configured runtimes must implement protocol v2 and complete the authenticated capabilities handshake.

The backend refuses redirects from a protocol-v2 HTTP runtime. Plain HTTP is accepted only for loopback endpoints; non-loopback runtime endpoints require HTTPS.

## First-party Linux worker

The first-party worker executable is:

```bash
cd backend
go run ./cmd/sandboxd
```

Required configuration:

```text
OMNILLM_SANDBOX_TOKEN=<same-token-used-by-backend>
OMNILLM_SANDBOX_ROOTFS=/absolute/path/to/sandbox-rootfs
```

Optional configuration:

```text
OMNILLM_SANDBOX_BIND=127.0.0.1:8090
OMNILLM_SANDBOX_SCRATCH_ROOT=/absolute/path/to/scratch-parent
OMNILLM_SANDBOX_BWRAP=/usr/bin/bwrap
```

The current Linux runtime uses Bubblewrap. `OMNILLM_SANDBOX_ROOTFS` must point to an operator-prepared runtime filesystem containing only the interpreters/build tools intentionally available to sandbox workloads. The runtime mounts that root filesystem read-only and creates a separate ephemeral writable `/workspace` scratch directory for each sandbox session. It does **not** bind-mount the host root into the sandbox.

### Current enforced Linux controls

The runtime currently advertises these controls as enforced:

- OS/process namespace isolation through Bubblewrap;
- filesystem isolation using a configured read-only root filesystem plus isolated scratch;
- network namespace isolation with no network access in the current revision;
- process-tree/session confinement and teardown;
- bounded wall time;
- bounded stdout and stderr.

It intentionally does **not** advertise memory, CPU, PID-count, or physical disk quota enforcement yet. Runtime capability negotiation allows the Broker to fail closed when a workload/deployment requires a control the selected worker cannot enforce.

Workspace mounts and network-enabled sessions are currently rejected by the first-party Linux runtime until the corresponding workspace and network-broker phases are implemented. This is intentional fail-closed behavior.

## Code execution

`code_execute` remains a high-risk, side-effecting tool under the normal Tool Executor policy. Its public tool name remains stable, but the execution contract is now Broker-oriented:

- a new session receives an application-issued `sbx_...` ID;
- session ownership is bound to the authenticated user/workspace/conversation/message/agent-run invocation scope;
- a returned session ID may be reused only from that same ownership scope;
- caller-created arbitrary session IDs are rejected;
- network is disabled by default;
- the runtime must advertise OS, filesystem, network, and process-tree isolation for the tool to create a session.

Supported code modes remain Python, JavaScript, and shell, subject to the binaries provided by the configured sandbox root filesystem.

## Restricted Python analysis

`python_analysis` retains its restricted AST/builtin policy as defense in depth but no longer relies on host Python execution when the sandbox migration is enabled. It requires:

```text
OMNILLM_CODE_EXEC_ENABLED=true
```

and an available protocol-v2 Broker. Without both, the tool is disabled. Analysis sessions are ephemeral, network-disabled, bounded, and destroyed after execution.

## Worker API

The authenticated worker currently exposes:

```text
GET    /v2/capabilities
POST   /v2/sandboxes
POST   /v2/sandboxes/{runtime_id}/exec
POST   /v2/sandboxes/{runtime_id}/cancel
GET    /v2/sandboxes/{runtime_id}/status
DELETE /v2/sandboxes/{runtime_id}
```

The model does not call these endpoints directly. The backend Broker owns public `sbx_...` session IDs and maps them to runtime-private IDs.

## Artifact trust boundary

Protocol v2 describes sandbox outputs with application-owned artifact IDs, name, MIME type, size, and SHA-256 metadata. Arbitrary worker-supplied artifact URLs are not the trust boundary. Later artifact-promotion work must validate ownership, size, content/path safety, and hashes before registering sandbox outputs with normal OmniLLM storage.

## Local plugins and stdio MCP

The sandbox program first removes ambient backend environment inheritance from these subprocesses through a shared process-construction seam. Full OS-level plugin/MCP migration is tracked separately in the roadmap. Until that phase is complete, sanitized host execution must not be described as equivalent to the OS sandbox.

## Platform status

- **Linux:** first-party Bubblewrap runtime implementation in progress.
- **Windows:** native confinement phase pending; unsupported first-party local runtime fails closed rather than silently using unrestricted host execution.
- **macOS:** native confinement phase pending; unsupported first-party local runtime fails closed rather than silently using unrestricted host execution.
- **Server/Kubernetes:** dedicated worker/deployment phase pending; arbitrary tenant execution must not be added to the primary API container as a shortcut.

## Validation

Sandbox changes use the repository's standard backend/frontend/Playwright gates as applicable, plus Security Scan and container/deployment validation for security-sensitive runtime work. Platform confinement is not considered complete solely because code cross-compiles; native isolation tests are required for each supported OS.
