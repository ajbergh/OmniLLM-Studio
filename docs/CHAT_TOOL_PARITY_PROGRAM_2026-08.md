# Chat Studio Tool and Agent Parity Program — August 2026

This document records the implementation program that brought OmniLLM-Studio's Chat Studio tool, agent, and extensibility experience toward functional parity with mature tool-calling chat runtimes while preserving OmniLLM-Studio's local-first architecture and creative-studio differentiation.

> **Status:** Phases 3–8 are implemented on `main`. The phase sections below are retained as an implementation record and now include current-state anchors. New work should be driven by verified runtime gaps rather than by the historical phase numbering.

## Invariants

1. A tool that is **Off** is neither advertised nor executed by Chat Studio or Agent Mode.
2. A tool that is **Ask** remains discoverable but cannot execute without approval.
3. Per-chat and per-turn controls may narrow access but never widen a hard Settings deny.
4. Deterministic routing may select a capability, but it must use the same policy and execution boundary as model-selected tools.
5. Capability availability, policy, provider support, credentials, and runtime health are separate states.
6. Large tool catalogs should be discovered progressively instead of injecting every schema into every turn.

## Completed phases

### Phase 3 — Unified capability gateway — COMPLETE

Implemented outcomes:

- Deterministic Chat Studio shortcuts use the same effective capability policy as model-selected tools.
- `allow` may execute a deterministic preflight; `ask` intentionally falls through to the approval-aware tool loop; `deny` fails closed.
- Denied, disabled, or unknown capabilities are suppressed from model-facing discovery.
- Request-scoped tool selection continues to narrow deterministic shortcuts.

Primary implementation anchors:

- `backend/internal/api/chat_capability_policy.go`
- `backend/internal/api/chat_tool_runtime.go`
- `backend/internal/api/chat_turn_tools.go`
- `backend/internal/tools/executor.go`

### Phase 4 — Per-turn controls and tri-state UX — COMPLETE

Implemented outcomes:

- Chat turns support `tool_mode`, `allowed_tools`, and `required_tool` restrictions.
- Per-turn restrictions are intersected with the effective tool policy and are also enforced at execution time.
- Chat Studio includes a composer tool picker.
- Settings exposes explicit Allow / Ask / Off policy controls rather than a binary tool toggle.

Primary implementation anchors:

- `backend/internal/api/chat_turn_tools.go`
- `backend/internal/api/message_handler.go`
- `frontend/src/components/ToolPicker.tsx`
- `frontend/src/components/ChatView.tsx`
- `frontend/src/components/SettingsPanel.tsx`
- `frontend/src/types.ts`

### Phase 5 — Reusable agents and Skills — COMPLETE

Implemented outcomes:

- Assistant Profiles persist reusable provider/model, system instructions, permitted tools, workspace scope, and attached Skills.
- Profiles are available through backend APIs and frontend management UI and support portability workflows.
- Skills are reusable Markdown instruction packages.
- `skill_list` exposes compact metadata only; `skill_read` loads one selected Skill body on demand, preserving progressive disclosure instead of injecting all Skill text into every prompt.

Primary implementation anchors:

- `backend/internal/models/assistant_profile.go`
- `backend/internal/repository/assistant_profile.go`
- `backend/internal/repository/assistant_profile_portability.go`
- `backend/internal/api/assistant_profile_handler.go`
- `backend/internal/tools/skill_tools.go`
- `frontend/src/assistantProfilesApi.ts`
- `frontend/src/components/AssistantProfilesPanel.tsx`
- `docs/ASSISTANT_PROFILE_PORTABILITY_2026-08.md`

### Phase 6 — OpenAPI and MCP integration parity — COMPLETE

Implemented outcomes:

- OpenAPI 3.x servers can be persisted and surfaced as governed OmniLLM tools alongside MCP capabilities.
- OpenAPI operations are converted into bounded tool definitions with per-tool policy, ownership checks, safe target construction, response limits, and readiness/runtime management.
- The frontend includes OpenAPI server configuration and inspection UI.
- MCP remains integrated through the same registry/executor policy boundary, including the hardened remote authentication and ownership behavior documented elsewhere in the repository.

Primary implementation anchors:

- `backend/internal/openapi/manager.go`
- `backend/internal/models/openapi_server.go`
- `backend/internal/repository/openapi_server.go`
- `backend/internal/api/openapi_handler.go`
- `frontend/src/openApiServersApi.ts`
- `frontend/src/components/OpenAPIServersPanel.tsx`
- `docs/MCP_HOW_TO_FAQ.md`

### Phase 7 — Sandboxed code execution and programmatic orchestration — COMPLETE

Implemented outcomes:

- `code_execute` exists only when an external sandbox service is configured; arbitrary code is never run by the OmniLLM backend process.
- Code execution is bounded by language, input-size, result-size, session, and timeout contracts.
- `tool_batch` provides governed programmatic orchestration over authorized registered tools, with bounded child calls and no recursive batch execution.
- Child invocations re-enter the shared Executor so policy, approval, timeout, idempotency, audit, and request restrictions remain authoritative.

Primary implementation anchors:

- `backend/internal/codesandbox/`
- `backend/internal/tools/code_sandbox_tool.go`
- `backend/internal/tools/tool_batch.go`
- `backend/internal/api/chat_tool_runtime.go`

## Code sandbox runtime

Arbitrary code execution is never performed by the OmniLLM backend process. Configure an external sandbox implementing `POST /v1/execute` with:

```bash
OMNILLM_CODE_SANDBOX_URL=http://sandbox:8090
```

When the variable is absent, `code_execute` is not registered. The sandbox is responsible for process/container isolation, filesystem lifecycle, and network policy; network access should be disabled by default.

`tool_batch` is a high-risk orchestration tool. It can execute at most eight child calls, never recurses, and every child call re-enters the shared Executor so policy, approvals, idempotency, audit, and request-scoped restrictions remain authoritative.

### Phase 8 — Scoped permissions and deferred ToolSearch — COMPLETE

Implemented outcomes:

- Effective policy composes global → user → workspace → conversation → per-turn restrictions with monotonic tightening.
- Scoped permission lookup failures fail closed.
- Large catalogs use `tool_search` for compact request-scoped discovery and `tool_invoke` for deferred invocation rather than injecting every schema into each turn.
- Assistant Profile allowlists, scoped policy, per-turn restrictions, planning, parallelization barriers, and execution share the same policy boundary.
- Decision/audit context is persisted sufficiently for the runtime to explain and enforce scoped availability decisions.

Primary implementation anchors:

- `backend/internal/models/tool_scope.go`
- `backend/internal/repository/scoped_tool_permission.go`
- `backend/internal/api/scoped_tool_permission_handler.go`
- `backend/internal/tools/executor_scoped.go`
- `backend/internal/tools/tool_search.go`
- `frontend/src/scopedToolPermissionsApi.ts`
- `frontend/src/components/ScopedToolPermissionsPanel.tsx`

## Scoped policy and deferred discovery runtime

The effective permission chain is global → user → workspace → conversation → per-turn restriction. Persisted scoped policies are monotonic (`allow < ask < deny`): a lower scope may tighten access but never widen an inherited Ask or Deny. Database lookup errors fail closed.

Large catalogs use `tool_search` for compact request-scoped discovery and `tool_invoke` for generic invocation. Both honor the same scoped policy, Assistant Profile allowlist, and per-turn restrictions as directly advertised tools. Chat and Agent planning filter scoped-denied tools before model exposure, and parallel planning treats scoped Ask/Deny as sequential barriers.

## Post-program extensions already implemented

The original parity program did not include a full coding-agent Git workflow. Subsequent work added guarded capabilities for:

- local Git inspection and reviewed local mutations;
- guarded remote inspection, fetch, existing-branch push, and clone;
- separately gated creation/publication of a new remote feature branch;
- separately gated GitHub.com draft pull request creation bound to reviewed local and remote Git state.

See:

- `docs/LOCAL_GIT_TOOLS.md`
- `docs/REMOTE_GIT_TOOLS.md`
- `docs/GITHUB_PULL_REQUEST_TOOLS.md`

## Verified next gaps

The next program should be based on concrete missing runtime capabilities rather than extending the historical phase list.

### Priority 1 — Read-only GitHub PR and CI inspection

After `github_create_draft_pull_request`, Chat Studio currently has no dedicated governed GitHub collaboration read path for inspecting the resulting pull request and its checks. A coding agent can publish a branch and open a draft PR, but it cannot natively close the feedback loop by reading PR metadata/review state and commit/check status through the same operator-bound GitHub boundary.

Recommended first slice:

- read one PR by number in the repository derived from the selected operator-configured `github.com` remote;
- list the current branch's/open repository PRs with bounded pagination;
- inspect the PR head SHA and mergeability/state without accepting an arbitrary repository or API host;
- inspect check runs / commit status for the exact PR head;
- keep all operations read-only, low-risk, bounded, and independently gated by the configured remote/token;
- return model-safe errors without copying GitHub API error bodies or credentials;
- add focused fake-API tests and registry/tool-policy coverage.

Do **not** combine this slice with ready-for-review, reviewer requests, labels, closing, merge, source-branch deletion, workflow reruns, or other hosted mutations. Those should remain separate, approval-gated capabilities only if a later audit demonstrates the need.

### Priority 2 — Collaboration mutations, only after the read path is proven

Potential later slices include marking a draft ready, requesting reviewers, handling review-thread state, or merging a validated PR. Each mutation should have its own state-binding and approval boundary; none should be implied by draft-PR creation.

## Validation

Changes to this program's runtime should pass:

```bash
cd backend
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm run lint
npm run test:unit
npm run build

cd ..
npx playwright test --project=chromium
```

Focused tests should additionally cover policy intersection, approval behavior, disabled-tool discovery suppression, provider compatibility, request-scoped restrictions, and regression paths for deterministic shortcuts. Security-sensitive Git, GitHub, browser, plugin, auth, persistence, and deployment changes should also pass the repository Security Scan and applicable container/release validation before merge.
