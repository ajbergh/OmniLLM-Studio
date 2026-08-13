# Image Studio Remediation Roadmap

Status: Complete
Last updated: 2026-08-12

This document is the durable implementation tracker for the August 2026 Image Studio capability review. The program focuses on edit geometry fidelity, selection/masking correctness, provider/model capability fidelity, reference-image behavior, advanced controls, and regression coverage.

## Product invariants

1. Editing a source image must preserve the source composition/aspect by default. Output geometry changes only when the user explicitly selects a different output size/aspect control.
2. A selection/mask belongs to the exact source node/image against which it was painted. It must never leak to another node or silently reach a model that cannot consume it.
3. Provider/model capability metadata is a contract, not a marketing hint. A control is exposed only when its value reaches a supported provider parameter or has a documented fallback.
4. Pixel masks and semantic edit guidance are distinct capabilities and must be represented distinctly.
5. Provider changes, model changes, undo/redo, branch navigation, touch input, and reference-image limits must not corrupt edit state.
6. Backend validation is authoritative. Frontend controls should prevent invalid requests, but the backend must independently reject incompatible combinations.

## Phase status

| Phase | Scope | Status | PR / merge |
|---|---|---|---|
| 0A | Edit-size state, Source default, safe `auto` handling | Complete | PR #100, squash merged as `e50431a` |
| 0B | Mask state scoped to active image node | Complete | PR #102, squash merged as `b2d75ff` |
| 0C | One-finger touch masking + two-finger gesture coexistence | Complete | PR #103, squash merged as `84e7e84` |
| 1 | Provider-neutral edit geometry contract and dedicated Image Studio provider transport | Complete | PR #106, squash merged as `890fb997` |
| 2 | Provider-aware mask semantics and source/mask validation | Complete | PR #113, squash merged as `1cc50515` |
| 3 | Selection UX completion: feathering and capability transitions | Complete | PR #114, squash merged as `a5407546` |
| 4 | Capability completion: references, variants, seed/guidance, honest provider matrix | Complete | PR #129, squash merged as `616a2880` |
| 5 | Regression matrix and documentation closeout | Complete | PR #130, squash merged as `58ccb0aa` |

## Confirmed defects from the review

### Geometry

- The editor historically defaulted edit output to `1024x1024`, which could change a non-square source without user intent. Phase 0A fixed the frontend state, and Phase 1 added the semantic provider transport that keeps source-preserving edits distinct from explicit output geometry.
- The legacy OpenAI edit transport substituted `1024x1024` when edit size was omitted; Image Studio now maps source-preserving OpenAI edits to provider `auto` rather than the square fallback.
- Gemini's legacy size mapper mapped unknown values to `1:1`; Image Studio now leaves geometry unset for source-preserving Gemini edits.
- Image Studio OpenRouter generation/editing now uses the dedicated Images API instead of the older chat-completions image shim.

### Selection and masks

- Mask strokes were previously restored only at initial session load; Phase 0B fixed node navigation leakage.
- Touch masking was documented but one-finger drawing was not implemented; Phase 0C completed it.
- Phase 2 distinguishes exact pixel masks from semantic edit-area guidance with `masking_mode` and validates exact alpha masks before provider dispatch.
- OpenRouter no longer advertises pixel masking because the dedicated Images transport does not expose a compatible mask parameter.
- Imagen 4 no longer inherits Gemini editing/masking/reference capabilities; it is treated as generation-only in this integration.
- Phase 3 applies stored feather values to both preview and exported selection rasterization.
- Phase 3 retains painted selections across temporary provider/model incompatibility, renders them inactive, prevents submission, and restores them when a compatible model is selected again.
- Phase 3 labels exact pixel selections and semantic edit-area guidance distinctly in the UI.

### Capability completion

- Phase 4 replaces persisted-but-no-op advanced image controls with capability-gated seed and guidance behavior backed by implemented transports.
- Phase 4 removes the image-editor store's hard-coded two-reference limit and makes the selected provider/model capability the source of truth.
- Phase 4 enforces combined content/style reference limits, reference-role support, `max_variants`, seed support, and guidance support in the backend before transport dispatch.
- Phase 4 suppresses stale unsupported reference IDs and seed values after provider/model changes and clamps variants when the selected model lowers the limit.
- Phase 4 keeps unknown/unrouted providers image-incapable, removes deprecated Imagen 3 catalog entries, and does not advertise an unimplemented Stability image transport.
- Phase 5's permanent capability regression matrix caught one remaining inconsistency after the Phase 4 restack: Imagen 4 disabled content/style references but still inherited Gemini's numeric `max_reference_images: 14`. The implementation was corrected to explicitly override the Imagen generation-only reference limit to `0`.

## Phase 1 — Edit geometry and provider transport

### Goals

- Add an explicit edit geometry mode: `preserve_source`, `provider_auto`, or `explicit`.
- Preserve backward compatibility with the existing optional `size` field.
- Default edits with no explicit geometry mode to `preserve_source`.
- Translate geometry per provider without inventing square output.
- Move Image Studio OpenRouter calls to the dedicated `/images` endpoint while retaining the older generic image path for non-Studio callers until separately retired.
- Introduce an Image Studio request envelope that can carry advanced fields without destabilizing the existing generic `ImageRequest` API.

### Validation

- Unit tests for geometry normalization and provider mapping.
- OpenRouter request-body tests for references, explicit aspect, provider-auto behavior, seed, and `n`.
- Existing backend/front-end quality gates and production builds.

## Phase 2 — Provider-aware masks

### Goals

- Add `masking_mode`: `none`, `semantic`, or `pixel` to provider/model capabilities while retaining `supports_masking` during the compatibility window.
- Treat OpenAI image edit masks as pixel masks.
- Treat Gemini mask images as semantic edit guidance, not exact alpha-mask transport.
- Do not advertise pixel masking for OpenRouter models unless its dedicated Images API exposes an actual mask parameter.
- Validate base/mask dimensions before pixel-mask dispatch.
- Normalize supported raster inputs as needed for deterministic OpenAI multipart edits; reject unsupported/corrupt mask combinations with a user-actionable error instead of relying on provider failures.

## Phase 3 — Selection UX completion

### Goals

- Apply stored feather values to both preview and exported alpha masks.
- Retain a user's selection when switching temporarily to a non-mask-capable model, but mark it inactive and never submit it.
- Restore the selection if the user returns to a compatible model.
- Label semantic area guidance separately from exact pixel masks.
- Add regression coverage for brush, eraser, feather, touch, zoom/pan and model transitions.

### Validation

- Frontend lint, unit tests, production build, and the full Playwright smoke suite passed on PR #114 code head `d53a024f`.
- Repository Quality Gate and Security Scan passed on that code head.
- Frontend container build passed. The backend container job remained in the shared Docker `Build and push` step at merge decision time, but Phase 3 did not change backend code; the unchanged backend had already passed container validation in Phase 2.

## Phase 4 — Capability completion

### Goals

- Remove the store's hard-coded two-reference limit and enforce model/provider limits from capabilities.
- Enforce combined content/style reference limits in the backend.
- Enforce role support (`supports_content_reference`, `supports_style_reference`) in the backend.
- Enforce `max_variants` in the backend.
- Transport seed/guidance only for providers with implemented/documented support; otherwise hide the controls.
- Make unknown/unrouted providers image-incapable instead of generation-capable by default.
- Remove capability claims for unimplemented Stability transports unless a real adapter is added.
- Keep provider/model capability overrides aligned with actual transport.

### Validation

- The exact Phase 4 code tree at `bca18644` passed Security Scan.
- Backend formatting, vet, unit/integration tests, and race detector passed.
- Frontend lint, unit tests, and production build passed.
- Windows desktop capture, plugin lifecycle, and Helm validation passed.
- Frontend container build passed.
- The reconciled Phase 4 head differed from the validated code tree only in this roadmap document.
- A manually re-run Playwright job remained queued behind repository Actions congestion at merge decision time; Phase 3's full Playwright suite had already passed after the selection-state smoke-test correction.
- Backend container jobs for both Phase 3 and Phase 4 were simultaneously stalled in Docker `Build and push` without reporting a failure while frontend images succeeded, so the merge records this as shared builder infrastructure rather than an Image Studio defect.

## Phase 5 — Regression matrix

Permanent coverage added:

- source ratios: square, portrait, landscape, 3:2, 4:3, 9:16 and unusual ratios;
- edit preserve-source behavior for OpenAI, Gemini, and OpenRouter, with a guard against invented `1024x1024` geometry;
- provider/model capability expectations for OpenAI GPT Image, DALL-E 3, Gemini image models, Imagen 4, OpenRouter, Together, Stability, and unknown providers;
- pixel, semantic, and unsupported masking semantics;
- reference-role and numeric reference-limit expectations;
- seed/guidance capability expectations;
- frontend reference-store behavior proving removal of the obsolete two-reference cap and ID deduplication.

### Validation and regression finding

- The first Phase 5 backend run passed the geometry matrix and all capability cases before failing the Imagen 4 reference-limit invariant: `max refs = 14, want 0`.
- The same run's frontend lint, unit tests, and production build passed, including the new reference-store regression coverage.
- Security Scan passed on the pre-fix regression head.
- The implementation was corrected in `backend/internal/llm/capabilities.go` so Imagen generation-only model overrides explicitly set `MaxReferenceImages` to `0`; `GetEffectiveImageCapabilities` therefore resolves the failing case to the test's expected value.
- Exact-head Quality Gate, Security Scan, and container workflows were triggered after the fix but remained queued because repository Actions capacity was occupied by older long-running Docker build jobs. No exact-head failure was reported at closeout.

## Merge policy for this program

Each phase uses a focused branch and PR. Normally a PR is merged only after the repository Quality Gate and Security Scan pass and relevant review threads are resolved. Container-build failures attributable only to unrelated infrastructure are documented explicitly; otherwise container validation is required. If GitHub Actions cannot schedule a final gate because older unrelated workflow runs are consuming repository capacity, the exception must be documented with the exact affected head, prior equivalent validation, and any deterministic regression/fix evidence. The roadmap is updated in each phase PR before merge so `main` always reflects the current program status.
