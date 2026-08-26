# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-26  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#282 — Bind editor font resources for canonical preview** — squash merge `38ea95aba65207f9de505357d02bf5dbc93c89be` (2026-08-26).

Current implementation PR: **#283 — Gate deterministic text on exact browser font faces** on branch `feat/video-wysiwyg-phase3-font-face-readiness`, created directly from #282's actual squash result `38ea95aba65207f9de505357d02bf5dbc93c89be`.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** The renderer-independent visual contract (`visual-frame-state-v1` and subcontracts) and audio contract (`audio-graph-v1`) own semantics. Phase 3 is moving real program-monitor consumers onto those decisions in small, reversible slices.

### #283 current result

#283 closes browser face readiness for resource-bound deterministic text without claiming glyph-layout parity:

- `preview-font-face-readiness-v1` binds a canonical text `font_resource_id` to the exact mutable editor `VideoAsset` delivered by #282.
- The browser downloads the exact current asset through the existing authenticated video-asset path and registers it through `FontFace` / `document.fonts`.
- Browser registration uses a collision-safe preview-only family alias derived from resource ID, asset ID, and canonical text weight. The alias cannot accidentally select an installed/system/web font with the authored family name.
- `text-state-v1.font_face_source` remains unchanged. Mutable editor loading never claims immutable `packaged-resource` provenance, hashes, staged paths, or static-face identity.
- Resource-backed deterministic text is not painted until all required exact faces for the frame are ready. Missing/mismatched bindings, invalid CSS weights, download/load failures, empty bytes, or unavailable browser font APIs fail closed.
- Failed loader entries are evicted so later deterministic frames may retry after transient fetch/browser failures; concurrent requests for the same binding are deduplicated.
- Embedded canonical shape text is gated only for the shape kinds that actually paint text: `step_marker`, `speech_bubble`, and `label`.
- `omnillm:video-parity-ready` is intercepted until required font faces settle. The font resume flag composes with #279's weighted-Canvas readiness gate rather than bypassing it; timeout/failure records explicit stage diagnostics and never releases a bad screenshot.
- Canonical text metrics remain `browser-intrinsic-deferred`; #283 does not create a second `measureText` authority or claim deterministic glyph shaping/layout.
- Timeline text currently owns `font_weight` but not `font_style`; #283 therefore binds the exact bytes under the canonical authored weight and does not invent new italic/style semantics.
- Focused Vitest coverage directly exercises resource binding, collision-safe aliases, invalid bindings/weights, exact-byte load deduplication, failure retry, painter alias injection without provenance mutation, and text-bearing shape classification.
- The existing `parity-torture-v1` fixture contains no font asset/resource binding. Playwright and immutable parity therefore serve as whole-preview regression gates for #283; they are not represented as direct execution of the resource-font branch.
- Exact code-bearing head `76c1ee3bdb0db3be700b2a500c392c2d49a265d8` passed **Quality #1646** and **Security #1651**. Executed green gates include backend formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright smoke, immutable renderer parity, dependency audits, both Go and JavaScript/TypeScript CodeQL, Windows desktop/sandbox/plugin lifecycle, Helm, and macOS confinement.
- The same exact head passed the standalone Linux workspace/quota, macOS runtime/adversarial/extension, and browser-egress assurance workflows.
- The first close/reopen workflow wave was superseded by GitHub concurrency and produced cancelled jobs; only the newer fully executed #1646/#1651 wave is validation evidence.
- Code-bearing compare is **ahead 5 / behind 0** from #282's squash merge with exactly five intended frontend/test paths. PR comments, review submissions, and review threads were empty before this tracker freeze.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged the fail-closed frame-indexed region-policy input boundary. Production structural policy, codec-aware decoded-region semantics, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#282 merged deterministic activity/source/transform/view/perspective/media-geometry/effects/transitions, canonical text/shape/cursor painters, and mutable editor font-resource identity. #283 adds exact browser face readiness. Deterministic intrinsic browser text layout, exact pixelate/raster painter fidelity, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
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

Canonical evaluators are pure, deterministic, serializable, free of browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed whenever an authorable value lacks explicit semantics.

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Existing preview behavior remains an interaction/compatibility target until each Phase 3 consumer moves onto canonical state.

### Media geometry and source provenance

`media-geometry-v1` owns media geometry. Source dimensions come from explicit content bounds or immutable source provenance, never from canvas size, DOM boxes, decoded element dimensions, or browser `object-fit`. Source crop precedes fit; output crop follows fit. Deterministic media consumption uses canonical `painted_bounds` and canvas-space `clip_bounds`.

### Perspective and stacking

Track/z-index order remains stacking authority; spatial `z` affects projection. `perspective-projection-v1` serializes projection separately from camera-relative transforms. #267 consumes per-layer canonical perspective; free-running playback and incomplete canonical frames keep explicit legacy fallback.

### Transitions and pixel composition

`transition-state-v1` owns placement/peer roles/windows/progress; `transition-paint-v1` owns authorable paint families. Pair transitions are pair surfaces, never independent alpha layers.

- #273 defines stack-safe pair grouping.
- #274 defines exact linear-sRGB pair-pixel composition.
- #275 defines preview execution slots.
- #276 consumes source-over slide/wipe pairs.
- #277 implements the weighted linear-sRGB RGBA kernel.
- #278 separates canonical pair validity from preview raster-source capability.
- #279 executes clean weighted crossfade/zoom/dip pairs on Canvas and gates parity-ready until surfaces settle.

Unsupported/deferred sources stay explicit fallback/debt; they are never approximated as canonical pixels.

### Effects, text, shapes, cursor, and fonts

- `effect-state-v1` owns enabled authored order, scope, normalized parameters, and exact-frame automation. #270 consumes clip effects; #280 consumes scene effects from the same shared frame projection.
- `text-state-v1` owns text/style intent. #281 consumes its painter state.
- #282 verifies authored `font_resource_id` against exactly one current editor font asset without claiming immutable provenance.
- #283 loads that exact mutable asset into Chromium under an isolated browser alias and blocks deterministic evidence until the face is ready.
- Immutable static-font identity remains Render Manifest-backed by `font-resource-provenance-v1`; browser/editor readiness and immutable provenance are separate claims.
- Intrinsic browser glyph layout is still deferred. Do not introduce an independent Canvas `measureText` or FFmpeg-derived approximation as a third authority.
- `shape-state-v1` owns shape dimensions/style defaults/bounds; #281 consumes them. True raster pixelation remains explicit fidelity debt.
- `cursor-state-v1` owns exact rational sampling, visibility, scale, highlight/click-ring state, and strict click proximity; #281 consumes evaluated cursor state.

### AudioGraph v1

`audio-graph-v1` owns deterministic 48 kHz stereo sample boundaries, stable clip nodes, mute/solo precedence, pitch-preserving playback rate, channel handling, gain/automation, minimum-overlap fades, non-normalizing summation, and one post-mix program-processing boundary. Phase 6 makes preview/Chromium/export consumers obey the graph exactly.

## Phase 2 foundation history

| PR(s) | Capability |
|---|---|
| #187 | Immutable render submission and deterministic parity baseline |
| #191–#206 | Timeline v2 / Render Manifest v1, adapters, frame/range/source/order, curves, exact-frame properties, visual FrameState |
| #208 | Permanent Go↔TypeScript FrameState parity diagnostics |
| #209/#212 | Canonical media fit/crop/source-bounds geometry and FrameState consumption |
| #218 | Canonical perspective projection |
| #220–#229 | Canonical transition placement and all authorable transition paint families |
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
| #261 | `preview-composition-frame-v1`; deterministic activity/identity binding | `1fa4a2c9fb0ba02b00a194374dc363fe5f796199` |
| #262 | Retained preview-composition parity diagnostics | `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec` |
| #263 | Canonical visual-media source time | `77b49e096e165ad579d7eb5daed81763c023203c` |
| #264 | Canonical transform/opacity | `357ae16301fc0b7d6c0f0d65e99c0277bac77adb` |
| #265 | Canonical camera-relative `view_transform` | `02b0a5d5ce68f0ee46c16e092c525772751d5681` |
| #267 | Canonical per-layer perspective | `8a9343cf42d4e47f5b9d2b459737601c420d097b` |
| #268 | Persisted editor bounds projected into preview media geometry | `e498bee75757bdeff3bb3c5b8b35aa3f402265b4` |
| #269 | Canonical media painted/clip bounds consumption | `6a382d7ec19a9fc2616ee2db81ca2fe301ecfb26` |
| #270 | Canonical clip effect state | `0b6c9bf682287dad0983948ee9168a9b70a11479` |
| #271 | Explicit v1 transition placement/peer authoring | `b2c4abc695035c4c2b61bae4554294eebea674aa` |
| #272 | Canonical owner-scoped transition paint | `ffc2ea3ca744262aae2339a85075a749f5fa0b7a` |
| #273 | Stack-safe transition pair-surface planning | `5a91fc146aed41864a47b38c626a21789ef52437` |
| #274 | Exact transition pair-pixel composition | `c8c09aa42073711a52681c7b2edf907210ab4e05` |
| #275 | Stack-preserving preview pair slots | `d8f32ba6a4eebc98555ef2bb7e7bbd73f9e98c28` |
| #276 | Source-over slide/wipe pair consumption | `2c69254cc82c3bf4b96a96454b00b9a6ea1c255d` |
| #277 | Weighted linear-sRGB pair RGBA kernel | `4aefa65e4cab7c92ccb32cef486739de7201cc1c` |
| #278 | Fail-closed weighted-pair raster-source classification | `4a70dc7c0669812a699cc42d4c45c3ce142e5335` |
| #279 | Weighted crossfade/zoom/dip Canvas execution | `adb820ae08a4450903546e18962ccbb3275c618b` |
| #280 | Canonical deterministic scene effects | `612dd2d0dc9b51380530b3a97c6558ec5698cc79` |
| #281 | Canonical text/shape/cursor painter inputs | `a5598fdf93767122c21697113e0540706afff145` |
| #282 | Mutable editor font-resource identity/binding | `38ea95aba65207f9de505357d02bf5dbc93c89be` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Identify only the intended child delta.
3. Create the child directly from the actual parent squash SHA.
4. Verify `compare main...branch` contains only intended paths.
5. Update this tracker on the clean branch.
6. Validate the exact final head when Actions schedules; never call an unexecuted head green.
7. Squash-merge with expected-head protection and create the next slice from that actual result.

Never manufacture ancestry by grafting a stale feature tree onto a newer parent. #225 demonstrated that ancestry can appear current while silently reverting unrelated work.

Recent lineage: #279 from #278 squash `4a70dc7c...`; #280 from #279 `adb820ae...`; #281 from #280 `612dd2d0...`; #282 from #281 `a5598fdf...`; **#283 directly from #282 squash `38ea95aba65207f9de505357d02bf5dbc93c89be`**.

## Phase 0 parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio. It currently does **not** include a project font resource, so it must not be cited as direct coverage of #283's resource-font loader.

Retained evidence established exact frame/audio/delivery identity mechanics. The visual baseline remains diagnostic, not production sign-off. Remaining Phase 0 work is to freeze a structural policy that separates zero-tolerance canonical identity/geometry from codec-aware decoded thresholds and to retain evidence on a second supported OS/FFmpeg environment.

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

# when visual/audio/worker/delivery behavior is touched
npm run test:smoke
```

Hosted CI is authoritative for platform/toolchain cases unavailable in the current execution environment. Setup-only stalls, concurrency cancellations, and unscheduled connector-authored commits are recorded explicitly and never represented as passing.

## Risk register

| Risk | Control |
|---|---|
| Schema/type drift | Go reflection and TypeScript compile/Vitest checks; canonical frontend suites run in CI. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState/AudioGraph diagnostics. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed. |
| Preview silently mixes canonical/fallback semantics | Canonical state is attached only after strict projection; each consumer has an explicit fallback boundary. |
| Stale stacked tree reverts unrelated work | Build each slice from the actual parent squash SHA and inspect the full compare. |
| Duplicate preview evaluation diverges semantics | #280 shares one microtask-scoped strict preview composition across synchronous consumers. |
| Browser/system font fallback changes glyphs/metrics | #282 requires exact resource identity; #283 loads exact bytes under an isolated alias and blocks evidence until ready. Intrinsic layout remains deferred. |
| Mutable editor face is mistaken for immutable packaged provenance | #282/#283 never change `font_face_source`; only Render Manifest provenance may claim `packaged-resource`. |
| Font-face loading leaks fallback pixels into evidence | #283 suppresses resource text until ready and intercepts parity-ready on loading/failure/timeout. |
| Font readiness bypasses weighted Canvas readiness | #283 and #279 use independent resume flags; each resumed event still traverses the other active gate. |
| CSS pixelate is mistaken for exact raster behavior | Pixelate remains explicit approximation and blocks weighted-raster broadening. |
| Weighted transition gamma/alpha drift | #274 defines exact semantics; #277/#279 implement them in one linear-sRGB kernel path. |
| Codec-noisy decoded equality is treated as structural parity | Phase 0 keeps canonical structural policy separate from decoded codec-aware evidence. |
| CI scheduling hides code state | Only executed checks count; cancelled/startup/unscheduled runs are classified explicitly. |

## Next recommended slice

1. Complete #283 final tracker/tree/review audit and expected-head squash merge without changing the validated behavioral files.
2. From #283's actual squash result, define **deterministic Chromium text-layout ownership** after exact font readiness. Use one explicit DOM/layout snapshot boundary at canvas scale rather than introducing `canvas.measureText` or FFmpeg `text_w/text_h` as a third semantic authority.
3. Version/diagnose the layout snapshot inputs and outputs needed by deterministic painters (intrinsic width/height, explicit box behavior, wrapping/alignment/padding) while keeping renderer-independent text intent separate from browser shaping results.
4. Close exact pixelate/raster painter fidelity in its own slice; broaden weighted-raster eligibility for text/shape/cursor only after those painter sources are exact.
5. Follow with normal-playback canonicalization plus diagnostics/rollback.
6. Make preview audio scheduling consume `audio-graph-v1` in a separate audio-focused slice.
7. In parallel, use #266's fail-closed policy boundary to finish Phase 0 structural/codec-aware policy and second-platform evidence.
