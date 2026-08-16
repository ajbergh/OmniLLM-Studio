# OmniLLM-Studio Master Plan

> **Authoritative source for outstanding engineering work.**
>
> Audited against repository `main` through PR #175 (`925e2585`) plus the current PR #176 implementation branch on 2026-08-16. Completed initiatives and superseded plans are in [archive/README.md](archive/README.md). This document deliberately excludes completed work except where it is needed to explain a dependency. macOS sandbox Phase 13 completed in PR #166 (`d52ab16`). Native Chromium browser-egress assurance completed in PR #168 (`76f4f4c`). SQLite foreign-key admission is also complete (schema V51).

## Execution order

1. Continue sandbox correctness/security hardening: complete Linux workspace mutation path-race hardening in #176, then persist and enforce exact granted-root identity, address remaining search-enumeration/non-Linux path races, destination-enforced sandbox egress, and credential consumers. Resolve CPU semantics before enabling a CPU capability; physical-disk accounting remains separate work.
2. Resolve remaining platform-wide data/runtime safety debt, beginning with cookie-session migration validation.
3. Add durable sandbox-backed task/worker infrastructure; it depends on the sandbox hardening work above.
4. Finish router, video, and URL-context correctness/validation work.
5. Charter one narrow GitHub G7D capability only after higher-risk correctness work is stable.
6. Reconsider explicitly deferred enhancements only when product demand justifies them.

## P1 — sandbox hardening and deployment

### Enforced quotas, egress, TOCTOU assurance, and credential consumers

- **Status:** IN PROGRESS
- **Implemented:** Broker ownership/grants, no-network first-party Linux/Windows/Darwin runtimes, bounded wall/output limits, safe staged read-only workspace copies, opaque credential handles, journaled workspace mutation tools, fail-closed Broker admission for unsupported non-zero memory/CPU/PID/disk quota requests (#170), Windows Job Object process-count enforcement (#171), Windows aggregate Job memory enforcement (#172), merged Linux cgroup-v2 `pids.max` enforcement (#173), merged Linux aggregate memory enforcement with `memory.max`, `memory.swap.max=0`, and `memory.events` evidence (#174), merged Linux descriptor-relative workspace content reads (#175), and natively proven Linux descriptor-relative write/delete/revert transactions on the current #176 implementation head. #176 pins the workspace root and final parent directory by file descriptor for state capture, mutation, after-state verification, and rollback; replacement uses `renameat`, deletion uses `unlinkat`, and final-file capture uses `openat(O_NOFOLLOW)` plus `fstat`.
- **Remaining:** Complete #176 on its documentation-inclusive final head; persist and verify exact granted-root filesystem identity so replacement of the registered root path fails closed; address pathname-based search candidate enumeration and non-Linux workspace path-race cases; resolve aggregate/cumulative CPU-time semantics before enabling `cpu_limit`; design enforceable physical-disk accounting; implement destination-scoped egress resistant to DNS/proxy/redirect/private-address bypass; add service-specific credential-broker consumers. macOS resource capabilities remain false until independently proven.
- **Files:** `backend/internal/sandbox/`, `backend/cmd/sandboxd/`, `.github/workflows/sandbox-linux-quota.yml`, `.github/workflows/sandbox-workspace-linux-assurance.yml`, `backend/internal/tools/`, `backend/internal/repository/scoped_tool_permission.go`.
- **Dependency/risk:** Capability reporting is runtime/platform-specific. Do not advertise a capability before native enforcement and negative evidence exist. Browser egress validation is not a substitute for arbitrary sandbox socket enforcement. Egress requires a separately enforceable runtime boundary, not just a Broker grant. Linux PID/memory capabilities are dynamic and require the relevant delegated cgroup-v2 controllers. For a positive `resources.memory_bytes`, `memory.max` alone is insufficient for the project contract because cgroup swap is separately governed; #174 therefore sets `memory.swap.max=0`. #175 closes tested Linux content-read redirection through parent/final symlink swaps. #176 extends the same identity discipline through mutation capture, commit, verification, and rollback even when a parent pathname is renamed and replaced with an outside symlink. Neither slice persists the identity of the registered workspace root itself, makes `WalkDir` enumeration descriptor-relative, or changes non-Linux governed workspace path semantics.
- **Next action:** Merge #176 only after its documentation-inclusive exact head passes the focused Ubuntu workspace-operation assurance and repository-wide gates; then take exact registered-root identity persistence/replacement detection as the next separate reviewable slice before moving to broader remaining path-race work and destination-enforced egress.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`, `SANDBOX_RUNTIME_CURRENT_2026-08.md`, `SANDBOX_QUOTA_EGRESS_HARDENING_DESIGN_2026-08.md`, `AGENT_SANDBOX_THREAT_MODEL.md`.

### Durable sandbox tasks and isolated workers

- **Status:** NOT STARTED
- **Implemented:** Synchronous sandbox execution has owner-bound sessions and explicit cancellation; the general Agent runtime has its own persisted runs/scheduler.
- **Remaining:** Persist sandbox/task association and recovery semantics; add sandbox admission/scheduling/checkpoints/artifact recovery; deploy dedicated server/Kubernetes workers with hardened identity/quotas/network policy; provide isolated multi-agent worktrees plus reviewed promotion/reconciliation.
- **Files:** `backend/internal/sandbox/`, `backend/internal/agent/`, `backend/internal/tasks/`, deployment/Helm configuration.
- **Dependency/risk:** Requires the prior hardening work; do not run arbitrary tenant execution in the primary API process/container.
- **Next action:** Define a durable sandbox-task contract that explicitly distinguishes existing Agent scheduling from sandbox execution recovery, then design the worker boundary.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`.

## P1 — correctness and security debt

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
