# Image Studio Remediation Roadmap

Status: In progress
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
| 2 | Provider-aware mask semantics and source/mask validation | Complete | PR #113 |
| 3 | Selection UX completion: feathering and capability transitions | In progress | PR #114 |
| 4 | Capability completion: references, variants, seed/guidance, honest provider matrix | Planned | PR #115 |
| 5 | Regression matrix and documentation closeout | Planned | PR #116 |

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
- Brush feather is stored in stroke state but is not applied to preview or exported masks; Phase 3 completes that path.
- A previously painted selection can remain in memory when switching to a model that cannot consume masks; Phase 3 makes that retained selection inactive and prevents submission.

### Capability completion

- `seed` and `creativity` are persisted for generation nodes but are not transported through the legacy `llm.ImageRequest`, so controls can appear meaningful without affecting generation.
- The frontend image-editor store hard-caps content/style reference arrays at two while provider capabilities advertise larger limits for some models.
- The capability matrix advertised image capabilities for provider types that the service did not route, including a permissive unknown-provider default; Phase 2 makes unknown/unimplemented transports image-incapable.
- Backend request validation does not consistently enforce reference-role support, combined reference limits, or model variant limits.

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

## Phase 5 — Regression matrix

Required permanent coverage:

- source ratios: square, portrait, landscape, 3:2, 4:3, 9:16 and unusual ratios;
- edit with preserve-source default versus explicit geometry override;
- PNG/JPEG/WebP source handling where supported;
- masked versus unmasked edits and source/mask dimension mismatch;
- provider/model switches before submit;
- mask-capable → non-mask-capable → mask-capable transitions;
- node direct selection, undo, redo and branching;
- one-finger mask drawing and two-finger pinch/pan;
- generation → edit → generation state transitions;
- unknown/stale capability values;
- reference-role and reference-count validation;
- variant-count validation;
- provider request-body tests for every Image Studio transport.

## Merge policy for this program

Each phase uses a focused branch and PR. A PR is merged only after the repository Quality Gate and Security Scan pass and relevant review threads are resolved. Container-build failures attributable only to unrelated infrastructure are documented explicitly; otherwise container validation is required. The roadmap is updated in each phase PR before merge so `main` always reflects the current program status.
