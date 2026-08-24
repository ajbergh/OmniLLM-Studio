# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-24  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG feature PR: **#260 — Define canonical AudioGraph v1** — squash merge `9acc544eac6cc63a14a4e0f22ee52cb07688e010` (2026-08-24).

**Phase 2 — Canonical contract is complete.** The renderer-independent visual contract (`visual-frame-state-v1` and its versioned subcontracts) and audio contract (`audio-graph-v1`) now define the semantic decisions needed by the remaining preview/export work. No consumer may silently re-define those semantics and still claim canonical parity.

Current implementation PR: **#261 — Add canonical preview composition projection** on branch `feat/video-wysiwyg-phase3-preview-composition`.

#261 was initially stacked on the still-running #260 head so Phase 3 work could continue without bypassing #260's merge gate. After #260 passed its exact-head matrix and merged, #261 was rebuilt from the actual new `main` (`9acc544eac6cc63a14a4e0f22ee52cb07688e010`) and retargeted to `main`. `compare main...branch` confirmed exactly four intended code/test paths before this tracker update:

- `frontend/src/video/renderContractPreviewComposition.ts`
- `frontend/test/renderContractPreviewComposition.test.ts`
- `frontend/src/components/video/pro/timelineIndex.ts`
- `frontend/src/components/video/pro/timelineIndex.test.ts`

#261 starts **actual Phase 3 consumer adoption**, not just another semantic definition:

- introduces `preview-composition-frame-v1`, a runtime bridge from canonical `visual-frame-state-v1` back to the exact persisted Timeline v1 track/clip/asset objects needed by the existing editor painter;
- retains the strict v1 → v2 adapter and its structured unavailable diagnostics rather than weakening ambiguous legacy semantics;
- validates positional and ID identity so a future adapter reordering cannot bind canonical state to the wrong editor clip;
- routes deterministic frame-addressed **visual activity** through canonical FrameState when canonical evaluation is available;
- attaches the exact `CanonicalFrameLayerState` to matching indexed preview entries for subsequent transform/source-time/painter adoption;
- keeps deterministic audio entries on the current path until AudioGraph runtime consumption lands;
- explicitly falls back to the previous deterministic frame-overlap path when the strict adapter cannot represent the saved v1 timeline (notably legacy transition placement), preserving current preview behavior without pretending the fallback is canonical.

The normal free-running preview, camera evaluation, source seeks, transform/geometry/effect/text/shape/cursor painting, and Web Audio gain/fade scheduling are **not yet fully canonical consumers**. They remain the next Phase 3 slices.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds and second-platform evidence remain. `audio-graph-v1` now defines the unsupported-audio semantic boundary. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261 introduces the canonical preview projection and routes deterministic visual frame activity through FrameState when available. Source-time/transform/camera/painter/audio consumption remains. |
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

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can look current while the tree silently reverts unrelated work. #261 followed the safe procedure after #260 merged.

## Phase 0 — Reproducible parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains a known-mismatch diagnostic baseline, not a production threshold.

Remaining Phase 0 sign-off:

1. Freeze production visual threshold policy and zero-tolerance structural regions.
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
- Exact-head **Quality Gate #1491 passed**, including backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright smoke, Windows desktop contract, and the deterministic Video renderer parity baseline.
- Exact-head **Security Scan #1497 passed**, including dependency audits and both Go and JavaScript/TypeScript CodeQL analysis.
- Linux workspace/quota, browser-egress, sandbox-worker, macOS runtime/extension/adversarial, and Windows/macOS confinement assurances all passed.
- There were no review submissions, inline review threads, or PR comments requiring remediation.
- `compare main...branch` was rechecked immediately before merge and contained only the six intended AudioGraph/test/tracker paths; `main` had not advanced.
- #260 squash-merged as `9acc544eac6cc63a14a4e0f22ee52cb07688e010`.

### Phase 2 exit gate — satisfied

The renderer-independent semantic contract now covers the currently authorable visual and audio state required by this plan. Phase 3/4/6 consumers must use the canonical contracts rather than duplicate curve, frame, source-time, ordering, geometry, projection, transition, effect, text, shape, cursor, or audio semantic math. New authorable fields without explicit semantics remain fail closed.

## Phase 3 — Shared preview composition

**In progress on #261.**

### Current slice — canonical preview projection and deterministic activity consumption

`preview-composition-frame-v1` is a lightweight runtime projection, not a new semantic engine. It:

- evaluates the existing strict `visual-frame-state-v1` diagnostic path;
- binds canonical layer state back to exact Timeline v1 track/clip/asset objects required by the current editor UI;
- verifies identity/position mapping;
- surfaces structured unavailable state instead of inventing v1 transition placement;
- allows the current DOM/CSS painter to migrate incrementally without re-deriving canonical semantics.

`queryActiveClipsAtFrame` now consumes that projection for deterministic visual activity when available and carries `canonicalState` on matched entries. The existing indexed point query still serves free-running playback. Audio entries remain separate until the AudioGraph consumer slice. This deliberately limits blast radius while proving the first production consumer seam.

Focused coverage proves:

- canonical z/order is preserved while exact editor object identity and asset binding remain intact;
- transition ambiguity remains fail closed in the projection;
- canonical source-time/transform state is propagated without recomputation;
- 120-fps clips that start inside an output frame retain canonical frame activity and receive `canonicalState`;
- when canonical v1 adaptation is unavailable, deterministic selection preserves prior behavior and does not falsely attach canonical state.

### Remaining Phase 3 work

Recommended order after #261:

1. Make deterministic `VideoPreviewCanvas` media seek/source-time reads consume `canonicalState.source_time_ms` when available; retain the existing source-timing helper only for free-running/fallback paths.
2. Consume canonical evaluated transforms, opacity/fades, bounds/media geometry, perspective/model state, and canonical camera state in the program monitor while keeping drag/live-direct-manipulation state as a separate overlay.
3. Move transitions/effects/text/shape/cursor painter inputs to their already-evaluated FrameState fields, removing the remaining local `sampleCursor`, fade/property, and registry re-evaluation where canonical state exists.
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
| Legacy FFmpeg approximations become contract | Canonical state is explicit; legacy renderers are evidence/consumers only. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed; #261 preserves this boundary. |
| Canonical preview projection binds wrong editor object | #261 verifies track/clip positional and ID identity before exposing canonical state. |
| Frame-addressed preview silently mixes canonical and fallback semantics | #261 attaches `canonicalState` only when the strict projection succeeds; fallback entries remain explicitly without it. |
| Millisecond rounding creates frame/source drift | Rational frame/source helpers and integer AudioGraph sample-boundary rules. |
| Source aspect ratio is guessed | Explicit bounds or immutable source provenance only. |
| Perspective differs between consumers | One canonical projection contract in FrameState. |
| Pair transitions become independent alpha layers | Pair paint operates on isolated surfaces with canonical weights/transforms. |
| Effect/text/shape/cursor defaults drift | Versioned evaluated state is authoritative; unsupported metadata fails closed. |
| Browser/system font fallback changes metrics | Packaged static-face provenance; deterministic intrinsic metric ownership remains explicit Phase 3–5 work. |
| Audio pitch/fade/processing location diverges | AudioGraph explicitly owns pitch preservation, minimum fade overlap, and post-mix processing. |
| Unsupported channels are silently remixed | v1 accepts mono→stereo or stereo passthrough only; other layouts fail closed. |
| Stacked branch carries stale tree | Rebuild from actual `main` and inspect `compare`; #261 was normalized this way. |
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
- Initial #260 head exposed only Go formatting; the corrected final head `43a0d1c1a5044937f13be30e0bad58bb7640d41c` passed the complete Quality/Security/platform matrix, including race, Playwright, CodeQL, and renderer parity.
- #260 squash-merged as `9acc544eac6cc63a14a4e0f22ee52cb07688e010`, completing Phase 2.
- Opened draft #261 while #260 parity was still executing, then safely rebuilt #261 directly from merged `main` and retargeted it to `main`.
- #261 introduced `preview-composition-frame-v1`, strict identity binding, and the first real canonical preview consumer: deterministic visual frame activity now comes from FrameState when available and carries `canonicalState` into the existing preview index.
- This tracker update creates a new exact #261 head; the complete required hosted matrix must pass before #261 can be marked ready or merged.

## Next recommended slice

1. Finish exact-head CI/review for #261, re-check `compare main...branch`, and squash-merge only if current, scoped, and green.
2. From resulting `main`, route deterministic media **source-time/seek** and evaluated transform/opacity reads in `VideoPreviewCanvas.tsx` through the `canonicalState` already carried by #261; preserve live drag/direct-manipulation state separately.
3. Continue removing preview-local canonical re-evaluation in small slices: camera/projection, geometry, transitions/effects, text/shape/cursor, then AudioGraph scheduling.
4. Keep Phase 0 production visual thresholds and second-platform evidence moving in parallel.
