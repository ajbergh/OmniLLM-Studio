# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-18  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and the final decoded export must represent the same immutable timeline revision with the same frame, layer, timing, geometry, styling, effect, transition, camera, and audio decisions.

> This file is the durable execution tracker for WYSIWYG rendering work. Every implementation PR in this program must update the tracker, implementation log, validation state, and next recommended slice before merge.

## Current handoff

- PR #187, **Establish immutable video rendering and deterministic parity baseline**, merged to `main` on 2026-08-17 as merge commit `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.
- Phase 1 immutable render submission is complete.
- Phase 0 has reproducible hosted evidence and passing audio/delivery gates, but remains open for visual threshold approval, unsupported-audio-boundary coverage, and a second OS/FFmpeg evidence run.
- The 103-frame visual baseline is intentionally a **known-mismatch baseline**. It proves the current preview and FFmpeg composition engines disagree; its mismatch values must not be adopted as acceptable production thresholds.
- The immediate implementation focus has moved to **Phase 2 — Canonical contract** while Phase 0 sign-off work remains in parallel.
- Active branch: `feat/video-wysiwyg-phase2-contract-foundation`.

## Implementation tracker

| Phase | Status | Progress note |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic fixture, immutable snapshot capture, 103 exact 640×360 frame pairs, 206 unique diagnostics, independent PCM reference, EBU R128, delivery timing checks, reports, and hosted artifacts are implemented. Final visual threshold approval, unsupported-audio boundary, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots, staged source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale-request rejection, legacy labeling, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Contract foundation started 2026-08-18: strict Timeline v2 and Render Manifest v1 schemas, shared cross-runtime contract fixture, canonical rational frame/boundary/source-time/easing helpers in Go and TypeScript, and frontend keyframe sampling delegation. Full canonical FrameState/AudioGraph evaluation and renderer adoption remain. |
| Phase 3 — Shared preview composition | Not started | Replace preview-local composition math with canonical FrameState/AudioGraph consumption. |
| Phase 4 — Shared Chromium render worker | Not started | Headless/browser renderer consumes the same canonical composition package behind a guarded rollout flag. |
| Phase 5 — Visual parity closure | Not started | Close geometry, text, crop/fit, effects, transitions, cursor, camera, color, and deterministic asset-loading parity. |
| Phase 6 — Audio parity closure | Not started | Shared AudioGraph/processed-stem architecture, rate/pitch policy, gain automation, fades, channel mapping, and delivery audio verification. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, and eventual retirement of the legacy composition path. |

## Verified baseline from PR #187

The merged Phase 0/1 work established the evidence and immutability prerequisites for a real WYSIWYG migration:

- deterministic `parity-torture-v1` 20-second, 640×360, 30 fps fixture;
- 103 named frames covering clip boundaries, keyframes, transitions, scenes, markers, seeded random samples, media ratios/rates, transforms, text, shapes, effects, cursor behavior, captions, camera motion, and audio cases;
- immutable snapshot frame endpoint producing lossless diagnostic PNGs;
- immutable snapshot 48 kHz stereo signed-16 PCM endpoint;
- independent browser/Web Audio reference PCM;
- automated real-project seeding, asset upload, timeline save, immutable snapshot creation, preview capture, render capture, delivery render, and report generation;
- visual metrics including channel tolerance/pass rate, MAE, RMSE, max delta, SSIM, changed bounds, structural regions, side-by-side images, and heat maps;
- audio metrics including sample count, offset, correlation, sample peak, EBU R128 integrated loudness, and true peak;
- decoded delivery validation for frame count, frame rate, PTS start, duration, and time base;
- hosted CI artifact retention with toolchain/font metadata and service logs.

### Hosted evidence state

Quality Gate `32074904557` / artifact `9303432653` verified:

- 103/103 preview PNGs at exactly 640×360;
- 103/103 rendered PNGs at exactly 640×360;
- 206/206 uniquely indexed diagnostic PNG filenames;
- equal 3,840,000-byte PCM streams for the 20-second reference/export pair;
- zero audio offset;
- correlation approximately `0.999993`;
- integrated loudness `-21.1 / -21.1 LUFS`;
- true peak `-3.1 / -3.0 dBFS`;
- decoded delivery `600/600` frames, constant 30 fps, PTS 0, exact 20.0-second duration, time base `1/15360`.

The visual baseline remains intentionally failing: 0/103 frames passed the provisional visual gate, with mean pixel pass rate approximately `0.182464` and mean SSIM approximately `0.094076`. These values are evidence of architectural divergence, not acceptable thresholds.

## Architectural decision

### Preview is the v1 compatibility target

For v1 timelines, the current editor preview defines compatibility semantics unless a behavior is explicitly declared incorrect and migrated through a versioned contract change. The legacy FFmpeg fidelity/compositor path must not become the source of truth simply because it already exports media.

Confirmed example: frontend `ease-in-out` is piecewise quadratic while the legacy Go fidelity evaluator uses smoothstep. Phase 2 therefore defines the canonical v1-compatible easing as the editor's piecewise quadratic curve. Legacy export callers migrate to the canonical evaluator rather than changing preview behavior to match the approximation.

### Freeze the legacy compositor during migration

Do not spend Phase 2 adding more approximation-only behavior to the legacy FFmpeg compositor unless required for a correctness/security regression. New WYSIWYG semantics belong in the canonical contract and shared composition path.

### Renderer-independent core

The canonical core must be:

- pure and deterministic;
- free of media I/O, browser APIs, FFmpeg command construction, filesystem access, and network access;
- serializable for fixtures and diagnostics;
- usable by preview and export callers;
- explicit about unsupported authorable fields and fail closed on unknown semantics.

## Canonical timing and ordering rules

These rules are contract requirements, not implementation suggestions:

1. `frameIndex` is the integer output-frame identity.
2. `frameTime` is rational `frameIndex / fps`; do not round-trip through milliseconds for render decisions.
3. Authored millisecond starts map to frames with `floor(ms × fps / 1000)`.
4. Authored millisecond ends map to exclusive frames with `ceil(ms × fps / 1000)`.
5. Activity is half-open: `startFrame <= frameIndex < endFrame`.
6. Layer order is stable and deterministic. The canonical tie-break tuple is `(track array index, z_index, clip array index)`; clip start time is not a z-order tie-breaker.
7. Source time is derived from canonical output time, trim, and playback rate in one evaluator.
8. Each keyframe segment uses the **later** keyframe's easing/curve, matching current editor behavior.
9. v1-compatible built-in easing is:
   - `linear`: `t`;
   - `ease-in`: `t²`;
   - `ease-out`: `1 - (1 - t)²`;
   - `ease-in-out`: piecewise quadratic (`2t²` before 0.5, symmetric quadratic after 0.5);
   - `step`: hold previous value until segment completion.
10. Composition matrix order, anchor behavior, crop/fit behavior, camera projection, transition placement, effect ordering, text bounds, and color-space assumptions must be encoded explicitly in Timeline v2/FrameState rather than inferred independently by renderers.

## Phase 0 — Reproducible parity baseline

### Objective

Maintain a deterministic, inspectable comparison harness that can prove both regressions and convergence while the renderer changes underneath it.

### Complete

- deterministic torture fixture and media generation;
- immutable project/snapshot seeding;
- named frame sampling;
- exact-dimension preview and render PNG capture;
- visual metrics and artifacts;
- independent browser preview PCM;
- snapshot diagnostic PCM;
- EBU R128 gates;
- decoded delivery timing validation;
- hosted CI artifact generation and retention;
- toolchain/font metadata capture.

### Remaining before Phase 0 sign-off

1. Review the 103 changed-bounds/heat-map results by feature family.
2. Freeze production visual thresholds and any zero-tolerance structural regions. Do **not** use the current mismatch distribution as the target.
3. Define the unsupported-audio boundary for playback-rate pitch preservation, custom volume curves, and full-program processing until Phase 6 provides a shared AudioGraph/processed stem.
4. Run the complete fixture on a second supported OS/FFmpeg environment and record deltas before approving audio tolerances.
5. Keep the baseline runnable throughout Phases 2–7.

### Exit gate

A reproducible parity report exists in CI; known mismatches are correctly identified; threshold and unsupported-boundary policy is documented and approved.

## Phase 1 — Immutable render submission

**Status: Complete.**

### Delivered

- timeline revision and canonical SHA-256 identity;
- HTTP stale-submission rejection with `409`;
- immutable render snapshots bound one-to-one to jobs;
- stable sorted asset manifest and hashes;
- snapshot-owned staged source bytes;
- FFprobe decode preflight and frozen media metadata;
- worker execution/recovery from persisted snapshot only;
- source re-verification and fail-closed missing/mutated input behavior;
- snapshot/timeline/manifest/contract/renderer identity on outputs;
- explicit `legacy_mutable_source` handling for historical jobs;
- frontend save-before-enqueue and hash-based “changed since render” state, including overlapping jobs;
- path-specific export capability diagnostics and Strict Parity blocking behavior;
- service-owned render/generation worker lifecycle and clean shutdown.

### Exit gate

Met: a queued render is bound to one immutable timeline revision and one immutable set of source bytes and cannot silently change if the project is later edited.

## Phase 2 — Canonical contract

**Status: In progress.**

### Objective

Centralize non-I/O render semantics so preview and export callers consume identical decisions rather than independently reimplementing timing, interpolation, ordering, geometry, and audio planning.

### Contract artifacts

Canonical contract files live under `video-renderer/` even before the shared Chromium worker is introduced:

- `video-renderer/contracts/timeline-v2.schema.json`
- `video-renderer/contracts/render-manifest-v1.schema.json`
- `video-renderer/test/fixtures/render-contract-v1.json`

Language adapters/evaluators are expected to prove conformance against the same fixtures until a single shared runtime can replace the duplicated language boundary.

### Implemented in current Phase 2 foundation slice

- Added strict JSON Schema draft 2020-12 Timeline v2 contract with `additionalProperties: false` at authorable semantic nodes and explicit extension points for metadata/effect parameter payloads.
- Added Render Manifest v1 schema binding immutable snapshot/timeline/asset identity to explicit output settings.
- Added a shared cross-runtime fixture covering canonical easing, half-open frame boundaries, long/high-fps frame mapping, and playback-rate source-time mapping.
- Added `backend/internal/video/rendercontract` with pure rational frame-time, start/end frame, frame-count, activity, source-time, and v1-compatible easing helpers.
- Added `frontend/src/video/renderContract.ts` with matching helpers.
- Added Go and Vitest conformance tests against the same fixture.
- Changed frontend `keyframeUtils.ts` so built-in easing delegates to the canonical frontend render contract while Bezier and segment-local spring behavior remains in the current curve implementation.

### Remaining Phase 2 work

1. Define mechanically verified/generated Go and TypeScript Timeline v2 and Render Manifest v1 types from the schemas; CI must fail on schema/type drift.
2. Add a v1-to-canonical adapter. It must preserve the current editor preview as the compatibility target, not legacy FFmpeg approximations.
3. Complete explicit Timeline v2 semantics for:
   - media fit and mask-source crop;
   - deterministic content bounds;
   - transition ownership/placement and peer relationship;
   - text box size, padding, wrapping, and vertical alignment;
   - working color space;
   - primitive composition behavior.
4. Implement pure evaluators:
   - `normalizeTimeline`;
   - rational frame count/time helpers;
   - `activeClips` with stable ordering;
   - source-time mapping;
   - keyframe/property/curve evaluation including Bezier and spring;
   - transform matrix including anchor and camera;
   - transition evaluation;
   - ordered effect-stack evaluation including animated effect amounts;
   - text layout/shape state;
   - cursor state;
   - `compileAudioGraph`.
5. Define a serializable `FrameState` containing every visual decision required to paint one frame.
6. Define a serializable `AudioGraph` containing every audio source, timing, rate, channel, gain, fade, mute/solo, and processing decision.
7. Replace millisecond comparisons inside render paths with canonical frame/timebase helpers.
8. Add structured diagnostics with severity, code, timeline path, relevant IDs, and remediation.
9. Fail closed when an authorable field has no canonical semantics.
10. Migrate the legacy Go fidelity evaluator to the canonical v1-compatible easing/source-time/frame helpers while it remains in service.

### Required Phase 2 fixtures

- exact start/end boundaries;
- overlapping clips and stable z-order ties;
- every built-in easing plus Bezier and spring;
- long timelines and 120 fps;
- trims, playback rates, and export ranges;
- crop/fit/anchor behavior;
- camera/depth projection;
- every transition placement/direction;
- text/font/layout cases;
- animated effect amounts and stack order;
- cursor interpolation/click state;
- audio graph source/rate/gain/channel/mute/solo cases.

### Exit gate

Preview and export callers can consume identical `FrameState`/`AudioGraph` fixtures. No renderer owns separate curve, range, ordering, transform, or source-time math. CI detects schema/type/fixture drift.

## Phase 3 — Shared preview composition

### Objective

Make the editor program monitor a consumer of the canonical contract instead of a second rendering specification.

### Work

- Move active-layer resolution, transform/camera state, effect/transition state, text layout inputs, cursor state, and media source-time decisions behind canonical `FrameState` evaluation.
- Move preview audio scheduling behind canonical `AudioGraph` evaluation.
- Keep direct-manipulation UI state separate from persisted/canonical render state; commit operations still produce timeline mutations through the existing store conventions.
- Preserve parity automation hooks and diagnostic capture.
- Add visual debug overlays that can display canonical bounds, matrices, active clip IDs, frame index, source time, transition state, and effect state.

### Exit gate

The visible program monitor can be driven entirely from canonical evaluated state, with legacy preview-local evaluation removed or isolated behind an explicitly temporary adapter.

## Phase 4 — Shared Chromium render worker

### Objective

Render exported frames with the same browser composition implementation used by the authoritative preview.

### Work

- Build a deterministic composition entry point that accepts Render Manifest v1 plus frame index and returns/render-paints canonical FrameState.
- Package the render UI and fonts/assets for desktop and server modes.
- Add a managed headless Chromium worker with startup health checks, process ownership, cancellation, restart handling, concurrency limits, and bounded resource use.
- Pipe deterministic frame output to FFmpeg for encoding; FFmpeg remains the media decode/encode/mux tool where appropriate, not the visual-semantic authority.
- Guard the new worker behind a configuration flag and preserve legacy fallback during rollout.

### Exit gate

A snapshot can be rendered through the shared Chromium path on supported deployment targets, producing deterministic frame/audio inputs to the delivery encoder.

## Phase 5 — Visual parity closure

### Objective

Drive deterministic visual parity to approved thresholds feature by feature.

### Work families

1. base media decode/source timing;
2. contain/cover/fill and crop/mask geometry;
3. anchor, scale, rotation, 2.5D projection, camera, and z-order;
4. opacity/fades/blending;
5. text metrics, wrapping, font loading/fallback, line height, letter spacing, stroke, shadow, background, padding, and alignment;
6. primitive shapes/annotations;
7. transition ownership, clipping, and timing;
8. effect stack order and parameter animation;
9. cursor interpolation, highlights, and click effects;
10. sRGB working-space and browser/encoder color metadata policy.

### Exit gate

All named visual fixtures meet approved structural and perceptual thresholds on supported CI/platform matrices, with zero unexplained changed regions.

## Phase 6 — Audio parity closure

### Objective

Make preview audition and export derive from one canonical audio plan.

### Work

- canonical 48 kHz stereo AudioGraph;
- deterministic trim/source-time/playback-rate policy;
- explicit pitch-preservation policy and supported rate range;
- channel mapping/downmix/upmix rules;
- clip and track mute/solo;
- gain keyframes using canonical curve evaluation;
- fade evaluation;
- program-level processing representation;
- either shared offline processing or a processed-stem contract when browser/runtime parity requires it;
- exact-duration sample count and decoded-delivery audio checks.

### Exit gate

Independent preview/export PCM derived from the same immutable snapshot meets approved timing, correlation, peak, loudness, and unsupported-feature policy on supported platforms.

## Phase 7 — Rollout and legacy retirement

### Objective

Switch production export safely without losing rollback capability.

### Work

- shadow-render representative jobs through legacy and canonical workers;
- collect parity/performance/failure telemetry without exposing user media outside local/server boundaries;
- staged opt-in, then default-on, then legacy opt-out;
- explicit rollback switch;
- documentation and capability matrix update;
- remove legacy fidelity expansion only after the canonical worker covers supported authoring semantics and rollback criteria are met.

### Exit gate

Canonical shared rendering is the production default, parity gates remain green, rollback has aged out safely, and the legacy composition path can be removed without reducing supported authoring fidelity.

## Validation matrix

Every Phase 2+ PR should run the smallest applicable focused tests plus repository gates. Before merge of a renderer-affecting slice, validate at minimum:

### Backend

```text
cd backend
go test ./internal/video/...
go test ./...
go test -race ./...
go vet ./...
```

### Frontend

```text
cd frontend
npm ci
npm run lint
npm run test:unit
npm run build
```

### End-to-end / parity when touched

```text
npm run test:smoke
```

Run the parity fixture/report workflow whenever a change can affect frame selection, visual state, audio state, asset identity, worker behavior, or delivery timing. Hosted CI remains authoritative for platform/toolchain cases that cannot be reproduced locally.

## Risk register

| Risk | Control |
|---|---|
| Schema exists but runtime silently diverges | Shared fixtures now; generated/mechanically verified types and drift CI before Phase 2 exit |
| Canonical v2 accidentally codifies FFmpeg approximations | v1 adapter compatibility target is the current editor preview; intentional changes require versioned semantics |
| Millisecond rounding produces boundary flicker | Rational frame identity plus floor-start/ceil-end and half-open activity rules |
| Browser and Go easing/source-time drift | Shared cross-runtime fixture and canonical helper tests |
| Headless Chromium increases packaging/resource cost | Dedicated managed worker, concurrency/admission control, health checks, guarded rollout, FFmpeg retained for media/encoding |
| Font or color differences become platform-dependent | Bundled/declared font policy, toolchain metadata, explicit sRGB working-space contract, multi-platform parity evidence |
| Audio rate/channel behavior differs by runtime | Explicit AudioGraph and unsupported-boundary policy before enabling shared export by default |
| Legacy renderer receives new semantics during migration | Freeze approximation-only feature growth; route new semantics to canonical contract/shared worker |

## Implementation log

### 2026-08-17 — Phase 0/1 foundation

- Implemented immutable timeline revision/hash render submission, source staging, snapshot identity, decode preflight, recovery, stale-request rejection, and Strict Parity diagnostics.
- Implemented deterministic 103-frame visual/audio/delivery parity baseline and hosted artifact workflow.
- Fixed worker shutdown ownership, hosted FFmpeg provisioning, exact screenshot dimensions, unique diagnostic naming, mono-to-stereo gain behavior, eased volume automation, diagnostic PCM duration, Windows path-containment fallback, command-length issues, and delivery timing checks discovered by the torture fixture.
- Draft PR #187 completed hosted Quality Gate verification and was subsequently merged to `main` as `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.

### 2026-08-18 — Phase 2 contract foundation

- Refreshed `main` after PR #187 and confirmed subsequent merges #188/#189 do not supersede the WYSIWYG plan.
- Started branch `feat/video-wysiwyg-phase2-contract-foundation` from `main` commit `2443db2228187597df7e8d28cba01069f05ca629`.
- Added Timeline v2 and Render Manifest v1 JSON schemas under `video-renderer/contracts/`.
- Added `video-renderer/test/fixtures/render-contract-v1.json` as a shared Go/TypeScript contract fixture.
- Added pure Go `backend/internal/video/rendercontract` frame/time/source/easing primitives and fixture/schema tests.
- Added matching frontend `frontend/src/video/renderContract.ts` primitives and Vitest fixture coverage.
- Routed built-in frontend keyframe easing through the canonical frontend helper, preserving Bezier and segment-local spring behavior.
- Next validation step: open the Phase 2 foundation PR, inspect repository Quality Gate results, fix any failures, and merge only when the applicable gates are green.

## Next recommended implementation slice

After this foundation PR is validated and merged:

1. Add generated/mechanically verified Timeline v2 / Render Manifest v1 Go and TypeScript type projections and a CI drift check.
2. Implement the v1-to-canonical adapter with explicit compatibility tests against current preview behavior.
3. Expand canonical property/curve evaluation to include Bezier and spring and migrate the legacy fidelity evaluator to it.
4. Add canonical `activeClips`, stable ordering, source-time, and export-range fixtures; migrate diagnostic/render frame selection away from millisecond comparisons.
5. Define the first serializable `FrameState` and `AudioGraph` structures before beginning Phase 3 preview integration.
6. In parallel, close remaining Phase 0 sign-off items: visual feature-family review, production threshold policy, unsupported audio boundary, and second-platform evidence.
