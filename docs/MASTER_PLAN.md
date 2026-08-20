# OmniLLM-Studio Master Plan

> **Authoritative source for outstanding engineering work.**
>
> Audited against repository `main` through merged sandbox PR #210 (`dadb0372`) and CI hardening PR #226 (`ffbf7971`) on 2026-08-19, with rebuilt Windows governed-workspace hardening active in PR #231. Completed initiatives and superseded plans are in [archive/README.md](archive/README.md). This document deliberately excludes completed work except where it is needed to explain a dependency. macOS sandbox Phase 13 completed in PR #166 (`d52ab16f`). Native Chromium browser-egress assurance completed in PR #168 (`76f4f4c`). SQLite foreign-key admission is also complete (schema V51).

## Execution order

1. Finish cross-platform governed-workspace filesystem hardening by validating and merging rebuilt Windows PR #231. Linux and Darwin governed-workspace path-race hardening are already merged.
2. Rebuild the cumulative CPU-budget slice from closed draft #221 directly on current `main`; keep capability reporting fail-closed until native exact-head proof is green. Physical-disk accounting remains separate work.
3. Continue Phase 8/9 sandbox work: destination-enforced egress and service-specific credential consumers.
4. Rebuild durable sandbox task execution/lifecycle from closed drafts #215/#223, then rebuild isolated Kubernetes worker deployment (#216) and owner-scoped isolated worktrees (#217) as independent current-main PRs.
5. Resolve remaining platform-wide data/runtime safety debt, beginning with cookie-session migration validation.
6. Finish router, video, and URL-context correctness/validation work, then charter one narrow GitHub G7D capability only after higher-risk correctness work is stable.

## P1 — sandbox hardening and deployment

### Enforced quotas, egress, TOCTOU assurance, and credential consumers

- **Status:** IN PROGRESS
- **Implemented:** Broker ownership/grants, no-network first-party Linux/Windows/Darwin runtimes, bounded wall/output limits, safe staged read-only workspace copies, opaque credential handles, journaled workspace mutation tools, fail-closed Broker admission for unsupported non-zero memory/CPU/PID/disk quota requests (#170), Windows Job Object process-count enforcement (#171), Windows aggregate Job memory enforcement (#172), Linux cgroup-v2 `pids.max` enforcement (#173), Linux aggregate memory enforcement with `memory.max`, `memory.swap.max=0`, and `memory.events` evidence (#174), Linux descriptor-relative workspace content reads (#175), Linux descriptor-relative write/delete/revert transactions (#176), Linux registered-root device+inode binding with legacy fail-closed migration (#177), Linux descriptor-relative search enumeration with same-lineage candidate reads (#182), Darwin durable registered-root identity (#183), and Darwin descriptor-relative governed reads/search/mutations merged in #210 (`dadb0372`).
- **Current work:** PR #231 rebuilds the Windows-native governed-workspace identity/path-race slice directly on current `main`. It persists volume/file identity, rejects root replacement and legacy pathname-only grants, rejects reparse-point roots/candidates, verifies opened file handles before reads, and pins mutation parents for native relative rename/delete/restore operations.
- **Remaining:** Exact-head validation and merge of #231; rebuild and validate cumulative CPU enforcement from closed draft #221 before enabling `cpu_limit`; design enforceable physical-disk accounting; implement destination-scoped egress resistant to DNS/proxy/redirect/private-address bypass; add service-specific credential-broker consumers. macOS resource capabilities remain false until independently proven.
- **Files:** `backend/internal/sandbox/`, `backend/cmd/sandboxd/`, `.github/workflows/sandbox-linux-quota.yml`, `.github/workflows/sandbox-workspace-linux-assurance.yml`, `.github/workflows/sandbox-macos-runtime.yml`, `backend/internal/tools/`, `backend/internal/repository/scoped_tool_permission.go`.
- **Dependency/risk:** Capability reporting is runtime/platform-specific. Do not advertise a capability before native enforcement and negative evidence exist. Browser egress validation is not a substitute for arbitrary sandbox socket enforcement. Linux PID/memory capabilities are dynamic and require the relevant delegated cgroup-v2 controllers. Governed-workspace authorization must stay bound to opened filesystem objects or pinned directory handles, not mutable pathnames.
- **Next action:** Merge #231 only after its rebuilt exact head passes Windows-native governed-workspace tests plus repository Quality/Security gates. Then rebuild the CPU slice independently on the resulting `main`.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md`, `SANDBOX_RUNTIME_CURRENT_2026-08.md`, `SANDBOX_QUOTA_EGRESS_HARDENING_DESIGN_2026-08.md`, `AGENT_SANDBOX_THREAT_MODEL.md`.

### Durable sandbox tasks and isolated workers

- **Status:** IN PROGRESS / REBUILD REQUIRED
- **Implemented on `main`:** Synchronous sandbox execution has owner-bound sessions and explicit cancellation. Dedicated sandbox worker image packaging is merged in #214. The general Agent runtime has its own persisted runs/scheduler.
- **Reviewed but not merged:** Closed draft #215 implemented a durable sandbox-specific task queue with immutable create/exec specs, leases, attempt identity, runtime association, explicit retry policy, cleanup-before-replay recovery, Broker-only execution, lease renewal, and cancellation. Closed draft #223 wired that worker into server/Wails lifecycle. Closed draft #216 implemented an isolated Kubernetes worker workload and backend wiring. Closed draft #217 implemented owner-scoped isolated Git commit snapshots/worktrees with digest-bound review and guarded promotion. These drafts were closed because they were based on stale/stacked branch history and must be rebuilt on current `main` before merge.
- **Remaining:** Rebuild #215 first; then rebuild the #223 lifecycle integration against the merged durable-task contract; rebuild #216 and #217 as independent current-main slices; add any still-missing checkpoint/artifact recovery and admission/scheduling behavior identified during rebuild review.
- **Files:** `backend/internal/sandbox/`, `backend/internal/agent/`, `backend/internal/tasks/`, `backend/cmd/server/`, `backend/cmd/desktop/`, deployment/Helm configuration.
- **Dependency/risk:** Arbitrary tenant execution must not move into the primary API process/container. Restart recovery must never replay side effects while a predecessor runtime may still be alive. Worker identity, sandbox identity, lease ownership, and owner scope must remain distinct authorization boundaries.
- **Next action:** After #231 and the CPU slice are settled, reconstruct #215 directly on current `main`, validate it, merge it, and only then layer the lifecycle worker work from #223.
- **Derived from:** `AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md` and closed drafts #215/#216/#217/#223.

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

- **Status:** IMPLEMENTED / ONGOING PROVIDER VALIDATION
- **Implemented:** FFmpeg rendering and capability reporting; provider-backed transcription; Windows capture; media probes/artifacts; interval indexing, clip virtualization, decoder budgeting, and patch-based undo/redo; large-project CI evidence; semantic animation blocks; deterministic Bezier/spring curve expansion; persisted track-solo export; scenes, camera/parallax, and cinematic scene effects; and governed agent diagnostic render tools.
- **Remaining:** Keep conservative capability reporting for true two-clip crossfades, rounded/geometric annotations, drop shadow/background blur, click-audio synthesis, and partial 3D tilt fidelity. Validate configured video providers with real credentials in deployment-specific smoke runs. Phase 7 review/share remains deliberately optional and inactive.
- **Files:** `backend/internal/video/`, `backend/internal/tools/video_motion_tools.go`, `frontend/src/components/video/`, `tests/video-motion-design.smoke.spec.ts`.
- **Dependency/risk:** Renderer capabilities are a model/UI contract. Do not mark a feature supported from preview behavior alone; golden-media coverage is required.
- **Next action:** Revalidate provider integrations when credentials or upstream APIs change, and only promote the remaining partial renderer features with matching golden-media coverage.
- **Derived from:** `VIDEO_MOTION_DESIGN_ROADMAP_2026-08.md`, `VIDEO_RENDERER_RELIABILITY_TRANSCRIPTION_SCALABILITY_2026-07-20.md`, archived `VIDEO_EDIT_STUDIO_COMPLETION_2026-07-20.md`.

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
