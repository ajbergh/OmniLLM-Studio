> **Archived — superseded implementation prompt.** The asynchronous generation and provider work described here is on `main`; remaining Video work is in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# GitHub Copilot Implementation Prompt: Enhance OmniLLM-Studio Video Studio for Gemini Veo 3.1

You are working in the `ajbergh/OmniLLM-Studio` repository on the local branch `feat_video_studio`.

> **Document status:** Reviewed and reconciled against the `feat_video_studio` branch and the live Gemini Veo docs on **2026-05-31**. The Veo 3.1 capability facts below are verified accurate. The branch already contains a substantial Video Studio implementation — this is **not** a greenfield build. Read **"Verified current implementation state"** below before treating any "add/implement" instruction as net-new; in most cases you are *refactoring or extending* existing code, and the schema/route/enum names in this doc must be mapped onto the structures that already exist (see the reconciliation notes). Following the original suggested schema verbatim would create a **parallel, conflicting** set of tables.

## Goal

Make Gemini Veo 3.1 a first-class, direct-API video generation provider with its full current capability suite (text-to-video, image-to-video, first/last-frame interpolation, reference images, and extension), backed by a **non-blocking async job flow** and durable local asset storage.

The immediate focus is AI **video generation** quality and the Veo async lifecycle — not new timeline-editor features. A full timeline editor, FFmpeg renderer, and render-job system **already exist on this branch and must be preserved**; do not remove or regress them, and do not rebuild them. Reuse the existing Image Studio / Music Studio patterns and the cross-studio asset import that is already wired.

## Verified current implementation state (as of 2026-05-31)

This branch already implements far more than a stub. Verified by inspection:

**Backend — `backend/internal/video/`:** `service.go`, `provider.go`, `gemini_provider.go`, `openrouter_provider.go`, `model_registry.go`, `provider_helpers.go`, `prompt_enhancer.go`, `assistant.go`, `renderer.go` (FFmpeg), `timeline.go`, `storage.go`, `types.go`, `models.go`. Tests exist: `model_registry_test.go`, `provider_adapters_test.go`, `renderer_test.go`, `service_external_import_test.go`, `timeline_test.go`.

**Database — migrations V37–V39 in `backend/internal/db/db.go`:** tables `video_projects`, `video_generations`, `video_assets`, `video_timelines`, `video_render_jobs`. **There is no `video_sessions` table (it is `video_projects`) and no `video_generation_references` table** — input/reference assets are stored as a JSON array column `input_asset_ids_json` on `video_generations`.

**API routes (all under `/v1/video/`, wired in `router.go`):** `providers`, `models`, `models/refresh`, `projects` CRUD, `generations` (POST is an **SSE** endpoint), `generations/{id}`, `generations/{id}/branch`, `generations/{id}/send-to-timeline`, project assets list/import, `assets/{id}` get/download/delete, `assets/{id}/attach-to-conversation`, `assets/{id}/register-in-library`, timeline get/save/import-asset, `render` + `render-jobs/{id}` + `render-jobs/{id}/cancel`, `enhance-prompt`, and assistant endpoints (storyboard, timeline-plan, edit-plan, apply-edit-plan, social-variants). **There is no generation-level cancel route and no on-demand generation poll route** (see §4 — these are gaps to close).

**Providers:** `gemini_provider.go` calls the Veo `predictLongRunning` REST flow directly and registers `veo-3.1-generate-preview` and `veo-3.1-fast-generate-preview` only — its capability set is **text-to-video only** (`text_to_video`, `seed`, `camera_motion`, `audio_generation`). `openrouter_provider.go` registers `google/veo-3.1`, `google/veo-3.1-fast`, `google/veo-3.1-lite` plus Kling/Hailuo/Seedance/Wan/Grok models, and is the only path that currently advertises `image_to_video` / `reference_images`.

**Frontend:** `frontend/src/components/video/` (`VideoStudio.tsx`, `VideoEditStudio.tsx`, `VideoInspector.tsx`, `VideoPreviewCanvas.tsx`, `VideoRenderPanel.tsx`, `RenderJobStatus.tsx`, `timeline/VideoTimeline.tsx`), store `frontend/src/stores/videoStudio.ts`, types `frontend/src/types/video.ts`, API functions in `frontend/src/api.ts`.

**Key gaps this task must close (the rest is largely present):**
- **Async lifecycle:** generation is currently **fully blocking** — `service.Generate` calls `provider.Generate(ctx, …)`, which polls the Veo operation *inline* and downloads the MP4 *within the same HTTP/SSE request*. There is no immediate-return job, no background polling worker, and no restart recovery. This is the single most important thing to change.
- **Veo direct-API capabilities:** the Gemini provider does text-to-video only. Image-to-video, first/last-frame, reference images, and extension are not implemented on the direct Gemini path.
- **Lite model:** `veo-3.1-lite-generate-preview` is not registered on the Gemini direct provider.
- **`person_generation`** is not surfaced as a first-class request field.

## Required reading before coding

1. Inspect the current checked-out branch first:
   - Identify all existing Video Studio files, routes, database migrations, services, React components, types, and API functions.
   - Determine what is already implemented versus stubbed.
   - Preserve good existing work and refactor only where needed.

2. Compare against `main` where helpful:
   - Reuse existing architectural patterns from Image Studio and Music Studio.
   - Reuse the existing Go backend, React/TypeScript frontend, SQLite persistence, encrypted provider settings, attachment storage, and local-first design.
   - Do not introduce a parallel storage system, separate auth model, or frontend-only secret handling.

3. Re-read the Gemini Veo 3.1 documentation:
   - Main doc: `https://ai.google.dev/gemini-api/docs/video?example=dialogue`
   - Pay special attention to:
     - Text-to-video generation
     - Aspect ratio
     - Resolution
     - Image-to-video generation
     - Reference images
     - First and last frame interpolation
     - Video extension
     - Handling asynchronous operations
     - Veo API parameters and specifications
     - Model versions
     - Veo prompt guide

## Product requirements

Implement a Video Studio generation suite with these capabilities:

### 1. Model support

A capability registry already exists — extend it, do not replace it. The Gemini direct provider (`gemini_provider.go` → `KnownGeminiVeoModels`) currently registers only `veo-3.1-generate-preview` and `veo-3.1-fast-generate-preview`. Add the missing model and align capabilities:

- `veo-3.1-generate-preview` (already present — extend capabilities beyond text-to-video)
- `veo-3.1-fast-generate-preview` (already present — extend capabilities)
- `veo-3.1-lite-generate-preview` (**missing on the Gemini provider — add it**)

Also preserve room for:
- `veo-3.0-generate-001`
- `veo-3.0-fast-generate-001`
- `veo-2.0-generate-001`
- future provider-backed video models (the OpenRouter provider already aggregates Kling, Hailuo, Seedance, Wan, Grok, etc.)

The capability model is the existing `video.Model` struct (`types.go`) with a `Capabilities []Capability` slice — there is **no `mode` field anywhere in the code** (see §2/§3 reconciliation). Express per-model support by populating `Capabilities`, `AspectRatios`, `Resolutions`, `DurationMin/MaxSeconds`, `FPSOptions`, and `MaxPromptChars`. Each model should expose only the controls its capability set allows; the frontend reads model metadata to enable/disable controls.

**Verified Veo 3.1 capability matrix (Gemini API, confirmed 2026-05-31):**

| Model | Resolutions | 4K | Extension | Notes |
|---|---|---|---|---|
| `veo-3.1-generate-preview` | 720p, 1080p, 4k | ✅ | ✅ (720p) | Full suite |
| `veo-3.1-fast-generate-preview` | 720p, 1080p | ❌ | ✅ (720p) | |
| `veo-3.1-lite-generate-preview` | 720p, 1080p | ❌ | ❌ | No 4K, no extension |

All Veo 3.1 models: aspect ratios `16:9` (default) and `9:16`; durations `4 | 6 | 8` seconds; **1080p and 4k require duration = 8**; extension and reference images also require duration = 8; prompt limit **1,024 tokens**; up to **3 reference images**; single output video per request.

### 2. Generation modes

Support the following generation modes:

#### Text to video

User provides:
- prompt
- model
- aspect ratio
- duration
- resolution
- person generation setting where applicable
- optional seed where supported
- optional prompt enhancement

#### Image to video

User can use an image as the starting frame.

The image source can be:
- uploaded directly in Video Studio
- selected from existing attachments
- selected from Image Studio generated assets
- imported from File Library if image assets are already supported there

The UI should make Image Studio generated images easy to reuse as Veo starting frames.

#### First and last frame generation

Support Veo 3.1 interpolation by allowing:
- first frame image
- last frame image
- prompt describing the transition/action between frames

Validation:
- `lastFrame` must not be accepted without a first frame / image.
- Label this clearly as a Veo 3.1-specific capability.

#### Reference images

Support up to three reference images for Veo 3.1 where supported.

Reference images should be typed and explained in the UI:
- subject / character reference
- product / object reference
- style / visual reference

The request layer should map these to the Gemini `referenceImages` shape with `referenceType: "asset"` unless the SDK requires a different exact type.

Validation:
- Maximum three reference images.
- Disable or hide reference images for models that do not support them.
- Ensure duration constraints are enforced when reference images require 8 seconds.

#### Video extension

Support extending a previously generated Veo video.

The source video should come from:
- a prior Video Studio generation asset
- only videos that were generated by Veo and have sufficient stored metadata to be extendable

The UI should offer an “Extend” action on eligible video results.

Validation:
- Do not offer extension for Veo 3.1 Lite.
- Enforce extension model limitations.
- Enforce 720p limitation for extension.
- Make clear that extension continues from the final second / final 24 frames.
- Store extension lineage so the user can see the chain of clips.

### 3. Veo parameter support

The backend request struct **already exists** as `video.GenerateRequest` in `backend/internal/video/types.go`. Extend it rather than introducing a new `VideoGenerationRequest`. Reconcile the conceptual fields below with the real ones:

> **Reconciliation — there is no `mode` field.** The current design is capability-driven, not mode-driven. `GenerateRequest` already has `Provider`, `Model`, `Prompt`, `Enhance`/`EnhancedPrompt`, `NegativePrompt`, `AspectRatio`, `DurationSeconds`, `Resolution`, `FPS`, `Seed *int64`, `ReferenceAssetIDs []string`, `CameraMotion`, `ShotType`, `StylePreset`, `Settings json.RawMessage`. The "modes" in earlier drafts map onto **which inputs are present + the model's capabilities**, not a discriminator enum. Do **not** add a `mode` column. Instead add the *missing* fields and infer the operation:

Fields to **add** to `GenerateRequest` (none of these exist today):
- `PersonGeneration` (string) — not currently surfaced
- `StartImageAssetID` / `LastFrameAssetID` — first/last-frame interpolation (today only the flat `ReferenceAssetIDs` exists)
- `SourceVideoAssetID` — for extension
- distinguish reference-image asset IDs from start/last-frame IDs (currently everything is conflated into `ReferenceAssetIDs` → `input_asset_ids_json`)

Fields that already exist and should be reused as-is: `Provider`, `Model`, `Prompt`, `AspectRatio` (`16:9 | 9:16`), `DurationSeconds` (`4 | 6 | 8`), `Resolution` (`720p | 1080p | 4k`), `Seed`, `Enhance`/`EnhancedPrompt`, `NegativePrompt`. Veo via the Gemini API **does** accept a negative prompt, so keep it for Veo; gate it per-capability (`negative_prompt`) for other providers.

Use capability-driven validation. Examples:
- 1080p and 4k require 8 seconds where the model requires it.
- 4k is not available on Veo 3.1 Lite.
- Extension is 720p only.
- Reference images require 8 seconds where required.
- Output video count is one for Veo 3.x.
- Text input should respect Veo’s documented **1,024 token** input limit. Note: the current Gemini models register `MaxPromptChars: 4000`, which is a rough char proxy for the token limit — surface this as token-based UI guidance and do not silently truncate without warning.

### 4. Async operation handling

> **This is the most important change in this task.** The current implementation is **blocking**: `service.Generate` (`service.go`) calls `provider.Generate(ctx, …)`, and the Gemini adapter polls the `predictLongRunning` operation *inline* and downloads the MP4 *inside the same `POST /v1/video/generations` SSE request*. The generation row transitions `pending → running → completed/failed` all within one request. The Veo operation name is not persisted for recovery, there is no background worker, and an app restart mid-generation orphans the job. Refactor this into a real async flow. (Note: `WriteTimeout` on the headless server is 5m — long Veo jobs at 1080p/4k can approach or exceed this, which is a concrete reason the blocking design must go.)

Veo returns a long-running operation. Build a backend job flow:

- `POST /v1/video/generations` (currently SSE + blocking — change to return immediately)
  - creates a local generation record (status `pending`)
  - starts the provider operation
  - **persists the provider operation name/id** — store it in the existing `upstream_job_id` / `upstream_request_id` columns on `video_generations` (do not add a new `provider_operation_name` column)
  - returns immediately with local generation id and status

- `GET /v1/video/generations/{id}` (**already exists** — extend it to drive polling)
  - returns current job state, provider operation state, progress/status text, metadata, errors, and assets

- polling worker or on-demand poll endpoint (**neither exists today — add one**):
  - polls Gemini operation status
  - handles done/error states
  - downloads the generated MP4 when complete
  - stores it in local attachment/media storage
  - creates a durable video asset record
  - records model, provider, request parameters, prompt, revised/enhanced prompt, source image ids, reference image ids, source video id, and lineage

- additional routes:
  - `POST /v1/video/generations/{id}/cancel` — **does not exist today; add it** (local cancellation at minimum; Gemini has no operation-cancel, so flip status to `cancelled` and stop polling)
  - `POST /v1/video/assets/{id}/attach-to-conversation` — **already exists**
  - `GET /v1/video/assets/{id}/download` — **already exists**

> **Status enum reconciliation.** `video_generations.status` currently uses `pending | running | completed | failed | cancelled` (constants in `models.go`). Earlier drafts proposed `queued` and `polling` — those are **not** the generation states (`queued` is used only by `video_render_jobs`). Either reuse the existing five states or add `polling` deliberately as a migration with a backward-compatible CHECK constraint; do not assume it already exists.

The frontend should poll the local backend, not the Gemini API directly.

The backend must use encrypted Gemini provider credentials already configured in OmniLLM-Studio. Never expose API keys to the frontend.

Generated videos must be downloaded and saved locally because the provider’s hosted generated videos are temporary.

### 5. Storage and database

> **Critical reconciliation.** The tables below **already exist** (migrations V37–V39 in `db.go`). Do **not** create `video_sessions` or `video_generation_references` — those names do not exist and would duplicate/conflict with the real schema. **Extend the existing tables with `ALTER TABLE ... ADD COLUMN` (with defaults, per project migration rules)**; do not redefine them. The columns listed in earlier drafts are the *conceptual* shape — the mapping to real columns is given for each table.

#### `video_projects` (this is the "session"-equivalent — **exists**)

Real columns: `id`, `user_id`, `title`, `active_timeline_id`, `default_provider`, `default_model`, `width`, `height`, `fps`, `duration_ms`, `aspect_ratio`, `metadata_json`, `created_at`, `updated_at`. Conversation/workspace scoping is not currently modeled here — if scope is needed, add it as a new nullable column rather than a new table.

#### `video_generations` (**exists** — extend, don't recreate)

Real columns today: `id`, `project_id`, `parent_id` (self-FK; extension/branch lineage lives here, **not** on assets), `status` (`pending|running|completed|failed|cancelled`), `provider`, `model`, `prompt`, `enhanced_prompt`, `negative_prompt`, `settings_json`, `input_asset_ids_json`, `output_asset_id`, `upstream_job_id`, `upstream_request_id`, `usage_json`, `cost_usd`, `error`, `created_at`, `completed_at`.

Mapping of conceptual → real:
- `mode` → **does not exist; do not add** (capability-driven, see §3).
- `aspect_ratio` / `duration_seconds` / `resolution` / `person_generation` / `seed` → currently serialized into `settings_json`, not dedicated columns. Keep them in `settings_json` unless you have a query reason to promote them; if promoted, add as nullable columns with defaults.
- `provider_operation_name` → use the existing `upstream_job_id` (and `upstream_request_id`).
- `source_video_asset_id` / `start_image_attachment_id` / `last_frame_attachment_id` → not present. Either add nullable columns or extend `input_asset_ids_json` into a structured object that records each asset's role. **Decide one approach and apply it consistently** (a structured `input_assets_json` is preferred over re-introducing the dropped `video_generation_references` table).
- `error_message` → existing column is `error`. `request_json`/`response_json` → `settings_json` + `usage_json` already serve these roles; preserve raw provider response in `usage_json`/`metadata` rather than adding new columns.

#### reference / frame roles (no separate table)

Input/start/last/reference assets are referenced via `input_asset_ids_json` on `video_generations`. To capture roles (`start_frame | last_frame | reference_image`), store a structured JSON array of `{asset_id, role}` rather than adding a `video_generation_references` table.

#### `video_assets` (**exists** — extend, don't recreate)

Real columns: `id`, `project_id`, `source_type`, `source_studio`, `source_id`, `kind` (`video|image|audio|music|text|caption|export|other`), `file_name`, `file_path`, `mime_type`, `size_bytes`, `duration_ms`, `width`, `height`, `fps`, `thumbnail_path`, `waveform_path`, `provider`, `model`, `metadata_json`, `created_at`.

Mapping of conceptual → real: `storage_path` → `file_path`; `bytes` → `size_bytes`; `duration_seconds` → `duration_ms`; `has_audio` → not present (add nullable, or infer from metadata); `generation_id` → not a direct column (assets link to generations via `video_generations.output_asset_id`); `parent_asset_id` for extension lineage → **prefer `video_generations.parent_id`**, which already models lineage, unless you specifically need asset-to-asset chaining.

#### `video_timelines` and `video_render_jobs` (**exist — leave intact**)

These back the timeline editor and FFmpeg renderer. Do not modify them for the generation work unless extension lineage requires it.

Follow the existing repository/migration style. Keep paths safe with existing safe-join patterns.

### 6. Backend provider implementation

A direct Gemini Veo client **already exists** in `gemini_provider.go` and uses the REST `predictLongRunning` flow with a `geminiOperation` poll loop — extend it rather than starting over. The dependency question below is therefore largely settled (no SDK update needed; the REST flow is in place). The real work is widening it from text-to-video to the full input suite (image-to-video, first/last-frame, reference images, extension) and moving its inline poll/download into the async job worker from §4.

Requirements:
- Prefer direct Gemini API integration for Veo 3.1.
- Use existing provider profile/secret lookup.
- Keep provider-specific code isolated behind a service interface.
- Use context timeouts and cancellation.
- Add structured logging for operation name, model, mode, and generation id, without logging API keys.
- Handle provider errors with useful user-facing messages.
- Preserve raw provider response JSON for debugging.

If the current Go SDK supports `generate_videos` and operations polling cleanly, use it. If the existing dependency version does not support it, either update the dependency carefully or implement the documented REST `predictLongRunning` flow in an isolated Gemini Veo client.

### 7. Frontend UX

Add or enhance Video Studio UI consistent with the existing dark glassmorphism Studio design.

The Video Studio should include:

#### Generate panel

Controls:
- model selector
- mode selector
- aspect ratio selector: `16:9`, `9:16`
- duration selector: `4`, `6`, `8`
- resolution selector: `720p`, `1080p`, `4k`
- person generation selector where applicable
- seed input where applicable
- prompt textarea
- prompt enhance button
- audio/dialogue guidance helper
- generate button

Disable controls that are invalid for the selected model/mode and show a short reason.

#### Image inputs

For image-to-video / first-last-frame / reference-image modes:
- upload image
- choose from current conversation attachments
- choose from Image Studio assets
- choose from File Library images where available
- preview selected images
- remove/replace images

#### Results and history

Show:
- active jobs with status
- completed video cards
- failed jobs with retry
- generation metadata
- download button
- extend button where eligible
- branch/remix action if session history supports it

#### Capability hints

Add inline help so users understand:
- 4k has higher latency/cost
- extension is 720p only
- reference images are limited to three
- first/last frame is Veo 3.1-specific
- prompt audio cues can include dialogue, SFX, and ambient sound

### 8. Prompt enhancement

A prompt enhancer **already exists** but is **deterministic/template-based** (`prompt_enhancer.go` → `EnhancePrompt`, exposed via `POST /v1/video/enhance-prompt`). It wraps the prompt with scene/camera/lighting directives and a default negative prompt — it does **not** apply LLM reasoning or the full Veo prompt-guide structure. Upgrade it (or add an LLM-backed path via the existing `service.llm.ChatComplete`, with the deterministic version as fallback) to produce the structured cinematic prompts below. The assistant features (`assistant.go`) already show the LLM-with-deterministic-fallback pattern to follow.

The enhancer should transform rough user prompts into structured cinematic prompts with:

- subject
- action
- style
- camera position and movement
- composition
- focus/lens effects
- ambiance/lighting/color
- audio cues:
  - dialogue in quotes
  - sound effects
  - ambient noise
- continuity notes for image-to-video, first/last frame, and extension modes

Do not overwrite the user’s original prompt. The `video_generations` table already has `prompt` and `enhanced_prompt` columns; there is **no** "whether enhanced was used" flag — record that in `settings_json` (e.g. `enhance_applied: true`) rather than adding a column.

Provide a preview/diff or “Use enhanced prompt” action.

### 9. Cross-studio integration

A cross-studio **import** path already exists: `service.ImportExternalAsset` + `POST /v1/video/projects/{projectId}/assets/import` accept `source_studio` values `file_library | image_studio | music_studio`, copy bytes into video storage, and stamp `source_studio`/`source_id` on the `video_assets` row. Reuse this — the remaining work is mostly the **frontend "Send to Video Studio" affordance from the Image Studio UI** and routing the imported asset into the correct generation input role (start frame vs reference). Confirm whether the Image Studio component already exposes this button before adding a new one.

Flow:
- user generates or selects an image in Image Studio
- user clicks “Send to Video Studio”
- Video Studio opens with that image selected as:
  - starting frame by default
  - reference image option if the user chooses
  - first frame in first/last-frame mode

Reuse existing attachment/asset records where possible instead of duplicating files unnecessarily. If duplication is necessary, preserve metadata linking back to the source image asset.

### 10. Validation and safety

Implement validation on both frontend and backend.

Backend validation is mandatory even if frontend disables invalid controls.

Handle:
- unsupported model/mode combinations
- too many reference images
- missing required image inputs
- invalid aspect ratio
- invalid duration/resolution combination
- extension attempted against non-extendable videos
- deleted/missing source assets
- provider API errors
- safety-filter blocks
- audio-related generation blocks
- timeout/polling errors
- app restart while jobs are in progress

On app startup or first access to Video Studio, pending/running jobs should be recoverable by polling stored provider operation ids.

### 11. Testing

Add tests where project conventions support them.

Existing backend tests live in `backend/internal/video/*_test.go` (`model_registry_test.go`, `provider_adapters_test.go`, `renderer_test.go`, `service_external_import_test.go`, `timeline_test.go`) — extend these. Minimum new backend tests:
- capability matrix validation
- request validation per model + input combination (there is no `mode`; validate by capability)
- first/last-frame requires a start image
- reference image max count (3)
- extension disabled for Lite
- 4k disabled for Fast and Lite
- 1080p/4k/extension/reference-images force duration = 8
- extension resolution forced/validated to 720p
- generated video asset persistence
- failed provider operation stores error state (`status='failed'`, `error` populated)
- async: a generation persists `upstream_job_id` and is recoverable after restart

Frontend: **there is no frontend unit-test framework in this repo** (per CLAUDE.md — only root-level Playwright smoke tests for the image editor). Prefer type-level checks and, if warranted, a new Playwright smoke spec. Cover:
- invalid controls disabled based on selected model + capabilities
- request payload shape is correct (matches `GenerateRequest`)
- Image Studio asset handoff populates the right Video Studio input state

Also run:
- Go tests
- TypeScript check
- frontend build
- any existing lint/build scripts

### 12. Documentation

Update README or docs with:

- Video Studio overview
- supported Gemini Veo models
- supported modes
- setup instructions for Gemini API key
- limitations and known provider constraints
- examples:
  - text-to-video with dialogue
  - image-to-video from Image Studio
  - first/last-frame transition
  - reference-image product/character generation
  - video extension

### 13. Implementation discipline

Before making changes:
1. Summarize the existing Video Studio state.
2. Identify files to modify.
3. Identify new files/migrations needed.
4. Explain any dependency update if needed.

While coding:
- Keep changes incremental and cohesive.
- Prefer existing project patterns over new abstractions.
- Do not expose secrets to frontend.
- Do not block HTTP requests for long-running Veo generation.
- Do not fake completion or return remote video URLs without downloading/storing the MP4 locally.
- Do not remove existing Image Studio or Music Studio behavior.
- Do not break chat, RAG, file library, artifact export, or provider settings.

After coding:
1. Summarize what changed.
2. List new routes/types/tables/components.
3. List known limitations.
4. Provide manual test steps.
5. Provide exact commands run and results.

## Acceptance criteria

The implementation is complete when:

- Video Studio can submit a Gemini Veo 3.1 text-to-video job and show async progress.
- Completed videos are downloaded from Gemini and stored locally as durable assets.
- User can download/play generated MP4s from the app.
- User can choose aspect ratio, duration, and resolution with model-aware validation.
- User can generate image-to-video from an uploaded image or Image Studio asset.
- User can generate first/last-frame Veo 3.1 interpolation.
- User can provide up to three reference images where supported.
- User can extend eligible prior Veo videos.
- Frontend never sees Gemini API keys.
- Pending jobs can recover after app restart using stored operation ids.
- Prompt enhancer creates stronger Veo-style prompts without destroying the original user prompt.
- Docs and tests are updated.

---

## Implementation status (updated 2026-06-02)

### ✅ Completed

**Phase 1 — Model capabilities & lite model**
- `backend/internal/video/types.go`: added `CapabilityFirstLastFrame` and `CapabilityPersonGeneration` constants.
- `backend/internal/video/gemini_provider.go`: added `veo-3.1-lite-generate-preview`; per-model capability sets via `geminiVeoCapabilitiesForID()` now expose the full verified matrix (lite has no 4k, no extension; fast has no 4k).

**Phase 2 — GenerateRequest extensions**
- `backend/internal/video/types.go`: added `PersonGeneration`, `StartImageAssetID`, `LastFrameAssetID`, `SourceVideoAssetID` to `GenerateRequest`; added `InputAsset` struct with `Role` enum; added `GenerateAsyncResponse`.

**Phase 3 — DB migration V40**
- `backend/internal/db/db.go`: migration V40 adds `input_assets_json TEXT NOT NULL DEFAULT '[]'` to `video_generations` (structured `{asset_id, role}` JSON array).
- `backend/internal/models/models.go`: `InputAssetsJSON` field added to `VideoGeneration`.

**Phase 4 — Repository updates**
- `backend/internal/repository/video_generation_repo.go`: `Create()` writes `input_assets_json`; scan includes it; new `SetUpstreamJobID()` and `ListActiveWithUpstreamJob()` methods.

**Phase 5 — Gemini provider full input suite**
- `backend/internal/video/gemini_provider.go`: new public methods `Submit()` (async only), `PollOnce()`, `DownloadVideo()`; `buildPayload()` handles all input modes (start frame, first/last frame, source video extension, reference images); validates max 3 references, forces 720p for extension, enforces duration=8 for 1080p/4k/extension/references.

**Phase 6 — LLM prompt enhancer**
- `backend/internal/video/prompt_enhancer.go`: `EnhancePromptWithLLM()` calls `llm.ChatComplete` with a structured Veo-style system prompt; falls back to deterministic `EnhancePrompt()` if LLM is unavailable or returns empty.
- `backend/internal/video/service.go`: `EnhancePrompt(ctx, req)` method on `Service` for handler use.

**Phase 7 — Async generation lifecycle**
- `backend/internal/video/service.go`: `GenerateAsync()` creates generation record, starts background goroutine, returns immediately (202); `runGenerationBackground()` resolves paths, calls `Submit()`, persists operation name via `SetUpstreamJobID()`, then calls `pollAndFinalize()`.
- `pollAndFinalize()`: 120 × 10s attempts → `PollOnce()` → `DownloadVideo()` → store bytes → `VideoAsset` record → `MarkCompleted()`.
- `CancelGeneration()`: validates status, calls `MarkCancelled()`.
- `RecoverPendingGenerations()`: re-spawns poll goroutines for in-flight jobs on startup.

**Phase 8 — Handler and router**
- `backend/internal/api/video_handler.go`: `Generate()` replaced SSE handler with async 202 JSON response; `CancelGeneration()` added; `EnhancePrompt()` now calls `h.service.EnhancePrompt()` (LLM path).
- `backend/internal/api/router.go`: `POST /video/generations/{generationId}/cancel` route added; `go videoService.RecoverPendingGenerations()` called at startup.

**Phase 9 — Frontend**
- `frontend/src/types/video.ts`: `VideoCapability` extended with `first_last_frame` and `person_generation`; `InputAsset` interface added; `GenerateVideoRequest` extended with `start_image_asset_id`, `last_frame_asset_id`, `source_video_asset_id`, `person_generation`; `VideoPromptForm` extended with same optional fields; `input_assets_json` added to `VideoGenerationDetail`.
- `frontend/src/api.ts`: `videoApi.generate()` converted from SSE callback pattern to simple `apiFetch` returning `{ generation_id, project_id, status, generation }`; `videoApi.cancelGeneration()` added; unused SSE callback types removed.
- `frontend/src/stores/videoStudio.ts`: `generate()` replaced with async polling implementation (POST → immediate 202 → `setInterval` polling `getGeneration()` every 8s until terminal state); `cancelGeneration()` added (stops poll interval, calls API, updates generation status); `stopGeneration()` updated to clear poll interval.
- `frontend/src/components/video/VideoStudio.tsx`: Start frame, last frame, source video, and person generation controls added (capability-gated); Cancel button replaces Stop button.

### ✅ Completed (continued — session 3 wrap-up)

**Asset pickers (start frame / last frame / source video)**
- `VideoStudio.tsx`: text inputs replaced with `AssetPicker` dropdown component that filters project assets by kind (`image` for start/last/reference, `video` for source). Inline hint text explains each constraint (720p for extension, start-frame required for last-frame, etc.).

**Reference images UI**
- `frontend/src/types/video.ts`: `reference_asset_ids?: string[]` added to `VideoPromptForm`.
- `VideoStudio.tsx`: When the selected model has `reference_images` capability, up to 3 `AssetPicker` dropdowns are rendered with a live count and a hint about the 8s duration requirement.

**Extend button on completed generations**
- `VideoStudio.tsx` `HistoryPanel`: added `onExtend` prop and an "Extend" button rendered for every `completed` generation that has an `output_asset_id`. Clicking it pre-fills `source_video_asset_id` in the prompt form and shows a toast. The extend video controls (source video picker + 720p/8s hints) then appear in the Create panel based on model capability.

**Image Studio → Video Studio handoff (start frame)**
- `frontend/src/stores/index.ts`: extended `to-video` crossover context data type with `attachmentId?: string`.
- `ImageEditStudio.tsx` `handleGenerateVideo`: now passes `canvasAttachmentId` (the currently displayed image's attachment ID) alongside the translated video prompt.
- `VideoStudio.tsx`: two `useEffect`s — the first reads crossover context, pre-fills the prompt, and stores the attachment ID in a ref; the second fires when `activeProjectId` becomes available and calls `videoApi.importExternalAsset(projectId, { source_studio: 'attachment', source_id: attachmentId, kind: 'image' })`, sets `start_image_asset_id` to the resulting video asset ID, and reloads project assets.

### ⬜ Remaining / not yet implemented

- **Playwright smoke test**: No new smoke test covering async generation flow.
- **Docs update**: README / Video Studio docs not yet updated with new capabilities.
