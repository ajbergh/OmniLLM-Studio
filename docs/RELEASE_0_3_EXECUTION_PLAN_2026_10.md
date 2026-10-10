# OmniLLM-Studio 0.3 — Phased Delivery & Reliability Plan

**Created:** 2026-10-07  
**Last verified update:** 2026-10-08  
**Original baseline:** `main` at `f365da67448a0c3b58eba2727dc6faf3da89ca40`  
**Integration reference:** `main` at `f9f75e0afd0b2d98ba9c5ec00b0bd1ae1643fd5a` (before in-review Phase 2 tests)  
**Goal:** A reliable, secure local-first multi-model and creative-production release, not another unrestricted feature expansion.

This is the active execution tracker for Phases 1–3. Preserve subsystem engineering detail in the other active design documents, especially the [sandbox roadmap](AGENT_SANDBOX_ROADMAP_CURRENT_2026-08.md), [video WYSIWYG plan](VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md), and [RAG architecture](RAG_MODERNIZATION.md). The older [Master Plan](MASTER_PLAN.md) contains technical backlog context but its August 2026 status statements must be independently revalidated.

## Operating rules

1. Work from current `main` on isolated branches; keep changes reviewable and narrowly scoped. Prefer squash merges after exact-head checks.
2. Never weaken Security Scan, sandbox confinement, resource capability truthfulness, or renderer-fidelity gates merely to merge a feature.
3. Require executable tests and source-backed evidence for completion. A created PR, passing unit test, or plan checkbox alone does not prove a production workflow.
4. No unreviewed autonomous actions, credential disclosure, or sandbox networking expansion.
5. Mark blocked/unsupported functionality explicitly, with responsible next action.

## Phase 1 — Stabilize CI, secure dependencies, reconcile work (P0)

| Work item | Verified status as of 2026-10-08 | Remaining acceptance requirement |
| --- | --- | --- |
| Align Go 1.26 across CI, module and parity jobs | **Merged** — [#320](https://github.com/ajbergh/OmniLLM-Studio/pull/320), [#319](https://github.com/ajbergh/OmniLLM-Studio/pull/319) | Keep the CI/runtime toolchain matrix aligned; defer optional Go 1.27 Docker image bump [#256](https://github.com/ajbergh/OmniLLM-Studio/pull/256) until coordinated |
| Resolve npm dependency advisories | **Merged** — [#321](https://github.com/ajbergh/OmniLLM-Studio/pull/321), [#315](https://github.com/ajbergh/OmniLLM-Studio/pull/315), [#317](https://github.com/ajbergh/OmniLLM-Studio/pull/317), [#318](https://github.com/ajbergh/OmniLLM-Studio/pull/318) | Monitor future automated advisories and preserve lockfile integrity |
| Protect `main` with enforceable branch rules | **Blocked on repository owner/admin** — [#322](https://github.com/ajbergh/OmniLLM-Studio/issues/322) | Configure required up-to-date checks, review and merge restrictions; confirm a failing-check PR cannot merge |
| Finish rounded-rectangle canonical playback | **Merged** — [#314](https://github.com/ajbergh/OmniLLM-Studio/pull/314) | Maintain fixture-specific video parity envelopes; do not claim universal preview/export equivalence |
| Reconcile historical sandbox status | **Roadmap merged** — [#323](https://github.com/ajbergh/OmniLLM-Studio/pull/323) | Review/close superseded conflicted docs PR [#236](https://github.com/ajbergh/OmniLLM-Studio/pull/236) without discarding genuinely unique content |
| Document reproducible release baseline | **In progress** | Validate README/reference, artifact matrix, signed platform builds and upgrade/rollback evidence for 0.3 prerelease |

**Current verified baseline:** Sandbox PRs #231 (Windows governed workspaces), #232 (CPU enforcement), #233 (durable task core), #234 (worker deployment workload), and #235 (isolated worktrees) are already merged. These are *prerequisites*, not proof that remaining Phase 14 task lifecycle composition, startup/shutdown recovery, enforced process egress, or hard disk quota support is complete. Avoid reimplementing merged work.

**Phase 1 merge record:** #321 was integrated into the #320 CI/security branch and the combined baseline merged into `main`; #314, #315, #317, #318, #319 and #323 also merged after CI validation. The historical merge queue below is superseded. Phase 1 remains administratively **incomplete** until [#322](https://github.com/ajbergh/OmniLLM-Studio/issues/322) is enforced. Do not upgrade Docker-only images to Go 1.27 ahead of the supported Go 1.26 toolchain.

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

**2.2 progress (2026-10-10):** [#327](https://github.com/ajbergh/OmniLLM-Studio/pull/327) (authenticated sync/SSE lifecycle), [#329](https://github.com/ajbergh/OmniLLM-Studio/pull/329) (calculator provider→tool→provider loop), [#331](https://github.com/ajbergh/OmniLLM-Studio/pull/331) (sanitized provider error paths) and [#332](https://github.com/ajbergh/OmniLLM-Studio/pull/332) (client disconnect propagation) are **merged** with exact-head CI green. [#333](https://github.com/ajbergh/OmniLLM-Studio/pull/333) (malformed provider frames / interrupted SSE) and [#334](https://github.com/ajbergh/OmniLLM-Studio/pull/334) (bounded tool-round limits) are **merged** after exact-head CI verification. The remaining handler-level coverage includes retrieval preflight followed by a tool in one turn, nil-summarizer regression via HTTP, stale search evidence, deadlines and measured duration of the complete targeted suite. Track that closure under [#324](https://github.com/ajbergh/OmniLLM-Studio/issues/324).

### 2.3 Authentication, modularization and data durability

- **Implementation merged — [#328](https://github.com/ajbergh/OmniLLM-Studio/pull/328):** browser sessions now use HttpOnly cookies instead of persistent `omnillm_auth_token` localStorage. Wails keeps a memory-only bearer fallback and `/auth/status` is no longer charged against the credential-attempt limiter. Still validate Vite/Wails/reverse-proxy behavior, CSRF, logout/refresh, desktop cold-start and controlled direct API token access; implementation merge does **not** prove that entire matrix.
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

## Current integration and acceptance queue (2026-10-08)

1. **Merged #333 and #334:** malformed-provider and finite tool-round HTTP/SSE tests both passed their exact-head Quality, Security and applicable sandbox/browser checks and were squash merged on 2026-10-10. Keep composite-path coverage under #324.
2. Complete [#324](https://github.com/ajbergh/OmniLLM-Studio/issues/324) with a deterministic retrieval-preflight+tool HTTP/SSE case, a nil-summarizer guard regression that fails under intentional mutation, stale evidence / timeout cases and recorded test commands/durations.
3. Validate [#328](https://github.com/ajbergh/OmniLLM-Studio/pull/328) session behavior across desktop/browser/reverse proxy and CSRF before declaring authentication migration fully completed.
4. Finish durable sandbox task lifecycle and startup/shutdown recovery; keep unproven `disk_limit` and network-isolation capability bits disabled.
5. Owner/admin completes [#322](https://github.com/ajbergh/OmniLLM-Studio/issues/322): main branch protection. Reconcile [#236](https://github.com/ajbergh/OmniLLM-Studio/pull/236); assess the separately scoped Go 1.27 PR [#256](https://github.com/ajbergh/OmniLLM-Studio/pull/256).
6. Only then progress [#326](https://github.com/ajbergh/OmniLLM-Studio/issues/326) release-journey and supported-platform evidence toward a controlled 0.3 prerelease.

**Release acceptance:** See [0.3 acceptance gate](RELEASE_0_3_ACCEPTANCE.md) for release checksum verification, desktop/runtime matrix and the end-to-end project workflow. The release workflow hardening must pass CI independently before a release tag is considered.\n\n**Status interpretation:** “Merged” denotes an integrated code change with recorded CI, not completion of all product or deployment acceptance criteria. “In review” is not green until the exact head passes every required check.
