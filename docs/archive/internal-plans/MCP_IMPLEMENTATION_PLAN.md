> **Archived — superseded.** MCP tools, HTTP transport, and OAuth are implemented beyond this original plan. Use [MCP_HOW_TO_FAQ.md](../../MCP_HOW_TO_FAQ.md) and [MCP_OAUTH_2026-08.md](../../MCP_OAUTH_2026-08.md).

# MCP Implementation Plan for OmniLLM-Studio

## Section 1: Executive Summary

**Should this project add MCP support?** Yes, absolutely. MCP support is both feasible and highly valuable for OmniLLM-Studio. The existing architecture — tool registry, executor, permission system, agent mode, plugin SDK, SSE streaming, and settings UI — provides a strong foundation that maps cleanly to MCP concepts. The project already has many adjacent capabilities (web search, sports lookup, document generation, RAG, URL fetch) that MCP can extend with thousands of community-built servers.

**What should be built first?** MCP Client support with stdio transport, focusing on tool discovery and execution. This provides the highest immediate value with the least architectural disruption. MCP tools should be wrapped as `tools.Tool` interface implementations and registered in the existing `tools.Registry`, making them available to the manual tool API and Agent Mode through the existing executor. Normal chat integration is deferred until a robust tool-calling loop exists.

**What should be deliberately deferred?**
- MCP Server mode (exposing OmniLLM as an MCP server)
- MCP Resources and Prompts (lower value, more complex)
- Remote HTTP/OAuth transport (security complexity)
- MCP sampling and elicitation (spec extensions, not core)
- Replacing the existing Plugin SDK (coexistence is better)

**Highest-risk areas:**
1. Stdio subprocess lifecycle management (cleanup, crashes, Windows compatibility)
2. Tool name collision and LLM provider naming constraints (dots vs underscores)
3. Real `ask` permission implementation for normal chat (Agent Mode `ask` is implemented; normal chat/manual API approval is still deferred)
4. Security model for arbitrary subprocess execution
5. **Chat Streaming Complexity:** OmniLLM does not currently pass native tool definitions to LLM providers. Normal chat relies on a pre-flight orchestrator (regex-based). Implementing a mid-stream tool-calling loop without native tool calling support is very messy and likely to leak JSON syntax to users.

**MVP scope:** MCP Client → stdio transport → tool discovery → tool execution via existing registry/executor → admin REST API for server config → per-tool permissions → audit logging. Settings UI follows the backend API.

---

## Section 2: Existing Architecture Observations

| Area | Current implementation | Relevant files | MCP implication |
|---|---|---|---|
| **Tool interface** | `tools.Tool` with `Definition()`, `Execute()`, `Validate()` | `backend/internal/tools/types.go` | MCP tools can implement this interface directly |
| **Tool registry** | `tools.Registry` — thread-safe `map[string]Tool` with `Register()`, `Get()`, `List()`, `ListEnabled()` | `backend/internal/tools/registry.go` | MCP tools register here; dynamic removal and underscore-safe namespacing are needed (`mcp_<server>_<tool>`) |
| **Tool executor** | `tools.Executor` — permission check → optional per-context approval handler for `ask` → validate → execute with timeout. Without an approval handler, `ask` returns an approval-required error. | `backend/internal/tools/executor.go` | MCP tools flow through same executor; Agent Mode supplies approval handling, while normal chat/manual API still need caller-specific approval UX |
| **Tool permissions** | `tool_permissions` table with `allow`/`deny`/`ask` policies. `ToolPermissionRepo` with `PolicyResolver()` | `backend/internal/repository/tool_permission.go`, V5 migration | Reuse directly for MCP execution policy; seed newly discovered MCP tools to `ask` |
| **Tool handler API** | `GET /v1/tools`, `POST /v1/tools/execute`, `PATCH /v1/tools/{name}/permission` | `backend/internal/api/tool_handler.go` | MCP tools appear automatically via registry; MCP server management still needs separate endpoints |
| **Agent planner** | `agent.Planner` — calls LLM with tool descriptions from registry, generates plan steps | `backend/internal/agent/planner.go` | MCP tools appear in planner's tool list automatically |
| **Agent runner** | `agent.Runner` — executes plan steps sequentially, handles approval via channel, SSE events | `backend/internal/agent/runner.go` | MCP tool calls flow through `executeStep` → `toolExecutor.Execute` |
| **Agent approval** | Approval steps block on channel; `ApproveStep()` sends signal. Works in Agent Mode only | `backend/internal/agent/runner.go` lines 220-280 | Real `ask` for chat needs separate implementation |
| **Plugin SDK** | JSON-RPC subprocess model with `PluginProcess`, `Loader` (discover, start, stop) | `backend/internal/plugins/loader.go`, `process.go` | MCP should use separate implementation — different protocol (JSON-RPC vs MCP), different lifecycle |
| **Plugin DB model** | `installed_plugins` table with name, version, manifest, enabled | V19 migration, `models.InstalledPlugin` | MCP servers need separate table — different schema |
| **Settings storage** | `settings` table (key-value), `AppSettings` struct, `SettingsRepo` | `backend/internal/models/models.go`, `repository/settings_repo.go` | MCP config is structured (multiple servers with env vars) — needs dedicated tables |
| **Crypto** | AES-256-GCM for API keys, machine-specific key in `~/.config/omnillm-studio/.omnillm_key` | `backend/internal/crypto/crypto.go` | Reuse for MCP server secrets (env vars, tokens) |
| **Auth** | Solo mode (passthrough) or multi-user with Bearer tokens, roles: admin/member/viewer | `backend/internal/auth/auth.go` | MCP server config should be admin-only by default |
| **SSE streaming** | `event:` + `data:` format for chat, agent runs, web search. `WriteTimeout: 5m` | `backend/internal/api/message_handler.go`, `agent_handler.go` | MCP tool execution results stream via existing SSE patterns |
| **Frontend settings** | `SettingsPanel.tsx` with tabs: providers, general, appearance, rag, **tools**, pricing, auth | `frontend/src/components/SettingsPanel.tsx` | MCP servers should be a new tab or sub-tab under Tools |
| **Frontend tools tab** | `ToolsTab` component — lists tools with allow/deny/ask dropdown | `frontend/src/components/SettingsPanel.tsx` | MCP tools appear here automatically; dedicated MCP server management now lives in Settings -> MCP |
| **Frontend API client** | `apiFetch<T>()`, typed API objects (`api.*`, `pluginApi.*`, `agentApi.*`, etc.) | `frontend/src/api.ts` | Add `mcpApi.*` following same pattern |
| **Frontend types** | TypeScript interfaces mirror Go models with `snake_case` | `frontend/src/types.ts` | Add MCP types following same conventions |
| **Frontend stores** | Zustand stores in `stores/index.ts` | `frontend/src/stores/index.ts` | Add MCP store or extend settings store |
| **DB migrations** | Versioned in `db.go` with `CREATE TABLE IF NOT EXISTS`, tracked in `schema_versions` | `backend/internal/db/db.go` (currently V1-V29) | Add V30+ for MCP tables |
| **Config** | `OMNILLM_*` env vars with defaults in `config/config.go` | `backend/internal/config/config.go` | Add `OMNILLM_MCP_DIR` or similar |
| **LLM service** | `llm.Service` — provider routing, chat, streaming, embeddings, image gen. **No native tool calling** — tools are server-side only | `backend/internal/llm/service.go` | MCP tools are server-side like existing tools; no LLM changes needed |
| **Chat message flow** | User message → web search gate → RAG injection → LLM stream → response. No tool-calling loop in normal chat | `backend/internal/api/message_handler.go` | MCP tools in chat require a new tool-calling loop (like Agent Mode but simpler) |
| **Web search** | `websearch.Orchestrator` — classifier gate → search → summarizer LLM | `backend/internal/websearch/orchestrator.go` | MCP web search tools are separate; orchestrator remains unchanged |
| **Sports lookup** | `sports.SportsLookupTool` implements `tools.Tool` — registered in registry, used by agent planner | `backend/internal/sports/tool.go` | Reference pattern for MCP tool adapter |
| **go.mod** | Go 1.24.1, chi, sqlite3, chromem-go, wails, etc. | `backend/go.mod` | No MCP SDK dependency needed for the current stdio/tools MVP |

### Key Gaps Discovered

1. **Normal chat has no tool-calling loop.** The `MessageHandler.Stream()` path uses a pre-flight `websearch.Orchestrator` to execute search before streaming, but there's no mechanism for the LLM to request tool calls mid-stream. Tools are only used dynamically in Agent Mode (via the planner/runner) or via the manual `POST /v1/tools/execute` endpoint. For MCP tools to work in normal chat, either native LLM tool calling needs to be implemented in `llm.Service` first, or MCP tools must be restricted to Agent Mode for the MVP. Mid-stream tool calling via text/JSON without provider support is extremely brittle and risks leaking syntax to the user.

2. **`ask` permission is caller-dependent.** Agent Mode now supplies a per-context approval handler to the executor, pauses on `ask`, and resumes or cancels through the existing approval endpoint. Manual tool execution and normal chat still return approval-required because they do not have an interactive approval channel yet.

3. **No dynamic tool registration.** The registry is populated at startup with hardcoded `MustRegister()` calls. MCP tools need to be registered dynamically when servers connect and deregistered on disconnect.

4. **Tool names have no namespace.** Current tools are flat names (`web_search`, `calculator`, `sports_lookup`). MCP needs namespacing, ideally using underscores to satisfy future LLM provider constraints.

5. **LLM provider integrations don't pass tool definitions.** The `llm.Service.ChatComplete()` and `ChatStream()` methods don't accept tool definitions. Tools are described to the LLM via system prompt text (in the agent planner), not via native function-calling APIs. This makes adopting MCP tools in normal chat much harder without first upgrading the LLM service to support native tool calling.

---

## Section 3: MCP Capability Model

| MCP primitive | OmniLLM-Studio mapping | MVP status |
|---|---|---|
| **Tools** | `tools.Tool` interface wrapper → `tools.Registry` → `tools.Executor` | **MVP** |
| **Resources** | Attachment browser / RAG context source picker | Later |
| **Prompts** | Prompt Templates system | Later |
| **Sampling** | Host-mediated model calls (LLM-as-judge) | Defer |
| **Elicitation** | User input/approval UI | Later |
| **Logging** | Server diagnostics panel + audit log | MVP-lite |
| **Progress** | Agent step events / SSE progress | Later |

### Transport support

| Transport | MVP status | Notes |
|---|---|---|
| **Stdio** | **MVP** | Subprocess management, `npx`, `uvx`, direct binaries |
| **Streamable HTTP** | Later | Remote MCP servers, requires auth model |
| **SSE (legacy)** | Defer | Deprecated in favor of Streamable HTTP |

### Server lifecycle

| Operation | MVP | Notes |
|---|---|---|
| Add server config | Yes | Name, transport, command/args/env |
| Start server | Yes | On config save + enabled |
| Stop server | Yes | Graceful shutdown |
| Restart server | Yes | Stop + start |
| Auto-start on app boot | Yes | Enabled servers start with backend |
| Test connection | Yes | `tools/list` ping |
| Discover tools | Yes | On start + refresh |
| Tool list change notifications | Later | Polling or server-sent |
| Remove server | Yes | Stop + delete config |

---

## Section 4: Recommended Product Behavior

### Settings UI

The MCP configuration should live as a new **"MCP Servers"** tab in the Settings panel, adjacent to the existing "Tools" tab. Each server card shows:

- Server name, transport type badge (stdio/HTTP)
- Connection status indicator (green=connected, yellow=starting, red=error, gray=disabled)
- Enable/disable toggle
- Start/stop/restart buttons
- "Test Connection" button
- Last error message (if any)
- Expandable tool list with per-tool enable/disable and permission policy
- Edit/Delete buttons
- Import/Export config JSON

### Chat behavior

**Recommendation:** Restrict MCP tools to Agent Mode for the MVP, or implement Native Tool Calling in `llm.Service` as a prerequisite. 

Because `MessageHandler.Stream()` does not support native tool calling and relies on a pre-flight regex-gated orchestrator for web searches, injecting a multi-turn tool-calling loop into normal streaming chat would require parsing the stream for JSON syntax and hiding it from the user. This is highly brittle. 

If native tool calling is added, the flow becomes:
1. User sends message
2. API passes MCP tool definitions natively to the provider
3. LLM responds with a `tool_calls` stop reason
4. Tool calls are executed via the executor (with permission checks)
5. Results are fed back to the LLM for a final response
6. Final response is streamed to the user

**Error handling:** If an MCP server disconnects mid-conversation, in-flight tool calls fail with a clear error. The LLM is informed and can respond accordingly.

### Agent Mode behavior

MCP tools appear automatically in the agent planner's tool list (via the registry). The planner can include `tool_call` steps for MCP tools. The runner executes them through the standard executor path. No Agent Mode changes are needed beyond dynamic tool registration.

### Multi-user/workspace behavior

- **Default:** MCP server configuration is admin-only (requires `admin` role)
- **Scope:** Global (all users/workspaces) for MVP
- **Future:** Per-workspace or per-user MCP servers
- **Secrets:** Never returned to frontend; encrypted at rest

---

## Section 5: Data Model and Migrations

### V30: `mcp_servers`

```sql
CREATE TABLE IF NOT EXISTS mcp_servers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    transport       TEXT NOT NULL DEFAULT 'stdio' CHECK(transport IN ('stdio','http')),
    command         TEXT,                          -- for stdio: executable path
    args_json       TEXT NOT NULL DEFAULT '[]',    -- for stdio: JSON array of args
    url             TEXT,                          -- for http: server URL
    env_json        TEXT NOT NULL DEFAULT '{}',    -- JSON object of env vars (values encrypted)
    enabled         INTEGER NOT NULL DEFAULT 0,
    workspace_id    TEXT,                          -- NULL = global. Future: scoped to workspace
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_enabled ON mcp_servers(enabled);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_workspace ON mcp_servers(workspace_id);
```

- `env_json` stores environment variables with encrypted values (using `crypto.Encrypt`)
- Server-scoped (global for MVP, `workspace_id` added for future-proofing)

### Tool permissions

MCP execution policy should use the existing `tool_permissions` table rather than a separate MCP-only permission table. The current `tools.Executor` receives a `PermissionResolver` keyed by internal tool name, and the existing `/v1/tools` API already reads and writes that table. Newly discovered MCP tools should be inserted into `tool_permissions` with policy `ask` only when no policy already exists.

Per-server metadata can be derived from the manager's discovered tool list for the MVP. If persisted tool enable/disable state is needed later, add a separate MCP tool settings table that stores MCP metadata but does not replace the canonical execution policy in `tool_permissions`.

### V31: `mcp_audit_log`

```sql
CREATE TABLE IF NOT EXISTS mcp_audit_log (
    id          TEXT PRIMARY KEY,
    server_id   TEXT NOT NULL,
    event_type  TEXT NOT NULL,  -- 'tool_call', 'tool_result', 'error', 'server_start', 'server_stop', 'server_crash'
    tool_name   TEXT,
    input_json  TEXT,
    output_json TEXT,
    duration_ms INTEGER,
    error_msg   TEXT,
    user_id     TEXT,
    workspace_id TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_mcp_audit_server ON mcp_audit_log(server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_audit_created ON mcp_audit_log(created_at);
```

### Design decision: reuse vs new tables

- **Tool permissions:** The existing `tool_permissions` table is the canonical execution policy store for both native and MCP tools. MCP-specific server/tool metadata is kept in runtime manager state for the MVP.
- **Settings:** The key-value `settings` table is unsuitable for structured MCP server config. Dedicated tables are required.
- **Tool permissions:** MCP execution policy reuses the existing `tool_permissions` table because `tools.Executor` resolves policy by registered internal tool name. Newly discovered MCP tools are seeded to `ask`.
- **Plugin tables:** `installed_plugins` has a different schema (manifest, version, entrypoint). Not reusable for MCP servers.

---

## Section 6: Backend Implementation Plan

### Phase 0: Codebase review and design validation (this document)

**Deliverable:** `docs/internal_docs/MCP_IMPLEMENTATION_PLAN.md`

### Phase 1: MCP client foundation

**New package:** `backend/internal/mcpclient/`

**Current implementation files:**

| File | Purpose |
|---|---|
| `manager.go` | `Manager` struct — server lifecycle, registry integration, startup/shutdown |
| `client.go` | MCP client per server — initialization, capability negotiation, tool discovery |
| `protocol.go` | Minimal MCP JSON-RPC request/response and tool result types |
| `names.go` | Provider-safe internal tool name mapping |
| `tool_adapter.go` | Converts MCP tool definitions → `tools.Tool` interface |

**Key design decisions:**

- **Use official MCP Go SDK?** The Go MCP ecosystem is immature. For MVP, implement the MCP protocol directly (it's simple JSON-RPC over stdio). The protocol spec is small enough that a custom implementation avoids dependency risks. Revisit when `github.com/mark3labs/mcp-go` or similar matures.
- **Stdio process management:** Use `os/exec` with `cmd.StdinPipe()`/`cmd.StdoutPipe()`. Read/write JSON-RPC messages line-delimited. Handle stderr for logging.
- **Concurrency:** One goroutine per server for reading responses. Use channels for request/response matching with `id` field correlation.
- **Cleanup:** `Manager.StopAll()` called from `main()` during graceful shutdown. Context cancellation for in-flight requests.

### Phase 2: Tool registry integration

MCP tools are wrapped as `tools.Tool` implementations via `tool_adapter.go`:

```go
type mcpToolAdapter struct {
    serverID   string
    serverName string
    toolName   string
    definition tools.ToolDefinition
    client     *mcpClient
}

func (a *mcpToolAdapter) Definition() tools.ToolDefinition {
    return a.definition
}

func (a *mcpToolAdapter) Execute(ctx context.Context, args json.RawMessage) (*tools.ToolResult, error) {
    // Call MCP server via client
    result, err := a.client.CallTool(ctx, a.toolName, args)
    // Normalize result
    return &tools.ToolResult{Content: result.Content, IsError: result.IsError}, nil
}

func (a *mcpToolAdapter) Validate(args json.RawMessage) error {
    // MVP validates a JSON object locally; the MCP server performs schema validation.
    return validateJSONObject(args)
}
```

**Naming convention:** `mcp_<server_slug>_<tool_name>`
- Slug: lowercase, underscores for spaces/special characters, alphanumeric plus underscores only
- Example: `mcp_filesystem_read_file`, `mcp_github_create_issue`
- Collision handling: If two servers expose the same tool name, append `_1`, `_2` etc.

**Dynamic registration:** The `Manager` calls `registry.Register()` for each discovered tool when a server starts, and removes them (via a new `registry.Remove(name)` method) when a server stops.

### Phase 3: REST API additions

**MVP endpoints:**

```
GET    /v1/mcp/servers              → List all configured MCP servers
POST   /v1/mcp/servers              → Create a new MCP server config
GET    /v1/mcp/servers/{id}         → Get server details + status
PATCH  /v1/mcp/servers/{id}         → Update server config
DELETE /v1/mcp/servers/{id}         → Delete server config (stops if running)
POST   /v1/mcp/servers/{id}/test   → Test connection (tools/list)
POST   /v1/mcp/servers/{id}/start  → Start server
POST   /v1/mcp/servers/{id}/stop   → Stop server
POST   /v1/mcp/servers/{id}/restart → Restart server
POST   /v1/mcp/servers/{id}/refresh → Re-discover tools
GET    /v1/mcp/servers/{id}/tools  → List discovered tools with permissions
PATCH  /v1/mcp/servers/{id}/tools/{toolName} → Update tool permission
```

**Deferred endpoints:**

```
GET    /v1/mcp/servers/{id}/resources → List MCP resources
GET    /v1/mcp/servers/{id}/prompts   → List MCP prompts
GET    /v1/mcp/audit                  → Query audit log
```

**Auth:** All MCP endpoints require `admin` role (gated by `auth.RequireRole("admin")`).

**Request/response models** follow existing conventions in `api/helpers.go` (`respondJSON`, `respondError`, `decodeJSON`).

### Phase 4: Frontend UI

**New components:**

| Component | Purpose |
|---|---|
| `MCPManager.tsx` | Main MCP settings tab — server list, add/edit/delete |
| `MCPServerForm.tsx` | Add/edit server form (name, transport, command, args, env vars) |
| `MCPToolList.tsx` | Per-server tool list with enable/disable and permission dropdowns |
| `MCPStatusBadge.tsx` | Connection status indicator |

**TypeScript types** (in `types.ts`):

```typescript
export interface MCPServer {
  id: string;
  name: string;
  transport: 'stdio' | 'http';
  command?: string;
  args_json?: string;
  url?: string;
  enabled: boolean;
  status: 'disconnected' | 'connecting' | 'connected' | 'error';
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface MCPTool {
  server_id: string;
  tool_name: string;
  description: string;
  enabled: boolean;
  policy: string;
}
```

**API client additions** (in `api.ts`):

```typescript
export const mcpApi = {
  listServers: () => apiFetch<MCPServer[]>('/mcp/servers'),
  createServer: (data: CreateMCPServerRequest) => apiFetch<MCPServer>('/mcp/servers', { method: 'POST', body: JSON.stringify(data) }),
  updateServer: (id: string, data: UpdateMCPServerRequest) => apiFetch<MCPServer>(`/mcp/servers/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteServer: (id: string) => apiFetch<void>(`/mcp/servers/${id}`, { method: 'DELETE' }),
  testServer: (id: string) => apiFetch<{ok: boolean}>('/mcp/servers/${id}/test', { method: 'POST' }),
  startServer: (id: string) => apiFetch<{ok: boolean}>('/mcp/servers/${id}/start', { method: 'POST' }),
  stopServer: (id: string) => apiFetch<{ok: boolean}>('/mcp/servers/${id}/stop', { method: 'POST' }),
  restartServer: (id: string) => apiFetch<{ok: boolean}>('/mcp/servers/${id}/restart', { method: 'POST' }),
  refreshTools: (id: string) => apiFetch<MCPTool[]>('/mcp/servers/${id}/refresh', { method: 'POST' }),
  listTools: (id: string) => apiFetch<MCPTool[]>(`/mcp/servers/${id}/tools`),
  updateToolPermission: (serverId: string, toolName: string, policy: string) =>
    apiFetch<void>(`/mcp/servers/${serverId}/tools/${toolName}`, { method: 'PATCH', body: JSON.stringify({ policy }) }),
};
```

**UI placement:** MCP servers get their own tab in the Settings panel, between "Tools" and "Pricing". The existing Tools tab continues to show all tools (native + MCP) with their permissions.

### Phase 5: Permission and approval improvements

**Current state:** Agent Mode `ask` approval is implemented. The executor supports a per-context approval handler; Agent Mode provides one for tool-call steps and reuses the existing approval channel/API. If no handler is attached, `ask` returns approval-required.

**Required for MCP:**

1. **Executor-level `ask` for Agent Mode:** Complete. When the executor encounters `ask` policy in Agent Mode, the runner emits an `agent_approval_required` SSE event, blocks on the existing `runContext.approval` channel, resumes after approval, and cancels after rejection.

2. **Executor-level `ask` for normal chat (Deferred to Phase 8):** Once native tool calling is implemented for chat, when the executor encounters `ask` policy during a chat tool call, it needs to:
   - Pause the streaming response
   - Emit an SSE event requesting approval
   - Wait for user approval via a new API endpoint
   - Resume or abort based on decision

3. **New API endpoint:** `POST /v1/tools/approve/{callId}` — approve or reject a pending tool call.

**Recommended defaults:**

| Tool type | Default policy |
|---|---|
| Native safe tools (calculator, web_search) | `allow` |
| MCP read-only tools (read_file, list_directory) | `ask` |
| MCP write/destructive tools (write_file, delete, execute) | `ask` |
| Unknown MCP tools | `ask` |
| Remote HTTP MCP servers | `ask` |

### Phase 6-9: Deferred

See Section 17 for the full phased roadmap.

---

## Section 7: Security and Trust Model

### Threats

| Threat | Severity | Mitigation |
|---|---|---|
| Data exfiltration via filesystem tools | High | Default `ask` policy; path restrictions in server config |
| Prompt injection via MCP resources | Medium | Resources deferred; tool descriptions sanitized |
| Tool poisoning via misleading descriptions | Medium | Admin-only server config; user reviews tools before enabling |
| Destructive tool calls (delete, execute) | High | Default `ask` policy; explicit enablement |
| Credential leakage in env vars | High | Encrypted at rest; never returned to frontend; redacted in logs |
| Remote MCP server compromise | High | HTTP transport deferred; stdio only for MVP |
| Local subprocess compromise | High | Run as same user; no sandboxing for MVP. **NOTE:** Subprocesses inherit the same OS permissions as the OmniLLM backend process. |
| Path traversal | Medium | Server config is admin-only; user responsible for safe paths |
| Environment variable leakage | Medium | Redacted in logs and audit |
| Cross-user data leakage | Low | Single-user mode for MVP; multi-user scoping deferred |
| Long-running/hanging servers | Medium | Timeouts on tool calls; server health checks |
| DoS via large outputs | Medium | Output size limits; streaming truncation |
| Untrusted binary execution | High | Warning UI for `npx`/`uvx` commands; admin-only config |

### Controls

1. **Admin-only configuration** — MCP server management requires `admin` role
2. **Explicit enablement** — Servers are disabled by default; must be explicitly enabled
3. **Per-tool permissions** — Each MCP tool has `allow`/`deny`/`ask` policy
4. **Secrets encrypted at rest** — Env var values encrypted via `crypto.Encrypt()`
5. **Secrets never returned to frontend** — API responses redact encrypted fields
6. **Environment redaction in logs** — Values matching `*KEY*`, `*TOKEN*`, `*SECRET*` patterns redacted
7. **Timeouts** — Default 30s per request/tool call
8. **Output size limits** — 4MB max JSON-RPC message read; normalized tool content truncated at 100KB
9. **Audit logging** — MCP server lifecycle, tool calls, durations, inputs, normalized outputs, and errors are logged
10. **Secure defaults** — `ask` policy for all MCP tools by default

### Open security questions

- Should MCP server commands be restricted to an allowlist (e.g., only `npx`, `uvx`, `docker`)?
- Should filesystem paths in server args be validated/restricted?
- How should `npx` trust warnings be handled in the UI?

---

## Section 8: Configuration Examples

### Stdio example (persisted format)

```json
{
  "id": "uuid-here",
  "name": "filesystem",
  "transport": "stdio",
  "command": "npx",
  "args_json": "[\"-y\", \"@modelcontextprotocol/server-filesystem\", \"/Users/adam/projects\"]",
  "env_json": "{}",
  "enabled": true
}
```

### Stdio with secrets

```json
{
  "id": "uuid-here",
  "name": "github",
  "transport": "stdio",
  "command": "npx",
  "args_json": "[\"-y\", \"@modelcontextprotocol/server-github\"]",
  "env_json": "{\"GITHUB_TOKEN\": \"<encrypted>\"}",
  "enabled": true
}
```

### API format (frontend ↔ backend)

```json
{
  "name": "github",
  "transport": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-github"],
  "env": {
    "GITHUB_TOKEN": "ghp_xxxxxxxxxxxx"
  }
}
```

The backend encrypts `env` values before storing in `env_json`. The frontend never receives encrypted values back — the API response omits `env` or returns `{"GITHUB_TOKEN": "••••••••"}`.

---

## Section 9: Tool Naming Convention

**Format:** `mcp_<server_slug>_<tool_name>`

**Slug normalization:**
- Lowercase
- Replace spaces/special chars with hyphens or underscores
- Remove non-alphanumeric chars except hyphens/underscores
- Max 64 chars total for the full name to comply with OpenAI requirements

**Examples:**
- `mcp_filesystem_read_file`
- `mcp_filesystem_write_file`
- `mcp_github_create_issue`
- `mcp_postgres_query`

**Collision handling:** If two servers expose `read_file`, the second gets `read_file_1`.

**LLM provider constraints:** Even though the current architecture passes tools via text, we MUST use underscores instead of dots. OpenAI strictly enforces `^[a-zA-Z0-9_-]{1,64}$` for tool names in native function calling. Using underscores now ensures MCP tools will be compatible without name rewriting when native tool calling is implemented.

**Display vs internal:** Internal name uses underscores. Display name shown to users can use a cleaner format: `filesystem → read_file`.

---

## Section 10: Tool Result Normalization

MCP tools can return multiple content types. The adapter normalizes them into the existing `ToolResult` model:

```go
type ToolResult struct {
    ToolCallID string                 `json:"tool_call_id"`
    Content    string                 `json:"content"`
    IsError    bool                   `json:"is_error"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

**Content type mapping:**

| MCP content type | ToolResult mapping |
|---|---|
| `text` | `Content` = text value |
| `image` | `Content` = `[Image: <mime_type>]`; binary/base64 payload is not exposed in MVP |
| `audio` | `Content` = `[Audio: <mime_type>]`; binary/base64 payload is not exposed in MVP |
| `resource` (text) | `Content` = resource text; metadata preserved |
| `resource` (blob) | `Content` = `[Resource: <mime_type>]`; metadata preserved |
| `resource_link` | `Content` = `[Resource: <name> <uri>]` |
| Multiple content items | Concatenated with newlines |
| Error | `IsError = true`; `Content` = error message |

**Large output truncation:** If `Content` exceeds 100KB, truncate and append `[Truncated: X bytes]`.

---

## Section 11: Logging, Audit, and Observability

**Log levels:**
- `INFO`: Server start/stop, tool discovery
- `WARN`: Server crash, tool call failure, reconnection
- `ERROR`: Fatal server errors, security violations

**Redaction rules:**
- Environment variable values matching `*KEY*`, `*TOKEN*`, `*SECRET*`, `*PASSWORD*` → `[REDACTED]`
- Full env block redacted in non-debug logs
- Tool call arguments are logged (user-provided, not secret)

**Audit table** (`mcp_audit_log`):
- Every tool call: server_id, tool_name, input_json, output_json, duration_ms
- Every error: error_msg
- Every server lifecycle event: server_start, server_stop, server_crash

**Correlation:** The current table includes `user_id` and `workspace_id` columns for future correlation. The current adapter does not yet populate conversation or agent run identifiers.

---

## Section 12: Testing Strategy

### Unit tests

| Area | What to test |
|---|---|
| Config validation | Valid/invalid server configs, slug normalization |
| Tool name mapping | Namespace generation, collision handling |
| Tool adapter | MCP tool → Tool interface conversion |
| Result normalization | All MCP content types → ToolResult |
| Permission behavior | allow/deny/ask resolution |
| Secret redaction | Env var redaction in logs |
| Schema conversion | MCP JSON Schema → ToolDefinition.Parameters |
| Error handling | Server disconnect, timeout, malformed responses |

### Integration tests

| Scenario | What to test |
|---|---|
| Mock MCP server over stdio | Tool discovery, tool call execution |
| Server crash/restart | Auto-reconnect, error reporting |
| Timeout behavior | Tool call exceeding timeout |
| Tool list refresh | Dynamic tool add/remove |
| Permission denial | `deny` policy blocks execution |
| Approval flow | `ask` policy triggers approval event |

### Frontend tests (manual for MVP)

| Scenario |
|---|
| Add MCP server via UI |
| Test connection |
| View discovered tools |
| Enable/disable tools |
| Execute MCP tool from Agent Mode |
| Error handling (server offline) |
| Secret input (env vars masked) |

### Regression tests

- Existing built-in tools still work
- Agent Mode still works with native tools
- Plugin SDK still works
- Settings UI remains functional
- Desktop/Wails build still works
- Docker/Helm builds still work

---

## Section 13: Backward Compatibility

- **Existing conversations:** Unchanged — MCP tools are new, not modifications
- **Existing tools:** Unchanged — MCP tools are additional registrations
- **Existing plugins:** Unchanged — MCP and Plugin SDK coexist
- **Existing settings:** Unchanged — MCP has its own tables
- **Existing feature flags:** Unchanged — MCP doesn't need a flag (disabled until configured)
- **Existing desktop builds:** Need to ensure `npx`/`node` discovery works in Wails context
- **Existing Docker/Helm:** MCP subprocesses may not work in containers — documented limitation
- **Users without MCP configured:** No impact — MCP is entirely optional
- **Migration rollback:** New tables can be dropped; no existing data affected

---

## Section 14: Build and Dependency Considerations

**Go SDK decision:** Implement MCP protocol directly (JSON-RPC 2.0 over stdio). The protocol is simple enough that a dependency is unnecessary for MVP. If the community SDK (`mark3labs/mcp-go`) matures, it can be adopted later.

**No new Go dependencies for MVP.** The stdlib `os/exec`, `encoding/json`, and `net/http` are sufficient.

**Platform considerations:**
- **Windows:** Stdio subprocess management works but needs `.cmd`/`.exe` resolution for `npx`
- **Linux/macOS:** Standard POSIX subprocess management
- **Wails desktop:** Working directory and PATH may differ from web mode — need to document

**Docker impact:** MCP subprocess servers (especially `npx`-based) may not work in minimal Docker images. Document that MCP in Docker requires either pre-installed runtimes or remote HTTP MCP servers.

**Helm impact:** Subprocess execution inside Kubernetes pods is possible but requires security context considerations. Recommend remote HTTP MCP servers for Helm deployments.

---

## Section 15: Deployment Considerations

### Local web mode
- Server process launches MCP subprocesses directly
- Uses local filesystem for config and secrets
- PATH and runtime discovery work normally

### Desktop/Wails mode
- Same as local mode
- **CRITICAL:** Wails apps on macOS/Windows often do not inherit the user's terminal `$PATH`. Therefore, commands like `npx`, `node`, or `uvx` might fail with "executable not found". The UI must provide clear instructions allowing users to specify absolute paths (e.g., `/opt/homebrew/bin/npx` or `C:\Program Files\nodejs\npx.cmd`) in the `command` field.
- Working directory should be the app's data directory.

### Docker mode
- Subprocess MCP servers may not exist in container
- Filesystem access limited to container mounts
- Recommend remote HTTP MCP servers or pre-installed runtimes in custom images
- Document that stdio MCP is limited in Docker

### Kubernetes/Helm mode
- Running arbitrary subprocesses inside backend pod is not recommended
- Better to use remote HTTP MCP servers or sidecar pattern
- Need security context considerations
- Need network policy for outbound connections

---

## Section 16: Open Questions

1. **Should MCP servers be global or per-user?** Global for MVP (admin-managed). Per-user servers are a future enhancement.

2. **Should non-admin users be able to add personal MCP servers?** Deferred. MVP is admin-only.

3. **Should tools be available to all workspaces or scoped?** Global for MVP. Workspace scoping deferred.

4. **Should server config support `npx`, `uvx`, Docker, or only direct executables?** MVP supports any executable command. `npx` and `uvx` are common patterns. Docker is deferred.

5. **Should MCP be disabled in hosted/multi-user mode by default?** Yes — admin must explicitly configure servers.

6. **How should approval work for normal chat tool calls?** Need a new pause-and-approve mechanism in the streaming path. Agent Mode already has this.

7. **Should MCP resources be indexed into RAG automatically or only attached manually?** Deferred with resources.

8. **How should remote OAuth be handled in desktop vs web mode?** Deferred with HTTP transport.

9. **How much of MCP spec should be implemented in MVP?** Tools only (list, call). Resources, prompts, sampling, and notifications deferred.

10. **Should the existing `tools.Registry` support dynamic removal?** Yes. The backend implementation adds `Registry.Remove(name)` for MCP stop/restart.

11. **Should MCP tool calls in normal chat use a single-turn or multi-turn loop?** Single-turn for MVP (LLM requests tools → execute → respond). Multi-turn (agent-like) deferred.

12. **How should `npx` trust prompts be handled?** The `--yes` flag suppresses them, but this has security implications. Document as a user responsibility.

---

## Section 17: Final Phased Roadmap

| Phase | Name | Scope | Dependencies | Risk | Deliverable |
|---|---|---|---|---|---|
| 0 | Review and plan | Codebase analysis and design doc | None | Low | `MCP_IMPLEMENTATION_PLAN.md` |
| 1 | Client foundation | Stdio client, config, lifecycle, DB migrations | SDK decision | Medium | `backend/internal/mcpclient/` package |
| 2 | Tool integration | MCP tools in registry/executor, dynamic registration | Phase 1 | Medium | MCP tools usable by backend Agent Mode |
| 3 | REST API | MCP server CRUD + lifecycle endpoints | Phase 2 | Low | `api/mcp_handler.go` + routes |
| 4 | Frontend UI | MCP settings tab, server form, tool list | Phase 3 | Medium | User-configurable servers |
| 5 | Approval flow | Real `ask` implementation for Agent Mode executor | Phase 4 | High | Safe tool execution in agents |
| 6 | Audit & logging | Audit table, admin activity view | Phase 3 | Low | Observability |
| 7 | Native Tool Calling | Upgrade `llm.Service` to pass tools natively to providers | Phase 2 | High | Pre-requisite for chat integration |
| 8 | Chat integration | Native tool-calling loop in `MessageHandler.Stream()` | Phase 7 | High | MCP tools safely in normal chat |
| 9 | Resources/prompts | MCP resources and prompt templates | Phase 4 | Medium | Context expansion |
| 10 | Remote HTTP/OAuth | Remote MCP servers | Phase 4 | High | Remote server support |
| 11 | MCP server mode | Expose OmniLLM tools externally | Phase 2+ | Medium | OmniLLM as MCP server |

### Progress update: 2026-05-09

Completed after the original backend MVP:

- **Phase 4: Frontend UI** - Settings now includes an MCP tab with server create/edit, a filesystem template, test/start/stop/restart/refresh actions, status/error display, env key display, and per-tool policy controls.
- **Phase 5: Agent Mode approval flow** - `tools.Executor` now accepts a per-context approval handler. Agent Mode attaches that handler for tool-call steps, emits `agent_approval_required`, pauses on `ask`, resumes after approval, and cancels after rejection.
- **Phase 6: Audit & logging view** - Added `GET /v1/mcp/audit`, repository audit listing, config/policy audit events, and an MCP activity panel in Settings.

Still deferred:

- **Phase 7/8:** Native provider tool calling and normal chat MCP use.
- **Phase 9:** MCP resources and prompts.
- **Phase 10:** Streamable HTTP/OAuth transport.
- **Phase 11:** MCP server mode.

---

## Section 18: Future Implementation Guidance

When implementing this plan:

1. **Work one phase at a time.** Do not mix backend, frontend, and DB work in uncontrolled large commits.
2. **Add migrations before repository logic.** Follow the current V30/V31 MCP migration pattern in `db.go`.
3. **Add backend models before handlers.** Models in `models.go`, repos in `repository/`, then handlers in `api/`.
4. **Add API client types before UI.** Types in `types.ts`, API functions in `api.ts`, then components.
5. **Preserve existing native tools and plugin behavior.** MCP is additive.
6. **Keep MCP optional.** Disabled until a server is configured.
7. **Add tests for each phase.** Unit tests for adapters, integration tests with mock servers.
8. **Do not log secrets.** Redact env var values matching sensitive patterns.
9. **Use existing patterns.** Follow the conventions documented in `CLAUDE.md` and `.github/copilot-instructions.md`.
10. **Update README only after the feature is functional.**

---

## Summary

**Recommendation:** Add MCP Client support starting with stdio transport and tool integration. The existing architecture maps cleanly to MCP concepts. The highest-value, lowest-risk path is:

1. MCP client foundation (stdio, config, lifecycle)
2. Tool registry integration (dynamic registration, naming)
3. REST API + frontend UI (server management)
4. Agent Mode approval flow (real `ask` implementation)
5. Chat integration (native provider tool-calling loop)

Items 1-4 are now implemented for the stdio/tools capability. Item 5 remains deferred.

**Key risks:** Subprocess lifecycle management, tool name collisions with LLM providers, and implementing real-time approval for normal chat.

**Open questions:** Scope of MCP servers (global vs per-user), `npx` trust handling, and Docker/Helm compatibility for stdio servers.
