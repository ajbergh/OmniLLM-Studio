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

PR #196 final post-rebase validation:

- Quality Gate `32153989115`: **passed**, including frontend lint/unit/performance/build, backend format/vet/unit/integration/race, Playwright smoke, renderer parity baseline, Windows/macOS platform checks, and Helm validation.
- Security Scan `32153989061`: **passed**.
- Container workflow `32153989087`: **passed** for frontend, backend, and Helm.
- Auxiliary sandbox/workspace/egress assurance workflows: **passed**.
- Review threads: none unresolved before merge.

Current branch: `feat/video-wysiwyg-phase2-runtime-normalize`.

Current slice: add evaluator-scoped Timeline v2 runtime normalization/defaulting in Go and TypeScript, with one shared cross-runtime fixture. This is deliberately separate from schema/type projection checks and deliberately narrower than full feature validation: it normalizes and rejects only the semantics needed by the canonical frame/range/source evaluators that now exist.

## Phase tracker

| Phase | Status | Progress |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Initial feature-family review confirms structural visual divergence. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Schemas, projections, timing/easing/curves, v1 adapter, and canonical frame activity/range/source/order evaluation are merged. Runtime v2 normalization is current. Preview/index adoption, property/transform/transition/effect evaluation, FrameState, and AudioGraph remain. |
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

### Current runtime Timeline v2 normalization slice

Branch: `feat/video-wysiwyg-phase2-runtime-normalize`.

Implemented on the branch:

- `backend/internal/video/rendercontract/normalize.go`
  - JSON deep-copy/nonmutation before normalization;
  - exact Timeline v2 version check;
  - positive canvas width/height, 1–120 fps, non-empty background;
  - nonnegative timeline duration;
  - missing working color → canonical sRGB; non-sRGB fails closed until another working space is versioned;
  - required root metadata/tracks/markers materialized when absent;
  - trimmed, non-empty, unique track IDs;
  - normalized/supported track type and optional 32–160 track-height validation;
  - globally unique, non-empty clip IDs;
  - nonnegative clip start/trims and positive duration;
  - playback rate defaults to 1 and is constrained to the Timeline v2 contract range 0.25–4;
  - canonical `trim_out_ms = trim_in_ms + round(duration_ms × playback_rate)`, minimum one source millisecond;
  - visual transform defaults `x=0`, `y=0`, `scale=1`, `rotation=0`, `opacity=1`;
  - finite transform checks, opacity 0–1, crop edges 0–1;
  - asset-backed visual clips default to `media_fit: contain`; unsupported fit values fail closed;
  - timeline duration expands to the maximum clip end when necessary;
  - effects/keyframes are normalized to arrays because they are required by the Timeline v2 projection.
- `frontend/src/video/renderContractNormalize.ts` mirrors these evaluator-critical semantics and emits the same path-addressed `TIMELINE_V2_RUNTIME_INVALID` failure contract.
- `video-renderer/test/fixtures/timeline-v2-normalization-v1.json` is shared by Go and Vitest and covers:
  - ID/type whitespace normalization;
  - duration expansion;
  - playback/source-window defaults;
  - partial visual transform defaulting;
  - visual media-fit default;
  - audio clip behavior without visual transform defaulting;
  - wrong version, unsupported fps, duplicate clip ID, invalid playback rate, and invalid opacity failures.
- Go and TypeScript tests assert input nonmutation and exact normalized serialized output from the same fixture.
- Clip-level metadata intentionally remains optional instead of being materialized as `{}`: it is not required by the current frame/range/source evaluators, and preserving omission avoids introducing a non-semantic cross-runtime shape difference. Root metadata remains required/defaulted.

Scope boundary:

This slice is **not** a replacement JSON-Schema engine and does **not** claim canonical semantics for text layout, shapes, effects, transitions, camera projection, content bounds, masks, or other later feature evaluators. Those fields remain structurally projected but are validated/evaluated only when their canonical semantics are implemented. This prevents the normalizer from silently inventing behavior.

Hosted validation for this branch is pending. Merge is blocked until applicable Quality Gate, Security Scan, container, smoke, parity, and review-thread checks are green.

### Remaining Phase 2 work

1. Validate and merge runtime Timeline v2 normalization/defaulting.
2. Add canonical clip-index/order identity to interval-index candidates and migrate preview ordering without sacrificing virtualization.
3. Migrate output-frame-driven preview/diagnostic/export source-time and frame-selection callers to canonical helpers.
4. Implement canonical keyframe/property evaluation on top of the merged curve evaluator.
5. Implement canonical transform/anchor/camera state.
6. Define transition placement/peer state and effect-stack ordering/animation.
7. Define text/shape/cursor evaluated state.
8. Define serializable `FrameState` containing every visual decision needed to paint one output frame.
9. Define/compile serializable `AudioGraph` for timing/rate/channel/gain/fade/mute/solo/processing decisions.
10. Fail closed whenever an authorable field lacks canonical semantics.

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
| Runtime normalizer silently invents later feature semantics | Text/shape/effect/transition/camera semantics remain outside this slice and fail closed in later evaluators. |
| v2 codifies FFmpeg approximations | Editor preview remains v1 compatibility target; intentional changes require versioned semantics. |
| v1 adapter guesses ambiguous semantics | Transitions/unknown transform semantics fail closed. |
| Cross-runtime normalizer/evaluator drift | Versioned shared fixtures asserted by Go and TypeScript. |
| Interval index accidentally defines z-order | Canonical order explicitly uses track index, z-index, clip index; interval index is performance-only. |
| Millisecond rounding creates boundary/source drift | Rational frame identity, floor-start/ceil-end, half-open activity, frame-derived source time. |
| Browser/Go curve drift | Shared built-in/Bezier/spring fixtures. |
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

## Next recommended slice

After runtime normalization is green and merged:

1. Add original `clipIndex` identity to `frontend/src/components/video/pro/timelineIndex.ts` candidates and route preview candidate ordering through canonical `(track index, z_index, clip index)` semantics without replacing virtualization.
2. Convert the program monitor's media-source seek calculation from playhead millisecond math to canonical frame-derived source-time evaluation, with explicit frame quantization behavior during deterministic rendering/capture.
3. Migrate output-frame-driven diagnostic/export frame-selection callers to canonical frame/range/source helpers where appropriate while keeping user-interaction playhead UX responsive.
4. Implement canonical keyframe/property evaluation, then define the first serializable FrameState.
5. Continue Phase 0 threshold policy, unsupported-audio boundary, and second-platform evidence in parallel.
