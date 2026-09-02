# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-09-02
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, font resources, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR updates current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG program PR: **#301 — Retain normal playback canonicalization browser evidence** — squash merge `1f891dc4338853357dcfdead8c073c9c81516bba`, directly from #300's actual squash result `d4049105d49ce4bc8337a3c5236593f503869d5d`.

Current implementation PR: **#302 — Promote ready weighted transitions during normal playback** on branch `feat/video-wysiwyg-phase3-weighted-playback`, created directly from #301's actual squash result `1f891dc4338853357dcfdead8c073c9c81516bba`. Pre-tracker exact head `630f71526b3ae4e01a84d34553520d90b3304ac1` passed all 11 PR workflows, including retained 10-case weighted playback browser evidence. This tracker update intentionally creates a new exact head that must re-run repository gates before merge.

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

### #293 merged decoded-frame identity

The validated #292 implementation tree, merged unchanged by #293, closes the decoded-video frame-selection debt before promoting the measured codec/color envelope into a gate:

- Keep canonical `source_time_ms` unchanged. For paused deterministic `<video>` seeks only, request a point just inside the rational frame boundary so a Float64 value infinitesimally below the source PTS cannot select the preceding decoded frame. Audio and free-running playback remain untouched.
- Keep the nudge below the existing 0.5 ms deterministic seek tolerance and well inside one output-frame interval.
- Extend the focused browser evidence with `requestVideoFrameCallback` presentation timestamps so Chromium's submitted decoded frame, not only `currentTime`/`seeked`, is retained.
- Require presented frame identity to match canonical frames `0`, `15`, `30`, and `59`; non-zero samples must also prove the media element sought past the exact boundary rather than landing infinitesimally below it.
- Only after frame identity passes, enforce the measured H.264/yuv420p codec/color envelope as an explicit `max_channel_delta <= 3` pixelate-region gate. Repository-global ±2/99.9% defaults and #289's byte-exact PNG gate remain unchanged.
- Implementation head `f9a3a6baa05c1ec82d5453b77e95c909cc09d22d` passed Video Pixelate Parity Evidence #41. The decoded job and the independent opaque-PNG job both completed successfully.
- Retained decoded artifact `9725989461` is 14,087,302 bytes with SHA-256 `1189464ee9067b55271bc507ce604843f258a338102481a84c2d53c435c6ed7f`; retained exact-PNG artifact `9725990152` is 8,632,756 bytes with SHA-256 `6e0d3a407e5ccedefa7f03f9a267d23cbdc53f9d9b3474c6dbf4f5c8d60962ea`.
- Chromium presentation identity is now explicit: frame `0` retains the allowed initial-current-time proof at `0`; frame `15` presents `mediaTime=0.5` after seeking to `currentTime=0.50025`; frame `30` presents `mediaTime=1.0` after seeking to `currentTime=1.00025`; frame `59` presents `mediaTime=1.966667` after seeking to `currentTime=1.966916`. The requested/presented frame indices are `0/15/30/59` exactly.
- Every corrected decoded pixelate region covers `[135,18)-[538,325)` = `123,721` pixels. All `123,721` pixels pass the focused ±3 RGB envelope on every sample and every sample has `max_channel_delta=3`. Frame `59`'s repository-default ±2 diagnostic improves from the pre-fix gross mismatch to `pixel_pass_rate=0.9967669191`, `SSIM=0.9999361828`, and `MAE=0.8941570146`, confirming the remaining difference is ordinary codec/color conversion rather than frame identity.
- Preserve the original 103-sample longitudinal baseline and transparent/premultiplied-alpha deferral. Final tracker-bearing head `f255243bc36da8d13efaf55ee770b4f20503723d` passed Quality #1716, Security #1721, Video Pixelate Parity Evidence #45, Chromium smoke, the full renderer parity baseline, and all applicable platform/sandbox assurances. #293 squash-merged that exact validated tree as `9850370c2aa25a076f3272077062aeab08c1f326`.

### #295 merged alpha-image composition

#294 implemented the first transparent-image pixelate composition gap without broadening canonical authored semantics; #295 promoted that exact validated implementation tree to `main`:

- Mirror FFmpeg's admitted renderer order for the one-media-layer pixelate path: start from the opaque project canvas background, source-over the decoded media through canonical media geometry, then sample/pixelate the already-composited RGB backdrop.
- Match the renderer's background grammar exactly: six-digit RGB with or without `#`; anything else falls back to black.
- Admit transparent and partial-alpha **images** by Canvas source-over composition. Do not inspect, unpremultiply, or reinterpret image source bytes. Hidden RGB under alpha `0` must never leak into the composed backdrop.
- Keep decoded-video source-only alpha/readiness proof unchanged before background composition. Transparent video remains fail-closed in this slice because the existing proof also guards Chromium's post-seek rasterizability race.
- Add deterministic `parity-pixelate-alpha-png-v1` evidence over non-black `#19324A`, using an NRGBA PNG with hidden RGB and alpha `0/64/128/192/255`, the established `403x307` region/block-20 raster, and frames `0/15/30/59`.
- Preserve #289's byte-exact opaque-PNG gate and #293's H.264 requested/presented frame-identity plus focused ±3 RGB gate as independent controls.
- First retained alpha evidence on head `d2825fc7f679d668d71cf9c02b4e348afc52797f` succeeded in Video Pixelate Alpha Parity Evidence #1. Artifact `9727309505` is 10,861,044 bytes with SHA-256 `853f9d6d8ccd19b1f2a6d3105441d9d9bd68446e9bef03f6f51a252e9fd30273`; timeline SHA-256 is `e4b77a9f0c019618ee801e76f33a63944102d998d6710e43b2db833524fa7976`.
- Browser evidence proves `canonical-canvas`, a ready surface/normalized target host, no structural/runtime deferral or CSS fallback, on all four samples. The project background is `#19324A`; the retained evidence schema is extended to record the Canvas surface's resolved background so the next exact-head run proves that consumer value directly.
- Every alpha pixelate region covers `[71,94)-[474,401)` = `123,721` pixels. Frames `0/15/30/59` are static-source identical and each reports `pixel_pass_rate=1`, `max_channel_delta=1`, `SSIM=0.9999377268`, `MAE=0.3303723701`, and `RMSE=0.5747802798`. The whole-frame and region reports both pass repository defaults.
- Because this lossless alpha path differs only by one RGB code value after browser-vs-FFmpeg source-over rounding, #294 freezes a **fixture-specific ±1 RGB gate with 100% pixel pass**. It does not weaken #289's zero-tolerance opaque PNG gate, #293's H.264 ±3 gate, or repository-global ±2/99.9% diagnostics.
- Exact hard-gate head `db8cab44108b668eac5d870f68b939052eaad8cc` passed Video Pixelate Alpha Parity Evidence #6. Artifact `9727353580` is 10,861,146 bytes with SHA-256 `cbf77e96ba204a206ec61baea061c9ce549439d63ffa8621b82e970bf0d88824`; retained timeline SHA-256 is `34df731b6f19146546aae9187ec6894677ca7ffe1daa91d092ae6782a1e2df02`. Every 123,721-pixel region on frames `0/15/30/59` reports `max_channel_delta=1`, `alpha_gate_pass=true`, and the browser surface explicitly records `surface_background=#19324A`.
- The same exact head also passed Video Pixelate Parity Evidence #59 (opaque-PNG byte-exact control plus H.264 decoded-frame identity/±3 control), Security Scan #1735, Quality Gate #1730 including frontend lint/unit/build, backend tests/vet/race detector, Playwright smoke, and the full 103-frame renderer parity baseline, plus all applicable Linux/macOS/browser/container assurance workflows. No PR comments, reviews, or unresolved review threads are present.


### #296 merged transparent-video presentation and VP9 alpha

#296 removes the remaining blanket transparent-video deferral for the admitted pixelate Canvas path while keeping canonical source time and decoder ownership unchanged:

- `VideoPreviewCanvasLegacy` now owns deterministic decoded-video presentation readiness. Each deterministic seek clears a request token and only a matching post-seek `requestVideoFrameCallback` can fulfill it; `PreviewPixelateCanvas` must observe that exact ready token before sampling the mounted `<video>`.
- Presentation readiness is independent of pixel alpha. Once the mounted decoder proves the requested frame is presented, transparent and partial-alpha video is source-over composited against the renderer-matched project background like the admitted alpha-image path; alpha is content semantics rather than a readiness proxy.
- Immutable probe metadata freezes video codec, pixel format, and alpha mode. VP9 WebM with `alpha_mode=1` selects `libvpx-vp9` explicitly because the hosted FFmpeg default VP9 decoder discards alpha even though the stream advertises it.
- `parity-pixelate-alpha-video-v1` is a deterministic changing 512×512/30 fps transparent VP9 fixture over `#19324A` with canonical frames `0`, `15`, `30`, and `59`. The retained decoder negative control proves the default VP9 decoder produces 262,144 fully opaque alpha samples, while `libvpx-vp9` preserves 205,672 fully transparent and 56,472 partial-alpha samples in the sampled frame.
- Hosted VP9 evidence exposed millisecond-quantized source PTS (`...1.900, 1.933, 1.967`). Browser deterministic seeks therefore use a bounded `0.49 ms` inside-frame nudge, still below the strict `0.5 ms` seek tolerance. The legacy FFmpeg visual stream mirrors that presentation choice using a 1 MHz filter timebase plus a `490 µs` visual-only PTS bias before playback-rate retiming. Canonical `source_time_ms`, audio timestamps, and free-running preview semantics are unchanged; no CFR regridding assumption is introduced.
- Implementation head `2129b30cd733ba042a98509e000fd2473680a505` passed Video Transparent Pixelate Parity Evidence #12. Retained artifact `9742745036` is 17,396,905 bytes with SHA-256 `f633fce66386b595db9ceb2dfa1c6c70e6c457f1fe0916b78bbe6da8c5a260d0`; retained timeline SHA-256 is `57b940883b59f9319ba38cd9f5f38301707bdebf947e54e64fe9c9600b81865c`.
- All four transparent-video samples prove matching presentation identity in one attempt: media times are `0.000000000`, `0.500000000`, `1.000000000`, and `1.967000000` for frames `0/15/30/59`. Every `[71,94)-[474,401)` pixelate region contains `123,721` pixels, all `123,721` pass the focused ±4 RGB gate, and every sample reports `max_channel_delta=3`.
- Repository-default ±2 whole-frame and region diagnostics remain measurement-oriented and report `pass=false` for this VP9 fixture; #296 does not broaden repository-global thresholds. The explicit transparent-video acceptance contract is presentation identity first, then 100% of region pixels within ±4 RGB.
- Independent controls remain green on the same implementation head: Video Pixelate Parity Evidence #78 passes both the byte-exact opaque-PNG job and H.264 decoded-frame/±3 job; Video Pixelate Alpha Parity Evidence #25 preserves the partial-alpha PNG ±1 gate; Security Scan #1758 and the applicable Linux/macOS/browser/container assurance workflows also pass.
- #295 promoted exact validated #294 head `66722d2cc863fd2b6eea17ed48abd714bbf783d7` and squash-merged it unchanged as `f6a08f72910677ed538e356d544a1d5d1b59d620`.
- Final tracker-bearing #296 head `ca97e06449600c77f443b380098bb139f34ca8ea` passed Quality Gate #1757, Security Scan #1762, Video Transparent Pixelate Parity Evidence #16, Video Pixelate Parity Evidence #82, Video Pixelate Alpha Parity Evidence #29, and all applicable Linux/macOS/browser/container assurance workflows. PR #296 had no comments, reviews, or unresolved review threads and squash-merged as `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`.

### #299 project-background pixelate raster scope

#299 broadens deterministic pixelate Canvas admission by exactly one source class whose renderer order and color semantics were already explicit: the canonical project background itself.

- `preview-pixelate-backdrop-plan-v1` now represents the no-lower-visual-layer case as an explicit `project-background` raster source with `runtimeRequirements: []`; one-media image/video behavior remains unchanged and more than one lower layer still fails closed.
- `PreviewPixelateCanvas` paints the renderer-matched project background unconditionally and only resolves/paints a mounted media source when a media backdrop actually exists. `PreviewPixelateBackdropConsumer` likewise makes poster and presentation-token logic conditional on a real media backdrop.
- This is deliberately pixelate-specific. `previewFrameWeightedPairRaster.ts` remains media-only, and text, shape, cursor, effects, transitions, multi-layer backdrops, and other painter classes are not broadened in this slice.
- Shape was inspected as the initially obvious next class and rejected for this slice because the current legacy FFmpeg shape path still contains renderer-specific approximations such as integerized box geometry and rounded-rectangle simplification. #299 does not promote those approximations into canonical Canvas semantics.
- `parity-pixelate-background-v1` is a deterministic 512×512/30 fps fixture over `#19324A` with exactly one visible pixelate layer, the established `[71,94)-[474,401)` / 123,721-pixel region and block size `20`, and canonical frames `0/15/30/59`. There is no visual media asset or decoder dependency.
- Pre-PR guarded run `33352378391` passed the focused pixelate planner Vitest and frontend production build before implementation commit `686844c17101c771dfad1a8bc1b18261885cd39f`. Guarded run `33352467556` then passed Go fixture validation, CLI fixture generation, the focused planner test, JavaScript syntax validation, and diff checks before evidence commit `156fa4c38a3e48247975561906d523c2d0ce5e11`.
- First retained PR evidence on head `f0a9496f29b66f94bd7f318e8f7120c986dceac8` passed Video Pixelate Background Parity Evidence #1. Browser evidence proves `canonical-canvas`, `surface_background=#19324A`, `surface_backdrop_clip=project-background`, ready execution, no structural/runtime deferral or CSS fallback, and no video presentation request/ready token on all four samples.
- Both whole-frame and region reports pass repository defaults. Every 123,721-pixel region on frames `0/15/30/59` reports `pixel_pass_rate=1`, `max_channel_delta=1`, `SSIM=0.9997632514784561`, `MAE=1`, and `RMSE=1`. Based on that retained measurement, #299 tightens the project-background fixture-specific acceptance contract to **100% of region pixels within ±1 RGB**; repository-global ±2/99.9% defaults remain unchanged.
- Retained background artifact `9744150134` is 11,086,886 bytes with SHA-256 `f3374a3c6a6f1523910e0cdac831e2e223a751148b7bdb758ea36a467df35248`; retained timeline SHA-256 is `e2dd081bafd070c344acac4f84f4c2fe9db9b4272dd57360ffbdc34e40cd44cf`.
- Independent controls are green on the same implementation head: Video Transparent Pixelate Parity Evidence #18, Video Pixelate Alpha Parity Evidence #31, and Video Pixelate Parity Evidence #84 all pass, preserving transparent VP9 ±4/presentation identity, partial-alpha PNG ±1, opaque-PNG exactness, and H.264 ±3/frame identity.
- Final clean head `6be3100cc48ab7b6d0ace8a3d80b296dd5f6805a` re-proved the tightened background contract in Video Pixelate Background Parity Evidence #9: every frame `0/15/30/59` again compares 123,721 pixels with `pixel_pass_rate=1`, `max_channel_delta=1`, `SSIM=0.9997632514784561`, `MAE=1`, and `RMSE=1`; the focused ±1 gate passes. Retained artifact `9744301414` is 11,086,823 bytes with SHA-256 `21739f388d74cd768bfdceba6dd0816d45981d3b0599562d49266a5886639d1f`; retained timeline SHA-256 is `35d0c0598cce81477fee4de82bd6e65bab5f1600b9ff8591d3d2cc01d56195b3`.
- The same exact head passed Quality Gate #1767, Security Scan #1772, Video Transparent Pixelate Parity Evidence #26, Video Pixelate Alpha Parity Evidence #39, Video Pixelate Parity Evidence #92, and all applicable Linux/macOS/browser/container assurance workflows. PR #299 had no comments or review debt and squash-merged as `f225175e9404762b872944cd2a0ddda0e8e8284f`.

### #300 media-only normal-playback canonicalization scope

#300 moves the first safe free-running visual path into the same integer output-frame domain used by deterministic canonical evaluation without quantizing the UI/audio clocks or claiming unsupported painters are canonical.

- `playbackVisualFrameIndex` maps the continuously moving playback playhead to the containing output frame using the existing `startFrame` contract. The store playhead remains sub-frame/continuous and normal-playback audio activity/source timing remains time-addressed.
- `VideoPreviewCanvasLegacy` queries canonical FrameState for that candidate frame and, when admitted, consumes canonical source time, transform/opacity, camera-relative `view_transform`, media geometry, clip effects, scene effects, and source-over pair transition paint. Exact paused video presentation tokens and the bounded deterministic seek nudge remain parity/deterministic-only.
- `resolvePreviewPlaybackCanonicalization` makes admission an all-frame fail-closed decision. The top-level FrameState and every visual layer must be authoritative; every visual layer must be an image/video media painter with canonical layer state; and transition composition must be clean `canonical-none` or `canonical-source-over`.
- Text, shapes, cursor, missing/non-media raster sources, non-authoritative state, legacy transition plans, weighted/deferred pairs, mixed transition plans, and explicit transition deferrals keep the **entire visual frame** on the existing continuous-time painter. This prevents partial semantic mixing while canonical non-media/weighted playback consumers remain deterministic-only.
- Preview diagnostics expose `deterministic-canonical`, `canonical-playback`, `legacy-time-fallback`, or `legacy-time`, the canonical/candidate frame index, scene-effect mode, and explicit playback deferral reason. Rollback is therefore observable instead of silent.
- The cursor raster-source audit preceding this slice found that browser `cursor-state-v1` painting exists but the current FFmpeg export renderer does not expose corresponding cursor rendering in the renderer path. Cursor therefore remains fail-closed rather than promoting a preview-only painter as export-equivalent. General shape raster admission also remains deferred because of the legacy FFmpeg approximations recorded by #299.
- Guarded run `33360924557` passed 8 focused Vitest files / 56 tests, repository frontend lint, and the production build before the initial playback implementation was committed. Guarded run `33361092075` then passed dedicated playback admission/fallback coverage plus the focused timing/index/geometry/transform/view/perspective/transition/effect suite, lint, and build before committing the media-only gate.
- Guarded contract-cleanup run `33361248348` passed the expanded focused suite, lint, and production build after aligning comments/parameter naming with admitted playback semantics. Earlier guard failures stopped before product commits and were corrected rather than bypassed.
- All temporary patch workflows/scripts were removed before PR creation. Clean pre-tracker head `e3aa6db4a4c13ec193f0c84df277b0a6ad395dff` is directly ahead of #299's actual squash result and behind `main` by zero.

### #301 retained normal-playback browser evidence

#301 adds retained decoded-browser evidence for #300's normal-playback admission contract without broadening any painter or raster-source class.

- `parity-playback-canonical-v1` is a backend-validated 18,000 ms / 640×360 / 30 fps timeline with one continuously active audio source and seven isolated playback windows: admitted video, admitted image, text fallback, cursor fallback, weighted-transition fallback, mixed-transition fallback, and a valid non-adjacent pair-transition deferral.
- The capture harness seeds the project through the production Video API, reloads the same immutable saved timeline before each case, issues the established parity-seek command until the editor itself reports the requested deterministic canonical frame, proves any mounted video decoder is settled, then starts ordinary free-running playback and samples the real preview DOM on `requestAnimationFrame`.
- The strict gate requires exact playback mode, exact fallback reason, actual consumer transition-plan mode, advancing continuous timeline/audio clocks, canonical frame candidate/consumer identity equality for admitted cases, multiple advancing canonical frames, and explicit proof that UI/audio time is not quantized to visual-frame boundaries. Fallback cases must publish no canonical visual-frame identity.
- Initial retained attempts exposed harness-observability debt rather than product semantic mismatches: run #1 blocked on a redundant expected-state pre-wait; runs #2/#4 exposed a route/effect-listener readiness race after earlier cases. The final harness removes the redundant pre-wait and acknowledges seeks from editor DOM state while retaining the same production seek command and strict semantic assertions.
- Retained **Video Playback Canonical Parity Evidence #5**, run `33399397597`, passed on pre-tracker head `c2effbfe6973f0f039d5651c68075f11798dfd8f`. Artifact `9760787171` is 11,684,163 bytes with SHA-256 `ed95b4b4a6f1334ce6f35863c2a7a0c1f82f549d69604ef8831869bb73cf6b2d`; retained timeline SHA-256 is `1a58a82698ef40f3c5e2b4c4c47b5f1ca07fd3a56cb4785126923a7202fc8110`.
- The retained toolchain is Go `1.25.13`, Node `24.19.0`, npm `11.17.0`, Playwright `1.62.1`, and FFmpeg/ffprobe `6.1.1-3ubuntu5` on Ubuntu 24.04.
- `video-canonical-playback` passed 38 observations: timeline advanced 713 ms, audio advanced 0.756792 s, and canonical visual identity advanced through 20 unique frames (`9`–`30`) with candidate/consumer/parity frame equality and non-quantized UI/audio clocks.
- `image-canonical-playback` passed 40 observations: timeline advanced 626 ms, audio advanced 0.639121 s, and canonical visual identity advanced through 17 unique frames (`69`–`87`) with the same clock-independence guarantees.
- `text-fallback` passed 27 observations with no canonical visual-frame publication, legacy consumer transition mode, timeline/audio advances of 430 ms / 0.425134 s, and exact reason `unsupported-playback-painter:playback-text`.
- `cursor-fallback` passed 27 observations with no canonical visual-frame publication, legacy consumer transition mode, timeline/audio advances of 426 ms / 0.431586 s, and exact reason `unsupported-playback-painter:playback-cursor`.
- `weighted-transition-fallback` passed 28 observations with timeline/audio advances of 450 ms / 0.461262 s and exact reason `transition-plan-weighted-deferred`; `mixed-transition-fallback` passed 19 observations with 417 ms / 0.429671 s and exact reason `transition-plan-mixed`; `deferred-transition-fallback` passed 28 observations with 432 ms / 0.432273 s and exact reason `transition-deferred:deferred-slide:pair-inputs-not-adjacent`. All three correctly expose legacy consumer transition mode after whole-frame fallback.
- #301 remains evidence-only: no product painter, canonical contract, playback admission, or renderer capability is broadened. The next implementation slice must separately prove runtime readiness before weighted-pair Canvas can become normal-playback visual authority.


## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #289 added the byte-exact opaque-PNG gate; #291 added H.264 measurement; #293 proves decoded frame identity on 0/15/30/59 and gates the focused ±3 H.264/yuv420p envelope only after identity proof. #295 adds deterministic partial-alpha PNG/background-composition evidence; #296 adds retained VP9 alpha-decoder, requested/presented-frame, and focused ±4 transparent-video evidence; #299 adds retained zero-readiness project-background evidence at focused ±1; #301 adds retained seven-window free-running browser playback evidence with independent continuous UI/audio clocks. Resource-font fixture coverage and second-platform retained evidence remain. |
| Phase 1 — Immutable submission | **Complete** | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend dirty/concurrency behavior are implemented. |
| Phase 2 — Canonical contract | **Complete** | Frame/range/source/order, curves, transforms/geometry/projection, transitions, effects, text/fonts, shapes, cursor, immutable source provenance, and AudioGraph semantics are versioned and cross-runtime checked. #260 closed the final contract gap. |
| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, font readiness/layout snapshots, deterministic pixelate semantics, exact opaque-PNG evidence, and sampled-render fan-out. #291/#293 close retained H.264 measurement and deterministic frame identity. #295 merges renderer-ordered project-background/source-over composition for transparent images; #296 adds mounted-video presentation tokens, transparent VP9 alpha preservation, source-over video composition, and browser/FFmpeg boundary alignment without changing canonical source/audio time; #299 admits project background itself as an explicit zero-readiness pixelate raster source. #300 merged authoritative media-only normal playback in the canonical output-frame domain with whole-frame fail-closed diagnostics, and #301 now retains real browser evidence for that contract. Weighted-pair normal-playback execution, mixed/deferred transition playback, canonical text/shape/cursor playback painters, further independently evidenced raster classes, and AudioGraph consumption remain. |
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
- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #291 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness; #293 freezes decoded H.264 frame identity/color acceptance; #295 admits renderer-ordered transparent-image source-over composition; #296 proves mounted transparent-video presentation readiness and alpha-preserving VP9 decode; #299 adds the project background itself as an explicit zero-readiness pixelate raster source without broadening legacy shape approximations.
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

### #302 ready weighted normal-playback Canvas promotion

#302 promotes the already-defined media-only weighted transition Canvas consumer into ordinary free-running playback while preserving one source-time/decoder authority and complete-frame fail-closed semantics.

- The branch starts directly from #301's actual squash result `1f891dc4338853357dcfdead8c073c9c81516bba`; no stale stacked base was carried forward.
- A playback-only runtime registry binds stable weighted-pair topology identity to exact canonical output-frame execution keys. Candidate weighted frames remain on complete-frame legacy-time fallback until every active weighted surface proves readiness for the matching execution key.
- The consumer reuses the already-mounted preview `<video>` / `<img>` nodes and the existing canonical weighted linear-sRGB pair-pixel kernel. It does not introduce a second decoder, seek loop, or source-time clock.
- Weighted crossfade, zoom, and dip surfaces are promoted only as `canonical-weighted-canvas` while the preview owns the matching `canonical-playback` frame. Decoder-budget posters, mixed plans, structural transition deferrals, unsupported painters, and Canvas/readiness failures remain explicit fallback paths.
- Retained debugging exposed a real lifecycle defect rather than a slow-GPU problem: canonical per-layer perspective changed `renderLayer()`'s React root shape, which recreated the mounted `<video>` after admission and reset decoder readiness. The final implementation keeps one stable perspective host across fallback/canonical promotion so media DOM identity and decoder state survive the transition; the evidence gate was not weakened.
- `parity-playback-canonical-v2` expands retained browser coverage to 10 cases: admitted video, admitted image, text fallback, cursor fallback, ready weighted crossfade, ready weighted zoom, ready weighted dip, forced decoder-budget weighted fallback, mixed-transition fallback, and non-adjacent structural transition deferral.
- Pre-tracker exact head `630f71526b3ae4e01a84d34553520d90b3304ac1` passed Video Playback Canonical Parity Evidence #50, Quality Gate #1826, Security Scan #1832, Video Transparent Pixelate Parity Evidence #64, and every applicable Linux/macOS/browser/container assurance workflow — 11/11 PR workflows successful.
- Playback evidence #50 retained timeline SHA-256 `dc42912f52e09986df18ca962ea1c0b5688aabc765e214daf62a1a70e7d26f97`. Artifact `9851179481` is 11,690,312 bytes with SHA-256 `8447638c4b3aa831ce527d242e79838cf67ba063af9f77be4ba3a1c342e8c330`.
- All three weighted canonical windows proved runtime `ready`, consumer `canonical-weighted-canvas`, exactly 1/1 weighted surface ready, no weighted errors or pending reasons, matching frame execution keys, and advancing canonical visual-frame identity while the continuous timeline/audio clocks advanced. Crossfade observed canonical frames `209–219`, zoom `286–297`, and dip `365–376` within their sampled windows.
- The forced decoder-budget case remained `legacy-time-fallback` with exact reason `transition-weighted-runtime-deferred:weighted-budget-crossfade-out:decoder-budget-poster`; mixed composition remained fallback with `transition-plan-mixed`; the non-adjacent pair remained fallback with `transition-deferred:deferred-slide:pair-inputs-not-adjacent`. Text and cursor remain deliberately unsupported for normal-playback canonical promotion in this slice.
- Quality #1826 passed frontend lint/unit/performance/build, backend formatting/vet/tests/race detector, the complete Playwright smoke suite, Windows/macOS platform checks, and the renderer parity baseline. Retained `video-parity-baseline` artifact `9851748601` is 53,867,919 bytes with SHA-256 `25b8e0f1ccf31bf518f7844b1ca95791d7f0fe5e4fc53e8544b2ce8002656bc0`.
- Security #1832 passed Go and JavaScript/TypeScript CodeQL plus Go/root-npm/frontend-npm dependency audits. The branch also moves the vulnerable transitive `browserslist` 4.28.6 lock entry to the fixed 4.28.8 release using npm-generated lock metadata.
- Transparent-video parity #64 retained artifact `9851138935` (17,396,827 bytes, SHA-256 `c2afb92d74d52cecb265124c182c947bcab5cb8dd3e4288f2c44dfe088ba55ed`), preserving the previously gated VP9-alpha path while this playback work changes media host structure.
- This documentation commit changes the exact PR head. The successful `630f7152...` run is implementation-tree evidence only; the tracker-bearing exact head must repeat the full applicable gate set before #302 is marked ready or merged.

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
| #293 | Deterministic decoded-video frame identity and focused ±3 H.264 pixelate gate | `9850370c2aa25a076f3272077062aeab08c1f326` |
| #295 | Renderer-ordered partial-alpha image composition and focused ±1 PNG gate | `f6a08f72910677ed538e356d544a1d5d1b59d620` |
| #296 | Mounted-video presentation tokens, VP9 alpha preservation, and focused ±4 transparent-video gate | `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9` |
| #299 | Explicit zero-readiness project-background pixelate raster and focused ±1 gate | `f225175e9404762b872944cd2a0ddda0e8e8284f` |
| #300 | Media-only normal-playback canonical output-frame admission with whole-frame fallback | `d4049105d49ce4bc8337a3c5236593f503869d5d` |
| #301 | Retained normal-playback canonicalization browser evidence | `1f891dc4338853357dcfdead8c073c9c81516bba` |

## Safe stacked-branch normalization

Every stacked slice starts from the **actual squash result on current `main`**:

1. Read current `main` commit/tree.
2. Create the child directly from the actual parent squash SHA.
3. Verify `compare main...branch` contains only intended paths and is behind by zero.
4. Update this tracker on the clean branch.
5. Validate executed checks on the exact head; never call an unexecuted/cancelled head green.
6. Audit comments/reviews/threads and merge with expected-head protection.
7. Create the next slice from the new actual squash result.

Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; #290 validated from #289 squash; #291 mirrored that exact validated #290 head and merged as `7d5f36c3...`; #292 validated directly from #291; #293 mirrored exact validated #292 head and squash-merged as `9850370c2aa25a076f3272077062aeab08c1f326`; #294 validated directly from #293; #295 promoted exact validated #294 head `66722d2c...` and squash-merged as `f6a08f72910677ed538e356d544a1d5d1b59d620`; #296 was directly from #295 and squash-merged as `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`; #299 was directly from #296 and squash-merged as `f225175e9404762b872944cd2a0ddda0e8e8284f`; #300 was directly from #299 and squash-merged as `d4049105d49ce4bc8337a3c5236593f503869d5d`; #301 was directly from #300 and squash-merged as `1f891dc4338853357dcfdead8c073c9c81516bba`; **#302 is directly from #301 squash `1f891dc4338853357dcfdead8c073c9c81516bba`**.

## Phase 0 parity baseline

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

The focused `parity-pixelate-opaque-v1` fixture is a byte-exact non-regression gate for the isolated opaque-PNG static pixelate path. `parity-pixelate-decoded-video-v1` is additive decoded-media evidence: #293 proves deterministic decoded-frame identity first and then applies the explicit ±3 RGB H.264/yuv420p pixelate-region envelope without changing repository-global parity defaults. `parity-pixelate-alpha-png-v1` is #295's additive straight-alpha/source-over project-background control. `parity-pixelate-alpha-video-v1` is #296's additive transparent VP9 control: it freezes the decoder-alpha contract, requires mounted-video presentation-token identity on frames `0/15/30/59`, and gates the pixelate region at 100% within ±4 RGB. `parity-pixelate-background-v1` is #299's zero-media control: it requires the explicit `project-background` raster source, no decoder/presentation token, and 100% of the retained region within ±1 RGB.

`parity-playback-canonical-v1` is #301's retained free-running browser control for #300. It proves admitted image/video windows consume advancing canonical output-frame identity while the store/UI and audio clocks remain continuous, and separately proves text, cursor, weighted, mixed, and canonically deferred transition windows fail the whole visual frame back to legacy time with exact reasons and no canonical visual-frame publication.

`parity-playback-canonical-v2` is #302's additive normal-playback authority/readiness control. It preserves the admitted video/image and text/cursor fallback cases, promotes ready media-only weighted crossfade/zoom/dip through the existing Canvas consumer with exact per-frame execution-key evidence, and separately proves decoder-budget-not-ready, mixed, and structurally deferred weighted cases still fail the complete visual frame closed. This is structural/runtime browser evidence; it does not redefine the independent pixel-equality gates for deterministic rendered-media fixtures.

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
| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural admission; #287 owns FFmpeg scaler selection; #288 owns runtime acquisition; #289 owns byte-exact PNG evidence; #291 owns decoded-video measurement; #293 owns decoded-frame identity/codec acceptance; #295 owns project-background/source-over image-alpha composition; #296 owns mounted-video presentation/transparent-alpha evidence; #299 owns explicit zero-readiness project-background raster admission. None is substituted for another. |
| MIME type or codec is treated as proof of an opaque backdrop | #295 does not infer image opacity from MIME. #296 separates video presentation readiness from alpha content: the mounted `<video>` must present the matching request token first, then decoded alpha is source-over composited. Codec metadata selects `libvpx-vp9` only to preserve an already-probed VP9 alpha stream; it is not an opacity shortcut. |
| Newly sought video reports data before its frame is Canvas-rasterizable | #296 moves proof to the mounted video owner: every deterministic seek clears a presentation request token and only a matching post-seek `requestVideoFrameCallback` can mark it ready. Pixelate Canvas stays fail-closed until that token matches. |
| A second decoder becomes a competing source-time authority | The exact Canvas path reuses the already-mounted preview `<video>`/`<img>` and paints only that existing decoded frame. |
| Browser or FFmpeg selects a neighboring decoded frame at a quantized source boundary | #296 keeps canonical source time unchanged but aligns deterministic visual presentation on both consumers: browser seeks use a bounded 0.49 ms inside-frame target; FFmpeg visuals use a 1 MHz filter timebase plus 490 µs PTS bias before rate retiming. Audio remains unchanged. Retained VP9 frame 59 presents `mediaTime=1.967000000` and passes the focused ±4 gate. |
| Sampled fidelity expansion multiplies FFmpeg decoder instances | #289 indexes immutable source paths once and uses deterministic `split` / `asplit` fan-out so hundreds of sampled consumers preserve independent semantics without reopening the same asset hundreds of times. |
| Pixelate target host applies a second transform to exact bytes | The Canvas stays hidden while proving readiness; only then is the existing target host normalized to full-stage geometry and the CSS painter hidden. |
| FFmpeg/static-region behavior is confused with canonical transform support | The pixelate planner explicitly defers target transform keyframes and explicit axis scale that diverges from legacy scalar `scale`. |
| Evidence accidentally measures CSS fallback instead of Canvas | Browser assertion requires `canonical-canvas`, ready surface/host markers, no deferred/error markers, and absence of `pixelate-css-approximation` for every sampled frame. |
| A diagnostic `--allow-fail` report is mistaken for exact parity | Reports retain pass rate, SSIM, MAE/RMSE, max-channel delta, and exact consumer identity. Workflow success proves evidence completeness unless an explicit threshold gate is configured. |
| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | #295 bounds lossless image source-over rounding to ±1 RGB at 100% pixel pass. #296 separately proves VP9 alpha preservation requires explicit `libvpx-vp9` in the hosted FFmpeg toolchain and freezes a focused ±4 transparent-video region envelope after exact presentation identity; repository-global ±2 diagnostics remain unchanged. |
| Codec-noisy decoded equality is treated as structural parity | PNG structural exactness remains independent; #293 gates decoded-video color at ±3 only after source-frame identity is proven, while repository-default ±2 diagnostics remain unchanged. |
| A renderer-specific painter approximation is mistaken for an exact raster source | #299 explicitly rejects broad shape admission after inspecting current FFmpeg shape approximations. New painter/source classes require independent ordering, readiness, geometry, and retained pixel evidence before classifier admission. |
| Normal playback mixes canonical and legacy semantics inside one visual frame | #300 admits canonical playback only for an authoritative media-only whole frame with clean none/source-over transition composition. Text/shape/cursor, non-authoritative state, weighted/mixed/deferred transitions, and unsupported raster sources roll the whole visual frame back to the continuous-time painter with an explicit diagnostic reason. #301 retains real browser proof that admitted video/image visual frames advance canonically while UI/audio clocks remain continuous and that every covered unsupported class publishes no canonical visual frame. |
| CI scheduling hides code state | Only actually executed checks count. |

## Next recommended slice

1. **Merge #302 only after this tracker-bearing exact head re-passes Video Playback Canonical Parity Evidence, Quality/Security, the full Playwright smoke and renderer baseline included by Quality, transparent-video parity, and all applicable platform/sandbox assurances.** The successful pre-tracker #50 run proves the implementation tree; it is not permission to skip exact-head validation after this documentation update.
2. **Start the next Phase 3 slice directly from #302's actual squash result and establish normal-playback text readiness/ownership before promoting text.** Reuse the existing canonical text painter, exact `FontFace` resource readiness, and Chromium text-layout stabilization contract; do not turn browser layout output into a second authored text semantic contract.
3. Retain normal-playback browser evidence for a ready resource-backed text case plus explicit face-not-ready/layout-not-ready/failure and mixed-frame fallback cases. Keep whole-frame authority fail-closed so one visual frame never mixes canonical text with legacy-time media semantics.
4. Keep cursor normal-playback admission deferred until export implements matching cursor rendering semantics. Keep general shape playback deferred until the current FFmpeg approximations are removed or replaced by a shared deterministic painter. Family-name-only text also remains insufficient evidence for cross-machine face identity; resource-backed provenance stays the deterministic path.
