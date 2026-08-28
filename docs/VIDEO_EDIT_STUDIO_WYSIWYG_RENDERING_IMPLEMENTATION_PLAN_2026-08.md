# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-28  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#288 — Execute proven pixelate backdrop on Canvas** — squash merge `7be8e86f9ded0b31b265844e804a92bca3c1b81c` (2026-08-28).

Current implementation PR: **#289 — Retain opaque pixelate preview-render evidence** on branch `test/video-wysiwyg-phase3-pixelate-opaque-parity-evidence`, created directly from #288's actual squash result `7be8e86f9ded0b31b265844e804a92bca3c1b81c`.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** Renderer-independent contracts own authored semantics. Browser/FFmpeg consumers may produce renderer-specific evidence, but they must not silently redefine canonical intent.

### #288 merged result

#288 executed the first deliberately narrow browser Canvas pixelate consumer for a runtime-proven opaque backdrop while preserving fail-closed fallback behavior:

- `PreviewPixelateCanvas.tsx` reuses the already-mounted `<video>`/`<img>` as the sole decoder and source-time authority. It does not create a second media element or decoder.
- `previewFrameWeightedPairCanvas.ts` exposes the existing canonical media crop/fit/transform painter as a shared Canvas primitive; weighted transition behavior delegates to the same implementation.
- `preview-pixelate-canvas-region-v1` mirrors the current FFmpeg pixelate region's integer shape scaling, center-relative placement, signed position truncation, canvas-bound clamping, and #285 non-divisible raster-grid policy.
- Runtime acquisition rasterizes the admitted lower media layer through canonical media geometry into an isolated full-canvas surface, samples the exact target rectangle, and requires **every** sampled alpha byte to equal `255` before exact Canvas execution.
- Proven opaque bytes feed `preview-pixelate-raster-v1`. The target's existing preview host is normalized only after the Canvas reports ready, preserving the host's z-order while preventing a second CSS transform/resample.
- While exact Canvas is active, the CSS pixelate child is hidden and its `pixelate-css-approximation` marker is removed. Cleanup restores the exact prior host style, child visibility, and deferred marker.
- Transparent regions, decoder-budget posters, structurally unsupported state, free-running playback, and other unproven cases keep the CSS compatibility painter. Runtime errors/timeouts remain evidence failures rather than silently passing parity-ready.
- The structural planner also defers pixelate transform keyframes and explicit axis-scale values that differ from the legacy scalar `scale`, because the current FFmpeg region path is static and consumes only scalar region scale. Static uniform scalar scale remains eligible.
- Focused Vitest coverage locks FFmpeg-compatible integer region geometry, edge clamping, non-divisible `403×307 → 20×15` raster dimensions, opacity proof, scalar-scale eligibility, and renderer-static transform deferrals.
- The existing `parity-torture-v1` pixelate annotation remains on the compatibility path because it has rotation and reduced opacity; #288 did not silently reinterpret that fixture.
- The first #288 Quality wave exposed only an incomplete TypeScript test-fixture cast. That superseded head was corrected by constructing a complete canonical frame fixture; production behavior did not change.
- Exact final head `e05bc342fe6c4cd95373ea076626e2abede7214d` passed Security #1672, Quality #1667, backend formatting/vet/tests/race, frontend lint/449 unit tests/performance/build, full Playwright smoke, immutable video-renderer parity capture, Windows desktop, Helm, plugin lifecycle, and all scheduled platform assurance workflows.
- Retained Quality #1667 artifacts include `video-parity-baseline` digest `sha256:0267093e348d00040af90c11e95686f92a84599e3560e93cd778edfe07bc928b` and `playwright-report` digest `sha256:fa1c6f488f12d2f3d081cd34a97a5882b7f876a3f35eca60e2a2d9ef7fd579c9`.
- Final review audit found no comments, reviews, or unresolved review threads.

### #289 current scope

#289 adds retained browser↔FFmpeg pixel evidence for the narrow #288-admitted opaque path without modifying the established 103-frame torture baseline or prematurely declaring decoded parity:

- `parity-pixelate-opaque-v1` is a separate deterministic 512×512/30 fps fixture with one 1:1 opaque PNG backdrop, one static `403×307` pixelate region using block size `20`, and four fixed sample frames (`0`, `15`, `30`, `59`).
- The fixture intentionally uses the deterministic `asset-square.png` source so the first decoded comparison isolates pixelate sampling/composition from lossy video codec and color-conversion noise.
- A versioned frame-indexed structural-region manifest binds `pixelate-output` to exact canonical frame identity and the fixed renderer-compatible rectangle `[71,94)-[474,401)`.
- The existing immutable snapshot capture pipeline writes same-frame preview/rendered PNG pairs; #289 does not introduce a competing capture path.
- `video-pixelate-parity-assert.mjs` independently proves that each sampled browser frame actually used `canonical-canvas`, reached a ready Canvas surface, normalized the target host, removed the CSS approximation marker, and reported no structural/runtime deferral or error.
- The existing `video-parity-report --regions` path compares every `403×307` pixelate-region RGB pixel exactly. The focused workflow verifies region presence and compared-pixel count and retains exact/non-exact results per frame.
- The dedicated evidence path is now a real parity gate: `video-parity-report` must pass, all four `pixelate-output` structural regions must be byte-exact, and the browser assertion must prove the requested frame used the visible `canonical-canvas` consumer.
- The dedicated `Video Pixelate Parity Evidence` workflow retains fixture, snapshot capture, `pixelate-canvas-evidence.json`, structural parity report, `pixelate-evidence-summary.json`, runtime logs, and toolchain identity for 14 days.
- `parity-torture-v1` and the existing Quality `video-parity-baseline` workflow remain unchanged, preserving longitudinal baseline comparability.
- Codec-decoded video RGB behavior, browser↔FFmpeg color conversion, transparent/premultiplied-alpha semantics, multiple regions/backdrops, and broader pixelate eligibility remain explicit later boundaries.
- Diagnostic capture exposed three browser-evidence lifecycle bugs before parity could be trusted: #289 now gates on the visible Canvas, binds readiness to the requested frame, and preserves the painted bitmap after the paint effect completes. A selected consumer path alone is no longer accepted as proof that the retained screenshot contains the Canvas pixels.
- The first genuine browser↔FFmpeg comparison then isolated renderer-side color-format drift: FFmpeg overlay negotiation introduced subsampled-YUV artifacts, and plain libswscale `neighbor` on packed RGBA still perturbed RGB samples. The production renderer now keeps the visual compositor in RGB, uses `neighbor+full_chroma_inp` for both pixelate scale passes, and applies RGB overlay format to the pixelate patch while leaving ordinary blur unchanged.
- Exact pre-tracker head `e2ceb0c143b9527a37716d0354398a21af30ee3b` passed all 10 PR workflows, including Quality #1685, Security #1690, and Video Pixelate Parity Evidence #17.
- Evidence run #17 proved frames `0`, `15`, `30`, and `59` byte-identical across the complete `512×512` frame: each `403×307` `pixelate-output` region was exact, `changed_pixels=0`, `pixel_pass_rate=1`, `SSIM=1`, and `max_channel_delta=0`; `whole_frame_report_pass=true`. Timeline SHA-256 was `00b1552b68061d77f37917004a8fa039e4e5aac7b31b3a2329527db61ade4acb`.
- Retained `video-pixelate-parity-evidence` artifact `9704331122` is 8,632,668 bytes with SHA-256 `d78118d66b8aa1bab8395ec65f10085f326131f1f4534e2a924fe92920f8019f`. The workflow retains the fixture, immutable preview/render PNGs, consumer evidence, structural report, summary, runtime logs, and toolchain identity for 14 days.
- Final merge evidence must execute again on #289's exact tracker-bearing head; the tracker path is deliberately included in the dedicated parity workflow trigger.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. #289 establishes a separate byte-exact retained opaque-PNG pixelate gate without changing the longitudinal torture baseline. Codec-aware decoded-region semantics, resource-font fixture coverage, and second-platform retained evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#288 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, Chromium text-layout snapshots, deterministic pixelate raster sampling/backdrop admission, explicit FFmpeg neighbor scaling, and runtime-proven opaque Canvas backdrop consumption. #289 closes byte-exact browser↔FFmpeg opaque-PNG pixelate parity for the admitted static path and locks RGB/libswscale behavior. Codec/video evidence, transparent-alpha semantics, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
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

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Browser layout snapshots are consumer evidence, not a new authored semantic contract. `text-state-v1` continues to own text intent; Chromium owns browser shaping/layout after exact face readiness.

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
- #283 loads exact editor font bytes under an isolated browser alias and gates deterministic evidence on readiness.
- #284 makes Chromium DOM layout the sole browser-side glyph-layout snapshot authority; it does not introduce Canvas `measureText` or FFmpeg `text_w`/`text_h` semantics.
- Immutable static-font identity remains Render Manifest-backed by `font-resource-provenance-v1`.
- A family-name-only snapshot is valid evidence for that Chromium environment but is not cross-machine exact font provenance. Resource-backed faces remain the route to deterministic face identity.
- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path while codec/video and transparent-alpha broadening remain explicit later evidence slices.
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
| #286 | Fail-closed exact pixelate backdrop admission | `40e895eec8323da03acd7b4f077743c8a3411eea` |
| #287 | Explicit FFmpeg neighbor sampling on both pixelate scale passes | `2774ee76913ddea0ef691d90bbc285180893b899` |
| #288 | Runtime-proven opaque pixelate Canvas consumption | `7be8e86f9ded0b31b265844e804a92bca3c1b81c` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Create the child directly from the actual parent squash SHA.
3. Verify `compare main...branch` contains only intended paths and is behind by zero.
4. Update this tracker on the clean branch.
5. Validate executed checks on the exact head; never call an unexecuted/cancelled head green.
6. Audit comments/reviews/threads and merge with expected-head protection.
7. Create the next slice from the new actual squash result.

Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; **#289 directly from #288 squash `7be8e86f9ded0b31b265844e804a92bca3c1b81c`**.

## Phase 0 parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

#289 intentionally keeps that longitudinal fixture unchanged and adds `parity-pixelate-opaque-v1` as a focused evidence fixture. The focused fixture can require exact structural-region equality without forcing codec-noisy or unrelated torture-frame behavior into the same policy decision.

The torture baseline does not yet include a project font resource. Add resource-font fixture coverage before treating cross-machine glyph identity as proven. Retained visual evidence remains diagnostic until structural zero-tolerance policy and codec-aware decoded thresholds are frozen and retained on a second supported OS/FFmpeg environment.

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
| Text-layout gate bypasses weighted/pixelate Canvas readiness | Independent resume flags preserve traversal through each deterministic readiness gate. |
| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural backdrop admission; #287 owns FFmpeg scaler selection; #288 owns browser runtime acquisition/opaque proof; #289 measures retained cross-runtime output. None is substituted for another. |
| MIME type is treated as proof of an opaque backdrop | #288 scans the sampled RGBA target rectangle and admits exact Canvas execution only when every alpha byte is `255`. |
| A second decoder becomes a competing source-time authority | #288 reuses the already-mounted preview `<video>`/`<img>` and only paints that existing decoded frame. |
| Pixelate target host applies a second transform to exact bytes | The Canvas stays hidden while proving readiness; only then is the existing target host normalized to full-stage geometry and the CSS painter hidden. |
| FFmpeg/static-region behavior is confused with canonical transform support | #288 explicitly defers target transform keyframes and explicit axis scale that diverges from legacy scalar `scale`. |
| Evidence accidentally measures the CSS fallback instead of the Canvas consumer | #289 independently requires `canonical-canvas`, ready surface/host markers, no deferred/error markers, and absence of `pixelate-css-approximation` for every sampled frame. |
| A diagnostic `--allow-fail` report is mistaken for exact parity | #289 retains `exact`, changed-pixel count, pass rate, SSIM, and max-channel delta per region/frame; workflow success proves evidence completeness, not exactness. Exact parity is claimed only if retained metrics say so. |
| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | Keep transparent-source execution deferred until explicit decoded RGBA/premultiplication evidence exists. |
| Codec-noisy decoded equality is treated as structural parity | #289 starts with a deterministic opaque PNG; codec/video evidence and thresholds remain a separate later boundary. |
| CI scheduling hides code state | Only actually executed checks count. |

## Next recommended slice

1. Freeze #289's opaque-PNG exactness as a non-regression gate and add a separate codec-decoded video pixelate evidence fixture that reuses the same canonical frame/region identity without weakening PNG equality.
2. Measure browser-decoded versus FFmpeg-decoded codec/color differences first, then set explicit format/color-space-specific acceptance thresholds only where exact bytes are not a meaningful codec contract.
3. Keep transparent/premultiplied-alpha sources deferred until their decoded RGBA, premultiplication, and compositing semantics are explicitly defined and retained.
4. Preserve the existing 103-frame `parity-torture-v1` baseline; codec evidence remains an additive focused slice rather than replacing longitudinal baseline coverage.
5. Broaden pixelate and weighted-Canvas raster eligibility only after each additional painter/source has exact composition semantics and retained evidence.
6. Continue Phase 3 with normal-playback canonicalization, explicit diagnostics/rollback, then shared AudioGraph consumption.
