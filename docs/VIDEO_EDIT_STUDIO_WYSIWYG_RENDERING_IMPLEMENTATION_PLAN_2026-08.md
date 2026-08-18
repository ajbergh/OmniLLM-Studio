# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-18  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing  
**Primary goal:** The authoritative editor preview and the final decoded export must represent the same immutable timeline revision with the same frame, layer, timing, geometry, styling, effect, transition, camera, and audio decisions.

> This file is the durable execution tracker for WYSIWYG rendering work. Every implementation PR in this program must update the tracker, implementation log, validation state, and next recommended slice before merge.

## Current handoff

- PR #187, **Establish immutable video rendering and deterministic parity baseline**, merged to `main` on 2026-08-17 as `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0`.
- PR #191, **Start canonical Video Edit Studio render contract**, merged to `main` on 2026-08-18 as `aabbb31288277287673cbed8546c9eb3f38588e4` after fully green Quality Gate, Security Scan, and container validation.
- PR #193, **Canonicalize Video Edit Studio motion curves**, merged to `main` on 2026-08-18 as `3bed9faf8a868b3a125c25cb141769bfcd7861d2` after fully green Quality Gate, Security Scan, and container validation.
- PR #194, **Enforce Video Edit Studio render contract type drift**, merged to `main` on 2026-08-18 as `62e2180b5153be505fac650cd41b3a0e2d951783` after fully green Quality Gate, Security Scan, container, parity-baseline, and smoke validation.
- Phase 1 immutable render submission is complete.
- Phase 0 has reproducible hosted evidence and passing audio/delivery gates. Initial feature-family visual review is complete; production visual-threshold approval, unsupported-audio-boundary policy, and a second OS/FFmpeg evidence run remain open.
- The retained 103-frame visual baseline is a **known-mismatch baseline**. It proves the preview and legacy FFmpeg composition engines disagree; its mismatch distribution must never be used to relax production parity goals.
- Phase 2 is active. Current branch: `feat/video-wysiwyg-phase2-v1-adapter`.
- Current slice: translate normalized Timeline v1 into Timeline v2 with editor-preview-compatible defaults, shared Go/TypeScript fixtures, non-mutating behavior, and fail-closed diagnostics where v1 semantics are not explicit enough for the canonical contract.

## Implementation tracker

| Phase | Status | Progress note |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Initial feature-family review confirms structural divergence, especially transitions and geometry/composition. Threshold policy, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots, staged source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale-request rejection, legacy labeling, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | In progress | #191 merged schema/timing/source/built-in easing foundations; #193 merged canonical Bezier/spring evaluation; #194 merged mechanically checked Go/TypeScript schema projections. Current slice implements the v1-to-Timeline-v2 adapter. Canonical active-clip/range/property/transform evaluation, FrameState, AudioGraph, and renderer adoption remain. |
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

### Schema is the canonical structural authority

- `video-renderer/contracts/timeline-v2.schema.json` is the source of truth for authorable Timeline v2 structure.
- `video-renderer/contracts/render-manifest-v1.schema.json` is the source of truth for immutable render-manifest structure.
- Go and TypeScript projections are mechanically checked against these schemas.
- CI must fail when schema properties, required fields, enum/const values, or supported structural projections drift without a corresponding language projection update.
- Projection checks are not runtime validation. Runtime normalization/validation remains a separate Phase 2 responsibility.

### v1 adapter boundary

- Backend v1 validation/defaulting remains the normalization authority for persisted v1 timelines while Timeline v1 is current.
- The canonical adapter translates normalized v1 semantics; it must not reinterpret the document to match legacy FFmpeg approximations.
- Asset-backed visual clips currently materialize `media_fit: contain`, matching the existing editor preview default.
- Existing v1 transitions fail closed until Timeline v2 transition placement/ownership/peer semantics are explicitly implemented. Guessing whether a transition belongs to an incoming or outgoing edge would create a new source of parity drift.
- Unknown transform/crop fields fail closed with a path-addressed diagnostic instead of being silently ignored.
- Adapter calls must not mutate the caller's v1 document, including nested metadata.

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
10. Cubic Bezier evaluation solves x-to-parameter before evaluating y; shared Go and TypeScript implementations must remain fixture-identical.
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

The visual gate intentionally remains red. The retained artifact's aggregate visual scores are approximately `0.183` mean pixel pass rate and `0.091` mean SSIM; those values represent architectural mismatch, not acceptable tolerances.

### Initial feature-family review — 2026-08-18

- **Transitions are a structural blocker:** all 11 transition-labeled sampled frames had zero channel-tolerance pixel pass rate. Representative fade output shows different composition/content placement rather than minor encoder noise.
- **Geometry/composition divergence is broad:** 2.5D/camera, keyframe, spring/Bezier, playback-rate, and boundary samples repeatedly show layer position, scale, crop/source selection, or composition differences.
- **Text/shape cases are not isolated font-noise failures:** multiple frames contain changed regions well beyond glyph edges, indicating upstream geometry/composition differences also affect these samples.
- **Audio delivery remains the strongest current parity area:** reference/export PCM and decoded delivery timing pass the current gates.
- **Threshold implication:** do not lower provisional visual targets (`0.999` pixel pass rate and `0.995` SSIM in the retained report) to accommodate the legacy mismatch. Dimensions, frame identity, active-layer identity/order, and exact boundary semantics should become zero-tolerance assertions as canonical FrameState lands.

### Remaining before Phase 0 sign-off

1. Initial feature-family visual review: **complete**. Repeat after each major shared-render milestone.
2. Freeze production visual threshold policy and zero-tolerance structural regions. Current provisional thresholds remain candidates, not final approval.
3. Define unsupported-audio policy for playback-rate pitch preservation, custom volume curves, and full-program processing until Phase 6 provides shared AudioGraph/processed-stem semantics.
4. Run the full fixture on a second supported OS/FFmpeg environment and record deltas before final audio-tolerance approval.
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

### Canonical artifacts

- `video-renderer/contracts/timeline-v2.schema.json`
- `video-renderer/contracts/render-manifest-v1.schema.json`
- `video-renderer/test/fixtures/render-contract-v1.json`
- `video-renderer/test/fixtures/v1-canonical-adapter-v1.json` — current branch
- `backend/internal/video/rendercontract/`
- `frontend/src/video/renderContract.ts`
- `backend/internal/video/rendercontract/types.go`
- `frontend/src/video/renderContractTypes.ts`
- `backend/internal/video/canonical_adapter.go` — current branch
- `frontend/src/video/renderContractAdapter.ts` — current branch

### Merged foundation — PR #191

- strict JSON Schema draft 2020-12 Timeline v2 contract with fail-closed authorable semantic nodes and explicit extension points;
- Render Manifest v1 immutable snapshot/timeline/asset identity and output settings;
- cross-runtime fixture for built-in easing, half-open frame boundaries, long/high-fps mapping, and playback-rate source-time mapping;
- pure Go rational frame-time/start/end/count/activity/source-time helpers;
- matching frontend helpers;
- shared Go/Vitest fixture verification;
- frontend built-in keyframe easing delegated to the canonical helper;
- schema invariant tests.

### Merged curve evaluator — PR #193

- canonical `MotionCurve` / `CanonicalMotionCurve` representation in Go and TypeScript;
- canonical cubic Bezier evaluation matching the editor's Newton + bounded refinement algorithm;
- canonical segment-local spring evaluation matching underdamped, overdamped, and critically damped preview semantics;
- shared fixture cases for Bezier, default spring, custom spring, and overshoot behavior;
- Go and Vitest cross-runtime assertions against identical expected values;
- frontend `keyframeUtils.ts` delegates built-in, Bezier, and spring interpolation semantics to `frontend/src/video/renderContract.ts` while retaining its sample cache.

### Merged schema/type drift enforcement — PR #194

- serializable Go projections of Timeline v2 and Render Manifest v1;
- Go reflection tests comparing JSON tags, exact property sets, required/optional tags, primitive kinds, arrays, metadata maps, internal refs, and the external Timeline v2 manifest ref to source schemas;
- serializable TypeScript projections and schema enum/const unions;
- TypeScript compile-time exact-key and required-key projections;
- Vitest schema comparisons for property sets, required fields, enums, motion-curve union requirements, and fixed manifest values;
- schema-bound Go constants for Timeline v2, Render Manifest v1, contract version, sRGB, 48 kHz, and stereo;
- no schema-codegen dependency or CI-time generator download.

### Current v1-to-canonical adapter slice

Branch: `feat/video-wysiwyg-phase2-v1-adapter`

Implemented on the branch:

- Go adapter deep-copies and runs `ValidateTimelineDocument` before projection so persisted v1 normalization/defaults remain authoritative and the caller is not mutated.
- Go adapter projects the normalized v1 JSON into the mechanically checked Timeline v2 type, sets canonical version/sRGB constants, materializes explicit camera zero/default values, and sets preview-compatible `media_fit: contain` for asset-backed visual clips.
- TypeScript adapter mirrors editor-visible v1 defaults required for canonical projection, including 1x missing playback rate, canonical `trim_out_ms`, 30-second fallback duration for an otherwise empty/zero-duration timeline, cursor scale normalization, camera defaults, and `media_fit: contain`.
- Both adapters preserve transforms/crop, effects, keyframes/curves, animation blocks, markers, scenes, and metadata without sharing mutable nested metadata with the caller.
- Both adapters reject v1 transitions with `V1_TRANSITION_PLACEMENT_AMBIGUOUS` until Timeline v2 transition ownership/placement/peer semantics are defined.
- Both adapters reject unknown transform/crop fields with `V1_TRANSFORM_FIELD_UNSUPPORTED`; invalid transform values produce a path-addressed diagnostic rather than being silently ignored.
- `video-renderer/test/fixtures/v1-canonical-adapter-v1.json` is consumed by Go and Vitest tests and covers default visual projection, playback-rate/source-window normalization, crop/2.5D transform preservation, camera defaults, cursor defaults, curve preservation, nonmutation, ambiguous transitions, and unknown transform fields.
- Hosted validation is pending for this branch; this slice is not complete until applicable Quality Gate, Security Scan, container, smoke, and parity jobs are green and the PR is merged.

### Remaining Phase 2 work

1. Go and TypeScript schema/type drift enforcement: **complete — merged in #194**.
2. v1-to-canonical adapter: **implemented on current branch; complete when merged and green**.
3. Complete explicit Timeline v2 semantics for:
   - media fit and mask-source crop;
   - deterministic content bounds;
   - transition ownership/placement and peer relationship;
   - text box size, padding, wrapping, and vertical alignment;
   - working color space;
   - primitive composition behavior.
4. Implement pure evaluators:
   - `normalizeTimeline` runtime validation/defaulting for Timeline v2;
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
8. Continue structured diagnostics with severity, code, timeline path, relevant IDs, and remediation.
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
| JSON Schema changes without language updates | Go reflection and TypeScript compile-time/Vitest schema projection checks fail CI |
| Projection checks are mistaken for runtime validation | Explicitly separate structural drift guards from runtime normalization/validation |
| Canonical v2 accidentally codifies FFmpeg approximations | v1 compatibility target is editor preview; intentional changes require versioned semantics |
| v1 adapter silently guesses ambiguous semantics | Fail closed on transitions and unknown transform/crop fields until canonical semantics are explicit |
| v1 adapter drifts across Go and TypeScript | One versioned shared adapter fixture is asserted in both runtimes |
| Millisecond rounding produces boundary flicker | Rational frame identity, floor-start/ceil-end, half-open activity |
| Browser and Go curve behavior drifts | Shared built-in/Bezier/spring fixtures verified by both runtimes |
| Spring overshoot is accidentally clamped | Contract clamps input progress only and fixtures include an overshoot sample |
| Headless Chromium increases packaging/resource cost | Managed worker, admission control, health checks, guarded rollout, FFmpeg retained for decode/encode/mux |
| Font or color output becomes platform-dependent | Bundled/declared font policy, toolchain metadata, explicit sRGB contract, multi-platform parity evidence |
| Audio rate/channel behavior differs by runtime | Explicit AudioGraph and unsupported-boundary policy before shared export is default |
| Legacy renderer receives new semantics during migration | Freeze approximation growth and move semantic authority to canonical contract |

## Validation log

### PR #191 — Phase 2 contract foundation

- First Quality Gate `32138535606`: failed on two branch-local integration issues: a Node-only fixture loader under browser `src`, and one Go file requiring `gofmt`.
- Both issues were corrected without changing contract semantics.
- Fresh Quality Gate `32139062167`: **passed**, including frontend lint/unit/build, Video Studio performance evidence, backend format/vet/tests/race, Playwright smoke, parity-baseline capture, and platform/Helm checks.
- Security Scan `32139062304`: **passed**.
- Container workflow `32139062182`: **passed**.
- Merged to `main` as `aabbb31288277287673cbed8546c9eb3f38588e4`.

### PR #193 — Canonical motion curves

- First hosted run exposed one formatting-only failure in `contract_test.go`; exact `gofmt` output was applied.
- Quality Gate `32141657454`: **passed**, including frontend lint/unit/build, backend format/vet/tests/race, Playwright smoke, parity-baseline capture, Windows/macOS sandbox/capture checks, and Helm validation.
- Security Scan `32141657409`: **passed**.
- Container workflow `32141657443`: **passed**.
- No unresolved review threads before merge.
- Merged to `main` as `3bed9faf8a868b3a125c25cb141769bfcd7861d2`.

### PR #194 — Schema/type drift enforcement

- Hosted validation identified formatting-only Go issues while the contract projections themselves passed frontend unit/build checks; exact `gofmt` output was applied.
- A stalled Playwright runner was replaced by a substantive schema-bound constants hardening commit, causing GitHub concurrency cancellation and a clean rerun.
- Final Quality Gate `32146367815`: **passed**, including frontend lint/unit/build/performance, backend format/vet/tests/race, Playwright smoke, parity-baseline capture, Windows/macOS sandbox/capture checks, and Helm validation.
- Security Scan `32146367676`: **passed**.
- Container workflow `32146367435`: **passed** for frontend, backend, and Helm.
- No unresolved review threads before merge.
- Merged to `main` as `62e2180b5153be505fac650cd41b3a0e2d951783`.

### Current v1 adapter slice

- Branch `feat/video-wysiwyg-phase2-v1-adapter` was reset directly onto merged #194/main before adapter commits so the eventual PR contains no duplicated #194 history.
- Initial draft review caught and corrected two cross-runtime semantic differences before PR creation: frontend `trim_out_ms` now matches backend v1 source-window normalization, and zero-duration fallback now matches backend's 30-second minimum.
- A second pre-PR review caught Go `map[string]any` numeric representation differences: JSON-authored transforms arrive as `float64`, while backend default transforms contain integers. Compatibility validation accepts both representations and still rejects non-finite/non-numeric values.
- Shared Go/TypeScript adapter fixture and tests are implemented.
- Hosted validation is pending. Merge is blocked until applicable repository gates are green.

## Implementation log

### 2026-08-17 — Phase 0/1 foundation

- Implemented immutable timeline revision/hash render submission, source staging, snapshot identity, decode preflight, recovery, stale-request rejection, and Strict Parity diagnostics.
- Implemented deterministic 103-frame visual/audio/delivery parity baseline and hosted evidence workflow.
- PR #187 merged.

### 2026-08-18 — Phase 2 contract foundation

- Refreshed `main` and reconciled the plan against merged Phase 0/1 work.
- Implemented Timeline v2 and Render Manifest v1 schemas.
- Implemented shared Go/TypeScript timing/source/easing primitives and fixtures.
- Routed built-in frontend keyframe easing through the canonical contract.
- Fixed first-run integration failures, obtained green Quality Gate/Security/container validation, and merged PR #191.
- Reviewed retained Phase 0 artifact `9303432653` by feature family and recorded the structural mismatch findings above.

### 2026-08-18 — Phase 2 curve evaluator

- Implemented canonical cubic Bezier and segment-local spring semantics in Go and TypeScript.
- Added cross-runtime fixtures, including spring overshoot behavior.
- Removed duplicate Bezier/spring semantic math from preview `keyframeUtils.ts`.
- Fixed one formatting-only first-run failure; Quality Gate, Security Scan, containers, smoke, and parity evidence passed.
- Merged PR #193 as `3bed9faf8a868b3a125c25cb141769bfcd7861d2`.

### 2026-08-18 — Phase 2 schema/type drift enforcement

- Added Go Timeline v2 / Render Manifest v1 serializable projections and reflection-based schema drift tests.
- Added TypeScript serializable projections, compile-time exact-key/required-key projections, and Vitest schema comparisons.
- Added schema-bound render contract/version/color/audio constants.
- Kept implementation dependency-free so repository CI provides enforcement without external generator/toolchain drift.
- Final Quality Gate, Security Scan, containers, smoke, and parity evidence passed.
- Merged PR #194 as `62e2180b5153be505fac650cd41b3a0e2d951783`.

### 2026-08-18 — Phase 2 v1 canonical adapter

- Created a clean adapter branch from merged #194/main.
- Implemented Go and TypeScript v1-to-Timeline-v2 adapters around editor/backend v1 semantics rather than legacy FFmpeg approximations.
- Added a shared fixture for visual defaults, playback-rate/source-window normalization, crop/2.5D transforms, camera/cursor defaults, curve preservation, nonmutation, and fail-closed transition/unknown-transform cases.
- Corrected pre-PR cross-runtime differences in `trim_out_ms`, zero-duration fallback, and Go numeric transform representation.
- Next step: run hosted validation, remediate any real failures, merge when green, then move directly to canonical active-clip/range ordering evaluation.

## Next recommended implementation slice

After the v1 adapter PR is validated and merged:

1. Implement canonical `activeClips` and range evaluation with half-open frame activity and stable `(track index, z_index, clip index)` ordering; add overlapping/tie/boundary/long-120-fps shared fixtures in Go and TypeScript.
2. Add Timeline v2 runtime `normalizeTimeline` validation/defaulting for the fields required by those evaluators, keeping structural schema checks separate from runtime semantic validation.
3. Migrate legacy diagnostic/render frame-selection and source-time callers to the canonical frame/range/source helpers while the legacy renderer remains in service; do not add approximation semantics.
4. Implement canonical keyframe/property evaluation on top of the merged curve evaluator, then define the first serializable `FrameState` structure.
5. Begin `AudioGraph` structure/compilation once visual activity/order/source-time identity is canonical.
6. In parallel, finalize Phase 0 visual threshold policy, unsupported-audio boundary, and second-platform evidence.
