> **Archived — superseded status snapshot.** It predates the shipped OAuth and current Streamable HTTP support; use [MCP_HOW_TO_FAQ.md](../../MCP_HOW_TO_FAQ.md).

# MCP Capability Status

Last updated: 2026-05-09

## Scope

Build MCP client support for OmniLLM-Studio, starting with stdio servers that expose tools. MCP tools should register into the existing `tools.Registry` and execute through the existing `tools.Executor`.

## Plan Corrections

- The repository is currently at schema migration V29, so MCP migrations start at V30.
- MCP tool execution policy uses the existing `tool_permissions` table. New MCP tools are seeded to `ask` on discovery.
- Internal MCP tool names use provider-safe underscores: `mcp_<server_slug>_<tool_slug>`.
- Normal chat MCP tool-calling remains deferred until native provider tool calling or another robust tool-call loop exists.
- MCP server management requires dedicated `/v1/mcp` endpoints; the existing `/v1/tools` endpoints only cover registered tool execution and policy.
- `ask` policy is now interactive in Agent Mode. Manual tool execution and normal chat still do not have a pause-and-approve path.

## Implementation Status

| Area | Status | Notes |
|---|---|---|
| Plan validation | Complete | Checked against current backend architecture and current MCP transport/tool docs. |
| Tracking document | Complete | This file tracks capability status and next work. |
| Database migrations | Complete | Added `mcp_servers` and `mcp_audit_log` as V30/V31. |
| MCP repository | Complete | CRUD, encrypted env storage, redacted responses, audit inserts. |
| Dynamic registry | Complete | Added `Registry.Remove()` for server stop/restart. |
| Stdio MCP client | Complete | JSON-RPC over newline-delimited stdio, initialize, tools/list, tools/call. |
| Tool adapter | Complete | Wraps MCP tools as `tools.Tool`; normalizes content to `ToolResult`. |
| Manager | Complete | Server lifecycle, discovery, default `ask` policy seeding, graceful shutdown. |
| Streamable HTTP client | Complete | Implemented `mcpclient.HTTPClient` following MCP 2025-06-18 spec (POST for all requests, SSE responses, custom headers, session ID). |
| REST API | Complete | Admin-only MCP server CRUD and lifecycle endpoints under `/v1/mcp`. |
| Frontend API/types | Complete | Added typed `mcpApi` client and MCP TypeScript interfaces, including audit event reads. |
| Focused MCP unit tests | Complete | Tool naming and result normalization tests added. |
| Real-world MCP tests | Complete | Added opt-in tests against `@modelcontextprotocol/server-filesystem@2025.8.21`; latest results in `MCP_REAL_WORLD_TEST_RESULTS.md`. |
| Frontend UI | Complete | Added Settings -> MCP tab with server create/edit, filesystem template, lifecycle controls, tool policy controls, and status/errors. |
| Agent Mode integration | Complete | Registry/executor path makes discovered tools visible to Agent Mode once servers are connected. |
| Agent Mode `ask` approval | Complete | Executor accepts a per-run approval handler; Agent Mode emits approval events and resumes/cancels through the existing approval endpoint. |
| Audit activity view | Complete | Added `GET /v1/mcp/audit`, repository listing, config/policy audit events, and Settings -> MCP activity panel. |
| Normal chat integration | Complete | Added native OpenAI-compatible tool calling to `llm.Service` and a robust tool-calling loop in `message_handler.go`. |
| Manual/API `ask` approval | Deferred | `POST /v1/tools/execute` still returns approval-required for `ask` because there is no interactive caller. |

## Current Decisions

- Transports: stdio and Streamable HTTP (2025-06-18) are supported.
- Legacy SSE transport (pre-2025-06-18) is deferred/unsupported.
- MCP resources, prompts, sampling, and elicitation are deferred.
- MCP servers are global and admin-managed for MVP.
- Server env values and HTTP headers are encrypted at rest and never returned in plaintext.

## Spec References

- [MCP 2025-06-18 transports](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports): stdio uses UTF-8 JSON-RPC messages delimited by newlines; Streamable HTTP replaces legacy HTTP+SSE.
- [MCP 2025-06-18 tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools): MVP uses `tools/list` for discovery and `tools/call` for execution.

## Verification

- `go test ./internal/mcpclient` passes.
- `go test ./internal/tools ./internal/agent ./internal/repository ./internal/api ./internal/mcpclient` passes after Phase 4-6 implementation.
- `OMNILLM_RUN_REAL_MCP_TESTS=1 go test ./internal/mcpclient -run RealWorld -v -count=1` passes against the official filesystem MCP server.
- `go test ./...` passes after allowing Go to use its normal build cache outside the workspace.
- `npm run build` passes after allowing Vite/esbuild to read required filesystem metadata outside the sandbox.
- Manual smoke after API implementation: create disabled server, enable/start server, list discovered tools, stop server, confirm tools unregister.
