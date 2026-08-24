# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-24  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG feature PR: **#255 — Enforce static-face-only variable-font policy in font provenance** — squash merge `853b59d07280f7441258586632aa6a43f618bff5` (2026-08-21).

Current implementation PR: **#260 — Define canonical AudioGraph v1** on branch `feat/video-wysiwyg-phase2-audio-graph`.

The branch was created from the actual current `main` head `dc51d7ddaf8060304fbe63153796e4ebafe48a20` on 2026-08-24, not from the older #255 merge SHA recorded in the previous handoff. The intervening `main` change was documentation-only, but the branch ancestry was still corrected rather than assuming the stale handoff was current.

PR #260 defines the last remaining Phase 2 semantic contract in both Go and TypeScript, with one shared fixture:

- canonical 48 kHz stereo output and exact timeline/range sample counts;
- deterministic source-node identity and stable track/clip ordering;
- track mute, clip mute, and track-solo selection while retaining suppressed-node identity;
- clip timeline/sample placement and normalized trim windows;
- playback-rate serialization with an explicit **pitch-preserving** policy;
- mono-to-stereo and stereo-passthrough channel mapping, failing closed for channel layouts without v1 semantics;
- base gain and volume automation with authored-order tie-breaking;
- linear fade durations in samples with the preview-compatible `minimum` overlap-combine policy;
- non-normalizing summation (`sum-no-normalize`);
- authorable `render_audio_processing` serialized as a **post-mix processed-stem requirement**, not renderer-specific FFmpeg filter syntax;
- fail-closed handling for malformed/unknown program-processing fields and unsupported audio metadata.

PR #260 deliberately does **not** change Web Audio preview mixing or legacy FFmpeg export mixing. Consumer adoption remains Phase 3/6 work.

Validation status for #260:

- initial exact head `a630f0fe67ed4d485182622140e24115e94bb925` passed the frontend lint/unit/build gate but the backend gate stopped at `gofmt` on `audio_graph_test.go` before semantic Go tests ran;
- formatting was corrected in `51770175e333e59725b44bd0c869e935002ccdd7`;
- on that corrected implementation head, Go formatting, `go vet`, the full backend unit/integration suite, and frontend lint/unit/build all passed; the complete hosted matrix was still finishing when this tracker commit was authored;
- this documentation commit creates a new exact PR head and therefore must itself pass the required hosted checks before merge.

No Phase 3 preview compositor or Phase 4 Chromium-renderer behavior is changed by #260.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds and second-platform evidence remain. #260 now defines the unsupported-audio semantic boundary that Phase 0 previously lacked. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | Merge-ready pending #260 CI | Timing, curves, v1 adapter, frame/range/source/order, normalization, frame addressing, property evaluation, FrameState, media geometry, perspective, all current transition state/paint families, effect stack state, canonical text/shape/cursor state, immutable source provenance, manifest-backed font resources, authored text-face binding, static-face policy, and AudioGraph are defined. Consumer adoption begins in Phase 3/6. |
| Phase 3 — Shared preview composition | Not started | Program monitor consumes canonical FrameState/AudioGraph instead of preview-local semantic math. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close text metrics/fonts, shapes, effects, transitions, cursor, camera, color, asset loading, and decoded visual thresholds. |
| Phase 6 — Audio parity closure | Not started | Shared AudioGraph/processed-stem architecture, rate/pitch policy, gain/fades/channel mapping, processing, and decoded-delivery verification. |
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

Canonical evaluators must be pure, deterministic, serializable, free of browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed whenever an authorable value does not have explicit canonical semantics.

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Preview behavior is the Timeline v1 compatibility target where it is actually implemented. Where preview lacks an authorable feature, the canonical contract defines intended behavior explicitly instead of treating absence as semantics.

### Media geometry and source provenance

`media-geometry-v1` is authoritative for asset geometry:

- source aspect ratio requires explicit `content_bounds` or versioned immutable `source-provenance-v1` media-probe state;
- source dimensions are never guessed from the output canvas;
- `mask_source_crop` operates in source coordinates before fit;
- `contain`, `cover`, `fill`, and `none` are canonical fit modes;
- `transform.crop` is a separate post-fit output viewport clip;
- FrameState carries evaluated painted bounds and immutable source provenance;
- missing or invalid source provenance remains explicit unresolved/fail-closed state rather than a canvas-sized fallback.

### Perspective and stacking

Perspective is projection state, not paint order. Track/z-index order remains authoritative for stacking; spatial `z` affects projection.

`perspective-projection-v1` preserves the preview-compatible 1200-canvas-pixel distance with no scene camera, derives projection distance from evaluated camera FOV when a camera is active, allows a positive per-clip perspective override, and serializes projection separately from the camera-relative model matrix.

### Transition timing and paint

`transition-state-v1` makes placement, owner/peer roles, windows, progress, and real-overlap requirements explicit. No hidden source handles or inferred adjacency are invented.

`transition-paint-v1` covers every currently authorable Timeline v2 transition family:

- `fade`: one-sided owner opacity;
- `crossfade`: isolated-surface pair blend;
- `dip_to_black`: explicit outgoing/black/incoming contribution weights;
- `slide`: normalized canvas-fraction translations;
- `wipe`: normalized layer-fraction clip insets;
- `zoom`: canonical scale envelope around the evaluated authored anchor plus one-sided opacity or pair weights.

Phase 3 consumers must apply pair operations to isolated surfaces and must not reinterpret pair paint as independent stacked layer opacity/transform.

### Effect stack

`effect-state-v1` is the renderer-independent evaluated effect operation carried by FrameState.

- only enabled effects enter the stack;
- authored effect-array order is preserved;
- scope is explicit as `clip` or `scene`;
- defaults and numeric bounds are registry-grounded;
- clip `effect.<id>.amount` automation is sampled at exact output-frame presentation time, with `effect.<type>.amount` retained as a compatibility fallback;
- scene effects remain static because Timeline v2 has no scene-effect keyframe collection;
- unknown effect types/parameters, non-finite values, and undefined amount automation fail closed.

### Text and font state

`text-state-v1` serializes text content, family/source, size, weight, colors, stroke, shadow, alignment, line height, letter spacing, border radius, box dimensions, and per-side padding once.

Deterministic font identity is manifest-backed:

- `font-resource-provenance-v1` identifies packaged static faces;
- authored `font_resource_id` binds text to an immutable packaged face;
- `font_face_source` distinguishes packaged-resource, family-name-only, and composition-default provenance;
- font uploads are stored as video-project assets, referenced resources are staged into immutable render snapshots, and hashes/bytes are revalidated at execution;
- v1 accepts static faces only; variable faces fail closed until explicit axis-selection semantics exist;
- intrinsic glyph bounds/metrics are not guessed and remain Phase 3–5 consumer work.

### Shape state

`shape-state-v1` covers all 14 currently authorable annotation kinds with canonical dimensions, fill/stroke/radius defaults, fail-closed validation, and FrameState projection. Shape-derived `content_bounds` come from evaluated canonical shape dimensions, not a second local defaulting implementation.

### Cursor state

`cursor-state-v1` is sampled at exact clip-relative rational presentation time with preview-compatible linear interpolation/endpoint hold, strict `<300ms` click proximity, optional-visible semantics, explicit scale/highlight/click-ring state, and fail-closed malformed-event validation. `smoothing:true` remains unsupported until one canonical algorithm exists.

### AudioGraph v1

`audio-graph-v1` is the renderer-independent evaluated audio contract introduced by #260.

Canonical v1 rules:

1. Output format is exactly 48,000 Hz, two-channel stereo.
2. Timeline and render-range boundaries are converted to exact integer sample boundaries with floor/ceil rules analogous to frame addressing.
3. Every audio-capable clip is represented by a stable source node even when mute/solo policy suppresses playback.
4. Track mute wins over clip mute; clip mute wins over solo suppression for diagnostic reason identity. If any track is soloed, non-solo tracks are suppressed.
5. Playback rate is serialized explicitly and **preserves pitch**. This matches the existing FFmpeg `atempo` intent and marks legacy Web Audio rate-induced pitch shift as consumer debt rather than contract semantics.
6. Mono sources map to stereo duplication; stereo sources pass through. Other probed channel counts fail closed in v1 until an explicit downmix/upmix matrix is defined.
7. Clip `volume` is a finite 0–2 base gain. Volume automation is serialized in exact keyframe order; equal-time points retain authored array order.
8. When volume automation exists it is the canonical gain envelope for that property (`automation-overrides-base`), matching existing property-evaluator semantics rather than multiplying a second base-gain implementation.
9. Fades are linear, serialized as exact sample counts, and overlapping fade-in/fade-out contributions combine with `minimum`, matching the current preview contract instead of legacy FFmpeg's multiplicative overlap side effect.
10. Source summation is `sum-no-normalize`; consumers must not silently normalize the mix.
11. `render_audio_processing` is a program-level **post-mix** operation. The canonical graph exposes `program-mix` → `program-output` as a processed-stem boundary rather than baking FFmpeg filter syntax into semantic state.
12. Unknown processing keys, missing required processing fields, unsupported presets/channel modes, invalid numeric domains, absent audio probes, and unsupported channel mappings fail closed.
13. Phase 2 defines semantics only. Phase 6 makes Web Audio/Chromium and export consumers obey the graph and validates decoded-delivery parity.

### Safe stacked-branch normalization

A stacked PR must be rebuilt from the **actual current `main` tree** after its parent merges.

Required procedure:

1. Read current `main` commit/tree.
2. Identify only the intended child delta.
3. Rebuild the child directly on current `main`.
4. Verify `compare main...branch` contains no unrelated changes/deletions.
5. Update this tracker on the clean branch.
6. Validate the exact final head before merge.

Never manufacture current ancestry by grafting a stale feature tree onto a newer parent; #225 demonstrated that Git ancestry can look current while the resulting tree silently reverts unrelated work.

## Phase 0 — Reproducible parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains intentionally a known-mismatch diagnostic baseline, not a production threshold.

Remaining Phase 0 sign-off:

1. Freeze production visual threshold policy and zero-tolerance structural regions.
2. Use `audio-graph-v1` as the explicit unsupported/semantic boundary until Phase 6 consumers land; no preview/export path may claim pitch/fade/program-processing parity merely because it can render audio.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable throughout Phases 3–7.

## Phase 1 — Immutable submission

**Complete.** A queued render is bound to one immutable timeline revision/hash and immutable source bytes. Snapshot identity, staged source bytes, decode preflight, recovery, stale-request rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are production foundations for later parity work.

## Phase 2 — Canonical contract

### Merged foundations

| PR | Capability | Merge SHA |
|---|---|---|
| #187 | Immutable render submission and deterministic parity baseline | `3cbe6ba81dde384ff1c6073537e3d9b71ed8d0b0` |
| #191 | Timeline v2 / Render Manifest v1; canonical frame/source/easing primitives | `aabbb31288277287673cbed8546c9eb3f38588e4` |
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
| #225 | Fade-family transition-paint consumption in FrameState | `a6f9145f92e2342dfa70144a4058bf10f64625da` |
| #227 | Canonical slide transition paint + FrameState consumption | `28639ec4fee09635de39764b33021bb6d9aa418c` |
| #228 | Canonical wipe transition paint + FrameState consumption | `3007be763dacde820b8b494f62b6a82bb9af5324` |
| #229 | Canonical zoom transition paint + FrameState consumption | `a8e2a0964e1e49232be08bf2fe7091ce3d9403e6` |
| #237 | Canonical effect stack state + FrameState consumption | `22e73cc291a4f8723a99ad123c963aedf0fd0d8a` |
| #241 | Canonical text renderer state + FrameState consumption | `9b072685b689cdb74e0a5590a26478f6a3ef12b4` |
| #242 | Canonical shape renderer state definition | `1a25ac0fef217731197169b229bde19aff158c8b` |
| #243 | Canonical shape state consumption in FrameState | `111fba5fd7ea73740aee1d92fc1038ee72fda30b` |
| #245 | Canonical cursor renderer state definition | `c428d81f25ce1d85faa6655fb5772430a8fe6b22` |
| #247 | Canonical cursor state consumption in FrameState | `66530a07e3a5585546d978794e24198083bbeaa2` |
| #251 | Immutable source provenance and FrameState consumption | `d49808f1ea7fc23f658e95f52fdbe404bf0be92a` |
| #252 | Canonical font-resource provenance definition | `a8a1d649a209d0013390f0f838b5f32613a2ce02` |
| #253 | Authored text-face binding to packaged resources | `c7db41ade06c6729a77c6d0464ae30aa8a9fa4a6` |
| #254 | Font-resource snapshot packaging and upload/storage | `b99991c5121c2122bf0cd0f5ef0f7ab8e3514845` |
| #255 | Static-face-only variable-font policy enforcement | `853b59d07280f7441258586632aa6a43f618bff5` |

Supporting unblocks during the program:

- #201 replaced reachable-vulnerable `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.
- #219/#226 improved bounded/retried CI dependency and Playwright setup.
- #239 aligned sandbox-worker SQLite test behavior with production WAL/busy-timeout behavior.
- #246 expanded Vitest discovery so canonical contract tests under `frontend/test/` run in the normal unit/Quality Gate.

### Current PR #260 — canonical AudioGraph v1

Implementation paths:

- `backend/internal/video/rendercontract/audio_graph.go`
- `backend/internal/video/rendercontract/audio_graph_test.go`
- `backend/internal/video/rendercontract/testdata/audio_graph_v1.json`
- `frontend/src/video/renderContractAudio.ts`
- `frontend/test/renderContractAudio.test.ts`

The shared fixture is intentionally consumed by both runtimes so serialized graph drift is a test failure, not a review-time inference.

### Phase 2 exit gate

After #260 merges, the renderer-independent semantic contract is complete for the currently authorable visual and audio state required by this plan. Phase 3 preview and Phase 4/6 export consumers must consume these contracts rather than duplicate semantic math. Any new authorable field without explicit semantics remains fail closed.

## Phase 3 — Shared preview composition

Drive the Video Edit Studio program monitor from canonical FrameState/AudioGraph while preserving direct-manipulation UI state separately.

Recommended implementation order:

1. Introduce a renderer-neutral preview composition adapter that accepts immutable/canonical Timeline v2 or Render Manifest v1 state plus `frameIndex` and returns the already-evaluated `visual-frame-state-v1` and `audio-graph-v1` inputs needed by painters.
2. Move active-layer selection, source-time selection, ordering, evaluated transforms/bounds/projection, transition paint, effects, text, shape, and cursor reads in the program monitor behind that adapter without changing paint technology in the first slice.
3. Add an opt-in diagnostic mode showing frame identity, active clip IDs, source time, matrices/bounds/projection, transition/effect identities, and AudioGraph identity.
4. Replace preview-local audio semantic math with AudioGraph-driven scheduling only after visual FrameState consumption is stable; processed-stem/program-processing audition remains Phase 6 if it requires offline DSP.
5. Keep direct-manipulation gesture state separate from canonical playback state so dragging/resizing remains responsive while playback/export semantics stay deterministic.

Phase 3 exit: normal playback/program-monitor rendering no longer independently decides frame activity, source time, layer order, canonical transforms/geometry/transition/effect/text/shape/cursor state, or audio selection/gain/fade semantics.

## Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets deterministically, manage browser health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux behind a guarded rollout.

## Phase 5 — Visual parity closure

Close decoded output parity for media timing/fit/crop, transforms/anchors/2.5D/camera/z-order, opacity/blending, text metrics/fonts, shapes, transitions, effects, cursor, color space, and deterministic asset loading.

## Phase 6 — Audio parity closure

Make preview/Chromium and export consumers obey `audio-graph-v1` exactly:

- source time and playback rate with pitch preservation;
- canonical mono/stereo mapping and future explicit matrices for additional layouts;
- mute/solo, gain automation, fade overlap semantics, and non-normalizing mix;
- one post-mix program-processing stage;
- processed-stem architecture when the deterministic renderer cannot reproduce a processing operation identically in real time;
- exact output/range sample counts and decoded-delivery verification.

Phase 6 must remove the current semantic mismatches rather than bless them: legacy Web Audio rate changes pitch, while legacy FFmpeg preserves it; legacy FFmpeg applies program processing per input, while authoring intent and AudioGraph define post-mix processing; overlapping legacy FFmpeg fades multiply, while AudioGraph defines minimum-envelope semantics.

## Phase 7 — Rollout and legacy retirement

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

Hosted CI is authoritative for platform/toolchain cases that cannot be reproduced in the current execution environment. Setup-only stalls and runner-capacity queues are recorded explicitly and are never represented as passing.

Before every merge:

1. Verify the PR's exact final head.
2. `compare main...branch` and inspect every changed path.
3. Resolve all review threads or explain why a non-actionable thread is being dismissed.
4. Record concrete validation evidence in this tracker/PR.
5. Never call an unexecuted or setup-only job green.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest projection checks fail CI; canonical `frontend/test/` suites are included by #246. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState/AudioGraph diagnostics. |
| Legacy FFmpeg approximations become de facto contract | Canonical semantics are explicit; legacy renderer is evidence only. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed. |
| Millisecond rounding creates frame/source drift | Canonical rational frame/range/source helpers; AudioGraph uses deterministic integer sample-boundary rules. |
| Source aspect ratio is guessed from canvas | Explicit bounds or immutable source provenance only. |
| Crop ordering diverges | Source mask before fit; output crop after fit. |
| Perspective differs between preview/export | One canonical projection contract carried in FrameState. |
| Pair transitions are misapplied as independent alpha layers | Pair paint operates on isolated surfaces using canonical weights/transforms. |
| Effect ordering/defaults diverge | `effect-state-v1` preserves authored order and normalized parameters. |
| Unsupported effect metadata is silently ignored | Canonical evaluation fails closed. |
| Browser/system font fallback changes text metrics | Packaged static-face provenance plus fail-closed resource binding; intrinsic metric ownership remains explicit Phase 3–5 work. |
| Intrinsic text bounds are guessed | Only explicit positive box dimensions are canonical until deterministic glyph layout exists. |
| Shape/cursor semantics inherit legacy approximations | Versioned canonical state is authoritative; unsupported state fails closed. |
| Audio pitch behavior diverges | `audio-graph-v1` explicitly requires pitch preservation. |
| Audio fade overlap diverges | `audio-graph-v1` explicitly requires linear minimum-envelope overlap semantics. |
| Program processing is applied at different graph locations | `audio-graph-v1` defines one post-mix processed-stem boundary. |
| Unsupported channel layout is silently remixed | v1 only accepts canonical mono→stereo or stereo passthrough; other layouts fail closed. |
| Stacked branch appears current but carries stale tree | Rebuild from actual current `main` and compare every path before merge. |
| CI setup/runner saturation hides code state | Distinguish setup/queue from executed code checks. |
| Browser worker resource cost | Admission control, health checks, cancellation, guarded rollout, FFmpeg retained for media I/O. |

## Implementation log

### 2026-08-17

- #187 established immutable submission and deterministic parity evidence.

### 2026-08-18

- #191–#206 advanced the canonical contract from schema/timing through exact-frame visual FrameState.
- #201 repaired the reachable PDF dependency vulnerability.
- #208 added permanent cross-runtime FrameState diagnostics and fixed the CodeQL allocation finding before merge.

### 2026-08-19

- #209 canonicalized media geometry and corrected an initial Go formatting-only Quality Gate failure before merge.
- #212 consumed media geometry in FrameState and removed canvas-sized source-bounds guessing.
- #218 canonicalized perspective projection.
- #219/#226 improved CI dependency/Playwright reliability.
- #220–#229 completed transition placement and all currently authorable canonical paint families.
- #237 defined `effect-state-v1` and projected clip/scene effects into FrameState.
- #241 defined `text-state-v1`; its initial connector-authored Go formatting issue was corrected before the exact-head hosted matrix passed.

### 2026-08-20 to 2026-08-21

- #242/#243 defined and consumed `shape-state-v1`.
- #246 fixed canonical Vitest discovery and corrected newly exposed stale assertions.
- #245/#247 defined and consumed `cursor-state-v1`.
- #251 added immutable source provenance and canonical anchor/geometry consumption.
- #252 defined font-resource provenance.
- #253 bound authored text to packaged font faces.
- #254 added font upload/storage and immutable snapshot packaging; hosted parity caught and drove a fix for a non-text-clip nil dereference before merge.
- #255 enforced static-face-only font semantics.

### 2026-08-24

- Refreshed against current `main` `dc51d7ddaf8060304fbe63153796e4ebafe48a20`; the stale handoff's claimed AudioGraph branch did not exist.
- Created `feat/video-wysiwyg-phase2-audio-graph` directly from current `main` and opened #260.
- Implemented mirrored Go/TypeScript `audio-graph-v1` evaluators plus a shared fixture covering rate/pitch policy, exact sample boundaries, mute/solo suppression identity, channel mapping, gain automation, fades, and post-mix program-processing intent.
- Initial exact head `a630f0fe67ed4d485182622140e24115e94bb925` exposed a formatting-only backend gate failure in `audio_graph_test.go`; frontend lint/unit/build was already green.
- Corrected `gofmt` in `51770175e333e59725b44bd0c869e935002ccdd7`; formatting, `go vet`, full backend tests, and frontend lint/unit/build passed on that implementation head while the remaining hosted jobs continued.
- Updated this tracker on the PR branch; the resulting exact head must pass the required hosted matrix before #260 merges.

## Next recommended slice

1. Finish exact-head hosted validation and review for #260, verify `compare main...branch`, and squash-merge only if current/scoped/green.
2. From the resulting current `main`, begin Phase 3 with the smallest shared-preview-composition slice: introduce the canonical preview composition adapter and route program-monitor **semantic reads** through `visual-frame-state-v1` without changing paint technology yet.
3. Keep Phase 0 production visual thresholds and second-platform evidence moving in parallel; use AudioGraph as the explicit unsupported-audio boundary until Phase 6 consumer parity is complete.
