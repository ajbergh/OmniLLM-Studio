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
- PR #199 — canonical preview/index ordering adoption — `19a1a7b635afd33954bc56ed6023845f2c9e3fd1`.
- PR #202 — deterministic frame-addressed preview/source selection — `02a1bbf4ec2b640a57d59fdd67f7906ae03eaa91`.
- PR #204 — canonical backend diagnostic/parity frame callers — `73fa7d78b5018eb19b88abc34790fd19e95a5a98`.
- PR #205 — canonical numeric property/keyframe evaluation across Go/TypeScript and preview/fidelity callers — `8a93f9ff90eeda92c944085715856907747584f1`.
- PR #206 — exact-frame visual FrameState foundation — `c37ba2ed8132133cc913531946d462c3b7b38911`.

Security unblock during this program:

- PR #201 — removed reachable `GO-2026-6115` by replacing `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.
- The production-only #201 head passed the repository dependency-vulnerability audit; the migration proof also passed the document parser regression and `govulncheck ./...` with no reachable vulnerabilities. Windows desktop compilation passed. Linux backend/Go-CodeQL runners were documented as setup-stalled rather than represented as green.

PR #198 merge validation and infrastructure exception:

- Final head `f3efbaeca241fa6dcb70cea5ac91fded72b3feda` passed frontend lint/unit/performance/build, backend format/vet/unit/integration/race, Helm, Windows/macOS platform checks, renderer parity baseline, dependency audit, and container validation.
- Its Playwright smoke job remained infrastructure-stalled in system-dependency installation, and final-head Go CodeQL remained infrastructure-stalled in desktop-dependency installation before CodeQL initialization.
- Immediately previous implementation head `b0d7e078dd81036e4228a90342d0487087ed98d9` passed complete Go/TypeScript CodeQL. Subsequent code changes were limited to field-specific trim diagnostics and a shared fixture case, covered by the green final-head tests.
- The exception was explicit; stalled/unexecuted jobs were not represented as passing.

PR #199 merge validation and infrastructure exception:

- The canonical-ordering frontend implementation passed lint, unit tests, Video Studio performance evidence, and production build on the byte-identical implementation head before the final security-base refresh.
- After PR #201 repaired the dependency graph, #199 was rebuilt onto the new `main`; its fresh normal dependency-vulnerability audit passed.
- The final combined Quality Gate remained runner-queued, not code-failing. #199 was merged under an explicit queue-only exception using the already executed byte-identical frontend evidence plus the fresh security result; queued checks were not represented as green.
- No unresolved review threads existed before merge.

PR #202 merge validation and infrastructure exception:

- Exact production `VideoPreviewCanvas.tsx` implementation `5a45434cde410fd88e28cd434eeb0864ce638005` passed hosted lint with zero errors, 106/106 unit tests, Video Studio performance evidence, and production build.
- Final production head passed frontend lint/unit/performance/build and Helm; the frontend container image also passed, and macOS auxiliary assurance checks passed.
- Security/backend/platform jobs remained runner-queued at merge time and were explicitly documented as unexecuted rather than green; no unresolved review threads existed.
- PR #202 merged as `02a1bbf4ec2b640a57d59fdd67f7906ae03eaa91`.

Current PR: **#208 — Bridge canonical FrameState into parity diagnostics**.
Current branch: `feat/video-wysiwyg-phase2-frame-state-diagnostics`.

Current slice: make canonical visual FrameState drift observable in the permanent parity baseline without changing preview or export composition. Browser/TypeScript and Go evaluate the same saved Timeline v1/frame samples through a shared diagnostic envelope; unsupported v1 semantics remain structured unavailable results rather than being guessed.

Focused implementation validation for #208:

- `visual-frame-state-diagnostic-v1` preserves adapter/runtime code, path, message, and remediation while containing generic evaluator failures behind `FRAME_STATE_EVALUATION_FAILED`.
- `backend/cmd/video-frame-state-diagnostic` compares available states recursively with a `1e-9` numeric tolerance, emits quantized SHA-256 fingerprints, and compares unavailable results by semantic code/path so wording differences do not create false drift.
- Hosted run `32200543950` passed focused Go tests/vet, frontend lint with zero errors, all 106 unit tests, and production build. Its live parity project then produced 103/103 matching fail-closed diagnostics for the real saved timeline and 103/103 matching available FrameState values for a transition-free diagnostic control, with zero mismatches.
- Permanent Quality Gate wiring runs both checks inside the existing immutable Video renderer parity baseline and retains the diagnostic JSON/reports in the existing parity artifact.

## Phase tracker

| Phase | Status | Progress |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Initial feature-family review confirms structural visual divergence. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Schemas, projections, timing/easing/curves, v1 adapter, frame activity/range/source/order, runtime normalization, indexed preview ordering, deterministic frame/source addressing, backend frame callers, property/keyframe evaluation, and exact-frame visual FrameState are merged. PR #208 adds permanent cross-runtime FrameState diagnostics. Full media fit/mask/perspective semantics, transitions/effects, text/shape/cursor state, and AudioGraph remain. |
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

### Deterministic capture vs interactive playback

- Explicit render/parity frame requests are frame-addressed and must use canonical frame-overlap activity plus `sourceTimeAtFrameMs` semantics.
- Interactive playback/scrubbing may remain playhead-time-addressed for responsiveness until the shared FrameState path supersedes preview-local behavior.
- A deterministic frame address is authoritative only while the playhead still represents that exact rational frame address and playback is paused.
- Deterministic paused media seeks use a sub-frame tolerance; the legacy 50 ms scrub tolerance is too wide for high-frame-rate deterministic capture.

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
- Go and TypeScript implement matching evaluator-scoped, non-mutating normalization.
- Exact Timeline v2 version/canvas/FPS/background/working-color checks and canonical sRGB defaulting.
- Stable unique track/clip IDs, supported track types/heights, nonnegative timing, positive duration, and playback-rate range/defaulting.
- Canonical `trim_out_ms = trim_in_ms + round(duration_ms × playback_rate)` with exact path-addressed trim diagnostics.
- Visual transform defaults plus finite/opacity/crop validation.
- Asset-backed visual clips default to `media_fit: contain`; unsupported fit values fail closed.
- Timeline duration expands to the maximum clip end.
- The normalizer remains intentionally narrower than full feature semantics; text/shape/effect/transition/camera/content-bounds/mask behavior is owned by later evaluators.

**PR #199 — indexed preview ordering adoption**
- `frontend/src/video/renderContractEvaluation.ts` exposes the shared canonical clip comparator.
- `frontend/src/components/video/pro/timelineIndex.ts` retains original `clipIndex` independently of temporal sorting, preserves prefix-max interval lookup, restores candidates to canonical order, and keeps deterministic selected-video decoder prioritization.
- `frontend/src/components/video/VideoPreviewCanvas.tsx` uses the same canonical ordering after mounted-video/poster recombination so decoder budgeting cannot alter equal-z composition order.
- Regression tests prove temporal index order can differ from authored composition order without semantic leakage.

**PR #202 — deterministic frame-addressed preview source selection**

- `frontend/src/components/video/pro/timelineIndex.ts` adds `queryActiveClipsAtFrame` using canonical `activeAtFrame` semantics; deterministic capture includes sub-frame-authored clips under floor-start/ceil-end mapping while interactive playback continues using the indexed millisecond query.
- `frontend/src/components/video/sourceTiming.ts` introduces explicit frame versus time addresses. Frame addresses delegate to canonical `sourceTimeAtFrameMs`; time addresses retain sub-frame-responsive `sourceTimeMs` behavior.
- `frontend/src/components/video/VideoPreviewCanvas.tsx` stores authoritative parity frame identity in React state, uses frame-overlap candidates and frame-derived source time only while an explicit paused frame address remains authoritative, and exposes that identity to the parity harness.
- Parity readiness queries marked preview videos directly so media synchronization refs remain private to the mutation path and React immutability lint remains satisfied.
- Deterministic paused seeks use a 0.5 ms tolerance while interactive scrubbing retains the 50 ms tolerance.
- Focused tests cover 120-fps sub-frame clip activity, direct frame-derived source time, no integer-millisecond roundtrip, trim/rate behavior, frame-address validity, and seek tolerance.

**PR #204 — backend diagnostic/parity frame caller migration**

Implemented:

- `backend/internal/video/diagnostic_frame.go`
  - replaces duplicate floating-point `ceil(duration × fps / 1000)` bounds math with canonical `rendercontract.FrameCount`.
- `backend/internal/video/parity_fixture.go`
  - uses canonical `FrameCount`, `StartFrame`, and `FrameTime` for sample identity and presentation metadata;
  - high-FPS sub-frame authored starts map to the same output frame identity as preview/canonical evaluation.
- `backend/internal/video/renderer_fidelity.go`
  - replaces diagnostic `floor(frameIndex × 1000 / fps)` point filtering with canonical frame-domain authored activity;
  - keeps diagnostic frame indices relative to an export-range origin without integer-ms round-tripping;
  - tags render-only sampled segments with parent provenance;
  - collapses canonically overlapping synthetic samples to the single sample containing the exact rational frame presentation time, falling back to the earliest overlapping sample when an authored clip begins partway through the output frame;
  - preserves cursor pointer/ring point-sampling until cursor semantics are explicitly canonicalized.
- `backend/internal/video/renderer_parity_test.go`
  - covers exact half-open boundaries, 120-fps sub-frame/range-relative starts, high-FPS parity sample mapping, and the synthetic-segment double-stack regression.
- FFmpeg diagnostic seek remains direct `frameIndex / fps`; it was already rational-frame-derived and is intentionally unchanged.

Scope boundary: PR #204 does not canonicalize property/keyframe, transform/camera, transition/effect, cursor, or AudioGraph semantics.

### Merged PR #205 — canonical numeric property/keyframe evaluation

Implemented:

- `backend/internal/video/rendercontract/property_evaluation.go` and `frontend/src/video/renderContractProperties.ts`
  - define one generic numeric keyframe sampler with trimmed/case-insensitive property matching, deterministic authored ordering, flat holds before/after the authored range, and the later keyframe's curve/easing owning each segment;
  - keep generic sampling property-agnostic so `effect.*` automation can reuse interpolation before effects receive full canonical semantics;
  - define supported clip properties (`x/y/z`, scale axes, rotations, opacity, volume) and camera properties (`x/y/z`, rotations, field of view, focus depth);
  - define v1-preview-compatible base/default values, including scale-axis fallback to uniform scale, `rotation_z` fallback to 2D rotation, volume default 1, and field-of-view default 50;
  - fail closed when semantic clip/camera evaluation is requested for an unsupported property.
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
  - delegates visual transform, scene-camera, and managed media volume property evaluation to the canonical semantic evaluator while leaving direct-manipulation overrides and transform composition unchanged.
- `frontend/src/components/video/effects/keyframeUtils.ts`
  - remains the editor-facing compatibility wrapper/cache but delegates interpolation to the canonical sampler.
- `frontend/src/components/video/parity/previewAudioRenderer.ts`
  - uses canonical clip volume evaluation for parity-audio gain curves.
- `backend/internal/video/renderer_fidelity.go`
  - delegates its legacy transform/camera/effect-amount keyframe interpolation helper to the canonical generic sampler; projection, transition, and effect semantics remain unchanged.
- Shared Go/TypeScript fixture prevents future interpolation/default drift.

Scope boundary: PR #205 does not yet define matrix/anchor/perspective/crop composition, camera projection state, transition peer semantics, effect-stack ordering, text/shape/cursor state, or AudioGraph semantics.

### Merged PR #206 — exact-frame transform/camera FrameState foundation

Implemented:

- `backend/internal/video/rendercontract/frame_property.go` keeps output-frame presentation time rational when sampling clip/camera keyframes, preventing high-FPS deterministic state from round-tripping through integer milliseconds.
- `backend/internal/video/rendercontract/frame_state.go` evaluates visible active visual layers in canonical order, frame-derived source time, clip transforms/fades/crop/content bounds, exact scene camera state, stage perspective distance, camera-relative view transforms, and row-major model matrices.
- `frontend/src/video/renderContractFrameState.ts` mirrors the Go evaluator so diagnostics and later preview/export consumers can share one serializable state contract.
- `visual-frame-state-v1` deliberately marks effects, transitions, text, shapes, cursor state, mask-source crop, non-`contain` media fit, non-zero clip perspective, and anchors without known bounds as unresolved; a state is authoritative only when its unresolved set is empty.
- Shared Go/TypeScript fixture verifies exact-frame semantics and cross-runtime output.

Scope boundary: PR #206 establishes the pure state/evaluation foundation only. It does not yet replace preview composition, renderer composition, transition/effect/text/shape/cursor semantics, non-`contain` media geometry, clip-perspective projection, or AudioGraph behavior.

### Current PR #208 — canonical FrameState parity diagnostics

Implemented:

- `frontend/src/video/renderContractFrameStateDiagnostics.ts` and `backend/internal/video/frame_state_diagnostic.go` expose the same serializable availability/error/state envelope around the existing fail-closed v1→v2 adapter and visual FrameState evaluator.
- `scripts/video-frame-state-diagnostics.mjs` captures browser/TypeScript diagnostics from the saved normalized Timeline v1 used by the parity project. A clearly labeled transition-free control removes only transition records from a copy so CI also exercises actual representable FrameState values without changing production adapter semantics.
- `backend/cmd/video-frame-state-diagnostic` evaluates the same timeline/frame samples in Go, compares structured unavailable results by semantic code/path, compares available state recursively at `1e-9` numeric tolerance, records fingerprints, and exits nonzero on drift.
- `.github/workflows/ci.yml` makes these checks part of the existing Video renderer parity baseline before the legacy pixel/audio report. The existing `video-parity-baseline` artifact retains the browser diagnostics, saved/control timelines, Go reports, and ordinary parity evidence together.
- Focused live run `32200543950` proved both sides of the contract: real saved timeline = 0 available / 103 matched unavailable / 0 mismatches; transition-free control = 103 available compared / 0 unavailable / 0 mismatches.

Scope boundary: #208 is diagnostic-only. It does not change preview composition, FFmpeg composition, v1 transition semantics, canonical geometry semantics, or AudioGraph behavior.

### Remaining Phase 2 work

1. Validate and merge PR #208 canonical FrameState parity diagnostics.
2. Canonicalize remaining media geometry: `cover`/`fill`/`none`, mask-source crop, clip perspective, and content-bounds provenance/anchor edge cases.
3. Define transition placement/peer state and effect-stack ordering/animation.
4. Define text/shape/cursor evaluated state.
5. Define/compile serializable `AudioGraph` for timing/rate/channel/gain/fade/mute/solo/processing decisions.
6. Fail closed whenever an authorable field lacks canonical semantics.

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
| Millisecond rounding creates boundary/source drift | Deterministic preview and backend diagnostic/parity frame callers use canonical frame/range helpers; later FrameState/property evaluators must stay frame-addressed. |
| Canonical overlap double-stacks renderer-generated samples | Fidelity samples carry render-only parent provenance and diagnostic selection collapses overlapping samples to one rational-presentation sample per authored parent. |
| Deterministic frame state leaks into interactive playback | Frame authority is explicit state, valid only while paused at the exact addressed frame; free-running playback keeps the indexed time-addressed path. |
| High-FPS deterministic seeks reuse stale frames | Frame-addressed paused seeks use a 0.5 ms tolerance instead of the interactive 50 ms scrub tolerance. |
| Browser/Go curve drift | Shared built-in/Bezier/spring fixtures. |
| Browser/Go property sampling/default drift | Versioned property-evaluation fixture covers interpolation, property normalization, clip/camera bases, and fail-closed unsupported semantic requests; preview/fidelity callers delegate instead of reimplementing. |
| FrameState claims authority before paint semantics are canonical | `visual-frame-state-v1` carries explicit layer/global unresolved sets; any unresolved feature makes the state non-authoritative until its dedicated evaluator lands. |
| Cross-runtime FrameState drift hides behind v1 adapter unavailability | Permanent parity CI checks the real saved timeline for identical fail-closed code/path and a transition-free control for 103 actual browser/Go state comparisons at `1e-9` numeric tolerance. |
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
- PR #193 merged canonical Bezier/spring evaluation.
- PR #194 merged schema/type drift enforcement.
- PR #195 merged v1→Timeline-v2 adapter after full Quality Gate/Security/container validation.
- PR #196 merged canonical frame range, frame-derived source time, and active-clip order as `42dd64cda9feb75a637b622bf33ac1350a4febd9` after full post-rebase validation.
- PR #198 merged runtime Timeline v2 normalization as `67982f4fdd80062c9439c528362f75382e5c3268`; setup-only Playwright/Go-CodeQL stalls were explicitly documented.
- An unrelated current-`main` dependency audit exposed reachable `GO-2026-6115` through `github.com/ledongthuc/pdf`; PR #201 replaced it with `github.com/tsawler/tabula v1.6.14` and merged as `57cb7764a73203fc1194dbe51992e7ee4779817f` after the normal vulnerability audit passed.
- PR #199 was refreshed onto the repaired dependency graph, preserved the intended five-file ordering delta, and merged as `19a1a7b635afd33954bc56ed6023845f2c9e3fd1` with a documented queue-only final-head exception and prior executed byte-identical frontend evidence.
- PR #202 implemented the deterministic frame-addressed preview/source slice. A staging lint failure exposed React cross-effect media-ref aliasing; parity readiness was moved to a dedicated DOM media marker, after which focused lint, all 106 unit tests, performance evidence, and production build passed. The validated implementation landed as `5a45434cde410fd88e28cd434eeb0864ce638005`, temporary integration scaffolding was removed, and PR #202 merged as `02a1bbf4ec2b640a57d59fdd67f7906ae03eaa91` under a documented queue-only final-head exception.
- PR #204 audited backend output-frame callers. The first canonical-overlap design was strengthened after identifying possible double-stacking of adjacent synthetic fidelity samples at high output FPS. Parent provenance plus rational presentation-time selection fixed that renderer-specific ambiguity; macOS validation `32174332876` passed focused backend tests and vet, and PR #204 merged as `73fa7d78b5018eb19b88abc34790fd19e95a5a98`.
- PR #205 introduced shared Go/TypeScript numeric property evaluation and migrated preview/fidelity sampling callers. Hosted integration run `32190060388` passed focused backend tests/vet plus frontend lint/unit/build before landing validated caller commit `b4652b779e2a9af5395d3712c968a5d57d5a8e4c`; #205 merged as `8a93f9ff90eeda92c944085715856907747584f1`.
- PR #206 added exact-frame clip/camera property sampling and the first visual FrameState foundation. Initial run `32194804878` exposed the stale normalizer symbol; repair/validation run `32194929252` passed focused Go test/vet plus frontend lint, 106/106 unit tests, and production build. Final clean head `724e673ab41f956b44184ae871d2f1093b44e770` passed full Quality Gate, Playwright, renderer parity, both CodeQL languages, and vulnerability audits; #206 merged as `c37ba2ed8132133cc913531946d462c3b7b38911`.
- PR #208 bridges visual FrameState into parity diagnostics without changing compositor behavior. Hosted run `32200543950` passed focused Go test/vet plus frontend lint, 106/106 unit tests, and production build; its live project produced 103 matched fail-closed saved-timeline diagnostics and 103 matched available transition-free control states with zero mismatches.

## Next recommended slice

After PR #208 is green and merged:

1. Implement canonical media-geometry evaluation for `contain`/`cover`/`fill`/`none`, mask-source crop, clip perspective, and content-bounds provenance/anchor edge cases in Go and TypeScript with a shared fixture.
2. Fold those geometry results into visual FrameState so the permanent browser/Go diagnostic control begins checking the newly canonicalized values automatically.
3. Keep preview/export compositor behavior unchanged until the geometry state is validated; migrate callers in a following reviewable slice.
4. Continue Phase 0 production visual-threshold policy, unsupported-audio boundary, and second-platform evidence in parallel.
