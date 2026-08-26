# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-26  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#281 — Consume canonical text, shape, and cursor painters** — squash merge `a5598fdf93767122c21697113e0540706afff145` (2026-08-26).

Current implementation PR: **#282 — Bind editor font resources for canonical preview** on branch `feat/video-wysiwyg-phase3-text-layout`, created directly from #281's actual squash result `a5598fdf93767122c21697113e0540706afff145`.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** The renderer-independent visual contract (`visual-frame-state-v1` and subcontracts) and audio contract (`audio-graph-v1`) own semantics. Phase 3 is migrating actual program-monitor consumers onto those already-evaluated decisions in small, reversible slices.

### #282 current result

#282 closes the mutable editor font-resource identity gap before browser face loading and intrinsic text measurement:

- `editor-font-resource-binding-v1` resolves authored `font_resource_id` values only against current project assets with `kind: font`, canonical lowercase resource-ID grammar, and exact metadata identity.
- Missing resources and duplicate declarations fail closed before canonical preview composition; malformed/legacy metadata is ignored rather than silently accepted.
- Plain Timeline v2 evaluation remains fail-closed when no font-resource context exists. An unverifiable `font_resource_id` still cannot become authoritative FrameState.
- Immutable Render Manifest evaluation remains the only path that can claim `font_face_source: packaged-resource`. Editor binding proves current mutable resource availability only and never fabricates staged paths, hashes, immutable face provenance, or glyph metrics.
- Editor diagnostics temporarily remove only a verified resource reference for canonical evaluation and then restore the authored identity while leaving `font_face_source` renderer-dependent.
- `preview-composition-frame-v1` binds the exact `VideoAsset` as `font_asset`; `timelineIndex.ts` carries that exact object as `fontAsset` into the real frame-addressed program monitor.
- A pre-CI audit caught and fixed an initial integration gap where preview composition exposed `font_asset` but `timelineIndex.ts` discarded it.
- Focused Vitest coverage exercises exact binding, missing/duplicate/malformed metadata, no-context fail-closed behavior, diagnostics, preview composition, and indexed-clip propagation.
- Exact code-bearing head `f96f91fc39011f5a052beeaded6ac02db3d8d276` passed **Quality #1642** and **Security #1647**. Executed green gates include backend gofmt/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright smoke, immutable renderer parity, dependency audits, and both Go and JavaScript/TypeScript CodeQL.
- The same exact head passed Linux workspace/quota, macOS runtime/adversarial/extension, and browser-egress standalone assurances.
- Exact code-bearing compare was **ahead 4 / behind 0** from #281's squash merge with exactly eight intended frontend/test paths. PR comments, review submissions, and review threads were empty before this documentation-only tracker freeze.
- GitHub initially created no #282 workflow records during a runner/scheduling disturbance. Once scheduling recovered, the exact `f96f91fc...` head executed the gates above successfully; the earlier absence of runs was never represented as passing evidence.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged the fail-closed frame-indexed region-policy input boundary. Production structural policy, codec-aware decoded-region semantics, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#281 merged deterministic activity/source/transform/view/perspective/media-geometry/effects/transitions and canonical text/shape/cursor painters. #282 closes mutable editor font-resource identity/binding before browser face loading. Browser face readiness, deterministic intrinsic text metrics, exact pixelate/raster painter fidelity, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close decoded visual thresholds for media, transforms, text metrics/fonts, shapes, transitions, effects, cursor, camera, color space, and deterministic asset loading. |
| Phase 6 — Audio parity closure | Not started | Make preview/Chromium/export obey AudioGraph exactly, including pitch, gain/fades, channels, program processing, processed stems, and decoded delivery. |
| Phase 7 — Rollout and legacy retirement | Not started | Shadow comparison, staged default switch, rollback, telemetry, capability/docs updates, and eventual legacy-composition retirement. |

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

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Existing preview behavior remains an interaction/compatibility target until each Phase 3 consumer moves onto canonical state.

### Media geometry and source provenance

`media-geometry-v1` owns media geometry:

- source aspect ratio requires explicit `content_bounds` or immutable `source-provenance-v1` media-probe state;
- editor preview may project persisted `VideoAsset.width` / `height` into temporary canonical `content_bounds`, but that is **not** immutable source provenance;
- source dimensions are never guessed from output canvas, CSS boxes, decoded DOM elements, or `object-fit`;
- source crop precedes fit; output crop follows fit;
- `contain`, `cover`, `fill`, and `none` have canonical meaning;
- deterministic media consumption places decoded pixels from canonical `painted_bounds` and clips through canvas-space `clip_bounds`.

### Perspective and stacking

Track/z-index order remains stacking authority; spatial `z` affects projection. `perspective-projection-v1` serializes projection separately from camera-relative model transforms. #267 consumes per-layer canonical perspective using full-stage perspective contexts; free-running playback and incomplete canonical frames retain the explicit legacy fallback.

### Transitions and pixel composition

`transition-state-v1` owns placement/peer roles/windows/progress. `transition-paint-v1` owns all authorable paint families. Pair transitions are pair surfaces, never independent alpha layers.

- #273 defines stack-safe pair grouping.
- #274 defines exact linear-sRGB pair-pixel composition.
- #275 defines preview pair execution slots.
- #276 consumes clean source-over slide/wipe pairs.
- #277 implements the weighted linear-sRGB RGBA kernel.
- #278 separates canonical pair validity from preview raster-source capability.
- #279 executes clean weighted crossfade/zoom/dip pairs on Canvas and gates parity-ready until active surfaces are settled.

Unsupported/deferred sources remain explicit fallback/debt; they are never approximated as canonical pixels.

### Effects, text, shapes, cursor, and fonts

- `effect-state-v1` preserves enabled authored order, scope, normalized parameters, and exact-frame automation. #270 consumes clip effects; #280 consumes scene effects from the same shared strict frame projection.
- `text-state-v1` serializes text/style intent once. #281 consumes evaluated text painter state.
- #282 verifies authored `font_resource_id` against exactly one current editor font asset without claiming immutable provenance.
- Packaged static-font identity remains Render Manifest-backed through `font-resource-provenance-v1`.
- Browser face loading/readiness and intrinsic glyph measurement are separate consumer responsibilities. A resource-backed face must not silently resolve to a system/web-font fallback.
- `shape-state-v1` supplies canonical shape dimensions/style defaults/bounds; #281 consumes its painter state. True raster pixelation remains explicit debt rather than a CSS fidelity claim.
- `cursor-state-v1` owns exact rational sampling, visibility, scale, highlight/click-ring state, and strict `<300ms` click proximity; #281 consumes evaluated cursor state instead of resampling events locally.

### AudioGraph v1

`audio-graph-v1` is the renderer-independent audio contract merged in #260:

1. Output is exactly 48,000 Hz stereo.
2. Timeline/range boundaries become deterministic integer sample boundaries.
3. Audio-capable clips retain stable node identity even when suppressed.
4. Track mute, clip mute, and solo selection have explicit precedence/reason identity.
5. Playback rate explicitly preserves pitch.
6. Mono duplicates to stereo; stereo passes through; unsupported layouts fail closed.
7. Base gain and volume automation are finite, deterministic, and authored-order stable.
8. Volume automation is the property envelope (`automation-overrides-base`).
9. Fades are linear and overlapping fade envelopes combine with `minimum`.
10. Summation is `sum-no-normalize`.
11. `render_audio_processing` is one post-mix program operation represented by a processed-stem boundary.
12. Unknown processing fields, unsupported layouts/modes, missing probes, and invalid numeric domains fail closed.
13. Phase 6 makes Web Audio/Chromium/export obey the graph and validates decoded delivery.

## Phase 2 foundation history

| PR(s) | Capability |
|---|---|
| #187 | Immutable render submission and deterministic parity baseline |
| #191–#206 | Timeline v2 / Render Manifest v1, adapters, frame/range/source/order, curves, exact-frame properties, visual FrameState |
| #208 | Permanent Go↔TypeScript FrameState parity diagnostics |
| #209/#212 | Canonical media fit/crop/source-bounds geometry and FrameState consumption |
| #218 | Canonical perspective projection |
| #220–#229 | Canonical transition placement and all currently authorable transition paint families |
| #237 | `effect-state-v1` |
| #241 | `text-state-v1` |
| #242/#243 | `shape-state-v1` |
| #245/#247 | `cursor-state-v1` |
| #251 | Immutable source provenance and canonical anchor/geometry consumption |
| #252–#255 | Font-resource provenance, authored binding, upload/storage/snapshot packaging, static-face enforcement |
| #260 | `audio-graph-v1`; Phase 2 completion |

## Phase 3 merged consumer sequence

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
| #271 | Explicit v1 transition placement/peer authoring | `b2c4abc695035c4c2b61bae4554294eebea674aa` |
| #272 | Canonical owner-scoped transition paint consumption | `ffc2ea3ca744262aae2339a85075a749f5fa0b7a` |
| #273 | Stack-safe canonical transition pair-surface planning | `5a91fc146aed41864a47b38c626a21789ef52437` |
| #274 | Exact canonical transition pair-pixel composition | `c8c09aa42073711a52681c7b2edf907210ab4e05` |
| #275 | Stack-preserving canonical preview pair execution slots | `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28` |
| #276 | Canonical source-over slide/wipe pair consumption | `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d` |
| #277 | Reusable weighted linear-sRGB pair RGBA kernel | `4aefa65e4cab7c92ccb32cef486739de7201cc1c` |
| #278 | Fail-closed weighted-pair raster-source classification | `4a70dc7c0669812a699cc42d4c45c3ce142e5335` |
| #279 | Weighted crossfade/zoom/dip Canvas pair execution | `adb820ae08a4450903546e18962ccbb3275c618b` |
| #280 | Canonical deterministic scene effects from the shared frame projection | `612dd2d0dc9b51380530b3a97c6558ec5698cc79` |
| #281 | Canonical deterministic text/shape/cursor painter inputs | `a5598fdf93767122c21697113e0540706afff145` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual current `main` tree** after its parent squash-merges:

1. Read current `main` commit/tree.
2. Identify only the intended child delta.
3. Build the child directly on current `main`.
4. Verify `compare main...branch` contains only intended paths.
5. Update this tracker on the clean branch.
6. Validate the exact final head before merge.

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can look current while the tree silently reverts unrelated work.

Recent lineage: #275 from #274 squash `c8c09aa...`; #276 from #275 `d8f32ba6...`; #277 from #276 `2c69254c...`; #278 from #277 `4aefa65e...`; #279 from #278 `4a70dc7c...`; #280 from #279 `adb820ae...`; #281 from #280 `612dd2d0...`; **#282 directly from #281 squash `a5598fdf93767122c21697113e0540706afff145`**.

## Phase 0 — Reproducible parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains a known-mismatch diagnostic baseline, not a production threshold.

Current defaults define production-like numeric tolerances (`channel <= 2`, pixel pass rate `>= 0.999`, SSIM `>= 0.995`) and support exact structural regions. #266 added a strict optional policy input bound by canonical integer frame index. Literal decoded RGBA equality is not a sound general policy for codec-affected areas, so exact-region support remains a mechanism rather than Phase 0 sign-off.

Remaining Phase 0 work:

1. Freeze a production structural policy separating zero-tolerance canonical structure/identity from codec-aware decoded-region thresholds.
2. Keep `audio-graph-v1` as the explicit audio boundary until Phase 6 consumers land.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable through Phases 3–7.

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

Hosted CI is authoritative for platform/toolchain cases not reproducible in the current execution environment. Setup-only stalls, runner-capacity queues, and unscheduled connector-authored commits are recorded explicitly and never represented as passing.

Before every merge:

1. Verify the exact final PR head.
2. `compare main...branch` and inspect every changed path.
3. Resolve all review threads/comments.
4. Record concrete validation evidence in this tracker/PR.
5. Never call an unexecuted or setup-only job green.
6. Squash-merge with expected-head protection.
7. Build the next slice from the actual squash result.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest checks; canonical frontend tests run in CI. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState/AudioGraph diagnostics. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed. |
| Preview binds wrong editor object | Track/clip positional and ID identity are verified before canonical state is exposed. |
| Canonical/fallback semantics silently mix | Canonical state attaches only when strict projection succeeds; every consumer keeps an explicit fallback boundary. |
| Source aspect ratio/geometry is guessed | Missing/invalid bounds remain unresolved; canvas/DOM dimensions are never substituted. |
| Camera/perspective is applied twice | Canonical `view_transform` and per-layer perspective own deterministic projection; local recomputation is fallback-only. |
| Pair transitions reorder unrelated layers | Pair planning admits only active authoritative adjacent inputs and replacement slots preserve canonical stack order. |
| Pair gamma/alpha semantics drift or weights apply twice | #274/#277 own exact linear-sRGB/premultiplied math; #279 rasterizes isolated inputs before the kernel. |
| Weighted Canvas admits unsupported sources | #278 separates canonical pair validity from raster-source capability; #279 adds decoder/poster/readiness gates. |
| Weighted Canvas releases stale/blank evidence | #279 withholds parity-ready until active weighted surfaces settle. |
| Effect/text/shape/cursor defaults drift | #270/#280/#281 consume evaluated canonical state rather than re-evaluating authored defaults. |
| Mutable editor font binding is mistaken for immutable provenance | #282 proves current asset identity/availability only; only verified Render Manifest font resources may claim `packaged-resource`. |
| Resource-backed text silently uses a system/web font | Next slice registers the exact bound bytes under a collision-safe preview-only family alias and gates readiness/failure explicitly. |
| Browser font loading is treated as deterministic glyph metrics | Face readiness and text layout/measurement remain separate authorities/slices. |
| CSS pixelate approximation is mistaken for exact raster behavior | Pixelate stays explicitly approximate until true raster painting lands. |
| Audio pitch/fade/program-processing diverges | AudioGraph owns pitch preservation, fade combination, summation, and post-mix processing location. |
| Structural parity is claimed from codec-noisy equality | Phase 0 separates canonical structure/identity from codec-aware decoded evidence. |
| Stacked branch carries a stale tree | Rebuild from actual `main` and inspect compare before every PR. |
| CI scheduling hides code state | Distinguish queued/not-scheduled/startup-failure from executed checks and record exact validated SHA. |
| Browser worker resource cost | Admission control, health checks, cancellation, guarded rollout, FFmpeg retained for media I/O. |

## Implementation log

### 2026-08-17 to 2026-08-21

- #187 established immutable render submission and deterministic parity evidence.
- #191–#206 advanced Timeline v2 / Render Manifest v1 through timing, ordering, curves, exact-frame properties, and visual FrameState.
- #208 added permanent cross-runtime FrameState diagnostics.
- #209/#212 canonicalized media geometry/source bounds.
- #218 canonicalized perspective projection.
- #220–#229 completed transition placement and authorable paint families.
- #237, #241, #242/#243, and #245/#247 defined effect, text, shape, and cursor states.
- #251 added immutable source provenance.
- #252–#255 completed packaged static-font provenance/upload/snapshot/static-face enforcement.

### 2026-08-24 to 2026-08-26

- #260 completed Phase 2 with `audio-graph-v1`; squash `9acc544eac6cc63a14a4e0f22ee52cb07688e010`.
- #261–#268 established strict preview composition, diagnostics, source time, transform/view/perspective consumption, and editor content-bound projection.
- #269–#280 incrementally consumed canonical media geometry, clip/scene effects, explicit transitions, stack-safe pair semantics, exact pair pixels, source-over and weighted pair execution, and shared scene effects.
- #281 consumed canonical text/shape/cursor painter inputs. Corrected code-bearing head `9e51e7b7f4f28b25e59c1cf56ad114c39d40e278` passed Quality #1636, Security #1642, the full backend/frontend/browser/parity/CodeQL/platform matrix, and squash-merged with expected-head protection as `a5598fdf93767122c21697113e0540706afff145`.
- A later #281 merged-head retry wave encountered zero-job runner startup failures; these were infrastructure failures, not passing evidence and not code regressions.
- #282 was created directly from #281's actual squash. Exact code-bearing head `f96f91fc39011f5a052beeaded6ac02db3d8d276` passed Quality #1642, Security #1647, backend/frontend/Playwright/parity/CodeQL and standalone Linux/macOS/browser assurances. Its eight-path ahead-4/behind-0 tree and PR discussion/review/thread audits were clean before the tracker freeze.

## Next recommended slice

1. Complete #282 final documentation/tree/review audit and expected-head squash merge without changing validated behavioral files.
2. From #282's actual squash result, implement deterministic browser `FontFace` loading/readiness for resource-backed text using the exact bound `fontAsset`, a collision-safe preview-only family alias, explicit failure state, and readiness gating before parity-ready evidence.
3. After exact face readiness exists, define deterministic intrinsic text layout/metric ownership.
4. Close true pixelate/raster painter fidelity, then broaden weighted-raster eligibility only for painter sources that are exact.
5. Canonicalize normal playback with diagnostics/rollback while keeping frame-addressed fail-closed behavior intact.
6. Make preview audio scheduling consume `audio-graph-v1` in a separate audio-focused slice.
7. In parallel, finish Phase 0 codec-aware structural policy and second-platform parity evidence.
