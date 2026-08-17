# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Date:** 2026-08-17  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and the final decoded export must represent the same immutable timeline revision with the same frame, layer, timing, geometry, styling, effect, transition, camera, and audio decisions.

## Implementation tracker

**Started:** 2026-08-17  
**Current milestone:** Phase 0 — reproducible parity baseline
**Current status:** Phase 1 is complete. Phase 0 now has automated project seeding, a complete 103-frame known-mismatch baseline, independent browser-preview and snapshot PCM with gated EBU R128, decoded-delivery frame/PTS validation, and a functioning hosted CI artifact job. Draft PR #187 passes hosted backend tests and the Linux race detector. Quality Gate attempt 2 produced the first full hosted parity artifact with passing audio and delivery gates; inspection exposed a one-pixel preview sizing error and repeated diagnostic-name overwrites. Both harness fixes are implemented and pass a real one-frame local smoke, pending hosted rerun. Unsupported-audio-boundary coverage, visual review, and final threshold approval remain.

| Phase | Status | Progress note |
|---|---|---|
| Phase 0 — Parity baseline | In progress | Fixture generation, real-project seeding, 103 named visual captures, passing independent PCM timing/correlation/EBU R128 gates, decoded-delivery frame/PTS checks, mismatch reports, and hosted CI artifacts are implemented; exact preview dimensions and unique per-frame diagnostics are fixed locally pending hosted rerun; unsupported-audio-boundary coverage, visual review, and threshold approval remain |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, durable staged inputs, decode preflight, snapshot-only execution/recovery, identity metadata, legacy labeling, path-specific capability diagnostics, Strict Parity, and HTTP/frontend concurrency coverage are implemented |
| Phase 2 — Canonical contract | Not started | — |
| Phase 3 — Shared preview | Not started | — |
| Phase 4 — Shared worker | Not started | — |
| Phase 5 — Visual parity | Not started | — |
| Phase 6 — Audio parity | Not started | — |
| Phase 7 — Rollout and retirement | Not started | — |

### Implementation log

- **2026-08-17:** Implementation started with the first recommended correctness slice. The active work is binding each render job to an immutable timeline and asset snapshot so queued output cannot change after submission.
- **2026-08-17:** Added database migration V52 with timeline revisions/content hashes, immutable render snapshots, per-snapshot asset leases, and one-to-one job snapshot bindings.
- **2026-08-17:** Render submission now requires the saved timeline ID/revision/hash at the HTTP boundary, rejects stale submissions with HTTP 409, verifies all referenced files, hashes a stable asset manifest, and atomically creates the snapshot and queued job.
- **2026-08-17:** Render execution now reads timeline JSON, settings, and asset records only from the immutable snapshot. It re-verifies timeline/manifest/source hashes and fails closed if bytes are missing or changed.
- **2026-08-17:** Referenced source bytes are copied into a contained, snapshot-owned staging directory before enqueue. The manifest points only to staged bytes, so changing or deleting the original asset cannot change queued output; staging is removed with the render job.
- **2026-08-17:** Completed-job and output-asset metadata now record snapshot, timeline, asset-manifest, contract, renderer, and renderer-version identity. Interrupted snapshot-bound jobs resume from the same persisted snapshot and staged inputs after restart.
- **2026-08-17:** The frontend now saves before enqueue, submits the exact returned revision/hash, and derives its “changed since render” state from the newest completed job's immutable timeline hash, including overlapping jobs.
- **2026-08-17:** Added required FFprobe validation for referenced image/video/audio sources before enqueue. Undecodable media blocks submission with exact clip/asset context, and verified stream metadata is frozen into the asset manifest.
- **2026-08-17:** Historical snapshot-less jobs are labeled `legacy_mutable_source`, report that exact source material is unavailable, fail closed on execution/recovery, and use explicit “render current timeline” retry copy.
- **2026-08-17:** Corrected stale wipe, zoom, cursor, and annotation capability messages. Export preflight now reports exact timeline paths for authored limitations, and Strict Parity promotes those limitations to blocking errors in both the client and backend.
- **2026-08-17:** Phase 1 verification passed for focused Go suites (`internal/db`, `internal/repository`, `internal/video`, `internal/api`), the frontend production build, and all 86 frontend unit tests. The full Go suite passed except for the pre-existing Windows symlink-privilege test `internal/gitrepo/TestCloneQuotaFilesystemCountsSymlinkTargetBytesAndEntry`.
- **2026-08-17:** Phase 0 implementation started with `parity-torture-v1`: a deterministic 20-second, 640×360/30 fps timeline that covers source aspect ratios, all supported playback rates, trims/fades/gain, overlap and track state, every easing/curve, spatial transforms/camera motion, international text/font fallback, all registered shapes/effects/transitions, cursor behavior, captions/range metadata, and audio/channel metadata.
- **2026-08-17:** Added deterministic named sampling for clip boundaries, keyframes, transition midpoints, scene boundaries, markers, and eight frames from seed `20260817`. The fixture emitter can also generate source media with FFmpeg; its default bundle currently contains 103 unique frame indices.
- **2026-08-17:** Added `GET /v1/video/render-snapshots/{snapshotId}/frames/{frameIndex}`. It authorizes through the snapshot project, re-verifies immutable timeline/manifest/source hashes, preserves snapshot range/resolution/FPS/caption settings, and emits a single lossless PNG directly from FFmpeg without a delivery-codec round trip.
- **2026-08-17:** Added canonical-preview seek/readiness hooks and Playwright capture support, plus a capture script that pairs program-monitor screenshots with immutable-snapshot PNGs by the generated sample names.
- **2026-08-17:** Added parity metrics and artifacts: per-frame channel tolerance/pass rate, MAE, RMSE, max delta, global SSIM, changed bounds, zero-tolerance named regions, audio offset/correlation/peak/approximate-LUFS metrics, JSON and Markdown summaries, side-by-side images, and heat maps. Thresholds remain provisional until the first full baseline is reviewed.
- **2026-08-17:** Added `GET /v1/video/render-snapshots/{snapshotId}/audio.pcm`, which preserves the submitted range and mix contract, verifies immutable inputs, and emits headerless 48 kHz stereo signed-16 PCM. The capture script retains this snapshot mix beside the named frame captures; the independent browser reference added later provides the genuine two-sided comparison.
- **2026-08-17:** Phase 0 focused Go tests, immutable-snapshot frame/PCM integration, command compilation, the fixture emitter plus all four generated FFmpeg media inputs, script syntax checks, and the frontend production build pass. Full `internal/video` and `internal/api` suites still encounter pre-existing Windows sandbox `EvalSymlinks` access-denied failures in unrelated path-containment tests; the focused video/API suites pass when using workspace-local Go caches.
- **2026-08-17:** Extended `video-parity-capture.mjs` into a test-only seeder: it creates a real project, uploads all generated inputs, remaps fixture asset IDs, saves the timeline, submits an immutable snapshot, cancels the delivery job while retaining snapshot inputs, opens the editor, and captures named preview/render pairs without manual actions.
- **2026-08-17:** Kept snapshot path verification fail-closed on Windows while handling sandbox-denied ancestor traversal: the fallback performs lexical containment plus an `Lstat` walk of every storage-root descendant and rejects symlinks/reparse points. The previously failing manifest mutation test and live seeded submission now pass.
- **2026-08-17:** Reduced diagnostic cost by selecting only clips active at the requested half-open timestamp after fidelity expansion and by building snapshot PCM from an audio-only graph. Diagnostic PCM now rebuilds timestamps from sample numbers and pads/trims to the exact timeline sample count; the 20-second live fixture returns exactly 3,840,000 bytes at 48 kHz stereo s16le.
- **2026-08-17:** Captured the first complete immutable-snapshot baseline under `output/video-parity-baseline`: all 103 named frame pairs are 640×360, but 0/103 pass the provisional visual gate. Mean pixel pass rate is 0.182464, median is 0, and mean SSIM is 0.094076. This is retained as intentional known-mismatch evidence and confirms that the separate preview/FFmpeg composition engines—not output dimensions—are the primary parity failure.
- **2026-08-17:** Added `--allow-fail` baseline mode to the report command and PowerShell wrapper, made missing diagnostic audio fail capture, and added a `video-parity-baseline` CI job. The job pins repository Go/Node versions, installs Chromium, generates media, seeds a real project, captures all samples, records FFmpeg/FFprobe/Playwright/font metadata, and uploads reports, frame pairs, PCM, seed identity, and service logs for 14 days. Its first hosted run is still pending.
- **2026-08-17:** Added an independent browser/Web Audio preview reference renderer. It evaluates persisted track solo/mute/visibility, clip mute, trims, rates, gain keyframes, fades, and channel conversion into exact-duration 48 kHz stereo signed-16 PCM while deliberately excluding monitor volume. The capture script now stores that reference and its declared limitations beside the immutable snapshot PCM.
- **2026-08-17:** Replaced the fixture's periodic sine control with a deterministic swept chirp plus one-second impulses so timing offsets cannot alias to a different cycle. The live 20-second reference and snapshot mixes both contain exactly 960,000 stereo frames (1,920,000 interleaved scalar samples; 3,840,000 bytes), align at zero samples, and correlate at 0.988807; the 0.194214 peak and approximate 2.496236 dB loudness differences expose a real gain/envelope mismatch that remains to be fixed.
- **2026-08-17:** Made long PCM comparisons practical by using a sampled coarse offset search followed by exact local refinement, and made unequal sample counts an explicit audio-gate failure.
- **2026-08-17:** Fixed legacy delivery failures exposed by the full torture fixture. Visual fidelity expansion no longer fragments audio-only clips or resets their gain/fade envelopes; visual segments retain one continuous soundtrack. Repeated inputs now use short per-render aliases, and the FFmpeg filter graph is passed by script file, avoiding the Windows command-line length limit.
- **2026-08-17:** Added decoded-delivery validation through FFprobe and the report wrapper. The live delivery contains exactly 600/600 decoded frames at constant 30 fps, starts at PTS 0, and is exactly 20.0 seconds with time base `1/15360`, so the delivery timing gate passes independently of the known visual/audio content failures.
- **2026-08-17:** Extended capture and CI to submit, poll, download, and report a separate immutable delivery render. Focused `internal/video` tests, parity command compilation, the preview-audio unit tests, script syntax checks, the frontend production build, and a local end-to-end report-wrapper run pass. Hosted CI execution remains pending.
- **2026-08-17:** Diagnosed and fixed the live PCM mismatch. FFmpeg's implicit mono-to-stereo pan law attenuated mono assets while Web Audio duplicated them at unity, and the legacy volume expression treated eased keyframes as linear. The mix graph now uses frozen/probed channel metadata for explicit unity-gain mono duplication and mirrors the preview's built-in later-keyframe easing. The regenerated fixture PCM keeps equal 1,920,000-interleaved-sample streams and zero offset, improves correlation from 0.988807 to 0.999973, reduces peak difference from 0.194214 to 0.000793, and reduces approximate loudness difference from 2.496236 dB to 0.001760 dB; the provisional PCM correlation gate now passes.
- **2026-08-17:** Replaced approximate loudness as a gate with FFmpeg's EBU R128 scanner and advanced the parity-report schema to v2. The provisional audio gate now also requires sample-peak difference ≤0.001, integrated-loudness difference ≤0.1 LU, and true-peak difference ≤0.1 dB. The live reference/export pair measures -21.1/-21.1 LUFS and -3.1/-3.0 dBFS true peak, so all audio gates pass; the RMS-style estimate remains diagnostic only.
- **2026-08-17:** The completed parity slice is recorded at commit `d1f2a6e` on `feat--Video-Motion-Design-Roadmap-Implementation`, matching its `origin/feat--Video-Motion-Design-Roadmap-Implementation` tracking ref. GitHub CLI authentication has been restored; draft-PR creation and the first hosted `video-parity-baseline` inspection are in progress.
- **2026-08-17:** Opened draft PR #187 and ran hosted Quality Gate workflow `32063980092`. Frontend lint/unit/build, Helm, macOS/Windows sandbox and desktop checks, and hosted `internal/video` tests passed. `Backend tests and race detector` failed in `TestAgentRuntimeProductionRouterWiring` because a SQLite temp directory was modified during Linux cleanup, so the dependent `video-parity-baseline` job was skipped before producing artifacts.
- **2026-08-17:** Traced the hosted cleanup race to unowned video startup-recovery goroutines launched by `NewRouterWithShutdown`. Startup scans now complete before router return; asynchronous generation, recovered-generation, and render workers are owned by a service context and wait group; `video.Service.Shutdown` closes admission, cancels work, shuts down the renderer, and waits before database cleanup. Added `TestServiceShutdownWaitsForActiveRenderWorkers`. The full `internal/video` package and 30 repeated router integration runs pass locally; the unrelated Windows sandbox `EvalSymlinks` permission test still prevents a full local API pass, and local `-race` execution is unavailable because CGO is disabled. Hosted Linux race verification is pending.
- **2026-08-17:** Pushed lifecycle commit `da17653` to PR #187. Quality Gate workflow `32071010787` passed backend formatting, vet, unit/integration tests, and the hosted Linux race detector, confirming the cleanup race is fixed and releasing the dependent parity job. The parity job then failed before capture because Ubuntu 24.04 did not provide `ffmpeg` or `ffprobe` in `PATH`; Playwright's private FFmpeg download is not a system toolchain and does not supply FFprobe. The workflow now explicitly installs the Ubuntu FFmpeg package and verifies both executables before fixture generation; rerun and full artifact inspection are pending.
- **2026-08-17:** Pushed FFmpeg dependency commit `2167bf6`. Quality Gate workflow `32071734277` attempt 2 passed the backend/race prerequisites and the complete `video-parity-baseline` job after attempt 1's hosted package-install runner stalled and was recycled. Artifact `9302801760` is 45,964,170 bytes with digest `sha256:2bf20e91…794b0`; it contains the real seeded project/snapshots, 103 preview/render pairs, PCM, delivery MP4, schema-v2 JSON/Markdown, diagnostics, service logs, and toolchain metadata (Go 1.25.13, Node 24.19.0, npm 11.17.0, FFmpeg/FFprobe 6.1.1-3ubuntu5, Playwright 1.62.1, DejaVu Sans fallback).
- **2026-08-17:** Hosted audio and delivery evidence passes: both PCM files are 3,840,000 bytes (960,000 stereo frames), align at zero interleaved samples with 0.999993 correlation and zero sample-peak difference, measure -21.1/-21.1 LUFS and -3.1/-3.0 dBFS true peak, and the delivery decodes to 600/600 CFR frames at 30 fps, PTS 0, 20.0 seconds, and time base `1/15360`. Visual thresholds intentionally remain failing, but artifact inspection also found that hosted preview screenshots were 639×360 versus 640×360 renders and repeated report labels overwrote diagnostics, leaving 140 rather than 206 report PNGs.
- **2026-08-17:** Fixed both hosted evidence-integrity defects. The capture runner now constrains the preview fit container to the exact fixture canvas, waits for exact CSS geometry, takes a locator screenshot at device scale 1, and rejects any PNG whose IHDR dimensions differ. Report artifacts now prefix the frame index, preserving repeated semantic labels. A new duplicate-label regression test passes, and a real backend/frontend one-frame smoke emits matching 640×360 PNGs, two uniquely indexed diagnostics, and equal 3,840,000-byte PCM streams. Full hosted rerun is pending.

## Executive recommendation

Do not continue trying to keep the current DOM/CSS preview and Go/FFmpeg composition graph in sync feature by feature. They are two separate renderers with different clocks, geometry models, text engines, effect implementations, transition behavior, and media decoding behavior. Every new editing feature currently needs to be implemented twice, and the existing fidelity expansion layer converts continuous animation into bounded static segments. That architecture can improve, but it cannot provide a durable 1:1 guarantee.

The recommended fix is to introduce one canonical, deterministic TypeScript render core and one shared React composition that are used by both:

1. the editor's authoritative **Fidelity Preview**, through a player component; and
2. final export, through a pinned headless-Chromium render worker, with FFmpeg retained for media normalization, audio processing/muxing, and final encoding.

A Remotion-based player/worker is the recommended implementation because it lets the editor and export execute the same frame-driven React composition. The Go backend should continue to own authorization, validation, persistence, immutable render snapshots, scheduling, admission limits, cancellation, asset storage, diagnostics, and output asset creation. The existing `video.Renderer` interface remains the adapter boundary.

The first production fix was immutable render snapshots. At plan creation, a queued job stored a timeline ID and `runRenderJob` reloaded the mutable timeline later. Phase 1 now binds every new render to an immutable revision and durable staged source bytes.

## What “1:1” means

“1:1” must be an explicit product and test contract rather than an informal visual expectation.

### Included in the parity contract

For a render snapshot, output resolution, and integer frame index, preview and export must agree on:

- the exact timeline revision and exact asset revisions;
- output frame count and frame timestamps;
- which clips and scenes are active, using half-open ranges `[start, end)`;
- source frame/time selection after trim and playback-rate changes;
- track visibility, persisted audio mute/solo, clip mute, and `audio_only` behavior;
- stable layer ordering by track index, `z_index`, and clip array order;
- canvas background and project aspect ratio;
- crop, fit mode, anchor, position, scale, rotation, opacity, and camera projection;
- keyframe interpolation, easing, Bezier curves, and spring curves;
- text shaping, line breaks, alignment, font file, weight, size, spacing, stroke, shadow, background, padding, and corner radius;
- every authorable shape and annotation primitive;
- effect order, parameters, animated effect amounts, and scene-effect scope;
- transition type, edge, duration, direction, and overlap semantics;
- cursor location, glyph, size, highlight, click-ring timing, and smoothing;
- audio source timing, channel layout, clip gain, gain keyframes, fades, mix order, and master processing; and
- range-export rebasing and caption burn-in behavior.

### Explicitly excluded

The following editor-only elements are not part of the rendered program:

- selection outlines and resize/rotation/crop handles;
- smart guides, grids, safe-area guides, hover states, and context menus;
- decoder-budget badges and poster placeholders;
- the editor's monitor/master volume control; rename this control to **Monitor Gain** so it cannot be mistaken for persisted program gain; and
- lossy codec artifacts. MP4/WebM encoding can alter decoded pixels slightly even when composition is correct.

### Verification levels

Use two output levels in tests:

1. **Lossless diagnostic parity:** Render PNG frames and PCM/WAV audio from the canonical worker. Geometry, timing, alpha, and layer selection must be exact. Proposed visual threshold: at least 99.9% of pixels within two 8-bit channel values, with no structural-region mismatch. Tune and freeze the final tolerance during Phase 0.
2. **Delivery-codec parity:** Decode the MP4/WebM output and compare it with the lossless reference. Proposed threshold: SSIM at least 0.995 for high-quality profiles, exact frame count, no frame offset, and no content-region displacement. Codec noise is allowed; semantic or geometric drift is not.

For audio, use decoded 48 kHz PCM. The lossless reference and export must have the same sample count, an offset no larger than one sample, correlation of at least 0.999 for unaffected segments, and matching peak/LUFS tolerances for processed mixes.

## Current-state findings

This table is the baseline captured when the plan was written. The Phase 1 tracker and implementation log above supersede the render-revision, missing-source, and dirty-tracking rows where noted; the preview/export composition mismatches remain open.

The current implementation has good validation, job scheduling, asset storage, and real FFmpeg coverage. The problem is that preview and export do not execute the same composition rules.

| Area | Current preview | Current export | Parity consequence |
|---|---|---|---|
| Render revision (baseline; fixed in Phase 1 core) | Uses live Zustand state | A queued job reloaded the timeline row when the worker ran | Edits after Render could change queued output |
| Clock | `requestAnimationFrame` and floating-point milliseconds | Output FPS plus FFmpeg timestamps | Boundaries and source-frame choice can differ |
| Active interval | Half-open clip/scene checks | FFmpeg `between(t,start,end)` is end-inclusive | A clip/effect can survive one extra output frame |
| Easing | Quadratic `ease-in-out` | Smoothstep `t*t*(3-2*t)` | The same keyframes follow different motion paths |
| Animation sampling | Continuous at the editor playhead | Midpoint-sampled static segments, capped at 300 per clip and 60 fps | Long clips and high-FPS exports visibly step or drift |
| Transitions | The preview does not apply clip transitions | Fade/dip/slide are applied; wipe/zoom are sampled; crossfade is an alpha fade | Export can show motion or fades the user never saw |
| Effects | CSS subset, mostly static amounts | Different FFmpeg filters; several export-only or approximated effects | Color, blur, motion, keying, and animated amounts differ |
| Crop | CSS `clip-path` masks a canvas-sized element | Source is cropped and then rescaled | Cropped subject size and placement change |
| Transforms | CSS 3D, anchor, X/Y tilt, perspective | 2D transform plus sampled 2.5D projection; anchor and X/Y tilt are not fully honored | Rotation center, perspective, and parallax differ |
| Media fit | Canvas-sized wrapper with `object-contain` | `scale=...:force_original_aspect_ratio=decrease` after crop | Letterboxing and cropped bounds can differ |
| Text | Browser font/layout engine and CSS box model | FFmpeg `drawtext`, fontconfig fallback, spacing/alignment approximation | Fonts, wrapping, box size, rotation, spacing, and corners differ |
| Shapes | CSS/SVG primitives | Several kinds normalize to rectangles or text glyphs | Ellipse, arrow, speech bubble, spotlight, rounded shapes, and marks differ |
| Shape animation/fades | Wrapper transforms and fades apply | `drawbox` handles a smaller static subset | Shape rotation, non-uniform scale, effects, and fades can be lost |
| Cursor | SVG cursor in clip-local canvas coordinates; 300 ms click window | Sampled `➤` text in center-offset coordinates; 160 ms click window | Cursor position, glyph, scale, and ring timing differ |
| Audio gain | HTML media volume, capped at 1.0 | FFmpeg gain allows up to 2.0 | Over-unity clips cannot be auditioned accurately |
| Audio finishing | Not previewed | FFmpeg filters run on each input before `amix` | The user cannot hear final processing; master processing is in the wrong graph position |
| Output dimensions | Preview uses the timeline canvas | Custom export can change width/height without a canonical viewport transform | Position, text size, and shape size do not scale consistently |
| Missing source file | Asset row may still make the preview/checklist look valid | `resolveMediaClips` silently skips a missing file | Export can complete with a missing layer |
| Capability data | Frontend registries contain independent flags and copy | Backend exposes another capability matrix | Wipe/zoom/cursor warnings are already stale relative to the fidelity renderer |
| Mixed clip content | Preview chooses media, else shape, else text | Export may composite media and then also draw shape/text from the same clip | An allowed document can mean different primitives on each side |

### Relevant current files

- `frontend/src/components/video/VideoPreviewCanvas.tsx`: DOM/CSS preview, media clock, direct manipulation, text, cursor, and shape hosting.
- `frontend/src/components/video/ShapePreview.tsx`: preview-only CSS/SVG shape implementation.
- `frontend/src/components/video/effects/keyframeUtils.ts`: frontend curve evaluation.
- `frontend/src/components/video/effects/effectRegistry.ts`: frontend effect previews plus duplicated export flags.
- `frontend/src/components/video/effects/transitionRegistry.ts`: transition metadata with export flags that no longer match the backend.
- `frontend/src/components/video/exportValidation.ts`: client preflight with stale cursor/wipe/zoom warnings.
- `backend/internal/video/renderer.go`: FFmpeg visual graph, text/shape drawing, audio mix, and encode.
- `backend/internal/video/renderer_fidelity.go`: sampled animation, camera, wipe/zoom, annotation normalization, and cursor expansion.
- `backend/internal/video/renderer_capabilities.go`: backend capability matrix.
- `backend/internal/video/service.go`: render enqueue and execution; Phase 1 now binds new jobs to immutable render snapshots.
- `backend/internal/repository/video_render_job_repo.go`: render-job persistence.
- `backend/internal/video/probe.go`: best-effort media metadata that does not yet capture display rotation, sample aspect ratio, time base, or color metadata.

## Target architecture

```text
Saved timeline + immutable asset revisions
                  │
                  ▼
       Versioned timeline adapter
                  │
                  ▼
    Canonical render contract/core
  frame index, source time, curves,
  ordering, matrices, transitions,
  effects, text layout, audio graph
          │                   │
          │                   └──────────────┐
          ▼                                  ▼
 Shared React composition             Canonical AudioGraph
          │                                  │
    ┌─────┴──────────┐                 ┌─────┴─────────┐
    ▼                ▼                 ▼               ▼
Editor Player   Headless worker   Web Audio/preview  FFmpeg/PCM
    │                │                 │               │
UI overlays     image/frame stream     └──────┬────────┘
outside output       └──────────────┬─────────┘
                                    ▼
                         FFmpeg mux + delivery encode
                                    │
                                    ▼
                         immutable export asset
```

### Ownership boundaries

**Go backend remains responsible for:**

- project/timeline/asset authorization;
- timeline validation and schema upgrade entry points;
- render preflight and immutable snapshot creation;
- durable job persistence, priority, concurrency, cancellation, restart recovery, stalls, and disk preflight;
- safe staging paths and asset leases;
- worker process lifecycle and progress ingestion;
- FFmpeg media normalization, audio graph execution where applicable, muxing, codec profiles, and output probing;
- output asset persistence and caption sidecars; and
- capability/health reporting based on the actually installed worker, Chromium, fonts, and FFmpeg.

**The new shared renderer package becomes responsible for:**

- exact timeline-to-frame and timeline-to-audio evaluation;
- the stable layer-order and time-range rules;
- source-time calculations using integer/rational arithmetic;
- transform matrix order, anchor, crop, fit, and camera projection;
- transition evaluation and effect parameter evaluation;
- deterministic text layout and shape geometry;
- the React composition used by player and worker; and
- feature-specific preflight diagnostics.

### Proposed repository layout

```text
schemas/
  video-timeline-v2.schema.json
  video-render-manifest-v1.schema.json

video-renderer/
  package.json
  src/contract/
  src/core/
    normalizeTimeline.ts
    evaluateFrame.ts
    evaluateAudio.ts
    timebase.ts
    curves.ts
    transforms.ts
    ordering.ts
    diagnostics.ts
  src/composition/
    VideoComposition.tsx
    MediaLayer.tsx
    TextLayer.tsx
    ShapeLayer.tsx
    EffectStack.tsx
    TransitionLayer.tsx
    CursorLayer.tsx
  src/player/
  src/worker/
  assets/fonts/
  test/fixtures/

backend/internal/video/
  render_snapshot.go
  shared_renderer.go
  render_manifest.go
  ffmpeg_encoder.go
  ffmpeg_audio_graph.go
```

The exact names may change, but the render core must be a real shared package. Copying its functions into the frontend and worker would recreate the current problem.

## Canonical render rules

These decisions must be recorded in a short architecture decision record and enforced by fixtures before implementation spreads across components.

### Time and frame boundaries

- The output is evaluated by integer `frameIndex`, never by wall-clock time.
- `frameTime = frameIndex / outputFps` is represented as a rational value for comparisons and source-time calculation.
- Clip and scene activity is half-open: `startFrame <= frameIndex < endFrame`.
- Timeline milliseconds are converted to frame boundaries with one documented rounding policy. Recommended: start uses floor, end uses ceil, then range membership remains half-open.
- Source time is computed from trim and playback rate using rational arithmetic and clamped to the immutable probed source duration.
- Preview playback may drop displayed frames for responsiveness, but scrubbing and final export must evaluate the same requested frame state.

### Ordering

- Visual order is `(track array index, z_index, clip array index)`.
- Start time never acts as an implicit z-order tiebreaker.
- Scene effects execute at their defined scope after the complete scene composite.
- A clip must declare its visual primitive model. If media, shape, and text may coexist, the contract must specify their internal order. Otherwise validation must reject mixed primitives. Do not let preview and export choose different branches.

### Geometry

- Canvas coordinates have one origin convention. Retain center-relative clip `x/y` for compatibility, but cursor events remain top-left canvas coordinates and are converted exactly once.
- Define matrix order explicitly: anchor translation, crop/fit, local scale, local rotation, position, camera/view projection, then canvas/output transform.
- `anchor_x/y` are canvas-pixel offsets from the content center in both preview and export.
- `crop` must have one meaning. For existing v1 projects, preserve the current visible preview as the compatibility target. New v2 documents should distinguish source crop from a rectangular mask if both behaviors are needed.
- A delivery resolution with the same aspect ratio scales the composed canvas uniformly. A different aspect ratio requires a saved reframe/variant timeline and must not silently relayout the project during export.
- Handle source rotation metadata and non-square pixels during ingest normalization, not independently in preview and export.

### Text and fonts

- Ship a small approved font set with explicit licenses and stable file hashes.
- Resolve family and weight to a specific font asset during preflight; never rely on OS font fallback for a strict render.
- Store or deterministically derive text bounds, wrap width, padding, line height, alignment, and vertical alignment.
- Use the same text shaping and line-breaking implementation in player and worker. If browser layout is retained, pin Chromium and bundle fonts; if cross-platform metrics still exceed the parity threshold, move shaping to a shared HarfBuzz/WASM path and render glyph outlines.
- Missing fonts are a blocking preflight error in strict mode, with an explicit user choice to substitute and save the substitution into the timeline.

### Effects and transitions

- The ordered effect list is executable truth; registries are UI metadata derived from that truth.
- Effect parameters, including animated amounts, are evaluated by the canonical frame evaluator.
- Use the same CSS/SVG/WebGL implementation in player and worker. Do not label a different FFmpeg approximation as exact.
- A transition must state whether it applies to the in edge, out edge, or a two-clip edit point. Existing v1 transitions receive a documented compatibility mapping.
- Crossfade must use both adjacent clip inputs. An alpha fade over the canvas background is not a crossfade.
- Transition duration is clamped once by normalization and is then identical everywhere.

### Media and color

- Extend probe metadata to include display rotation, display dimensions, sample aspect ratio, stream time base, duration source, color primaries, transfer, matrix, range, and audio start time.
- Build a normalized proxy/mezzanine at ingest for formats whose browser and FFmpeg decode paths disagree. Preview and final high-quality source must share display geometry and color transforms.
- Adopt one working color contract for the first release: sRGB/BT.709 SDR, full documentation of range conversion, and explicit tone mapping for HDR sources.
- Make HDR preservation a separate future contract. Silent browser tone mapping is incompatible with 1:1 output.

### Audio

- Use 48 kHz as the internal program sample rate.
- Compile both preview and export from one `AudioGraph` containing source windows, rates, gains, fades, channel maps, delays, mix buses, and master processing.
- Apply denoise/EQ/compression/normalization/limiting at the intended bus. The current implementation applies the timeline processing chain to each input before `amix`; program mastering should occur after mix unless the UI explicitly exposes per-clip processing.
- Use Web Audio `GainNode` or a worklet so preview supports the same 0–2 gain range as export.
- Full-program operations such as loudness normalization may require analysis. Cache a processed preview stem keyed by the render snapshot hash and play that same stem in Fidelity Preview and export. Show a clear “processed audio preview is stale” state after relevant edits.
- Monitor Gain remains outside the persisted graph.

## Phased implementation

No phase should be declared complete solely because code exists. Each phase has an exit gate that must be met in CI and in an end-to-end project fixture.

## Phase 0 — Define and measure the parity baseline

**Objective:** Turn WYSIWYG into a measurable contract and capture the current divergence before changing engines.

### Current implementation status

| Work item | Status | Evidence / remaining work |
|---|---|---|
| Deterministic torture timeline | Implemented | `ParityTortureFixture` plus the checked-in manifest and fixture-emitter command cover the Phase 0 authoring matrix with stable IDs and values |
| Deterministic source media and project seed | Implemented | `video-parity-fixture --generate-media` creates landscape, portrait, square, and mono-audio inputs; `video-parity-capture.mjs --media-dir` creates the project, uploads/remaps assets, saves the timeline, and submits/cancels an immutable render snapshot |
| Named frame selection | Implemented | Boundary/keyframe/transition/scene/marker samples plus seed `20260817`; duplicate frame indices merge their reasons |
| Immutable-snapshot diagnostic PNG | Implemented | Authenticated frame endpoint emits one direct PNG while preserving snapshot render settings and verifying all hashes |
| Canonical preview capture hook | Implemented | Program monitor exposes a stable locator and frame seek/readiness events; Playwright helper and capture script consume them |
| Visual metrics and artifacts | Implemented | Tolerance pass rate, MAE/RMSE/max delta, SSIM, changed bounds, structural regions, side-by-side images, and heat maps |
| Snapshot diagnostic audio | Implemented | Authenticated endpoint emits the immutable submitted mix as exact-duration 48 kHz stereo signed-16 PCM and records format headers; the 20-second live fixture is 3,840,000 bytes and snapshots submitted without audio reject the request |
| Independent preview-reference audio | Implemented | Browser/Web Audio evaluates persisted audition state, trims, rates, gain keyframes, fades, and channels into exact-duration 48 kHz stereo signed-16 PCM; it records unsupported pitch-preservation/master-processing cases and excludes monitor gain |
| Audio metrics | Implemented; provisional gate passing | Report schema v2 gates exact sample count, offset, correlation, sample peak, EBU R128 integrated loudness, and true peak; approximate loudness remains diagnostic only. Hosted evidence has equal 1,920,000-interleaved-sample streams, zero offset, 0.999993 correlation, zero sample-peak difference, -21.1/-21.1 LUFS, and -3.1/-3.0 dBFS true peak. Final tolerance approval and unsupported-audio-boundary coverage remain |
| Decoded delivery timing | Implemented | FFprobe counts decoded frames and validates CFR, start PTS, duration, and time base. The live 20-second delivery passes with 600/600 frames, 30 fps, start PTS 0, duration 20.0 seconds, and time base `1/15360` |
| Machine/human report | Implemented | `video-parity-report` writes versioned JSON and Markdown, exits non-zero on gated threshold failure, and supports explicit `--allow-fail` for known-mismatch baseline retention |
| End-to-end baseline evidence | Implemented | Real project/snapshot `1b38f197-38ba-4f48-9bd4-b8091fa99dd9` produced all 103 visual pairs: 0 passing, 103 dimension matches, 0.182464 mean pixel pass rate, and 0.094076 mean SSIM. Project `0f305bf2-6cdd-43e6-851f-0b6feac2d71b` with timeline hash `6aed0422…8388` produced the independent audio reference and passing delivery-timing evidence above |
| CI artifact job | Implemented; evidence-integrity rerun pending | Quality Gate `32071734277` attempt 2 produced artifact `9302801760` with successful real-project capture, audio/delivery gates, toolchain recording, and upload. Exact preview sizing and unique per-frame diagnostic names are fixed locally after artifact review and await hosted confirmation |
| Final threshold approval | Pending | Current 2-channel-value/99.9%/0.995/0.999/one-sample values are provisional until baseline review |

### Work

1. Create a deterministic “parity torture timeline” fixture containing:
   - landscape, portrait, and square media;
   - image and video clips with different source aspect ratios;
   - trim, 0.25x/0.5x/1x/2x/4x rate, fades, volume above 1, and audio-only video;
   - overlapping tracks, equal `z_index` values, hidden/muted/solo tracks, and exact clip boundaries;
   - every easing and curve type;
   - crop, anchors, non-uniform scale, Z rotation, X/Y tilt, depth, and scene camera motion;
   - all text properties, bundled and missing fonts, multiline text, emoji, and non-Latin text;
   - every shape, annotation, effect, transition, and cursor option;
   - caption burn-in and range export; and
   - audio processing and channel conversion.
2. Add a diagnostic frame endpoint/tool that can request named frame indices from a render snapshot without performing a full delivery encode.
3. Capture current preview screenshots and decoded FFmpeg frames at clip starts, ends, keyframes, transition midpoints, scene boundaries, and random seeded frames.
4. Add a parity-report script that produces:
   - side-by-side images;
   - heat-map diffs;
   - pixel metrics and bounding-box displacement;
   - frame-count/timestamp differences;
   - audio offset, correlation, peak, and LUFS differences; and
   - a machine-readable JSON summary.
5. Freeze the final thresholds and identify a small set of zero-tolerance structural regions, such as text bounds, cursor center, and crop rectangle.

### Likely files

- `backend/internal/video/renderer_golden_test.go`
- new `backend/internal/video/renderer_parity_test.go`
- new `video-renderer/test/fixtures/`
- new `scripts/video-render-parity.*`
- Playwright fixture support under `tests/`

### Exit gate

- CI can produce a reproducible parity report from a fixed fixture.
- The report proves at least the known mismatches in the current-state table.
- Thresholds and frame-boundary rules are approved and documented.

## Phase 1 — Make render submission immutable and fail closed

**Objective:** Guarantee that the job renders exactly the revision the user submitted, even before the new visual engine is ready.

### Current implementation status

| Work item | Status | Evidence / remaining work |
|---|---|---|
| Timeline revision and canonical content hash | Implemented | V52 fields; repository create/save computes the hash and monotonically increments revision |
| Required ID/revision/hash at render API | Implemented | Missing identity returns 400; stale identity maps to 409 |
| Atomic snapshot and job creation | Implemented | Snapshot, asset lease rows, and job are committed in one repository transaction |
| Asset resolution, containment, and hashing | Implemented | Every referenced asset must belong to the project, exist, be regular, resolve inside storage after symlink evaluation, and match its SHA-256/size at execution |
| Durable source-byte retention | Implemented | Each referenced source is copied into a contained snapshot-owned staging directory; logical/original deletion and mutation no longer affect the job, and staging is deleted with the job |
| Snapshot-only worker execution | Implemented | The worker no longer reloads mutable timeline JSON, settings, or asset rows |
| Identity metadata | Implemented | Job and output asset metadata record snapshot, timeline, manifest, contract, renderer, and renderer-version identity |
| Missing/corrupt input behavior | Implemented | Missing files, containment violations, undecodable media, and post-enqueue staged-byte changes block/fail with exact clip and asset context |
| Hash-based dirty-render UI | Implemented | Uses the newest completed job in creation order, avoiding overlapping-job completion races |
| Capability messages and Strict parity mode | Implemented | Runtime capability data drives path-specific diagnostics; stale wipe/zoom/cursor/annotation claims are corrected; Strict Parity blocks known mismatches in client and backend |
| Historical snapshot-less jobs | Implemented | Reads remain compatible, jobs are labeled `legacy_mutable_source`, exact source is reported unavailable, and execution/recovery fails closed |

### Verification status

| Scenario | Status |
|---|---|
| Mutate timeline after enqueue; renderer still receives submitted snapshot | Passing automated Go test |
| Submit stale revision; no job is created | Passing service and direct HTTP 409 tests |
| Missing, undecodable, deleted, or changed source bytes | Passing automated Go tests |
| Active asset lease lifecycle | Passing repository test |
| Two overlapping revisions and dirty-render selection | Passing frontend unit test |
| Restart with queued snapshots | Passing automated Go test |
| Strict Parity rejects path-specific known mismatches without creating a job | Passing backend and frontend tests |

### Work

1. Add monotonic `revision` and canonical `content_sha256` fields to `video_timelines`.
2. Require `POST /v1/video/projects/{projectId}/render` to include the expected timeline ID, revision, and content hash returned by the last successful save.
3. In one backend transaction:
   - verify ownership and expected revision;
   - validate/normalize the timeline;
   - resolve every referenced asset;
   - verify every file exists and remains inside the storage root;
   - create a canonical asset manifest with content hash, byte length, media metadata, and stable staging reference;
   - store the normalized timeline JSON, export settings, renderer version, schema/contract version, asset manifest, and hashes as an immutable render snapshot; and
   - create the queued render job pointing to that snapshot.
4. Return HTTP 409 when the expected timeline revision is stale. The client should save again and ask the user to resubmit, not silently render a newer revision.
5. Add durable asset leases or a render-snapshot staging directory. Logical asset deletion may proceed, but physical bytes referenced by queued/running jobs must survive until the job is terminal and its retention period expires.
6. Change `runRenderJob` to load only the snapshot. Never reload current timeline JSON or re-resolve mutable asset rows for composition.
7. Persist snapshot/timeline/asset/renderer hashes into job metadata and the output asset metadata.
8. Fail preflight for missing/corrupt media instead of silently dropping a layer.
9. Update dirty-render tracking to compare the completed job's `timeline_sha256` with the current saved timeline hash. Remove the single global `_renderStartedSaveSeq`, which is incorrect when multiple jobs overlap.
10. Align current capability messages while the migration runs:
    - remove independent hard-coded frontend support booleans where possible;
    - correct stale cursor and wipe/zoom warnings;
    - report exact timeline paths for partial/unsupported features; and
    - add a **Strict parity** preflight mode that blocks known mismatches instead of offering “Render anyway.”

### Persistence proposal

Implemented in migration V52 as a `video_render_snapshots` table rather than growing `video_render_jobs` with large JSON blobs:

```text
id
project_id
timeline_id
timeline_revision
timeline_json
timeline_sha256
asset_manifest_json
asset_manifest_sha256
settings_json
render_contract_version
renderer
renderer_version
created_at
```

The job keeps its current lifecycle columns and a `snapshot_id` foreign key. Migration must preserve existing job reads; historical jobs without a snapshot are explicitly labeled `legacy_mutable_source` and are not retryable as exact renders.

### Likely files

- `backend/internal/db/db.go`
- `backend/internal/models/models.go`
- `backend/internal/repository/video_timeline_repo.go`
- `backend/internal/repository/video_render_job_repo.go`
- new `backend/internal/repository/video_render_snapshot_repo.go`
- `backend/internal/video/service.go`
- new `backend/internal/video/render_snapshot.go`
- `backend/internal/api/video_handler.go`
- `frontend/src/types/video.ts`
- `frontend/src/api.ts`
- `frontend/src/stores/videoStudio.ts`
- `frontend/src/components/video/VideoRenderPanel.tsx`
- `frontend/src/components/video/exportValidation.ts`

### Tests

- Queue a job, mutate the timeline before the worker starts, and prove the output/job snapshot hash does not change.
- Queue a job, logically delete a referenced asset, and prove the leased/staged bytes still render.
- Submit a stale revision and assert HTTP 409 with no job created.
- Delete/corrupt source bytes before enqueue and assert a blocking preflight error naming the clip and asset.
- Run two jobs from different revisions concurrently and verify dirty-render UI per job.
- Restart the service with queued jobs and verify the same snapshots are recovered.

### Exit gate

- Every new render job is bound to immutable timeline and asset hashes.
- A queued render cannot be affected by later edits or deletes.
- Missing media cannot produce a successful but incomplete export.

## Phase 2 — Introduce the canonical timeline/render contract

**Objective:** Centralize all non-I/O render semantics in a pure, deterministic package.

### Work

1. Add a versioned JSON Schema for timeline v2 and render-manifest v1.
2. Generate or mechanically verify frontend and Go types from the schema. Add a CI test that fails when Go, TypeScript, and JSON contract fields drift.
3. Implement a v1-to-canonical adapter that preserves the current editor preview as the compatibility target. Do not reinterpret existing projects to match current FFmpeg approximations.
4. Add explicit v2 fields where v1 is ambiguous:
   - media fit and mask/source-crop semantics;
   - deterministic content bounds;
   - transition edge/edit-point semantics;
   - text wrap box, padding, and vertical alignment;
   - working color space; and
   - primitive composition behavior for clips that contain media plus text/shape data.
5. Implement pure evaluators:
   - `normalizeTimeline(document, assets, settings)`;
   - `frameCount(duration, fps)` and rational time conversion;
   - `activeClips(frameIndex)` and stable order;
   - `sourceTime(clip, frameIndex)`;
   - `evaluateCurve` and `evaluateProperty`;
   - `evaluateTransformMatrix` including anchor/camera;
   - `evaluateTransition`;
   - `evaluateEffectStack` including animated amounts;
   - `evaluateTextLayout` and shape geometry;
   - `evaluateCursor`; and
   - `compileAudioGraph`.
6. Return a serializable `FrameState` containing already-resolved render decisions. Components should draw it, not repeat timeline math.
7. Replace millisecond comparisons in render paths with the canonical frame/timebase helpers.
8. Define structured diagnostics with severity, stable code, timeline JSON path, clip/track ID, and suggested remediation.
9. Treat unknown fields according to schema-version policy. Unknown authorable effects/transitions must fail closed, not disappear.

### Required contract fixtures

- before/at/after clip and scene boundaries;
- ties in track/z/clip order;
- every easing/curve at fixed sample points;
- long clips and 120 fps output without segment caps;
- trim/rate/range-export combinations;
- crop/fit/anchor matrix fixtures;
- camera and depth fixtures;
- transition in/out/edit-point fixtures;
- text-layout and font-resolution fixtures;
- effect amount keyframes; and
- cursor coordinate and click-window fixtures.

### Immediate confirmed correction

The frontend and Go implementations currently disagree on `ease-in-out`. The canonical contract should use one formula and fixture values. Prefer the current editor's piecewise quadratic curve for v1 compatibility unless product design intentionally chooses a new v2 curve.

### Exit gate

- Preview and export callers can consume the same `FrameState`/`AudioGraph` fixtures.
- No renderer-specific code evaluates curves, active ranges, ordering, transforms, or source time independently.
- Cross-language contract drift is a CI failure.

## Phase 3 — Replace the authoritative preview with the shared composition

**Objective:** Make what the user sees in Fidelity Preview the exact composition that the export worker will execute.

### Work

1. Build `VideoComposition` from canonical `FrameState` objects.
2. Integrate it through a frame-driven player in the editor. Remotion Player is the recommended host; a custom host is acceptable only if it executes the same composition and exact frame clock.
3. Split `VideoPreviewCanvas.tsx` into:
   - a composition host that contains only renderable program output;
   - an editor overlay for selection, handles, crop UI, guides, grids, safe areas, and context menus; and
   - playback controls/monitor audio outside the composition.
4. Port media, text, shape, cursor, effect, transition, and scene rendering into the shared composition. Delete preview-only implementations only after equivalent fixtures pass.
5. Evaluate effect keyframes, fades, and transitions from the canonical frame state. Do not read static clip objects directly in the drawing component.
6. Bundle deterministic fonts and preload them before marking a frame ready.
7. Add two explicit preview modes:
   - **Fidelity Preview:** authoritative, same composition and source/proxy normalization as export;
   - **Performance Preview:** may use posters, reduced effect quality, or decoder budgets, but always displays a persistent “Draft preview” badge and is never used for parity approval.
8. Remove the current proxy badge from inside renderable layer content; editor performance indicators belong in the UI overlay so they cannot leak into an export component.
9. Preserve direct manipulation by converting pointer changes into canonical transform patches. During drag, the composition receives an ephemeral timeline patch; pointer-up commits one store mutation as it does today.
10. Rename preview master volume to Monitor Gain and expose a separate persisted Program Gain only if product requirements need it.

### Preview-specific acceptance cases

- Pausing on any integer frame produces the same `FrameState` as the worker.
- Starting, ending, or splitting a clip does not show a duplicate boundary frame.
- Transition midpoint, last transition frame, and first post-transition frame match fixtures.
- Resizing/rotating around a nonzero anchor does not jump on commit.
- Text bounds do not change after fonts finish loading.
- Fidelity Preview never substitutes a poster for an active video layer.
- UI overlays are absent from composition screenshots.

### Likely files

- major refactor of `frontend/src/components/video/VideoPreviewCanvas.tsx`
- retirement/refactor of `frontend/src/components/video/ShapePreview.tsx`
- `frontend/src/components/video/effects/*`
- new shared composition package under `video-renderer/`
- `frontend/vite.config.ts`
- root/frontend package-workspace configuration

### Exit gate

- The editor's Fidelity Preview is running the shared composition.
- All existing direct-manipulation and playback smoke tests pass.
- Phase 0 diagnostic screenshots are generated from the canonical player.

## Phase 4 — Add the headless shared-render worker behind a feature flag

**Objective:** Render final visual frames with the exact same composition used by Fidelity Preview while preserving the durable Go job system.

### Work

1. Implement a local worker CLI/service with a narrow protocol:

```text
video-render-worker render
  --manifest <contained absolute path>
  --output <contained absolute path>
  --progress <pipe/socket descriptor>
```

2. The manifest contains only normalized timeline data, immutable staged asset paths, font manifest, output dimensions/FPS, snapshot hashes, and renderer version. It must not contain secrets or arbitrary remote URLs.
3. Pin the worker's Chromium build, disable network access for the render page, use a strict CSP, and use software rendering in CI for determinism. Production may use approved acceleration after parity tests prove it stays within tolerance.
4. Render exact integer frames from `0` through `frameCount-1`. Wait for media decode, fonts, and composition readiness before capturing a frame.
5. Pipe frames directly to FFmpeg where practical instead of storing every PNG. Keep an optional lossless diagnostic-frame path for tests and support.
6. Add `SharedCompositionRenderer` implementing the existing Go `Renderer` interface.
7. Keep `ScheduledRenderer`, admission limits, disk checks, cancellation, recovery, and job progress. Replace only the visual composition delegate.
8. Split `renderer.go` responsibilities:
   - legacy visual compositor retained temporarily as `legacy_ffmpeg_compositor.go`;
   - codec/mux logic moved to `ffmpeg_encoder.go`;
   - audio graph logic moved to `ffmpeg_audio_graph.go`; and
   - worker orchestration placed in `shared_renderer.go`.
9. Propagate cancellation to the entire worker/Chromium/FFmpeg process tree, then wait for cleanup using existing scheduler shutdown rules.
10. Record worker version, Chromium version, font-manifest hash, FFmpeg version, snapshot hashes, frame count, and timing diagnostics in job metadata.
11. Add runtime health/capability checks. The capability endpoint must reflect the active renderer selected by configuration, not always return `FFmpegRendererCapabilities()`.
12. Package the worker for all supported deployments:
   - backend Docker image includes the worker, pinned Chromium, fonts, and FFmpeg;
   - Wails builds bundle or install a signed worker runtime and resolve it relative to the application, with no assumption that system Node exists;
   - web build scripts copy the renderer bundle;
   - Helm adds renderer cache/temp volumes, resource requests/limits, probes, and configuration; and
   - offline installations do not download Chromium at first render.

### Feature flags

```text
OMNILLM_VIDEO_RENDER_ENGINE=legacy_ffmpeg|shared_chromium
OMNILLM_VIDEO_RENDER_SHADOW_COMPARE=false|true
```

`shared_chromium` stays opt-in until Phases 5 and 6 pass. Shadow comparison creates diagnostic metrics only and must not double the user's persisted outputs or unexpectedly double long-running resource use.

### Security requirements

- Resolve and validate all staging/output paths in Go before process launch.
- Pass discrete process arguments; never compose a shell command.
- Run the worker without network, secrets, or broad filesystem access.
- Limit worker memory, CPU, render duration, frame dimensions, and frame count.
- Treat timeline strings, SVG/text, and asset metadata as untrusted input.
- Do not expose local paths or full worker/FFmpeg commands to ordinary API clients.

### Exit gate

- A render snapshot completes through the shared worker using the current scheduler.
- Cancel, restart recovery, stall detection, and output asset creation work unchanged.
- Lossless worker frames pass the Phase 0 parity thresholds against Fidelity Preview for the supported feature subset.

## Phase 5 — Close all visual feature gaps and introduce timeline v2

**Objective:** Reach complete visual parity for every feature that the editor allows in strict mode.

### Work by feature family

#### Media, crop, and transforms

- Implement explicit `contain`, `cover`, `fill`, and `none` fit modes.
- Preserve current v1 preview crop behavior through the adapter; expose source crop and mask separately in v2.
- Implement anchors and one documented transform matrix order.
- Render non-uniform scale, rotation, 2.5D depth, X/Y tilt, perspective, camera position/rotation/FOV, and parallax in the shared composition.
- Scale same-aspect delivery resolutions without changing layout.
- Require a saved reframe timeline for aspect-ratio changes.

#### Text and captions

- Resolve fonts to bundled/file-backed assets.
- Add explicit text bounds/wrap behavior and remove dependence on implicit browser shrink-to-content sizing.
- Match weight, line height, letter spacing, alignment, stroke, shadow, background, padding, and rounded corners.
- Ensure text participates in all clip transforms, effects, fades, transitions, and scene camera behavior.
- Use the same text component for captions, callouts, and ordinary text, with style presets only supplying data.

#### Shapes and annotations

- Render rectangle, rounded rectangle, ellipse, highlight, blur, pixelate, spotlight, arrow, line, checkmark, X mark, step marker, speech bubble, and label as actual shared primitives.
- Remove glyph/rectangle normalization from the new renderer.
- Apply position, anchor, uniform/non-uniform scale, rotation, opacity, keyframes, fades, transitions, and effects consistently.
- Define whether blur/pixelate samples the composition below the shape before or after scene effects, and lock that order in fixtures.

#### Effects

- Implement one canonical version of brightness, contrast, saturation, grayscale, blur, sharpen, vignette, shadow, background blur, chroma key, film grain, bloom, color grade, edge fade, RGB split, ghost trail, motion blur, depth of field, and rack focus.
- Preserve effect list order.
- Evaluate effect-amount keyframes in preview and export.
- Replace approximate capability labels only after golden coverage passes.
- If an effect cannot meet parity, hide/disable it in strict authoring mode instead of silently substituting another look.

#### Transitions

- Add explicit in/out/edit-point semantics to v2.
- Implement true two-input crossfade.
- Implement fade, dip to black, slide, wipe, and zoom in the shared composition.
- Define behavior for missing/overlapping neighbors, media gaps, and clips on different tracks.
- Remove transition-specific alpha-fade and sampled-crop compatibility code from the new path.

#### Cursor

- Use one cursor SVG/path asset and the same canvas-coordinate conversion.
- Use the same smoothing, sample interpolation, click detection window, scale, highlight, ring geometry, colors, and lifetime.
- Keep click audio absent unless it becomes a persisted timeline feature; do not synthesize it only at export.

### Timeline migration

1. Add `UpgradeTimelineDocument` support from v1 to v2.
2. Preserve an untouched v1 document until the user saves an edit that needs v2 semantics, or perform an explicit migration with a preview diff.
3. Store migration metadata with the previous hash and migration version.
4. Provide a rollback/export of the original v1 JSON.
5. Add golden fixtures proving migrated v1 projects match their current editor preview, not the legacy FFmpeg approximation.

### Capability policy after this phase

Capability data becomes granular and generated from the active renderer manifest:

```json
{
  "feature": "transition.crossfade",
  "support": "exact",
  "contract_version": 2,
  "renderer_version": "..."
}
```

Allowed values are `exact`, `draft_preview_only`, `blocked`, and `legacy_approximation`. “Supported plus partial” is too ambiguous for a strict parity promise.

### Exit gate

- Every visual feature enabled in strict authoring mode is marked `exact` and has a parity fixture.
- The Phase 0 visual timeline passes the lossless and delivery-codec thresholds.
- No strict render relies on `renderer_fidelity.go` segment expansion or FFmpeg visual approximations.

## Phase 6 — Unify audio preview and export

**Objective:** Make the heard program match the exported program, including source timing and final processing.

### Work

1. Compile one `AudioGraph` from the immutable snapshot.
2. Correct source window/rate/delay calculations with rational timebase helpers.
3. Preview clip gain up to 2.0 through Web Audio rather than the HTML element's 1.0 volume limit.
4. Apply clip gain keyframes and fade/transition envelopes from the canonical evaluator.
5. Keep persisted track solo as program state and Monitor Gain as editor-only state.
6. Move program denoise/EQ/compression/normalization/limiting to the post-mix master bus unless a separate per-clip effect is authored.
7. Add a two-pass normalization path where required and persist measured loudness values in job metadata.
8. Generate/cache a processed preview stem tied to `audio_graph_sha256` for operations that cannot be reproduced interactively without full-program analysis.
9. Invalidate the cached stem when any audible source, timing, gain, solo/mute, fade, rate, or processing setting changes.
10. Use the same cached PCM/stem for Fidelity Preview and final mux when exact mastering preview is selected.
11. Add explicit channel layout and resampling rules, including mono-to-stereo behavior.
12. Probe output audio for duration, start time, sample rate, channel layout, peaks, and silence.

### Tests

- impulse at known clip boundaries to catch sample offsets;
- overlapping tones to validate mix gain and ordering;
- rate changes with pitch preservation;
- volume keyframes and fades at exact sample positions;
- muted, soloed, audio-only, and video-soundtrack cases;
- over-unity gain without preview clipping;
- mono/stereo conversion;
- post-mix compression/limiting/normalization; and
- range exports with audio beginning at sample zero.

### Exit gate

- The Phase 0 audio fixture passes the PCM timing, correlation, peak, and loudness thresholds.
- Fidelity Preview clearly indicates whether it is playing the live graph or the exact cached processed stem.
- Program mastering is no longer applied independently to every input.

## Phase 7 — Roll out, observe, and retire the legacy compositor

**Objective:** Make the shared renderer the safe default and remove duplicated semantics.

### Rollout stages

1. **Developer-only:** shared renderer behind environment flag; all parity reports retained as CI artifacts.
2. **Internal opt-in:** selected projects render with both engines for short diagnostic ranges; only the selected engine produces the user asset.
3. **New-project default:** timeline v2 projects use the shared renderer; v1 remains explicit legacy mode.
4. **General default:** shared renderer for all projects after migration preview/golden results pass.
5. **Legacy removal:** remove the old visual graph and fidelity expander after at least one stable release window with no parity rollback.

### Observability

Persist non-content diagnostics per job:

- snapshot/timeline/asset/font/renderer hashes;
- render contract and timeline versions;
- engine and Chromium/FFmpeg versions;
- frame count, output duration, FPS, dimensions, and color contract;
- per-stage duration and peak memory;
- dropped/retried frame count;
- audio graph hash and loudness measurements;
- preflight diagnostic codes; and
- parity summary for diagnostic/shadow renders.

Do not collect frame contents or user media as telemetry.

### Removal work

- Delete visual composition paths from `renderer.go` after encoding/audio functions are split out.
- Delete `renderer_fidelity.go` and its segment cap.
- Remove frontend `exportSupported` copies and stale capability copy.
- Remove obsolete warning text from `exportValidation.ts`, `VideoInspector.tsx`, and docs.
- Update `docs/VIDEO_RENDERING.md`, `docs/VIDEO_STUDIO_ARCHITECTURE.md`, `docs/VIDEO_TIMELINE_SCHEMA.md`, README capability claims, and deployment docs.

### Exit gate

- Shared renderer is the default across web, Docker, Helm, and Wails builds.
- Rollback to legacy remains available for one release but is not offered as an “exact” render.
- No authorable strict-mode feature lacks a shared composition implementation and golden fixture.

## Test strategy and CI gates

### Unit and contract tests

- JSON schema validation and v1-to-v2 migration.
- Rational time/frame/source-time math.
- Half-open ranges and exact output frame count.
- Stable ordering including equal `z_index`.
- Curves at fixed sample values.
- Transform matrices and anchor/crop/fit order.
- Text layout and font resolution.
- Shape geometry.
- Effect and transition parameter evaluation.
- Audio graph compilation.
- Diagnostic codes and timeline JSON paths.

### Golden visual tests

- Player frame versus lossless worker frame.
- Worker lossless frame versus decoded delivery frame.
- Boundary frame triplets around every start/end/keyframe/transition/scene boundary.
- Alpha-edge tests on rotation, rounded corners, chroma key, shadow, and overlays.
- Cross-platform smoke tests with one canonical Linux golden environment; Windows/macOS results use the same structural tests and bounded raster tolerance.

### Golden audio tests

- PCM sample count and offset.
- Correlation, peak, RMS, and LUFS.
- Spectral checks for rate/pitch and EQ fixtures.
- Silence detection where audio is intentionally absent.

### End-to-end tests

1. Create/edit the torture timeline through UI/API fixtures.
2. Save and capture returned revision/hash.
3. Render a snapshot.
4. Continue editing before the queued job begins.
5. Verify the output metadata retains the submitted hash.
6. Compare named preview frames with decoded export frames.
7. Verify output audio and captions.
8. Restart/cancel/retry jobs and confirm snapshot identity.

### Performance gates

Establish final budgets from Phase 0 hardware baselines. Initial targets:

- Fidelity Preview reaches first ready frame within 750 ms for a cached 1080p project.
- Playback sustains the project FPS for the standard fixture without source-time drift; UI may coalesce state updates independently.
- A 1080p30 standard fixture stays within a documented renderer memory ceiling and does not grow with timeline duration once the active working set is stable.
- Cancellation terminates worker descendants and releases leases/staging data promptly.
- Long projects do not expand into per-frame timeline JSON or unbounded in-memory clip copies.

## API and data-contract changes

### Timeline save response

Add:

```json
{
  "revision": 42,
  "content_sha256": "...",
  "render_contract_version": 2
}
```

### Render request

Add required optimistic binding:

```json
{
  "timeline_id": "...",
  "timeline_revision": 42,
  "timeline_sha256": "...",
  "render_contract_version": 2,
  "settings": {}
}
```

### Render job response

Add:

```json
{
  "snapshot_id": "...",
  "timeline_revision": 42,
  "timeline_sha256": "...",
  "asset_manifest_sha256": "...",
  "renderer": "shared_chromium",
  "renderer_version": "...",
  "render_contract_version": 2
}
```

### Preflight endpoint

Add a server-authoritative preflight that uses the same normalization and worker capability manifest as rendering:

```text
POST /v1/video/projects/{projectId}/render/preflight
```

It returns normalized output geometry, estimated frame count/disk needs, resolved assets/fonts, and structured diagnostics. Client validation remains useful for immediate UX but is not authoritative.

## Workstream dependency order

| Workstream | Depends on | Can proceed in parallel with |
|---|---|---|
| Baseline/parity harness | Existing system | Snapshot schema design |
| Immutable snapshots | Existing save/render APIs | Render-core spike |
| Timeline v2 + render manifest | Phase 0 decisions | Snapshot implementation |
| Canonical render core | Timeline contract | Worker packaging spike |
| Shared preview composition | Render core | Go worker adapter |
| Headless export worker | Composition + snapshots | AudioGraph implementation |
| Visual feature closure | Shared preview/worker | Audio parity |
| Audio parity | Timebase + snapshots | Visual feature closure |
| Default rollout | All parity gates | Documentation and cleanup |

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Chromium/worker increases image and desktop bundle size | Pin one renderer runtime, share browser cache where safe, publish size budgets, and package it at build time rather than downloading on first render |
| Headless render is slower than pure FFmpeg for simple cuts | Keep FFmpeg fast paths only when they are proven output-equivalent by the same manifest, add frame piping/cache, and benchmark before enabling any optimization |
| Browser/WebView text rasterization differs | Bundle fonts, make line layout deterministic, pin export Chromium, compare structure separately from antialiasing, and use shared glyph shaping/outlines if needed |
| GPU effects vary by driver | Use deterministic software rendering for golden/strict modes; enable hardware acceleration only after device-qualified parity checks |
| Large frame streams consume disk/memory | Pipe frames, bound queues, stream progress, use contained job staging, and retain frames only for diagnostic jobs |
| Asset deletion races queued jobs | Immutable manifest plus leases/hardlinks/copies in durable snapshot staging |
| Timeline v2 changes existing project appearance | Target current editor preview for v1 migration, show migration diff, retain original JSON/hash, and make migration reversible until saved |
| Worker expands attack surface | No network/secrets, contained paths, strict CSP, resource limits, discrete argv, pinned dependencies, and untrusted-input tests |
| Audio mastering cannot be live | Hash and cache exact processed preview stems; clearly distinguish live mix from processed Fidelity Preview |
| Team continues adding features to both old paths | Freeze legacy compositor features after Phase 1; all new authorable visuals must land in the canonical contract/composition first |

## Definition of done

The project is complete only when all of the following are true:

- Clicking Render creates an immutable snapshot and later edits cannot affect it.
- The completed job and output asset identify the exact timeline, assets, fonts, settings, contract, and renderer versions used.
- Fidelity Preview and final export execute the same frame evaluator and shared visual composition.
- Every enabled strict-mode feature has an `exact` capability and a passing parity fixture.
- Preview and export use the same half-open frame boundaries, source-time math, ordering, curves, transforms, text layout, shapes, effects, transitions, cursor rules, and camera rules.
- Fidelity Preview can audition the exact processed audio or clearly reports that its cached processed stem is stale.
- Missing media/fonts, unsupported features, stale revisions, or unavailable renderer dependencies block submission with actionable diagnostics.
- Same-aspect resolution changes preserve composition; aspect changes require an explicit saved reframe.
- The Phase 0 torture timeline passes lossless visual, delivery-codec visual, PCM audio, snapshot, restart, cancellation, and cross-platform gates.
- The shared renderer is packaged and health-checked in Docker, Helm, web, and Wails distributions.
- The legacy FFmpeg visual compositor and `renderer_fidelity.go` are retired, leaving FFmpeg responsible for the tasks it performs well: normalization, audio processing, muxing, and delivery encoding.

## Recommended first implementation slice

The first mergeable slice should be deliberately narrow and should not wait for the new renderer:

1. add timeline revision/content hash;
2. persist immutable render snapshots and asset manifests;
3. bind jobs and dirty-render UI to snapshot hashes;
4. fail preflight on missing files and stale revisions;
5. correct the capability/validation contradictions for cursor, wipe, and zoom;
6. add the parity torture fixture and report harness; and
7. freeze new work in the legacy visual compositor except for critical correctness fixes.

That slice immediately fixes “the render was not the timeline I submitted,” creates reliable evidence for later work, and gives the shared-renderer migration a safe rollback boundary.

## Next recommended implementation slice

1. Push the exact preview-dimension and unique diagnostic-artifact fixes to PR #187, rerun the hosted Quality Gate, and confirm 103 dimension-matched frame pairs, 206 uniquely indexed report PNGs, passing audio/delivery gates, and complete toolchain metadata.
2. Review the 103-frame changed bounds and heat maps by feature category, then freeze Phase 0 visual thresholds and zero-tolerance structural regions without treating the current known-mismatch values as acceptable.
3. Define and test the unsupported audio boundary for playback-rate pitch preservation, custom volume curves, and full-program processing until all three are supplied by a shared `AudioGraph` or immutable processed stem.
4. Re-run the full fixture on at least one second OS/FFmpeg build before approving the provisional 0.001 sample-peak, 0.1 LU integrated-loudness, and 0.1 dB true-peak tolerances.
5. Begin Phase 2 by defining Timeline v2, `AudioGraph`, and the canonical frame/audio evaluation contracts that will replace the divergent DOM/Web Audio and FFmpeg evaluators.
