# OmniLLM-Studio 0.3 — Phased Delivery & Reliability Plan

**Created:** 2026-10-07  
**Baseline:** `main` at `f365da67448a0c3b58eba2727dc6faf3da89ca40`  
**Goal:** A reliable, secure local-first multi-model and creative-production release, not another unrestricted feature expansion.

This is the active execution tracker for Phases 1–3. Preserve subsystem engineering detail in the other active design documents, especially the [sandbox roadmap](AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md), [video WYSIWYG plan](VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md), and [RAG architecture](RAG_MODERNIZATION.md). The older [Master Plan](MASTER_PLAN.md) contains technical backlog context but its August 2026 status statements must be independently revalidated.

## Operating rules

1. Work from current `main` on isolated branches; keep changes reviewable and narrowly scoped. Prefer squash merges after exact-head checks.
2. Never weaken Security Scan, sandbox confinement, resource capability truthfulness, or renderer-fidelity gates merely to merge a feature.
3. Require executable tests and source-backed evidence for completion. A created PR, passing unit test, or plan checkbox alone does not prove a production workflow.
4. No unreviewed autonomous actions, credential disclosure, or sandbox networking expansion.
5. Mark blocked/unsupported functionality explicitly, with responsible next action.

## Phase 1 — Stabilize CI, secure dependencies, reconcile work (P0)

| Work item | Status as of 2026-10-07 | Completion evidence |
| --- | --- | --- |
| Align Go 1.26 toolchain across CI/release/parity, Go module and docs | **In review — [#320](https://github.com/ajbergh/OmniLLM-Studio/pull/320)** | Quality, Security, all applicable native sandbox and video parity checks green on exact head |
| Remediate npm audit advisories with reproducible lockfile | **In review — [#321](https://github.com/ajbergh/OmniLLM-Studio/pull/321)**, stacked on #320 | `npm ci`, frontend audit, lint, tests/build and complete CI green |
| Protect `main` with enforceable branch rules | **Owner/admin action — [#322](https://github.com/ajbergh/OmniLLM-Studio/issues/322)** | Failing-check PR cannot merge; up-to-date required checks and review policy verified |
| Finish rounded-rectangle canonical playback | **Open — [#314](https://github.com/ajbergh/OmniLLM-Studio/pull/314)** | Revalidate full triggered matrix after CI repairs, then squash merge; preserve v6/v7 evidence |
| Reconcile historical sandbox status and stale PRs | **Pending** | Refresh docs with verified #231–#235 merge history and review/close obsolete docs PR #236 |
| Document reproducible release baseline | **Pending** | README, technical reference, version/build matrix and platform tests agree |

**Current verified baseline:** Sandbox PRs #231 (Windows governed workspaces), #232 (CPU enforcement), #233 (durable task core), #234 (worker deployment workload), and #235 (isolated worktrees) are already merged. These are *prerequisites*, not proof that remaining Phase 14 task lifecycle composition, startup/shutdown recovery, enforced process egress, or hard disk quota support is complete. Avoid reimplementing merged work.

**Go/npm stacking rule:** #321 targets the #320 Go branch so the two fixes can receive one coherent full-security audit. Merge #321 into #320 first after its exact-head checks, then rerun/review #320 on the updated head before merging into `main`. Update/rebase Dependabot #319 (which currently bumps `backend/go.mod` to Go 1.26) only after baseline toolchains are consistent. Handle #315/#317/#318 independently.

**Phase 1 exit:** Security Scan and Quality Gate green against combined head; supported native sandbox/video checks green; no outstanding high/critical npm advisory; merge policy enforceable; source of truth current; video PR disposition explicit.

## Phase 2 — Runtime safety, correct behavior, and maintainability (P1)

### 2.1 Durable agent execution

- Validate application lifecycle composition for sandbox tasks using process-wide Broker and existing SQLite authority: startup recovery before claiming work, bounded shutdown/cancellation, crash recovery, and no unsafe replay of side effects.
- Prove worker isolation in server/Kubernetes deployment under non-root identities, strict security contexts and default-deny network policies. Retain `disk_limit=false` until physical quota enforcement is proven.
- Keep forced arbitrary-process destination-scoped egress and service-specific credential consumers in separately threat-modeled slices; a trusted broker HTTP client is not a replacement for socket-level isolation.
- Add adversarial native negative cases and exact-head assurance before promoting any capability bit.

### 2.2 Chat and model reliability

- Build a real authenticated HTTP/SSE integration harness for `backend/internal/api/message_handler.go`, retrieval preflight, provider routing, browser tool eligibility, failure recovery, and multi-round tool execution. Test both streaming and non-streaming paths and captured citations/cost.
- Add table-driven compatibility/fallback tests for structured model outputs and decide to implement or remove the currently UI-exposed reserved router cache.
- Validate provider model discovery, timeouts, retries and explicit failure reporting with controllable mocks, then run opt-in credentialed smoke tests.

### 2.3 Authentication, modularization and data durability

- Validate HttpOnly cookie sessions across Vite, Wails, direct API and reverse-proxy deployments. Only after compatibility and CSRF/session tests, retire browser `omnillm_auth_token` localStorage fallback while retaining deliberate API token support.
- Extract smaller bounded modules from `frontend/src/components/SettingsPanel.tsx`, `frontend/src/stores/videoStudio.ts`, `frontend/src/components/ChatView.tsx`, `backend/internal/api/message_handler.go`, and `backend/internal/llm/service.go`. Keep behavior-preserving tests and schema compatibility.
- Test persistent asset upgrades/rollbacks, SQLite migration, restart recovery, RAG index repair, encrypted-secret portability, and lossless project import/export.

### 2.4 Video WYSIWYG completion

- Continue the export-first canonical shape program after #314: ellipse, additional independently measured painters, whole-frame fail-closed readiness, and AudioGraph preview/export agreement.
- Do not declare universal preview/export pixel identity from a subset fixture; preserve fixture-specific envelopes and independent platform evidence.
- Delay shared Chromium render-worker or legacy retirement until current canonical consumers and migration telemetry are proven.

**Phase 2 exit:** 100% critical-path API handler integration suite, native sandbox adverse/recovery cases, authenticated migration tests, scoped refactor without behavior regressions, and measured supported preview/export fidelity.

## Phase 3 — End-to-end product readiness and 0.3 prerelease (P2)

1. **One project journey:** a user can research with private and public evidence, generate/revise an image, select or create music, assemble an editable video, export final media, and reopen the project without broken asset lineage. Include explicit provider availability and cloud-consent behavior.
2. **Guided setup:** local-only, hybrid, creator, developer, and team presets over the same secured backend; progressive disclosure for deep settings, persistent diagnostics, and clear offline/cloud indicators.
3. **Repeatable provider evaluation:** same prompt/dataset across models with citations, latency, token usage, pricing provenance, grounded quality tests, and deterministic export of comparison results.
4. **Desktop/release acceptance:** Windows/macOS Wails installer launch, signed/notarized packaging where provisioned, clean install, credentials, migrations, content storage, large project reopen, offline/local model path, updates and rollback. Linux/container paths should have explicit independent support matrices.
5. **Release:** produce a controlled `0.3.0-rc1` prerelease only after gating evidence; publish supported platform binaries, checksums, changelog, known limitations and migration guidance. Version bump/release is a separate PR with tested artifacts.

**Phase 3 exit:** full user-journey smoke with failure-state tests, reproducible release builds, upgrade/rollback evidence, no blocking known security issue, and published release notes.

## Review cadence and change bookkeeping

Each work PR must update the relevant scoped roadmap (if any) and record: baseline/head SHA, test matrix, exact findings, unsupported paths, user-visible behavior, and next independent slice. Maintain one highest-priority fix per PR; document accepted deferrals rather than presenting them as shipped.

## Immediate merge queue

1. Validate [#321](https://github.com/ajbergh/OmniLLM-Studio/pull/321) on the [#320](https://github.com/ajbergh/OmniLLM-Studio/pull/320) base; merge #321 into that branch after checks.
2. Validate combined #320 exact head, then merge it to `main`.
3. Rebase/revalidate [#314](https://github.com/ajbergh/OmniLLM-Studio/pull/314) against updated `main` and merge when green.
4. Reconcile dependency PRs #315/#317/#318/#319 individually and close or replace stale #236.
5. Complete owner-side [main-branch protection](https://github.com/ajbergh/OmniLLM-Studio/issues/322) before accepting feature expansion.
