# OmniLLM-Studio Master Plan

> **Authoritative source for outstanding engineering work.**
>
> Audited against repository `main` at `113a2b9` on 2026-08-14. Completed initiatives and superseded plans are in [archive/README.md](archive/README.md). This document deliberately excludes completed work except where it is needed to explain a dependency. macOS sandbox Phase 13, previously the top open item, completed in PR #166 (`d52ab16`).

## Execution order

1. Close the browser request perimeter: complete and validate request-time enforcement for every browser-controlled network path.
2. Address sandbox correctness/security gaps: path-race assurance, resource quotas, egress enforcement, and credential delivery.
3. Resolve remaining platform-wide data/runtime safety debt, beginning with SQLite foreign-key admission and cookie-session migration validation.
4. Add durable sandbox-backed task/worker infrastructure; it depends on the sandbox hardening work above.
5. Finish router, video, and URL-context correctness/validation work.
6. Charter one narrow GitHub G7D capability only after higher-risk correctness work is stable.
7. Reconsider explicitly deferred enhancements only when product demand justifies them.

## P1 — sandbox hardening and deployment

### Enforced quotas, egress, TOCTOU assurance, and credential consumers

- **Status:** IN PROGRESS
- **Implemented:** Broker ownership/grants, no-network first-party Linux/Windows/Darwin runtimes, bounded wall/output limits, safe staged read-only workspace copies, opaque credential handles, and journaled workspace mutation tools.
- **Remaining:** Enforce and natively test memory, CPU, PID/process-count, and physical-disk quotas; implement destination-scoped egress resistant to DNS/proxy/redirect/private-address bypass; extend workspace registry/path-component TOCTOU tests beyond the staging flows; add service-specific credential-broker consumers.
- **Files:** `backend/internal/sandbox/`, `backend/cmd/sandboxd/`, `backend/internal/tools/`, `backend/internal/repository/scoped_tool_permission.go`.
- **Dependency/risk:** Do not advertise a capability before platform enforcement and negative evidence exist. Egress requires a separately enforceable runtime boundary, not just a Broker grant.
- **Next action:** Write a platform-by-platform quota/egress design with capability reporting and adversarial test criteria; take one independently verifiable control at a time.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`, `SANDBOX_RUNTIME_CURRENT_2026-08.md`, `AGENT_SANDBOX_THREAT_MODEL.md`.

### Durable sandbox tasks and isolated workers

- **Status:** NOT STARTED
- **Implemented:** Synchronous sandbox execution has owner-bound sessions and explicit cancellation; the general Agent runtime has its own persisted runs/scheduler.
- **Remaining:** Persist sandbox/task association and recovery semantics; add sandbox admission/scheduling/checkpoints/artifact recovery; deploy dedicated server/Kubernetes workers with hardened identity/quotas/network policy; provide isolated multi-agent worktrees plus reviewed promotion/reconciliation.
- **Files:** `backend/internal/sandbox/`, `backend/internal/agent/`, `backend/internal/tasks/`, deployment/Helm configuration.
- **Dependency/risk:** Requires the prior hardening work; do not run arbitrary tenant execution in the primary API process/container.
- **Next action:** Define a durable sandbox-task contract that explicitly distinguishes existing Agent scheduling from sandbox execution recovery, then design the worker boundary.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`.

## P1 — correctness and security debt

### Browser request perimeter

- **Status:** IN PROGRESS
- **Implemented:** The headless browser uses incognito contexts, managed profiles, feature gating, navigation checks, and browser tools. Request-time CDP interception now validates page/frame/subresource requests and aborts requests that fail the public HTTP(S) policy; page traffic bypasses service-worker response handling. A browser-backed fetch/subresource regression fixture is included.
- **Remaining:** Run the new browser-backed fixture on a host/CI runner where Chromium starts successfully, then add redirect, iframe, media, and WebSocket cases; prove or close interception coverage for worker/service-worker initiated traffic; address the DNS validation-to-connect race (for example with a controlled egress proxy or address-pinned transport). Do not describe the perimeter as complete until those paths are proven.
- **Files:** `backend/internal/browser/manager.go`, `backend/internal/browser/manager_test.go`, `backend/internal/browser/session.go`, `backend/internal/api/browser_fallback_navigator.go`.
- **Dependency/risk:** CDP validation blocks known private/internal destinations before dispatch but does not pin Chromium's DNS result to the address validated by Go. Worker targets may have separate CDP sessions.
- **Next action:** Add adversarial Chromium fixtures and close worker/WebSocket/DNS-rebinding coverage before enabling the browser in exposed deployments.
- **Derived from:** archived `REMEDIATION_STATUS_2026-07-18.md`, archived `Plan-headless-Chrome-tool.md`.

### SQLite foreign-key admission

- **Status:** NOT STARTED
- **Implemented:** WAL/busy-timeout and related SQLite pragmas are configured; foreign keys are deliberately disabled (`backend/internal/db/db.go`).
- **Remaining:** Audit delete paths, detect/repair existing orphan rows, add compatibility migration/tests, then enable foreign-key enforcement safely.
- **Files:** `backend/internal/db/db.go`, repositories and migrations under `backend/internal/repository/` and `backend/internal/db/`.
- **Dependency/risk:** Enabling this globally without repairing data can break existing installations.
- **Next action:** Build a read-only orphan audit/report first, agree migration behavior, then gate the pragma change on a clean repair path.
- **Derived from:** archived `REMEDIATION_STATUS_2026-07-18.md`, archived `copilot_video_editor_rve_true_up_prompt.md`.

### Cookie-session migration closeout

- **Status:** NOT STARTED
- **Implemented:** HttpOnly cookie sessions are supported; bearer authentication remains supported for API compatibility.
- **Remaining:** Exercise cookie login across Vite, Wails, reverse-proxy, and direct API modes, then remove the frontend `omnillm_auth_token` localStorage compatibility path if compatibility requirements permit.
- **Files:** `frontend/src/api.ts`, `frontend/src/App.tsx`, `backend/internal/auth/auth.go`.
- **Dependency/risk:** This is a compatibility migration; API clients may still need bearer tokens.
- **Next action:** Define supported client modes and add integration coverage before removing the browser fallback.
- **Derived from:** archived `REMEDIATION_STATUS_2026-07-18.md`.

## P2 — feature reliability and validation

### Router-model reliability completion

- **Status:** IN PROGRESS
- **Implemented:** Settings, structured request modes, sports-only routing, validation, deterministic fallback, provider-aware suggestions, telemetry, and focused router tests.
- **Remaining:** Implement or remove the UI-exposed router cache (currently labeled reserved for future use); add automatic structured-output fallback across schema/object/prompted JSON modes; add API-level fallback/settings/suggestion tests and authenticated end-to-end/provider smoke coverage. Comparison mode and non-sports tool routing remain separate product decisions.
- **Files:** `backend/internal/router/`, `backend/internal/api/message_handler.go`, `backend/internal/models/models.go`, `frontend/src/components/SettingsPanel.tsx`.
- **Dependency/risk:** Do not broaden routing beyond sports until failure semantics and provider compatibility are proven.
- **Next action:** First remove or implement the cache setting, with an explicit TTL/key/scope contract and tests; then add the structured-output fallback matrix.
- **Derived from:** archived `omnillm-router-model-copilot-implementation-prompt.md`.

### Video export fidelity and operational validation

- **Status:** NEEDS VALIDATION
- **Implemented:** FFmpeg rendering, capability reporting, provider-backed transcription, Windows capture, media probes/artifacts, interval indexing, clip virtualization, decoder budgeting, poster substitution, and patch-based undo/redo.
- **Remaining:** Add representative large-project browser frame-time budgets plus React commit/memory artifacts in CI. Preserve and prioritize conservative renderer gaps: true two-clip crossfades, rounded/geometric annotations, drop shadow/background blur, continuous-curve fidelity, track-solo export, and click-audio synthesis. Validate real configured video providers separately; capability metadata must remain conservative.
- **Files:** `backend/internal/video/renderer.go`, `backend/internal/video/renderer_capabilities.go`, `frontend/src/components/video/`, `tests/video-editor-*.spec.ts`.
- **Dependency/risk:** Renderer capabilities are a model/UI contract. Do not mark a feature supported from preview behavior alone; golden-media coverage is required.
- **Next action:** Add a large-project performance fixture and CI budget first, then select one renderer-capability gap with golden-media tests.
- **Derived from:** `VIDEO_RENDERER_RELIABILITY_TRANSCRIPTION_SCALABILITY_2026-07-20.md`, archived `VIDEO_EDIT_STUDIO_COMPLETION_2026-07-20.md`.

### URL-context quality gaps

- **Status:** NOT STARTED
- **Implemented:** URL fetch caching/ETag handling, untrusted context packaging, large-source RAG ingest, and a feature-gated headless-browser fallback tool.
- **Remaining:** PDF text extraction for URL-sourced PDFs, persistent cache storage, workspace-persistent URL collections, and optional explicit URL-to-RAG/search tools.
- **Files:** `backend/internal/urlcontext/`, `backend/internal/document/`, `backend/internal/tools/url_context_tools.go`.
- **Dependency/risk:** Current URL-PDF behavior intentionally returns guidance rather than pretending to extract text; any persistence must preserve user/workspace scoping.
- **Next action:** Decide whether URL-PDF extraction has product priority; if so, reuse the shared document parser behind bounded download and SSRF controls.
- **Derived from:** archived `url_context_followups.md`.

### GitHub collaboration G7D charter

- **Status:** NOT STARTED
- **Implemented:** G7A bounded failing-check diagnostics, G7B workflow/job/step status metadata, and the G7C decision to defer raw logs.
- **Remaining:** Choose and approve one narrow G7D capability family (for example issues/discussions metadata, remote-branch lifecycle, or release/tag workflow) with its own authorization, exact-object binding, bounded untrusted output model, and adversarial tests.
- **Files:** `backend/internal/gitrepo/github_pull_request_*.go`, `backend/internal/tools/github_pull_request_*.go`, `docs/GITHUB_INTEGRATION_PHASE7_2026-08.md`.
- **Dependency/risk:** Raw workflow logs remain deferred; PR-read authority must not be reused for a broader content/mutation surface.
- **Next action:** Collect actual diagnostic demand for G7A/G7B, then write a single-family G7D threat model before implementation.
- **Derived from:** `GITHUB_INTEGRATION_PHASE7_2026-08.md`, `GITHUB_INTEGRATION_PHASE7C_LOG_THREAT_REVIEW_2026-08.md`.

## Deferred by design

Raw GitHub Actions logs, broad new GitHub mutation families, MCP resources/prompts/sampling/elicitation/server mode, and RAG HNSW/HTTP-vector deployment are not active commitments. They require a separately approved design rather than being inferred from historical research or library components. Manual interactive approval for `POST /v1/tools/execute` is likewise deferred because it has no interactive caller contract.
