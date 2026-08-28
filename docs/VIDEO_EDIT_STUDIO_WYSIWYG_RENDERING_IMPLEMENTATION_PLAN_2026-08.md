# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-28  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#285 — Define deterministic pixelate raster kernel** — squash merge `64e34450806fc97da07ef85901fee591b4a59171` (2026-08-28).

Current implementation PR: **#286 — Plan exact pixelate backdrop admission** on branch `feat/video-wysiwyg-phase3-pixelate-backdrop-planner`, created directly from #285's actual squash result `64e34450806fc97da07ef85901fee591b4a59171`.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** Renderer-independent contracts own authored semantics. Browser/FFmpeg consumers may produce renderer-specific evidence, but they must not silently redefine canonical intent.

### #285 merged result

#285 separated deterministic pixelate raster math from backdrop acquisition so the existing CSS approximation cannot be mislabeled parity-safe:

- `preview-pixelate-raster-v1` owns pixelate block size and reduced-surface dimensions in canonical canvas pixels.
- Block policy matches the existing FFmpeg region path: rounded `blur_radius`, a two-pixel minimum block, integer floor division, and a one-pixel minimum reduced surface.
- The browser kernel reproduces libswscale `flags=neighbor` sample-index selection using the rounded 16.16 fixed-point coordinate step, including non-divisible tie cases; it does not substitute a center/floor approximation.
- `pixelatePreviewRgba` performs deterministic two-pass straight-alpha RGBA sampling without reading Canvas, DOM, media elements, CSS, network state, or renderer state.
- Focused Vitest coverage proves aligned/non-aligned dimensions, libswscale tie behavior, straight-alpha byte preservation, and fail-closed plan/buffer validation.
- The canonical DOM `pixelate` painter remains explicitly marked `pixelate-css-approximation`; #285 intentionally did not remove it.
- Exact code head `12c9210d574503a4a66a6c18bd5276af0cb80d45` passed Quality #1655, Security #1660, Linux workspace/quota, macOS runtime/adversarial/extension, and browser-egress assurance before merge.
- Quality #1655 passed backend tests/race, frontend lint/unit/performance/build, full Playwright smoke, and immutable video-renderer parity baseline/capture/report on that exact head.
- Retained Quality #1655 artifacts include `video-parity-baseline` digest `sha256:44ef97f80910d0015ee55ae430b3e971ef95cf5f1dc252beb2fe096d4cd21d85` and `playwright-report` digest `sha256:1ca3b817d630b92e5b5ab0e36ae545023771314ceacc00ec9c91f7ee6f4ecabc`.

### Export blocker discovered during #285

The browser kernel is **not yet evidence of byte-identical FFmpeg pixelate output**:

- Current `blurRegionParts` specifies `flags=neighbor` on the pixelate **upsample** but leaves the first downsample on FFmpeg's default scaler.
- Synthetic FFmpeg 7.1.5 probes demonstrated that the implicit first pass can select materially different reduced colors than explicit neighbor scaling.
- FFmpeg pixel-format conversion/scaler behavior can also perturb alpha bytes. Do not infer transparent-source parity from the straight-alpha TypeScript kernel.
- Therefore the first exact preview consumer must stay fail-closed, require explicit decoded-pixel evidence, and must not remove `pixelate-css-approximation` until export sampling/composition is aligned and tested.

### #286 current scope

#286 adds a structural backdrop admission contract only; it still does not change preview pixels:

- `preview-pixelate-backdrop-plan-v1` consumes the canonical bottom-to-top preview layer order.
- V1 admits exactly one active pixelate region over exactly one lower canonical media layer.
- The pixelate target must be authoritative and resolved, with no text, cursor, clip effects, transitions, crop/perspective, rotation/3D state, opacity reduction, nonuniform scale, nonzero anchor, or camera-relative placement drift.
- The lower media layer reuses the existing weighted-raster source classifier and additionally rejects opacity reduction, transitions, rotation, and camera-relative placement drift.
- Multiple active pixelate regions and zero/multiple lower visual layers remain explicit deferrals.
- Runtime pixel evidence remains separate from structural eligibility. Every ready plan carries `decoded-frame-ready` and `opaque-region-proof` requirements; MIME type alone is never treated as opacity proof.
- The existing DOM `pixelate-css-approximation` marker remains unchanged. Canvas acquisition/execution is a later slice.
- Focused Vitest coverage exercises ready, legacy/none, multiple-region, complex-target, backdrop-count, and independent backdrop blocker paths.
- **Validation status:** hosted Actions are pending on #286's exact branch head; do not call this slice green until Quality/Security and parity evidence actually execute.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. Production structural policy, codec-aware decoded-region semantics, font-resource fixture coverage, and second-platform evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#285 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, Chromium text layout snapshots, and deterministic pixelate raster sampling. #286 adds fail-closed backdrop admission. Exact pixelate Canvas consumption/export sampling alignment, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
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
- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines the first fail-closed backdrop admission boundary. True preview pixelation remains explicit fidelity debt until exact decoded backdrop pixels feed that kernel and FFmpeg sampling is aligned.
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
| #285 | Deterministic pixelate raster grid and libswscale-neighbor sampling | `64e34450806fc97da07ef85901fee591b4a59171` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Create the child directly from the actual parent squash SHA.
3. Verify `compare main...branch` contains only intended paths and is behind by zero.
4. Update this tracker on the clean branch.
5. Validate executed checks on the exact head; never call an unexecuted/cancelled head green.
6. Audit comments/reviews/threads and merge with expected-head protection.
7. Create the next slice from the new actual squash result.

Recent lineage: #283 from #282 squash `38ea95ab...`; #284 from #283 `3543ddf7...`; #285 from #284 `7884fef8...`; **#286 directly from #285 squash `64e34450806fc97da07ef85901fee591b4a59171`**.

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
| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math only; #286 owns structural backdrop admission only. Neither removes the CSS approximation. |
| libswscale-neighbor sampling is confused with current FFmpeg pixelate output | Current export downsample is implicit/default scaling. Align both FFmpeg scale passes and add retained evidence before claiming output parity. |
| MIME type is treated as proof of an opaque backdrop | #286 requires a separate runtime `opaque-region-proof`; decoded-frame readiness is also independent. |
| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | Keep transparent-source execution deferred until explicit decoded RGBA/premultiplication evidence exists. |
| Codec-noisy decoded equality is treated as structural parity | Phase 0 keeps canonical structural policy separate from codec-aware decoded evidence. |
| CI scheduling hides code state | Only actually executed checks count. |

## Next recommended slice

1. Complete #286 hosted validation, exact-tree/review audit, tracker freeze, and expected-head squash merge.
2. Align FFmpeg pixelate sampling in a separate renderer slice: make the first downsample scaler explicit, add renderer contract/golden coverage, and update capabilities only after evidence passes.
3. Add exact Canvas backdrop acquisition for #286-admitted media, including decoded-frame readiness and region opacity proof; feed the raster through `preview-pixelate-raster-v1` and remove `pixelate-css-approximation` only for proven-ready cases.
4. Broaden pixelate and weighted-Canvas raster eligibility only after each additional painter/source has exact composition semantics.
5. Continue Phase 3 with normal-playback canonicalization, explicit diagnostics/rollback, then shared AudioGraph consumption.
