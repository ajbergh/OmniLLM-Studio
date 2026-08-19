# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-19  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for WYSIWYG rendering. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Merged geometry foundation: **PR #209 — Canonicalize media fit and crop geometry** — `6365b3dcc13fac0726e7407735c2a6b5664e0d1a`.  
Merged FrameState integration: **PR #212 — Consume canonical media geometry in FrameState** — `ae29d57e2e7d4e94e298bb155501583f4577e1ed`.  
Current canonical-contract PR: **#218 — Define canonical perspective projection contract**.  
Current branch: `feat/video-wysiwyg-phase2-perspective-projection`.

PR #208 merged as `52683e4d25b22f70e5c6c3b4a8cf3417240be4bc`. It added permanent browser/TypeScript ↔ Go visual FrameState diagnostics to the deterministic Video renderer parity baseline without changing compositor behavior. Focused hosted run `32200543950` passed Go tests/vet, frontend lint/unit/build, 103/103 matching fail-closed saved-timeline diagnostics, and 103/103 matching available transition-free control states. Final-head frontend, dependency audit, and JavaScript/TypeScript CodeQL passed. Go CodeQL/backend runners remained in system-package setup and were recorded as infrastructure-only incomplete checks rather than green.

PR #209 established `media-geometry-v1` in Go and TypeScript with a shared fixture. It defines `contain`, `cover`, `fill`, and `none`; applies `mask_source_crop` in source coordinates before fit; applies `transform.crop` to the output viewport; and refuses to infer source aspect ratio from the canvas when `content_bounds` is absent. Its corrected head `3e11a21a1ea105060a8484de7ee98c2a60c9cc8d` passed formatting, Go vet, backend unit/integration tests, race detector, frontend lint/unit/build, Windows/macOS contract jobs, Security Scan, and relevant sandbox assurance before merge. Preview and FFmpeg composition were unchanged.

PR #212 integrated `media-geometry-v1` into `visual-frame-state-v1`. It removed the canvas-sized source-bounds fallback for asset-backed clips, attached canonical evaluated media geometry when explicit source bounds exist, used painted media bounds for anchor-space geometry, and reported `media_geometry:content_bounds` when source provenance is absent. Final-head backend formatting/vet/tests/race, frontend lint/unit/performance/build, deterministic renderer parity, Security Scan, container build, and platform assurance jobs passed. The repository-wide Playwright job never reached application setup/tests and remained in `Install system dependencies`; that setup-only incompleteness was explicitly documented before merge and was not represented as green. Preview and FFmpeg composition remained unchanged.

PR #218 defines `perspective-projection-v1` and projects it into visual FrameState. It preserves the current preview's 1200-canvas-pixel projection distance when no scene camera is active and derives distance from evaluated FOV when an authored scene camera is active. A positive per-clip `transform.perspective` overrides the inherited projection distance, while zero/omitted clip perspective inherits it. The canonical projection is a homogeneous CSS-style matrix with `w = 1 - z/d` in the existing row-major/column-vector convention. `model_matrix` remains the camera-relative model transform and perspective is represented separately as `perspective_projection`, avoiding a silent reinterpretation of the existing matrix field. Go and TypeScript share a projection fixture and the visual FrameState fixture verifies the preview default, FOV projection, and clip override. Preview and FFmpeg composition are intentionally unchanged in this slice.

## Merged WYSIWYG foundations

- PR #187 — immutable render submission and deterministic parity baseline — `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.
- PR #191 — Timeline v2 / Render Manifest v1 plus canonical frame/source/easing primitives — `aabbb31288277287673cbed8546c9eb3f38588e4`.
- PR #193 — canonical cubic-Bezier and spring evaluation — `3bed9faf8a868b3a125c25cb141769bfcd7861d2`.
- PR #194 — mechanically checked Go/TypeScript schema projections and contract constants — `62e2180b5153be505fac650cd41b3a0e2d951783`.
- PR #195 — Timeline v1 → canonical Timeline v2 compatibility adapter — `b5f76aa6328240a6b516d768756c34f68e6fdedb`.
- PR #196 — canonical frame activity, range mapping, source time, and stable clip ordering — `42dd64cda9feb75a637b622bf33ac1350a4febd9`.
- PR #198 — evaluator-scoped Timeline v2 runtime normalization/defaulting — `67982f4fdd80062c9439c528362f75382e5c3268`.
- PR #199 — canonical preview/index ordering adoption — `19a1a7b635afd33954bc56ed6023845f2c9e3fd1`.
- PR #202 — deterministic frame-addressed preview/source selection — `02a1bbf4ec2b640a57d59fdd67f7906ae03eaa91`.
- PR #204 — canonical backend diagnostic/parity frame callers — `73fa7d78b5018eb19b88abc34790fd19e95a5a98`.
- PR #205 — canonical numeric property/keyframe evaluation — `8a93f9ff90eeda92c944085715856907747584f1`.
- PR #206 — exact-frame visual FrameState foundation — `c37ba2ed8132133cc913531946d462c3b7b38911`.
- PR #208 — canonical cross-runtime FrameState parity diagnostics — `52683e4d25b22f70e5c6c3b4a8cf3417240be4bc`.
- PR #209 — canonical media fit/crop/source-bounds geometry — `6365b3dcc13fac0726e7407735c2a6b5664e0d1a`.
- PR #212 — canonical media geometry consumption in FrameState — `ae29d57e2e7d4e94e298bb155501583f4577e1ed`.

Security unblock during this program:

- PR #201 replaced reachable-vulnerable `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.

## Phase tracker

| Phase | Status | Progress |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Timing, curves, v1 adapter, frame activity/range/source/order, runtime normalization, deterministic frame addressing, backend callers, property/keyframe evaluation, visual FrameState, cross-runtime diagnostics, media fit/source-mask/output-crop geometry, and FrameState media-geometry consumption are merged. PR #218 canonicalizes perspective projection. Transitions/effects, text/shape/cursor state, and AudioGraph remain. |
| Phase 3 — Shared preview composition | Not started | Program monitor consumes canonical FrameState/AudioGraph instead of preview-local semantic math. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close geometry, text, crop/fit, effects, transitions, cursor, camera, color, and deterministic asset-loading parity. |
| Phase 6 — Audio parity closure | Not started | Shared AudioGraph/processed-stem architecture, rate/pitch policy, gain/fades/channel mapping, and decoded-delivery verification. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, and eventual removal of legacy composition semantics. |

## Architectural rules

### Preview remains the Timeline v1 compatibility target

Current editor preview semantics are the Timeline v1 compatibility target unless an intentional behavior correction is introduced through a versioned contract change. The legacy FFmpeg compositor is not semantic authority merely because it already exports media.

### Freeze legacy approximation growth

Do not add approximation-only semantics to the legacy FFmpeg compositor during Phases 2–4 except for correctness/security regressions. New WYSIWYG behavior belongs in the canonical contract and shared composition path.

### Renderer-independent canonical core

Canonical evaluators must be pure, deterministic, serializable, free of media/browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed on unknown authorable semantics.

### Canonical timing and ordering

1. `frameIndex` is integer output-frame identity.
2. Frame time is rational `frameIndex / fps`; deterministic rendering must not round-trip through integer milliseconds.
3. Authored starts map with `floor(ms × fps / 1000)`; authored ends are exclusive and map with `ceil(ms × fps / 1000)`.
4. Stable clip order is `(track array index, z_index, clip array index)`.
5. Source time is derived from output-frame identity, clip start, trim-in, and playback rate in one canonical evaluator.
6. A keyframe segment uses the later keyframe's easing/curve.
7. Composition matrices, anchors, crop/fit, perspective, camera, transitions, effects, text bounds, cursor state, and color-space assumptions must become explicit FrameState semantics rather than renderer inference.

### Media-geometry authority

For asset-backed media, source aspect ratio is semantic input. `content_bounds` (or a future explicitly versioned source-probe projection) must provide that source box. The canonical evaluator must not substitute output-canvas dimensions for missing source dimensions. `mask_source_crop` operates before fit; `transform.crop` is a separate output clip box. This ordering is captured by `media-geometry-v1`, and PR #212 projects the evaluated geometry into visual FrameState rather than leaving fit/crop to renderer inference.

### Perspective authority

Perspective is projection state, not stacking state. `z_index`/track ordering remains authoritative for paint order; spatial `z` affects camera-relative projection only. `perspective-projection-v1` inherits the preview-compatible 1200-canvas-pixel distance when no scene camera is active, uses the evaluated camera FOV distance when an authored scene camera is active, and permits a positive `transform.perspective` to override the inherited distance. Zero or omitted clip perspective inherits projection. FrameState preserves the camera-relative `model_matrix` and serializes perspective separately so preview/export consumers can apply the same projection without inferring CSS or FFmpeg-specific behavior.

## Phase 0 evidence and remaining sign-off

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains intentionally a known-mismatch baseline; it is diagnostic evidence of architectural divergence, not a production visual threshold.

Remaining Phase 0 sign-off:

1. Freeze production visual threshold policy and zero-tolerance structural regions.
2. Define unsupported-audio policy for pitch preservation, custom gain curves, and program processing until Phase 6.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable through Phases 2–7.

## Phase 1 — Immutable submission

**Complete.** A queued render is bound to one immutable timeline revision/hash and immutable source bytes. Snapshot identity, source staging, decode preflight, recovery, stale-request rejection, and Strict Parity diagnostics are production foundations for later parity work.

## Phase 2 — Canonical contract

### Merged capabilities

- Timeline v2 / Render Manifest v1 schemas and mechanically checked Go/TypeScript projections.
- Rational frame identity, half-open activity, source-time mapping, canonical ordering, and deterministic frame-addressed preview/backend callers.
- Shared built-in easing, cubic-Bezier, spring, numeric property, and exact-frame clip/camera sampling.
- Fail-closed Timeline v1 → v2 compatibility adapter and evaluator-scoped Timeline v2 normalization.
- `visual-frame-state-v1` with explicit unresolved feature families.
- Permanent `visual-frame-state-diagnostic-v1` browser/TypeScript ↔ Go comparison in the Video renderer parity baseline.
- `media-geometry-v1` with explicit source-bounds provenance, contain/cover/fill/none fit, source masking, and output crop.
- FrameState consumption of canonical media geometry with no canvas-sized source-bounds fallback.

### Merged PR #208 — FrameState parity diagnostics

Implemented:

- matching serializable browser/TypeScript and Go diagnostic envelopes around the fail-closed v1 adapter and visual FrameState evaluator;
- exact saved-timeline/frame sample comparison with `1e-9` numeric tolerance for available state;
- semantic code/path comparison for unavailable state;
- stable quantized SHA-256 state fingerprints;
- a transition-free diagnostic control so CI exercises actual representable FrameState values while v1 transition ambiguity remains fail closed;
- permanent Quality Gate integration with diagnostic reports retained in the existing parity artifact.

A CodeQL review finding on an overflow-prone `len(left)+len(right)` map-capacity hint was fixed before merge by removing the combined untrusted-capacity preallocation. No unresolved review threads remained at merge.

### Merged PR #209 — canonical media geometry foundation

Implemented:

- `backend/internal/video/rendercontract/media_geometry.go` and `frontend/src/video/renderContractMediaGeometry.ts` define matching `media-geometry-v1` evaluators;
- `contain`: uniform scale by the smaller viewport/source ratio;
- `cover`: uniform scale by the larger ratio;
- `fill`: independent X/Y scale to the viewport;
- `none`: source pixels remain 1:1 and are centered;
- `mask_source_crop` reduces the explicit source box before fit;
- `transform.crop` produces a distinct output clip box after fit;
- collapsed/invalid crop regions fail closed;
- missing explicit source/content bounds fail closed instead of guessing the source aspect ratio;
- `video-renderer/test/fixtures/media-geometry-v1.json` is shared by Go and TypeScript tests and covers every fit mode plus source-mask/output-crop composition;
- initial Quality Gate formatting drift was corrected before merge;
- corrected-head formatting, Go vet, backend unit/integration tests, race detector, frontend lint/unit/build, Security Scan, and relevant sandbox/desktop contract checks were green before merge.

Preview and FFmpeg compositor behavior were unchanged.

### Merged PR #212 — FrameState media geometry integration

Implemented:

- Go and TypeScript FrameState layers expose matching optional canonical `media_geometry` state;
- asset-backed clips no longer receive fabricated canvas-sized `content_bounds`;
- explicit source bounds are evaluated through the shared `media-geometry-v1` contract;
- missing asset source bounds produce `media_geometry:content_bounds` and keep the layer/state non-authoritative;
- supported asset `media_fit` and `mask_source_crop` semantics no longer remain unresolved once canonical geometry can evaluate them;
- anchor-space dimensions use evaluated painted media bounds for asset-backed media;
- the shared `visual-frame-state-v1` fixture carries explicit source provenance for its authoritative media clip and a negative missing-provenance case;
- Go and TypeScript tests assert geometry contract identity, painted bounds, and no fabricated geometry in the negative case.

Validation before merge:

- backend formatting, vet, unit/integration tests, and race detector passed;
- frontend lint, unit tests, Video Studio performance evidence, and production build passed;
- deterministic Video renderer parity baseline passed;
- Security Scan, container build, Linux/macOS sandbox assurance, browser-egress assurance, Windows desktop/sandbox contracts, plugin lifecycle, and Helm checks passed;
- repository-wide Playwright remained in system-package installation and never reached application tests, so it was documented as infrastructure-only incomplete rather than green.

Preview and FFmpeg compositor behavior were unchanged.

### Current PR #218 — canonical perspective projection

Implemented on the branch:

- `backend/internal/video/rendercontract/perspective_projection.go` and `frontend/src/video/renderContractPerspectiveProjection.ts` define matching `perspective-projection-v1` evaluators;
- no-scene-camera state preserves the preview-compatible 1200-canvas-pixel projection distance;
- an active authored scene camera uses its evaluated FOV and canvas height to derive projection distance;
- a positive per-clip `transform.perspective` overrides the inherited projection distance;
- zero or omitted clip perspective inherits projection;
- non-finite or non-positive resolved projection distances fail closed;
- projection uses a homogeneous matrix with `m[14] = -1 / distance` and records `origin_w = 1 - view_z / distance`;
- `visual-frame-state-v1` now exposes `perspective_projection` for every visual layer while preserving the existing camera-relative `model_matrix` unchanged;
- `clip_perspective` is no longer unresolved because its semantics are canonical;
- `video-renderer/test/fixtures/perspective-projection-v1.json` is shared by Go and TypeScript and covers inherited distance, clip override, and zero-value inheritance;
- the shared visual FrameState fixture verifies the 1200px preview default, active scene-camera FOV projection, projection matrix/source/distance, and a resolved 500px per-clip override;
- PR #218 was normalized onto merged `main` after #212 merged.

Remaining before #218 merge:

1. Complete final-head Quality Gate, Security Scan, and relevant assurance jobs.
2. Fix any formatting, type, fixture, or cross-runtime diagnostic drift discovered by hosted validation.
3. Mark ready and merge only after code validation is green or any infrastructure-only incompleteness is explicitly documented.
4. Keep preview and FFmpeg compositor behavior unchanged.

### Remaining Phase 2 work after #218

1. Finish any remaining anchor/content-bound provenance edge cases discovered by parity diagnostics.
2. Define transition placement/peer state and effect-stack ordering/animation.
3. Define text/shape/cursor evaluated state.
4. Define/compile serializable `AudioGraph` for timing/rate/channel/gain/fade/mute/solo/processing decisions.
5. Fail closed whenever an authorable field lacks canonical semantics.

### Phase 2 exit gate

Preview and export callers consume identical FrameState/AudioGraph fixtures. No renderer owns separate curve, range, ordering, transform, geometry, projection, or source-time math, and CI detects schema/type/fixture drift.

## Phases 3–7

### Phase 3 — Shared preview composition

Drive the program monitor from canonical FrameState/AudioGraph while preserving direct-manipulation UI state separately. Add diagnostic overlays for frame identity, active clip IDs, matrices/bounds, source time, transitions, and effects.

### Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets, manage Chromium health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux behind a guarded rollout.

### Phase 5 — Visual parity closure

Close media timing, fit/crop, anchors/transforms/2.5D/camera/z-order, opacity/blending, text metrics/fonts, shapes, transitions, effects, cursor state, and sRGB/browser/encoder color policy.

### Phase 6 — Audio parity closure

Build canonical 48-kHz stereo AudioGraph semantics for source time, rate/pitch, channels, mute/solo, gain automation, fades, program processing, processed stems, exact sample counts, and decoded delivery.

### Phase 7 — Rollout and legacy retirement

Shadow-render, collect safe parity/performance/failure telemetry, stage opt-in → default-on → legacy opt-out, preserve rollback, update capabilities/docs, then retire legacy composition only when canonical coverage and rollback criteria are satisfied.

## Validation matrix

Every Phase 2+ PR runs focused tests plus repository gates:

```text
cd backend
go test ./internal/video/...
go test ./...
go test -race ./...
go vet ./...

cd frontend
npm ci
npm run lint
npm run test:unit
npm run build

# when frame/visual/audio/worker/delivery behavior is touched
npm run test:smoke
```

Hosted CI is authoritative for platform/toolchain cases not reproducible in the current execution environment. Setup-only stalls are recorded explicitly and are never represented as passing.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest projection checks fail CI. |
| v2 codifies legacy FFmpeg approximations | Editor preview remains compatibility target; semantic changes require versioned contract behavior. |
| v1 adapter guesses ambiguous behavior | Ambiguous transitions and unsupported transforms fail closed. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState parity diagnostics. |
| Millisecond rounding creates boundary/source drift | Deterministic callers use canonical frame/range/source helpers. |
| Source aspect ratio is guessed from the canvas | `media-geometry-v1` requires explicit `content_bounds`; FrameState reports `media_geometry:content_bounds` rather than fabricating source dimensions. |
| Crop order differs between renderers | Source-mask crop occurs before fit; output transform crop is represented separately after fit. |
| Perspective differs between preview/export | `perspective-projection-v1` resolves one inherited/overridden distance, source, and matrix; FrameState carries it separately from the model transform. |
| FrameState claims authority before feature semantics are canonical | Explicit unresolved sets keep noncanonical families non-authoritative. |
| CI setup stalls hide code status | Record setup-only stalls separately; never label an unexecuted check green. |
| Chromium packaging/resource cost | Managed worker, admission control, health checks, guarded rollout; FFmpeg retained for decode/encode/mux. |
| Audio runtime differences | Explicit AudioGraph and unsupported-boundary policy before shared export becomes default. |

## Implementation log

### 2026-08-17

- PR #187 established immutable submission and deterministic parity evidence.

### 2026-08-18

- PRs #191, #193, #194, #195, #196, #198, #199, #202, #204, #205, and #206 advanced the canonical contract from schemas/timing through exact-frame visual FrameState.
- PR #201 repaired the reachable PDF dependency vulnerability encountered during the program.
- PR #208 added permanent cross-runtime visual FrameState diagnostics and merged as `52683e4d25b22f70e5c6c3b4a8cf3417240be4bc` after focused parity validation and remediation of the CodeQL allocation finding.
- PR #209 opened on `feat/video-wysiwyg-phase2-media-geometry` and began canonical media fit/crop/source-bounds semantics with a shared Go/TypeScript fixture.

### 2026-08-19

- PR #209's first Quality Gate identified only Go formatting drift in the new geometry evaluator/tests; both files were corrected on head `3e11a21a1ea105060a8484de7ee98c2a60c9cc8d`.
- Corrected #209 code-executing Quality Gate steps passed formatting, Go vet, backend unit/integration tests, race detector, frontend lint/unit/build, and Windows/macOS contract checks; Security Scan and sandbox assurances were green.
- PR #209 merged to `main` as `6365b3dcc13fac0726e7407735c2a6b5664e0d1a`.
- PR #212 consumed `media-geometry-v1` in visual FrameState, removed canvas-sized source-bounds guessing, made missing source provenance explicit/non-authoritative, and passed all code-executing gates plus the deterministic renderer parity baseline.
- PR #212's repository-wide Playwright job remained in `Install system dependencies` and never reached application tests; that infrastructure-only incompleteness was documented explicitly before merge.
- PR #212 merged to `main` as `ae29d57e2e7d4e94e298bb155501583f4577e1ed`.
- PR #218 opened on `feat/video-wysiwyg-phase2-perspective-projection`, was normalized onto merged `main`, defined cross-runtime `perspective-projection-v1`, integrated it into visual FrameState, and advanced the shared fixtures.
- Manual semantic review of #218 caught a no-scene-camera compatibility mismatch before merge; FrameState was corrected to preserve the preview's 1200-canvas-pixel default rather than applying the default 50° FOV distance outside an authored scene camera. Final-head validation is pending.

## Next recommended slice

Finish PR #218 first:

1. Complete final-head Quality Gate/security/assurance validation and remediate any drift.
2. Mark ready and merge #218 after validation is defensible.
3. Inspect parity diagnostics for any remaining anchor/content-bound provenance edge case; close it if present.
4. Move immediately to transition placement/peer state and effect-stack ordering/animation as the next Phase 2 canonical feature family.
5. Continue Phase 0 production visual thresholds, unsupported-audio boundary, and second-platform evidence in parallel.
