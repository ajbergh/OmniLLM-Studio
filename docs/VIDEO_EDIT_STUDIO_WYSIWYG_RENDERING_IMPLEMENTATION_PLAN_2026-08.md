# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-29  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#291 — Merge validated codec-decoded pixelate parity evidence** — squash merge `7d5f36c3230d23eef46de310d6ac785b1b998c33` (2026-08-29). #291 contains the exact validated head from draft #290; #290 was closed only because the connector's ready-for-review mutation failed against GitHub's current GraphQL schema.

Current implementation PR: **#292 — Prove deterministic decoded-video frame identity** on branch `fix/video-wysiwyg-phase3-decoded-frame-identity`, created directly from #291's actual squash result `7d5f36c3230d23eef46de310d6ac785b1b998c33`.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** Renderer-independent contracts own authored semantics. Browser/FFmpeg consumers may produce renderer-specific evidence, but they must not silently redefine canonical intent.

### #289 merged result

#289 converted the first #288-admitted opaque pixelate Canvas path into retained byte-exact browser↔FFmpeg evidence and fixed a renderer scalability defect exposed by full-delivery validation:

- `parity-pixelate-opaque-v1` is a deterministic 512×512/30 fps fixture with one 1:1 opaque PNG backdrop, one static `403×307` pixelate region using block size `20`, and fixed frames `0`, `15`, `30`, and `59`.
- A versioned frame-indexed `pixelate-output` structural-region manifest binds exact canonical frame identity to `[71,94)-[474,401)`.
- Browser evidence proves the retained screenshot actually uses the visible `canonical-canvas` consumer, that the surface is ready, the target host is normalized, the CSS approximation marker is absent, and no structural/runtime deferral or error is present.
- FFmpeg visual composition remains in RGB for pixelate; both scale passes use `neighbor+full_chroma_inp`, avoiding subsampled-YUV and packed-RGB scaler drift discovered by the evidence fixture.
- Sampled fidelity expansion no longer opens one FFmpeg input per expanded segment. `renderer.go` indexes each immutable source path once and uses deterministic `split` / `asplit` fan-out while preserving independent segment trim, rate, timing, transform/effect, and audio semantics.
- Exact merge-candidate head `3fe95357216c1075e16ea0e07d67ec4c86de361b` passed all 10 PR workflows: Quality #1694, Security #1699, Video Pixelate Parity Evidence #26, and all scheduled platform/sandbox assurance workflows.
- Evidence #26 proved all four sampled PNG frames byte-identical: 4/4 exact regions, `changed_pixels=0`, `pixel_pass_rate=1`, `SSIM=1`, `max_channel_delta=0`, and `whole_frame_report_pass=true`. Retained artifact `9717793446` is 8,632,638 bytes with SHA-256 `030bf4c40c147e14c7dd989efa0f20cd55504058d7dbd1fce51c81e3b151d2dc`.
- Quality #1694 preserved the unchanged 103-sample `parity-torture-v1` fixture and completed the 20,000 ms delivery using exactly four unique immutable media inputs while the sampled graph still reached `[a793]`. Retained `video-parity-baseline` artifact `9717953769` is 51,206,965 bytes with SHA-256 `c3693f643f467eed3886db4bc8f4f5e8932098c5c1e0c2c11b65e8589f33937c`.
- The longitudinal torture-fixture report still intentionally runs with `--allow-fail`; that existing Phase 0 debt is not represented as universal WYSIWYG parity closure.

### #291 merged codec-decoded evidence

#291 merged the exact validated #290 head and adds codec-decoded video evidence without weakening #289's byte-exact opaque-PNG control:

- `parity-pixelate-decoded-video-v1` reuses the same static pixelate semantics with deterministic H.264/yuv420p `asset-landscape.mp4`, a 640×360 1:1 source/canvas, the same non-divisible `403×307` region and block size `20`, and frames `0`, `15`, `30`, and `59`.
- `video-parity-region-report` retains tolerant RGB metrics separately from structural-exactness evidence so codec/color diagnostics cannot redefine the PNG contract.
- The focused workflow has two independent jobs: the existing opaque-PNG byte-exact gate and additive H.264 decoded-video evidence.
- Initial H.264 evidence exposed a runtime-readiness bug, not a pixelate geometry bug: Chromium can report `HAVE_CURRENT_DATA` after a seek before the requested decoded frame is rasterizable to Canvas. The first transparent/partial Canvas read was incorrectly finalized as `opaque-region-proof`, which allowed the compatibility CSS painter to remain active.
- `PreviewPixelateCanvas.tsx` now treats **video-only** opacity misses as transient for a bounded 60-paint-frame retry window. Exact Canvas still activates only when every sampled alpha byte is exactly `255`; images remain fail-closed immediately, and video that never proves opacity still defers to compatibility rendering after the bounded window.
- `resolvePreviewPixelateOpacityProof` centralizes that readiness policy and focused Vitest coverage locks exact-alpha readiness, bounded video retry behavior, immediate image deferral, exhausted-video deferral, and invalid retry accounting.
- Tracker-bearing exact head `0edcce0018e7afe68b304b0c11c0c8cfd650400f` passed the complete PR workflow set: Quality #1704, Security #1709, Video Pixelate Parity Evidence #36, the full Chromium smoke suite, backend tests/race detector, frontend lint/unit/build, and all platform/sandbox assurance workflows.
- Evidence #36 retained decoded-video artifact `9725388648` (14,094,549 bytes, SHA-256 `69de2792b4958357732a817aae9daa628d6e89cfac55bd83b81b1aa2a443df03`) and the unchanged exact-PNG artifact `9725388161` (8,632,693 bytes, SHA-256 `72c42eff1842e38fad7b06cb7745b7468f099a0c5ca099aee3765d84e3055274`). Timeline SHA-256 remains `fbc96eb288c716b9862127fb484d7ea0f05a19122dac18096c364644afe36eea`.
- Frames `0`, `15`, and `30` are source-time aligned and establish the measured H.264 browser↔FFmpeg color envelope: maximum RGB channel delta `3`, SSIM `0.9999356684` or better, and mean absolute error below `0.9`. Re-evaluating those aligned regions at ±3 channel tolerance produces a 100% pixel pass rate for all three frames.
- Frame `59` is deliberately **not** folded into that codec/color envelope. Retained source-frame comparison shows the browser preview raster corresponds to source frame `58` while FFmpeg renders source frame `59`; its max delta `255` and lower SSIM therefore represent decoded-video frame-selection/seek-boundary debt, not ordinary YUV↔RGB color conversion.
- The current decoded-video region report remains measurement-oriented (`--allow-fail`) at the repository default ±2/99.9% threshold. It reports approximately 98.69–99.03% pixel pass on aligned frames and 85.64% on frame 59. #290 does not broaden one threshold to hide the timing mismatch.
- Transparent/premultiplied-alpha sources remain explicitly deferred. No MIME-only or codec-only opacity assumption was introduced.
- The existing 103-sample longitudinal baseline remains unchanged and additive to both focused pixelate fixtures.
- #291 squash-merged that exact validated head as `7d5f36c3230d23eef46de310d6ac785b1b998c33`.

### #292 current scope

#292 closes the decoded-video frame-selection debt before promoting the measured codec/color envelope into a gate:

- Keep canonical `source_time_ms` unchanged. For paused deterministic `<video>` seeks only, request a point just inside the rational frame boundary so a Float64 value infinitesimally below the source PTS cannot select the preceding decoded frame. Audio and free-running playback remain untouched.
- Keep the nudge below the existing 0.5 ms deterministic seek tolerance and well inside one output-frame interval.
- Extend the focused browser evidence with `requestVideoFrameCallback` presentation timestamps so Chromium's submitted decoded frame, not only `currentTime`/`seeked`, is retained.
- Require presented frame identity to match canonical frames `0`, `15`, `30`, and `59`; non-zero samples must also prove the media element sought past the exact boundary rather than landing infinitesimally below it.
- Only after frame identity passes, enforce the measured H.264/yuv420p codec/color envelope as an explicit `max_channel_delta <= 3` pixelate-region gate. Repository-global ±2/99.9% defaults and #289's byte-exact PNG gate remain unchanged.
- Implementation head `f9a3a6baa05c1ec82d5453b77e95c909cc09d22d` passed Video Pixelate Parity Evidence #41. The decoded job and the independent opaque-PNG job both completed successfully.
- Retained decoded artifact `9725989461` is 14,087,302 bytes with SHA-256 `1189464ee9067b55271bc507ce604843f258a338102481a84c2d53c435c6ed7f`; retained exact-PNG artifact `9725990152` is 8,632,756 bytes with SHA-256 `6e0d3a407e5ccedefa7f03f9a267d23cbdc53f9d9b3474c6dbf4f5c8d60962ea`.
- Chromium presentation identity is now explicit: frame `0` retains the allowed initial-current-time proof at `0`; frame `15` presents `mediaTime=0.5` after seeking to `currentTime=0.50025`; frame `30` presents `mediaTime=1.0` after seeking to `currentTime=1.00025`; frame `59` presents `mediaTime=1.966667` after seeking to `currentTime=1.966916`. The requested/presented frame indices are `0/15/30/59` exactly.
- Every corrected decoded pixelate region covers `[135,18)-[538,325)` = `123,721` pixels. All `123,721` pixels pass the focused ±3 RGB envelope on every sample and every sample has `max_channel_delta=3`. Frame `59`'s repository-default ±2 diagnostic improves from the pre-fix gross mismatch to `pixel_pass_rate=0.9967669191`, `SSIM=0.9999361828`, and `MAE=0.8941570146`, confirming the remaining difference is ordinary codec/color conversion rather than frame identity.
- Preserve the original 103-sample longitudinal baseline and transparent/premultiplied-alpha deferral. Final tracker-bearing merge validation must still pass the complete exact-head Quality, Security, Pixelate Evidence, and platform/sandbox workflow set.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. #289 added the byte-exact opaque-PNG gate; #291 merged retained H.264 decoded-video measurement. #292 implementation evidence now proves decoded frame identity on 0/15/30/59 and enforces the focused ±3 H.264/yuv420p color envelope only after that identity proof. Resource-font fixture coverage and second-platform retained evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, Chromium text-layout snapshots, deterministic pixelate raster/backdrop semantics, runtime-proven opaque Canvas consumption, byte-exact opaque-PNG evidence, and sampled-render source fan-out. #291 extends retained evidence to decoded H.264; #292 implementation evidence closes deterministic decoded-video frame identity while preserving fail-closed alpha behavior. Transparent-alpha semantics, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |
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
- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #291 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness; #292 proves decoded frame identity before freezing the codec/color gate.
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
| #289 | Retained byte-exact opaque-PNG pixelate evidence and immutable-source fan-out | `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b` |
| #291 | Retained H.264 decoded-video pixelate evidence and bounded video Canvas readiness | `7d5f36c3230d23eef46de310d6ac785b1b998c33` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Create the child directly from the actual parent squash SHA.
3. Verify `compare main...branch` contains only intended paths and is behind by zero.
4. Update this tracker on the clean branch.
5. Validate executed checks on the exact head; never call an unexecuted/cancelled head green.
6. Audit comments/reviews/threads and merge with expected-head protection.
7. Create the next slice from the new actual squash result.

Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; #290 validated directly from #289 squash `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b`; #291 mirrored that exact validated #290 head because the ready-for-review connector mutation failed, then squash-merged as `7d5f36c3230d23eef46de310d6ac785b1b998c33`; **#292 is directly from #291 squash `7d5f36c3230d23eef46de310d6ac785b1b998c33`**.

## Phase 0 parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

The focused `parity-pixelate-opaque-v1` fixture is a byte-exact non-regression gate for the isolated opaque-PNG static pixelate path. `parity-pixelate-decoded-video-v1` is additive decoded-media evidence: #292 proves deterministic decoded-frame identity first and then applies the explicit ±3 RGB H.264/yuv420p pixelate-region envelope without changing repository-global parity defaults.

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
| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural backdrop admission; #287 owns FFmpeg scaler selection; #288 owns browser runtime acquisition/opaque proof; #289 owns byte-exact PNG evidence; #291 owns decoded-video measurement; #292 owns decoded-frame identity and the focused codec/color acceptance gate. None is substituted for another. |
| MIME type or codec is treated as proof of an opaque backdrop | #288/#291 scan the sampled RGBA target rectangle and activate exact Canvas only when every alpha byte is `255`; no MIME-only or codec-only shortcut exists. |
| Newly sought video reports data before its frame is Canvas-rasterizable | #291 keeps video-only opacity misses pending for a bounded retry window, still requiring exact alpha before activation and failing closed after exhaustion. |
| A second decoder becomes a competing source-time authority | The exact Canvas path reuses the already-mounted preview `<video>`/`<img>` and paints only that existing decoded frame. |
| Browser seek lands on a neighboring decoded frame at an exact boundary | #291 retained frame 59 as timing debt. #292 implementation evidence applies a 0.25 ms-or-smaller video-only boundary nudge and proves `requestVideoFrameCallback` presents the requested 0/15/30/59 frames before the ±3 codec/color gate is accepted. |
| Sampled fidelity expansion multiplies FFmpeg decoder instances | #289 indexes immutable source paths once and uses deterministic `split` / `asplit` fan-out so hundreds of sampled consumers preserve independent semantics without reopening the same asset hundreds of times. |
| Pixelate target host applies a second transform to exact bytes | The Canvas stays hidden while proving readiness; only then is the existing target host normalized to full-stage geometry and the CSS painter hidden. |
| FFmpeg/static-region behavior is confused with canonical transform support | The pixelate planner explicitly defers target transform keyframes and explicit axis scale that diverges from legacy scalar `scale`. |
| Evidence accidentally measures CSS fallback instead of Canvas | Browser assertion requires `canonical-canvas`, ready surface/host markers, no deferred/error markers, and absence of `pixelate-css-approximation` for every sampled frame. |
| A diagnostic `--allow-fail` report is mistaken for exact parity | Reports retain pass rate, SSIM, MAE/RMSE, max-channel delta, and exact consumer identity. Workflow success proves evidence completeness unless an explicit threshold gate is configured. |
| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | Transparent-source execution stays deferred until decoded RGBA/premultiplication/compositing semantics are defined and retained. |
| Codec-noisy decoded equality is treated as structural parity | PNG structural exactness remains independent; #292 gates decoded-video color at ±3 only after source-frame identity is proven, while repository-default ±2 diagnostics remain unchanged. |
| CI scheduling hides code state | Only actually executed checks count. |

## Next recommended slice

1. **Finish and merge #292 after the tracker-bearing exact-head workflow wave is green.** The implementation evidence already proves requested/presented decoded frame identity on 0/15/30/59 and a 100% pixel pass at the focused ±3 H.264/yuv420p envelope, while the byte-exact PNG control remains independent.
2. **Next slice: define transparent/premultiplied-alpha pixelate semantics.** Keep current fail-closed behavior until canonical decoded RGBA representation, browser Canvas premultiplication, FFmpeg alpha handling, and source-over compositing order are explicit and cross-runtime testable.
3. Add a deterministic transparent/partial-alpha fixture with retained browser↔renderer region evidence. Do not infer opacity or compositing semantics from MIME type, codec, or container.
4. Broaden pixelate and weighted-Canvas raster eligibility only after each additional painter/source has exact composition semantics and retained evidence; preserve the opaque PNG and H.264 controls as independent non-regression gates.
5. Add resource-font fixture coverage and retain parity evidence on a second supported OS/FFmpeg environment before calling cross-machine Phase 0 visual identity closed.
6. Continue Phase 3 with normal-playback canonicalization, explicit diagnostics/rollback, then shared AudioGraph consumption.
