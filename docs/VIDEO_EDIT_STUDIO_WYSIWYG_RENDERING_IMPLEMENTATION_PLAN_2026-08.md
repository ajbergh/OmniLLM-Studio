# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-26  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#280 — Consume canonical scene effects in deterministic preview** — squash merge `612dd2d0dc9b51380530b3a97c6558ec5698cc79` (2026-08-26).

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** The renderer-independent visual contract (`visual-frame-state-v1` and its subcontracts) and audio contract (`audio-graph-v1`) define semantics; current work is migrating real program-monitor consumers onto those already-evaluated decisions in small, reversible slices.

Current implementation PR: **#281 — Consume canonical text, shape, and cursor painters** on branch `feat/video-wysiwyg-phase3-canonical-painters`, created directly from #280's actual squash result `612dd2d0dc9b51380530b3a97c6558ec5698cc79`.

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
- normalized exact head `a485c3cef968411d53f04482ab3f6e5e90d2c6cb` passed Quality #1578, Security #1584, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, CodeQL, desktop/Helm/plugin lifecycle, and all standalone platform/sandbox assurances before expected-head squash merge as `ffc2ea3ca744262aae2339a85075a749f5fa0b7a`.

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

#275 established the preview execution-slot boundary before direct painter wiring:

- deterministic canonical preview layers are converted into replacement slots using #273's exact bottom-to-top pair-surface indices;
- unrelated canonical layers stay in their original positions and non-adjacent pairs remain ungrouped/deferred;
- #274's pair-pixel operator selects `source-over-dom` only for pair-slide/pair-wipe and keeps crossfade/zoom/dip as `weighted-canvas-deferred`;
- source-over pair-slide resolves canonical outgoing/incoming canvas-fraction translations and pair-wipe resolves the canonical incoming layer clip without adding opacity weights;
- free-running or partially canonical preview state stays on the explicit legacy slot path;
- focused Vitest coverage exercises legacy fallback, in-place slot replacement, slide, wipe, weighted deferral, and non-adjacent rejection;
- behavioral+tracker head `37b84024b89e13af37f603a913dadebd46053cc2` passed Quality #1599, Security #1605, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and all standalone Linux/macOS/browser sandbox assurances. Compare was ahead 4 / behind 0 with exactly the tracker, planner, planner tests, and pair-surface input-type narrowing; review submissions and review threads were empty. It squash-merged as `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28`.

#276 consumed the first canonical transition pair plan in the real program monitor:

- deterministic complete canonical frames consume only a clean all-source-over pair plan; weighted, deferred, non-adjacent, mixed, or simultaneous-paint pair frames remain on the established independent-layer fallback;
- eligible adjacent pair-slide/pair-wipe inputs are replaced in-place by one full-stage structural group at the canonical replacement slot, preserving unrelated bottom-to-top ordering;
- pair-slide canvas-fraction translation and pair-wipe layer clipping are applied exactly once on the two pair inputs without introducing opacity weights;
- the pair wrapper preserves direct manipulation and `preserve-3d`; CSS `isolation: isolate` is intentionally not used because that grouping property can flatten 3D descendants;
- stage and pair diagnostic attributes expose plan mode, consumer mode, deferral reasons, pair identity, lower/upper input identity, and per-layer pair-paint mode;
- exact final head `e51cef9284cfa2a6c88f46083421fb71514b54fd` passed Quality #1608, Security #1614, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance. Final compare was ahead 8 / behind 0 with exactly four intended paths and no PR comments, review submissions, or review threads. It squash-merged with expected-head protection as `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d`.

#277 completed the executable weighted pair-pixel math boundary without prematurely wiring a Canvas surface:

- `composeWeightedTransitionPairRgba` accepts Canvas `ImageData`-compatible straight-alpha sRGB byte buffers and a validated `transition-pair-pixel-composition-v1` contract;
- byte RGB inputs use an IEC 61966-2-1 sRGB decode lookup table, composition occurs in linear sRGB with premultiplied accumulation, output alpha is recovered as straight alpha, channels are clamped to the unit interval, then RGB is encoded back to sRGB;
- pair-crossfade and pair-zoom apply canonical outgoing/incoming weights exactly once; between dip-to-black adds the canonical opaque-linear-black alpha contribution without adding RGB energy;
- source-over families, color/alpha contract drift, malformed weights/black metadata, empty/non-RGBA buffers, and unequal input/target buffers fail closed;
- focused Vitest evidence covers the linear-sRGB 50/50 midpoint (`188,188,0` rather than encoded-space `128,128,0`), semitransparent premultiplied accumulation/straight-alpha recovery, dip black, transparent inputs, caller-provided multi-pixel targets, and fail-closed cases;
- exact final head `a3beed0a3bc7c402ff9360e5f78d0485a9c7efb1` passed Quality #1615, Security #1621, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance. Final compare was ahead 3 / behind 0 with exactly the tracker, kernel, and focused kernel tests; PR comments, review submissions, and review threads were empty. It squash-merged with expected-head protection as `4aefa65e4cab7c92ccb32cef486739de7201cc1c`.

#278 completed the weighted preview raster-source capability boundary before Canvas execution:

- weighted pair planning separately classifies canonical pair-surface validity and preview raster-source capability so consumer limitations never masquerade as canonical contract failures;
- only authoritative, unresolved-free image/video sources with canonical `media-geometry-v1` and 2D view transforms are source-eligible;
- canonical text, shapes, cursor paint, clip effects, unsupported/missing media assets, missing geometry, and 3D/perspective transforms are explicitly deferred with per-input reasons;
- pair classification preserves both blocking input reasons and keeps weighted execution as `weighted-canvas-deferred`; decoded video-frame availability and decoder-budget poster substitution remain runtime gates;
- source-over slide/wipe planning and consumption semantics are unchanged;
- exact final head `0a4ebeb0b623cfd39eef844d475d8048c338a973` passed Quality #1619, Security #1625, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance. Final compare was ahead 6 / behind 0 with exactly five intended paths and no PR comments, review submissions, or review threads before expected-head squash merge as `4a70dc7c0669812a699cc42d4c45c3ce142e5335`.

#279 consumes clean weighted transition pairs on an exact Canvas surface in deterministic frame-address mode:

- only a complete all-weighted plan with #278-supported raster sources is admitted; decoder-budget posters, mixed/deferred pair semantics, multiple owner paints, unsupported media, effects, text/shape/cursor paint, missing geometry, and 3D/perspective state remain fail-closed;
- already-mounted image/video elements remain the sole decoded sources, so the Canvas consumer adds no video decoder and no second source-time authority;
- each pair input is rasterized into an isolated full-stage 2D surface using canonical visible source crop, `painted_bounds`, `clip_bounds`, camera-relative 2D translation/rotation, clip transform scale/opacity, anchors, and pair-zoom scale exactly once;
- #277's linear-sRGB straight-alpha/premultiplied kernel performs the final crossfade/zoom/dip accumulation exactly once after isolated input rasterization;
- the result is portaled into the canonical lower replacement slot while the upper peer is hidden, preserving unrelated bottom-to-top stack order; free-running playback and editor interaction remain on the established preview path;
- parity-ready interception waits for every active weighted Canvas surface to settle; source/decode/readback failure never releases a bad screenshot and instead records `weighted-canvas-not-ready` so evidence fails closed;
- the previous interactive preview implementation is preserved byte-for-byte as `VideoPreviewCanvasLegacy.tsx` blob `2dccf0d3bd428a0801b017e39d3ca3e4fe359b19`; GitHub's large textual diff is pathname reuse, not a 1,389-line behavioral rewrite;
- scale ownership was audited against `visual-frame-state-v1`: `view_transform` copies `transform.scale_x/y` unchanged and only camera-adjusts position/rotation, so the Canvas helper's current scale values are contract-equivalent to the established DOM consumer;
- code-bearing head `3d51a47e665c2ec1d3fc46108066b93de7e7d3e6` passed Quality #1627 and Security #1633; exact final head `c90dd38f9bf0b85a60ac64196692f6f365b0e3a4` then passed Quality #1628, Security #1634, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance before expected-head squash merge as `adb820ae08a4450903546e18962ccbb3275c618b`.

#280 consumes frame-level canonical scene effects in deterministic frame-address mode without introducing a second semantic evaluator:

- `queryActiveClipsAtFrameWithState` carries the top-level `CanonicalVisualFrameState` alongside the per-layer canonical states produced by the same strict preview-composition projection;
- synchronous wrapper/legacy frame consumers share one microtask-scoped canonical preview-composition result keyed by the exact timeline object, asset-array object, and frame index, avoiding a duplicate FrameState evaluation while preventing persistent/stale canonical state;
- scene-effect resolution validates `effect-state-v1`, exact `scene` scope, non-empty identity, unique authored order, registry membership, canonical defaults, and the existing CSS-paintable subset; canonical omission remains authoritative zero effects rather than falling back to stale authored state;
- deterministic authoritative frames replace only the existing whole-program stage filter; free-running playback and fail-closed projection fallback retain the established authored scene-effect path;
- focused Vitest coverage proves canonical scene defaults/order, canonical zero-effect authority, malformed scope/order rejection, fail-closed projection fallback, and object-identity reuse across two synchronous wrapper-style indexes;
- exact code-bearing head `ca0dc9b73f4c15199e9af4b9aed7d1e972130b5e` passed Quality #1632, Security #1638, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance;
- the final documentation head passed Quality #1633, Security #1639, and every standalone assurance; the final five-path tree plus PR comments/reviews/threads audit was clean before expected-head squash merge as `612dd2d0dc9b51380530b3a97c6558ec5698cc79`.

#281 consumes canonical text, shape, and cursor painter inputs in deterministic frame-address mode while preserving the established interactive preview:

- the canonical wrapper consumes `text-state-v1`, `shape-state-v1`, and exact rational `cursor-state-v1` from the same already-evaluated per-layer FrameState used by the other deterministic consumers;
- `VideoPreviewCanvasLegacy.tsx` remains the free-running playback/direct-manipulation implementation; canonical painter portals replace only deterministic base text/shape content and cursor paint without widening normal-playback semantics;
- canonical omission is authoritative, and the established media-before-shape-before-text base-content precedence is preserved while cursor canonicalization remains independent;
- canonical text maps evaluated defaults/style, explicit box dimensions, horizontal/vertical alignment, line-height mode, letter spacing, stroke, shadow, padding, and border radius without re-evaluating authored defaults;
- canonical shape paint uses canonical dimensions/style defaults across all current annotation kinds; pixelate remains explicitly marked as a CSS approximation because the DOM preview still cannot perform true raster pixelation;
- canonical cursor paint consumes sampled position, scale, visibility, highlight, click-ring, and strict click-proximity state rather than resampling authored cursor events locally;
- intrinsic text layout without explicit box dimensions remains `browser-intrinsic-deferred`; no deterministic glyph metrics are fabricated, and weighted transition Canvas raster eligibility remains unchanged until painter fidelity is exact;
- a pre-merge audit caught that `visibility:hidden` would leave replaced text/shape flex items participating in layout; corrected head `9e51e7b7f4f28b25e59c1cf56ad114c39d40e278` uses `display:none` for replaced base content with exact restoration while cursor remains visibility-hidden because it is absolutely positioned;
- exact corrected code-bearing head `9e51e7b7f4f28b25e59c1cf56ad114c39d40e278` passed Quality #1636, Security #1642, backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone Linux/macOS/browser sandbox assurance. PR comments, review submissions, and review threads were empty before this documentation-only tracker freeze.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged the fail-closed optional frame-indexed exact-region input boundary. Production structural policy, tolerance-aware codec-region semantics, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#280 merged deterministic activity/source/transform/view/perspective/media-geometry/effect consumption, explicit transition authoring, owner paint, stack-safe pair planning, exact pair-pixel semantics, real source-over slide/wipe consumption, the weighted linear-sRGB byte kernel, fail-closed raster-source classification, weighted Canvas pair execution, and canonical scene effects. #281 consumes deterministic canonical text/shape/cursor painter inputs; deterministic intrinsic text metrics, exact pixelate/raster painter fidelity, weighted raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close decoded visual thresholds for media, transforms, text metrics/fonts, shapes, transitions, effects, cursor, camera, color space, and deterministic asset loading. |
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

`transition-state-v1` owns placement, peer/owner roles, windows, progress, and overlap requirements. `transition-paint-v1` owns all currently authorable paint families: fade, crossfade, dip-to-black, slide, wipe, and zoom. Pair transitions operate on pair surfaces and must not be reinterpreted as independent layer opacity. `transition-pair-surface-plan-v1` owns stack-safe pair grouping/slot replacement eligibility. `transition-pair-pixel-composition-v1` owns pair sample blending: linear-sRGB working values, straight input alpha, premultiplied accumulation, straight output alpha, exact-once canonical weighting for crossfade/zoom/dip, opaque linear black for dip contribution, and canonical source-over stack order for slide/wipe. #275 adds the preview execution-slot boundary. #276 consumes only complete, adjacent, unweighted slide/wipe pair plans as one structural DOM pair group at the canonical replacement slot; deferred, weighted, non-adjacent, or multi-paint frames remain on the independent-layer fallback. The group deliberately avoids CSS `isolation: isolate` because that grouping property can flatten `preserve-3d` descendants. #277 implements the reusable exact weighted RGBA byte kernel for crossfade/zoom/dip. #278 adds the media raster-source boundary. #279 consumes only clean raster-eligible weighted plans in deterministic frame-address mode, reuses already-synchronized media sources, rasterizes each pair input with canonical 2D geometry/transform/opacity, and ports the final weighted Canvas surface into the canonical replacement slot. Unsupported/deferred sources and decoder-budget posters remain explicit fallback/debt rather than approximate pixels.

### Effects, text, shape, and cursor

- `effect-state-v1` preserves enabled authored order, scope, normalized parameters, and exact-frame automation; #270 consumes deterministic clip effect state and #280 consumes deterministic frame-level scene-effect state from the same shared canonical frame evaluation; unsupported metadata fails closed.
- `text-state-v1` serializes text/style intent once. #281 consumes that evaluated state in deterministic preview, but intrinsic glyph measurement without explicit box metrics remains a Phase 3–5 consumer problem and is never guessed. Packaged static font identity remains manifest-backed via `font-resource-provenance-v1`.
- `shape-state-v1` covers all currently authorable annotation kinds and supplies canonical dimensions/style defaults/bounds; #281 consumes those painter inputs, while true raster pixelation remains explicit fidelity debt rather than a CSS claim.
- `cursor-state-v1` owns exact rational sampling, visibility, scale, highlight/click-ring state, and strict `<300ms` click proximity; undefined smoothing fails closed, and #281 consumes the evaluated cursor state rather than locally resampling events.

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

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can look current while the tree silently reverts unrelated work. #261/#262/#264/#265/#266/#267 followed this normalization rule; #268 was created directly from #267's squash result, #269 from #268's actual squash result, #270 from #269's actual squash result, #271 from #270's actual squash result, #272 from #271's actual squash result, #273 from #272's actual squash result, #274 from #273's actual squash result, #275 from #274's actual squash result `c8c09aa42073711a52681c7b2edf907210ab4e05`, #276 from #275's actual squash result `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28`, #277 from #276's actual squash result `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d`, #278 from #277's actual squash result `4aefa65e4cab7c92ccb32cef486739de7201cc1c`, #279 from #278's actual squash result `4a70dc7c0669812a699cc42d4c45c3ce142e5335`, #280 from #279's actual squash result `adb820ae08a4450903546e18962ccbb3275c618b`, and #281 from #280's actual squash result `612dd2d0dc9b51380530b3a97c6558ec5698cc79`.

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
| #275 | Stack-preserving canonical preview pair execution slots | `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28` |
| #276 | Canonical source-over slide/wipe pair consumption in the real preview | `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d` |
| #277 | Reusable weighted linear-sRGB transition-pair RGBA kernel | `4aefa65e4cab7c92ccb32cef486739de7201cc1c` |
| #278 | Fail-closed weighted pair raster-source classification | `4a70dc7c0669812a699cc42d4c45c3ce142e5335` |
| #279 | Deterministic weighted crossfade/zoom/dip Canvas pair execution | `adb820ae08a4450903546e18962ccbb3275c618b` |
| #280 | Canonical deterministic scene-effect state from the shared frame projection | `612dd2d0dc9b51380530b3a97c6558ec5698cc79` |

Current #281 consumes deterministic `text-state-v1`, `shape-state-v1`, and `cursor-state-v1` painter inputs through portals layered around the established interactive preview. It intentionally does not claim deterministic intrinsic glyph metrics, exact pixelate rendering, or weighted-raster source eligibility for those painters.

Remaining Phase 3 sequence should stay reviewable:

1. Complete #281 final tracker/tree/review audit and expected-head squash merge.
2. Define deterministic intrinsic text-metric ownership and close exact painter raster fidelity, including true pixelate behavior where required; broaden the weighted raster capability boundary only after those painter sources are exact.
3. Canonicalize normal playback rather than only parity frame-addressed mode, with diagnostics/rollback and explicit transition-pair runtime observability.
4. Make preview audio scheduling consume `audio-graph-v1` in a separate audio-focused slice.

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
| Pair transitions reorder unrelated layers or become independent alpha layers | #273 admits only active authoritative adjacent pair inputs; #275 converts only those exact replacement indices into preview slots; #276 consumes only clean all-source-over pair plans and preserves the canonical replacement slot. |
| Pair DOM grouping flattens canonical 3D projection | #276 deliberately avoids CSS `isolation: isolate`; the structural pair wrapper preserves `transform-style: preserve-3d` while adjacency provides exact source-over ordering. |
| Pair blend gamma/alpha semantics drift or weights are applied twice | #274 fixes transfer/color/alpha/clamp and exact-once weighted semantics; #277 implements those semantics in one reusable byte kernel. #279 rasterizes isolated inputs before applying that kernel, so DOM/CSS opacity cannot duplicate pair weights. |
| Any visible transition peer is treated as weighted-Canvas-ready | #278 separates canonical pair validity from raster-source capability and fails closed for text, shape, cursor, effects, missing/unsupported media, missing geometry, and 3D/perspective state. #279 adds runtime decoder/poster/readiness gates and refuses mixed/deferred frames; #281 does not broaden this gate until canonical painter raster fidelity is exact. |
| Weighted Canvas capture releases stale or blank evidence | #279 intercepts parity-ready until all active weighted surfaces settle; a source/decode/readback failure records `weighted-canvas-not-ready` and does not release a screenshot-ready event. |
| v1 transition migration guesses placement or peer identity | #271 persists explicit placement/peer intent and validates renderable peer/type/overlap semantics; legacy rows without placement remain `V1_TRANSITION_PLACEMENT_AMBIGUOUS`. |
| Effect/text/shape/cursor defaults drift | #270 makes deterministic clip `effect-state-v1` authoritative, #280 makes deterministic scene `effect-state-v1` authoritative, and #281 consumes deterministic canonical text/shape/cursor painter state; unsupported metadata stays fail-closed. |
| Canonical painter replacement changes interactive layout semantics | #281 is frame-address-only; replaced base content uses `display:none` only while the canonical portal is active and restores the exact prior display value. Free-running/direct manipulation remains legacy-owned. |
| Duplicate canonical preview evaluation diverges top-level and layer semantics | #280 carries top-level FrameState through the same frame query and shares one microtask-scoped strict preview composition across synchronous wrapper/legacy consumers; #281 reuses those per-layer states. |
| Browser/system font fallback changes metrics | Packaged static-face provenance; #281 labels intrinsic measurement `browser-intrinsic-deferred`, and deterministic metric ownership remains explicit Phase 3–5 work. |
| CSS pixelate approximation is mistaken for exact raster behavior | #281 keeps pixelate explicitly approximate and does not widen weighted-raster eligibility until a true raster painter exists. |
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

### 2026-08-24 to 2026-08-26

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
- #275 established stack-preserving preview transition pair execution slots, passed the full exact-head matrix, and squash-merged as `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28`.
- #276 consumed clean source-over slide/wipe pair slots in the real program monitor. Exact final head `e51cef9284cfa2a6c88f46083421fb71514b54fd` passed Quality #1608, Security #1614, Playwright, immutable renderer parity, CodeQL, backend/frontend/platform gates, and every standalone assurance; it squash-merged with expected-head protection as `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d`.
- #277 exact final head `a3beed0a3bc7c402ff9360e5f78d0485a9c7efb1` passed Quality #1615, Security #1621, Playwright, immutable renderer parity, CodeQL, backend/frontend/platform gates, and every standalone assurance; it squash-merged with expected-head protection as `4aefa65e4cab7c92ccb32cef486739de7201cc1c`.
- #278 exact final head `0a4ebeb0b623cfd39eef844d475d8048c338a973` passed Quality #1619, Security #1625, Playwright, immutable renderer parity, CodeQL, backend/frontend/platform gates, and every standalone assurance; it squash-merged with expected-head protection as `4a70dc7c0669812a699cc42d4c45c3ce142e5335`.
- #279 exact final head `c90dd38f9bf0b85a60ac64196692f6f365b0e3a4` passed Quality #1628, Security #1634, Playwright, immutable renderer parity, CodeQL, backend/frontend/platform gates, and every standalone assurance; it squash-merged with expected-head protection as `adb820ae08a4450903546e18962ccbb3275c618b`.
- #280 exact code-bearing head `ca0dc9b73f4c15199e9af4b9aed7d1e972130b5e` consumed deterministic canonical scene-effect state through the existing program-stage filter while sharing the strict frame projection; its final documentation head also passed the complete matrix and it squash-merged with expected-head protection as `612dd2d0dc9b51380530b3a97c6558ec5698cc79`.
- #281 was created directly from #280's actual squash result. Corrected code-bearing head `9e51e7b7f4f28b25e59c1cf56ad114c39d40e278` consumes deterministic canonical text/shape/cursor painter inputs while preserving the legacy interactive path and passed Quality #1636, Security #1642, backend race, frontend lint/unit/performance/build, Playwright, immutable renderer parity, both CodeQL languages, desktop/Helm/plugin lifecycle, and every standalone platform/sandbox assurance before this documentation-only tracker freeze.

## Next recommended slice

1. Complete #281 final tracker/tree/review audit and expected-head squash merge without changing the validated behavioral files.
2. From #281's actual squash result, define deterministic intrinsic text-metric ownership and close exact painter raster fidelity, including true pixelate behavior where needed.
3. Broaden the weighted raster capability boundary only after canonical text/shape/cursor painter sources are exact and can be rasterized without introducing a second semantic authority.
4. Follow with normal-playback canonicalization plus diagnostics/rollback as a separate slice; keep frame-addressed fail-closed behavior intact while broadening runtime coverage.
5. Make preview audio scheduling consume `audio-graph-v1` in a separate audio-focused slice.
6. In parallel, use #266's fail-closed input boundary to define the codec-aware Phase 0 production structural policy and add second-platform parity evidence; current global numeric thresholds or arbitrary exact decoded regions alone are not visual sign-off.