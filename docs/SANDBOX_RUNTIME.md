# Sandbox Runtime

OmniLLM-Studio's agent/code sandbox uses an application-owned Broker and authenticated protocol-v2 execution worker. This runtime is separate from the Chromium browser sandbox and from tool policy: the existing Tool Executor decides whether a capability is allowed/Ask/denied, while the sandbox runtime enforces what an approved process can technically access.

The durable implementation program and threat model are tracked in:

- `AGENT_SANDBOX_ROADMAP_2026-08.md`
- `AGENT_SANDBOX_ARCHITECTURE.md`
- `AGENT_SANDBOX_THREAT_MODEL.md`
- `AGENT_SANDBOX_PHASE12_WINDOWS_2026-08.md`
- `AGENT_SANDBOX_PHASE12D_WINDOWS_ASSURANCE_2026-08.md`

## Protocol-v2 configuration

The backend runtime endpoint is configured with:

```text
OMNILLM_SANDBOX_URL=http://127.0.0.1:8090
OMNILLM_SANDBOX_TOKEN=<long-random-service-token>
```

`OMNILLM_SANDBOX_TOKEN` is mandatory. It is an application/service credential and must not be placed in model prompts, tool arguments, frontend state, logs, or sandbox child environments.

During the migration from the historical one-shot code-sandbox adapter, `OMNILLM_CODE_SANDBOX_URL` remains accepted as a URL compatibility alias. It no longer means the old unauthenticated `/v1/execute` contract: configured runtimes must implement protocol v2 and complete the authenticated capabilities handshake.

The backend refuses redirects from a protocol-v2 HTTP runtime. Plain HTTP is accepted only for loopback endpoints; non-loopback runtime endpoints require HTTPS.

## First-party worker

The first-party worker executable is:

```bash
cd backend
go run ./cmd/sandboxd
```

The same `sandboxd` executable selects the supported local runtime for its host platform. Packaging `sandboxd` into all desktop/server distributions remains separate deployment work; the source/runtime implementations described below are already present.

### Linux configuration

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

The Linux runtime uses Bubblewrap. `OMNILLM_SANDBOX_ROOTFS` must point to an operator-prepared runtime filesystem containing only the interpreters/build tools intentionally available to sandbox workloads. The runtime mounts that root filesystem read-only and creates a separate ephemeral writable `/workspace` scratch directory for sessions without a project workspace. It does **not** bind-mount the host root into the sandbox.

#### Enforced Linux controls

The runtime currently advertises these controls as enforced:

- OS/process namespace isolation through Bubblewrap;
- filesystem isolation using a configured read-only root filesystem plus explicit trusted workspace/scratch mounts;
- network namespace isolation with no network access in the current first-party runtime;
- process-tree/session confinement and teardown;
- runtime session TTL cleanup;
- bounded wall time;
- bounded stdout and stderr.

It intentionally does **not** advertise destination allowlist egress, memory, CPU, PID-count, or physical disk quota enforcement. Runtime capability negotiation allows the Broker to fail closed when a workload/deployment requires a control the selected worker cannot enforce.

#### Linux workspace mounts

Models never provide physical host paths to the runtime. The backend resolves an owner-scoped opaque workspace ID to a trusted runtime-only mount descriptor after validating the stored grant.

The Linux worker supports at most one project workspace per sandbox execution:

- `read_only` is mounted read-only at `/workspace`;
- `read_write_no_delete` is narrowed to read-only for arbitrary shell execution because a POSIX bind mount cannot reliably enforce write-without-delete semantics;
- `read_write` is supported at the runtime layer, but `.git` is remounted read-only when present; a symlinked `.git` causes the whole workspace mount to narrow to read-only.

The current `terminal_exec` tool deliberately requests only a read-only project mount. Source mutations remain routed through the state-bound, journaled workspace mutation tools.

### Windows first-party runtime

Windows native confinement Phase 12 is complete. On supported Windows 10+ hosts, `NewLocalRuntime` uses stable AppContainer/process-creation mechanisms rather than unrestricted host execution.

The Windows runtime:

- creates an ephemeral AppContainer profile/package SID per runtime session;
- launches children with `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` and **zero AppContainer network capabilities**;
- attaches a kill-on-close Job Object through `PROC_THREAD_ATTRIBUTE_JOB_LIST` at process creation, eliminating a pre-confinement execution window;
- restricts inherited handles to explicit stdio handles through `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`;
- tears down Job descendants on root completion, execution-context cancellation, timeout, and session destruction;
- uses a minimal runtime-owned environment and applies the sandbox credential-sensitive environment policy to explicit values;
- enforces bounded wall time and stdout/stderr;
- does not advertise CPU, memory, process-count, or physical disk quotas.

#### Windows workspace policy

Windows arbitrary-process execution does not widen ACLs on the user's source workspace.

- no project mount creates an ephemeral writable workspace inside the AppContainer profile;
- one `read_only` project mount may be staged into AppContainer-owned storage;
- `read_write` and `read_write_no_delete` arbitrary-process mounts fail closed;
- staged input is bounded to 20,000 entries / 256 MiB;
- staging rejects reparse points/junctions, multiply-linked files, and special files;
- every opened source handle is resolved with `GetFinalPathNameByHandle` and must still be beneath the canonical source root before bytes are copied;
- the staged workspace receives a protected DACL granting the AppContainer only read/traverse/execute authority;
- runtime-owned `home` and `tmp` remain writable inside the AppContainer profile.

The staging limit is an admission/operational bound, not a physical disk quota.

#### Windows native evidence

The merged native suite includes normal and adversarial evidence for:

- AppContainer token state and distinct per-session/per-extension identities;
- denial of unrelated host-file reads/writes;
- cross-profile authority isolation;
- read-only staged workspace/extension bundles;
- writable sandbox-owned home/tmp;
- absence of ambient backend secrets;
- default-deny loopback/network behavior;
- descendant Job teardown after root completion and context cancellation;
- active session destruction/cleanup behavior;
- hard-link and junction/reparse-point staging rejection;
- fail-closed unrelated absolute extension arguments;
- extension policy-mode selection (`auto`, `required`, `off`).

Phase 12D final head `8f4ee1b7de5d3ea6203c44089dadfae4fd6d30cb` passed the complete Quality, Security, native Windows, race, Chromium Playwright, Helm, and multi-architecture container gate set before merge in PR #149.

### Windows code-language limitation

The Windows arbitrary-process runtime deliberately fails closed for Python and JavaScript code shortcuts until an AppContainer-readable interpreter/package layout is implemented and natively validated. This does not affect explicit executable/argv execution when the executable is accessible under the runtime's confinement policy. There is no hidden fallback to unrestricted host Python or Node.

## Desktop workspace settings and grants

Phase 11 adds a safe Settings surface for inspecting the active sandbox and managing filesystem grants without exposing configured host roots to model tools or normal API responses.

The authenticated settings API exposes:

```text
GET    /v1/sandbox/status
GET    /v1/sandbox/workspaces
POST   /v1/sandbox/workspaces
DELETE /v1/sandbox/workspaces/{workspace_id}
GET    /v1/sandbox/workspaces/{workspace_id}/changes
```

Response contracts are intentionally narrow:

- runtime status reports capability booleans and extension-sandbox mode, not runtime URL/token/rootfs configuration;
- workspace lists return only opaque ID, access mode, and timestamps;
- recent change history returns relative path, operation, before/after existence and hashes, revertability, and timestamp;
- internal user, conversation, agent-run, task, sandbox, execution, snapshot-content, and physical-root fields are not serialized through these Settings endpoints.

Physical host paths are accepted only by the grant-creation endpoint. Creation requires all of the following:

1. administrator authorization;
2. `OMNILLM_SANDBOX_ALLOW_PATH_GRANTS=true`;
3. the actual HTTP socket peer is loopback.

`RemoteAddr` is used for the loopback check; forwarded client-IP headers are not trusted for this decision.

Server/web deployments therefore do not gain a generic remote host-filesystem selector merely because the Settings API exists. An operator must explicitly enable path grants, and creation remains loopback-only.

The Wails desktop build uses its native folder picker to select a directory. Desktop startup enables the path-grant flow for its protected per-launch loopback capability URL. The selected physical path is kept only in ephemeral frontend component state and cleared after the backend converts it into an opaque owner-scoped grant.

Grant deletion revokes authorization only; it never removes files from disk. The UI can show whether a journaled change is revertable, but Phase 11 deliberately does not add an HTTP revert shortcut. Reverts remain behind the existing governed workspace tool and approval boundary.

## Network grants

The backend supports owner-bound destination grants, but authorization is distinct from enforcement. A sandbox that requests `NetworkAllowlist` must run on a runtime that advertises `network_allowlist=true`.

The first-party Linux Bubblewrap runtime and Windows AppContainer runtime currently advertise `network_allowlist=false`, so both remain no-network even when a user has an approved destination grant. This is intentional fail-closed behavior until an enforceable first-party destination-scoped egress mechanism lands.

Windows native persistent extensions also use AppContainer with zero network capabilities; they do not currently implement destination allowlisting.

## Code execution

`code_execute` remains a high-risk, side-effecting tool under the normal Tool Executor policy. Its public tool name remains stable, but the execution contract is Broker-oriented:

- a new session receives an application-issued `sbx_...` ID;
- session ownership is bound to the authenticated user/workspace/conversation/message/agent-run invocation scope;
- a returned session ID may be reused only from that same ownership scope;
- caller-created arbitrary session IDs are rejected;
- network is disabled by default;
- the runtime must advertise OS, filesystem, network, and process-tree isolation for the tool to create a session.

On Linux, supported code modes remain Python, JavaScript, and shell subject to binaries in the configured sandbox root filesystem. Python code execution uses isolated interpreter flags (`-I -S`). Windows code-language shortcuts remain fail-closed as described above until the interpreter layout is natively validated.

## Restricted Python analysis

`python_analysis` retains its restricted AST/builtin policy as defense in depth but no longer relies on host Python execution when the sandbox migration is enabled. It requires:

```text
OMNILLM_CODE_EXEC_ENABLED=true
```

and an available protocol-v2 Broker. Without both, the tool is disabled. Analysis sessions are ephemeral, network-disabled, bounded, and destroyed after execution. A Windows runtime that cannot provide the required confined Python interpreter fails closed rather than using host Python.

## Worker API

The authenticated worker exposes:

```text
GET    /v2/capabilities
POST   /v2/sandboxes
POST   /v2/sandboxes/{runtime_id}/exec
POST   /v2/sandboxes/{runtime_id}/cancel
GET    /v2/sandboxes/{runtime_id}/status
DELETE /v2/sandboxes/{runtime_id}
```

The model does not call these endpoints directly. The backend Broker owns public `sbx_...` session IDs and maps them to runtime-private IDs.

### Explicit execution cancellation caveat — issue #151

Context cancellation, timeouts, session `Destroy`, and runtime process-tree teardown are implemented and tested. A separate protocol lifecycle defect remains open as issue #151: synchronous `Exec` currently returns the runtime-generated `execution_id` only after the execution completes, while the separate `/cancel` operation requires that ID. A caller therefore cannot use the explicit execution-ID cancellation endpoint against an active synchronous `Exec` unless the protocol is changed to make the execution reference known at start time.

This is not a Windows confinement regression—the Windows Job teardown paths are natively proven—but operators/developers should not treat the explicit `Cancel(executionID)` endpoint as a complete active-execution control until #151 is repaired.

## Artifact trust boundary

Protocol v2 describes sandbox outputs with application-owned artifact IDs, name, MIME type, size, and SHA-256 metadata. Arbitrary worker-supplied artifact URLs are not the trust boundary. Later artifact-promotion work must validate ownership, size, content/path safety, and hashes before registering sandbox outputs with normal OmniLLM storage.

## Local plugins and stdio MCP

Local plugins and stdio MCP servers are persistent streaming subprocesses, so they keep their existing JSON-RPC stdin/stdout lifecycle while process construction uses the shared extension-confinement policy.

Configuration is:

```text
OMNILLM_EXTENSION_SANDBOX_MODE=auto|required|off
```

The default is `auto`:

- on Linux, `auto` uses Bubblewrap when `OMNILLM_SANDBOX_ROOTFS` is configured; otherwise it preserves the sanitized host-process boundary;
- on Windows, `auto` uses the native AppContainer/Job backend when the required Windows APIs are available;
- on macOS, `auto` still preserves the sanitized boundary until Phase 13 provides a native backend;
- `required` fails closed if native extension confinement is unavailable/not configured;
- `off` explicitly selects the sanitized host compatibility boundary.

The sanitized compatibility boundary strips ambient backend secrets. Explicitly configured MCP/plugin environment values continue to work in compatibility mode so existing configured extension credentials are not silently broken.

When native Linux extension confinement is active, the child receives a read-only rootfs, private namespaces/session, private tmp/home, no network, read-only extension/working-directory mounts, and a cleared environment.

When native Windows extension confinement is active, the child receives:

- a unique ephemeral AppContainer profile;
- zero network capabilities;
- creation-time Job Object membership;
- explicit inherited stdio handles only;
- read-only staged command bundle/working-directory roots rather than host ACL widening;
- runtime-owned writable home/tmp;
- a minimal non-ambient Windows environment;
- argument remapping only for absolute paths beneath staged roots; unrelated absolute host arguments fail closed;
- Job teardown on root completion, context cancellation, and forced plugin shutdown;
- retryable AppContainer profile cleanup.

Credential-sensitive explicit environment entries are rejected by default under native confinement. A deployment that knowingly requires a legacy extension credential may opt in with:

```text
OMNILLM_EXTENSION_ALLOW_SECRET_ENV=true
```

That override should be treated as transitional. Service-specific credential broker consumers are preferred because arbitrary extension environments should not become the normal secret-delivery path.

## Platform status

- **Linux:** first-party Bubblewrap code/terminal runtime is available; persistent extension confinement is available when a sandbox rootfs is configured. Resource quotas and destination-enforced egress remain incomplete.
- **Windows:** native AppContainer/Job code/terminal runtime and persistent extension confinement are implemented and natively adversarial-tested. The runtime remains no-network, does not advertise memory/CPU/PID/disk quotas, and fails closed for Python/JavaScript shortcuts until a confined interpreter layout is validated. Explicit execution-ID cancellation addressability remains issue #151.
- **macOS:** native confinement Phase 13 is not started; `required` fails closed while `auto` retains the sanitized extension boundary.
- **Server/Kubernetes:** dedicated worker/deployment phase remains pending; arbitrary tenant execution must not be added to the primary API container as a shortcut.

## Validation

Sandbox changes use the repository's standard backend/frontend/Playwright gates as applicable, plus Security Scan and container/deployment validation for security-sensitive runtime work. Windows confinement additionally runs native `windows-latest` sandbox/adversarial, plugin-lifecycle, and desktop compatibility jobs. Platform confinement is never considered complete solely because code cross-compiles or capability flags are set; native behavior evidence is required for each supported OS.
