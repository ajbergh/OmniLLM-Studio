# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-18  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and the final decoded export must represent the same immutable timeline revision with the same frame, layer, timing, geometry, styling, effect, transition, camera, and audio decisions.

> This file is the durable execution tracker for WYSIWYG rendering work. Every implementation PR in this program must update the tracker, implementation log, validation state, and next recommended slice before merge.

## Current handoff

- PR #187, **Establish immutable video rendering and deterministic parity baseline**, merged to `main` on 2026-08-17 as `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.
- PR #191, **Start canonical Video Edit Studio render contract**, merged to `main` on 2026-08-18 as `aabbb31288277287673cbed8546c9eb3f38588e4` after a fully green hosted Quality Gate, Security Scan, and container-build validation.
- Phase 1 immutable render submission is complete.
- Phase 0 has reproducible hosted evidence and passing audio/delivery gates. Initial feature-family visual review is now recorded below, but production visual-threshold approval, unsupported-audio-boundary policy, and second OS/FFmpeg evidence remain open.
- The 103-frame visual baseline remains a **known-mismatch baseline**. It proves the preview and legacy FFmpeg composition engines disagree; its mismatch distribution must never be used to relax production parity goals.
- Phase 2 is active. The current branch is `feat/video-wysiwyg-phase2-curve-evaluator`, which moves built-in easing, cubic Bezier, and segment-local spring evaluation into the renderer-independent contract and makes the editor preview consume it.

## Implementation tracker

| Phase | Status | Progress note |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Initial feature-family review confirms structural divergence, especially transitions and geometry/composition. Threshold policy, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots, staged source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale-request rejection, legacy labeling, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Timeline v2/Render Manifest v1 schemas, cross-runtime timing/source/easing fixtures, rational frame helpers, and canonical built-in easing are merged. Current slice adds canonical Bezier/spring curves and removes preview-local curve math. FrameState/AudioGraph and renderer adoption remain. |
| Phase 3 — Shared preview composition | Not started | Replace preview-local composition decisions with canonical FrameState/AudioGraph consumption. |
| Phase 4 — Shared Chromium render worker | Not started | Headless/browser renderer consumes the same canonical composition package behind a guarded rollout flag. |
| Phase 5 — Visual parity closure | Not started | Close geometry, text, crop/fit, effects, transitions, cursor, camera, color, and deterministic asset-loading parity. |
| Phase 6 — Audio parity closure | Not started | Shared AudioGraph/processed-stem architecture, rate/pitch policy, gain automation, fades, channel mapping, and decoded-delivery audio verification. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, and eventual retirement of the legacy composition path. |

## Architectural decisions

### Preview is the v1 compatibility target

For v1 timelines, the current editor preview defines compatibility semantics unless behavior is explicitly declared incorrect and migrated through a versioned contract change. The legacy FFmpeg compositor must not become the source of truth merely because it already exports media.

Confirmed example: the editor's `ease-in-out` is piecewise quadratic while the legacy Go fidelity evaluator uses smoothstep. Canonical v1-compatible easing therefore follows the editor. Legacy export callers migrate toward the canonical evaluator rather than changing preview semantics to preserve an approximation.

### Freeze legacy approximation growth

Do not add new approximation-only behavior to the legacy FFmpeg compositor during Phases 2–4 unless required for a correctness or security regression. New WYSIWYG semantics belong in the canonical contract and shared composition path.

### Renderer-independent core

The canonical core must be:

- pure and deterministic;
- free of media I/O, browser APIs, FFmpeg command construction, filesystem access, and network access;
- serializable for fixtures and diagnostics;
- usable by preview and export callers;
- explicit about unsupported authorable fields and fail closed on unknown semantics.

## Canonical timing, interpolation, and ordering rules

These are contract requirements:

1. `frameIndex` is the integer output-frame identity.
2. `frameTime` is rational `frameIndex / fps`; render decisions must not round-trip through milliseconds.
3. Authored millisecond starts map to frames with `floor(ms × fps / 1000)`.
4. Authored millisecond ends map to exclusive frames with `ceil(ms × fps / 1000)`.
5. Activity is half-open: `startFrame <= frameIndex < endFrame`.
6. Layer order is stable. The canonical tie-break tuple is `(track array index, z_index, clip array index)`; clip start time is not a z-order tie-breaker.
7. Source time is derived from canonical output time, trim, and playback rate in one evaluator.
8. Each keyframe segment uses the **later** keyframe's easing/curve, matching editor behavior.
9. Built-in v1-compatible easing is:
   - `linear`: `t`;
   - `ease-in`: `t²`;
   - `ease-out`: `1 - (1 - t)²`;
   - `ease-in-out`: piecewise quadratic (`2t²` before 0.5, symmetric quadratic after 0.5);
   - `step`: hold the previous value until segment completion.
10. Cubic Bezier evaluation uses x-to-parameter solving before evaluating y; the shared Go and TypeScript implementation must stay fixture-identical.
11. Spring evaluation is segment-local, resets velocity for each segment, normalizes the response at `t=1`, and may overshoot beyond 1.0. Only input progress is clamped.
12. Composition matrix order, anchor behavior, crop/fit behavior, camera projection, transition placement, effect ordering, text bounds, and color-space assumptions must be explicit in Timeline v2/FrameState rather than inferred independently by renderers.

## Phase 0 — Reproducible parity baseline

### Objective

Maintain deterministic, inspectable evidence that proves both regressions and convergence while the renderer architecture changes.

### Complete

- deterministic `parity-torture-v1` 20-second, 640×360, 30 fps fixture;
- 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor behavior, and audio cases;
- immutable snapshot/project seeding and staged source media;
- exact-dimension preview and render PNG capture;
- 206 uniquely indexed diagnostic images;
- visual metrics: channel tolerance/pass rate, MAE, RMSE, max delta, SSIM, changed bounds, structural regions, side-by-side images, and heat maps;
- independent browser/Web Audio reference PCM and snapshot diagnostic PCM;
- sample count, correlation, offset, sample-peak, EBU R128 loudness, and true-peak checks;
- decoded delivery frame count, frame rate, PTS start, duration, and time-base validation;
- hosted CI artifact retention with logs/toolchain/font metadata.

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

The visual gate intentionally remains red. The current artifact's aggregate visual scores are approximately `0.183` mean pixel pass rate and `0.091` mean SSIM; those values represent architectural mismatch, not acceptable tolerances.

### Initial feature-family review — 2026-08-18

The retained Phase 0 artifact was reviewed by frame label and representative side-by-side/heat-map output. Feature labels overlap, so these observations are diagnostic rather than a mutually exclusive statistical partition.

- **Transitions are a structural blocker:** all 11 transition-labeled sampled frames had zero channel-tolerance pixel pass rate. Representative fade output shows different composition/content placement rather than minor encoder noise.
- **Geometry/composition divergence is broad:** 2.5D/camera, keyframe, spring/Bezier, playback-rate, and boundary samples repeatedly show layer position, scale, crop/source selection, or composition differences.
- **Text/shape cases are not isolated font-noise failures:** multiple frames contain changed regions well beyond glyph edges, indicating upstream geometry/composition differences also affect these samples.
- **Audio delivery remains the strongest current parity area:** reference/export PCM and decoded delivery timing pass the current gates.
- **Threshold policy implication:** do not lower the provisional visual targets (`0.999` pixel pass rate and `0.995` SSIM in the retained report) to accommodate the legacy mismatch. Structural invariants such as dimensions, frame identity, active-layer identity/order, and exact boundary semantics should become zero-tolerance assertions as canonical FrameState lands.

### Remaining before Phase 0 sign-off

1. **Initial feature-family visual review: complete.** Repeat feature-family review after each major shared-render milestone to prove convergence.
2. Freeze the production visual threshold policy and zero-tolerance structural regions. Current provisional thresholds remain candidates, not final approval.
3. Define the unsupported-audio boundary for playback-rate pitch preservation, custom volume curves, and full-program processing until Phase 6 provides shared AudioGraph/processed-stem semantics.
4. Run the complete fixture on a second supported OS/FFmpeg environment and record deltas before final audio-tolerance approval.
5. Keep the baseline runnable throughout Phases 2–7.

### Exit gate

A reproducible parity report exists in CI; known mismatches are classified; threshold and unsupported-boundary policy is documented and approved; required cross-platform evidence is retained.

## Phase 1 — Immutable render submission

**Status: Complete.**

Delivered:

- timeline revision and canonical SHA-256 identity;
- stale-submission rejection with HTTP `409`;
- immutable render snapshots bound one-to-one to jobs;
- stable sorted asset manifests and hashes;
- snapshot-owned source bytes;
- FFprobe decode preflight and frozen media metadata;
- worker execution/recovery from persisted snapshots only;
- fail-closed missing/mutated source handling;
- snapshot/timeline/manifest/contract/renderer identity on outputs;
- explicit `legacy_mutable_source` labeling for historical jobs;
- frontend save-before-enqueue and hash-based “changed since render” state, including overlapping jobs;
- path-specific capability diagnostics and Strict Parity blocking behavior;
- service-owned render/generation worker lifecycle and clean shutdown.

### Exit gate

Met: a queued render is bound to one immutable timeline revision and one immutable set of source bytes and cannot silently change after later editing.

## Phase 2 — Canonical contract

**Status: In progress.**

### Objective

Centralize non-I/O render semantics so preview and export consume identical decisions instead of independently reimplementing timing, interpolation, ordering, geometry, and audio planning.

### Contract artifacts

- `video-renderer/contracts/timeline-v2.schema.json`
- `video-renderer/contracts/render-manifest-v1.schema.json`
- `video-renderer/test/fixtures/render-contract-v1.json`
- `backend/internal/video/rendercontract/`
- `frontend/src/video/renderContract.ts`

Language adapters/evaluators must prove conformance against shared fixtures until the composition runtime is fully shared.

### Merged foundation — PR #191

- strict JSON Schema draft 2020-12 Timeline v2 contract with fail-closed authorable semantic nodes and explicit extension points;
- Render Manifest v1 immutable snapshot/timeline/asset identity and output settings;
- cross-runtime fixture for built-in easing, half-open frame boundaries, long/high-fps mapping, and playback-rate source-time mapping;
- pure Go rational frame-time/start/end/count/activity/source-time helpers;
- matching frontend helpers;
- shared Go/Vitest fixture verification;
- frontend built-in keyframe easing delegated to the canonical helper;
- schema invariant tests;
- Node-only frontend fixture loader kept outside the browser `src` build tree.

### Current curve-evaluator slice

Branch: `feat/video-wysiwyg-phase2-curve-evaluator`

Implemented on the branch:

- canonical `MotionCurve` / `CanonicalMotionCurve` representation in Go and TypeScript;
- canonical cubic Bezier evaluation matching the editor's existing Newton + bounded refinement algorithm;
- canonical segment-local spring evaluation matching the editor's underdamped/overdamped/critically damped response semantics;
- shared fixture cases for Bezier, default spring, custom spring, and overshoot behavior;
- Go and Vitest cross-runtime assertions against those same expected values;
- frontend `keyframeUtils.ts` no longer owns Bezier/spring math and delegates built-in, Bezier, and spring interpolation to `frontend/src/video/renderContract.ts`;
- the existing curve-sample cache remains in the preview-facing keyframe utility, while semantics live in the renderer-independent contract.

### Remaining Phase 2 work

1. Define mechanically generated or mechanically verified Go and TypeScript Timeline v2 / Render Manifest v1 type projections; CI must fail on schema/type drift.
2. Add a v1-to-canonical adapter that preserves current editor-preview semantics, not legacy FFmpeg approximations.
3. Complete explicit Timeline v2 semantics for:
   - media fit and mask-source crop;
   - deterministic content bounds;
   - transition ownership/placement and peer relationship;
   - text box size, padding, wrapping, and vertical alignment;
   - working color space;
   - primitive composition behavior.
4. Implement pure evaluators:
   - `normalizeTimeline`;
   - canonical frame/range helpers;
   - `activeClips` with stable ordering;
   - keyframe/property evaluation using the canonical curve evaluator;
   - transform matrix including anchor and camera;
   - transition state;
   - ordered effect-stack state including animated effect amounts;
   - text/shape state;
   - cursor state;
   - `compileAudioGraph`.
5. Define serializable `FrameState` containing every visual decision required to paint one output frame.
6. Define serializable `AudioGraph` containing every audio source, timing, rate, channel, gain, fade, mute/solo, and processing decision.
7. Replace millisecond comparisons inside render paths with canonical frame/timebase helpers.
8. Add structured diagnostics with severity, code, timeline path, relevant IDs, and remediation.
9. Fail closed when an authorable field has no canonical semantics.
10. Migrate the legacy Go fidelity evaluator to canonical built-in/Bezier/spring/source-time/frame helpers while it remains in service; do not add new approximation semantics during that migration.

### Required Phase 2 fixtures

- exact start/end boundaries;
- overlapping clips and stable z-order ties;
- every built-in easing, cubic Bezier, and spring, including overshoot;
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

Preview and export callers consume identical `FrameState`/`AudioGraph` fixtures. No renderer owns separate curve, range, ordering, transform, or source-time math. CI detects schema/type/fixture drift.

## Phase 3 — Shared preview composition

### Objective

Make the editor program monitor a consumer of the canonical contract rather than a second rendering specification.

### Work

- Resolve active layers, transforms/camera, effects/transitions, text layout inputs, cursor state, and media source time through canonical `FrameState` evaluation.
- Schedule preview audio through canonical `AudioGraph` evaluation.
- Keep direct-manipulation UI state separate from persisted/canonical render state.
- Preserve parity automation and diagnostic capture.
- Add debug overlays for canonical bounds, matrices, active clip IDs, frame index, source time, transitions, and effects.

### Exit gate

The visible program monitor can be driven entirely from canonical evaluated state; legacy preview-local evaluation is removed or isolated behind an explicitly temporary adapter.

## Phase 4 — Shared Chromium render worker

### Objective

Render export frames with the same browser composition implementation used by the authoritative preview.

### Work

- deterministic composition entry point accepting Render Manifest v1 plus frame index;
- packaged render UI, fonts, and assets for desktop/server modes;
- managed headless Chromium worker with health checks, process ownership, cancellation, restart handling, concurrency limits, and bounded resources;
- deterministic frame output piped to FFmpeg for encoding/muxing where appropriate;
- guarded rollout flag and legacy fallback during migration.

### Exit gate

An immutable snapshot renders through the shared Chromium path on supported deployment targets and produces deterministic frame/audio inputs to the delivery encoder.

## Phase 5 — Visual parity closure

Close parity by feature family:

1. media decode/source timing;
2. contain/cover/fill and crop/mask geometry;
3. anchor, scale, rotation, 2.5D projection, camera, and z-order;
4. opacity/fades/blending;
5. text metrics, wrapping, fonts/fallback, line height, letter spacing, stroke, shadow, background, padding, and alignment;
6. primitive shapes/annotations;
7. transition ownership, clipping, and timing;
8. effect stack order and parameter animation;
9. cursor interpolation, highlights, and click effects;
10. sRGB working-space and browser/encoder color metadata policy.

### Exit gate

All named visual fixtures meet approved structural/perceptual thresholds on supported CI/platform matrices with zero unexplained changed regions.

## Phase 6 — Audio parity closure

### Objective

Make preview audition and export derive from one canonical audio plan.

### Work

- canonical 48 kHz stereo AudioGraph;
- deterministic trim/source-time/playback-rate policy;
- explicit pitch-preservation policy and supported rate range;
- channel mapping/downmix/upmix rules;
- clip/track mute and solo;
- gain keyframes using canonical curves;
- fades;
- program-level processing representation;
- shared offline processing or processed-stem contract when browser/runtime parity requires it;
- exact-duration sample count and decoded-delivery checks.

### Exit gate

Independent preview/export PCM derived from the same immutable snapshot meets approved timing, correlation, peak, loudness, and unsupported-feature policy across supported platforms.

## Phase 7 — Rollout and legacy retirement

### Work

- shadow-render representative jobs through legacy and canonical workers;
- collect parity/performance/failure telemetry without exposing user media outside local/server boundaries;
- staged opt-in, default-on, then legacy opt-out;
- explicit rollback switch;
- capability/documentation updates;
- remove legacy fidelity expansion only after canonical coverage and rollback criteria are satisfied.

### Exit gate

Canonical shared rendering is the production default, parity gates remain green, rollback has aged out safely, and the legacy composition path can be removed without reducing supported authoring fidelity.

## Validation matrix

Every Phase 2+ PR should run the smallest applicable focused tests plus repository gates. Renderer-affecting slices should validate at minimum:

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

Run the parity fixture/report workflow whenever a change can affect frame selection, visual state, audio state, asset identity, worker behavior, or delivery timing. Hosted CI is authoritative for platform/toolchain cases not reproducible in the current environment.

## Risk register

| Risk | Control |
|---|---|
| Schema exists but runtime silently diverges | Shared fixtures now; generated/mechanically verified types and drift CI before Phase 2 exit |
| Canonical v2 accidentally codifies FFmpeg approximations | v1 compatibility target is editor preview; intentional changes require versioned semantics |
| Millisecond rounding produces boundary flicker | Rational frame identity, floor-start/ceil-end, half-open activity |
| Browser and Go curve behavior drifts | Shared built-in/Bezier/spring fixtures verified by both runtimes |
| Spring overshoot is accidentally clamped | Contract clamps input progress only and fixtures include an overshoot sample |
| Headless Chromium increases packaging/resource cost | Managed worker, admission control, health checks, guarded rollout, FFmpeg retained for decode/encode/mux |
| Font or color output becomes platform-dependent | Bundled/declared font policy, toolchain metadata, explicit sRGB contract, multi-platform parity evidence |
| Audio rate/channel behavior differs by runtime | Explicit AudioGraph and unsupported-boundary policy before shared export is default |
| Legacy renderer receives new semantics during migration | Freeze approximation growth and move semantic authority to canonical contract |

## Validation log

### PR #191 — Phase 2 contract foundation

- First Quality Gate `32138535606`: **failed** on two branch-local integration issues.
  - Node-only fixture loader was initially under browser `src`, causing production TypeScript build errors.
  - one new Go test file required `gofmt`.
- Fixes moved the Node-only fixture loader to `frontend/test/renderContract.test.ts` and corrected Go formatting without changing contract semantics.
- Fresh Quality Gate `32139062167`: **passed**.
  - frontend lint: passed;
  - frontend unit tests: passed (25 files / 100 tests);
  - Video Studio performance evidence: passed;
  - frontend production build: passed;
  - backend formatting, vet, unit/integration tests, and race detector: passed;
  - Playwright smoke suite: passed;
  - video renderer parity-baseline capture: passed;
  - Windows desktop capture/sandbox/plugin lifecycle, macOS sandbox primitive, and Helm checks: passed.
- Security Scan `32139062304`: **passed**.
- Build and Publish Container Images `32139062182`: frontend image, backend image, and Helm chart validation **passed**.
- PR #191 merged to `main` as `aabbb31288277287673cbed8546c9eb3f38588e4`.

### Current curve-evaluator slice

- Branch: `feat/video-wysiwyg-phase2-curve-evaluator`.
- Branch is based directly on merged #191/main commit `aabbb31288277287673cbed8546c9eb3f38588e4`.
- Cross-runtime curve fixture and preview delegation are implemented.
- Hosted validation is pending until the PR is opened; merge is blocked until applicable Quality Gate/Security/container checks are green.

## Implementation log

### 2026-08-17 — Phase 0/1 foundation

- Implemented immutable timeline revision/hash render submission, source staging, snapshot identity, decode preflight, recovery, stale-request rejection, and Strict Parity diagnostics.
- Implemented deterministic 103-frame visual/audio/delivery parity baseline and hosted evidence workflow.
- Corrected worker shutdown ownership, hosted FFmpeg provisioning, screenshot dimensions, diagnostic naming, mono-to-stereo gain, eased volume automation, diagnostic PCM duration, Windows path containment, command-length issues, and delivery timing checks discovered by the torture fixture.
- PR #187 merged.

### 2026-08-18 — Phase 2 contract foundation

- Refreshed current `main` and reconciled the plan against merged Phase 0/1 work.
- Implemented Timeline v2 and Render Manifest v1 schemas.
- Implemented shared Go/TypeScript timing/source/easing primitives and fixtures.
- Routed built-in frontend keyframe easing through the canonical contract.
- Fixed first-run integration failures, obtained green Quality Gate/Security/container validation, and merged PR #191.
- Reviewed retained Phase 0 artifact `9303432653` by feature family and recorded the architectural mismatch findings above.

### 2026-08-18 — Phase 2 curve evaluator

- Created `feat/video-wysiwyg-phase2-curve-evaluator` from merged #191/main.
- Added canonical cubic Bezier and segment-local spring semantics in Go and TypeScript.
- Added shared expected-value fixtures, including spring overshoot behavior.
- Removed duplicate Bezier/spring semantic math from preview `keyframeUtils.ts`; preview now delegates all supported keyframe curve semantics to `frontend/src/video/renderContract.ts`.
- Next step: open the focused PR, run hosted validation, remediate any failures, merge when green, then begin schema/type drift enforcement plus the v1-to-canonical adapter.

## Next recommended implementation slice

After the curve-evaluator PR is validated and merged:

1. Add mechanically generated or mechanically verified Timeline v2 / Render Manifest v1 Go and TypeScript type projections plus CI drift enforcement.
2. Implement the v1-to-canonical adapter with compatibility fixtures against current editor-preview behavior.
3. Migrate the legacy Go fidelity evaluator to canonical built-in/Bezier/spring/source-time/frame helpers while it remains in service.
4. Add canonical `activeClips`, stable ordering, source-time, export-range, and long/high-fps fixtures; migrate diagnostic/render frame selection away from ad hoc millisecond comparisons.
5. Define the first serializable `FrameState` and `AudioGraph` structures before Phase 3 preview integration.
6. In parallel, finalize Phase 0 visual threshold policy, unsupported-audio boundary, and second-platform evidence.
