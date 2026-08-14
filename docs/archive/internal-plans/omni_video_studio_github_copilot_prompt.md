> **Archived — historical implementation prompt.** It describes an earlier build stage; current open Video work is consolidated in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# GitHub Copilot Implementation Prompt: OmniLLM-Studio Video Studio

You are working in the repository:

```text
https://github.com/ajbergh/OmniLLM-Studio
```

Always inspect the current `main` branch before making changes. This project is a local-first Go + React/TypeScript application with existing first-class studios for Chat, Image, and Music. Implement a first-class **Video Studio** and **Video Edit Studio** that fit the current architecture, UI conventions, storage model, provider model, and cross-studio workflows.

## Primary Goal

Add/refactor the video experience in OmniLLM-Studio into two separate, focused workspaces:

1. **Video Studio: AI Video Creation Suite**
   - Multi-provider AI video generation.
   - Prompt input and prompt enhancement.
   - Provider/model/capability discovery.
   - Generation history and branching.
   - Durable generated video assets.
   - Cross-studio import/export with Image Studio, Music Studio, Chat, and File Library.
   - Single-output preview and download controls.
   - No timeline editor, media bin, inspector, or render panel in this workspace.

2. **Video Edit Studio: Full Timeline Editing Suite**
   - Camtasia-like timeline editing experience.
   - Multi-track video/audio/image/text/caption editing.
   - Preview canvas.
   - Inspector panel.
   - Clip trim/split/move/delete.
   - Effects, transitions, overlays, and audio controls over time.
   - Render/export pipeline.

The target UX should feel like a natural set of first-class studios beside Chat Studio, Image Studio, and Music Studio. Video Studio should stay focused on creating a single generated video. Timeline editing, rendering, and multi-asset composition belong in the separate Video Edit Studio.

---

# Implementation Progress

Last updated: 2026-06-01 (mock provider, mock renderer, and stale embedded bundle removed)

| Phase | Status | Progress | Notes |
| --- | --- | --- | --- |
| Mock Provider / Renderer Removal | ✅ Complete | Verified against code search, `go test ./internal/video/...`, `go test ./...`, and `npm run build`. | Removed the video mock provider and mock renderer paths from backend and frontend. `video.NewService()` now registers OpenRouter and Gemini providers only. Frontend defaults now use real provider keys and disable generation until a configured provider profile exists. Renderer tests now exercise FFmpeg output instead of a mock renderer. The embedded desktop frontend bundle was refreshed from the current build so stale mock defaults are not shipped. |
| Product Scope Pivot: Video Creation / Video Edit Split | ✅ Complete | Verified with `npx tsc -b`, `npm run build`, and a browser smoke test. | `VideoStudio.tsx` is now a creation-focused panel. Timeline, media-bin, preview-canvas, inspector, assistant-editing, and render/export UI moved into `VideoEditStudio.tsx`, reachable through new `appMode === 'video-edit'`. Both tabs share the existing `video_studio` feature flag and project store. |
| Phase 0: Technical Spike / Foundation | ✅ Complete | Verified; updated for split. | `video_studio` feature flag seeded in V37 migration. `/v1/video` route family wired in `router.go`. `App.tsx` now renders both `appMode === 'video'` (`<VideoStudio />`) and `appMode === 'video-edit'` (`<VideoEditStudio />`). Both are feature-gated in `Sidebar.tsx` (`isEnabled('video_studio')`) and toggleable in `SettingsPanel.tsx`. |
| Phase 1: AI Video Generation MVP | ✅ Complete | Verified. | `video_projects`, `video_generations`, `video_assets` tables in V37. Repositories and service layer wired. SSE generation stream uses `video_generation_started`, `video_generation_progress`, `video_generation_done`, `video_generation_error` events. Frontend `VideoStudio.tsx` has full generate-controls panel including provider/model selector, prompt, enhancer, aspect ratio, duration, resolution, FPS, seed, negative prompt, and cancel. History and assets panels present. Real generation requires OpenRouter or Gemini provider profiles; no local mock generation fallback remains. |
| Phase 2: Timeline Editing MVP | ✅ Complete | Verified; moved to Video Edit Studio. | V38 migration (`video_timelines`). `GET/PUT /timeline` and `/timeline/import-asset` routes implemented. `VideoTimeline.tsx`, `TimelineTrack.tsx`, `TimelineClip.tsx`, `TimelineRuler.tsx`, `TimelinePlayhead.tsx`, `TimelineToolbar.tsx` all present and now surfaced from `VideoEditStudio.tsx`, not the creation-focused `VideoStudio.tsx`. Preview canvas in `VideoPreviewCanvas.tsx`; inspector in `VideoInspector.tsx`; Zustand timeline state in `videoStudio.ts`. **Caveat: no waveform rendering for audio tracks. No thumbnail strip rendering for video clips.** |
| Phase 3: Render / Export MVP | ✅ Complete | Verified; moved to Video Edit Studio. | V39 migration (`video_render_jobs`). `VideoRenderPanel.tsx` and `RenderJobStatus.tsx` are now reachable from `VideoEditStudio.tsx`. FFmpeg renderer reads actual video/image/audio asset files from disk, composites them via FFmpeg `filter_complex` (overlay/scale/amix), and renders text/caption/callout clips as `drawtext`. Falls back to text-only when no media files present. Export settings support format, quality, resolution, audio toggle. |
| Phase 4: Cross-Studio Workflows | ✅ Complete | Verified. | Crossover routes extended: `image→video`, `music→video`, `chat→video`, `video→image`, `video→music` all present in `crossover_handler.go`. Frontend `api.ts` has `imageToVideo`, `musicToVideo`, `chatToVideo`, `videoToImage`, `videoToMusic` typed methods. Backend `ImportExternalAsset` copies real bytes from File Library, Music Studio, and Image/attachment sources. **Image Studio**: `handleGenerateVideo` calls `crossoverApi.translate.imageToVideo()` then sets crossover context to `to-video` and switches app mode. **Music Studio**: `handleGenerateVideo` calls `crossoverApi.translate.musicToVideo()` then sets crossover context to `to-video` and switches app mode. Both studios have working Video Studio crossover buttons. |
| Phase 5: Camtasia-Style Editing Polish | ✅ Complete | Verified; moved to Video Edit Studio. | Timeline neutral model supports effects, transitions, fades, volume, transforms, keyframes, and text/caption-style clips in both Go types (`timeline.go`) and the Zustand store. Inspector UI (`VideoInspector.tsx`) surfaces clip properties including effects, transitions, fades, volume, and text editing inside `VideoEditStudio.tsx`. Add/remove/toggle effects. Add transitions (fade, crossfade, slide, wipe, zoom). Add text clips. Volume envelope and fade in/out. Keyframes for transform/opacity/volume. **Remaining polish: effects/transitions/fade/keyframe data is persisted but not applied during FFmpeg render. Inspector shows a tooltip: "Advanced effects not yet rendered in export."** |
| Phase 6: AI-Assisted Editing | ✅ Complete (with caveats) | Verified; moved to Video Edit Studio. | `assistant.go` now imports `*llm.Service` and implements LLM-backed storyboard + edit plan generation with deterministic fallback. `CreateStoryboard` calls `s.llm.ChatComplete()` with a structured system prompt and JSON response parsing. `CreateEditPlan` calls `s.llm.ChatComplete()` with JSON schema instructions. Both fall through to the deterministic keyword-matching path on LLM error. `CreateSocialVariants` remains rule-based (returns the same three hardcoded aspect-ratio plans). Routes present. Frontend `VideoInspector.tsx` surfaces storyboard and edit-plan controls in Video Edit Studio. |
| Phase 7: Testing, Hardening, and Documentation | 🔄 In progress | 9 video tests passing; known gaps remain. | All 9 video package tests pass (`go test ./internal/video/...`). Full backend suite passes (`go test ./...`). Frontend build clean (`npm run build`, including `npx tsc -b`). `TestFFmpegRendererProducesVideoAsset` validates real FFmpeg output bytes by checking the MP4 `ftyp` box. **Remaining gaps: no end-to-end smoke test covers the render/export UI flow, and no integration test covers the crossover UI buttons.** |
| Provider Phase A: Real Video Provider Adapters | ✅ Complete | Verified against httptest servers. | OpenRouter: real submit/poll/download HTTP lifecycle, model discovery with fallback snapshot. Gemini: `predictLongRunning` operation poll and download. Provider profiles (encrypted API keys) auto-configure both providers. Gemini model list now queries `/v1beta/models?pageSize=100` at runtime filtering to `veo`-named models supporting `predictLongRunning`, with a static fallback snapshot. **Gemini reference image (image-to-video) is fully wired**: reads asset file from disk, base64-encodes it, and embeds into the Gemini prediction payload as `instances[0].image`. OpenRouter's `/videos/models` discovery endpoint may not exist; falls back to the built-in May 2026 snapshot. |
| Render Phase A: FFmpeg Export | ✅ Complete | Verified. | FFmpeg renderer creates a real MP4/WebM file with the correct canvas dimensions, frame rate, duration, and optional audio. `resolveMediaClips()` iterates the timeline tracks and reads actual video/image/audio asset files from disk using `os.Stat` + `filepath.Join(req.AttachmentsDir, ...)`. `buildFilterComplex()` constructs an FFmpeg `filter_complex` expression compositing video/image clips via overlay/scale/pad filters and mixes audio via `amix`. Text/caption/callout clips render as `drawtext` overlays. Falls back gracefully to text-only when no media files are present on disk. |
| Cross-Studio Import Phase A: Real Asset Copy | ✅ Complete | Verified by test. | `ImportExternalAsset` copies real bytes from File Library records, Music Studio assets, and Image/attachment sources. Tests `TestImportExternalAssetCopiesFileLibraryBytes` and `TestImportExternalAssetCopiesMusicBytes` pass with real SQLite and temp file fixtures. Placeholder `.ref.txt` path anti-pattern was confirmed absent. Image Studio and Music Studio both have working "Send to Video" buttons that call the crossover/translate API and set crossover context. |

## Current Verification State (as of 2026-06-01 mock-removal work)

| Check | Result |
| --- | --- |
| Video Studio / Video Edit Studio UI split | ✅ Implemented and verified (`npx tsc -b`, `npm run build`, browser smoke test) |
| `go test ./internal/video/...` (9 tests) | ✅ All pass |
| `go test ./...` (all backend packages) | ✅ All pass |
| `npm run build` | ✅ Clean (chunk-size warning only) |
| `npx tsc -b` | ✅ No type errors |
| Video mock provider | ✅ Removed (`video.NewService()` registers OpenRouter and Gemini only) |
| Video mock renderer | ✅ Removed; render/export tests exercise FFmpeg |
| Embedded desktop frontend bundle | ✅ Refreshed from current `frontend/dist`; stale mock defaults removed |
| FFmpeg composites actual video clips | ✅ Implemented (`resolveMediaClips` + `buildFilterComplex`) |
| Image/Music Studio "Send to Video" UI | ✅ Implemented (both studio components have crossover buttons) |
| LLM-backed assistant (storyboard + edit plan) | ✅ Implemented (with deterministic fallback; social variants still rule-based) |
| Gemini image-to-video / reference assets | ✅ Implemented (base64-encoded image in Gemini payload) |
| Gemini live model discovery | ✅ Implemented (`/v1beta/models` API with static fallback) |
| Preview canvas real video playback | ✅ Implemented (rAF loop, `<video>` element, play/pause/scrub) |
| Video-to-Chat / Register-in-Library buttons | ✅ Implemented (AssetPanel `MessageSquare` + `BookMarked` icons) |
| Text asset preview | ✅ Implemented (generic text/unsupported asset card; no mock-specific placeholder state) |
| Duplicate playback loop conflict | ✅ Fixed (setInterval removed from VideoTimeline; single rAF in VideoPreviewCanvas) |
| Timeline waveform / thumbnail rendering | ❌ Not implemented |

---

# ⚠️ Next Steps: What Is Missing or Not Implemented to Spec

## Removed Mock / Development-Only Paths

All Video Studio mock provider and mock renderer paths were removed on 2026-06-01:

- `backend/internal/video/provider.go`: removed the local development provider implementation and model.
- `backend/internal/video/service.go`: provider registration now includes OpenRouter and Gemini only.
- `backend/internal/video/renderer.go`: removed the package-local development renderer.
- `frontend/src/types/video.ts`, `frontend/src/stores/videoStudio.ts`, and `frontend/src/components/SettingsPanel.tsx`: removed mock provider keys/defaults and now require a real configured provider for generation.
- Tests that depended on development provider/renderer behavior now use real provider snapshots, OpenRouter/Gemini HTTP fixtures, or FFmpeg-backed rendering.

Video generation now requires OpenRouter or Gemini provider credentials. If neither is configured, the UI keeps generation disabled and shows a provider-configuration prompt.

## 🔴 Previously Critical Gaps (now resolved)

### 1. FFmpeg renderer does not composite actual media clips — ✅ RESOLVED

The FFmpeg renderer now reads actual video/image/audio asset files from disk via `resolveMediaClips()` and composites them via `buildFilterComplex()` which constructs an FFmpeg `filter_complex` expression. Video/image clips are positioned with overlay/scale/pad filters. Audio clips are mixed via `amix`. Text/caption/callout clips render as `drawtext` overlays. Falls back gracefully to text-only if no media files are present on disk.

---

### 2. No "Send to Video" buttons in Image Studio or Music Studio — ✅ RESOLVED

Image Studio (`ImageEditStudio.tsx`) has `handleGenerateVideo` that calls `crossoverApi.translate.imageToVideo()` then sets crossover context to `'to-video'` and switches app mode. Music Studio (`MusicStudio.tsx`) has `handleGenerateVideo` that calls `crossoverApi.translate.musicToVideo()` then sets crossover context to `'to-video'` and switches app mode. Both also have `Video` icon import for a dedicated button.

---

### 3. AI assistant is entirely rule-based — ✅ PARTIALLY RESOLVED (LLM-backed with deterministic fallback)

The `Service` struct in `assistant.go` now imports `*llm.Service` and implements LLM-backed storyboard + edit plan generation via `s.llm.ChatComplete()`. `CreateStoryboard` uses a structured director/JSON system prompt and parses the LLM response. `CreateEditPlan` uses an editor/JSON schema system prompt. Both fall through to the deterministic keyword-matching path on LLM error. `CreateSocialVariants` remains rule-based (same three hardcoded plans).

---

### 4. Gemini image-to-video and reference assets not wired — ✅ RESOLVED

`GeminiProvider.Generate()` now handles `len(req.ReferenceAssetPaths) > 0`: it calls `readReferenceImage()` to read the file, detects MIME type (JPEG/PNG/WebP/GIF), base64-encodes the bytes, and embeds them into the Gemini `predictLongRunning` payload as `instances[0].image.bytesBase64Encoded` and `instances[0].image.mimeType`. Gracefully skips if reference asset is unavailable.

---

### 5. Gemini model list is hardcoded; no live discovery — ✅ RESOLVED

`ListModels()` now queries `GET /v1beta/models?pageSize=100` at runtime with live pagination, filtering to models containing `"veo"` and supporting the `predictLongRunning` method. Falls back to `KnownGeminiVeoModels()` static snapshot on any error or empty response.

---

## 🟡 Remaining Polish / UX Gaps (still outstanding)

### 6. No waveform / thumbnail rendering on timeline

There is no waveform rendering for audio tracks and no thumbnail strip for video clips on the timeline. Audio clips show as generic colored bars. Video clips show as labeled blocks.

### 7. Social variants still rule-based

`CreateSocialVariants` returns the same three hardcoded aspect-ratio plans (vertical 9:16, square 1:1, widescreen 16:9) every time, regardless of prompt content.

### 8. Effects/transitions/keyframes data is persisted but silently ignored during export

Effects, transitions, fades, brightness, opacity, transform, and keyframe data round-trip correctly through the timeline JSON and are visible in the inspector, but the FFmpeg renderer ignores them entirely. Users will see no difference in the export between a clip with a fade-in and one without. The inspector shows a tooltip: "Advanced effects not yet rendered in export."

### 9. Timeline plan is still deterministic

`CreateTimelinePlan` always returns a fixed scaffold (set_duration 30s + add_text_clip) regardless of prompt. Only `CreateStoryboard` and `CreateEditPlan` have been wired to the LLM.

---

# Historical Implementation Notes

The sections below preserve the original implementation prompt and phase guidance. They include earlier instructions to start with mock/stub/placeholder scaffolding. Treat the progress tables and "Removed Mock / Development-Only Paths" section above as the current source of truth for what is implemented today.

---

# Existing Architecture to Follow

Before implementation, inspect these areas:

```text
frontend/src/App.tsx
frontend/src/components/Sidebar.tsx
frontend/src/components/image/ImageEditStudio.tsx
frontend/src/components/music/MusicStudio.tsx
frontend/src/stores/imageEditor.ts
frontend/src/stores/musicStudio.ts
frontend/src/types.ts
frontend/src/types/music.ts
frontend/src/api.ts
frontend/src/components/SettingsPanel.tsx
backend/internal/api/router.go
backend/internal/api/image_session_handler.go
backend/internal/api/music_handler.go
backend/internal/api/crossover_handler.go
backend/internal/music/service.go
backend/internal/repository/music_session_repo.go
backend/internal/db/db.go
backend/internal/models/models.go
backend/internal/llm/service.go
backend/internal/filelibrary
```

Important existing patterns (verified against `main`):

- `App.tsx` switches studios via `appMode`. The current `appMode` union is `'chat' | 'image' | 'music' | 'video' | 'video-edit'` and is defined in **`frontend/src/stores/index.ts`** inside the `SettingsState` interface, the `setAppMode` signature, and the `getInitialAppMode()` helper (which persists to `localStorage` under the key `omnillm_app_mode`). There is **no** separate `frontend/src/stores/settings.ts` — the settings store lives in `stores/index.ts` as `useSettingsStore`.
- `Sidebar.tsx` contains the studio mode switcher and studio-specific session lists. It gates Music Studio with `const musicStudioEnabled = features.length === 0 || isEnabled('music_studio')` and force-resets `appMode` to `'chat'` if a disabled studio is active.
- Music Studio uses a standalone top-level route family under `/v1/music/...`. Concretely: `GET /music/providers`, `GET /music/models`, `POST /music/models/refresh`, a nested `r.Route("/music/sessions", ...)` group with `{sessionId}` sub-routes (incl. `/generations`), then top-level `POST /music/generations`, `GET /music/generations/{generationId}`, `POST /music/generations/{generationId}/branch`, and `/music/assets/{assetId}` routes (`GET`, `/download`, `DELETE`, `/attach-to-conversation`). Mirror this layout for video.
- Image Studio has session-level routes under conversation-backed image sessions (`/conversations/{id}/images/sessions/...`) plus standalone `/images/sessions` routes.
- Music Studio has provider/model/session/generation/asset concepts (`MusicSession`, `MusicGeneration`, `MusicAsset` in `models/models.go`) that should heavily influence Video Studio.
- Cross-studio translation exists via `POST /v1/crossover/translate` (`crossover_handler.go`) and now includes video source/target paths (`image→video`, `music→video`, `chat→video`, `video→image`, `video→music`). The frontend client is `crossoverApi.translate` in `frontend/src/api.ts` with `xToY`-named methods.
- File Library already provides durable asset indexing/storage semantics.
- API keys are stored as encrypted provider profiles; do not expose secrets to the frontend.
- SQLite migrations are versioned inside `backend/internal/db/db.go` via the `versionedMigrations()` slice. Video Studio uses migrations **37** (`video_studio_foundation`), **38** (`video_studio_timelines`), and **39** (`video_studio_render_jobs`). New migrations must increment from the current latest version.
- Feature flags already exist and Music Studio is feature-gated. Flags are **seeded inside a migration** with `INSERT INTO feature_flags (key, enabled, metadata) VALUES ('music_studio', 1, '{"label":"Music Studio","description":"..."}')` (see `db.go`). There is no in-code default-flags registry. Frontend gating uses `isEnabled('music_studio')` in `Sidebar.tsx`, with a toggle in `SettingsPanel.tsx`.

Use the existing conventions for:

- Zustand stores.
- TypeScript API client methods.
- Toast notifications.
- Tailwind classes and visual design.
- Go service/repository/handler layering.
- SSE streaming for long-running generation jobs. Music writes raw SSE frames directly (`event: <name>\n` then `data: <json>\n\n`) — follow that exact framing, not a higher-level helper.
- Local-first file storage under the configured attachments directory. The attachments root is `cfg.AttachmentsDir` (env `OMNILLM_ATTACHMENTS_DIR`). Music roots its `Storage` at `filepath.Join(attachmentsDir, "music")` and writes to `music/<sessionId>/<generationId>/<file>` via a `safeJoin` path-traversal guard. Mirror this for video: root the video `Storage` at `filepath.Join(attachmentsDir, "video")` and use the same `safeJoin` guard. The video service constructor should accept `attachmentsDir string` like `music.NewService`.

---

# Product Design Direction

The video experience now has two top-level workspaces with a shared project store:

## 1. Video Studio: Generate

For focused AI video creation.

Controls:

- Provider selector.
- Model selector.
- Capability-aware control rendering.
- Prompt field.
- Prompt enhancer.
- Negative prompt, if supported.
- Aspect ratio.
- Duration.
- Resolution.
- FPS if supported.
- Seed if supported.
- Reference image(s), if supported.
- Reference video, if supported.
- Camera motion.
- Shot type.
- Style preset.
- Lighting/style/production notes.
- Generate button.
- Stop/cancel button.
- Generate directly to project outputs.
- Preview, download, branch, and open the selected output in Video Edit Studio.

Prompt enhancer should transform short prompts into structured cinematic prompts, for example:

```text
Subject:
Scene:
Action:
Camera:
Lighting:
Style:
Duration:
Aspect ratio:
Negative prompt:
```

Video Studio should not render the timeline, multi-asset media bin, clip inspector, assistant-edit controls, or render/export panel.

## 2. Video Edit Studio: Timeline

Camtasia-like editor.

Layout:

- Left panel: asset bin, generated assets, imported Image Studio assets, imported Music Studio tracks, File Library imports, text/caption/effects library.
- Center panel: preview canvas with playback controls, safe guides, fit/fill, zoom, frame/time display.
- Right panel: inspector for selected asset/clip/track/timeline.
- Bottom panel: multi-track timeline with ruler, clips, waveforms, thumbnails, playhead, zoom controls, snapping, split/trim tools.

Core editing interactions:

- Add asset to timeline.
- Move clip on track.
- Trim clip start/end.
- Split clip at playhead.
- Delete clip.
- Duplicate clip.
- Track mute/solo/lock/visibility.
- Clip volume.
- Clip opacity.
- Clip transform: x/y/scale/rotation/crop.
- Basic fades.
- Timeline save/load.
- Render/export from the active timeline.

## Shared Assets / History

The shared project data combines:

- Generation history tree.
- Project media bin.
- Imported assets.
- Rendered exports.

Video Studio shows generation history and generated outputs only. Video Edit Studio owns the media bin, imported assets, timeline placement, and rendered exports.

Each generated clip should preserve:

- Prompt.
- Enhanced prompt.
- Provider.
- Model.
- Parent generation ID.
- Input references.
- Output asset ID.
- Cost/usage, if available.
- Status/error.
- Created/completed timestamps.

Actions:

- Branch from generation.
- Regenerate.
- Send to timeline.
- Download asset.
- Send to Chat.
- Register in File Library.

---

# Technical Principles

## Use a Native OmniLLM Video Data Model

Do **not** persist a third-party timeline/editor model directly. Persist an OmniLLM-native neutral timeline JSON schema. This allows the app to use Remotion, Twick, designcombo, FFmpeg, or another renderer later without rewriting saved projects.

## Separate Generation History from Timeline State

Generation history answers: “How was this AI clip created?”

Timeline state answers: “How is this video project composed?”

They are related but must remain separate data models.

## Treat Video as Projects, Not Sessions

Use these core entities:

```text
VideoProject
VideoGeneration
VideoAsset
VideoTimeline
VideoRenderJob
```

Do not call them “video sessions” in backend models unless required temporarily for compatibility.

## Capability-Driven Provider UI

Video generation providers will differ dramatically. The frontend must render controls based on backend-provided capabilities rather than hard-coded assumptions.

Capability examples:

```text
text_to_video
image_to_video
video_to_video
extend_video
reference_images
reference_video
negative_prompt
seed
camera_motion
duration_range
aspect_ratios
resolutions
fps_options
audio_generation
watermark_behavior
async_polling
max_prompt_chars
```

## Keep Preview and Export Separate

- Interactive preview should use browser-native playback, React state, HTML video/audio, canvas/SVG overlays, and cached thumbnails/waveforms.
- Final export should use a backend render job and a renderer adapter.
- Start with a Remotion-compatible render adapter or a stubbed render adapter that can later call Remotion/FFmpeg.

---

# Phase 0: Technical Spike / Foundation

Implement the minimal structural skeleton to prove that Video Studio can exist as a first-class mode.

## Backend

1. Add feature flag:

```text
video_studio
```

Follow the existing Music Studio feature-flag pattern: seed it inside the new versioned migration (alongside the table DDL, or in its own migration) with:

```sql
INSERT INTO feature_flags (key, enabled, metadata)
VALUES ('video_studio', 1, '{"label":"Video Studio","description":"AI video creation and Video Edit Studio availability."}')
ON CONFLICT(key) DO NOTHING;
```

There is no in-code default-flags registry to update — seeding happens via migration. Gate both video workspaces on the frontend with `isEnabled('video_studio')` in `Sidebar.tsx` and add a toggle in `SettingsPanel.tsx`, mirroring `music_studio`.

2. Add placeholder backend package:

```text
backend/internal/video/
  service.go
  models.go
  storage.go
  timeline.go
  provider.go
```

3. Add placeholder API handler:

```text
backend/internal/api/video_handler.go
```

4. Add placeholder routes under `/v1/video`:

```text
GET  /v1/video/providers
GET  /v1/video/models
GET  /v1/video/projects
POST /v1/video/projects
```

5. Add empty provider capability registry with at least a mock/dev provider.

The mock/dev provider should allow development without a paid video API key. It can return a sample local test asset or generated placeholder metadata.

## Frontend

1. Extend the `appMode` union to include `'video'`. This union is **not** in a `settings.ts` file — it lives in **`frontend/src/stores/index.ts`** and must be updated in three places: the `SettingsState.appMode` field, the `setAppMode` parameter type, and the `getInitialAppMode()` return type/body (add a `if (saved === 'video') return 'video';` branch). Video projects are **not** conversation-backed, so do **not** add `'video'` to `ConversationKind` in `types.ts`.

```ts
appMode: 'chat' | 'image' | 'music' | 'video';
```

2. Add:

```text
frontend/src/components/video/VideoStudio.tsx
frontend/src/stores/videoStudio.ts
frontend/src/types/video.ts
```

3. Update:

```text
frontend/src/App.tsx
frontend/src/components/Sidebar.tsx
frontend/src/api.ts
```

4. Add Video Studio mode to the sidebar with an appropriate icon from `lucide-react`, such as `Film`, `Clapperboard`, or `Video`.

5. Render a placeholder Video Studio shell with:

- Top header.
- Left Generate panel.
- Center preview placeholder.
- Right history/assets placeholder.
- Bottom timeline placeholder.

6. Ensure app builds and Video Studio can be opened without breaking Chat, Image, or Music.

## Acceptance Criteria

- App has a visible Video Studio mode switch.
- Video Studio is feature-gated.
- Opening Video Studio does not break existing studios.
- Backend exposes basic `/v1/video/...` endpoints.
- Mock provider returns capability data.
- TypeScript and Go builds pass.

---

# Phase 1: AI Video Generation MVP

Implement durable video projects, generation history, provider/model discovery, prompt enhancement, and generated video assets.

## Backend Files to Add

```text
backend/internal/video/service.go
backend/internal/video/provider.go
backend/internal/video/model_registry.go
backend/internal/video/storage.go
backend/internal/video/prompt_enhancer.go
backend/internal/video/types.go
backend/internal/api/video_handler.go
backend/internal/repository/video_project_repo.go
backend/internal/repository/video_generation_repo.go
backend/internal/repository/video_asset_repo.go
```

## Database Migration

At the time this bootstrap phase was written, the latest migration was `{Version: 36, Name: "music_studio"}`, so Video Studio foundation was registered as **Version 37** by appending to the `versionedMigrations()` slice in `db.go`:

```go
{Version: 37, Name: "video_studio_foundation", SQL: migrationVideoStudioFoundation},
```

Define `migrationVideoStudioFoundation` as a SQL string constant (same pattern as `migrationMusicStudio`). New tables must use `CREATE TABLE IF NOT EXISTS`; any new columns added to existing tables must have defaults.

Tables:

```sql
CREATE TABLE IF NOT EXISTS video_projects (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  title TEXT NOT NULL,
  active_timeline_id TEXT,
  default_provider TEXT,
  default_model TEXT,
  width INTEGER NOT NULL DEFAULT 1920,
  height INTEGER NOT NULL DEFAULT 1080,
  fps INTEGER NOT NULL DEFAULT 30,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  aspect_ratio TEXT NOT NULL DEFAULT '16:9',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS video_generations (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  parent_id TEXT,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','completed','failed','cancelled')),
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt TEXT NOT NULL,
  enhanced_prompt TEXT,
  negative_prompt TEXT,
  settings_json TEXT NOT NULL DEFAULT '{}',
  input_asset_ids_json TEXT NOT NULL DEFAULT '[]',
  output_asset_id TEXT,
  upstream_job_id TEXT,
  upstream_request_id TEXT,
  usage_json TEXT,
  cost_usd REAL,
  error TEXT,
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  completed_at DATETIME,
  FOREIGN KEY (project_id) REFERENCES video_projects(id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES video_generations(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS video_assets (
  id TEXT PRIMARY KEY,
  project_id TEXT,
  source_type TEXT NOT NULL,
  source_studio TEXT,
  source_id TEXT,
  kind TEXT NOT NULL CHECK(kind IN ('video','image','audio','music','text','caption','export','other')),
  file_name TEXT NOT NULL,
  file_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER,
  width INTEGER,
  height INTEGER,
  fps REAL,
  thumbnail_path TEXT,
  waveform_path TEXT,
  provider TEXT,
  model TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (project_id) REFERENCES video_projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_video_projects_user ON video_projects(user_id);
CREATE INDEX IF NOT EXISTS idx_video_projects_updated_at ON video_projects(updated_at);
CREATE INDEX IF NOT EXISTS idx_video_generations_project ON video_generations(project_id);
CREATE INDEX IF NOT EXISTS idx_video_generations_parent ON video_generations(parent_id);
CREATE INDEX IF NOT EXISTS idx_video_generations_status ON video_generations(status);
CREATE INDEX IF NOT EXISTS idx_video_assets_project ON video_assets(project_id);
CREATE INDEX IF NOT EXISTS idx_video_assets_kind ON video_assets(kind);
```

Use repository conventions already present in the project.

## Provider Abstraction

Implement a provider interface similar to:

```go
type Provider interface {
    Key() string
    DisplayName() string
    ListModels(ctx context.Context) ([]VideoModel, error)
    Capabilities(model string) VideoCapabilities
    Generate(ctx context.Context, req GenerationRequest, progress func(GenerationProgress)) (*GenerationResult, error)
}
```

Start with:

1. `mock` provider for development.
2. One real provider adapter if an existing configured provider can support video generation through an OpenAI-compatible route, Gemini route, OpenRouter route, or a custom provider. If no safe real provider is already obvious from the repo, build the interface and mock provider only, then leave clear extension points.

Do not hard-code unsupported provider assumptions.

## Backend API Routes

Implement:

```text
GET    /v1/video/providers
GET    /v1/video/models?provider=...
POST   /v1/video/models/refresh
GET    /v1/video/projects
POST   /v1/video/projects
GET    /v1/video/projects/{projectId}
PATCH  /v1/video/projects/{projectId}
DELETE /v1/video/projects/{projectId}
GET    /v1/video/projects/{projectId}/generations
POST   /v1/video/generations
GET    /v1/video/generations/{generationId}
POST   /v1/video/generations/{generationId}/branch
POST   /v1/video/generations/{generationId}/send-to-timeline
GET    /v1/video/projects/{projectId}/assets
GET    /v1/video/assets/{assetId}
GET    /v1/video/assets/{assetId}/download
DELETE /v1/video/assets/{assetId}
POST   /v1/video/enhance-prompt
```

Generation should stream progress via SSE, following the Music Studio generation pattern.

Suggested SSE events:

```text
video_generation_started
video_generation_progress
video_generation_done
video_generation_error
```

## Frontend Types

Add:

```text
frontend/src/types/video.ts
```

Include:

```ts
export type VideoProviderKey = 'mock' | 'openrouter' | 'gemini' | 'openai' | 'custom';

export type VideoCapability =
  | 'text_to_video'
  | 'image_to_video'
  | 'video_to_video'
  | 'extend_video'
  | 'reference_images'
  | 'reference_video'
  | 'negative_prompt'
  | 'seed'
  | 'camera_motion'
  | 'audio_generation';

export interface VideoModel {
  id: string;
  provider: VideoProviderKey;
  name: string;
  capabilities: VideoCapability[];
  aspect_ratios?: string[];
  resolutions?: string[];
  duration_min_seconds?: number;
  duration_max_seconds?: number;
  fps_options?: number[];
  max_prompt_chars?: number;
  notes?: string;
}

export interface VideoProject {
  id: string;
  user_id?: string;
  title: string;
  active_timeline_id?: string;
  default_provider?: VideoProviderKey;
  default_model?: string;
  width: number;
  height: number;
  fps: number;
  duration_ms: number;
  aspect_ratio: string;
  metadata_json?: string;
  created_at: string;
  updated_at: string;
}

export interface VideoAsset {
  id: string;
  project_id?: string;
  source_type: string;
  source_studio?: string;
  source_id?: string;
  kind: 'video' | 'image' | 'audio' | 'music' | 'text' | 'caption' | 'export' | 'other';
  file_name: string;
  file_path: string;
  mime_type: string;
  size_bytes: number;
  duration_ms?: number;
  width?: number;
  height?: number;
  fps?: number;
  thumbnail_path?: string;
  waveform_path?: string;
  provider?: string;
  model?: string;
  metadata_json?: string;
  created_at: string;
}

export interface VideoGenerationDetail {
  id: string;
  project_id: string;
  parent_id?: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | string;
  provider: VideoProviderKey;
  model: string;
  prompt: string;
  enhanced_prompt?: string;
  negative_prompt?: string;
  settings_json?: string;
  input_asset_ids_json?: string;
  output_asset_id?: string;
  asset_url?: string;
  error?: string;
  cost_usd?: number;
  created_at: string;
  completed_at?: string;
}
```

## Frontend Store

Add:

```text
frontend/src/stores/videoStudio.ts
```

State should include:

```ts
projects
activeProjectId
activeGenerationId
generations
assets
providers
selectedProvider
modelsByProvider
selectedModel
promptForm
isLoading
isGenerating
generationProgress
error
abortGeneration
```

Actions:

```ts
loadProviders
loadModels
loadProjects
createProject
selectProject
updateProject
deleteProject
loadGenerations
loadAssets
setProvider
setModel
setPromptField
setGenerationSetting
clearPrompt
enhancePrompt
generate
stopGeneration
branchFromGeneration
regenerateFromGeneration
sendGenerationToTimeline
deleteAsset
```

## Frontend UI

Create:

```text
frontend/src/components/video/VideoStudio.tsx
frontend/src/components/video/VideoPromptBuilder.tsx
frontend/src/components/video/VideoResultCard.tsx
frontend/src/components/video/VideoHistoryPanel.tsx
frontend/src/components/video/VideoAssetBin.tsx
frontend/src/components/video/VideoAssetDetails.tsx
```

Use existing Music Studio visual conventions but make it more visual/video-oriented.

## Acceptance Criteria

- User can open Video Studio.
- User can create/select/delete video projects.
- User can select provider/model from backend capabilities.
- User can enhance a prompt.
- User can generate a mock video asset through SSE.
- Generated asset appears in project assets.
- Generation history persists and reloads.
- User can branch/regenerate from history.
- User can download generated asset.
- Existing Chat/Image/Music functionality remains intact.

---

# Phase 2: Timeline Editor MVP

Implement the first working timeline editor with project-scoped timeline JSON.

## Backend

Add repositories:

```text
backend/internal/repository/video_timeline_repo.go
```

Add migration:

```sql
CREATE TABLE IF NOT EXISTS video_timelines (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  name TEXT NOT NULL DEFAULT 'Main Timeline',
  active INTEGER NOT NULL DEFAULT 1,
  timeline_json TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (project_id) REFERENCES video_projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_video_timelines_project ON video_timelines(project_id);
CREATE INDEX IF NOT EXISTS idx_video_timelines_active ON video_timelines(project_id, active);
```

Add routes:

```text
GET  /v1/video/projects/{projectId}/timeline
PUT  /v1/video/projects/{projectId}/timeline
POST /v1/video/projects/{projectId}/timeline/import-asset
```

## Timeline JSON Schema

Persist neutral JSON like:

```json
{
  "version": 1,
  "canvas": {
    "width": 1920,
    "height": 1080,
    "fps": 30,
    "background": "#000000"
  },
  "duration_ms": 30000,
  "tracks": [
    {
      "id": "track-video-1",
      "type": "video",
      "name": "Video 1",
      "locked": false,
      "muted": false,
      "visible": true,
      "clips": [
        {
          "id": "clip-1",
          "asset_id": "asset-video-abc",
          "start_ms": 0,
          "duration_ms": 8000,
          "trim_in_ms": 0,
          "trim_out_ms": 8000,
          "transform": {
            "x": 0,
            "y": 0,
            "scale": 1,
            "rotation": 0,
            "opacity": 1
          },
          "effects": [],
          "keyframes": []
        }
      ]
    }
  ],
  "markers": [],
  "metadata": {}
}
```

## Frontend Components

Add:

```text
frontend/src/components/video/timeline/VideoTimeline.tsx
frontend/src/components/video/timeline/TimelineTrack.tsx
frontend/src/components/video/timeline/TimelineClip.tsx
frontend/src/components/video/timeline/TimelineRuler.tsx
frontend/src/components/video/timeline/TimelinePlayhead.tsx
frontend/src/components/video/timeline/TimelineToolbar.tsx
frontend/src/components/video/VideoPreviewCanvas.tsx
frontend/src/components/video/VideoInspector.tsx
```

Add timeline state to `videoStudio.ts`:

```ts
timeline
selectedClipId
selectedTrackId
playheadMs
zoom
isPlaying
snappingEnabled
```

Actions:

```ts
loadTimeline
saveTimeline
addAssetToTimeline
moveClip
trimClip
splitClipAtPlayhead
deleteClip
duplicateClip
selectClip
setPlayhead
setZoom
toggleTrackMute
toggleTrackLock
toggleTrackVisibility
updateClipTransform
updateClipVolume
updateClipFade
```

## MVP Editing Behaviors

Implement:

- Add video/image/audio asset to first compatible track.
- Auto-create track if no compatible track exists.
- Drag clip horizontally to change `start_ms`.
- Drag clip between compatible tracks.
- Trim clip start/end.
- Split clip at playhead.
- Delete clip.
- Timeline zoom in/out.
- Snap to playhead and nearby clip edges.
- Basic keyboard shortcuts:
  - Space: play/pause preview.
  - Delete/Backspace: delete selected clip.
  - S: split at playhead.
  - Ctrl/Cmd+S: save timeline.

## Preview MVP

The preview should:

- Resolve active clips at the current playhead time.
- Show the topmost visible video/image clip.
- Mix audio/music through browser playback if feasible.
- Display text/caption clips as overlays if present.
- Show playhead position and duration.

Do not attempt perfect NLE-level frame accuracy in Phase 2. Prioritize correctness of state, save/load, and intuitive UI.

## Acceptance Criteria

- User can create a timeline in a video project.
- User can add assets to timeline.
- User can move, trim, split, and delete clips.
- Timeline persists and reloads.
- Preview updates based on playhead.
- Tracks support lock/mute/visibility states.
- UI resembles a practical Camtasia-like editor.

---

# Phase 3: Render / Export MVP

Implement render jobs and export outputs.

## Backend

Add:

```text
backend/internal/video/renderer.go
backend/internal/video/render_worker.go
backend/internal/repository/video_render_job_repo.go
```

Add migration:

```sql
CREATE TABLE IF NOT EXISTS video_render_jobs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  timeline_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','completed','failed','cancelled')),
  progress REAL NOT NULL DEFAULT 0,
  settings_json TEXT NOT NULL DEFAULT '{}',
  output_asset_id TEXT,
  error TEXT,
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  started_at DATETIME,
  completed_at DATETIME,
  FOREIGN KEY (project_id) REFERENCES video_projects(id) ON DELETE CASCADE,
  FOREIGN KEY (timeline_id) REFERENCES video_timelines(id) ON DELETE CASCADE,
  FOREIGN KEY (output_asset_id) REFERENCES video_assets(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_video_render_jobs_project ON video_render_jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_video_render_jobs_status ON video_render_jobs(status);
```

Add routes:

```text
POST /v1/video/projects/{projectId}/render
GET  /v1/video/render-jobs/{jobId}
POST /v1/video/render-jobs/{jobId}/cancel
```

## Renderer Adapter

Create a renderer abstraction:

```go
type Renderer interface {
    Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error)
}
```

Start with either:

1. A Remotion renderer adapter that invokes a local Node/Remotion render worker.
2. A mock renderer adapter that produces a deterministic placeholder MP4 for development, with clear TODOs for Remotion/FFmpeg integration.

Preferred long-term pipeline:

```text
React frontend
  -> Go backend render job
    -> Node/Remotion render worker or FFmpeg adapter
      -> MP4/WebM output
        -> video_assets + optional File Library registration
```

## Export Settings

Support at least:

```ts
interface VideoExportSettings {
  format: 'mp4' | 'webm';
  codec?: 'h264' | 'h265' | 'vp9';
  resolution: '720p' | '1080p' | 'project';
  fps?: number;
  quality?: 'draft' | 'standard' | 'high';
  include_audio: boolean;
  register_in_file_library?: boolean;
}
```

## Frontend

Add:

```text
frontend/src/components/video/VideoRenderPanel.tsx
frontend/src/components/video/RenderJobStatus.tsx
```

Add store actions:

```ts
renderTimeline
pollRenderJob
cancelRenderJob
downloadRender
```

Export UX:

- Export button in Video Studio header.
- Modal/panel for export presets.
- Progress indicator.
- Result appears in assets as `kind: 'export'`.
- Download button.
- Optional register in File Library.

## Acceptance Criteria

- User can start render/export from current timeline.
- Render job persists.
- Progress is visible.
- Render output becomes a VideoAsset.
- User can download export.
- Export does not freeze the UI.
- Render failure returns useful error.

---

# Phase 4: Cross-Studio Workflows

Extend the existing cross-studio concept so Video Studio becomes the composition hub for Image Studio and Music Studio outputs.

## Image Studio → Video Studio

Add actions from Image Studio asset/result cards:

- “Send to Video Studio”
- “Animate in Video Studio”
- “Use as timeline image”
- “Use as image-to-video reference”

Implementation:

- Create or select a VideoProject.
- Import the image asset into `video_assets` with:

```text
source_studio = 'image'
source_id = image asset or node asset ID
kind = 'image'
```

- Optionally prefill Video Studio prompt using an image-to-video prompt translation.

## Music Studio → Video Studio

Add actions from Music Studio result cards:

- “Send to Video Studio”
- “Add track to timeline”
- “Create music visualizer”
- “Generate video from this track”

Implementation:

- Import music asset into `video_assets` with:

```text
source_studio = 'music'
source_id = music asset ID
kind = 'music'
```

- Add to timeline audio/music track.
- For visualizer mode, create a default timeline with:
  - background image/video placeholder.
  - music track.
  - waveform/visualizer overlay placeholder.

## Chat → Video Studio

Add action from Chat output or message menu:

- “Turn into video project”

This should:

- Create a new VideoProject.
- Generate a storyboard/script using LLM.
- Optionally create title/text clips.
- Prefill generation prompts.

## File Library → Video Studio

Add Video Studio import panel support for:

- MP4/MOV/WebM.
- MP3/WAV/M4A.
- PNG/JPG/WebP/GIF.
- Documents as script/storyboard source.

Imported file should become a `video_asset`. When imported from the File Library, preserve:

```text
source_studio = 'file_library'
source_id = library_file_id
```

## Crossover Handler

Update:

```text
backend/internal/api/crossover_handler.go
```

Support:

```text
image -> video
music -> video
chat -> video
video -> image
video -> music
```

Add frontend API methods:

```ts
crossoverApi.translate.imageToVideo(...)
crossoverApi.translate.musicToVideo(...)
crossoverApi.translate.chatToVideo(...)
crossoverApi.translate.videoToImage(...)
crossoverApi.translate.videoToMusic(...)
```

## Acceptance Criteria

- Image Studio can send an image to Video Studio.
- Music Studio can send a track to Video Studio.
- Chat can create a new Video project from text.
- File Library assets can be imported into a project.
- Imported assets can be added to timeline.
- Source metadata is preserved.

---

# Phase 5: Camtasia-Style Editing Polish

Expand timeline editing features toward a practical lightweight editor.

## Add Effects

Support effect definitions in clip JSON:

```ts
interface TimelineEffect {
  id: string;
  type: 'blur' | 'brightness' | 'contrast' | 'saturation' | 'grayscale' | 'shadow' | 'background_blur' | 'chroma_key';
  enabled: boolean;
  params: Record<string, unknown>;
}
```

UI:

- Effects panel.
- Add/remove/reorder effects.
- Toggle effect.
- Inspector controls.

## Add Transitions

Support transitions:

```ts
interface TimelineTransition {
  id: string;
  type: 'fade' | 'crossfade' | 'dip_to_black' | 'slide' | 'wipe' | 'zoom';
  duration_ms: number;
  direction?: 'left' | 'right' | 'up' | 'down';
}
```

Apply transitions to clip in/out or between adjacent clips.

## Add Text, Captions, and Callouts

Track types:

```text
text
caption
shape
callout
```

Support:

- Text clip editor.
- Font size.
- Font weight.
- Position.
- Background box.
- Stroke/shadow.
- Captions as timed segments.
- Arrows, rectangles, circles, highlights.

## Add Audio Polish

- Fade in/out handles.
- Volume envelope.
- Mute/solo track controls.
- Audio ducking placeholder.
- Waveform display.

## Add Keyframes

Support keyframes:

```ts
interface TimelineKeyframe {
  id: string;
  property: 'x' | 'y' | 'scale' | 'rotation' | 'opacity' | 'volume';
  time_ms: number;
  value: number;
  easing?: 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out';
}
```

## Acceptance Criteria

- User can add effects to clips.
- User can add transitions.
- User can create text/callout/caption clips.
- User can adjust fades and volume.
- User can add simple keyframes for transform/opacity/volume.
- Render/export respects at least core effects, text, fades, and simple transitions.

---

# Phase 6: AI-Assisted Editing

Add higher-level AI editing capabilities that differentiate OmniLLM-Studio from a basic editor.

## AI Actions

Add a Video AI Assistant panel with actions:

```text
Create storyboard
Create script
Generate shot list
Turn prompt into timeline
Make this 30 seconds
Make this vertical
Add captions
Generate title cards
Suggest b-roll
Cut to the beat
Create intro/outro
Generate thumbnail prompt
Create social variants
```

## Backend

Add:

```text
backend/internal/video/assistant.go
```

Routes:

```text
POST /v1/video/projects/{projectId}/assistant/storyboard
POST /v1/video/projects/{projectId}/assistant/timeline-plan
POST /v1/video/projects/{projectId}/assistant/edit-plan
POST /v1/video/projects/{projectId}/assistant/apply-edit-plan
POST /v1/video/projects/{projectId}/assistant/social-variants
```

## Edit Plan Schema

Do not allow the LLM to mutate timeline JSON directly. It should return a validated edit plan.

Example:

```json
{
  "summary": "Create a 30 second vertical promo from the current timeline",
  "operations": [
    {
      "type": "set_canvas",
      "width": 1080,
      "height": 1920,
      "fps": 30
    },
    {
      "type": "trim_clip",
      "clip_id": "clip-1",
      "start_ms": 0,
      "duration_ms": 8000
    },
    {
      "type": "add_text_clip",
      "track_id": "track-text-1",
      "start_ms": 0,
      "duration_ms": 3000,
      "text": "New Product Launch"
    }
  ]
}
```

Validate operations before applying.

## Captions

If no transcription provider exists yet, add placeholder interfaces for STT/caption generation. Do not hard-code an unavailable provider.

## Acceptance Criteria

- User can generate a storyboard from a prompt.
- User can ask for a timeline plan.
- User can preview an edit plan before applying.
- User can apply validated edit operations to timeline.
- User can generate social-format variants.
- AI never directly writes unvalidated timeline JSON.

---

# Phase 7: Testing, Hardening, and Documentation

## Tests

Add tests for:

- Video provider capability registry.
- Video project repository CRUD.
- Video generation repository CRUD/status updates.
- Video asset repository CRUD.
- Timeline validation.
- Timeline operations: add/move/trim/split/delete.
- Render job lifecycle.
- Crossover translation request validation.

Frontend tests where existing test framework allows:

- Video Studio mode render.
- Create project flow.
- Add asset to timeline.
- Timeline reducer/store operations.
- Prompt enhancer button state.
- Generation SSE event parsing.

## Validation

Implement backend validation for:

- Timeline JSON schema.
- Project ownership/user visibility.
- Asset belongs to project before timeline import.
- Media kind compatibility with track type.
- Render settings.
- Provider/model capabilities.

## Error Handling

Use clear user-facing errors:

- No video provider configured.
- Model does not support selected capability.
- Unsupported media type.
- Asset not found.
- Timeline invalid.
- Render failed.
- Generation cancelled.

## Documentation

Update:

```text
README.md
docs/
```

Add:

```text
docs/VIDEO_STUDIO.md
docs/VIDEO_STUDIO_ARCHITECTURE.md
docs/VIDEO_PROVIDER_ADAPTERS.md
docs/VIDEO_TIMELINE_SCHEMA.md
docs/VIDEO_RENDERING.md
```

README updates:

- Add Video Studio to feature overview.
- Add supported video provider section.
- Add notes for mock/dev provider.
- Add export/render requirements.

## Acceptance Criteria

- Backend tests pass.
- Frontend typecheck passes.
- Existing studios still work.
- New docs explain how to add a provider and how timeline JSON works.
- README accurately describes Video Studio.

---

# Suggested File Checklist

## Frontend New Files

```text
frontend/src/types/video.ts
frontend/src/stores/videoStudio.ts
frontend/src/components/video/VideoStudio.tsx
frontend/src/components/video/VideoPromptBuilder.tsx
frontend/src/components/video/VideoResultCard.tsx
frontend/src/components/video/VideoHistoryPanel.tsx
frontend/src/components/video/VideoAssetBin.tsx
frontend/src/components/video/VideoAssetDetails.tsx
frontend/src/components/video/VideoPreviewCanvas.tsx
frontend/src/components/video/VideoInspector.tsx
frontend/src/components/video/VideoRenderPanel.tsx
frontend/src/components/video/timeline/VideoTimeline.tsx
frontend/src/components/video/timeline/TimelineTrack.tsx
frontend/src/components/video/timeline/TimelineClip.tsx
frontend/src/components/video/timeline/TimelineRuler.tsx
frontend/src/components/video/timeline/TimelinePlayhead.tsx
frontend/src/components/video/timeline/TimelineToolbar.tsx
```

## Frontend Existing Files to Update

```text
frontend/src/App.tsx                  # add `appMode === 'video'` render branch + lazy import
frontend/src/components/Sidebar.tsx   # add Video Studio switch + isEnabled('video_studio') gate
frontend/src/api.ts                   # add video API client + extend crossoverApi.translate
frontend/src/stores/index.ts          # extend appMode union + getInitialAppMode() (NOT a settings.ts)
frontend/src/components/SettingsPanel.tsx  # add video_studio feature-flag toggle
frontend/src/types.ts                 # only if shared types are needed; video types go in types/video.ts
```

Note: there is no `frontend/src/stores/settings.ts`. The settings/app-mode store is `useSettingsStore` inside `frontend/src/stores/index.ts`.

## Backend New Files

```text
backend/internal/video/service.go
backend/internal/video/types.go
backend/internal/video/provider.go
backend/internal/video/model_registry.go
backend/internal/video/storage.go
backend/internal/video/timeline.go
backend/internal/video/renderer.go
backend/internal/video/render_worker.go
backend/internal/video/prompt_enhancer.go
backend/internal/video/assistant.go
backend/internal/api/video_handler.go
backend/internal/repository/video_project_repo.go
backend/internal/repository/video_generation_repo.go
backend/internal/repository/video_asset_repo.go
backend/internal/repository/video_timeline_repo.go
backend/internal/repository/video_render_job_repo.go
```

## Backend Existing Files to Update

```text
backend/internal/api/router.go
backend/internal/db/db.go
backend/internal/models/models.go
backend/internal/llm/service.go
backend/internal/api/crossover_handler.go
backend/internal/filelibrary/service.go if needed for registration integration
```

## Docs New Files

```text
docs/VIDEO_STUDIO.md
docs/VIDEO_STUDIO_ARCHITECTURE.md
docs/VIDEO_PROVIDER_ADAPTERS.md
docs/VIDEO_TIMELINE_SCHEMA.md
docs/VIDEO_RENDERING.md
```

---

# Historical Recommended Implementation Order

This was the original bootstrap order and is retained for context:

1. Add feature flag, app mode, placeholder UI, placeholder API.
2. Add database migration and repositories for projects/generations/assets.
3. Add mock provider and generation SSE.
4. Add frontend generation UI and history.
5. Add timeline database and timeline store.
6. Add timeline editor MVP.
7. Add render job database and mock/initial renderer.
8. Add cross-studio imports.
9. Add effects/transitions/captions/keyframes.
10. Add AI assistant edit planning.
11. Split creation and editing UI into Video Studio and Video Edit Studio.
12. Add tests and documentation.

Each step was intended to compile and leave the app usable.

---

# Quality Bar

The implementation must:

- Match the existing app visual language.
- Preserve existing Chat/Image/Music behavior.
- Use strongly typed TypeScript interfaces.
- Keep provider secrets server-side only.
- Validate all backend inputs.
- Avoid blocking UI during long-running jobs.
- Avoid one-off hacks for a single provider.
- Keep timeline persistence renderer-agnostic.
- Include meaningful user-facing error messages.
- Include clear TODOs where real provider/render integration requires external services.
- Prefer small, composable components over one giant `VideoStudio.tsx`.
- Keep generation, assets, timeline, and render jobs as separate concerns.

---

# Final Deliverable

When finished, the repository should support focused Video Studio and Video Edit Studio workspaces where a user can:

1. Open Video Studio from the sidebar.
2. Create a video project.
3. Generate a single video output from a provider-backed prompt.
4. See generation history and branch from previous generations.
5. Preview and download generated outputs.
6. Open Video Edit Studio from Video Studio or the sidebar.
7. Import video/image/audio/music assets into the edit workspace.
8. Add assets to a timeline.
9. Perform basic timeline edits.
10. Preview the timeline.
11. Export/render a video.
12. Move assets between Image Studio, Music Studio, Chat, File Library, and the video project.
13. Use AI assistance for prompt enhancement, storyboarding, and edit planning.

The original bootstrap started with a development provider and durable data model. Current state is reflected in the progress and mock-removal sections at the top of this document.
