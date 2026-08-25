# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-25  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#274 — Define canonical transition pair pixel composition** — squash merge `c8c09aa42073711a52681c7b2edf907210ab4e05` (2026-08-25).

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** The renderer-independent visual contract (`visual-frame-state-v1` and its subcontracts) and audio contract (`audio-graph-v1`) define semantics; current work is migrating real program-monitor consumers onto those already-evaluated decisions in small, reversible slices.

Current implementation PR: **#275 — Plan canonical preview transition pair execution** on branch `feat/video-wysiwyg-phase3-transition-pair-preview-consumer`, created directly from #274's actual squash result `c8c09aa42073711a52681c7b2edf907210ab4e05`.

#269 completed deterministic media pixel geometry consumption:

- deterministic image/video pixels use canonical `painted_bounds` and full-stage canvas-space `clip_bounds` instead of browser-recomputed fit/crop;
- free-running playback, unavailable/invalid canonical geometry, live transforms, and crop editing keep the explicit legacy compatibility path;
- exact clean head `679945435ceb8850bed1853b81005ba9e3146ba9` passed Quality #1562, Security #1568, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone platform/sandbox assurance;
- final compare was ahead 1 / behind 0 with exactly four intended files and no review/comment activity before expected-head squash merge.

#270 completed canonical deterministic clip-effect consumption:

- deterministic clip CSS filters consume `CanonicalFrameLayerState.effects`, including exact-frame amount automation, while canonical empty stacks remain authoritative;
- exact normalized head `8a9494a8251428b05382551893cc2bde64cfeb22` passed Quality #1564, Security #1570, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and all standalone platform/sandbox assurances;
- final compare was ahead 1 / behind 0 with exactly four intended paths and no review/thread activity before expected-head squash merge as `0b6c9bf682287dad0983948ee9168a9b70a11479`.

#271 completed the explicit v1 transition-authoring bridge:

- additive optional `placement` / `peer_clip_id` fields persist explicit intent without changing Timeline v1 or reclassifying legacy rows;
- legacy transitions with no placement still fail closed as `V1_TRANSITION_PLACEMENT_AMBIGUOUS`;
- Go and TypeScript adapters validate renderable peer identity, distinct starts, authored overlap, and supported type/placement combinations with shared fixture coverage;
- exact normalized head `b5a27f84317f7219ec76eea22f09826ba217d64c` passed Quality #1570, Security #1576, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and all standalone platform/sandbox assurances before expected-head squash merge as `b2c4abc695035c4c2b61bae4554294eebea674aa`.

#272 completed the first exact transition-paint consumer without inventing pair semantics:

- deterministic owner-scoped `owner-opacity`, `owner-translate`, `owner-wipe`, and `owner-zoom` paint modifies the already-canonical layer surface;
- canonical omission is authoritative identity paint; transform dragging and crop editing remain on the explicit legacy interaction path;
- pair transitions, dip-to-black, and multiple simultaneous paints remain explicitly deferred rather than using an incorrect independent-opacity approximation;
- normalized exact head `a485c3cef968411d53b04482ab3f6e5e90d2c6cb` passed Quality #1578, Security #1584, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, CodeQL, desktop/Helm/plugin lifecycle, and all standalone platform/sandbox assurances before expected-head squash merge as `ffc2ea3ca744262aae2339a85075a749f5fa0b7a`.

#273 completed the renderer-independent isolated pair-surface stacking boundary:

- `transition-pair-surface-plan-v1` resolves pair-transition inputs from already-canonical bottom-to-top FrameState ordering;
- a two-input surface is eligible only when both inputs are active, authoritative, and adjacent, so replacing the two slots with one surface cannot reorder an unrelated layer;
- missing, non-authoritative, and non-adjacent inputs remain explicitly deferred; duplicate ownership and conflicting pair claims fail closed;
- crossfade, slide, wipe, zoom, and between dip-to-black share one Go/TypeScript fixture; owner-only paint remains outside pair planning;
- code-bearing head `ac332dc4980c145c4e1eafbdea6f2037b4dfcfda` passed Quality #1583, Security #1589, and the Linux/macOS/browser sandbox assurance matrix. Final head `6e3d61982c8b754627b60f58783e3d0404a8d7e7` changed only this tracker and removed temporary tracker scaffolding; connector-authored cleanup did not schedule fresh Actions, so exact-final-head CI was not claimed. The six-path tree/review audit was clean before squash merge as `5a91fc146aed41864a47b38c626a21789ef52437`.

#274 completed exact renderer-independent pair-pixel composition:

- `transition-pair-pixel-composition-v1` uses sRGB input/output with IEC 61966-2-1 transfer, linear-sRGB working values, straight-alpha inputs/output, premultiplied accumulation, and explicit unit-interval clamping;
- crossfade and pair-zoom apply canonical outgoing/incoming weights exactly once through a weighted-sum operation; between dip-to-black adds an explicit opaque linear-black contribution;
- pair-slide and pair-wipe add no opacity weights and preserve the canonical lower/upper source-over stack after their `transition-paint-v1` spatial operations;
- surface/paint identity, owner/peer membership, lower/upper membership, finite unit-interval weights, unit-sum contributions, and unsupported compositions fail closed;
- shared Go/TypeScript fixtures cover the contract;
- exact final head `f7c5829d68e12f682646c7c1d1043928c266de34` passed Quality #1596, Security #1602, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone platform/sandbox assurance. No review submissions or review threads were present before expected-head squash merge as `c8c09aa42073711a52681c7b2edf907210ab4e05`.

#275 establishes the preview execution-slot boundary before direct painter wiring:

- deterministic canonical preview layers are converted into replacement slots using #273's exact bottom-to-top pair-surface indices;
- unrelated canonical layers stay in their original positions and non-adjacent pairs remain ungrouped/deferred;
- #274's pair-pixel operator selects `source-over-dom` only for pair-slide/pair-wipe and keeps crossfade/zoom/dip as `weighted-canvas-deferred`;
- source-over pair-slide resolves canonical outgoing/incoming canvas-fraction translations and pair-wipe resolves the canonical incoming layer clip without adding opacity weights;
- free-running or partially canonical preview state stays on the explicit legacy slot path;
- focused Vitest coverage exercises legacy fallback, in-place slot replacement, slide, wipe, weighted deferral, and non-adjacent rejection;
- behavioral+tracker head `37b84024b89e13af37f603a913dadebd46053cc2` passed Quality #1599, Security #1605, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and all standalone Linux/macOS/browser sandbox assurances. Compare was ahead 4 / behind 0 with exactly the tracker, planner, planner tests, and pair-surface input-type narrowing; review submissions and review threads were empty. This final commit only freezes those completed results into the tracker.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged the fail-closed optional frame-indexed exact-region input boundary. Production structural policy, tolerance-aware codec-region semantics, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#274 merged deterministic activity/source/transform/view/perspective/media-geometry/effect consumption, explicit transition authoring, owner paint, stack-safe pair-surface planning, and exact pair-pixel semantics. #275 establishes safe preview pair execution slots; direct source-over pair slot wiring, weighted linear-sRGB Canvas composition, scene effects, text/shape/cursor painter inputs, normal-playback canonicalization, diagnostics, and audio consumption remain. |
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
- editor preview may project already-persisted `VideoAsset.width` / `height` into temporary canonical `content_bounds`, but that projection is **not** immutable source provenance;
- source dimensions are never guessed from the output canvas, CSS box, decoded DOM element, or `object-fit` result;
- source crop precedes fit; output crop follows fit;
- `contain`, `cover`, `fill`, and `none` have canonical meaning;
- FrameState carries evaluated source/visible/painted/clip bounds;
- missing or invalid bounds remain explicit unresolved state rather than a canvas-sized fallback;
- deterministic Canvas media consumption places decoded pixels from canonical `painted_bounds` and clips through canonical canvas-space `clip_bounds`; browser `object-fit` remains only an explicit compatibility fallback.

### Perspective and stacking

Track/z-index order remains authoritative for stacking; spatial `z` affects projection. `perspective-projection-v1` serializes projection independently from camera-relative model transforms and preserves the preview-compatible 1200-canvas-pixel no-camera distance.

#267 consumes per-layer canonical perspective distance through full-stage per-layer CSS perspective contexts with stage-centered origin. The shared parent perspective is disabled only for complete deterministic canonical frames; free-running playback, unavailable/empty frames, and active live transforms retain the legacy shared-stage path.

### Transition state, paint, surfaces, and pixels

`transition-state-v1` owns placement, peer/owner roles, windows, progress, and overlap requirements. `transition-paint-v1` owns all currently authorable paint families: fade, crossfade, dip-to-black, slide, wipe, and zoom. Pair transitions operate on isolated surfaces and must not be reinterpreted as independent layer opacity. `transition-pair-surface-plan-v1` owns stack-safe pair grouping/slot replacement eligibility. `transition-pair-pixel-composition-v1` owns pair sample blending: linear-sRGB working values, straight input alpha, premultiplied accumulation, straight output alpha, exact-once canonical weighting for crossfade/zoom/dip, opaque linear black for dip contribution, and canonical source-over stack order for slide/wipe. #275 adds a preview execution-slot boundary that may use DOM source-over only when the pixel contract is unweighted; weighted families remain blocked on a true Canvas implementation.

### Effects, text, shape, and cursor

- `effect-state-v1` preserves enabled authored order, scope, normalized parameters, and exact-frame automation; #270 consumes deterministic clip effect state while frame-level scene-effect consumption remains explicit debt; unsupported metadata fails closed.
- `text-state-v1` serializes text/style intent once. Packaged static font identity is manifest-backed via `font-resource-provenance-v1`; intrinsic glyph measurement remains a Phase 3–5 consumer problem and is never guessed.
- `shape-state-v1` covers all currently authorable annotation kinds and supplies canonical dimensions/style defaults/bounds.
- `cursor-state-v1` owns exact rational sampling, visibility, scale, highlight/click-ring state, and strict `<300ms` click proximity; undefined smoothing fails closed.

### AudioGraph v1

`audio-graph-v1` is the renderer-independent audio contract merged in #260:

1. Output is exactly 48,000 Hz stereo.
2. Timeline/range boundaries become deterministic integer sample boundaries.
3. Audio-capable clips retain stable node identity even when suppressed.
4. Track mute, clip mute, and solo selection have explicit precedence/reason identity.
5. Playback rate explicitly preserves pitch.
6. Mono duplicates to stereo; stereo passes through; unsupported layouts fail closed until an explicit matrix exists.
7. Base gain and volume automation are finite, deterministic, and authored-order stable.
8. Volume automation is the property envelope (`automation-overrides-base`), avoiding double application.
9. Fades are linear and overlapping fade envelopes combine with `minimum`.
10. Summation is `sum-no-normalize`.
11. `render_audio_processing` is one post-mix program operation represented by a processed-stem boundary, not FFmpeg filter syntax.
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

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can look current while the tree silently reverts unrelated work. #261/#262/#264/#265/#266/#267 followed this normalization rule; #268 was created directly from #267's squash result, #269 from #268's actual squash result, #270 from #269's actual squash result, #271 from #270's actual squash result, #272 from #271's actual squash result, #273 from #272's actual squash result, #274 from #273's actual squash result, and #275 was created directly from #274's actual squash result `c8c09aa42073711a52681c7b2edf907210ab4e05`.

## Phase 0 — Reproducible parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains a known-mismatch diagnostic baseline, not a production threshold.

Current parity metric defaults define production-like numeric tolerances (`channel <= 2`, pixel pass rate `>= 0.999`, SSIM `>= 0.995`) and support exact structural regions. #266 merged a fail-closed optional versioned CLI input that binds those regions to frame pairs by canonical integer frame index. It rejects unknown fields/multiple JSON values, invalid or duplicate policy entries, exact rectangles that extend outside either decoded frame, and configured region frames absent from the matched preview/rendered PNG set. Region slices are cloned before binding. Omitting `--regions` preserves previous behavior.

`ParityRegion` exact comparison is decoded RGBA equality; the global tolerance metric is RGB. Retained decoded H.264 evidence on 2026-08-24 showed that literal decoded equality is not a sound general structural policy for codec-affected image areas. #266 therefore intentionally did not enable exact regions in the existing parity CI baseline or claim Phase 0 sign-off.

Remaining Phase 0 sign-off:

1. Use #266's manifest boundary to freeze and wire a production structural policy separating zero-tolerance canonical structure/identity from codec-aware decoded-region thresholds.
2. Use `audio-graph-v1` as the explicit audio boundary until Phase 6 consumers land; do not claim pitch/fade/program-processing parity merely because a path emits audio.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable throughout Phases 3–7.

## Phase 1 — Immutable submission

**Complete.** A queued render is bound to one immutable timeline revision/hash and immutable source bytes. Snapshot identity, staged source bytes, decode preflight, recovery, stale-request rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are production foundations for later parity work.

## Phase 2 — Canonical contract

**Complete as of merged #260 on 2026-08-24.**

Key merged foundations:

| PR(s) | Capability |
|---|---|
| #187 | Immutable render submission and deterministic parity baseline |
| #191–#206 | Timeline v2 / Render Manifest v1, adapters, frame/range/source/order, curves, exact-frame properties, visual FrameState |
| #208 | Permanent Go↔TypeScript FrameState parity diagnostics |
| #209/#212 | Canonical media fit/crop/source-bounds geometry and FrameState consumption |
| #218 | Canonical perspective projection and FrameState consumption |
| #220–#229 | Canonical transition placement plus all currently authorable transition paint families |
| #237 | `effect-state-v1` definition and consumption |
| #241 | `text-state-v1` definition and consumption |
| #242/#243 | `shape-state-v1` definition and consumption |
| #245/#247 | `cursor-state-v1` definition and consumption |
| #251 | Immutable source provenance and canonical anchor/geometry consumption |
| #252/#253/#254/#255 | Font-resource provenance, authored font binding, upload/storage/snapshot packaging, static-face enforcement |
| #260 | `audio-graph-v1`; Phase 2 completion |

## Phase 3 — Shared preview composition

Merged consumer sequence:

| PR | Consumer step | Merge SHA |
|---|---|---|
| #261 | `preview-composition-frame-v1`; deterministic visual activity/identity binding | `1fa4a2c9fb0ba02b00a194374dc363fe5f796199` |
| #262 | Retained preview-composition parity diagnostics | `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec` |
| #263 | Canonical deterministic visual-media source time | `77b49e096e165ad579d7eb5daed81763c023203c` |
| #264 | Canonical deterministic transform/opacity | `357ae16301fc0b7d6c0f0d65e99c0277bac77adb` |
| #265 | Canonical deterministic camera-relative `view_transform` | `02b0a5d5ce68f0ee46c16e092c525772751d5681` |
| #267 | Canonical deterministic per-layer perspective projection | `8a9343cf42d4e47f5b9d2b459737601c420d097b` |
| #268 | Persisted editor asset bounds projected into canonical preview `media_geometry` | `e498bee75757bdeff3bb3c5b8b35aa3f402265b4` |
| #269 | Canonical deterministic media `painted_bounds` / `clip_bounds` Canvas consumption | `6a382d7ec19a9fc2616ee2db81ca2fe301ecfb26` |
| #270 | Canonical deterministic clip effect state | `0b6c9bf682287dad0983948ee9168a9b70a11479` |
| #271 | Explicit v1 transition placement/peer authoring and fail-closed canonical adaptation | `b2c4abc695035c4c2b61bae4554294eebea674aa` |
| #272 | Canonical owner-scoped transition paint consumption | `ffc2ea3ca744262aae2339a85075a749f5fa0b7a` |
| #273 | Stack-safe canonical transition pair-surface planning | `5a91fc146aed41864a47b38c626a21789ef52437` |
| #274 | Exact canonical transition pair-pixel composition | `c8c09aa42073711a52681c7b2edf907210ab4e05` |

Current #275 establishes stack-preserving preview pair execution slots, resolves exact source-over slide/wipe spatial paint, and deliberately keeps weighted pair families Canvas-deferred.

Remaining Phase 3 sequence should stay reviewable:

1. Wire #275 source-over pair slots into `VideoPreviewCanvas` so eligible adjacent slide/wipe inputs render inside one full-stage isolated pair surface while unrelated canonical layers keep their exact slot order.
2. Implement weighted linear-sRGB Canvas pair composition for crossfade/zoom/dip using #274's transfer/alpha/clamp contract and apply canonical weights exactly once.
3. Consume frame-level canonical scene effects without duplicate frame evaluation.
4. Consume canonical text/shape/cursor painter inputs and deterministic intrinsic text metrics.
5. Canonicalize normal playback rather than only parity frame-addressed mode, with diagnostics/rollback.
6. Make preview audio scheduling consume `audio-graph-v1` in a separate audio-focused slice.

## Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets deterministically, manage browser health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux behind a guarded rollout.

## Phase 5 — Visual parity closure

Close decoded output parity for media timing/fit/crop, transforms/anchors/2.5D/camera/z-order, opacity/blending, text metrics/fonts, shapes, transitions, effects, cursor, camera, color space, and deterministic asset loading. Freeze production thresholds only from retained, reproducible evidence.

## Phase 6 — Audio parity closure

Make preview/Chromium/export consumers obey `audio-graph-v1` exactly, including source time/rate with pitch preservation, channel mapping, mute/solo, gain automation, minimum-envelope fades, non-normalizing summation, one post-mix program-processing stage, processed stems where required, and exact range/delivery sample counts.

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

Hosted CI is authoritative for platform/toolchain cases that cannot be reproduced in the current execution environment. Setup-only stalls and runner-capacity queues are recorded explicitly and are never represented as passing. Connector-authored commits currently may not schedule repository Actions; when that occurs, record the exact unvalidated head and never represent those checks as executed.

Before every merge:

1. Verify the PR's exact final head.
2. `compare main...branch` and inspect every changed path.
3. Resolve all review threads/comments or document why a non-actionable item is dismissed.
4. Record concrete validation evidence in this tracker/PR.
5. Never call an unexecuted or setup-only job green.
6. Squash-merge with expected-head protection, then build the next slice from the actual squash result.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest checks; canonical `frontend/test/` suites run in CI. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState/AudioGraph diagnostics. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed; preview projection keeps this boundary. |
| Preview projection binds wrong editor object | Track/clip positional and ID identity are verified before canonical state is exposed. |
| Frame-addressed preview silently mixes canonical/fallback semantics | Canonical state is attached only when strict projection succeeds; each consumer has an explicit fallback boundary. |
| Visual media recomputes source time | #263 consumes `canonicalState.source_time_ms` for deterministic visual entries. |
| Preview re-evaluates deterministic transform/opacity | #264 consumes canonical transform/opacity; live gesture fallback remains explicit. |
| Exact zero scale is lost | #264 preserves compatibility handling for canonical zero-axis scale. |
| Camera is subtracted twice | #265 consumes canonical `view_transform`; local camera subtraction is fallback only. |
| Per-layer perspective overrides collapse into one parent | #267 uses per-layer full-stage perspective contexts and complete-frame fallback semantics. |
| Independent perspective contexts reorder layers by spatial z | Canonical DOM/track/z-index ordering remains stacking authority. |
| Editor asset dimensions are mistaken for immutable provenance | #268 projects persisted width/height only as temporary `content_bounds`; it never emits `source_provenance`. |
| Source aspect ratio is guessed | Missing/partial/invalid probe bounds remain unresolved; canvas/DOM dimensions are never substituted. |
| Browser `object-fit` reinterprets canonical fit | #269 places deterministic decoded media directly at canonical `painted_bounds` and uses `object-fit: fill` only after fit is resolved; compatibility paths remain explicit. |
| Output crop is applied in the wrong coordinate system | #269 applies canonical `clip_bounds` to a full-stage media surface in canvas coordinates; legacy element-relative percentage crop is fallback-only. |
| Pair transitions reorder unrelated layers or become independent alpha layers | #273 admits only active authoritative adjacent pair inputs; #275 converts only those exact replacement indices into preview slots and leaves non-adjacent pairs ungrouped. |
| Pair blend gamma/alpha semantics drift or weights are applied twice | #274 fixes transfer/color/alpha/clamp and exact-once weighted semantics; #275 permits DOM execution only for unweighted source-over slide/wipe and keeps crossfade/zoom/dip `weighted-canvas-deferred` until a compliant Canvas path exists. |
| v1 transition migration guesses placement or peer identity | #271 persists explicit placement/peer intent and validates renderable peer/type/overlap semantics; legacy rows without placement remain `V1_TRANSITION_PLACEMENT_AMBIGUOUS`. |
| Effect/text/shape/cursor defaults drift | #270 makes deterministic clip `effect-state-v1` authoritative; scene/text/shape/cursor consumers remain explicit debt and unsupported metadata fails closed. |
| Browser/system font fallback changes metrics | Packaged static-face provenance; deterministic intrinsic metric ownership remains explicit Phase 3–5 work. |
| Audio pitch/fade/processing location diverges | AudioGraph owns pitch preservation, minimum fade overlap, and post-mix processing. |
| Unsupported channels are silently remixed | v1 accepts mono→stereo or stereo passthrough only; other layouts fail closed. |
| Structural parity is claimed from codec-noisy decoded equality | Phase 0 separates canonical zero-tolerance identity/geometry from codec-aware decoded evidence. |
| Stacked branch carries stale tree | Rebuild from actual `main` and inspect `compare` before every PR. |
| CI setup/runner saturation or connector scheduling behavior hides code state | Distinguish setup/queue/not-scheduled from executed checks and record the exact validated head. |
| Browser worker resource cost | Admission control, health checks, cancellation, guarded rollout, FFmpeg retained for media I/O. |

## Implementation log

### 2026-08-17 to 2026-08-18

- #187 established immutable submission and deterministic parity evidence.
- #191–#206 advanced the canonical contract through schema/timing, adapters, ordering, curves, exact-frame property evaluation, and visual FrameState.
- #201 repaired the reachable PDF dependency vulnerability.
- #208 added permanent cross-runtime FrameState diagnostics and corrected its CodeQL finding before merge.

### 2026-08-19 to 2026-08-21

- #209/#212 canonicalized media geometry, source bounds, and FrameState consumption.
- #218 canonicalized perspective projection.
- #219/#226 improved CI dependency/Playwright reliability.
- #220–#229 completed transition placement and all currently authorable transition paint families.
- #237, #241, #242/#243, and #245/#247 defined and consumed effect, text, shape, and cursor canonical states.
- #246 fixed canonical Vitest discovery and corrected newly exposed stale assertions.
- #251 added immutable source provenance and canonical anchor/geometry consumption.
- #252–#255 completed packaged static-font provenance/upload/snapshot/static-face enforcement.

### 2026-08-24 to 2026-08-25

- #260 implemented mirrored Go/TypeScript `audio-graph-v1`, passed complete validation, and squash-merged as `9acc544eac6cc63a14a4e0f22ee52cb07688e010`, completing Phase 2.
- #261 introduced `preview-composition-frame-v1` and canonical deterministic visual activity; squash merge `1fa4a2c9fb0ba02b00a194374dc363fe5f796199`.
- #262 added retained preview-composition parity diagnostics; squash merge `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec`.
- #263 consumed canonical deterministic visual-media source time; squash merge `77b49e096e165ad579d7eb5daed81763c023203c`.
- #264 consumed canonical deterministic transform/opacity including exact-zero scale compatibility; squash merge `357ae16301fc0b7d6c0f0d65e99c0277bac77adb`.
- #265 consumed canonical deterministic camera-relative `view_transform`, passed Quality #1532/Security #1538 plus full parity/platform gates, and squash-merged as `02b0a5d5ce68f0ee46c16e092c525772751d5681`.
- #266 added the fail-closed parity-region manifest boundary, passed Quality #1545/Security #1551 plus the complete exact-head matrix, and squash-merged as `7cf8a82a4081f487e13c2117e1c9176c91266253`.
- #267 consumed deterministic per-layer canonical CSS perspective. Final head `ca54a256ff6872ce65c52f99885bbad5d8ba1cdf` passed Quality #1548, Security #1554, backend race, frontend lint/unit/performance/build, Playwright, deterministic renderer parity, CodeQL, desktop/Helm, plugin lifecycle, and all standalone platform/sandbox assurances. It squash-merged as `8a9343cf42d4e47f5b9d2b459737601c420d097b`.
- #268 projected complete persisted editor asset width/height into temporary canonical `content_bounds`, passed the complete exact-head matrix, and squash-merged as `e498bee75757bdeff3bb3c5b8b35aa3f402265b4`.
- #269 consumed canonical deterministic media geometry and squash-merged as `6a382d7ec19a9fc2616ee2db81ca2fe301ecfb26` after the complete matrix.
- #270 consumed canonical deterministic clip effect state and squash-merged as `0b6c9bf682287dad0983948ee9168a9b70a11479` after the complete matrix.
- #271 added explicit transition placement/peer authoring plus fail-closed renderable-peer/type/overlap validation and squash-merged as `b2c4abc695035c4c2b61bae4554294eebea674aa` after the full matrix.
- #272 consumed exact owner-scoped transition paint and squash-merged as `ffc2ea3ca744262aae2339a85075a749f5fa0b7a` after Quality #1578/Security #1584 and the full platform matrix.
- #273 defined `transition-pair-surface-plan-v1`. Code-bearing head `ac332dc4980c145c4e1eafbdea6f2037b4dfcfda` passed Quality #1583, Security #1589, and the platform assurance matrix. Final docs/scaffolding cleanup head did not schedule fresh Actions; the final six-path tree/review audit was clean and the PR squash-merged as `5a91fc146aed41864a47b38c626a21789ef52437` without claiming exact-final-head CI.
- #274 defined `transition-pair-pixel-composition-v1`; exact final head `f7c5829d68e12f682646c7c1d1043928c266de34` passed Quality #1596, Security #1602, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, CodeQL, desktop/Helm/plugin lifecycle, and every standalone platform/sandbox assurance. Review/thread audit was empty before expected-head squash merge as `c8c09aa42073711a52681c7b2edf907210ab4e05`.
- #275 was created directly from #274's squash result to establish stack-preserving preview transition pair execution slots. Behavioral+tracker head `37b84024b89e13af37f603a913dadebd46053cc2` passed Quality #1599, Security #1605, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, CodeQL, desktop/Helm/plugin lifecycle, and every standalone platform/sandbox assurance; the final documentation-only commit freezes this evidence for merge review.

## Next recommended slice

1. Complete #275 final documentation/tree/review audit and expected-head squash merge.
2. From #275's actual squash result, wire the source-over pair slot into `VideoPreviewCanvas` so adjacent slide/wipe inputs are isolated in-place without changing unrelated canonical stack order; activate this only for a clean all-source-over pair plan so a frame cannot mix exact and deferred pair semantics.
3. Follow with the true weighted Canvas compositor for crossfade/zoom/dip using #274's linear-sRGB premultiplied-alpha contract; do not implement these with CSS opacity or duplicate canonical weights.
4. Separately expose frame-level canonical `scene_effects` without a second canonical frame evaluation, then consume text/shape/cursor painter inputs and deterministic intrinsic text metrics.
5. Follow with normal-playback canonicalization/diagnostics and AudioGraph scheduling as separate reviewable slices.
6. In parallel, use #266's fail-closed input boundary to define the codec-aware Phase 0 production structural policy and add second-platform parity evidence; current global numeric thresholds or arbitrary exact decoded regions alone are not visual sign-off.
