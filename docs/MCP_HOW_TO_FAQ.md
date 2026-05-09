# How to MCP in OmniLLM-Studio

This FAQ explains how OmniLLM-Studio's Model Context Protocol support works, what is currently implemented, how to configure a server, and how the real-world MCP tests were run.

## Did real MCP testing confirm this works?

Yes. The opt-in integration tests confirmed OmniLLM-Studio can talk to a real MCP server over stdio.

The test target was the official filesystem MCP server:

```text
@modelcontextprotocol/server-filesystem@2025.8.21
```

The tests started the server with `npx`, completed MCP initialization, discovered real tools through `tools/list`, executed `read_file` and `write_file` through `tools/call`, and verified the written file on disk.

The manager/executor test also confirmed the application path works: persisted MCP server config, dynamic tool registration, default `ask` policy, policy change to `allow`, and execution through the existing `tools.Executor`.

## What MCP support exists right now?

Current support is MCP client support for stdio servers that expose tools.

Implemented:

- Stdio MCP subprocess launch.
- Streamable HTTP MCP transport (2025-06-18 spec).
- MCP JSON-RPC initialization.
- `tools/list` discovery.
- `tools/call` execution.
- Dynamic registration into OmniLLM-Studio's existing tool registry.
- Tool execution through the existing tool executor.
- Admin REST API under `/v1/mcp`.
- Encrypted environment variable storage for server config.
- Audit logging for lifecycle events, tool calls, and errors.
- Settings UI for MCP server management.
- Admin activity view for recent MCP audit events.
- Agent Mode approval for tools with `ask` policy.
- TypeScript API/types for UI and integration work.
- Real-world integration tests against the filesystem MCP server.

- Native tool-calling loop in normal chat (supports autonomous execution of tools set to `allow`).

Deferred:

- Interactive approval for manual `POST /v1/tools/execute` calls.
- MCP resources, prompts, sampling, and elicitation.
- MCP server mode, where OmniLLM-Studio itself exposes tools to other MCP clients.

## Where is the MCP UI?

MCP server management is available in Settings -> MCP.

The tab supports:

- Adding and editing stdio MCP servers.
- Loading a pinned filesystem-server template.
- Testing server connectivity.
- Starting, stopping, restarting, and refreshing servers.
- Viewing runtime status and startup errors.
- Viewing redacted environment variable keys.
- Changing discovered MCP tool policy between `allow`, `ask`, and `deny`.
- Viewing recent MCP activity from the audit log.

The REST examples below are still useful for automation, debugging, and headless setups.

## What transport is supported?

OmniLLM-Studio supports both **stdio** and **Streamable HTTP** transports (MCP 2025-06-18).

For **stdio**, OmniLLM-Studio launches a local MCP server as a subprocess and communicates over stdin/stdout using newline-delimited JSON-RPC messages.

Examples of stdio MCP commands:

```text
npx -y @modelcontextprotocol/server-filesystem@2025.8.21 C:\Users\you\Documents
```

```text
uvx some-mcp-server
```

For **HTTP**, OmniLLM-Studio communicates with remote or out-of-process MCP servers using stateless HTTP POST requests, with Server-Sent Events (SSE) for streaming responses. The Streamable HTTP transport requires a URL to connect to. You can also supply custom headers (e.g. `Authorization: Bearer ...`) that will be sent with every request.

## What happens when OmniLLM-Studio starts?

On backend startup:

1. Database migrations create MCP tables if needed.
2. Enabled MCP server configs are loaded.
3. Each enabled stdio server is started.
4. The server is initialized using MCP JSON-RPC.
5. Tools are discovered through `tools/list`.
6. Discovered tools are registered into the existing tool registry.
7. Newly discovered MCP tools are seeded in `tool_permissions` with policy `ask`.

On backend shutdown, running MCP subprocesses are stopped and their dynamic tools are removed from the registry.

## What database tables are used?

MCP adds these tables:

- `mcp_servers`: persisted MCP server configuration.
- `mcp_audit_log`: lifecycle, tool execution, and error records.

MCP tool execution policy reuses the existing `tool_permissions` table. There is not a separate MCP permission table.

## How do I view MCP activity?

Use Settings -> MCP -> MCP Activity, or call the audit API:

```text
GET /v1/mcp/audit?limit=50
```

Filter by server:

```text
GET /v1/mcp/audit?server_id=<server-id>&limit=50
```

The audit log records lifecycle events, config changes, policy changes, tool calls, durations, normalized outputs, and errors. Deleting a server removes its server-scoped audit rows because the audit table references `mcp_servers`.

## Are MCP server secrets encrypted?

Yes. Values sent in the `env` object are encrypted before being stored in `mcp_servers.env_json`.

API responses do not return plaintext env values. They return `env_keys`, which lets the UI show which environment variables are configured without revealing values.

## How are MCP tools named inside OmniLLM-Studio?

Internal tool names are provider-safe and use this format:

```text
mcp_<server_slug>_<tool_slug>
```

Examples:

```text
mcp_filesystem_read_file
mcp_filesystem_write_file
mcp_github_create_issue
```

The naming intentionally avoids dots so the names remain compatible with stricter provider tool-name rules later.

If names collide, the manager appends a suffix like `_1`.

## Why do MCP tools default to `ask`?

MCP servers can expose powerful tools, including file writes, shell-like operations, API calls, and database changes. New MCP tools are therefore seeded with `ask`.

Agent Mode now supports interactive `ask`: the run pauses, emits an approval-required event, and resumes only when the user approves the step. Rejecting the approval cancels the run.

Manual execution through `POST /v1/tools/execute` still has no interactive caller, so it returns an approval-required error for `ask`. Change a tool to `allow` only when you trust the server and the specific tool.

## How do I configure the filesystem MCP server?

Start the backend, then create an MCP server with the REST API.

In solo mode, no auth token is required. In multi-user mode, use an admin bearer token:

```powershell
$headers = @{
  "Content-Type" = "application/json"
  "Authorization" = "Bearer <admin-token>"
}
```

Create a disabled filesystem server:

```powershell
$body = @{
  name = "filesystem"
  transport = "stdio"
  command = "npx.cmd"
  args = @(
    "-y",
    "@modelcontextprotocol/server-filesystem@2025.8.21",
    "C:\Users\you\Documents"
  )
  enabled = $false
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/mcp/servers" `
  -Headers $headers `
  -Body $body
```

On macOS/Linux, the command is usually `npx` instead of `npx.cmd`.

## How do I configure an HTTP MCP server?

Create an HTTP MCP server by specifying the `http` transport, a URL, and optional custom headers for authentication.

```powershell
$body = @{
  name = "remote-search"
  transport = "http"
  url = "https://mcp.example.com/api/v1/mcp"
  headers = @{
    "Authorization" = "Bearer abcdef123456"
    "X-Custom-Header" = "true"
  }
  enabled = $true
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/mcp/servers" `
  -Headers $headers `
  -Body $body
```

Like environment variables, custom headers are encrypted at rest and are not returned in plaintext by the API.

## How do I test the connection?

Use the server ID returned from creation:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/test" `
  -Headers $headers
```

This starts a temporary MCP client, initializes the server, runs `tools/list`, returns discovered tools, and then stops the temporary process. It does not register tools into the app-wide registry.

## How do I start a server and register its tools?

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/start" `
  -Headers $headers
```

After the server starts, discovered tools appear in:

```text
GET /v1/tools
```

They also appear in:

```text
GET /v1/mcp/servers/<server-id>/tools
```

## How do I allow one MCP tool to run?

List server tools:

```powershell
Invoke-RestMethod `
  -Method Get `
  -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/tools" `
  -Headers $headers
```

Pick the `internal_name`, then set it to `allow`:

```powershell
$body = @{ policy = "allow" } | ConvertTo-Json

Invoke-RestMethod `
  -Method Patch `
  -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/tools/mcp_filesystem_read_file" `
  -Headers $headers `
  -Body $body
```

Allowed values are:

- `allow`
- `deny`
- `ask`

## How do I manually execute an MCP tool?

Use the existing tool execution API with the MCP tool's internal name.

Example for `read_file`:

```powershell
$body = @{
  name = "mcp_filesystem_read_file"
  arguments = @{
    path = "C:\Users\you\Documents\example.txt"
  }
} | ConvertTo-Json -Depth 5

Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8080/v1/tools/execute" `
  -Headers $headers `
  -Body $body
```

If the policy is still `ask`, manual execution returns an approval-required error because the manual API has no interactive approval channel.

## How do Agent Mode and normal chat use MCP tools?

Agent Mode can use connected MCP tools because it builds its tool list from the shared registry. If a selected tool has `ask` policy, the agent run pauses and shows approval controls through the existing Agent Mode approval flow.

Normal chat does not yet have a general-purpose tool-calling loop. MCP tools are not automatically invoked during ordinary chat streaming. This is intentional because the current chat path does not pass native tool definitions to LLM providers.

## Can MCP tools be used with plugins?

MCP and the existing Plugin SDK are separate extension paths.

MCP tools are registered in the same runtime tool registry as native tools, but MCP server subprocesses are managed by the MCP manager, not the plugin loader.

## What content types can MCP tool results return?

The adapter normalizes MCP content into the existing `ToolResult` shape.

Current mappings:

- `text`: returned as text content.
- `image`: summarized as `[Image: <mime_type>]`.
- `audio`: summarized as `[Audio: <mime_type>]`.
- `resource` with text: returned as text.
- `resource` with blob/mime type: summarized as `[Resource: <mime_type>]`.
- `resource_link`: summarized as `[Resource: <name> <uri>]`.
- Multiple content blocks: joined with newlines.

Normalized text content is truncated at 100KB.

## How do I stop, restart, or refresh a server?

Stop:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/stop" -Headers $headers
```

Restart:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/restart" -Headers $headers
```

Refresh tools:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/refresh" -Headers $headers
```

Refresh currently restarts the server and re-runs discovery.

## How do I delete a server?

```powershell
Invoke-RestMethod `
  -Method Delete `
  -Uri "http://localhost:8080/v1/mcp/servers/<server-id>" `
  -Headers $headers
```

Deleting stops the running server first, then removes the server config.

## How do I run the real-world MCP tests?

The real-world tests are opt-in because they require Node/npm, `npx`, and access to the npm package or npm cache.

Default behavior:

```powershell
go test ./internal/mcpclient -run RealWorld -v -count=1
```

Expected result: tests are skipped unless explicitly enabled.

Run against the real filesystem MCP server:

```powershell
$env:OMNILLM_RUN_REAL_MCP_TESTS='1'
go test ./internal/mcpclient -run RealWorld -v -count=1
```

If `npx` is not on `PATH`:

```powershell
$env:OMNILLM_REAL_MCP_NPX='C:\Program Files\nodejs\npx.cmd'
$env:OMNILLM_RUN_REAL_MCP_TESTS='1'
go test ./internal/mcpclient -run RealWorld -v -count=1
```

## What did the real-world test run prove?

The real run verified:

- OmniLLM-Studio can start a real MCP server process.
- Stdio JSON-RPC communication works.
- MCP initialization works.
- `tools/list` works against a real server.
- `tools/call` works against a real server.
- Filesystem `read_file` returns expected fixture content.
- Filesystem `write_file` writes a real file.
- The manager registers real MCP tools dynamically.
- New real MCP tools default to `ask`.
- The executor blocks `ask` tools when no approval handler is attached.
- Switching policy to `allow` lets execution proceed through the normal executor path.

The filesystem server logged that the client does not support MCP Roots and used allowed directories from command-line args. That is acceptable for this MVP.

## What should I do if `npx` cannot be found?

Use the absolute path to `npx`.

Common Windows path:

```text
C:\Program Files\nodejs\npx.cmd
```

Common macOS Homebrew path:

```text
/opt/homebrew/bin/npx
```

Desktop/Wails apps may not inherit the terminal `PATH`, so absolute paths are often more reliable.

## What should I do if a server starts but no tools appear?

Check these in order:

1. Confirm the server status from `GET /v1/mcp/servers/<server-id>`.
2. Run `POST /v1/mcp/servers/<server-id>/test`.
3. Check the backend logs for MCP stderr output.
4. Confirm the server command and args work in a terminal.
5. Confirm the server actually exposes tools, not only resources or prompts.
6. Confirm the tool is visible in `GET /v1/tools`.

## What should I do if a tool says it requires approval?

That means the tool policy is `ask`.

In Agent Mode, approve or reject the awaiting step in the Agent Mode panel.

For manual `POST /v1/tools/execute` calls, switch the tool to `allow` if you trust the server and the specific tool:

```powershell
$body = @{ policy = "allow" } | ConvertTo-Json
Invoke-RestMethod -Method Patch -Uri "http://localhost:8080/v1/mcp/servers/<server-id>/tools/<internal-tool-name>" -Headers $headers -Body $body
```

Use `deny` for tools you do not want available.

## What are the security expectations?

Treat MCP servers like executable extensions.

Important points:

- Stdio MCP servers run as local subprocesses with the same OS user permissions as the backend.
- Admins should only configure MCP servers they trust.
- MCP tools default to `ask` to avoid silent execution.
- Env values are encrypted at rest and not returned in plaintext.
- Filesystem MCP servers should be scoped to the smallest practical allowed directories.
- `npx -y` skips npm's package prompt, so pin package versions when possible.

## Does this work in Docker or Kubernetes?

The current stdio implementation can work in containers only if the runtime and MCP server command exist inside the container.

For example, an `npx`-based MCP server needs Node/npm available in the backend container. Filesystem access is limited to mounted container paths.

Remote Streamable HTTP MCP support is deferred and would likely be the better fit for many Kubernetes deployments.

## Which files implement MCP?

Backend:

- `backend/internal/mcpclient/client.go`
- `backend/internal/mcpclient/manager.go`
- `backend/internal/mcpclient/tool_adapter.go`
- `backend/internal/mcpclient/names.go`
- `backend/internal/api/mcp_handler.go`
- `backend/internal/repository/mcp.go`
- `backend/internal/db/db.go`
- `backend/internal/tools/executor.go`
- `backend/internal/tools/registry.go`
- `backend/internal/agent/runner.go`

Frontend:

- `frontend/src/api.ts`
- `frontend/src/types.ts`
- `frontend/src/components/SettingsPanel.tsx`

Tests:

- `backend/internal/mcpclient/realworld_test.go`
- `backend/internal/mcpclient/names_test.go`
- `backend/internal/mcpclient/tool_adapter_test.go`
- `backend/internal/tools/executor_test.go`
- `backend/internal/agent/runner_test.go`
- `backend/internal/repository/repository_test.go`

## Where can I read the MCP spec?

- [MCP transports](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
- [MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
- [Filesystem MCP server package](https://www.npmjs.com/package/%40modelcontextprotocol/server-filesystem)
