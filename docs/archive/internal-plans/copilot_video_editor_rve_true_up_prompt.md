> **Archived — superseded.** Later renderer and scalability work invalidated parts of this 2026-06 true-up. Use [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Video Edit Studio — React Video Editor Capability True-Up (Living Plan)

_Last verified against the codebase: 2026-06-10 (branch `feat_video_studio_enhancements`)._

This document is the working plan for upgrading **Video Edit Studio** toward functional parity with the public feature set of **React Video Editor** (`reactvideoeditor.com`, `demo.reactvideoeditor.com`), while preserving OmniLLM-Studio's architecture: local-first model, Go backend (Chi + SQLite), React/TypeScript frontend (Zustand), neutral timeline JSON, and FFmpeg render jobs.

Do **not** copy proprietary React Video Editor code, private SDK code, or licensed assets. Implement equivalent capabilities natively using existing codebase conventions.

> **Keep this file up to date.** Every implementation session must update the phase statuses and the capability matrix below. Companion status doc: `docs/internal_docs/video_studio_next_phases.md` (Phases 1–7 of the broader Video Studio effort; its Phase 3/4 overlap this document's baseline).

---

## Corrections to the original draft (audit, 2026-06-10)

The first version of this document made assumptions that did not survive a code audit. They are recorded here so they are not re-introduced:

1. **The renderer gap was overstated.** The draft cited `docs/VIDEO_RENDERING.md` claiming FFmpeg export does not apply effects, transitions, opacity/fades, audio mixing, or per-clip transforms. That doc was stale; the Phase 4 fidelity work (committed on this branch) already renders: position/scale (`transform.x/y/scale`), fractional crop, opacity (`colorchannelmixer`), video+audio fade in/out, fade-style transitions (`fade`/`crossfade`/`dip_to_black` as alpha fades), effects `brightness`/`contrast`/`saturation`/`blur`/`grayscale`, per-clip volume, multi-track `amix`, and hidden/muted track semantics. **Actual remaining export gaps: keyframes, rotation, slide/wipe/zoom (true `xfade`), `chroma_key`/`shadow`/`background_blur` effects, track solo.** `docs/VIDEO_RENDERING.md` has been corrected as part of this true-up.
2. **The proposed crop shape would break existing data.** Draft proposed `crop: {x, y, width, height}`; the implemented and *rendered* shape is fractional `{top, right, bottom, left}` (0–0.95 each). Keep the existing shape.
3. **The proposed effect/transition unions silently dropped existing types.** `shadow`, `background_blur` (effects) and `dip_to_black` (transition) exist in saved timelines and in the TS unions. All schema changes must be additive. `dip_to_color` is achieved as a param on dip-style transitions, not a replacement type.
4. **Keyframe values stay numeric.** Draft proposed `value: number | string | boolean` and free-string `property`. Go stores `Value float64`; every animatable property today is scalar. No consumer needs non-numeric keyframes — do not widen the type until one does.
5. **No per-clip `type` field.** Clip type derives from its track type everywhere (preview, renderer, validation). Mixed-type tracks add renderer complexity with no user-visible parity gain.
6. **No `templates` array inside the timeline document.** Templates are factories that *produce* timeline documents. Embedding template metadata in every saved timeline pollutes the source of truth.
7. **No `editor_config` inside the timeline document.** The draft itself required that editor modes "hide/show feature groups without changing persisted timeline semantics" — persisting mode in the canonical timeline contradicts that. Editor mode is frontend state (per-project `localStorage`, or later `video_projects.metadata_json`).
8. **Captions need no new clip payload.** `caption` is already a track type, and caption clips already render in preview and export (`drawtext`). Model each SRT/VTT cue as one clip on a caption track; import/export is a pure mapping layer. Styling rides the existing text payload.
9. **One styling home, not two.** Draft proposed a parallel `style` object next to the existing `text` payload (which already has `font_size`, `font_weight`, `color`, `background`, `stroke`, `shadow`). Extend `TimelineText` instead (`font_family`, `text_align`, `line_height`, `letter_spacing`, `stroke_width`, `border_radius`) — the renderer consumes one payload and the inspector edits one payload.
10. **No version bump while changes are additive.** All Phase 1 schema changes are optional fields with `omitempty`; v1 documents remain valid v1 documents. An upgrader entry point exists for the future, and unknown versions fail with an actionable error. Bump to v2 only on the first breaking semantic change.
11. **Remotion render worker (draft "path B") is rejected for now.** It adds a Node runtime dependency that conflicts with the single-binary Wails desktop distribution. The richer-FFmpeg-graph path is the strategy; the `video.Renderer` interface already exists for a future adapter.
12. **3D transforms are preview-only by design.** FFmpeg has no practical per-layer 3D perspective compositing. If/when added, they are CSS-preview features with explicit `supported: false` renderer-capability entries and UI warnings. Deferred to last.
13. **`AssetPanel` is not a separate file** — it is a component inside `VideoEditStudio.tsx`. The media bin already has grid/list toggle, type filters, thumbnails, rename, delete, drag-to-timeline, and source metadata. Remaining: search, sort, and a local upload button inside the *edit* studio (upload exists in the generation workspace `VideoStudio.tsx`).
14. **`npm run test:smoke` is an explicit file list** (`image-editor-toolbar` + `music-studio` specs, chromium only). A new `tests/video-editor.smoke.spec.ts` must be added to that script in the root `package.json` to run under it.
15. **Metadata extraction gap is real but narrower than drafted.** Upload already MIME-sniffs, enforces per-kind size limits, and extracts image dimensions. Missing: `ffprobe`-based duration/dimensions/FPS/codec for video/audio and poster thumbnails. Upload must keep working when `ffprobe` is absent.

---

## Verified current state (2026-06-10)

### Files (confirmed)

Frontend — `frontend/src/components/video/`: `VideoEditStudio.tsx` (contains `AssetPanel` + `ProjectStrip`), `VideoPreviewCanvas.tsx`, `VideoInspector.tsx`, `VideoRenderPanel.tsx`, `RenderJobStatus.tsx`, `VideoStudio.tsx` (generation workspace), `timeline/{VideoTimeline,TimelineToolbar,TimelineTrack,TimelineClip,TimelineRuler,TimelinePlayhead}.tsx`. State: `frontend/src/stores/videoStudio.ts`. Types: `frontend/src/types/video.ts`. API: `frontend/src/api.ts` (`videoApi`).

Backend — `backend/internal/video/`: `timeline.go` (document model + validation), `renderer.go` (FFmpeg), `renderer_capabilities.go`, `validation.go` (generation preflight), `service.go`, `assistant.go`, provider adapters (`gemini_provider.go`, `openrouter_provider.go`, `luma_provider.go`). Handler: `backend/internal/api/video_handler.go`. Routes wired in `backend/internal/api/router.go` (≈39 endpoints under `/v1/video/...`, including `GET /v1/video/render/capabilities`).

Docs — `docs/VIDEO_STUDIO.md`, `docs/VIDEO_STUDIO_ARCHITECTURE.md`, `docs/VIDEO_TIMELINE_SCHEMA.md`, `docs/VIDEO_RENDERING.md`, `docs/VIDEO_PROVIDER_ADAPTERS.md`.

### Timeline document (v1, `timeline.go` / `types/video.ts`)

- Document: `version` (1), `canvas {width,height,fps,background}`, `duration_ms`, `tracks[]`, `markers[]`, `metadata{}`.
- Track: `id`, `type` (`video|image|audio|music|text|caption|shape|callout`), `name`, `locked`, `muted`, `visible`, `clips[]`. Track order = array order.
- Clip: `id`, `asset_id?`, `start_ms`, `duration_ms`, `trim_in_ms`, `trim_out_ms`, `transform? {x,y,scale,rotation,opacity,crop{top,right,bottom,left}}`, `volume?`, `fade_in_ms?`, `fade_out_ms?`, `text?`, `effects[]`, `transitions[]?`, `keyframes[]`.
- Backend validation: version==1, canvas defaults, unique track/clip IDs, non-negative start, positive duration, trim sanity, auto-IDs, duration recompute. **Not yet validated: marker sanity, effect/transition type whitelists, keyframe property/time sanity, z-order.**

### What already works (do not rebuild)

- Multi-track timeline: drag/move (grab-offset preserved), trim handles, split at playhead, duplicate, delete, multi-select (Ctrl/Cmd/Shift-click), snapping (clip edges + playhead, toggleable), zoom + zoom-to-fit, undo/redo (50-deep, covers assistant plans), track mute/lock/visibility toggles, media-bin drag-to-track.
- Shortcuts: `Space` play/pause, `Ctrl/Cmd+Z` undo, `Ctrl/Cmd+Shift+Z`/`Ctrl/Cmd+Y` redo, `Delete`/`Backspace` delete, `Ctrl/Cmd+S` save, `S` split, `Escape` deselect.
- Media bin: grid/list, type filters, image/video thumbnails, rename (`PATCH /v1/video/assets/{id}`), delete, source studio/type/duration/size metadata.
- Inspector: transform sliders (scale/opacity/rotation), volume, fades, text editing, add/toggle/remove effect, add transition, add keyframe, capability-derived export warnings, assistant panel with quick workflows.
- Render: FFmpeg jobs (persistent, cancellable, recovered-on-restart), export presets (project/720p/1080p/16:9/9:16/1:1/custom), V41 diagnostics metadata (command + stderr), progress events; fidelity as listed in correction #1.
- Assistant: storyboard, timeline-plan, edit-plan (timeline-context-aware, validated, partial-tolerant apply, per-op previews), social variants; deterministic fallbacks; ops: `set_canvas`, `set_duration`, `add_text_clip`, `move_clip`, `trim_clip`, `delete_clip`.

### Real gaps vs React Video Editor baseline

| # | Capability area | Current state | Gap |
|---|---|---|---|
| G1 | Track management | Fixed 4 default tracks; toggles only | No add/remove/rename/reorder/height UI or store actions |
| G2 | Markers | In schema; no UI/actions | Add/remove/jump UI on ruler |
| G3 | Layer order | Track array order only | No clip `z_index`, no bring-forward/send-backward |
| G4 | Grouping | — | No `group_id`, no group/ungroup |
| G5 | Inspector depth | **Done 2026-06-10**: registry pickers, effect param sliders, transition duration/direction editors, keyframe time/value/easing editors, per-type export chips | Effect reorder UI (deferred) |
| G6 | Effects/transitions registry | **Done 2026-06-10**: `effects/{effectRegistry,transitionRegistry,keyframeUtils}.ts` | — |
| G7 | Preview compositing | Renders **only the topmost active clip** at playhead | Full multi-layer compositing in track/z order |
| G8 | On-canvas editing | Read-only preview | Selection box, drag/resize/rotate handles, crop mode, safe-area/grid guides |
| G9 | Captions | **Done 2026-06-10**: segment editor panel, SRT/VTT import/export, style presets | AI transcription (deferred — no provider) |
| G10 | Keyframe export | Position (x/y) exports with linear interpolation (2026-06-10); full preview animation | Scale/rotation/opacity/volume keyframes + easing at export |
| G11 | Rotation export | **Done 2026-06-10** (`rotate` with transparent fill) | — |
| G12 | Directional/true transitions | Alpha-fade approximation | `xfade` for slide/wipe/zoom where clips abut |
| G13 | Text style depth | Full styling in preview; font family/stroke width/line spacing also export (2026-06-10) | Letter spacing, radius, alignment at export (drawtext limits) |
| G14 | Media metadata | **Done 2026-06-10** (ffprobe duration/dims/FPS on upload, graceful fallback) | Poster thumbnails; probe on cross-studio imports |
| G15 | Media bin | **Done 2026-06-10**: filters, thumbnails, rename/delete, upload (button + drag-and-drop), search, sort | — |
| G16 | Templates | **Done 2026-06-10**: 6 starter templates via header menu | Before/after template; save-as-template |
| G17 | Editor modes | **Done 2026-06-10**: full / simple-trim / captions-only / social-clip via header selector, persisted in localStorage; UI-only (timeline semantics untouched) | Per-project persistence if ever needed |
| G18 | Extra shortcuts | Core set exists | `+`/`-` zoom, `M` marker, arrow nudge, Shift+arrow big nudge, `[`/`]` trim-to-playhead |
| G19 | Canvas controls | Set at project creation only | Runtime canvas patch (size/FPS/background) via store + inspector |
| G20 | 3D transforms | — | CSS-preview-only fields + parallax presets, capability-flagged unsupported in export (deferred, last) |
| G21 | Smoke test | **Done 2026-06-10**: `tests/video-editor.smoke.spec.ts` in `test:smoke` (open → project → bin/timeline → text clip → inspector → captions/export panels → save → caption segment) | — |
| G22 | Track solo | Not in schema | Schema field + renderer + UI (low priority) |

---

## Schema extensions (additive, version stays 1)

Mirror in `backend/internal/video/timeline.go` (Go, `snake_case` tags) and `frontend/src/types/video.ts`. All fields optional with `omitempty`; existing documents stay valid.

```ts
// Track additions
height?: number;            // UI row height px (clamped 32–160)

// Clip additions
z_index?: number;           // compositing order within overlapping clips (default 0)
group_id?: string;          // multi-select grouping

// VideoTimelineText additions (single styling home — correction #9)
font_family?: string;       // preview always; export best-effort font match
text_align?: 'left' | 'center' | 'right';
line_height?: number;
letter_spacing?: number;    // preview-only (drawtext has no tracking) — capability-flagged
stroke_width?: number;
border_radius?: number;     // preview-only — capability-flagged

// New effect types (additive): 'sharpen' (FFmpeg unsharp), 'vignette' (FFmpeg vignette)
// New keyframe easing (additive): 'step'
// Transitions: keep existing union; 'dip_to_black' gains optional params.color → dip-to-color
```

New backend validation (Phase 1): marker time ≥ 0 + auto-ID + sort, effect/transition type whitelists (unknown → validation error), transition ID auto-gen + uniqueness per clip, keyframe property whitelist (`x|y|scale|rotation|opacity|volume`) + time ≥ 0 + clamp into clip duration, `z_index` finite, group_id trimmed. Plus `UpgradeTimelineDocument` scaffolding: version 0→1 default (exists), version >1 → actionable error (exists as rejection; keep message useful).

---

## Implementation phases & status

Statuses: ☐ not started · ◐ in progress · ☑ done (with date).

### Phase 0 — Audit & true-up doc ☑ 2026-06-10
This document. Capability matrix above; renderer strategy: **richer FFmpeg graph, no Remotion** (correction #11). `docs/VIDEO_RENDERING.md` corrected to match the code.

### Phase 1 — Schema, types, validation ☑ 2026-06-10
- ☑ Go (`timeline.go`): track `height` (clamped 32–160); clip `z_index`, `group_id` (trimmed); `TimelineText` gains `font_family`, `stroke_width`, `text_align`, `line_height`, `letter_spacing`, `border_radius`; effect/transition type constants incl. `sharpen`/`vignette`; `CurrentTimelineVersion` + `UpgradeTimelineDocument` (future versions fail with actionable message); validation for markers (auto-ID, clamp, trim, sort, dup-ID error), effect/transition/keyframe type whitelists, per-clip effect/transition/keyframe ID uniqueness, positive transition durations, non-negative keyframe times, unknown easing → `linear`.
- ☑ Go tests (`timeline_test.go`): future-version rejection, unknown effect/transition/keyframe rejection, zero-duration transition rejection, negative keyframe time rejection, new-field normalization (height clamp, z_index/group_id, type normalization, marker clamp/sort/trim), legacy v1 doc passthrough.
- ☑ TS (`types/video.ts`): mirrored all additions; keyframe easing union gains `'step'`; effect union gains `'sharpen' | 'vignette'`.
- ☑ `docs/VIDEO_TIMELINE_SCHEMA.md` updated.
- Note: keyframe `time_ms` is validated ≥ 0 but not clamped to clip duration. ~~Semantics pinned down in Phase 6~~ **Decided in Phase 5 (2026-06-10): `time_ms` is clip-relative** — the inspector now writes playhead-minus-clip-start, and the preview samples accordingly. Phase 6's FFmpeg keyframe expressions must use the same semantics.

### Phase 2 — Timeline UX parity (G1, G2, G3, G4, G18, G19) ☑ 2026-06-10
- ☑ Store actions (all push undo, clear redo, recompute duration where relevant, keep selection coherent): `addTrack`, `removeTrack`, `renameTrack`, `reorderTrack`, `setTrackHeight`, `addMarker`, `removeMarker`, `updateClipZIndex`, `bringClipForward`, `sendClipBackward`, `updateClipTransition`, `removeClipTransition`, `updateClipEffect`, `updateKeyframe`, `removeKeyframe`, `nudgeSelection`, `setCanvas`, `splitClipAt` (generalized from playhead-only), `trimClipEdgeToPlayhead`, `groupClips`, `ungroupClips`, `alignSelection` (start/end/distribute), `setToolMode`. (Text styling rides the existing `updateClipText`, whose payload now includes the new style fields.)
- ☑ Timeline UI: "+ Track" menu (corner cell, 6 track types); track header double-click rename + right-click context menu (rename, move up/down, height ±16px, remove with confirm when clips exist); per-track `height` applied to row.
- ☑ Markers: amber flags on the ruler — `M` adds at playhead, click jumps, right-click removes; markers join the snap-point set.
- ☑ Clip context menu (right-click): split at playhead, trim start/end to playhead, duplicate (selection-aware), bring forward/send backward, add fade in/out, add fade transition, group/ungroup, align starts/ends, distribute (3+), delete (selection-aware).
- ☑ Grouping: `group_id` assigned via menu; non-additive click on a grouped clip selects the whole group, so nudge/delete/duplicate act on the group. Caveat: pointer-dragging one grouped clip moves only that clip — group-drag is future polish.
- ☑ Tools: `V` select / `C` blade (click a clip to split at that point), toolbar buttons with active state, crosshair cursor in blade mode.
- ☑ Shortcuts: `M` marker, `+`/`-` zoom, `[`/`]` trim edge to playhead, `C`/`V` tools, `?` help, Arrow nudge ±1 frame, Shift+Arrow ±10 frames (frame size from canvas FPS; locked tracks skipped). `?` opens a shortcut-help overlay (also via toolbar button); Escape closes menus/help, then deselects.
- ☑ Snap guide: a vertical guide line renders during drag-over at the snapped drop position (cursor-based; the committed drop additionally honors the grab offset).
- ☑ Ruler: 250ms sub-second ticks at high zoom (labels stay on whole seconds). True frame-number ticks aren't feasible at the current zoom ceiling (max 0.08 px/ms ≈ 2.7px per 30fps frame) — revisit if the zoom range is raised.
- ☑ Canvas controls: inspector shows a Canvas section when no clip is selected — 16:9/9:16/1:1 presets, width/height/FPS (commit on blur), background color picker — driving `setCanvas`.
- ☑ Inspector: layer-order Forward/Backward buttons (shows `z_index`); effect rows toggle on click + remove; transition rows remove; keyframe rows listed (property @ time = value) + remove.
- ☐ Deferred polish: drag-to-resize track height (menu ±16px exists), group-aware pointer drag.

### Phase 3 — Preview compositing & on-canvas editing (G7, G8) ☑ 2026-06-10
- ☑ `VideoPreviewCanvas` rewritten to composite **all** visible visual tracks at the playhead, stacked by track order then `z_index` (DOM paint order). Only clips active at the playhead mount media elements.
- ☑ Per-layer fidelity: position/scale/rotation/opacity from the transform, fade-in/out opacity modulation matching export semantics, fractional crop via `clip-path: inset(...)`, full text styling (font family/size/weight, color, background + radius + padding, stroke + width, shadow, align, line-height, letter-spacing) scaled to the stage.
- ☑ Semantics fix: muted tracks now still **show** video (muting only silences their `<video>` audio), matching the renderer's hide-drops-video / mute-drops-audio rules — previously muted tracks vanished from preview entirely.
- ☑ Multi-video playback: every mounted clip video seeks to its trim offset on play start and re-seeks while scrubbing.
- ☑ Stage is sized to the exact canvas aspect ratio via ResizeObserver, so on-canvas edits convert pointer deltas to canvas pixels exactly.
- ☑ On-canvas editing (paused only): click selects (syncs timeline/inspector), drag moves, corner handle scales (0.05–4, matching renderer clamp), top handle rotates; edits preview live and commit once on release (single undo entry + save); locked tracks are not editable.
- ☑ Guides: rule-of-thirds grid toggle, action/title safe-area + centerline toggle in the preview header; clicking empty stage deselects.
- ☑ On-canvas crop mode: header Crop button (enabled for an unlocked, selected video/image clip at the playhead) shows the full frame with dimmed margins, a dashed kept-region outline, and four edge handles; drags clamp each side to 0–0.95 with ≥10% of the frame kept, commit once on release, and "Reset crop" clears it. Rotation is suppressed while editing so handle math stays axis-aligned. Caveat (pre-existing preview semantics): crop insets apply to the element box including any letterbox, while export crops the source frame — identical when asset and canvas aspect ratios match.
- ☑ Snap-to-center: while dragging a layer (and the global snap toggle is on), x/y snap to canvas center within an 8-stage-px threshold, with primary-colored centerline guides shown while engaged.
- ☑ Media cap: only the topmost 4 video layers mount real `<video>` elements; deeper video layers render a lightweight "preview capped — layer renders in export" card. Images/text are uncapped (cheap).

### Phase 4 — Captions (G9) ☑ 2026-06-10
- ☑ `components/video/captions/captionUtils.ts`: SRT + WebVTT parser (BOM-tolerant, skips WEBVTT/NOTE/STYLE/REGION blocks, drops VTT cue settings, skips malformed blocks, sorts cues) and SRT/VTT serializers; caption style presets (Subtitle, Bold social, Lower third) defined as text-payload patches + canvas-relative position offsets.
- ☑ Store actions: `addCaptionSegment` (2s clip at playhead on a found-or-created caption track, subtitle-preset styling), `importCaptions` (cues → one clip each), `exportCaptions('srt'|'vtt')` (collects caption-track clips → serialized string), `mergeCaptionClipWithNext` (extends duration, joins text), `applyCaptionPreset` (restyles + repositions every caption clip). All undo-aware.
- ☑ `VideoCaptionPanel.tsx` in the right sidebar (between inspector and render panel): segment list sorted by time with inline text (commit-on-blur textarea), start/end second fields, split-at-playhead, merge-with-next, delete, row click selects + seeks; Add-at-playhead, Import (.srt/.vtt file picker), Export SRT/VTT (blob download); preset buttons; AI captions rendered as an explicitly disabled "coming soon" button (no transcription provider exists).
- Captions are ordinary clips on `caption` tracks (correction #8) — they already render in preview (Phase 3 compositing) and burn into export (`drawtext`), so no schema or backend change was needed.

### Phase 5 — Effects/transitions/keyframes UI + registry (G5, G6) ☑ 2026-06-10
- ☑ `effects/effectRegistry.ts`: all 10 effect types with label, `exportSupported` (tracks `renderer.go`: brightness/contrast/saturation/blur/grayscale true, rest false), slider param metadata with defaults, and a CSS-`filter` preview mapping (`composePreviewFilter`). `effects/transitionRegistry.ts`: all 6 transition types with export support/notes, direction support, default durations. `effects/keyframeUtils.ts`: property/easing constants and `sampleKeyframes` interpolation (hold-before/after, per-segment easing incl. `step`).
- ☑ Preview now applies **enabled effects as CSS filters** and **animates keyframed x/y/scale/rotation/opacity** (clip-relative sampling; an in-flight canvas drag overrides keyframes).
- ☑ Inspector: hardcoded add-buttons replaced with registry-driven pickers (effect picker with defaults, transition picker with default duration/direction, keyframe-at-playhead picker capturing the property's current value); effect rows gained param sliders + "preview only" chips; transition rows gained duration (commit-on-blur) and direction editors + alpha-fade approximation chips; keyframe rows gained time/value/easing editors. Feature-level warnings stay derived from the renderer-capabilities endpoint.
- Effect reordering within a clip is still not exposed in the UI (rarely needed; `updateClipEffect` + array order is the hook if it becomes needed).

### Phase 6 — Render/export parity (G10, G11, G12, G13-export, G22) ◐ 2026-06-10
- ☑ **Rotation** — `rotate=<rad>:c=black@0:ow=rotw:oh=roth` after `format=rgba` (transparent expanded corners); transform `rotation` parsed mod 360.
- ☑ **Sharpen + vignette** — `unsharp=5:5:<amount 0–3>` and `vignette=a=<strength·π/2>`; frontend registry flags flipped to `exportSupported: true`.
- ☑ **Position (x/y) keyframes** — piecewise-linear `overlay` x/y time expressions (`positionKeyframeExpr`): clip-relative `time_ms` converted to output-timeline seconds, hold before first/after last, comma-escaped for the filter graph. Easing is linearized at export (capability note says so).
- ☑ **Drawtext styling** — `font='<family>'` (fontconfig best-match, degrades safely), `borderw=<stroke_width 1–20>`, `line_spacing=(line_height−1)·fontsize` (clamped).
- ☑ Capabilities updated (rotation supported; keyframes partial; effects/text notes) + fidelity tests for all of the above (`TestBuildFilterComplexRotationNewEffectsAndPositionKeyframes`).
- ☐ Remaining: keyframed scale/rotation/opacity/volume at export (needs `zoompan`/`geq`-class work or a smarter segmenting approach), true `xfade` for slide/wipe/zoom, chroma-key (`chromakey` filter is feasible — schedule next), track solo (schema + UI + mix rules).
- ☐ Reliability (found 2026-06-10): audit and enable SQLite `foreign_keys` — the schema declares ~48 FK constraints that have never been enforced (driver ignored the old DSN param); requires checking delete paths and existing orphaned rows first.

### Phase 7 — Metadata, media bin, templates, modes (G14, G15, G16, G17) ◐ 2026-06-10
- ☑ `ffprobe` extraction on upload (`probe.go`: duration/dimensions/FPS from `-show_format -show_streams` JSON, rational frame rates, 10s timeout, returns nil-without-error when ffprobe is absent) wired into `UploadAsset` for video/audio — dragged clips now land with their real duration. Parse-layer unit tests included.
- ☑ Media bin search (file-name substring) + sort (newest/name/duration/size), and **drag-and-drop file upload** onto the bin (highlight on drag-over) alongside the upload button.
- ☑ Starter templates (`templates/timelineTemplates.ts`): Blank 16:9, 9:16 Short/Reel (hook title + caption track), 1:1 Square, Title + Lower Third, Captioned Talking Head, Image Slideshow (slide markers) — all produce real timeline JSON via `createProjectFromTemplate`, surfaced in a header Templates menu.
- ☑ Editor modes (G17, landed 2026-06-10): `editorModes.ts` defines full / simple-trim / captions-only / social-clip with per-feature flags (assistant, transform controls, effect controls, captions panel, templates, canvas controls, add-track, add-text); header selector persists to localStorage; gates apply in the studio header, inspector, captions panel, and timeline "+ Track". UI-only — persisted timelines are identical across modes.
- ☐ Remaining: poster thumbnail generation (media bin currently uses `<video preload="metadata">`), probe on cross-studio imports, before/after comparison template (needs assets to be meaningful).

### Phase 8 — Tests, docs, polish (G21) ◐ 2026-06-10
- ☑ `tests/video-editor.smoke.spec.ts` added to `test:smoke`/`test:smoke:headed`: opens the studio, creates a project, asserts media bin/timeline/save controls, adds a text clip, checks inspector controls (Scale, Layer order), asserts Captions + Export panels, saves, and adds a caption segment. Passing.
- ☑ Backend validation tests (Phase 1) and renderer capability/fidelity tests (Phases 1/6) in place; `docs/VIDEO_TIMELINE_SCHEMA.md` and `docs/VIDEO_RENDERING.md` kept current throughout.
- ☑ `docs/VIDEO_STUDIO.md` refreshed 2026-06-10: Edit Studio capability list rewritten for the new surface (compositing preview, captions panel, registries, templates, media bin), stale "effects/transitions not exported" rendering note replaced with the capabilities-endpoint reference, and the upload-endpoint description corrected (per-kind limits, MIME sniffing, ffprobe enrichment).
- ☐ Remaining: `docs/VIDEO_STUDIO_ARCHITECTURE.md` pass if it drifts.

### Commercial-UX polish pass ☑ 2026-06-10
Beyond the tracked gaps, a dedicated polish sweep added: **sticky track headers + corner cells** (headers stay visible while scrolling long timelines), **Ctrl/Cmd+wheel timeline zoom** (native non-passive listener), **auto-follow playhead** during playback, a **frame-accurate timecode readout** in the timeline toolbar (`M:SS.FF / M:SS.FF`), **group-aware pointer drag** (dragging one selected/grouped clip shifts the rest of the selection by the same delta, locked tracks excluded), **drag-to-resize track height** (strip under the track name, 32–160px, single commit), and **effect reorder buttons** (apply earlier/later) in the inspector.

### Deferred (explicitly)
- 3D transforms / parallax (G20) — after Phases 1–8.
- Remotion/WebCodecs adapters — interface exists; no implementation planned.
- Detach-audio, karaoke word emphasis, AI auto-captions — need groundwork that doesn't exist yet (audio extraction, transcription provider).
- Preview fidelity note: video-asset embedded audio plays in preview but is **not mixed at export** (the renderer only mixes audio/music-track clips) — surface or fix when audio handling gets its next pass.

---

## Standing constraints

- Neutral timeline JSON is the only source of truth; renderer-specific data never persists in it.
- Additive migrations; new columns need defaults; no ORM; keep router→handler→service→repository layering; wire routes in `router.go`.
- Responses via `respondJSON`/`respondError`; bodies via `decodeJSON`; params via `chi.URLParam`.
- Local-first: editing/render never requires cloud; provider calls only on explicit AI actions; encrypted provider secrets untouched; ownership checks intact.
- Preserve existing visual language, the Video Studio ↔ Video Edit Studio split, Wails desktop builds, and all currently working features listed above.
- Validation before claiming completion: `cd frontend && npm run lint && npm run build`; `cd backend && go test ./...`; `npm run test:smoke` from root where UI behavior changed. Do not claim tests passed without running them.

## Session log

- **2026-06-10** — Audited draft against code; wrote corrections #1–#15; restructured into living plan; fixed stale `docs/VIDEO_RENDERING.md`. Started Phase 1.
- **2026-06-10 (cont.)** — Completed Phase 1 (backend schema + validation + upgrader + tests, TS types, schema doc). Landed the Phase 2 core: all new store actions, track add/remove/rename/reorder/height UI, ruler markers, nudge/zoom/marker shortcuts, inspector layer-order + effect/transition/keyframe row management. Repaired the smoke suite, which was already broken by the committed sidebar-label refactor (88feb38): updated `tests/image-editor-toolbar.smoke.spec.ts` and `tests/music-studio.smoke.spec.ts` selectors (`'Image Studio'`→`'Image'`, `'Music Studio'`→`'Music'`, `'Lyria model'`→`'Music model'`, strict-mode `.first()`/`.last()` disambiguation) and added a SQLITE_BUSY retry to the parallel seed helper. Validation: `go test ./...` ✅, `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 2)** — Finished Phase 2: clip context menu, grouping with group-aware selection, align/distribute, blade/select tools, `[`/`]` trim-to-playhead, `?` shortcut-help overlay, drag-over snap guide, sub-second ruler ticks, and the inspector Canvas section (presets + width/height/FPS/background via `setCanvas`, commit-on-blur). Frontend-only changes. Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 3)** — Phase 3 core landed: preview now composites all visible tracks (track order + `z_index`), renders transforms/opacity/fades/crop/text styling, fixes mute-vs-hide semantics to match export, syncs multiple clip videos, and supports on-canvas select/drag/scale/rotate with single-commit undo plus grid/safe-area guides. Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 4)** — Phase 3 finished: on-canvas crop mode (edge handles, dim overlay, clamped fractions, single-commit, reset button), snap-to-center during canvas drags with centerline guides (respects the global snap toggle), and a 4-element cap on mounted preview videos with placeholder cards for deeper layers. Frontend-only. Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 5)** — Closed the G15 upload gap: the Edit Studio media bin header now has an Upload button (multi-file, `video/*,image/*,audio/*`) backed by a new `uploadAsset` store action calling the existing `POST /v1/video/projects/{id}/assets/upload` (server-side MIME sniffing and per-kind size limits apply); uploaded assets prepend to the bin and become the selection. Before this, media reached the Edit Studio bin only via generations, Video Studio picker uploads, or cross-studio import. Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 6)** — Phase 4 (captions) landed: parser/serializers + presets in `captionUtils.ts`, five caption store actions, and `VideoCaptionPanel` in the right sidebar. **Separately, found and fixed a latent database bug while chasing a smoke flake:** `db.Open` passed mattn-style DSN params (`_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&...`) to the **modernc.org/sqlite** driver, which only understands `_pragma=name(value)` — every tuning pragma was silently ignored, so the app has been running without WAL or a busy timeout (the source of `SQLITE_BUSY` failures when the Playwright seeder and server contend). Converted the DSN to `_pragma=` form (busy_timeout, WAL, synchronous, cache_size, mmap_size, temp_store, journal_size_limit). **`foreign_keys` was deliberately left OFF**: the schema declares ~48 FK constraints that have never been enforced under this driver; enabling them needs a dedicated audit of delete paths and orphaned rows (logged as Phase 6 work). Validation: `go test ./...` ✅, `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ ×2 consecutive runs (3/3 each, no SQLITE_BUSY messages).
- **2026-06-10 (cont. 7)** — Phase 5 landed: typed registries (`effectRegistry`, `transitionRegistry`, `keyframeUtils`), preview CSS-filter effects + keyframe animation, and the registry-driven inspector editors (effect param sliders, transition duration/direction, keyframe time/value/easing, per-type "preview only" chips). **Keyframe `time_ms` semantics decided: clip-relative**; inspector add-at-playhead writes relative times and `docs/VIDEO_TIMELINE_SCHEMA.md` documents it (pre-existing keyframes created by the old inspector carried absolute times — they were never rendered anywhere, so no migration is needed; users re-add them via the new picker). Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 8)** — Phase 6 core landed in `renderer.go`: rotation (transparent-fill `rotate`), sharpen/vignette filters, position-keyframe `overlay` expressions (clip-relative, linearized easing), and drawtext font-family/stroke-width/line-spacing. `renderer_capabilities.go` updated (rotation supported, keyframes partial, richer effects/text notes — inspector warnings update automatically); frontend registry export flags flipped; new fidelity test covers all additions; `docs/VIDEO_RENDERING.md` refreshed. Remaining Phase 6: scale/rotation/opacity/volume keyframes at export, `xfade`, chroma-key, track solo, FK audit. Validation: `go test ./...` ✅, `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (3/3).
- **2026-06-10 (cont. 9)** — Phase 8 smoke test landed: `tests/video-editor.smoke.spec.ts` (full editor flow incl. caption segment) added to the `test:smoke` scripts; suite now runs 4 specs. Validation: `npm run test:smoke` ✅ (4/4 on first run).
- **2026-06-10 (cont. 10, polish sweep)** — Cleared every tracked deferred-polish item and most of Phase 7: ffprobe upload enrichment (G14, `probe.go` + handler wiring + parse tests), media-bin search/sort + drag-and-drop upload (G15 fully closed), six starter templates with header picker (G16), group-aware pointer drag, drag-to-resize track height, effect reorder UI. New commercial-UX finds shipped: sticky track headers, Ctrl+wheel zoom, auto-follow playhead, frame-accurate toolbar timecode. Phase 8 doc refresh done for `docs/VIDEO_STUDIO.md` (three stale sections corrected). Validation: `go test ./...` ✅, `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (4/4). **Remaining:** Phase 6 remainder (scale/rotation/opacity/volume keyframes at export, `xfade`, chroma-key, track solo, FK audit), Phase 7 leftovers (poster thumbnails, probe-on-import, editor modes G17), and the explicit deferrals (3D transforms, AI captions, preview audio-track playback, save-as-template).
- **2026-06-10 (cont. 11)** — **Fixed a real playback bug**: the preview only played clips whose `<video>` was mounted when play started — any clip mounting mid-playback (i.e. every clip after the first) sat frozen at frame 0 because the play/seek effect ran only on `isPlaying` toggles. Replaced the two media-sync effects with one per-tick sync: paused-but-mounted elements seek to their trim offset and play, drifting elements re-seek (>0.35s tolerance), and pause/scrub behavior is unchanged. The same mechanism now drives **preview audio playback** (G-deferred → done): audio/music-track clips at the playhead mount hidden `<audio>` elements with per-tick volume = clip volume × fade factor (element volume caps at 1; >1 boosts only at export); muted tracks stay silent, matching export. **Editor modes (G17) landed** — see Phase 7. Also hardened the music smoke spec against parallel-worker noise (resource-load console errors filtered; JS errors still fail). Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ ×2 (4/4 each).
- **2026-06-10 (cont. 12, bug hunt)** — Deliberate bug hunt across both studios; 11 real bugs found and fixed:
  - **Renderer (export correctness):** muted tracks wrongly dropped their text/caption overlays at export (mute must only silence audio); text clips ignored `transform.scale` at export (now multiplies fontsize, matching preview); text fades/opacity were preview-only (now export via a drawtext `alpha` expression). New test `TestDrawTextHonorsMuteScaleAndFades`.
  - **Service (lifecycle):** Cancel Render couldn't actually stop FFmpeg — jobs ran on an uncancellable `context.Background()` (now a per-job cancel registry kills the process; a cancelled job no longer gets overwritten to "failed"); cancelled Gemini generations kept polling the API for up to 20 minutes and then clobbered "cancelled" with "failed" (poll loop now checks DB status each iteration); the non-Gemini `GenerateAsync` fallback called `Generate` with an **empty userID** and left the original async generation row pending forever (now passes the real user and retires the superseded row); corrupt render-settings JSON was silently ignored (now fails the job with a clear error).
  - **Store (races):** switching projects didn't stop the generation poll interval or render poll timeout — they kept injecting the old project's data into the new one (cleared on `selectProject`, render polls also self-guard by comparing `job.project_id`); out-of-order `saveTimeline` responses could clobber newer local edits (save sequence guard — only the latest save's response applies); splitting a clip didn't rebase clip-relative keyframes for the right half and copied both fades to both halves (keyframes now partition + rebase, fade-in stays left / fade-out stays right).
  - **UI:** inspector sliders committed per input event, flooding undo history (50 entries in one drag) and firing a save per pixel (now draft-while-dragging, commit on release — transform, volume, fade, and effect-param sliders); three pointer-drag commit paths ran store actions inside setState updaters, which double-commit under React StrictMode in dev (preview move/scale/rotate, crop drag, track-height drag — now commit from refs).
  - Validation: `go test ./...` ✅, `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (4/4). Noted, not fixed (by design / needs schema): `SaveTimeline` last-write-wins without optimistic locking; video-asset embedded audio audible in preview but never mixed at export.
- **2026-06-10 (cont. 13)** — Music Studio → media bin handoff: completed tracks gained an "Add to Video Project" button (next to "Make Video", which only carries prompt context). It calls a new `importMusicAsset` store action — imports into the active video project via the existing `assets/import` endpoint (`source_studio: "music"`), auto-creates a project when none is active, prepends the asset to the media bin + selects it, and the success toast offers "Open editor". This closes the long-standing baseline caveat that only Image Studio had a wired cross-studio handoff. Validation: `npm run lint` ✅ (0 errors), `npm run build` ✅, `npm run test:smoke` ✅ (4/4).
