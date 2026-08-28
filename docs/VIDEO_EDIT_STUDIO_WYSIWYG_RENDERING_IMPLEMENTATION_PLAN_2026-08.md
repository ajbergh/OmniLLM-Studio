# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-27  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#284 — Own deterministic text layout with Chromium snapshots** — squash merge `7884fef888eadd95a1dd575470e06125d5ec618d` (2026-08-27).

Current implementation branch: **`feat/video-wysiwyg-phase3-pixelate-raster`**, created directly from #284's actual squash result `7884fef888eadd95a1dd575470e06125d5ec618d`. This slice establishes deterministic pixelate raster-grid/kernel semantics before changing the existing preview painter.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** Renderer-independent contracts own authored semantics. Browser/FFmpeg consumers may produce renderer-specific evidence, but they must not silently redefine canonical intent.

### #284 merged result

#284 established one explicit Chromium DOM/layout snapshot boundary after font readiness instead of introducing a competing text-measurement engine:

- `preview-text-layout-snapshot-v1` records browser shaping/layout output in canonical canvas-pixel units while leaving `text-state-v1` unchanged.
- Snapshot inputs/diagnostics include text content, box mode, font face provenance/runtime, browser family/weight, font size, literal `line-height` mode/value, letter spacing, alignment, whitespace, padding, and whether width/height were authored.
- Snapshot outputs include border-box width/height, hard-line count, browser line-fragment count, and whether soft wrapping occurred.
- `line-height: normal` remains literal. #284 does not invent a numeric multiplier that Chromium does not expose.
- Intrinsic dimensions are frozen from the first Chromium layout result; a second animation-frame layout pass must retain the same width/height and line-fragment count before parity-ready resumes.
- The text-layout readiness listener registers before the React font/weighted-Canvas listeners. It waits for #283 font readiness first, then redispatches with its own resume flag so neither existing gate is bypassed.
- Missing canvas geometry, nonuniform preview scaling, font failure/timeout, missing text painters after resource readiness, invalid CSS measurements, and post-freeze instability fail closed with explicit stage diagnostics.
- Focused Vitest coverage exercises canonical-pixel normalization, scale-invariant input fingerprints, literal `normal` line-height, wrapping diagnostics, stable second-pass rules, and invalid measurements.
- Final head `16907e5ece060922d04b336f9db463b9e0b8bf20` passed Quality #1650, Security #1655, Linux workspace/quota, macOS runtime/adversarial/extension, and browser-egress assurance.
- Quality #1650 completed backend tests/race, frontend lint/unit/performance/build, full Playwright smoke, and the immutable video-renderer parity baseline/capture/report successfully on that exact head before merge.

### Current pixelate-raster slice

The current branch separates exact raster math from backdrop acquisition so a CSS visual approximation cannot be mislabeled parity-safe:

- `preview-pixelate-raster-v1` resolves pixelate block size and reduced-surface dimensions in canonical canvas pixels.
- The grid policy mirrors the current FFmpeg region path: rounded `blur_radius`, a two-pixel minimum block, integer floor division, and a one-pixel minimum reduced surface.
- `pixelatePreviewRgba` performs a deterministic two-pass nearest-neighbor RGBA kernel: center-mapped downsample followed by nearest-neighbor expansion.
- The kernel is pure and does not read Canvas, DOM, media elements, CSS, network state, or renderer state.
- Focused Vitest coverage proves aligned and non-aligned dimensions, center mapping, edge distribution, straight-alpha byte preservation, and fail-closed validation.
- The existing canonical DOM `pixelate` painter remains explicitly deferred as `pixelate-css-approximation`; this slice does **not** remove that marker yet.
- Exact backdrop acquisition/composition remains the next consumer problem. The approximation may be removed only after a deterministic consumer can provide the already-composited pixels beneath the pixelate shape and execute this kernel on those pixels.
- **Validation status:** hosted validation has not executed on the current branch yet; do not call the slice green until Actions run on the exact head.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. Production structural policy, codec-aware decoded-region semantics, font-resource fixture coverage, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#284 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, and Chromium text layout snapshots. The current slice establishes pixelate raster-grid/kernel semantics. Exact pixelate backdrop consumption, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close decoded visual thresholds for media, transforms, Chromium text metrics/fonts, shapes, transitions, effects, cursor, camera, color space, and deterministic asset loading. |
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

### Renderer-independent semantic core

Canonical evaluators are pure, deterministic, serializable, free of browser/FFmpeg/filesystem/network I/O, usable by preview and export, and fail closed whenever an authorable value lacks explicit semantics.

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Browser layout snapshots are also consumer evidence, not new authored semantics. `text-state-v1` continues to own text intent; Chromium owns the browser shaping/layout result after exact face readiness.

### Media geometry and source provenance

`media-geometry-v1` owns source crop, fit, output crop, painted bounds, and clip bounds. Source dimensions come from explicit content bounds or immutable source provenance, never from output canvas size, DOM boxes, decoded element dimensions, or `object-fit` guesses.

### Perspective and stacking

Track/z-index order remains stacking authority; spatial `z` affects projection. `perspective-projection-v1` serializes projection separately from camera-relative transforms. Free-running and incomplete canonical frames keep an explicit compatibility fallback until normal-playback canonicalization is complete.

### Transitions and pixel composition

`transition-state-v1` owns placement/peer roles/windows/progress; `transition-paint-v1` owns authorable paint families. Pair transitions are pair surfaces, never two independent alpha layers.

- #273 defines stack-safe pair grouping.
- #274 defines exact linear-sRGB pair-pixel composition.
- #275 defines preview execution slots.
- #276 consumes source-over slide/wipe pairs.
- #277 implements the weighted linear-sRGB RGBA kernel.
- #278 separates canonical pair validity from preview raster-source capability.
- #279 executes clean weighted crossfade/zoom/dip pairs on Canvas and gates parity-ready until active surfaces settle.

Unsupported/deferred painter sources stay explicit debt and do not become canonical approximations.

### Effects, text, shapes, cursor, and fonts

- `effect-state-v1` owns enabled authored order, scope, normalized parameters, and exact-frame automation. #270 consumes clip effects; #280 consumes scene effects.
- `text-state-v1` owns renderer-independent text intent. #281 consumes text painter inputs.
- #282 verifies current editor `font_resource_id` identity without claiming immutable provenance.
- #283 loads the exact editor font bytes under an isolated browser alias and gates deterministic evidence on readiness.
- #284 makes Chromium DOM layout the sole browser-side glyph-layout snapshot authority; it does not introduce Canvas `measureText` or FFmpeg `text_w`/`text_h` semantics.
- Immutable static-font identity remains Render Manifest-backed by `font-resource-provenance-v1`.
- A family-name-only snapshot is valid evidence for that Chromium environment but is not cross-machine exact font provenance. Resource-backed faces remain the route to deterministic face identity.
- `shape-state-v1` owns shape geometry/style. The current branch defines `preview-pixelate-raster-v1` for deterministic raster math, but true preview pixelation remains explicit fidelity debt until exact backdrop pixels feed that kernel.
- `cursor-state-v1` owns exact rational cursor sampling, visibility, scale, highlight/click-ring state, and click proximity.

### AudioGraph v1

`audio-graph-v1` owns deterministic 48 kHz stereo sample boundaries, stable clip nodes, mute/solo precedence, pitch-preserving playback rate, channel handling, gain/automation, minimum-overlap fades, non-normalizing summation, and one post-mix program-processing boundary. Phase 6 makes browser/export consumers obey it exactly.

## Phase 2 foundation history

| PR(s) | Capability |
|---|---|
| #187 | Immutable render submission and deterministic parity baseline |
| #191–#206 | Timeline v2 / Render Manifest v1, adapters, frame/range/source/order, curves, exact-frame properties, visual FrameState |
| #208 | Permanent Go↔TypeScript FrameState parity diagnostics |
| #209/#212 | Canonical media fit/crop/source-bounds geometry and FrameState consumption |
| #218 | Canonical perspective projection |
| #220–#229 | Canonical transition placement and authorable transition paint families |
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
| #283 | Exact browser `FontFace` readiness and parity gating | `3543ddf7189161a84699a1c4efb296fc8a928400` |
| #284 | Chromium text layout snapshots and parity-ready stabilization | `7884fef888eadd95a1dd575470e06125d5ec618d` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Create the child directly from the actual parent squash SHA.
3. Verify `compare main...branch` contains only intended paths and is behind by zero.
4. Update this tracker on the clean branch.
5. Validate executed checks on the exact head; never call an unexecuted/cancelled head green.
6. Audit comments/reviews/threads and merge with expected-head protection.
7. Create the next slice from the new actual squash result.

Recent lineage: #282 from #281 squash `a5598fdf...`; #283 from #282 `38ea95ab...`; #284 from #283 `3543ddf7...`; **current pixelate-raster branch directly from #284 squash `7884fef888eadd95a1dd575470e06125d5ec618d`**.

## Phase 0 parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

It does not yet include a project font resource. Add resource-font fixture coverage before treating cross-machine glyph identity as proven. Retained visual evidence remains diagnostic until structural zero-tolerance policy and codec-aware decoded thresholds are frozen and retained on a second supported OS/FFmpeg environment.

## Validation matrix

Every Phase 2+ behavioral slice should execute focused tests plus repository gates when applicable:

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

Hosted CI is authoritative for platform/toolchain cases unavailable in the current connector environment. Setup-only stalls, concurrency cancellations, and unscheduled commits are recorded explicitly and never represented as passing.

## Risk register

| Risk | Control |
|---|---|
| Browser layout becomes a second authored semantic contract | #284 snapshots Chromium consumer output only; `text-state-v1` stays unchanged. |
| `line-height: normal` is approximated differently across runtimes | Preserve `normal` literally; do not invent a numeric multiplier. |
| Resource font is measured before exact face readiness | #284 listener waits on #283 readiness before snapshotting and does not bypass the font gate on resume. |
| Intrinsic size changes after dimensions are frozen | Require a second Chromium pass with stable width/height and line-fragment count before parity-ready. |
| Family-name-only text is mistaken for deterministic face identity | Snapshot records provenance/runtime; exact cross-machine identity still requires a resource-backed face. |
| Text-layout gate bypasses weighted Canvas readiness | Independent resume flags preserve traversal through both gates. |
| Pixelate raster math is confused with backdrop-source parity | `preview-pixelate-raster-v1` owns only grid/kernel math; the DOM CSS approximation remains explicitly deferred until exact already-composited backdrop pixels feed the kernel. |
| Codec-noisy decoded equality is treated as structural parity | Phase 0 keeps canonical structural policy separate from codec-aware decoded evidence. |
| CI scheduling hides code state | Only actually executed checks count. |

## Next recommended slice

1. Validate the current pixelate-raster kernel branch on hosted frontend/unit/build gates and audit its exact tree.
2. Add a deterministic pixelate backdrop-source capability planner that admits only compositions whose already-composited pixels can be reproduced exactly; keep unsupported DOM/text/shape/cursor/effect cases fail-closed.
3. Consume `preview-pixelate-raster-v1` on an exact Canvas backdrop surface and only then remove `pixelate-css-approximation` for admitted cases.
4. Broaden weighted-Canvas raster eligibility for text/shape/cursor only after each source painter is exact.
5. Continue Phase 3 with normal-playback canonicalization, explicit diagnostics/rollback, then shared AudioGraph consumption.