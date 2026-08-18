# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-18  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for WYSIWYG rendering. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Merged WYSIWYG foundations:

- PR #187 — immutable render submission and deterministic parity baseline — `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.
- PR #191 — Timeline v2 / Render Manifest v1 plus canonical frame/source/easing primitives — `aabbb31288277287673cbed8546c9eb3f38588e4`.
- PR #193 — canonical cubic-Bezier and spring evaluation — `3bed9faf8a868b3a125c25cb141769bfcd7861d2`.
- PR #194 — mechanically checked Go/TypeScript schema projections and contract constants — `62e2180b5153be505fac650cd41b3a0e2d951783`.
- PR #195 — Timeline v1 → canonical Timeline v2 compatibility adapter — `b5f76aa6328240a6b516d768756c34f68e6fdedb`.
- PR #196 — canonical frame activity, range mapping, source time, and stable clip ordering — `42dd64cda9feb75a637b622bf33ac1350a4febd9`.
- PR #198 — evaluator-scoped Timeline v2 runtime normalization/defaulting — `67982f4fdd80062c9439c528362f75382e5c3268`.

PR #196 final post-rebase validation:

- Quality Gate `32153989115`: **passed**, including frontend lint/unit/performance/build, backend format/vet/unit/integration/race, Playwright smoke, renderer parity baseline, Windows/macOS platform checks, and Helm validation.
- Security Scan `32153989061`: **passed**.
- Container workflow `32153989087`: **passed** for frontend, backend, and Helm.
- Auxiliary sandbox/workspace/egress assurance workflows: **passed**.
- Review threads: none unresolved before merge.

PR #198 merge validation and infrastructure exception:

- Final head `f3efbaeca241fa6dcb70cea5ac91fded72b3feda` passed frontend lint/unit/performance/build, backend format/vet/unit/integration/race, Helm, Windows/macOS platform checks, renderer parity baseline, dependency audit, and container workflow `32157611014`.
- Quality Gate `32157611051` had every repository-code and parity job green; its Playwright smoke job remained infrastructure-stalled in `Install system dependencies` before browser tests started.
- Final-head Security Scan `32157611045` passed dependency audit and TypeScript CodeQL; Go CodeQL remained infrastructure-stalled in `Install Go desktop build dependencies` before CodeQL initialization.
- Immediately previous implementation head `b0d7e078dd81036e4228a90342d0487087ed98d9` passed complete Security Scan `32156645125`, including Go and TypeScript CodeQL. The only subsequent code changes were field-specific trim diagnostic routing (+5/-2 Go, +5/-2 TypeScript) plus one shared fixture case; those final changes were covered by the green final-head Go/TypeScript tests.
- Review threads: none unresolved before merge.
- PR #198 was merged under an explicit setup-only CI infrastructure exception; the stalled jobs were not represented as green checks.

Current PR: **#199 — Preserve canonical Video Edit Studio preview clip ordering**.  
Current branch: `feat/video-wysiwyg-phase2-preview-ordering`.

Current slice: preserve the existing timeline interval-index/virtualization performance path while making canonical `(track array index, z_index, clip array index)` ordering explicit at every boundary where temporal lookup or decoder budgeting could otherwise reorder equal-z clips.

## Phase tracker

| Phase | Status | Progress |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Initial feature-family review confirms structural visual divergence. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Schemas, projections, timing/easing/curves, v1 adapter, canonical frame activity/range/source/order evaluation, and runtime v2 normalization are merged. PR #199 adopts canonical ordering in the indexed program-monitor path. Output-frame source selection, property/transform/transition/effect evaluation, FrameState, and AudioGraph remain. |
| Phase 3 — Shared preview composition | Not started | Program monitor consumes canonical FrameState/AudioGraph instead of preview-local semantic math. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close geometry, text, crop/fit, effects, transitions, cursor, camera, color, and deterministic asset-loading parity. |
| Phase 6 — Audio parity closure | Not started | Shared AudioGraph/processed-stem architecture, rate/pitch policy, gain/fades/channel mapping, and decoded-delivery verification. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, and eventual removal of legacy composition semantics. |

## Architectural rules

### Preview is the Timeline v1 compatibility target

Current editor preview semantics are the Timeline v1 compatibility target unless an intentional behavior correction is introduced through a versioned contract change. The legacy FFmpeg compositor is not semantic authority merely because it already exports media.

### Freeze legacy approximation growth

Do not add approximation-only semantics to the legacy FFmpeg compositor during Phases 2–4 except for correctness/security regressions. New WYSIWYG behavior belongs in the canonical contract and shared composition path.

### Renderer-independent canonical core

Canonical evaluators must be pure, deterministic, serializable, free of media/browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed on unknown authorable semantics.

### Structural authority vs runtime semantics

- `video-renderer/contracts/timeline-v2.schema.json` is the structural authority for Timeline v2.
- `video-renderer/contracts/render-manifest-v1.schema.json` is the structural authority for immutable render manifests.
- Go and TypeScript projections are mechanically checked against those schemas.
- Projection checks prevent structural drift; they do **not** replace runtime semantic normalization/validation.
- Runtime normalizers/evaluators must not silently validate feature families whose semantics have not yet been canonicalized.

### Canonical timing and ordering

1. `frameIndex` is integer output-frame identity.
2. Frame time is rational `frameIndex / fps`; output-frame-driven rendering must not round-trip through integer milliseconds.
3. Authored starts map with `floor(ms × fps / 1000)`.
4. Authored ends map to exclusive frames with `ceil(ms × fps / 1000)`.
5. Activity is half-open: `startFrame <= frameIndex < endFrame`.
6. Stable clip order is `(track array index, z_index, clip array index)`; clip start time is not a z-order tie-breaker.
7. Source time is derived from output-frame identity, clip start, trim-in, and playback rate in one canonical evaluator.
8. A keyframe segment uses the later keyframe's easing/curve.
9. Built-in easing is editor-compatible; Bezier and spring evaluation is shared and fixture-verified in Go and TypeScript.
10. Composition matrices, anchors, crop/fit, camera, transitions, effects, text bounds, cursor state, and color-space assumptions must become explicit FrameState semantics rather than renderer inference.

## Phase 0 evidence and remaining sign-off

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained hosted evidence `32074904557` / artifact `9303432653` verified:

- 103/103 preview and 103/103 rendered PNGs at exactly 640×360;
- 206 uniquely indexed diagnostics;
- equal 3,840,000-byte PCM streams;
- zero audio offset;
- correlation ≈ `0.999993`;
- integrated loudness `-21.1 / -21.1 LUFS`;
- true peak `-3.1 / -3.0 dBFS`;
- decoded delivery 600/600 frames, constant 30 fps, PTS 0, exact 20.0 seconds.

The retained visual baseline is intentionally a known-mismatch baseline: aggregate pixel-pass ≈ `0.183` and SSIM ≈ `0.091` reflect architectural divergence, not acceptable production tolerances. Transition, geometry/composition, 2.5D/camera, keyframe, playback-rate, and boundary samples demonstrate structural differences; audio/delivery remains the strongest current parity area.

Remaining Phase 0 sign-off:

1. Freeze production visual threshold policy and zero-tolerance structural regions.
2. Define unsupported-audio policy for pitch preservation, custom gain curves, and program processing until Phase 6.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable through Phases 2–7.

## Phase 1 — Immutable submission

**Complete.** A queued render is bound to one immutable timeline revision/hash and one immutable set of source bytes. Snapshot identity, source staging, decode preflight, recovery, stale-request rejection, and Strict Parity diagnostics are production foundations for later parity work.

## Phase 2 — Canonical contract

### Merged slices

**PR #191 — contract/timing foundation**
- Timeline v2 and Render Manifest v1 schemas.
- Rational frame/start/end/count/activity and source-time helpers.
- Shared Go/TypeScript built-in easing fixtures.

**PR #193 — motion curves**
- Canonical cubic-Bezier and segment-local spring evaluation in Go/TypeScript.
- Shared overshoot/default/custom curve fixtures.
- Preview keyframe utility delegates curve math to canonical helpers.

**PR #194 — schema/type drift enforcement**
- Serializable Go/TypeScript Timeline v2 and Render Manifest projections.
- Go reflection schema checks plus TypeScript exact-key/required-key checks.
- Schema-bound version/contract/sRGB/48-kHz/stereo constants.

**PR #195 — v1 compatibility adapter**
- Non-mutating Go/TypeScript v1→Timeline-v2 adapter.
- Backend v1 validation/defaulting remains normalization authority.
- Shared fixture covers defaults, source windows, crop/2.5D, camera/cursor, curves, metadata, and fail-closed boundaries.
- v1 transitions and unknown/invalid transform semantics fail closed rather than being guessed.

**PR #196 — frame activity/range/source/order**
- `FrameRange` / `frameRangeFromMs` half-open output-frame range semantics.
- Frame-derived source-time calculation without integer-ms round-tripping.
- Serializable active clip identity containing track/clip indices, IDs, z-index, frame bounds, and source time.
- Stable canonical ordering `(track index, z_index, clip index)`.
- Shared Go/TypeScript fixture covering boundaries, equal-z ties, track precedence, hidden-track temporal activity, rate/trim source time, 120-fps sub-frame mapping, and one-hour timeline mapping.
- Existing frontend interval index remains a performance structure; its temporal sorting is explicitly not semantic layer order.

**PR #198 — runtime Timeline v2 normalization**
- `backend/internal/video/rendercontract/normalize.go` and `frontend/src/video/renderContractNormalize.ts` implement matching evaluator-scoped, non-mutating normalization.
- Exact Timeline v2 version/canvas/FPS/background/working-color checks and canonical sRGB defaulting.
- Stable unique track/clip IDs, supported track types/heights, nonnegative timing, positive duration, and playback-rate range/defaulting.
- Canonical `trim_out_ms = trim_in_ms + round(duration_ms × playback_rate)` with exact path-addressed trim diagnostics.
- Visual transform defaults plus finite/opacity/crop validation.
- Asset-backed visual clips default to `media_fit: contain`; unsupported fit values fail closed.
- Timeline duration expands to the maximum clip end.
- Root metadata/tracks/markers and required effects/keyframes arrays are materialized; clip-level metadata intentionally remains optional.
- Shared Go/TypeScript fixture verifies exact normalized serialization, caller nonmutation, and fail-closed diagnostic paths.
- The normalizer remains intentionally narrower than full feature semantics: text/shape/effect/transition/camera/content-bounds/mask behavior is owned by later evaluators rather than guessed here.

### Current PR #199 — indexed preview ordering adoption

Implemented:

- `frontend/src/video/renderContractEvaluation.ts`
  - exposes `CanonicalClipOrderIdentity`;
  - exposes `compareCanonicalClipOrder` as the shared `(track index, z_index, clip index)` comparator;
  - `activeClipsAtFrame` delegates final ordering to that comparator.
- `frontend/src/components/video/pro/timelineIndex.ts`
  - retains original `clipIndex` separately from the interval index's temporal sort;
  - keeps the prefix-maximum interval lookup and therefore preserves long-timeline virtualization/performance behavior;
  - restores queried active candidates to canonical composition order after temporal lookup;
  - applies deterministic reverse-canonical priority to otherwise equal decoder candidates while preserving selected-video promotion.
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
  - uses the canonical indexed comparator for active layers;
  - uses the same comparator after mounted-video/poster recombination so decoder budgeting cannot silently alter equal-z composition order.
- `frontend/src/components/video/pro/timelineIndex.test.ts`
  - includes an authored-order fixture whose clip-array order intentionally differs from temporal start order;
  - proves the internal index can remain temporal while active results return canonical authored composition order;
  - covers equal-z clip-index ties and track-order precedence.

Scope boundary: PR #199 changes layer ordering only. Program-monitor media seeking still uses playhead-millisecond math; output-frame-derived source selection remains the next slice so its timing behavior can be reviewed and parity-tested independently.

### Remaining Phase 2 work

1. Validate and merge PR #199 canonical preview/index ordering adoption.
2. Migrate output-frame-driven preview/diagnostic/export source-time and frame-selection callers to canonical helpers.
3. Implement canonical keyframe/property evaluation on top of the merged curve evaluator.
4. Implement canonical transform/anchor/camera state.
5. Define transition placement/peer state and effect-stack ordering/animation.
6. Define text/shape/cursor evaluated state.
7. Define serializable `FrameState` containing every visual decision needed to paint one output frame.
8. Define/compile serializable `AudioGraph` for timing/rate/channel/gain/fade/mute/solo/processing decisions.
9. Fail closed whenever an authorable field lacks canonical semantics.

### Phase 2 exit gate

Preview and export callers consume identical FrameState/AudioGraph fixtures. No renderer owns separate curve, range, ordering, transform, or source-time math, and CI detects schema/type/fixture drift.

## Phases 3–7

### Phase 3 — Shared preview composition

Drive the program monitor from canonical FrameState/AudioGraph while preserving direct-manipulation UI state separately. Add diagnostic overlays for frame identity, active clip IDs, matrices/bounds, source time, transitions, and effects.

### Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets, manage Chromium health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux where appropriate behind a guarded rollout.

### Phase 5 — Visual parity closure

Close media timing, contain/cover/fill/crop, anchors/transforms/2.5D/camera/z-order, opacity/blending, text metrics/layout/fonts, shapes, transitions, effects, cursor state, and sRGB/browser/encoder color policy.

### Phase 6 — Audio parity closure

Build canonical 48-kHz stereo AudioGraph semantics for source time, rate/pitch, channels, mute/solo, gain automation, fades, program processing, processed stems, exact sample counts, and decoded delivery.

### Phase 7 — Rollout and legacy retirement

Shadow-render, collect safe parity/performance/failure telemetry, stage opt-in→default-on→legacy opt-out, preserve rollback, update capabilities/docs, then retire legacy composition only when canonical coverage and rollback criteria are satisfied.

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

Hosted CI is authoritative for platform/toolchain cases not reproducible in the current execution environment.

## Risk register

| Risk | Control |
|---|---|
| Schema changes without projections | Go reflection and TypeScript compile/Vitest drift checks fail CI. |
| Projection checks mistaken for runtime validation | Runtime normalization is explicit and evaluator-scoped; feature validation lands with each evaluator. |
| Runtime normalizer silently invents later feature semantics | Text/shape/effect/transition/camera semantics remain outside normalization and fail closed in later evaluators. |
| v2 codifies FFmpeg approximations | Editor preview remains v1 compatibility target; intentional changes require versioned semantics. |
| v1 adapter guesses ambiguous semantics | Transitions/unknown transform semantics fail closed. |
| Cross-runtime normalizer/evaluator drift | Versioned shared fixtures asserted by Go and TypeScript. |
| Interval index accidentally defines z-order | Original clip index is retained; queried candidates are restored with the canonical comparator after temporal lookup, and preview decoder/poster recombination uses the same comparator. |
| Millisecond rounding creates boundary/source drift | Rational frame identity, floor-start/ceil-end, half-open activity, frame-derived source time; output-frame caller adoption remains the next Phase 2 slice. |
| Browser/Go curve drift | Shared built-in/Bezier/spring fixtures. |
| CI runner setup stalls hide code status | Record setup-only stalls explicitly, preserve successful code-level evidence, and never label an unexecuted check as green. |
| Chromium packaging/resource cost | Managed worker, admission control, health checks, guarded rollout; FFmpeg retained for decode/encode/mux. |
| Font/color platform drift | Explicit sRGB contract, declared font policy, retained toolchain metadata, multi-platform evidence. |
| Audio runtime differences | Explicit AudioGraph and unsupported-boundary policy before shared export becomes default. |
| Legacy semantics expand during migration | Freeze approximation growth; semantic authority moves to canonical contract. |

## Implementation log

### 2026-08-17
- Phase 0/1 immutable submission and deterministic parity foundation implemented; PR #187 merged.

### 2026-08-18
- PR #191 merged canonical schemas and timing/source/easing foundation.
- Retained Phase 0 evidence reviewed by feature family; structural mismatch documented.
- PR #193 merged canonical Bezier/spring evaluation.
- PR #194 merged schema/type drift enforcement.
- PR #195 merged v1→Timeline-v2 adapter after full Quality Gate/Security/container validation.
- PR #196 implemented canonical frame range, frame-derived source time, and active-clip order with a shared Go/TypeScript fixture.
- #196 first correction was formatting-only; its semantic fixture passed both runtimes.
- Unrelated Music PR #197 advanced `main`; #196 was rebuilt cleanly on the new `main`, reopened after GitHub auto-closed the temporary zero-diff PR, and fully revalidated.
- #196 final post-rebase Quality Gate `32153989115`, Security `32153989061`, and container `32153989087` all passed; PR #196 merged as `42dd64cda9feb75a637b622bf33ac1350a4febd9`.
- Runtime Timeline v2 evaluator-scoped normalization was rebuilt directly on the #196 merge commit, with clip metadata intentionally left optional for cross-runtime serialization consistency.
- PR #198 final head passed backend/frontend tests, race, parity, containers, and dependency audit. Playwright smoke and final-head Go CodeQL remained stalled before repository execution; the prior implementation head had full Go/TypeScript CodeQL coverage. The infrastructure exception was documented on the PR, which merged as `67982f4fdd80062c9439c528362f75382e5c3268`.
- Draft PR #199 was opened as a stacked ordering slice, then brought onto the #198/main history with an exact four-file frontend diff before this plan update.

## Next recommended slice

After PR #199 is green and merged:

1. Convert deterministic/program-monitor media-source seek calculation from playhead millisecond math to canonical frame-derived `sourceTimeAtFrameMs`, explicitly separating frame-quantized rendering/capture from responsive free-running playback.
2. Migrate output-frame-driven diagnostic/export frame-selection callers to canonical frame/range/source helpers and add shared boundary/rate fixtures for caller behavior.
3. Implement canonical keyframe/property evaluation, then define the first serializable FrameState.
4. Continue Phase 0 threshold policy, unsupported-audio boundary, and second-platform evidence in parallel.
