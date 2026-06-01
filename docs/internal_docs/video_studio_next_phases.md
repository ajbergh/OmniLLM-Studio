# Video Studio Next Phases

_Last updated: 2026-06-01_

## Purpose

This document captures the next implementation phases for Video Studio after the large `main` branch update that introduced the AI video creation workspace, Video Edit Studio, real OpenRouter/Gemini provider adapters, durable video assets, timeline editing primitives, render/export jobs, cross-studio imports/exports, and assistant endpoints.

The goal is to move Video Studio from a broad working foundation to a production-quality creation and editing workflow.

## Current Baseline

Video Studio now has a strong architectural base:

- Project-based AI video creation workspace behind the `video_studio` feature flag.
- Separate Video Studio and Video Edit Studio workspaces.
- Real provider adapters for OpenRouter Video and direct Gemini Veo 3.1.
- Provider and model discovery with static fallbacks.
- Generation history, branching metadata, durable generated assets, and output preview/download.
- Gemini-oriented controls for aspect ratio, duration, resolution, start frame, last frame, source video extension, reference images, person generation, and cinematic prompt detail.
- Cross-studio asset flow from Image Studio and Music Studio into Video Studio.
- Upload/import support for local assets.
- Neutral timeline JSON for video, image, audio, music, text, caption, shape, and callout track types.
- FFmpeg-backed render/export jobs.
- Assistant endpoints for storyboard and edit planning, with deterministic fallbacks where needed.
- Video-to-chat and File Library registration paths.

The next phases should harden this foundation rather than expanding surface area too quickly.

---

## Phase 1 — Harden Gemini Veo 3.1 and Provider Capability Validation

### Objective

Make Veo 3.1 generation reliable, predictable, and safe before adding more providers or deeper editing features.

### Why This Comes First

Video Studio depends on trust in the generation path. If provider validation, payload construction, or async job handling is inconsistent, the rest of the workflow becomes harder to debug. Gemini Veo 3.1 is currently the most important provider path to stabilize because it exposes the richest capability set.

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

### Implementation Tasks

1. Add stronger post-generation actions.
   - Regenerate with same settings.
   - Branch/edit prompt.
   - Use output as start frame.
   - Extend this video.
   - Send to timeline.
   - Make social variant.
   - Send to chat.
   - Register in File Library.

2. Improve generation history cards.
   - Show input mode.
   - Show start/last/reference/source inputs.
   - Show normalized settings.
   - Show provider job metadata.
   - Show duration, resolution, aspect ratio, and cost when available.
   - Show failure diagnostics in a readable way.

3. Add prompt iteration helpers.
   - Preserve original prompt and enhanced prompt.
   - Allow side-by-side comparison.
   - Allow one-click reuse of enhanced prompt.
   - Add prompt variant generation for cinematic, social, product, explainer, and documentary outputs.

4. Add asset role shortcuts.
   - Generated video as source video for extension.
   - Generated frame/image as start frame.
   - Imported image as reference image.
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

Move Video Edit Studio from a structurally correct editor shell to a usable timeline editing experience.

### Implementation Tasks

1. Add editor interaction fundamentals.
   - Undo/redo stack.
   - Timeline zoom.
   - Timeline snapping.
   - Multi-select clips.
   - Keyboard shortcuts.
   - Drag/drop media from bin to timeline.
   - Better trim handles.
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

The timeline JSON stores effects, transitions, fades, opacity keyframes, transforms, and audio information, but export support may lag behind what the inspector and timeline can express. This creates a risk where users build something in the editor that does not render as expected.

### Implementation Tasks

1. Formalize renderer capability reporting.
   - Add backend renderer capability metadata.
   - Expose which timeline features are export-supported.
   - Show warnings in the inspector/render panel based on actual renderer capability.

2. Improve FFmpeg export fidelity.
   - Clip trim accuracy.
   - Clip ordering.
   - Video/image scaling.
   - Positioning and cropping.
   - Opacity.
   - Fade in/out.
   - Basic text/caption rendering.
   - Basic callout rendering.
   - Basic transitions.

3. Add audio support.
   - Audio track inclusion.
   - Music bed inclusion.
   - Volume control.
   - Fade in/out.
   - Basic mixdown.
   - Mute/solo behavior.

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
2. Phase 2 — Generation review and iteration workflow.
3. Phase 3 — Timeline editor usability.
4. Phase 4 — Render fidelity.
5. Phase 5 — Timeline-aware assistant.
6. Phase 6 — Production hardening.
7. Phase 7 — Provider expansion.

## Guiding Principle

Do not expand the number of providers or UI surface area until the core loop is dependable:

```text
Prompt or asset input → validated provider generation → durable asset → review/iterate → timeline composition → faithful export → share/register/reuse
```

That loop should be the product quality bar for Video Studio.
