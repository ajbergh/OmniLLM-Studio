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

The original parity program did not include a full coding-agent Git/GitHub workflow. Subsequent work now provides a governed loop from local changes through hosted review feedback:

- local Git inspection and reviewed local mutations;
- guarded remote inspection, fetch, existing-branch push, and clone;
- separately gated creation/publication of a new remote feature branch;
- separately gated GitHub.com draft pull request creation bound to reviewed local and remote Git state;
- bounded read-only pull-request metadata/listing and exact-head CI/check inspection;
- a provider-result trust boundary that treats hosted/tool text as untrusted reference data rather than instruction authority;
- bounded hosted review, inline-comment, timeline-comment, and review-request inspection;
- bounded cursor-based review-thread state/location inspection through fixed application-owned GraphQL;
- separately gated replies to reviewed top-level inline review comments;
- separately gated resolve/unresolve of an exact reviewed review thread with PR-head, ownership, state, and viewer-capability revalidation.

Primary implementation/documentation anchors:

- `docs/LOCAL_GIT_TOOLS.md`
- `docs/REMOTE_GIT_TOOLS.md`
- `docs/GITHUB_PULL_REQUEST_TOOLS.md`
- `docs/TOOL_RESULT_TRUST_BOUNDARY.md`
- `backend/internal/gitrepo/github_pull_request.go`
- `backend/internal/gitrepo/github_pull_request_read.go`
- `backend/internal/gitrepo/github_pull_request_feedback.go`
- `backend/internal/gitrepo/github_pull_request_threads.go`
- `backend/internal/gitrepo/github_pull_request_reply.go`
- `backend/internal/gitrepo/github_pull_request_thread_resolution.go`

## Verified next gaps

The next program remains driven by concrete missing runtime capabilities rather than by the historical phase list. The previous roadmap item for read-only PR/CI inspection is complete and must no longer be treated as pending work.

### Priority 1 — Guarded draft-to-ready transition

The hosted collaboration loop can now create a draft PR, inspect its exact head/checks/feedback/thread state, reply to reviewed inline feedback, and resolve addressed review threads. It still cannot explicitly advance a reviewed draft PR to **ready for review** through OmniLLM.

This is the recommended next implementation slice because it advances the PR lifecycle without changing Git objects, source refs, reviewer membership, workflow execution, or merge state. It should remain an independently gated hosted mutation rather than being implied by draft creation or PR-read permission.

Recommended guardrails:

- accept only the configured remote ID, positive PR number, and exact reviewed 40-character PR head;
- derive repository/API host/token only from the operator-configured `github.com` remote;
- require a dedicated process-wide gate and per-remote opt-in independent from PR read/create/reply/thread-resolution permissions;
- re-fetch the PR immediately before mutation and require it to remain open, unmerged, draft, on the exact reviewed head, and targeted at the configured repository's expected base state;
- use one fixed application-owned GitHub mutation; do not accept arbitrary GraphQL text or API URLs;
- validate the returned PR back to the same repository/number/head and require `draft=false` before reporting success;
- treat transport/provider/GraphQL ambiguity after mutation as an unknown outcome requiring fresh PR inspection before retrying;
- keep the tool high-risk, side-effecting, credentialed, non-parallel, approval-aware, and separately policy-controlled;
- do not combine this slice with reviewer requests, review submission/dismissal, workflow reruns, merge/close, labels/assignees, or source-branch deletion.

### Priority 2 — Validated merge lifecycle, only after ready-state transition is proven

A coding agent still cannot close the lifecycle by merging an exact validated PR. This is materially higher risk and less reversible than marking a draft ready, so it should be a later independently gated capability.

Before implementing merge, require a dedicated design/review for at least:

- exact current PR head and base binding with no arbitrary repository/base/ref inputs;
- open, non-draft, unmerged hosted state;
- current mergeability/merge-state validation;
- explicit check/status policy for the exact head rather than trusting stale prior output;
- review-thread/review-state policy appropriate to the repository workflow;
- one explicitly selected operator-approved merge method or a repository-policy-derived method, never a model-supplied arbitrary strategy;
- ambiguous-outcome handling that re-inspects the PR before any retry;
- no implicit source-branch deletion.

Do not infer merge authorization from `viewerCan*` provider flags, existing Git push permission, draft creation permission, or thread-resolution permission.

### Priority 3 — Lower-priority collaboration/operations gaps

These remain intentionally unsupported until a concrete workflow need justifies a separate capability and threat-model review:

- request/remove reviewers or teams;
- submit/dismiss reviews;
- rerun/cancel GitHub Actions workflows;
- arbitrary PR metadata changes such as labels, assignees, milestones, or base retargeting;
- close/reopen arbitrary PRs;
- delete hosted source branches.

Repository search and current remote configuration confirm there are no corresponding core OmniLLM tool registrations or operator gates on `main` as of this reconciliation. Their absence is intentional rather than a stale implementation omission.

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
