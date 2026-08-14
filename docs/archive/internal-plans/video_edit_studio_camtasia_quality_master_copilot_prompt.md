> **Archived — superseded implementation prompt.** The current renderer capability matrix and open work are in [VIDEO_RENDERER_RELIABILITY_TRANSCRIPTION_SCALABILITY_2026-07-20.md](../../VIDEO_RENDERER_RELIABILITY_TRANSCRIPTION_SCALABILITY_2026-07-20.md) and [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Video Edit Studio — Commercial-Quality / Camtasia-Like Upgrade (Master Prompt)

> **Accuracy revision 2026-06-10 (updated same day after implementation).**
> This document was audited against the `feat_video_studio_enhancements`
> branch. The original draft assumed a much earlier baseline; in reality,
> large portions of phases 3–9 were already implemented. The audit-identified
> gaps for phases 1–6 and parts of 5/7/10 were then implemented on
> 2026-06-10 — see per-phase status. Each phase carries a status marker:
>
> - ✅ **Done** — implemented and covered by tests or verified in code.
> - 🟡 **Partial** — core exists; the listed gaps are the remaining work.
> - ❌ **Missing** — not implemented; full phase is open work.
>
> Implementers: do not re-build ✅ items. Work the 🟡/❌ gap lists only.

## Repository

Repository: `ajbergh/OmniLLM-Studio`

Working branch: `feat_video_studio_enhancements`

## Objective

Upgrade **Video Edit Studio** from a working editor shell into a polished,
commercial-quality video editor with a Camtasia-like editing experience:

- Layer-based editing, not media-type-specific tracks.
- A professional timeline with predictable clip/layer manipulation.
- Right-click context menus across every major editor surface.
- A multi-layer preview compositor that reflects the timeline.
- FFmpeg render/export fidelity that matches preview as closely as practical.
- Strong media-bin workflows, thumbnails, metadata, waveforms, and asset actions.
- Professional inspector controls for timing, transforms, text, effects,
  transitions, audio, and keyframes.
- Timeline-aware AI assistant edits that are previewed, validated, and undoable.
- Commercial polish: keyboard shortcuts, undo/redo, autosave, diagnostics,
  empty states, accessibility, and tests.

Do **not** implement this as a broad rewrite. Preserve the existing Go backend,
React/TypeScript frontend, Zustand store, Chi routes, SQLite persistence model,
and FFmpeg render-job architecture. Prefer small, reviewable phases with tests
after each phase.

---

## Current Codebase Map (verified)

### Frontend

- `frontend/src/components/video/VideoEditStudio.tsx` — editor shell, 3-panel
  resizable layout, media bin (grid/list, search, sort, filter tabs, thumbnails,
  rename/delete/download, drag-to-timeline).
- `frontend/src/components/video/VideoPreviewCanvas.tsx` — **multi-layer**
  compositor: renders all active clips at the playhead, sorted by track index +
  z-index; transforms (x/y/scale/rotation/opacity/crop), fades, full text
  styling, click-to-select, scale/rotate/crop handles, thirds grid and
  safe-area overlays.
- `frontend/src/components/video/VideoInspector.tsx` — assistant section
  (storyboard / edit plan / timeline plan / social variants / quick workflows,
  plan preview + issues + apply), canvas controls, transform sliders, audio
  volume/fades, effects manager, transitions, keyframe list editor, renderer
  capability warnings.
- `frontend/src/components/video/VideoRenderPanel.tsx` — export presets
  (project / 720p / 1080p / YouTube 16:9 / Shorts 9:16 / Square 1:1 / custom),
  format/fps/quality/audio settings, capability warnings.
- `frontend/src/components/video/RenderJobStatus.tsx` — progress, cancel,
  download, FFmpeg failure diagnostics (`<details>`), last-3 job history.
- `frontend/src/components/video/VideoCaptionPanel.tsx` +
  `captions/captionUtils.ts` — SRT/VTT import/export, segment editing,
  split-at-playhead, merge-with-next, three style presets.
- `frontend/src/components/video/effects/effectRegistry.ts` — 10 effects with
  `exportSupported` flags; `transitionRegistry.ts` — 6 transitions;
  `keyframeUtils.ts` — interpolation with 5 easings.
- `frontend/src/components/video/templates/timelineTemplates.ts` — 6 starter
  templates; `editorModes.ts` — full / simple_trim / captions_only /
  social_clip UI modes.
- `frontend/src/components/video/timeline/` — VideoTimeline (keyboard
  shortcuts, snap targets, help overlay), TimelineToolbar (undo/redo, tools,
  zoom, snap, save), TimelineTrack (header controls, inline rename, height
  resize, drop zones), TimelineClip (trim handles, drag, blade), TimelineRuler
  (adaptive ticks, markers), TimelinePlayhead.
- `frontend/src/stores/videoStudio.ts` — the store. Already implements:
  undo/redo (50-deep, `timelineUndoStack`/`timelineRedoStack`), track ops
  (`addTrack`, `removeTrack`, `renameTrack`, `reorderTrack`, `setTrackHeight`,
  `toggleTrackLock/Mute/Visibility`), clip ops (`moveClip`, `trimClip`,
  `trimClipEdgeToPlayhead`, `splitClipAt(Playhead)`, `duplicateClip`,
  `deleteClip`, `groupClips`/`ungroupClips`), multi-select
  (`selectedClipIds`), transform/effect/transition/keyframe/volume/fade/text
  mutations, z-order (`bringClipForward`/`sendClipBackward`), markers,
  snapping, zoom + `zoomToFit`, captions, assistant requests + `applyAssistantPlan`.
- `frontend/src/types/video.ts` — types mirror the Go schema.

### Backend

- `backend/internal/video/timeline.go` — schema (version 1), 8 typed tracks
  (`video|image|audio|music|text|caption|shape|callout`), validation,
  `AddAssetToTimeline`, `SplitClipAt`, `DeleteClip`. **Still type-restricted:**
  `trackAcceptsKind` rejects mismatched media; `NewEmptyTimeline` creates
  Video 1 / Overlay 1 / Audio 1 / Text 1.
- `backend/internal/video/renderer.go` — FFmpeg compositing bottom-to-top,
  transforms (x/y/scale/rotation/opacity/crop), fades, text burn-in, audio
  mixdown with per-clip volume + afades + mute/hidden track semantics,
  fade-family transitions, 7/10 effects, x/y position keyframes.
- `backend/internal/video/renderer_capabilities.go` — capability matrix served
  at `GET /v1/video/render/capabilities` (note: **not** `/render-capabilities`).
- `backend/internal/video/probe.go` — ffprobe duration/width/height/fps with
  graceful fallback. No codec/audio-stream fields, no thumbnails, no waveforms.
- `backend/internal/video/assistant.go` — `CreateEditPlan` (LLM with
  deterministic fallback), `ValidateEditPlanOperations`,
  `ApplyEditPlanToTimeline`; ops: `set_canvas`, `set_duration`,
  `add_text_clip`, `move_clip`, `trim_clip`, `delete_clip`; timeline context
  summary includes canvas, tracks, clips, assets, selection, playhead, and
  renderer capabilities; social variants for 9:16 / 1:1 / 16:9.
- `backend/internal/video/service.go` — render jobs with diagnostics persisted
  to `metadata_json` (`ffmpeg_command`, `ffmpeg_stderr` truncated to 8KB,
  output probe results), cancellation via stored `context.CancelFunc`,
  `RecoverInterruptedRenderJobs` (marks interrupted jobs failed on startup),
  music-studio asset import wiring.
- `backend/internal/api/video_handler.go` + `router.go` — full route set
  including timeline get/save/import-asset, render + render-jobs, assistant
  endpoints, asset upload/import/update/download/delete,
  attach-to-conversation, register-in-library.
- DB (in `backend/internal/db/db.go` migrations): `video_projects`,
  `video_generations`, `video_assets` (has `thumbnail_path` and
  `waveform_path` columns — **currently never populated**), `video_timelines`,
  `video_render_jobs` (`metadata_json` since v41).

### Docs / tests

- `docs/VIDEO_STUDIO.md`, `docs/VIDEO_TIMELINE_SCHEMA.md`,
  `docs/VIDEO_RENDERING.md`, `docs/VIDEO_STUDIO_ARCHITECTURE.md`,
  `docs/VIDEO_PROVIDER_ADAPTERS.md` — all exist; update alongside changes.
- Backend: 49 Go test functions across 9 files in `backend/internal/video/`
  (timeline, renderer fidelity, capabilities, assistant plans, probe,
  providers).
- Frontend: **no unit-test framework configured** (no vitest/jest).
- Playwright at repo root: `tests/video-editor.smoke.spec.ts` already covers
  the Video Edit Studio happy path (plus image-editor and music-studio specs).

---

## Non-Negotiable Design Decisions

### 1. Timeline tracks are generic ordered layers ✅ Implemented

A track/layer can contain any supported clip type (video, image, text,
caption, shape, callout, audio, music, export/other). Media behavior comes
from the clip and asset, not the track.

```text
Layer 4  ← top of the UI list = foreground (last in the tracks array)
Layer 3
Layer 2
Layer 1  ← bottom of the UI list = background (index 0)
```

> **As implemented** (deviation from this doc's original "Layer 1 = top"
> sketch): the data convention was preserved — later tracks in the array stack
> on top — and the UI displays the list reversed, so the foreground layer is
> the top row. Numbering follows the Camtasia/Premiere convention (Track 1 at
> the bottom). This kept every existing timeline rendering identically with no
> migration.

- Higher rows in the UI list visually overlay lower rows (preview and export).
- Audio/music/video audio contributes to the mix if the layer is not muted.
- `visible` controls visual output; `muted` controls audio contribution;
  `locked` prevents editing/moving/dropping clips. Solo is an ephemeral
  preview-monitoring control (not persisted).
- Legacy typed tracks remain loadable; explicit drops accept any kind on any
  unlocked track, while auto-placement still routes media to matching legacy
  tracks.

### 2. Right-click context menus are first-class

Reusable, accessible, portal-based context-menu system, safe inside
scroll/overflow containers. Today only inline menus exist on timeline clips,
track headers, and ruler markers.

### 3. Preview and export should converge

Preview and FFmpeg renderer must agree on layer ordering, timing, trim, text,
transforms, opacity, fades, visibility/mute, and basic audio mixdown. Where
preview supports a feature export does not, surface a capability warning —
the capability matrix and inspector/render-panel warnings already exist;
keep them truthful as features land.

### 4. Assistant edits must be safe

Edit plans are timeline-aware, validated, previewed, and undoable. Never
mutate the timeline without user confirmation. (Already enforced: plans
require explicit Apply; application goes through the undo stack.)

---

## Phase 1 — Refactor Timeline Tracks Into Generic Layers ✅ Done (2026-06-10)

Implemented: `TrackTypeLayer`, layer-creating `NewEmptyTimeline`,
kind-agnostic `AddAssetToTimeline` (locked-only rejection, legacy auto-routing
preference, new-layer fallback), `defaultDurationForAssetKind` +
`kindForAssetOrMime`/`isVisualAssetKind`/`isAudioAssetKind`, frontend `'layer'`
type + layer-first store defaults + reversed track display, preview/audio
selection made asset-kind-based, **and the renderer layer-ordering fix** —
visual stacking now follows track order + z-index instead of start time, with
text clips interleaved into the same compositing chain (this was a real
preview/export divergence the original audit missed). Tests cover all of it.

### Backend requirements (`backend/internal/video/timeline.go`)

1. Add `TrackTypeLayer = "layer"` and accept it in `normalizeTrackType`.
2. Keep existing media-specific track types for backward compatibility;
   legacy documents must still validate and load. No destructive migration.
3. `NewEmptyTimeline` creates four generic layers (`Layer 1`–`Layer 4`,
   `type: "layer"`, visible, unlocked, unmuted, empty clips).
4. Refactor `AddAssetToTimeline`:
   - With `track_id`: accept the asset on that track unless locked — **no
     kind/type mismatch rejection** (for any track type, including legacy).
   - Without `track_id`: first unlocked layer, else create a new generic layer.
   - Clip defaults from asset kind/MIME (not track type): visual clips get a
     default transform; audio-capable clips a default volume; video clips both.
5. Replace `defaultDurationForTrack` with `defaultDurationForAssetKind`
   (image/text/caption/shape/callout → 5000ms, audio/music → 30000ms,
   default → 8000ms).
6. Add helpers `isVisualAssetKind`, `isAudioAssetKind`,
   `kindForAssetOrMime(asset models.VideoAsset)` (kind first, MIME prefix
   fallback).
7. Renderer/validation: treat `layer` tracks correctly — per-clip behavior is
   already asset-kind-driven in `renderer.go`, so the main work is making
   validation's transform/volume defaults clip-kind-aware instead of
   track-type-aware (see `ValidateTimelineDocument`'s
   `track.Type != TrackTypeAudio` check).

### Frontend requirements

- `types/video.ts`: add `'layer'` to `VideoTimelineTrackType` (keep legacy values).
- `stores/videoStudio.ts`:
  - Default/new timelines use generic layers; `addTrack` defaults to `layer`.
  - Asset insertion targets the selected layer or first unlocked layer; stop
    routing by media type.
  - Add the missing layer actions (existing ones noted above are kept):
    `duplicateTrack`, `clearTrack`, `soloTrack` (UI-level solo is acceptable
    if schema solo is deferred), `selectClipsOnTrack`,
    `selectClipsBeforePlayhead` / `selectClipsAfterPlayhead`.
- Timeline components: label tracks as “layers” in user-facing text; any clip
  kind drops onto any unlocked layer; top layer = foreground (already true in
  preview and renderer).

### Acceptance criteria

- New projects create generic layers; legacy typed-track projects still load.
- Any supported asset kind can be placed on any unlocked layer.
- Top UI layer overlays lower layers in preview and export (already true —
  add a regression test for layer-type tracks).
- Muting a layer affects audio only; hiding affects visuals only (already
  true — extend tests to `layer` tracks).
- No existing project data destroyed.

---

## Phase 2 — Shared Context Menu System Everywhere ✅ Done (2026-06-10)

Implemented: `frontend/src/components/common/ContextMenu.tsx` (portal,
viewport-aware flip, `menu`/`menuitem` roles, arrow/Home/End/Enter/Escape
keyboard navigation, danger styling, shortcut labels, click-outside/scroll/
resize close) plus `useContextMenu` hook. All previous inline menus migrated.
Surfaces wired: clips (incl. clipboard, attributes, move-to-layer), layer
headers (rename/add above-below/duplicate/move/lock/hide/mute/solo/select-all/
clear/delete), empty lanes (time-aware paste, add text/marker here), ruler
(markers, go to start/end, split selected/all, select before/after, set
duration, zoom-to-fit), preview canvas (select-underneath, fit/fill/center/
reset, z-order, grid/safe areas), media-bin cards (add/rename/download/
send-to-chat/register-in-library/delete with in-use counts), inspector
effect/transition/keyframe rows, render jobs (copy error/diagnostics, retry,
cancel). Shift+F10/ContextMenu key opens menus on focused clips and asset
cards. **Remaining:** assistant-result menus (storyboard scenes/variants).

### Build

`frontend/src/components/common/ContextMenu.tsx` (new — no shared context-menu
component exists anywhere in the app today): right-click positioning,
keyboard open (`Shift+F10` / `ContextMenu` key), portal rendering,
viewport-aware placement, disabled items, separators, icons, shortcut labels,
danger styling, Escape/click-outside/scroll/resize close, `menu`/`menuitem`
roles, arrow-key navigation. Submenus optional — leave an extension point.

Menu definitions in `frontend/src/components/video/contextMenus.ts`.
Migrate the three existing inline menus to the shared component.

### Surfaces and commands

- **Media bin asset cards** (`VideoEditStudio.tsx`): add to timeline at
  playhead / to selected layer / to new layer, preview, rename, download,
  send to chat (`attach-to-conversation` route exists), register in file
  library (route exists), delete; per-kind extras (video → send to timeline;
  image → overlay layer; audio/music → add at playhead).
- **Layer headers** (`TimelineTrack.tsx`): rename, add above/below, duplicate,
  delete, move up/down/top/bottom, lock, hide, mute, solo, clear, select all
  clips on layer.
- **Clips** (`TimelineClip.tsx`): migrate existing menu; add copy/cut/paste,
  copy/paste attributes, move to layer above/below, set duration, fit/fill/
  center/reset transform (visual), mute clip/set volume (audio), edit text
  (text clips).
- **Empty lane** (`TimelineTrack.tsx`): paste here / at playhead, add text
  clip here, add marker here, add layer above/below. Convert click-x to time
  via `pxPerMs`.
- **Ruler/playhead** (`TimelineRuler.tsx`): add marker, go to start/end, set
  project duration to playhead, split selected/all at playhead, select clips
  before/after playhead, zoom to fit.
- **Preview canvas** (`VideoPreviewCanvas.tsx`): select top clip at point,
  select next underneath, fit/fill/center/reset, z-order, add title card,
  toggle grid/safe areas (toggles already exist as buttons).
- **Inspector rows** (`VideoInspector.tsx`): effect enable/disable/duplicate/
  move/remove; transition edit/remove; keyframe edit/duplicate/delete.
- **Render jobs** (`RenderJobStatus.tsx`): download, copy error, copy FFmpeg
  diagnostics, retry render, cancel.

### Acceptance criteria

- Menus on all listed surfaces; no interference with drag/drop or selection;
  keyboard-openable; disabled items visible; destructive items separated;
  commands reuse store actions.

---

## Phase 3 — Commercial Timeline Editing Fundamentals 🟡 Partial

**Exists (do not rebuild):** undo/redo (50-deep + Ctrl/Cmd+Z, Shift+Z, Y);
single + modifier multi-select; clip grouping; drag between tracks with snap
(clip edges, playhead, markers, 8px radius) and snap guides; trim handles;
`[`/`]` trim-to-playhead; S split; blade tool (C); nudge ←/→ (1 frame) and
Shift (10 frames); markers (M); zoom +/- and zoom-to-fit; Ctrl+wheel zoom;
adaptive ruler; Space play/pause; Ctrl/Cmd+S save; Delete/Backspace; help
overlay (?); align starts/ends; distribute evenly.

**Done (2026-06-10):** clipboard (`copySelection`/`cutSelection`/`pasteClips`
with relative-offset multi-paste + new IDs, `copyClipAttributes`/
`pasteClipAttributes`, Ctrl/Cmd+C/X/V), selection upgrades (Ctrl/Cmd+A,
before/after playhead, all-on-layer), layer ops (`duplicateTrack`,
`clearTrack`, `insertTrackAdjacent`, `moveTrackToEdge`, ephemeral
`toggleTrackSolo` honored by preview audio), `splitAllAtPlayhead`,
`setTimelineDuration`, `F` zoom-to-fit shortcut, updated help overlay.

**Also done (2026-06-11):** playhead-follow toggle (ruler context menu;
auto-scroll gates on it); **marquee select** — dragging on empty lane space
sweeps a selection rectangle over clips (DOM-rect intersection, suppresses
the trailing lane-click deselect).

**Remaining:** none.

---

## Phase 4 — Multi-Layer Preview Compositor ✅ Done

`VideoPreviewCanvas.tsx` renders all active clips at the playhead, sorted
bottom-to-top by track order + z-index, with transforms, opacity, crop, fades,
full text styling, click-to-select, scale/rotation/crop handles, thirds grid,
and safe-area overlays. Audio elements sync to playhead with volume + fade +
solo applied. The two former gaps closed 2026-06-10: right-click menus on
objects and the empty stage, and “Select next clip underneath” cycling.

---

## Phase 5 — Media Bin Asset Intelligence 🟡 Partial

**Exists:** grid/list toggle, search, sort (newest/name/duration/size),
filter tabs (All/Video/Image/Audio/Exports), image/video thumbnails in cards,
source/kind/duration/size line, inline rename, delete with “clips using it
will stop rendering” warning, download, drag-to-timeline
(`application/x-video-asset-id`).

**Done (2026-06-10):** `backend/internal/video/artifacts.go` generates
thumbnails (video poster frame / image downscale) and audio waveform PNGs
(`showwavespic`) next to the asset file, best-effort without FFmpeg; wired
into upload, generation output, external import, and render output; served at
`GET /v1/video/assets/{assetId}/artifacts/{thumbnail|waveform}` and removed on
asset delete. Probe now extracts video/audio codec, channels, and sample rate
into asset `metadata_json`. Media bin shows artifact thumbnails/waveforms and
an `in use ×N` badge; timeline audio clips render their waveform; delete
warnings include the live clip count. Card context menus shipped in Phase 2.

**Remaining:** none for this phase (pre-existing assets get artifacts only on
re-ingest — a backfill pass is optional future work).

---

## Phase 6 — Professional Inspector Controls 🟡 Partial

**Exists:** transform sliders (scale/opacity/rotation), z-order buttons,
audio volume + fade in/out, text content field, full effects manager
(add/toggle/reorder/remove/params), transitions (add/duration/direction/
remove), keyframe list editor (add at playhead, time/value/easing, remove),
canvas presets (16:9/9:16/1:1) + width/height/fps/background, renderer
capability warnings, the whole assistant section.

**Done (2026-06-10):** timecode-capable timing fields (start/end/duration/
trim-in, accepting `H:MM:SS.cc` or plain seconds), numeric X/Y fields,
fit/fill/center/reset transform actions (fill computed from asset vs. canvas
aspect ratios), text styling controls (size, weight, color, background +
clear, alignment, shadow toggle) with title-card / lower-third / subtitle
presets, and row context menus (effect enable/duplicate/reorder/remove,
transition remove, keyframe duplicate/delete).

**Remaining:** trim-out field (trim-in + duration cover the common case);
crop numeric fields (on-canvas crop mode exists); normalize / detach-audio
(blocked on backend support — intentionally no dead buttons).

---

## Phase 7 — FFmpeg Render Fidelity and Capability Reporting 🟡 Partial

**Exists:** capability matrix at `GET /v1/video/render/capabilities`
(`renderer_capabilities.go`, 16 features with partial/unsupported detail),
frontend warnings in inspector + render panel, bottom-to-top compositing,
x/y/scale/rotation/opacity/crop, visual + audio fades, text burn-in
(family/size/color/background/stroke/line-height), audio mixdown honoring
muted/hidden semantics and per-clip volume, fade/crossfade/dip_to_black
(as alpha fades), brightness/contrast/saturation/blur/grayscale/sharpen/
vignette, x/y position keyframes, diagnostics persisted on jobs, export
presets in the render panel, render history (last 3), startup job recovery.

**Done (2026-06-10):** the visual-stacking bug fix (export now composites by
layer order + z-index instead of start time, with text interleaved — see
Phase 1); solo resolved as a preview-only monitoring control with a truthful
capability note; capability notes updated (`clip_ordering`, `track_solo`);
diagnostics polish — render-job context menu with copy error / copy FFmpeg
diagnostics / copy job ID / retry / cancel.

**Done (2026-06-11):** volume keyframes export via frame-evaluated `volume`
expressions; **rotation keyframes** export via per-frame `rotate` angle
expressions in a fixed diagonal bounding box; slide transitions export as
animated overlay positions (enter from the chosen edge, exit the opposite);
chroma key exports via `chromakey` (green default, `color`/`similarity`/
`blend` params — preview shows the unkeyed frame since CSS can't key); audio
from video clips now joins the mixdown (`has_audio` recorded at ingest,
ffprobe fallback, per-asset cache).

**Remaining work (in priority order):**

1. **Keyframes at export** — scale and opacity (no clean FFmpeg per-frame
   path: scale expressions don't take `t`, alpha would need slow `geq`;
   intentionally preview-only with truthful capability flags).
2. **Transitions at export** — wipe/zoom (true `xfade` directional work).
3. **Effects at export** — shadow, background_blur (likely stay preview-only).
4. **Text fidelity** — letter-spacing, border-radius, per-line alignment are
   preview-only today; close or keep documented.

---

## Phase 8 — Captions, Callouts, Annotations, and Audio Workflows 🟡 Partial

**Captions — done:** SRT + VTT import/export, segment text/timing editing,
split at playhead, merge with next, three style presets, burn-in at export.
**Captions — remaining:** transcription flow (button is a disabled “soon”
placeholder — wire to an STT provider only if one exists in the app; none
does today, so this is gated on adding one).

**Callouts/annotations — done (2026-06-11):** parameterized `clip.shape`
payload (`rectangle` outlined / `highlight` filled / **`blur` redaction
region**, dimensions + fill/stroke/blur-radius, position/scale/opacity via
the clip transform, optional centered text label); creation from the
preview-canvas and lane context menus; inspector controls; preview rendering
(blur via CSS backdrop-filter); export via FFmpeg `drawbox` and a
split→crop→boxblur→overlay subgraph (validated + tested).
**Callouts — remaining:** arrows, lines, speech bubbles, spotlight/dim
(drawbox/drawtext can't draw diagonals or rounded bubbles — would need
generated overlay images).

**Audio — exists:** clip volume + volume keyframes (preview + export), fade
in/out, layer mute, **clip mute** (`muted`, independent of volume),
**detach audio from video** (`audio_only` twin on a new layer, original goes
silent), **background-music helper** ("Add as music bed" places the asset
full-length on the bottom layer), timeline waveforms, music-studio import.
**Audio — remaining:** level meters, ducking/normalize (later;
capability-flag honestly).

---

## Phase 9 — Timeline-Aware Assistant ✅ Done (extensions optional)

**Exists:** `CreateEditPlan` with LLM + deterministic fallback; context
summary includes canvas, tracks, clips, assets, selection, playhead, renderer
capabilities; `ValidateEditPlanOperations` separates valid ops + previews +
issues; apply requires explicit user action and enters the undo stack; quick
workflows (30s cut, 15s teaser, 9:16, 1:1, title card, lower third, captions,
tighten pacing); storyboard + social variants endpoints.

**Done (2026-06-11):** `set_volume` and `add_marker` op types (validated,
previewed, applied, in the LLM prompt schema); context menu on the edit-plan
result (apply / copy plan JSON / copy summary / dismiss); **per-operation
selective apply** — `annotatePlan` now returns the validated operation subset
so `operations[i] ↔ preview[i]`, and the editor renders per-line checkboxes
("Apply N selected").

**Also done (2026-06-11):** `add_asset_clip` and `set_transform` op types
(validated, previewed, applied, in the LLM prompt schema); **variant→project**
— social-variant chips open a menu that duplicates the project (backend asset
copy) and applies the variant's plan to the copy.

**Optional extensions:** `add_caption_segments` op type.

---

## Phase 10 — Commercial Polish, Project Workflow, and Reliability 🟡 Partial

**Exists:** resizable side panels, editor modes (full/simple_trim/
captions_only/social_clip), timeline help overlay (?), per-mutation explicit
saves with in-flight dedup (`_saveSeq`), render-job startup recovery (marks
interrupted jobs failed), cancellation, delete-asset warning, last-3 render
history.

**Done (2026-06-10/11):** fullscreen preview toggle (Fullscreen API on the
preview root), persistent panel widths (`useResizablePanels` gained an
optional localStorage `storageKey`), a visible Saving…/Saved autosave
indicator, **collapsible side panels** (chevron collapse/expand strips),
**duplicate project** (backend asset copy + timeline remap with preserved
clip IDs, via `POST /v1/video/projects/{id}/duplicate` and the project-strip
context menu), **storage accounting** (media-bin header shows total asset
bytes per project), and **lazy artifact backfill** (the artifact endpoint
generates and persists missing thumbnails/waveforms on first request for
pre-existing assets).

**Remaining work:**

- UI: tooltips audit, shortcut help reachable from the toolbar (not just `?`).
- Workflow: duplicate timeline/version + revert checkpoints (needs timeline
  versioning endpoints), project notes panel.
- Reliability: orphaned file cleanup, missing-media
  placeholder handling in preview/timeline, clearer FFmpeg-missing messaging
  at render start (capability endpoint already reports availability).
- Performance: throttle drag/trim store writes (save on interaction end —
  currently every mutation awaits `saveTimeline`), memoize clip layout,
  virtualize media bin if large.
- Accessibility: menu roles + focus management (Phase 2 component), visible
  focus rings on timeline elements.

---

## Testing Requirements

### Backend Go tests (extend the existing 9 files)

- Generic layer creation; legacy typed timelines still load (extend
  `timeline_test.go`).
- Any asset kind onto any layer; locked layer rejects insert.
- Layer-type tracks: ordering, mute-audio-only, hide-visuals-only in render
  graph (extend `renderer_fidelity_test.go`).
- Capability matrix stays truthful as features land
  (`renderer_capabilities` test exists).
- Thumbnail/waveform generation (skip when FFmpeg absent — follow
  `renderer_test.go`'s pattern).

### Frontend tests

**Done (2026-06-11):** vitest is configured (`frontend/vitest.config.ts`,
node environment, `npm run test:unit`) with store-level coverage in
`src/stores/videoStudio.test.ts` — selection (all / relative-to-playhead /
per-layer), clipboard copy/cut/paste with relative offsets, layer
duplicate/insert/clear, clip mute, detach audio, split-all, and undo.

### Playwright smoke

`tests/video-editor.smoke.spec.ts` exists. Extend with: right-click menus
(clip duplicate/delete via menu), drag asset to a layer, multi-layer preview
spot-check, render start/graceful-fail.

---

## Validation Commands

```bash
cd backend && go test ./...
cd frontend && npm run build && npm run lint
npm run test:smoke   # repo root; boots its own backend on :8090
```

Report failures honestly, including pre-existing unrelated ones.

---

## Documentation Updates

Update as implementation lands: `docs/VIDEO_STUDIO.md`,
`docs/VIDEO_TIMELINE_SCHEMA.md` (layer type, solo flag),
`docs/VIDEO_RENDERING.md` (capability truth), and this file's status markers.

---

## Implementation Order (remaining work only — updated 2026-06-10)

Everything tractable on this plan has landed on
`feat_video_studio_enhancements` across 2026-06-10/11: phases 1–6 in full,
and phases 7–10 except the items below (backend `go test ./...`, frontend
`npm run build` + `npm run lint` + `npm run test:unit` (vitest, 12 store
tests), and `npm run test:smoke` (5 specs incl. context-menu/clipboard) all
green). The intentionally-deferred remainder:

1. Phase 7 — scale/opacity keyframes at export (no clean FFmpeg per-frame
   path; capability-flagged as preview-only) and wipe/zoom transitions
   (junction-based `xfade` doesn't fit the per-clip transition model).
2. Phase 8 — arrow/line/speech-bubble callouts (need generated overlay
   images), level meters, ducking/normalize.
3. Phase 9 — `add_caption_segments` op type.
4. Phase 10 — timeline versioning (duplicate timeline / revert checkpoints
   need new endpoints), project notes panel, orphaned-file cleanup,
   tooltips audit.

## Definition of Done

- New timelines use generic layers; legacy projects load unchanged.
- Any media on any unlocked layer; top layer overlays lower layers in
  preview and export.
- Context menus on all major surfaces, keyboard-accessible.
- Clipboard, selection upgrades, solo/duplicate/clear layer work and are
  undoable.
- Thumbnails/waveforms populate and display; probe captures codec/audio.
- Inspector edits timing (timecode), transforms (numeric + fit/fill/center),
  and text styling.
- Capability matrix stays truthful; renders carry actionable diagnostics.
- Tests and docs updated; existing data backward-compatible.
