# Video Studio Next Phases

_Last updated: 2026-06-01_

## Purpose

This document captures the next implementation phases for Video Studio after the large `main` branch update that introduced the AI video creation workspace, Video Edit Studio, real OpenRouter/Gemini provider adapters, durable video assets, timeline editing primitives, render/export jobs, cross-studio imports/exports, and assistant endpoints.

The goal is to move Video Studio from a broad working foundation to a production-quality creation and editing workflow.

> **Verification note (2026-06-01):** The baseline below was re-verified against the `main` branch source. File-path anchors and per-phase "Status" callouts were added so the phases reflect what is actually implemented today rather than treating each phase as greenfield. Where a subsystem is more (or less) complete than originally written, the discrepancy is called out explicitly.

> **Implementation progress (2026-06-01):** Phase 1 implementation has started. Added backend provider/model capability validation, a frontend validation preflight, Gemini negative-prompt payload wiring, seed UI exposure, provider documentation updates, and focused backend tests. Phase 2 implementation now includes completed generation actions for timeline/chat/File Library handoff, explicit regenerate-from-history, richer review-card metadata, readable failure diagnostics, enhanced-prompt reuse, and deterministic prompt variants. Phase 3-7 implementation remains pending.

## Current Baseline

Video Studio has a strong architectural base. Backend lives in [backend/internal/video/](backend/internal/video/) (provider adapters, `timeline.go`, `renderer.go`, `assistant.go`, `service.go`), wired in [backend/internal/api/router.go](backend/internal/api/router.go) (`/v1/video/...`). Frontend lives in [frontend/src/components/video/](frontend/src/components/video/) with state in [frontend/src/stores/videoStudio.ts](frontend/src/stores/videoStudio.ts).

- Project-based AI video creation workspace behind the `video_studio` feature flag (seeded enabled in [backend/internal/db/db.go](backend/internal/db/db.go) migration V37).
- Separate Video Studio and Video Edit Studio workspaces (`VideoStudio.tsx`, `VideoEditStudio.tsx`), switched by `appMode` in [frontend/src/App.tsx](frontend/src/App.tsx).
- Real provider adapters for OpenRouter Video ([openrouter_provider.go](backend/internal/video/openrouter_provider.go)) and direct Gemini Veo 3.1 ([gemini_provider.go](backend/internal/video/gemini_provider.go)), behind a `video.Provider` interface ([provider.go](backend/internal/video/provider.go)).
- Provider and model discovery with static fallbacks (`KnownOpenRouterVideoModels()`, `KnownGeminiVeoModels()`); model lookup/validation in `model_registry.go`, plus a Phase 1 provider capability validation layer (`validation.go`) that returns structured errors, warnings, normalizations, and a normalized request before generation.
- Generation history, branching metadata, durable generated assets, and output preview/download. Persistence spans five tables: `video_projects`, `video_generations`, `video_assets`, `video_timelines`, `video_render_jobs`.
- Gemini Veo request support for aspect ratio, duration, resolution, start frame, last frame, source-video extension, reference images (≤3), person generation, negative prompt, and **seed**. Seed is now exposed in the Video Studio advanced controls for models advertising the `seed` capability.
- "Cinematic prompt detail" controls (style/camera/shot/composition/lens/lighting/dialogue/SFX/ambient/continuity/production notes). **Important:** dialogue, sound-effects, and ambient-noise fields are folded into the **prompt text** by `provider_helpers.go` — Gemini Veo has **no native audio/dialogue API parameter**. Only the OpenRouter adapter exposes a native `generate_audio` flag.
- Cross-studio asset flow via `ImportExternalAsset` (`service.go`) resolving `file_library`, `music`, `image`, and `attachment` sources. **UI caveat:** only Image Studio has a wired handoff (auto-loads as start frame); there is no in-UI picker to pull from Music Studio or other studios yet.
- Upload/import support for local assets.
- Neutral timeline JSON ([timeline.go](backend/internal/video/timeline.go), `TimelineDocument` v1) for video, image, audio, music, text, caption, shape, and callout track types.
- FFmpeg-backed render/export jobs ([renderer.go](backend/internal/video/renderer.go)). **Fidelity caveat:** the renderer composites clip trim, ordering, scaling, fixed-position overlay (`x=0:y=0`), `drawtext` for text/caption/callout, and multi-track audio mix (`atrim`/`adelay`/`amix`). It does **not** yet apply opacity, fades, arbitrary transforms/positioning, transitions, effects, or keyframes — these are stored in the timeline JSON but dropped at export (the inspector shows a standing warning to this effect). This is the core driver for Phase 4.
- Assistant endpoints for storyboard, timeline-plan, edit-plan, apply-edit-plan, and social-variants ([assistant.go](backend/internal/video/assistant.go)), with deterministic fallbacks; surfaced in the inspector UI.
- Video-to-chat (`attach-to-conversation`) and File Library registration (`register-in-library`) endpoints exist on the backend, in [frontend/src/api.ts](frontend/src/api.ts), and are now exposed as generation history actions (see Phase 2).

The next phases should harden this foundation rather than expanding surface area too quickly.

---

## Phase 1 — Harden Gemini Veo 3.1 and Provider Capability Validation

### Objective

Make Veo 3.1 generation reliable, predictable, and safe before adding more providers or deeper editing features.

### Why This Comes First

Video Studio depends on trust in the generation path. If provider validation, payload construction, or async job handling is inconsistent, the rest of the workflow becomes harder to debug. Gemini Veo 3.1 is currently the most important provider path to stabilize because it exposes the richest capability set.

### Status as of 2026-06-01

- **What exists:** Gemini and OpenRouter payload construction (start/last frame, source video, reference images, negative prompt, person generation, seed, duration/resolution/aspect ratio), model discovery with static fallback, a `Capabilities(model)` method on each provider, and a shared capability validation layer that returns structured errors/warnings/normalizations.
- **What remains:** frontend component-level coverage for validation rendering/disabled states and any future provider-specific validation refinements discovered during live provider testing.
- **Assumption to correct:** task 2 ("Audio/dialogue controls") and task 3 ("Dialogue/audio controls on silent models") are largely an **OpenRouter** concern. Gemini Veo has no native audio/dialogue parameter — dialogue/SFX/ambient are appended to the prompt text. Validation here should warn that these fields influence the prompt only (not a hard audio toggle) for Gemini, and gate the native `generate_audio` flag for OpenRouter models that don't support it.

### Implementation Progress as of 2026-06-01

- **Completed:** Added `GenerateValidationResult`/`GenerationValidationIssue` and `ModelRegistry.ValidateGenerateRequest` in `backend/internal/video/validation.go`.
- **Completed:** Both sync and async generation paths now validate and use `normalized_request` before project/generation creation and provider submission.
- **Completed:** Added `POST /v1/video/generations/validate` and frontend API/types for structured preflight validation.
- **Completed:** Video Studio now shows validation errors, warnings, and normalizations before generation and disables Generate on hard errors.
- **Completed:** Seed is exposed in the Advanced controls for seed-capable models.
- **Completed:** Gemini `GenerateRequest.NegativePrompt` is now sent as `parameters.negativePrompt`.
- **Completed:** `docs/VIDEO_PROVIDER_ADAPTERS.md` now documents Gemini start image, last frame, source video, reference image, prompt-only audio cue behavior, and normalization rules.
- **Verified:** `go test ./internal/video`, `go test ./...`, and `npm run build` pass after these changes.
- **Remaining:** Add frontend component-level tests around the validation message rendering and disabled Generate states when the test harness is expanded for Video Studio.

### Implementation Tasks

1. Add a backend provider capability validation layer.
   - Validate selected provider and model.
   - Validate capability-specific fields before provider submission.
   - Return structured validation errors/warnings.
   - Distinguish hard failures from automatic normalization.

2. Validate Gemini Veo request combinations.
   - Text-to-video.
   - Image-to-video/start frame.
   - First/last-frame interpolation.
   - Reference images.
   - Source video extension.
   - Negative prompt.
   - Person generation.
   - Seed.
   - Duration.
   - Resolution.
   - Aspect ratio.
   - Audio/dialogue controls.

3. Prevent unsupported or ambiguous combinations.
   - Last frame without start frame.
   - Source video extension combined with incompatible image modes.
   - Reference images beyond model/provider limits.
   - Unsupported aspect ratios.
   - Unsupported resolution/duration combinations.
   - Unsupported negative prompt on models that do not expose that capability.
   - Dialogue/audio controls on silent models.

4. Surface validation in the frontend before generation.
   - Hide unsupported controls.
   - Disable incompatible controls.
   - Show warnings when the backend will normalize duration/resolution.
   - Show clear, actionable messages when a user selects an unsupported combination.

5. Update provider documentation.
   - Bring `docs/VIDEO_PROVIDER_ADAPTERS.md` in sync with the actual Gemini payload structure.
   - Document start image, last frame, source video, and reference image behavior separately.
   - Document provider-specific normalization rules.

6. Add focused unit tests.
   - Gemini payload construction for each supported mode.
   - Validation edge cases.
   - Duration/resolution normalization.
   - Unsupported capability combinations.
   - Model discovery fallback behavior.
   - No real API keys required.

### Acceptance Criteria

- Invalid generation combinations fail before upstream API submission.
- Automatic request normalization is visible to the user before generation starts.
- Gemini payload tests cover text-to-video, start frame, first/last frame, reference images, and source video extension.
- Provider adapter docs match implemented behavior.
- Existing OpenRouter and Gemini generation flows remain functional.

### Suggested Copilot Prompt

```text
Review the current main branch Video Studio implementation. Focus only on hardening Gemini Veo 3.1 and provider capability validation. Add a backend validation layer that checks selected model capabilities and rejects or normalizes unsupported request combinations before provider submission. Cover text-to-video, image-to-video/start frame, first/last frame interpolation, reference images, source video extension, aspect ratio, duration, resolution, seed, person generation, negative prompt, and audio/dialogue controls. Mirror validation results in the frontend so unsupported controls are hidden or disabled and automatic normalization is shown to the user before generation. Update docs/VIDEO_PROVIDER_ADAPTERS.md to match the actual Gemini payload structure. Add unit tests for Gemini payload construction and validation edge cases using mocked inputs; do not require real API keys. Preserve existing Video Studio behavior and do not add new providers in this task.
```

---

## Phase 2 — Improve Generation Review and Iteration Workflow

### Objective

Turn completed generations into an obvious creative loop: review, branch, reuse, extend, send to timeline, or export.

### Status as of 2026-06-01

Much of this phase is **wiring, not building**. Inventory of post-generation actions:

| Action | Backend endpoint | `api.ts` function | Wired in UI? |
|---|---|---|---|
| Branch / edit prompt | n/a (client-side) | — | ✅ `branchFromGeneration` |
| Extend video | ✅ | ✅ | ✅ (Extend button on history card) |
| Use output as start frame | n/a | — | ✅ (AssetPicker) |
| Make social variant | ✅ | ✅ | ✅ (inspector) |
| Send to timeline | ✅ `/send-to-timeline` | ✅ `sendGenerationToTimeline` | ✅ generation card action |
| Send to chat | ✅ `/attach-to-conversation` | ✅ `attachToConversation` | ✅ generation card action |
| Register in File Library | ✅ `/register-in-library` | ✅ `registerInLibrary` | ✅ generation card action |
| Regenerate (same settings) | n/a | ✅ `regenerateFromGeneration` store action | ✅ generation card action |

So the highest-leverage Phase 2 endpoint wiring and review-card affordances have landed. Remaining Phase 2 work is now a few asset-role shortcuts outside the generation history card and any future LLM-backed variant expansion beyond the deterministic review helpers.

### Implementation Progress as of 2026-06-01

- **Completed:** Added per-generation actions in `VideoStudio.tsx` for Send to Timeline, Send to Chat, Register in File Library, and Extend.
- **Completed:** Send to Timeline calls the existing generation endpoint, refreshes timeline state, selects the output asset, and opens Video Edit Studio.
- **Completed:** Send to Chat creates a conversation, attaches the generated video asset, writes a no-reply chat message containing the attachment download URL, and opens Chat Studio.
- **Completed:** Register in File Library calls the existing asset registration endpoint and reports success/failure per asset.
- **Completed:** Regenerate now has an explicit generation-card action. It reloads provider/model/settings/input asset roles from the source generation, disables re-enhancement, and starts a child generation with the previous effective prompt.
- **Completed:** Generation detail responses now include structured input asset roles, upstream job/request IDs, and provider usage JSON so review cards can show input mode, normalized settings, provider metadata, usage/cost, and readable failure diagnostics.
- **Completed:** Result preview and generation history preserve original/enhanced prompt comparison and provide one-click enhanced-prompt reuse.
- **Completed:** Result preview now offers deterministic Cinematic, Social, Product, Explainer, and Documentary prompt variants that load the previous effective prompt and reuse compatible settings.
- **Verified:** `go test ./internal/api ./internal/video` passes after API/detail changes.
- **Verified:** `npm run build` passes after the Phase 2 UI wiring.
- **Remaining:** additional asset-role shortcuts for generated frames/images and soundtrack candidates.

### Implementation Tasks

1. Wire up and strengthen post-generation actions.
   - Regenerate with same settings (explicit action, not only via branch). _(wired as generation card action)_
   - Branch/edit prompt. _(exists)_
   - Use output as start frame. _(exists)_
   - Extend this video. _(exists)_
   - Send to timeline. _(wired as generation card action)_
   - Make social variant. _(exists)_
   - Send to chat. _(wired as generation card action)_
   - Register in File Library. _(wired as generation card action)_

2. Improve generation history cards.
   - Show input mode. _(wired)_
   - Show start/last/reference/source inputs. _(wired from structured `input_assets_json`)_
   - Show normalized settings. _(wired from `settings_json`)_
   - Show provider job metadata. _(wired from enriched generation detail fields)_
   - Show duration, resolution, aspect ratio, and cost when available. _(wired)_
   - Show failure diagnostics in a readable way. _(wired)_

3. Add prompt iteration helpers.
   - Preserve original prompt and enhanced prompt. _(wired)_
   - Allow side-by-side comparison. _(wired in result preview/history summaries)_
   - Allow one-click reuse of enhanced prompt. _(wired)_
   - Add prompt variant generation for cinematic, social, product, explainer, and documentary outputs. _(wired as deterministic result-preview variants)_

4. Add asset role shortcuts.
   - Generated video as source video for extension. _(wired through history-card Extend)_
   - Generated frame/image as start frame. _(remaining; no frame extraction shortcut yet)_
   - Imported image as reference image. _(exists through reference image pickers/uploads)_
   - Music asset as soundtrack candidate for edit workflow.

### Acceptance Criteria

- A user can complete a generation and immediately understand what to do next.
- Generated assets can be reused as inputs without manual upload/download.
- Branching preserves enough metadata to recreate or modify a generation.
- Failed generations provide actionable troubleshooting details.

### Suggested Copilot Prompt

```text
Enhance the Video Studio generation review workflow. After a generation completes, provide clear actions for regenerate, branch prompt, reuse output as start frame, extend video, send to timeline, send to chat, register in File Library, and create a social variant. Improve generation history cards to show input mode, normalized settings, provider metadata, cost/usage when available, and readable failure diagnostics. Preserve existing API contracts where possible. Add frontend tests or component-level coverage for the new actions and ensure all actions gracefully handle missing assets or failed generations.
```

---

## Phase 3 — Make Video Edit Studio Feel Like a Real Timeline Editor

### Objective

Move Video Edit Studio from a partially usable editor to a fully usable timeline editing experience. (It is further along than "a structurally correct editor shell" — see status.)

### Status as of 2026-06-01

The editor already has more than the original draft assumed. Already implemented: timeline **zoom**, **snapping** (toggle, on by default), a partial **keyboard shortcut** set (Space play/pause, Delete, Ctrl+S save, S split-at-playhead), **playhead playback with preview sync** (`VideoPreviewCanvas` scrubs to `playheadMs`), clip move/split/duplicate/delete reducers, and a working **inspector** (transform/opacity/volume/fade/text + effect/transition/keyframe authoring). 

**Not yet implemented — this is the real Phase 3 work:** undo/redo (no history stack in the store), multi-select, true drag-and-drop from the media bin (the bin uses "Add to timeline" buttons, not drag), media-bin thumbnails/filters/rename/delete, and dedicated trim *handles* (trimming today is done via numeric inputs in the inspector).

> Note: opacity/fade/transform values authored in the inspector are **not honored by the export renderer yet** (Phase 4). The inspector already shows a warning; keep that warning accurate as Phase 4 lands.

### Implementation Tasks

1. Add editor interaction fundamentals.
   - Undo/redo stack. _(missing — highest priority; also a prerequisite for Phase 5 safety rails)_
   - Timeline zoom. _(exists)_
   - Timeline snapping. _(exists)_
   - Multi-select clips. _(missing)_
   - Keyboard shortcuts. _(partial — extend the existing set)_
   - Drag/drop media from bin to timeline. _(missing — bin currently uses buttons)_
   - Better trim handles. _(missing — only numeric trim today)_
   - Better clip move affordances.

2. Improve playback and preview.
   - Timeline playhead playback.
   - Preview follows playhead.
   - Clip selection syncs between timeline and inspector.
   - Preview reflects transforms, opacity, text, and basic clip timing.

3. Improve media bin workflow.
   - Thumbnail grid/list toggle.
   - Asset type filters.
   - Rename/delete asset actions.
   - Drag asset into timeline.
   - Show source studio/source ID metadata.

4. Improve inspector usability.
   - Clip timing controls.
   - Transform controls.
   - Opacity/fade controls.
   - Text/caption styling controls.
   - Audio volume controls.
   - Warnings for settings not yet supported by export.

5. Add timeline usability polish.
   - Track headers.
   - Track lock/mute/solo visibility flags.
   - Clip duration labels.
   - Time ruler.
   - Zoom-to-fit.
   - Empty-state onboarding.

### Acceptance Criteria

- Users can assemble a simple multi-clip video without fighting the timeline.
- Timeline selection, preview, and inspector remain synchronized.
- Basic edit actions are undoable.
- Drag/drop and trim interactions feel predictable.
- Unsupported export fidelity is clearly disclosed.

### Suggested Copilot Prompt

```text
Improve Video Edit Studio timeline usability without changing the underlying neutral timeline JSON schema unless absolutely necessary. Add undo/redo, timeline zoom, snapping, keyboard shortcuts, better trim handles, drag/drop from media bin to timeline, selection synchronization with the preview and inspector, and clear export-fidelity warnings. Preserve existing project, asset, timeline, and render APIs. Add tests for reducer actions and key timeline interactions where feasible.
```

---

## Phase 4 — Close Render Fidelity Gaps

### Objective

Make exported videos match the timeline preview more closely.

### Current Gap

The timeline JSON stores effects, transitions, fades, opacity, keyframes, transforms, and audio information, but the FFmpeg renderer ([renderer.go](backend/internal/video/renderer.go)) honors only a subset. This is a confirmed gap, not a hypothetical one: the inspector ships a standing warning — _"Effects, transitions, fades, opacity, and keyframes are stored in the timeline JSON but are not yet applied by the FFmpeg renderer during video export."_

Verified renderer behavior as of 2026-06-01:

| Timeline feature | Rendered today? |
|---|---|
| Clip trim (`trim_in_ms`/`trim_out_ms`) | ✅ |
| Clip ordering / timing (`overlay enable=between(...)`) | ✅ |
| Video/image scaling | ✅ |
| Positioning / transform | ❌ overlay is hardcoded `x=0:y=0` |
| Cropping | ❌ |
| Opacity | ❌ |
| Fade in/out (video) | ❌ |
| Text / caption / callout (`drawtext`) | ✅ |
| Transitions | ❌ |
| Effects / keyframes | ❌ |
| Multi-track audio mix (`atrim`/`adelay`/`amix`) | ✅ (basic) |

So tasks below that read as "add" are in two buckets: **accuracy hardening** for what already renders (trim, ordering, scaling, text, audio mix) and **net-new** for what is dropped (positioning/crop, opacity, fades, transitions, effects, keyframes).

### Implementation Tasks

1. Formalize renderer capability reporting.
   - Add backend renderer capability metadata (single source of truth so the inspector warning is derived, not hardcoded).
   - Expose which timeline features are export-supported.
   - Show warnings in the inspector/render panel based on actual renderer capability.

2. Improve FFmpeg export fidelity.
   - Clip trim accuracy. _(harden — already rendered)_
   - Clip ordering. _(harden — already rendered)_
   - Video/image scaling. _(harden — already rendered)_
   - Positioning and cropping. _(net-new — overlay is fixed at `x=0:y=0` today)_
   - Opacity. _(net-new)_
   - Fade in/out. _(net-new for video)_
   - Basic text/caption rendering. _(harden — already rendered via `drawtext`)_
   - Basic callout rendering. _(harden — already rendered)_
   - Basic transitions. _(net-new — e.g. `xfade`)_

3. Add audio support.
   - Audio track inclusion. _(exists)_
   - Music bed inclusion. _(exists via audio/music tracks)_
   - Volume control. _(verify wiring of per-clip `volume` into the mix)_
   - Fade in/out. _(net-new — audio fades not applied)_
   - Basic mixdown. _(exists — `amix`)_
   - Mute/solo behavior. _(net-new)_

4. Add export presets.
   - 16:9 YouTube/LinkedIn.
   - 9:16 Shorts/TikTok/Reels.
   - 1:1 social square.
   - Custom resolution/FPS.

5. Add render diagnostics.
   - FFmpeg command capture in job metadata.
   - Render stderr preservation for failed jobs.
   - Media probe metadata.
   - Estimated duration validation.

### Acceptance Criteria

- A simple timeline export visually matches preview for clip order, timing, basic transforms, and text.
- Audio can be included and mixed at a basic level.
- Renderer limitations are explicit before export.
- Failed render jobs include useful diagnostics.
- Export presets produce correct canvas dimensions and aspect ratios.

### Suggested Copilot Prompt

```text
Improve Video Edit Studio FFmpeg render fidelity. Add renderer capability metadata and surface export-fidelity warnings in the frontend based on actual supported features. Implement export support for clip trim/order, scaling, positioning, cropping, opacity, fade in/out, basic text/caption rendering, and basic audio mixdown. Add render presets for 16:9, 9:16, and 1:1 outputs. Preserve FFmpeg stderr and command diagnostics in render job metadata. Add tests for timeline-to-render translation and renderer capability reporting.
```

---

## Phase 5 — Make the Assistant Timeline-Aware

### Objective

Move the assistant from generic storyboard/edit suggestions to concrete timeline-aware editing help.

### Status as of 2026-06-01

The assistant scaffolding exists end-to-end: five endpoints (`storyboard`, `timeline-plan`, `edit-plan`, `apply-edit-plan`, `social-variants`) in [assistant.go](backend/internal/video/assistant.go) with deterministic fallbacks, `api.ts` wrappers, and inspector UI that renders storyboards, edit-plan summaries (with an Apply button), and social-variant badges. The work here is **depth and safety**, not plumbing: feeding real timeline/asset/track/selection context into planning, validating each operation, and producing human-readable previews before apply.

> **Cross-phase dependency:** task 4's "Keep undo/redo compatible with assistant-applied plans" and the acceptance criterion "Assistant-applied edits are undoable" both presuppose the **Phase 3 undo/redo stack, which does not exist yet**. Either sequence Phase 3's undo/redo before this phase, or descope undoability from Phase 5's acceptance criteria.

### Implementation Tasks

1. Give assistant endpoints structured timeline context.
   - Active project settings.
   - Asset list.
   - Clip list.
   - Track structure.
   - Current playhead/selection when available.
   - Renderer capability metadata.

2. Improve assistant edit plans.
   - Add deterministic validation for every edit operation.
   - Require operation previews before application.
   - Explain what will change.
   - Support partial application when some operations are invalid.

3. Add high-value assistant workflows.
   - Create 30-second social cut.
   - Create 15-second teaser.
   - Convert 16:9 to 9:16.
   - Add title card.
   - Add lower thirds.
   - Add captions from supplied text.
   - Tighten pacing.
   - Build timeline from storyboard.
   - Generate alternate versions.

4. Add assistant safety rails.
   - Never mutate timeline without confirmation.
   - Validate asset ownership and existence.
   - Avoid destructive operations unless explicitly requested.
   - Keep undo/redo compatible with assistant-applied plans.

### Acceptance Criteria

- Assistant edit plans reference real clips/assets/tracks.
- Users can preview and apply assistant edits safely.
- Assistant-applied edits are undoable.
- Invalid plans are rejected with clear explanations.
- Social variant generation uses actual timeline/project state.

### Suggested Copilot Prompt

```text
Make the Video Edit Studio assistant timeline-aware. Pass structured project, asset, track, clip, selection, and renderer capability context into assistant planning. Improve edit-plan validation so every operation is checked before application and produce a human-readable preview of the changes. Add assistant workflows for 30-second social cut, 15-second teaser, title card, lower thirds, captions from supplied text, vertical variant, and storyboard-to-timeline. Do not mutate the timeline without explicit user confirmation. Ensure assistant-applied changes are compatible with undo/redo.
```

---

## Phase 6 — Production Hardening and Reliability

### Objective

Make Video Studio resilient across app restarts, provider failures, large assets, and long-running jobs.

### Implementation Tasks

1. Harden generation lifecycle.
   - Recover pending/running jobs after app restart where provider APIs support it.
   - Poll orphaned upstream jobs.
   - Retry transient download failures.
   - Preserve provider operation IDs.
   - Persist progress state where useful.
   - Handle browser close/reopen.

2. Harden cancellation.
   - Map local cancellation to upstream cancellation where available.
   - Mark local jobs cancelled even if upstream cannot cancel.
   - Avoid completed assets being orphaned after late upstream completion.

3. Improve asset validation.
   - File size limits by asset type.
   - MIME sniffing and extension validation.
   - Video duration limits.
   - Image dimension checks.
   - Safer upload errors.

4. Improve storage hygiene.
   - Remove orphaned files.
   - Delete project assets safely.
   - Add storage accounting.
   - Add cleanup jobs for failed partial outputs.

5. Improve observability.
   - Structured logs for provider requests/responses, excluding secrets.
   - Request IDs across generation/render jobs.
   - Metrics for job duration, failure rates, and provider errors.

### Acceptance Criteria

- Long-running generation and render jobs behave predictably across reloads.
- Failed jobs retain enough information to debug.
- Uploads reject unsafe or unsupported files cleanly.
- Storage cleanup avoids orphan accumulation.
- Provider secrets never leak to frontend, logs, or persisted metadata.

### Suggested Copilot Prompt

```text
Harden Video Studio generation, render, asset, and storage lifecycle behavior. Add recovery handling for pending/running provider jobs where possible, retry transient output downloads, improve cancellation behavior, validate uploaded files more strictly, preserve useful diagnostics, and add storage cleanup for orphaned partial files. Ensure secrets are never logged or returned to the frontend. Add tests for failed generation, failed render, cancellation, bad upload, and orphan cleanup scenarios.
```

---

## Phase 7 — Provider Expansion

### Objective

Add more video generation providers only after Gemini/OpenRouter paths and the editing workflow are stable.

### Recommended Order

1. Finish Gemini Veo 3.1 hardening.
2. Verify OpenRouter video end-to-end.
3. Add provider-agnostic validation and capability metadata.
4. Add one new provider at a time.

Potential future providers:

- Luma.
- Runway.
- Pika.
- Kling.
- Stability video.
- OpenAI video when a stable public API path is available and desired.

### Implementation Tasks

1. Add provider capability mapping.
   - Supported input modes.
   - Supported aspect ratios.
   - Supported durations.
   - Supported resolutions.
   - Reference image limits.
   - Source video limits.
   - Audio/dialogue behavior.

2. Add provider-specific tests.
   - Payload construction.
   - Submit/poll/download.
   - Error mapping.
   - Cost/usage metadata.
   - Multi-output handling.

3. Add provider-specific UI hints.
   - Model notes.
   - Capability badges.
   - Estimated generation time/cost where possible.
   - Provider-specific limitations.

### Acceptance Criteria

- New providers plug into the existing provider abstraction.
- Frontend controls remain capability-driven.
- Provider-specific behavior does not leak into neutral timeline or project state.
- Each provider has mocked tests and clear documentation.

### Suggested Copilot Prompt

```text
Add the next video provider only after Gemini and OpenRouter generation paths are stable. Implement the provider using the existing video.Provider abstraction. Add capability metadata, provider-specific validation, model discovery or static fallback, submit/poll/download behavior, error mapping, cost/usage preservation where available, and mocked tests. Do not persist provider-native timeline/project formats. Keep frontend controls capability-driven.
```

---

## Recommended Execution Order

1. Phase 1 — Gemini/provider validation.
2. Phase 2 — Generation review and iteration workflow. _(front-loaded: ~3 of the headline actions are already-built endpoints awaiting UI wiring — fast wins.)_
3. Phase 3 — Timeline editor usability. _(land undo/redo here; Phase 5 depends on it.)_
4. Phase 4 — Render fidelity.
5. Phase 5 — Timeline-aware assistant. _(depends on Phase 3 undo/redo for the "assistant edits are undoable" criterion.)_
6. Phase 6 — Production hardening.
7. Phase 7 — Provider expansion.

## Guiding Principle

Do not expand the number of providers or UI surface area until the core loop is dependable:

```text
Prompt or asset input → validated provider generation → durable asset → review/iterate → timeline composition → faithful export → share/register/reuse
```

That loop should be the product quality bar for Video Studio.
