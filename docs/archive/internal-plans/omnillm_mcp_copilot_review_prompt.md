> **Archived — superseded feasibility prompt.** The MCP client architecture is implemented and documented in the active MCP references.

# GitHub Copilot Brief: Feasibility Review and Implementation Plan for MCP Support in OmniLLM-Studio

## Purpose of this prompt

You are GitHub Copilot Agent Mode working inside the `ajbergh/OmniLLM-Studio` repository.

Your task is **not to implement MCP support yet**.

Your task is to perform a deep technical review of the existing codebase and create a detailed implementation plan as a new Markdown file in the repository.

Create the final output as:

```text
docs/internal_docs/MCP_IMPLEMENTATION_PLAN.md
```

If `docs/internal_docs/` does not exist, create it.

The plan should be written as an implementation-ready technical design document for adding **Model Context Protocol (MCP)** support to OmniLLM-Studio.

---

## Project context

OmniLLM-Studio is a local-first LLM chat and image editing suite with a Go backend and React/TypeScript frontend.

The app already includes many capabilities that are adjacent to MCP:

- Multi-provider LLM chat
- Streaming responses over SSE
- OpenAI-compatible provider support
- Anthropic, OpenAI, Gemini, Ollama, OpenRouter, Groq, Together AI, Mistral, and custom provider support
- Image Studio for generation, editing, masking, variant comparison, and image session history
- RAG over uploaded files using embedded vector storage
- Conversation management and branching
- Agent Mode with planning, tool calling, user approval steps, and run visualization
- Web search tools
- URL fetch tools
- Calculator tools
- ESPN-backed sports lookup tools
- Artifact/document generation tools including `.docx`, `.xlsx`, `.csv`, `.pdf`, `.md`, `.html`, `.json`, and `.yaml`
- Prompt templates
- Usage and cost analytics
- Optional multi-user auth and roles
- Encrypted secrets
- JSON-RPC subprocess plugin SDK
- Tool permission policies
- Desktop app support through Wails
- Web server mode
- SQLite local storage

The key architectural idea is that MCP should be treated as a **standards-based interoperability layer**, not as a replacement for everything that already exists.

---

## High-level decision to validate

The working recommendation is:

1. **Add MCP Client support first**
   - OmniLLM-Studio should be able to connect to external MCP servers.
   - External MCP tools should become available in chat and Agent Mode.
   - MCP tools should be surfaced through the existing tool registry/executor model where possible.

2. **Defer MCP Server support**
   - Exposing OmniLLM-Studio itself as an MCP server is useful, but not the first priority.
   - This can come after MCP Client support is stable.

3. **Do not replace the existing Plugin SDK initially**
   - The existing JSON-RPC plugin SDK can coexist with MCP.
   - Later, OmniLLM plugins may optionally support MCP-style runtimes or wrappers.
   - For now, keep Plugin SDK and MCP as separate but interoperable extension paths.

4. **Start with MCP tools**
   - MCP tools provide the highest immediate value.
   - Resources, prompts, remote HTTP/OAuth, elicitation, sampling, and OmniLLM-as-server can come later.

Copilot should validate, refine, or challenge this recommendation based on the actual codebase.

---

## What MCP should mean for OmniLLM-Studio

Model Context Protocol support should allow OmniLLM-Studio to connect to external MCP servers and make their capabilities available through the OmniLLM user experience.

In practical terms, users should eventually be able to configure MCP servers such as:

- Filesystem server
- GitHub server
- GitLab server
- Postgres server
- SQLite server
- Browser automation server
- Slack server
- Notion server
- Jira server
- Linear server
- Custom local script/tool server
- Any compliant stdio or HTTP MCP server

Once connected, OmniLLM-Studio should be able to:

- Discover the tools exposed by each server
- Display them in the UI
- Enable or disable servers
- Enable or disable individual tools
- Apply permissions per tool
- Let chat and Agent Mode invoke those tools
- Show tool results in the conversation or agent run
- Log MCP connection and execution errors clearly
- Persist MCP server configuration
- Keep secrets encrypted and server-side
- Respect user/workspace boundaries in multi-user mode
- Avoid giving the model unrestricted access to external data or destructive actions

---

## Required first step: deep codebase review

Before writing the implementation plan, perform a careful review of the existing repository.

Do not rely only on README-level assumptions. Inspect the code.

At minimum, review these areas:

### Backend

Review these packages and files, adjusting paths if the code has moved:

```text
backend/internal/api/
backend/internal/tools/
backend/internal/agent/
backend/internal/plugins/
backend/internal/models/
backend/internal/repository/
backend/internal/db/
backend/internal/config/
backend/internal/crypto/
backend/internal/auth/
backend/internal/llm/
backend/internal/rag/
backend/internal/websearch/
backend/internal/sports/
backend/internal/artifacts/
backend/internal/wordgen/
backend/go.mod
```

Specifically inspect:

- Router composition and dependency wiring
- Current API route structure
- Tool registry and tool executor
- Tool definition schema
- Tool permission repository
- Tool handler API
- Built-in tool implementations
- Agent planner
- Agent runner
- Agent step storage
- Agent approval model
- Current plugin runtime
- Plugin manifest model
- Plugin install/update/delete handling
- Settings storage
- Provider profile storage
- Secret encryption approach
- Auth middleware and role checks
- Workspace/user scoping
- SQLite migration patterns
- Logging patterns
- SSE event patterns
- Test conventions

### Frontend

Review these areas:

```text
frontend/src/
frontend/src/api.ts
frontend/src/types.ts
frontend/src/components/
frontend/src/stores/
frontend/src/components/SettingsPanel.tsx
frontend/src/components/PluginManager.tsx
frontend/src/components/AgentRunView.tsx
frontend/src/components/ChatView.tsx
```

Specifically inspect:

- API client conventions
- TypeScript type mirroring of Go models
- Settings UI structure
- Tools UI, if any
- Plugin management UI
- Agent run visualization
- Approval step UI
- Error display patterns
- Zustand stores
- Component style conventions
- Tailwind/shadcn/lucide usage, if present
- Whether MCP should be its own settings tab or part of the Tools tab

### Existing tests

Review current test patterns:

```text
backend/internal/**/*_test.go
frontend test setup, if any
```

Document:

- What currently has test coverage
- What MCP-related areas should get unit tests
- What integration tests are feasible
- What should be manually tested in the UI

---

## Primary deliverable

Create this file:

```text
docs/internal_docs/MCP_IMPLEMENTATION_PLAN.md
```

The document should include the following sections.

---

# Required section 1: Executive summary

Summarize:

- Whether MCP support is feasible
- Whether it is valuable for OmniLLM-Studio
- Whether it is required or optional
- The recommended implementation sequence
- The highest-risk areas
- The first MVP scope

Be clear and opinionated.

The executive summary should answer:

```text
Should this project add MCP support?
What should be built first?
What should be deliberately deferred?
```

---

# Required section 2: Existing architecture observations

Based on code inspection, describe the relevant current architecture.

Include:

- Backend layering
- Tool registry model
- Tool executor model
- Permission model
- Agent Mode flow
- Plugin SDK flow
- Settings/config storage
- Auth and role model
- Frontend settings/plugin/tool surfaces
- Any relevant DB tables/migrations
- Gaps or inconsistencies discovered during review

Do not simply restate the README. Cite actual files and functions by path.

Use a table like this:

```markdown
| Area | Current implementation | Relevant files | MCP implication |
|---|---|---|---|
| Tool registry | ... | `backend/internal/tools/registry.go` | ... |
| Tool executor | ... | `backend/internal/tools/executor.go` | ... |
```

---

# Required section 3: MCP capability model

Explain how MCP should map into OmniLLM-Studio.

Include at least these mappings:

```markdown
| MCP primitive | OmniLLM-Studio mapping | MVP status |
|---|---|---|
| Tools | Existing tool registry/executor | MVP |
| Resources | Attachment/context/RAG source browser | Later |
| Prompts | Prompt Templates | Later |
| Sampling | Host-mediated model calls | Defer |
| Elicitation | User input/approval UI | Later |
| Logging | Server diagnostics/debug panel | MVP-lite |
| Progress | Agent step events / UI progress | Later |
```

Also cover:

- Stdio transport
- Streamable HTTP transport
- Remote authentication
- Tool list change notifications
- Resource list change notifications
- Capability negotiation
- Server lifecycle
- Multi-server support
- Namespacing tools by server

---

# Required section 4: Recommended product behavior

Describe the intended user experience.

Include:

## Settings UI

User should be able to:

- Add an MCP server
- Edit an MCP server
- Delete an MCP server
- Enable/disable an MCP server
- Start/stop/restart a server
- Test connection
- View discovered tools
- Enable/disable individual tools
- Set permission policy per tool
- See connection status
- See last error
- Import MCP server config JSON
- Export MCP server config JSON with secrets redacted

## Chat behavior

Describe how MCP tools should become available in normal chat.

Cover:

- Whether tool use is automatic or opt-in
- How tool descriptions are exposed to the LLM
- How tool results appear
- How errors appear
- How user approval should work
- What should happen if a server disconnects mid-conversation

## Agent Mode behavior

Describe how MCP tools should appear to the Agent planner and runner.

Cover:

- Tool discovery at planning time
- Tool namespacing
- Approval before risky actions
- Tool results as step output
- Cancellation behavior
- Timeouts
- Long-running tools
- Retry policy

## Multi-user/workspace behavior

Cover:

- Server configuration scope
- Per-user vs global servers
- Workspace scoping
- Admin-only configuration
- Member/viewer access
- Secret visibility

---

# Required section 5: Data model and migrations

Propose the SQLite schema changes.

Include suggested tables such as:

```sql
mcp_servers
mcp_server_env
mcp_tools
mcp_tool_permissions
mcp_server_status
mcp_audit_log
```

But do not blindly implement these exact tables if the current repository has a better existing model.

For each table, document:

- Purpose
- Columns
- Indexes
- Whether it stores secrets
- Whether it is user-scoped or global
- Migration file naming
- Backward compatibility considerations

Important: determine whether existing settings, plugin, and tool permission tables can be reused. If reusing is better, recommend reuse instead of new tables.

---

# Required section 6: Backend implementation plan

Break this into phases.

## Phase 0: Codebase review and design validation

This is the phase Copilot is doing now.

Deliverable:

```text
docs/internal_docs/MCP_IMPLEMENTATION_PLAN.md
```

No functional code should be changed except creating the plan document.

## Phase 1: MCP client foundation

Design a new package, likely:

```text
backend/internal/mcpclient/
```

Include proposed files such as:

```text
manager.go
config.go
session.go
transport_stdio.go
transport_http.go
tool_adapter.go
resource_adapter.go
prompt_adapter.go
errors.go
logging.go
```

The plan should determine whether all these files are necessary.

Cover:

- Use of official MCP Go SDK if practical
- Client initialization
- Server lifecycle
- Stdio process management
- HTTP client support
- Capability negotiation
- Tool discovery
- Tool call execution
- Context cancellation
- Timeouts
- Server restart behavior
- Logging stderr/stdout safely
- Avoiding deadlocks
- Concurrency model
- Cleanup on shutdown
- Handling tool list change notifications

## Phase 2: Tool registry integration

Explain how MCP tools should be registered.

Preferred design:

- MCP tools become wrappers implementing the existing `tools.Tool` interface
- Tool names are prefixed/namespaced
- Native tools and MCP tools share the existing executor
- Existing permission model should apply
- MCP tool schemas should be adapted into existing `ToolDefinition.Parameters`

Address:

- Name collisions
- Disabled tools
- Dynamic tool updates
- Server disconnected state
- Validation
- Tool result normalization
- Structured vs text content
- Images/files returned by tools
- Large responses
- Metadata preservation

## Phase 3: REST API additions

Propose API endpoints.

Possible endpoints:

```text
GET    /v1/mcp/servers
POST   /v1/mcp/servers
GET    /v1/mcp/servers/{id}
PATCH  /v1/mcp/servers/{id}
DELETE /v1/mcp/servers/{id}

POST   /v1/mcp/servers/{id}/test
POST   /v1/mcp/servers/{id}/start
POST   /v1/mcp/servers/{id}/stop
POST   /v1/mcp/servers/{id}/restart
POST   /v1/mcp/servers/{id}/refresh

GET    /v1/mcp/servers/{id}/tools
PATCH  /v1/mcp/servers/{id}/tools/{toolName}
PATCH  /v1/mcp/servers/{id}/tools/{toolName}/permission

GET    /v1/mcp/servers/{id}/resources
GET    /v1/mcp/servers/{id}/prompts
GET    /v1/mcp/audit
```

The plan should decide which are MVP and which are later.

Include:

- Request/response models
- Auth requirements
- Admin role requirements
- Error response format
- Rate limiting, if relevant
- Server-side validation

## Phase 4: Frontend UI

Design the React UI changes.

Likely additions:

```text
frontend/src/components/MCPManager.tsx
frontend/src/components/MCPServerForm.tsx
frontend/src/components/MCPToolList.tsx
frontend/src/components/MCPStatusBadge.tsx
frontend/src/components/MCPConfigImport.tsx
```

But inspect existing frontend structure first and match existing conventions.

Cover:

- Where the UI should live
- Required TypeScript interfaces
- API client additions
- Form validation
- Secret input handling
- Tool list display
- Tool permission controls
- Connection status display
- Last error display
- Import/export config UX
- User feedback/toasts
- Loading states
- Empty states

## Phase 5: Permission and approval improvements

The current permission system may support `allow`, `deny`, and `ask`, but the current execution path may not fully support live user approval for all tool calls.

Review and document what exists.

Plan improvements:

- Implement real `ask` behavior
- Approval event model
- Frontend approval prompt
- Works in chat
- Works in Agent Mode
- Approval timeout
- Persisted audit record
- Risk classification
- Read-only vs write/destructive detection
- Default policies for MCP tools

Recommended defaults:

```text
Native safe tools: allow
MCP read-only tools: ask first or allow depending on configuration
MCP write/destructive tools: ask every time
Unknown MCP tools: ask every time
Remote MCP servers: ask every time by default
```

## Phase 6: MCP resources

Defer this until tools are working, but include a plan.

Cover:

- `resources/list`
- `resources/read`
- Resource browser UI
- Attach resource to conversation
- Add resource to RAG
- Context budget management
- Resource URI display
- Resource refresh
- Resource permissions

## Phase 7: MCP prompts

Defer this until tools are working.

Cover:

- `prompts/list`
- `prompts/get`
- Import into Prompt Templates
- Use directly from prompt picker
- Variable mapping

## Phase 8: Remote HTTP and OAuth

Defer if too large for MVP.

Cover:

- Streamable HTTP transport
- Bearer token support
- OAuth flow if feasible
- Token storage
- Refresh tokens
- TLS requirements
- CORS implications
- Desktop vs web mode considerations

## Phase 9: OmniLLM-Studio as an MCP server

Later phase.

Plan how OmniLLM-Studio could expose its own tools:

- `omnillm.web_search`
- `omnillm.url_fetch`
- `omnillm.sports_lookup`
- `omnillm.generate_docx`
- `omnillm.generate_xlsx`
- `omnillm.generate_pdf`
- `omnillm.search_conversations`
- `omnillm.query_rag`
- `omnillm.generate_image`

Cover:

- Whether to expose through stdio, HTTP, or both
- Auth model
- Tool schemas
- Privacy concerns
- Which features should not be exposed
- Whether this should run only locally

---

# Required section 7: Security and trust model

This is critical.

Write a thorough security model.

Include:

## Threats

- Data exfiltration from local files
- Prompt injection through MCP resources
- Tool poisoning through misleading tool descriptions
- Destructive tool calls
- Credential leakage
- Remote MCP server compromise
- Local subprocess compromise
- Malicious server logs
- Path traversal
- Environment variable leakage
- Cross-user data leakage
- Workspace boundary bypass
- Long-running or hanging servers
- Denial of service through large outputs
- Untrusted binary execution
- Untrusted `npx` packages or remote package managers

## Controls

- Admin-only MCP server configuration by default
- Explicit enablement
- Per-tool permissions
- Approval for sensitive tools
- Secrets encrypted at rest
- Secrets never returned to frontend
- Environment redaction in logs
- Server command allowlist or warnings
- Path restrictions for filesystem servers
- Timeouts
- Output size limits
- Audit logging
- Tool result sanitization
- Workspace/user scoping
- Secure defaults
- Clear UI warnings

## Open questions

Ask Copilot to identify any unresolved security issues after code review.

---

# Required section 8: Configuration examples

Include example user-facing configuration formats.

## Stdio example

```json
{
  "name": "filesystem",
  "transport": "stdio",
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-filesystem",
    "/Users/adam/projects"
  ],
  "env": {},
  "enabled": true
}
```

## Stdio with environment variables

```json
{
  "name": "github",
  "transport": "stdio",
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-github"
  ],
  "env": {
    "GITHUB_PERSONAL_ACCESS_TOKEN": "{{secret:github_pat}}"
  },
  "enabled": true
}
```

## HTTP example

```json
{
  "name": "remote-tools",
  "transport": "http",
  "url": "https://example.com/mcp",
  "auth": {
    "type": "bearer",
    "token": "{{secret:remote_tools_token}}"
  },
  "enabled": true
}
```

The plan should decide the actual persisted format and API format.

---

# Required section 9: Tool naming convention

Propose a naming convention.

Preferred:

```text
mcp.<server_slug>.<tool_name>
```

Examples:

```text
mcp.filesystem.read_file
mcp.github.create_issue
mcp.postgres.query
```

Cover:

- Slug normalization
- Collision handling
- Maximum length
- Display name vs internal name
- How names are shown to the LLM
- How names are shown to the user
- Whether dots are safe for current tool handling
- Whether provider APIs support those names if tools are passed to model APIs directly
- Fallback naming if dots cause issues

Important: inspect current LLM provider tool-call integration before finalizing. Some model APIs impose naming constraints.

---

# Required section 10: Tool result normalization

MCP tools can return different content types.

Plan how to handle:

- Text content
- JSON/structured content
- Image content
- Embedded resources
- File-like outputs
- Binary outputs
- Errors
- Progress updates
- Large output truncation
- Metadata

Define how this maps to the existing `ToolResult` model:

```go
type ToolResult struct {
    ToolCallID string
    Content    string
    IsError    bool
    Metadata   map[string]interface{}
}
```

If the current model is insufficient, propose a backward-compatible extension.

---

# Required section 11: Logging, audit, and observability

Plan:

- Server start/stop logs
- Tool discovery logs
- Tool execution logs
- Tool errors
- Redaction rules
- Audit table
- Admin UI for recent MCP activity
- Debug view for server stderr
- Correlation IDs
- Conversation/agent run correlation
- User ID/workspace ID correlation
- Privacy controls

Do not log secrets.

---

# Required section 12: Testing strategy

Define test coverage.

Include:

## Unit tests

- Config validation
- Server slug normalization
- Tool name mapping
- Tool adapter result conversion
- Permission behavior
- Secret redaction
- Schema conversion
- Error handling

## Integration tests

- Mock MCP server over stdio
- Tool discovery
- Tool call execution
- Server crash/restart
- Timeout behavior
- Tool list refresh
- Permission denial
- Approval-required behavior

## Frontend tests/manual tests

- Add server
- Test connection
- View tools
- Enable/disable tools
- Execute MCP tool from chat
- Execute MCP tool from Agent Mode
- Error handling
- Secret redaction
- Multi-user access

## Regression tests

- Existing built-in tools still work
- Agent Mode still works with native tools
- Plugin SDK still works
- Settings UI remains functional
- Desktop/Wails build still works

---

# Required section 13: Backward compatibility

Address:

- Existing conversations
- Existing tools
- Existing plugins
- Existing settings
- Existing feature flags
- Existing desktop builds
- Existing Docker/Helm deployments
- Users without MCP configured
- Running with MCP disabled
- Migration rollback concerns

MCP should be optional and disabled until configured.

---

# Required section 14: Build and dependency considerations

Review `backend/go.mod`.

Determine:

- Whether to use official MCP Go SDK
- Whether the SDK supports required transports
- Whether Go version compatibility is acceptable
- Whether any transitive dependencies are problematic
- Whether Wails desktop builds are impacted
- Whether Linux/macOS/Windows stdio behavior differs
- Whether Docker image needs updates
- Whether Helm chart needs optional MCP settings/env

Include proposed dependency changes.

---

# Required section 15: Deployment considerations

Cover:

## Local web mode

- Server process launches MCP subprocesses
- Uses local filesystem
- Stores config/secrets locally

## Desktop/Wails mode

- Same as local mode, but app bundle/path issues may differ
- Need to consider working directory and executable discovery
- User experience for missing `npx`, `node`, `uv`, or other runtimes

## Docker mode

- Subprocess MCP servers may not exist in container
- Filesystem access limited to container
- Need volume mounts
- External tools may require image customization

## Kubernetes/Helm mode

- Running arbitrary subprocesses inside backend pod may be undesirable
- Better to use remote HTTP MCP servers or sidecar pattern
- Need security context considerations
- Need network policy considerations

---

# Required section 16: Open questions

Include a list of open questions discovered during code review.

Examples:

- Should MCP servers be global or per-user?
- Should non-admin users be able to add personal MCP servers?
- Should tools be available to all workspaces or scoped?
- Should server config support `npx`, `uvx`, Docker, or only direct executables?
- Should MCP be disabled in hosted/multi-user mode by default?
- How should approval work for normal chat tool calls?
- Should MCP resources be indexed into RAG automatically or only attached manually?
- How should remote OAuth be handled in desktop vs web mode?
- How much of MCP spec should be implemented in MVP?

Add codebase-specific open questions after review.

---

# Required section 17: Final phased roadmap

Create a crisp roadmap with phases, estimated complexity, and dependencies.

Use a table like:

```markdown
| Phase | Name | Scope | Dependencies | Risk | Deliverable |
|---|---|---|---|---|---|
| 0 | Review and plan | Codebase analysis and design doc | None | Low | `MCP_IMPLEMENTATION_PLAN.md` |
| 1 | Client foundation | Stdio client, config, lifecycle | SDK decision | Medium | Backend MCP manager |
| 2 | Tool integration | MCP tools in registry/executor | Phase 1 | Medium | MCP tools usable by backend |
| 3 | API + UI | Settings UI and REST APIs | Phase 2 | Medium | User-configurable servers |
| 4 | Approval/security | Real ask flow and audit | Phase 3 | High | Safe tool execution |
| 5 | Resources/prompts | MCP resources and prompt templates | Phase 4 | Medium | Context expansion |
| 6 | Remote HTTP/OAuth | Remote MCP servers | Phase 4 | High | Remote server support |
| 7 | MCP server mode | Expose OmniLLM tools externally | Phase 2+ | Medium | OmniLLM as MCP server |
```

Adjust based on the actual review.

---

# Required section 18: Copilot implementation instructions for future phases

At the end of the plan, include instructions for how future Copilot prompts should proceed.

Example:

```markdown
## Future implementation guidance

When implementing this plan:

1. Work one phase at a time.
2. Do not mix backend, frontend, and DB work in uncontrolled large commits.
3. Add migrations before repository logic.
4. Add backend models before handlers.
5. Add API client types before UI.
6. Preserve existing native tools and plugin behavior.
7. Keep MCP optional.
8. Add tests for each phase.
9. Do not log secrets.
10. Update README only after the feature is functional.
```

---

## Important implementation constraints

Follow these constraints while creating the plan:

- Do not implement MCP yet.
- Do not modify existing source code except to add the Markdown plan file.
- Do not remove or rewrite existing tool, agent, plugin, provider, or RAG code.
- Do not assume README statements are fully accurate; verify in code.
- Do not introduce a new tool execution path if the existing registry/executor can be reused.
- Prefer incremental changes over large rewrites.
- Keep MCP optional and disabled unless configured.
- Maintain local-first behavior.
- Maintain desktop/Wails compatibility.
- Maintain web server compatibility.
- Maintain Docker/Helm compatibility where practical.
- Do not expose secrets to the frontend.
- Do not log secrets.
- Avoid destructive tool execution without approval.
- Preserve existing API behavior.
- Preserve existing database compatibility.
- Use idiomatic Go.
- Use existing frontend style and component conventions.
- Use existing error response conventions.
- Use existing repository/migration conventions.
- Use existing auth/role conventions.

---

## Specific questions Copilot must answer in the plan

After reviewing the codebase, answer these explicitly:

1. Where should MCP server configuration live?
2. Should MCP server configuration be global, per-user, per-workspace, or hybrid?
3. Can the existing `tools.Tool` interface cleanly wrap MCP tools?
4. Does the current tool name format support names like `mcp.github.create_issue`?
5. Does any LLM provider integration impose tool name restrictions?
6. How should MCP tool schemas be converted or preserved?
7. Does current Agent Mode need any changes to consume dynamic MCP tools?
8. Does normal chat currently support provider-native tool calling, or only local preflight/tool enrichment?
9. How should `ask` permission be implemented for regular chat?
10. How should `ask` permission be implemented for Agent Mode?
11. Can the existing plugin subprocess runtime be reused for stdio MCP, or should MCP use a separate implementation?
12. How should MCP server stderr/stdout be handled?
13. How should server process cleanup work on app shutdown?
14. How should MCP work in Wails desktop mode?
15. How should MCP work in Docker and Helm deployments?
16. What is the minimal schema required for MVP?
17. What should be audited?
18. Which parts require tests before implementation?
19. Which areas are highest risk?
20. What can safely be deferred?

---

## Suggested final document style

The output file should be written as a serious engineering plan.

Use:

- Clear headings
- Tables where helpful
- File path references
- Proposed structs where useful
- Proposed API schemas where useful
- Proposed database schema where useful
- Explicit MVP vs later distinctions
- Security notes
- Testing notes
- Open questions
- Final recommended next step

Avoid:

- Vague recommendations
- Marketing language
- Hand-wavy “just add MCP”
- Large speculative rewrites
- Unverified assumptions
- Starting implementation before planning

---

## Expected final answer from Copilot

After completing the codebase review and creating the plan file, report back with:

```text
Created docs/internal_docs/MCP_IMPLEMENTATION_PLAN.md

Summary:
- [brief summary of recommendation]
- [key implementation phases]
- [main risks]
- [open questions]
```

Do not paste the entire document into chat unless requested.
