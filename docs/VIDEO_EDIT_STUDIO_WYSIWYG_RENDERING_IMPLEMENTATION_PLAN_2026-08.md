# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-24  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG feature PR: **#264 — Consume canonical preview transform state** — squash merge `357ae16301fc0b7d6c0f0d65e99c0277bac77adb` (2026-08-24).

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active.** The renderer-independent visual contract (`visual-frame-state-v1` and its subcontracts) and audio contract (`audio-graph-v1`) define semantics; Phase 3 is migrating actual program-monitor consumers onto those canonical decisions in small, reversible slices.

Current implementation PR: **#265 — Consume canonical preview view transform** on branch `feat/video-wysiwyg-phase3-canonical-view-transform`, rebuilt directly from current `main` `357ae16301fc0b7d6c0f0d65e99c0277bac77adb` after #264 merged.

#265 advances deterministic camera-relative transform consumption without claiming full perspective parity:

- `resolvePreviewFrameViewTransform` consumes `CanonicalFrameLayerState.view_transform` for deterministic canonical x/y/z and XYZ rotation rather than subtracting locally evaluated camera state a second time;
- free-running playback and canonical-unavailable compatibility fallback retain the established local camera subtraction behavior;
- an active `liveTransform` gesture deliberately bypasses canonical view state so direct manipulation remains responsive until commit;
- exact zero camera-relative values are preserved;
- focused coverage includes a real Timeline v1 scene-camera projection, proving canonical view state owns the deterministic path even if the caller supplies a deliberately wrong local camera;
- the current single parent CSS `perspective` remains unchanged because canonical per-layer `perspective_projection` can carry clip-specific perspective overrides and requires a separate explicit painter/projection mapping.

Media geometry/bounds, full perspective projection, transition/effect paint, text/shape/cursor paint, normal-playback canonicalization, and AudioGraph scheduling are **not changed by #265**. They remain separate Phase 3 slices.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production structural-region policy and second-platform evidence remain. `audio-graph-v1` defines the unsupported-audio semantic boundary. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261 merged canonical visual activity/projection consumption; #262 retained parity evidence for projection identity/availability; #263 routes deterministic visual-media source time through FrameState; #264 consumes deterministic transform/opacity state; #265 consumes deterministic camera-relative `view_transform` while preserving fallback/live-interaction behavior. Full perspective/geometry/painter/audio consumption remains. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close decoded visual thresholds for media, transforms, text metrics/fonts, shapes, transitions, effects, cursor, camera, color, and deterministic asset loading. |
| Phase 6 — Audio parity closure | Not started | Make preview/Chromium/export obey AudioGraph exactly, including pitch, gain/fades, channels, program processing, processed stems, and decoded delivery. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, capability/docs updates, and eventual removal of legacy composition semantics. |

## Canonical architecture rules

### Immutable frame identity

1. `frameIndex` is integer output-frame identity.
2. Frame time is rational `frameIndex / fps`; deterministic rendering does not round-trip through integer milliseconds.
3. Authored starts map with `floor(ms × fps / 1000)`; authored ends are exclusive and map with `ceil(ms × fps / 1000)`.
4. Active visual ordering is stable `(track array index, z_index, clip array index)`.
5. Source time comes from one canonical evaluator using frame identity, clip start, trim-in, and playback rate.
6. Keyframe segments use the later keyframe's easing/curve.

### Renderer-independent canonical core

Canonical evaluators are pure, deterministic, serializable, free of browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed whenever an authorable value does not have explicit canonical semantics.

The legacy FFmpeg compositor is implementation evidence, not semantic authority. The existing editor preview is a compatibility target where behavior is implemented, but Phase 3 consumers must stop re-evaluating semantics that the canonical contract already owns.

### Media geometry and source provenance

`media-geometry-v1` is authoritative for asset geometry:

- source aspect ratio requires explicit `content_bounds` or immutable `source-provenance-v1` media-probe state;
- source dimensions are never guessed from the output canvas;
- source crop precedes fit; output crop follows fit;
- `contain`, `cover`, `fill`, and `none` have canonical meaning;
- FrameState carries evaluated painted bounds and source provenance;
- missing/invalid provenance remains explicit unresolved state rather than a canvas-sized fallback.

### Perspective and stacking

Track/z-index order remains authoritative for stacking; spatial `z` affects projection. `perspective-projection-v1` serializes projection independently from camera-relative model transforms and preserves the preview-compatible 1200-canvas-pixel no-camera distance.

### Transition state and paint

`transition-state-v1` owns placement, peer/owner roles, windows, progress, and overlap requirements. `transition-paint-v1` owns all currently authorable paint families: fade, crossfade, dip-to-black, slide, wipe, and zoom. Pair transitions operate on isolated surfaces and must not be reinterpreted as independent layer opacity.

### Effects, text, shape, and cursor

- `effect-state-v1` preserves enabled authored order, scope, normalized parameters, and exact-frame automation; unsupported metadata fails closed.
- `text-state-v1` serializes text/style intent once. Packaged static font identity is manifest-backed via `font-resource-provenance-v1`; intrinsic glyph measurement remains a Phase 3–5 consumer problem and is never guessed.
- `shape-state-v1` covers all currently authorable annotation kinds and supplies canonical dimensions/style defaults/bounds.
- `cursor-state-v1` owns exact rational sampling, visibility, scale, highlight/click-ring state, and strict `<300ms` click proximity; undefined smoothing fails closed.

### AudioGraph v1

`audio-graph-v1` is the renderer-independent audio contract merged in #260:

1. Output is exactly 48,000 Hz stereo.
2. Timeline/range boundaries become deterministic integer sample boundaries.
3. Audio-capable clips retain stable node identity even when suppressed.
4. Track mute, clip mute, and solo selection have explicit precedence/reason identity.
5. Playback rate explicitly **preserves pitch**.
6. Mono duplicates to stereo; stereo passes through; unsupported layouts fail closed until an explicit matrix exists.
7. Base gain and volume automation are finite, deterministic, and authored-order stable.
8. Volume automation is the property envelope (`automation-overrides-base`), avoiding double application.
9. Fades are linear and overlapping fade envelopes combine with `minimum`.
10. Summation is `sum-no-normalize`.
11. `render_audio_processing` is one **post-mix** program operation represented by a processed-stem boundary, not FFmpeg filter syntax.
12. Unknown processing fields, unsupported channel modes/layouts, missing probes, and invalid numeric domains fail closed.
13. Phase 6 makes Web Audio/Chromium/export obey the graph and validates decoded delivery.

## Safe stacked-branch normalization

A stacked PR is rebuilt from the **actual current `main` tree** after its parent merges:

1. Read current `main` commit/tree.
2. Identify only the intended child delta.
3. Rebuild the child directly on current `main`.
4. Verify `compare main...branch` contains only intended paths.
5. Update this tracker on the clean branch.
6. Validate the exact final head before merge.

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can look current while the tree silently reverts unrelated work. #261 followed this rule after #260; #262 followed it after #261; #264 was rebuilt from #263's actual squash result after its original stack diverged by one final parent-tracker commit; #265 was rebuilt directly from #264's squash result before production Canvas/tracker integration.

## Phase 0 — Reproducible parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains a known-mismatch diagnostic baseline, not a production threshold.

Current parity metric defaults already define production-like numeric tolerances (`channel <= 2`, pixel pass rate `>= 0.999`, SSIM `>= 0.995`) and support exact structural regions, but the CLI does not currently attach structural regions to loaded frame pairs. Inspection of retained decoded H.264 evidence on 2026-08-24 also showed that literal RGB equality is not a sound general structural policy: codec-affected decoded pixels can differ even where frame identity and tolerance metrics are materially aligned. Structural sign-off therefore needs canonical geometry/identity assertions for truly zero-tolerance structure plus tolerance-aware decoded-image regions where codec output is involved, rather than arbitrary raw RGB-exact rectangles.

Remaining Phase 0 sign-off:

1. Freeze and wire a production structural policy that separates zero-tolerance canonical structure/identity from codec-aware decoded-region thresholds, and retain the evidence in CI.
2. Use `audio-graph-v1` as the explicit audio boundary until Phase 6 consumers land; do not claim pitch/fade/program-processing parity merely because a path emits audio.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable throughout Phases 3–7.

## Phase 1 — Immutable submission

**Complete.** A queued render is bound to one immutable timeline revision/hash and immutable source bytes. Snapshot identity, staged source bytes, decode preflight, recovery, stale-request rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are production foundations for later parity work.

## Phase 2 — Canonical contract

**Complete as of merged #260 on 2026-08-24.**

### Merged foundations

| PR | Capability | Merge SHA |
|---|---|---|
| #187 | Immutable render submission and deterministic parity baseline | `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0` |
| #191 | Timeline v2 / Render Manifest v1; frame/source/easing primitives | `aabbb31288277287673cbed8546c9eb3f38588e4` |
| #193 | Canonical cubic-Bezier and spring evaluation | `3bed9faf8a868b3a125c25cb141769bfcd7861d2` |
| #194 | Mechanically checked Go/TypeScript schema projections/constants | `62e2180b5153be505fac650cd41b3a0e2d951783` |
| #195 | Timeline v1 → canonical Timeline v2 adapter | `b5f76aa6328240a6b516d768756c34f68e6fdedb` |
| #196 | Canonical frame activity, range, source time, ordering | `42dd64cda9feb75a637b622bf33ac1350a4febd9` |
| #198 | Evaluator-scoped Timeline v2 runtime normalization | `67982f4fdd80062c9439c528362f75382e5c3268` |
| #199 | Canonical preview/index ordering adoption | `19a1a7b635afd33954bc56ed6023845f2c9e3fd1` |
| #202 | Deterministic frame-addressed preview/source selection | `02a1bbf4ec2b640a57d59fdd67f7906ae03eaa91` |
| #204 | Canonical backend diagnostic/parity frame callers | `73fa7d78b5018eb19b88abc34790fd19e95a5a98` |
| #205 | Canonical numeric property/keyframe evaluation | `8a93f9ff90eeda92c944085715856907747584f1` |
| #206 | Exact-frame `visual-frame-state-v1` foundation | `c37ba2ed8132133cc913531946d462c3b7b38911` |
| #208 | Permanent Go↔TypeScript FrameState parity diagnostics | `52683e4d25b22f70e5c6c3b4a8cf3417240be4bc` |
| #209 | Canonical media fit/crop/source-bounds geometry | `6365b3dcc13fac0726e7407735c2a6b5664e0d1a` |
| #212 | Media geometry consumption in FrameState | `ae29d57e2e7d4e94e298bb155501583f4577e1ed` |
| #218 | Canonical perspective projection + FrameState consumption | `0bf0ffc897589d71d2f62d75e18b63319bd59fae` |
| #220 | Canonical transition placement/peer state | `1bbbabed5185cbe44640426aad1ab141b59d50cc` |
| #222 | Transition-state consumption and frame-scoped paint debt | `73d7851cff9e0d7efce711f022659db60cc39dd2` |
| #224 | Canonical fade/crossfade/dip-to-black paint | `86bd0af3924bb10d6c49c411176f72bcdc07b453` |
| #225 | Fade-family paint consumption in FrameState | `a6f9145f92e2342dfa70144a4058bf10f64625da` |
| #227 | Canonical slide transition paint + consumption | `28639ec4fee09635de39764b33021bb6d9aa418c` |
| #228 | Canonical wipe transition paint + consumption | `3007be763dacde820b8b494f62b6a82bb9af5324` |
| #229 | Canonical zoom transition paint + consumption | `a8e2a0964e1e49232be08bf2fe7091ce3d9403e6` |
| #237 | Canonical effect stack state + FrameState consumption | `22e73cc291a4f8723a99ad123c963aedf0fd0d8a` |
| #241 | Canonical text renderer state + FrameState consumption | `9b072685b689cdb74e0a5590a26478f6a3ef12b4` |
| #242 | Canonical shape renderer state definition | `1a25ac0fef217731197169b229bde19aff158c8b` |
| #243 | Canonical shape state consumption in FrameState | `111fba5fd7ea73740aee1d92fc1038ee72fda30b` |
| #245 | Canonical cursor renderer state definition | `c428d81f25ce1d85faa6655fb5772430a8fe6b22` |
| #247 | Canonical cursor consumption in FrameState | `66530a07e3a5585546d978794e24198083bbeaa2` |
| #251 | Immutable source provenance + FrameState consumption | `d49808f1ea7fc23f658e95f52fdbe404bf0be92a` |
| #252 | Canonical font-resource provenance | `a8a1d649a209d0013390f0f838b5f32613a2ce02` |
| #253 | Authored text-face binding to packaged resources | `c7db41ade06c6729a77c6d0464ae30aa8a9fa4a6` |
| #254 | Font-resource snapshot packaging and upload/storage | `b99991c5121c2122bf0cd0f5ef0f7ab8e3514845` |
| #255 | Static-face-only variable-font policy | `853b59d07280f7441258586632aa6a43f618bff5` |
| **#260** | **Canonical AudioGraph v1** | **`9acc544eac6cc63a14a4e0f22ee52cb07688e010`** |

Supporting unblocks: #201 repaired the reachable PDF dependency; #219/#226 improved CI setup resilience; #239 aligned sandbox-worker SQLite test behavior; #246 ensured `frontend/test/` canonical suites run in the normal unit/Quality Gate.

### #260 validation and merge evidence

- Initial head `a630f0fe67ed4d485182622140e24115e94bb925` exposed only connector-authored Go formatting in `audio_graph_test.go`; frontend lint/unit/build was already green.
- Formatting was corrected; the final exact PR head was `43a0d1c1a5044937f13be30e0bad58bb7640d41c`.
- Exact-head **Quality Gate #1491 passed**, including backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright smoke, Windows desktop contract, and deterministic renderer parity.
- Exact-head **Security Scan #1497 passed**, including dependency audits and Go/JavaScript-TypeScript CodeQL.
- Linux workspace/quota, browser-egress, sandbox-worker, macOS runtime/extension/adversarial, and Windows/macOS confinement assurances all passed.
- No review submissions, inline review threads, or PR comments required remediation.
- `compare main...branch` contained only intended AudioGraph/test/tracker paths immediately before merge.
- #260 squash-merged as `9acc544eac6cc63a14a4e0f22ee52cb07688e010`.

### Phase 2 exit gate — satisfied

The renderer-independent semantic contract now covers the currently authorable visual and audio state required by this plan. Phase 3/4/6 consumers must use the canonical contracts rather than duplicate curve, frame, source-time, ordering, geometry, projection, transition, effect, text, shape, cursor, or audio semantic math. New authorable fields without explicit semantics remain fail closed.

## Phase 3 — Shared preview composition

**In progress. #261–#264 merged; #265 is the current camera-relative view-transform consumer slice.**

### Merged #261 — canonical preview projection and deterministic activity consumption

`preview-composition-frame-v1` is a lightweight runtime projection, not a new semantic engine. It:

- evaluates the existing strict `visual-frame-state-v1` diagnostic path;
- binds canonical layer state back to exact Timeline v1 track/clip/asset objects required by the current editor UI;
- verifies identity/position mapping;
- surfaces structured unavailable state instead of inventing v1 transition placement;
- allows the current DOM/CSS painter to migrate incrementally without re-deriving canonical semantics.

`queryActiveClipsAtFrame` consumes that projection for deterministic visual activity when available and carries `canonicalState` on matched entries. The existing indexed point query still serves free-running playback. Audio entries remain separate until the AudioGraph consumer slice.

Focused coverage proves canonical z/order, object/asset identity binding, fail-closed transition ambiguity, canonical source-time/transform propagation, high-FPS frame activity, and explicit compatibility fallback without falsely attaching canonical state.

### #261 validation and merge evidence

- Final exact head: `c6417af5682adb54ccaaa0b340089e9b18d162cb`.
- Exact-head **Quality Gate #1501 passed**: backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Windows desktop, Helm, Playwright smoke, and deterministic Video renderer parity all completed successfully.
- Exact-head **Security Scan #1507 passed**, including dependency vulnerability audit and both Go and JavaScript/TypeScript CodeQL.
- Linux workspace/quota, browser-egress, macOS runtime/extension/adversarial, Windows confinement, macOS confinement, and plugin lifecycle assurances passed.
- There were no review submissions, inline review threads, or PR comments requiring remediation.
- Final `compare main...branch` showed exactly the four intended Phase 3 code/test files plus this tracker; the branch was ahead of and not behind `main`.
- #261 was marked ready and squash-merged with expected-head protection as `1fa4a2c9fb0ba02b00a194374dc363fe5f796199`.

### Merged #262 — retained preview-composition parity diagnostics

The browser diagnostics script evaluates FrameState and preview composition together for every parity sample. It fails immediately on availability or ordered clip-identity drift and serializes preview-composition evidence into the retained diagnostic artifact. Both real saved-timeline fail-closed behavior and the transition-free positive control remain intact.

Exact head `64a9a034b6e77334980c87bfcbc73416e1fd927e` passed Quality Gate #1506, Security #1512, Playwright, deterministic renderer parity, CodeQL, and the platform/sandbox assurances, then squash-merged as `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec`.

### Merged #263 — canonical deterministic visual-media source time

`sourceTimeForPreviewMediaMs` consumes `canonicalState.source_time_ms` for explicit frame-addressed visual media, while free-running playback remains timeline-time based and deterministic compatibility fallback retains rational frame evaluation. Managed audio remains on the existing path until AudioGraph runtime adoption.

Exact head `5e22a663506bcc8321534329693bcd1b185cf444` passed Quality Gate #1517, Security #1523, Playwright, deterministic renderer parity, backend race, CodeQL, and all platform/sandbox assurances. Review submissions, inline threads, and PR comments were empty; final compare contained only the four intended source-time/test/canvas/tracker paths. #263 squash-merged with expected-head protection as `77b49e096e165ad579d7eb5daed81763c023203c`.

### Merged #264 — canonical deterministic transform/opacity consumption

`resolvePreviewFrameTransform` centralizes the transform values fed into the current preview painter. Deterministic canonical entries consume FrameState transform/opacity directly; free-running and fallback entries retain local property evaluation; live direct-manipulation intentionally bypasses canonical state until gesture commit. Canonical opacity owns clip fades exactly once.

Exact head `b84b4b82e72393b27fcf6bf579fb9a8579ea9f5c` passed Quality Gate #1523, Security Scan #1529, backend race, frontend lint/unit/performance/build, Playwright smoke, deterministic renderer parity, CodeQL, desktop/Helm, and all platform/sandbox assurances. Final compare was ahead 4 / behind 0 with exactly the resolver/test/Canvas/tracker paths; review submissions, inline threads, and PR comments were empty. #264 squash-merged with expected-head protection as `357ae16301fc0b7d6c0f0d65e99c0277bac77adb`.

A focused compatibility fix in #264 preserves exact canonical zero axis scale despite the legacy painter's truthy axis fallback.

### Current #265 — canonical deterministic camera-relative view-transform consumption

`resolvePreviewFrameViewTransform` centralizes the camera-relative x/y/z and XYZ rotation values consumed by the current CSS painter. Deterministic canonical entries use `CanonicalFrameLayerState.view_transform` directly; free-running and compatibility-fallback entries retain local camera subtraction; live direct manipulation bypasses canonical view state until commit.

#265 was first opened as a helper-only stack while #264's final renderer-parity gate was running. After #264 merged, #265 was rebuilt directly from `357ae16301fc0b7d6c0f0d65e99c0277bac77adb` using only its resolver/test blobs, retargeted to `main`, and verified ahead 1 / behind 0 before production wiring. The Canvas integration adds one resolver import, one camera-relative resolution call, and replaces the old inline local camera-subtraction transform string. Focused coverage includes exact zeros, fallback/live behavior, and a real Timeline v1 scene-camera projection.

This slice intentionally does **not** consume canonical `perspective_projection`: per-layer perspective distance can differ due to clip overrides, while the current DOM painter exposes one parent CSS `perspective`. Full projection adoption needs an explicit representation decision rather than a silent approximation.

### Remaining Phase 3 work

Recommended order after #265:

1. Consume canonical `perspective_projection` in the program monitor with an explicit mapping that can represent per-layer clip perspective overrides; do not force it through one parent CSS perspective when semantics differ.
2. Consume canonical media geometry/bounds and fit/crop placement, removing remaining source/canvas geometry assumptions while preserving crop-edit interaction as a temporary overlay.
3. Move transitions/effects/text/shape/cursor painter inputs to their already-evaluated FrameState fields, removing remaining local `sampleCursor`, fade/property, and registry re-evaluation where canonical state exists.
4. Extend the same frame-addressed composition model to normal playback without sacrificing interactive responsiveness; keep unsupported legacy v1 semantics visibly on the compatibility path until their persisted representation is unambiguous.
5. Add a preview diagnostic overlay/event payload for frame identity, canonical/fallback mode, active clip IDs, source times, transforms/bounds/projection, transition/effect IDs, and eventually AudioGraph identity.
6. Introduce the AudioGraph preview consumer: selection, mute/solo, playback-rate/pitch policy, gain/fades, and channel policy come from `audio-graph-v1`; program processing/processed-stem audition may remain Phase 6 where offline DSP is required.

Phase 3 exit: normal program-monitor playback no longer independently decides frame activity, source time, layer order, evaluated transforms/geometry/projection, transition/effect/text/shape/cursor semantics, or audio selection/gain/fade semantics.

## Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets deterministically, manage browser health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux behind a guarded rollout.

## Phase 5 — Visual parity closure

Close decoded output parity for media timing/fit/crop, transforms/anchors/2.5D/camera/z-order, opacity/blending, text metrics/fonts, shapes, transitions, effects, cursor, color space, and deterministic asset loading. Freeze production thresholds only from retained, reproducible evidence.

## Phase 6 — Audio parity closure

Make preview/Chromium/export consumers obey `audio-graph-v1` exactly:

- source time and playback rate with pitch preservation;
- canonical mono/stereo mapping and future explicit matrices for additional layouts;
- mute/solo, gain automation, fade overlap, and non-normalizing summation;
- one post-mix program-processing stage;
- processed stems when deterministic real-time reproduction is not possible;
- exact output/range sample counts and decoded-delivery verification.

Do not bless current mismatches: legacy Web Audio rate changes can pitch-shift while the contract requires preservation; legacy FFmpeg has historically applied some processing per input while the contract defines post-mix processing; overlapping legacy FFmpeg fades can multiply while the contract defines minimum-envelope behavior.

## Phase 7 — Rollout and legacy retirement

Shadow-render, collect parity/performance/failure telemetry, stage opt-in → default-on → legacy opt-out, preserve rollback, update capabilities/docs, then retire legacy composition only when canonical coverage and rollback criteria are satisfied.

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

Hosted CI is authoritative for platform/toolchain cases that cannot be reproduced in the current execution environment. Setup-only stalls and runner-capacity queues are recorded explicitly and are never represented as passing.

Before every merge:

1. Verify the PR's exact final head.
2. `compare main...branch` and inspect every changed path.
3. Resolve all review threads or document why a non-actionable thread is dismissed.
4. Record concrete validation evidence in this tracker/PR.
5. Never call an unexecuted or setup-only job green.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest checks; canonical `frontend/test/` suites run in CI via #246. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState/AudioGraph diagnostics. |
| Preview projection drifts from FrameState identity/availability | #262 parity diagnostics evaluate both contracts on every sampled frame and fail on availability/order drift. |
| Legacy FFmpeg approximations become contract | Canonical state is explicit; legacy renderers are evidence/consumers only. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed; #261 preserves this boundary. |
| Canonical preview projection binds wrong editor object | #261 verifies track/clip positional and ID identity before exposing canonical state. |
| Frame-addressed preview silently mixes canonical and fallback semantics | #261 attaches `canonicalState` only when the strict projection succeeds; fallback entries remain explicitly without it. |
| Visual media recomputes source time after canonical FrameState evaluation | #263 consumes `canonicalState.source_time_ms` only for frame-addressed canonical visual entries; free-running/fallback paths retain `sourceTiming.ts`, and audio waits for AudioGraph. |
| Preview re-evaluates deterministic transform/opacity after FrameState | #264 routes deterministic canonical entries through `resolvePreviewFrameTransform`; canonical opacity owns clip fades once, while free-running/fallback/live-gesture paths remain explicit. |
| Exact zero axis scale is lost by legacy truthy CSS fallback | #264 carries compatibility `scale: 0` whenever either canonical axis is exactly zero; the painter should still migrate from `||` to nullish/exact axis selection in a later cleanup. |
| Deterministic preview subtracts camera twice or re-evaluates camera-relative transforms | #265 consumes canonical `view_transform` for deterministic entries; fallback/free-running/live-gesture paths alone use local camera subtraction. |
| Per-layer perspective overrides are collapsed into one parent perspective | #265 explicitly leaves `perspective_projection` for a separate slice that can represent per-layer distance rather than approximating it. |
| Millisecond rounding creates frame/source drift | Rational frame/source helpers and integer AudioGraph sample-boundary rules. |
| Source aspect ratio is guessed | Explicit bounds or immutable source provenance only. |
| Perspective differs between consumers | One canonical projection contract in FrameState. |
| Pair transitions become independent alpha layers | Pair paint operates on isolated surfaces with canonical weights/transforms. |
| Effect/text/shape/cursor defaults drift | Versioned evaluated state is authoritative; unsupported metadata fails closed. |
| Browser/system font fallback changes metrics | Packaged static-face provenance; deterministic intrinsic metric ownership remains explicit Phase 3–5 work. |
| Audio pitch/fade/processing location diverges | AudioGraph explicitly owns pitch preservation, minimum fade overlap, and post-mix processing. |
| Unsupported channels are silently remixed | v1 accepts mono→stereo or stereo passthrough only; other layouts fail closed. |
| Structural parity is claimed from codec-noisy raw RGB equality | Phase 0 separates canonical zero-tolerance identity/geometry from tolerance-aware decoded regions; global metrics or arbitrary exact rectangles alone do not satisfy the structural gate. |
| Stacked branch carries stale tree | Rebuild from actual `main` and inspect `compare`; #261/#262/#264/#265 were normalized this way. |
| CI setup/runner saturation hides code state | Distinguish setup/queue from executed checks. |
| Browser worker resource cost | Admission control, health checks, cancellation, guarded rollout, FFmpeg retained for media I/O. |

## Implementation log

### 2026-08-17 to 2026-08-18

- #187 established immutable submission and deterministic parity evidence.
- #191–#206 advanced the canonical contract through schema/timing, adapters, ordering, curves, exact-frame property evaluation, and visual FrameState.
- #201 repaired the reachable PDF dependency vulnerability.
- #208 added permanent cross-runtime FrameState diagnostics and corrected its CodeQL finding before merge.

### 2026-08-19

- #209/#212 canonicalized media geometry, source bounds, and FrameState consumption.
- #218 canonicalized perspective projection.
- #219/#226 improved CI dependency/Playwright reliability.
- #220–#229 completed transition placement and all currently authorable transition paint families.
- #237 defined and consumed `effect-state-v1`.
- #241 defined and consumed `text-state-v1`.

### 2026-08-20 to 2026-08-21

- #242/#243 defined and consumed `shape-state-v1`.
- #246 fixed canonical Vitest discovery and corrected newly exposed stale assertions.
- #245/#247 defined and consumed `cursor-state-v1`.
- #251 added immutable source provenance and canonical anchor/geometry consumption.
- #252 defined font-resource provenance.
- #253 bound authored text to packaged font faces.
- #254 added font upload/storage and immutable snapshot packaging; hosted parity exposed and drove a fix for a non-text-clip nil dereference before merge.
- #255 enforced static-face-only font semantics.

### 2026-08-24

- Refreshed against actual `main` `dc51d7ddaf8060304fbe63153796e4ebafe48a20`; the previous handoff's claimed AudioGraph branch did not exist.
- Created `feat/video-wysiwyg-phase2-audio-graph`, opened #260, implemented mirrored Go/TypeScript `audio-graph-v1`, and added one shared cross-runtime fixture.
- Initial #260 head exposed only Go formatting; corrected final head passed complete Quality/Security/platform validation and #260 squash-merged as `9acc544eac6cc63a14a4e0f22ee52cb07688e010`, completing Phase 2.
- Opened draft #261 while #260 parity was still executing, then safely rebuilt #261 directly from merged `main` and retargeted it to `main`.
- #261 introduced `preview-composition-frame-v1`, strict identity binding, and the first real canonical preview consumer: deterministic visual frame activity comes from FrameState when available and carries `canonicalState` into the existing preview index.
- #261 exact head `c6417af5682adb54ccaaa0b340089e9b18d162cb` passed Quality Gate #1501, Security #1507, backend race, frontend lint/unit/build/performance, Playwright, renderer parity, desktop/Helm, CodeQL, and all platform/sandbox assurances; no review threads/comments required remediation.
- #261 squash-merged as `1fa4a2c9fb0ba02b00a194374dc363fe5f796199`.
- #262 was rebuilt from the #261 merge, retargeted to `main`, and retained only its diagnostics/tracker delta. Exact head `64a9a034b6e77334980c87bfcbc73416e1fd927e` passed Quality Gate #1506, Security #1512, Playwright, deterministic renderer parity, CodeQL, and the platform/sandbox assurances, then squash-merged as `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec`.
- Retained decoded parity evidence was inspected before defining Phase 0 structural policy; raw RGB exactness is codec-noisy and is not being promoted into a false zero-tolerance production gate.
- #263 exact head `5e22a663506bcc8321534329693bcd1b185cf444` passed Quality Gate #1517, Security #1523, Playwright, renderer parity, backend race, CodeQL, and all platform/sandbox assurances with no review comments/threads; final compare contained only the intended four paths.
- #263 squash-merged with expected-head protection as `77b49e096e165ad579d7eb5daed81763c023203c`.
- #264 was rebuilt from #263's exact squash result, retained only its transform resolver/test/Canvas/tracker scope, and added exact-zero-axis compatibility coverage for the legacy truthy CSS scale fallback.
- #264 exact head `b84b4b82e72393b27fcf6bf579fb9a8579ea9f5c` passed Quality Gate #1523, Security #1529, backend race, frontend lint/unit/performance/build, Playwright, renderer parity, CodeQL, desktop/Helm, and all platform/sandbox assurances with a clean review audit; it squash-merged with expected-head protection as `357ae16301fc0b7d6c0f0d65e99c0277bac77adb`.
- #265 was opened helper-only while #264 parity was still running. After #264 merged, #265 was rebuilt directly from `357ae16301fc0b7d6c0f0d65e99c0277bac77adb`, retargeted to `main`, verified ahead 1 / behind 0, and then wired into `VideoPreviewCanvas.tsx` while keeping full perspective projection explicitly out of scope.

## Next recommended slice

1. Run exact-head Quality/Security/platform/Playwright/renderer-parity validation for #265, inspect `compare main...branch` and review threads, then squash-merge only if the canonical view-transform consumer remains current/scoped/green.
2. From resulting `main`, consume canonical **perspective projection** with an explicit per-layer-capable mapping rather than forcing clip overrides through the current single parent CSS perspective.
3. Then consume canonical media geometry/bounds, followed by transition/effect and text/shape/cursor painter inputs, and finally AudioGraph scheduling in separate reviewable slices.
4. In parallel, implement the codec-aware Phase 0 structural-policy wiring and add second-platform parity evidence; current global numeric thresholds alone are not visual sign-off.