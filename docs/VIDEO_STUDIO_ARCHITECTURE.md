# Video Studio Architecture

Video Studio and Video Edit Studio follow the existing Go service/repository/API layering and React/Zustand frontend pattern. They share the same video project backend but keep creation and editing UI concerns separate.

## Backend

- `models.VideoProject`: project shell and defaults.
- `models.VideoGeneration`: AI generation lineage and provider metadata.
- `models.VideoAsset`: durable project media bin entries.
- `models.VideoTimeline`: active neutral timeline JSON.
- `models.VideoRenderJob`: export job lifecycle and output asset linkage.

Routes live under `/v1/video`. Generation uses Server-Sent Events. Timeline, render, and assistant endpoints are normal JSON APIs.

Provider registration includes `NewOpenRouterProvider("", "")` and `NewGeminiProvider("", "")`. OpenRouter and Gemini are the real generation providers. Render/export jobs use `NewFFmpegRenderer("")` by default.

Key files inside `backend/internal/video/`:

- `timeline.go`: timeline document structs, `ValidateTimelineDocument` (normalizes and rejects bad documents before save/render, including 0.25×–4× playback rates and canonical source out-points), `UpgradeTimelineDocument` (schema versioning), and pure document transforms — `SliceTimelineRange` (export ranges, retime-aware trim/keyframe/cursor rebasing) and `StripCaptionOverlays` (caption burn-in toggle).
- `renderer.go`: the FFmpeg renderer — filtergraph construction, codec/bitrate argument mapping (H.264/H.265/VP9), constant-speed video `setpts` plus pitch-preserving audio `atempo` chains, shape/redaction subgraphs (drawbox, blur, pixelate mosaic), keyframe expressions, and audio mixdown. When export audio is disabled it omits the audio subgraph entirely.
- `renderer_capabilities.go`: the export-fidelity matrix served at `GET /v1/video/render/capabilities`; it is the single source of truth for frontend "preview only"/"partial" warnings and must track `renderer.go`.
- `captions.go`: caption cue extraction and SRT/VTT serialization for render-time sidecar assets.
- `assistant.go`: storyboard/edit-plan/social-variant generation with LLM calls plus deterministic fallbacks, and plan validation against the live timeline.
- `service.go`: orchestration — render-job lifecycle (including sidecar asset creation and FFmpeg diagnostics persisted to job metadata), export settings validation, asset ingest/import, and artifact generation.

## Frontend

- `frontend/src/types/video.ts`: strongly typed provider, project, asset, timeline, render, and assistant contracts. `VideoPromptForm` includes optional fields for 7 cinematic dimensions (`composition`, `lens_effect`, `lighting`, `dialogue`, `sound_effects`, `ambient_noise`, `continuity_notes`). Timeline types mirror the Go structs exactly (`snake_case`), including constant `playback_rate`, the 14 shape/annotation kinds, cursor metadata, and the extended export settings (codec, range, caption burn-in/sidecar, audio bitrate).
- `frontend/src/stores/videoStudio.ts`: the single Zustand store for both studios — project state, generation stream handling, timeline editing actions (move/trim/split, retime, ripple and gap operations, grouping, clipboard, fades/ducking, keyframes and motion presets, captions, markers), undo/redo snapshots (one per user action), serialized timeline saves with explicit `idle`/`saving`/`saved`/`error` status, render polling with dirty-since-render tracking, preview-only master volume, and assistant actions. `setPromptField<K>` is generically typed for safe key-value updates.
- `frontend/src/components/video/VideoStudio.tsx`: focused AI video creation panel, generation history, and selected output preview. The creation panel uses a `CollapsibleSection` component (`<details>/<summary>` with a `ChevronDown` chevron) to organize inputs into independently collapsible sections: Prompt, Start / Last Frame, Reference Images, Output Format, Cinematic Controls, and Advanced. The `AssetPicker` component renders a single dropdown alongside an upload `+` button; when an asset is selected, its thumbnail (image or video poster frame) renders inline below the control.
- `frontend/src/components/video/VideoEditStudio.tsx`: editor shell — header (editor modes, templates, text, record), project strip, project-media/favorites bin with global-library shortcut, preview canvas + timeline center column, preview master volume, and a mode-gated right rail (Properties / Effects / Transitions / Captions / Audio / AI Assistant / Export).
- `frontend/src/components/video/timeline/`: `VideoTimeline.tsx` (shortcuts, context menus, marquee, dialogs), `TimelineTrack.tsx` (lanes, drag/drop, snapping), `TimelineClip.tsx` (clip visuals, trim/fade handles, volume envelopes), `TimelineToolbar.tsx` (tools, ripple/snap toggles, status readout), `TimelineRuler.tsx`/`TimelinePlayhead.tsx`, and `KeyframeLane.tsx` (collapsible per-property keyframe lane for the selected clip).
- `frontend/src/components/video/effects/`: registries that are the single source of truth for what previews vs. exports — `effectRegistry.ts` (categorized effects + CSS preview filters), `transitionRegistry.ts`, `annotationRegistry.ts` (shape kinds, creation defaults, style presets), `motionPresets.ts` (keyframe-generating pan/zoom presets), and `keyframeUtils.ts` (sampling/easing).
- `frontend/src/components/video/`: `VideoPreviewCanvas.tsx` (asset-driven compositing, managed preview audio for video/audio clips, 8-handle selection, smart guides, inline text editing, crop mode, cursor overlay), `VideoInspector.tsx` (properties, retiming, and assistant sections, switchable via `section`/`focus` for the rail tabs), `ShapePreview.tsx` (annotation rendering), `EffectBrowser.tsx` (effect/transition card browsers), `VideoCaptionPanel.tsx` (transcript editor), `VideoRenderPanel.tsx` + `RenderJobStatus.tsx` (export settings, validation checklist, job queue), `RecordingModal.tsx` (screen/camera/voiceover capture), `exportValidation.ts` (pre-render checklist rules), and `planDiff.ts` (assistant operation before→after diffs).
- `frontend/src/components/common/`: `ContextMenu.tsx` (shared portal menu) and `AppDialog.tsx` (`ConfirmDialog`/`InputDialog`, the app-native replacements for `window.confirm`/`window.prompt`).

### Asset Upload Flow

1. User clicks the `+` button next to an image/video asset picker in the creation panel.
2. A hidden `<input type="file">` triggers the OS file picker (scoped to `image/*` or `video/*`).
3. `videoApi.uploadAsset(projectId, file)` POSTs `multipart/form-data` to `POST /v1/video/projects/{projectId}/assets/upload`.
4. The backend validates project ownership, accepts up to 25 MB images, 100 MB audio, or 500 MB video, detects MIME type from content, derives `kind`, writes the file to video storage, and creates a `VideoAsset` row.
5. The new asset is returned (HTTP 201) and immediately selected in the picker; the store's `assets` array is updated via `useVideoStudioStore.setState({ assets: updated })`.
6. The thumbnail renders below the picker from `videoAssetUrl(assetId)` → `GET /v1/video/assets/{id}/download`.

## Storage

The video project backend uses the configured attachments directory and stores files under a `video` namespace. Paths are persisted as relative paths and resolved through the same guarded path-join approach used elsewhere in the app.

## Separation Of Concerns

Generation history and timeline composition are intentionally separate:

- Generation history answers how an AI clip was created.
- Timeline state answers how a final video is composed.

This allows later Remotion, FFmpeg, or provider-specific renderers without rewriting saved projects.
