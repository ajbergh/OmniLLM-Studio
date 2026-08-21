# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-21
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG feature PR: **#251 — Project immutable source provenance into FrameState** — squash merge `d49808f1ea7fc23f658e95f52fdbe404bf0be92a`.

CI coverage prerequisite merged immediately afterward: **#246 — Run canonical render-contract suite in Vitest** — merge `ad7ec51596807293ebb1206ce873088a0ad07da7`.

Current draft PR: **#252 — Define canonical font-resource provenance** (`feat/video-wysiwyg-phase2-font-provenance`), created directly from merged #251 (`d49808f1ea7fc23f658e95f52fdbe404bf0be92a`). It defines the manifest-backed `font-resource-provenance-v1` identity/package contract for explicit static font faces. It deliberately does not add font upload/snapshot creation, choose a text face from a family name, use a system-font fallback, or claim intrinsic glyph metrics; those remain explicit follow-on work. It is open as a draft with no reviews or comments; hosted validation is in progress.

PR #243 is complete. `shape-state-v1` is projected into `visual-frame-state-v1`, evaluated shape dimensions are the single shape-derived content-bounds source, and generic `shape` unresolved debt is removed. It intentionally retained cursor debt, which #247 now consumes canonically.

PR #246 fixed a test-discovery gap that had excluded the canonical TypeScript render-contract suite under `frontend/test/` from `npm run test:unit` and the hosted Quality Gate. The normal frontend unit gate now executes 51 files / 248 tests instead of 28 files / 131 tests, so every Phase 2 TypeScript contract mirror added under `frontend/test/` is exercised in CI.

PR #245 defines the renderer-independent `cursor-state-v1` evaluator. Merged #247 consumes it in Go and TypeScript `visual-frame-state-v1`: valid visible cursor state is serialized at exact clip-relative rational time, valid hidden/empty state is intentional no-paint, and malformed or unsupported cursor authoring fails closed. Preview painting and legacy FFmpeg cursor composition remain unchanged.

No Phase 3 preview compositor or Phase 4 Chromium renderer behavior is changed by #245, #247, #251, or the current font-resource-contract branch.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Timing, curves, v1 adapter, frame/range/source/order, normalization, frame addressing, property evaluation, FrameState, media geometry, perspective, all current transition state/paint families, effect stack state, canonical text/shape state, cursor FrameState consumption (#247), and immutable source-provenance/anchor consumption (#251) are merged. Manifest-backed font-resource provenance is in progress; snapshot packaging/text-face selection/glyph metrics and AudioGraph remain. |
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

### Media geometry

`media-geometry-v1` is authoritative for asset geometry:

- source aspect ratio requires explicit `content_bounds` or the current versioned immutable source-probe projection;
- source dimensions are never guessed from the output canvas;
- `mask_source_crop` operates in source coordinates before fit;
- `contain`, `cover`, `fill`, and `none` are canonical fit modes;
- `transform.crop` is a separate post-fit output viewport clip;
- FrameState carries evaluated painted bounds and source provenance;
- missing source provenance remains explicit unresolved state.

### Perspective and stacking

Perspective is projection state, not paint order. Track/z-index order remains authoritative for stacking; spatial `z` affects projection.

`perspective-projection-v1` preserves the preview-compatible 1200-canvas-pixel distance with no scene camera, derives projection distance from evaluated camera FOV when a camera is active, allows a positive per-clip perspective override, and serializes projection separately from the camera-relative model matrix.

### Transition timing and paint

`transition-state-v1` makes placement, owner/peer roles, windows, progress, and real-overlap requirements explicit. No hidden source handles or inferred adjacency are invented.

`transition-paint-v1` is renderer-neutral composition state. Every currently authorable Timeline v2 transition type has canonical paint semantics and FrameState consumption:

- `fade`: one-sided owner opacity;
- `crossfade`: isolated-surface pair blend;
- `dip_to_black`: explicit outgoing/black/incoming contribution weights;
- `slide`: normalized canvas-fraction translations;
- `wipe`: normalized layer-fraction clip insets;
- `zoom`: canonical scale envelope around the evaluated authored anchor plus one-sided opacity or pair weights.

Phase 3 consumers must apply pair operations to isolated surfaces and must not reinterpret pair paint as independent stacked layer opacity/transform.

### Effect stack

`effect-state-v1` is the renderer-independent evaluated effect operation carried by FrameState.

Merged #237 semantics:

- only enabled effects enter the evaluated stack;
- authored effect-array order is preserved with the original array index;
- scope is explicit as `clip` or `scene`;
- defaults and numeric bounds are grounded in the existing effect registry;
- chroma-key color defaults to `#00FF00` when not authored;
- clip `effect.<id>.amount` automation is sampled at exact output-frame presentation time, with `effect.<type>.amount` retained as a compatibility fallback;
- scene effects remain static because Timeline v2 has no scene-effect keyframe collection;
- unknown effect types/parameters, non-finite values, and undefined amount automation fail closed;
- FrameState carries clip and active-scene effect stacks and no longer marks supported effects as generic unresolved debt.

### Text renderer state

`text-state-v1` is the renderer-independent evaluated text styling contract merged in #241.

Merged #241 semantics:

- text content, font family/source, size, weight, foreground/background, stroke, shadow, alignment, line height, letter spacing, border radius, box dimensions, and per-side padding are explicit serialized state;
- preview-compatible defaults are canonicalized: `round(canvasHeight/18)` font size, weight `700`, white foreground, centered horizontal alignment, middle composition alignment, 2px stroke width when a stroke exists without a positive authored width, and the existing 2/2/4 black shadow semantics;
- background text receives the current preview-compatible default padding of 8px vertical and 18px horizontal unless authored per-side padding overrides it;
- missing font family remains explicit `composition-default` provenance instead of being replaced with a guessed font name;
- line height distinguishes `normal` from an authored multiplier rather than serializing both as an ambiguous zero value;
- explicit `box_width` and `box_height` become text content bounds only when both are present and positive; intrinsic glyph bounds are not guessed from text length or browser metrics;
- Timeline v2 text `params` remains non-authoritative extension metadata and is not silently promoted into renderer semantics;
- Timeline v2 currently defines no text-style keyframe property family, so #241 deliberately does not invent text animation semantics;
- invalid alignment, non-finite numbers, negative dimensions/padding/style widths, and non-positive authored box dimensions fail closed;
- FrameState carries canonical text state and no longer carries generic text unresolved debt.

Compatibility boundary for #241:

- the legacy preview already consumes most basic text fields and owns its current browser font metrics;
- explicit v2 vertical alignment, per-side padding, and box semantics are canonical authoring intent even where legacy preview/FFmpeg does not yet consume them directly;
- deterministic font packaging/resource provenance and intrinsic glyph measurement remain Phase 3–5 work and must not be guessed in Phase 2.

### Shape renderer state

`shape-state-v1` is the renderer-independent static annotation contract merged in #242 and consumed by FrameState in #243.

Merged shape semantics:

- all 14 currently authorable shape kinds are explicit: rectangle, highlight, blur, rounded rectangle, ellipse, arrow, line, speech bubble, spotlight, pixelate, checkmark, x mark, step marker, and label;
- dimensions are canvas pixels and default to current preview-compatible 320×180 when not authored;
- stroked primitive kinds default to `#f59e0b` with 6px stroke width; checkmark and x-mark instead default to `#22c55e` and `#ef4444`; speech bubble and label default to **no stroke** because their preview border is conditional on an authored `shape.stroke`;
- blur/pixel block radius defaults to 12px;
- kind-specific fill defaults are explicit: highlight `#facc15`, spotlight `rgba(0,0,0,0.6)`, step marker `#2563eb`, speech bubble white, label `#1e293b`, otherwise transparent;
- rounded rectangle, speech bubble, and label corner-radius defaults are 12px, 18px, and 10px respectively;
- positive authored stroke width and blur radius preserve the preview minimum of 1px; positive authored corner radius overrides the kind default, while authored zero preserves the preview fallback to the kind default for rounded rectangle/speech bubble/label;
- an explicit non-empty authored callout stroke enables that border and overrides the no-stroke default;
- unsupported kinds, non-positive dimensions, negative numeric style values, and non-finite numeric style values fail closed;
- embedded callout text remains the separate `text-state-v1` projection and is not duplicated into shape state;
- legacy FFmpeg square-corner/vector omissions and pixelate implementation differences are explicitly not contract semantics.

Merged #243 FrameState consumption rules:

- `FrameLayerState.shape` carries the evaluated `shape-state-v1` object in both Go and TypeScript;
- shape-derived `content_bounds` come from evaluated canonical shape dimensions rather than a second 320×180/defaulting implementation;
- invalid shape dimensions/kinds fail through FrameState instead of being silently replaced by local bounds defaults;
- explicit clip `content_bounds` still take precedence over shape-derived bounds;
- generic `shape` unresolved debt is removed once evaluation succeeds;
- no preview/export painter consumes the new shape field yet; that remains Phase 3/4 work.

### Cursor renderer state

`cursor-state-v1` is the renderer-independent sampled cursor contract defined by #245 and consumed by `visual-frame-state-v1` in merged #247.

Merged #245 semantics:

- Timeline v2 `visible` preserves presence in Go: omitted means visible, explicit `false` means hidden;
- cursor position is sampled at exact clip-relative rational presentation time;
- event interpolation is linear and endpoint-held to match the current editor preview;
- event coordinates remain canvas pixels from the top-left;
- click proximity is the preview-compatible strict `<300ms` window around authored click events;
- `click` is sampled press proximity, while a consumer renders a ring only when both sampled `click` and authored `click_rings` are true;
- scale defaults to 1 and is constrained to the established preview-compatible 0.25–4 range;
- highlight and click-ring enablement remain explicit booleans;
- `smoothing:true` fails closed because the current editor has no defined canonical smoothing algorithm;
- negative event times, unordered event streams, non-finite coordinates, non-finite scale, invalid scale, and invalid rational denominator fail closed;
- empty events or explicit hidden visibility yield no evaluated cursor state;
- no custom cursor asset/resource contract is invented in v1.

FrameState-consumption rules in #247:

- `FrameLayerState.cursor` / `CanonicalFrameLayerState.cursor` carries the evaluated `cursor-state-v1` object when the cursor is visible and has events;
- both runtimes sample with the exact clip-relative rational presentation time `(frameIndex × 1000 - clip.start_ms × fps) / fps`, not rounded milliseconds;
- valid explicit-hidden or event-empty cursor metadata produces no cursor state and no generic unresolved debt;
- invalid cursor state, including undefined smoothing, fails the whole FrameState evaluation closed;
- shared Go/TypeScript FrameState fixtures cover the fractional 120-fps sample, click state, no-paint cases, and fail-closed propagation;
- legacy Timeline v1 cursor visibility remains plain-bool and therefore cannot distinguish omission from explicit false. That compatibility limitation is retained as implementation debt rather than promoted into canonical v2 semantics;
- preview painter and FFmpeg cursor overlay behavior remain unchanged; Phase 3/4 consumers must use the serialized canonical state rather than re-evaluating it.

### Safe stacked-branch normalization

A stacked PR must be rebuilt from the **actual current `main` tree** after its parent merges.

Required procedure:

1. Read current `main` commit and tree SHA from GitHub.
2. Identify only the intended child-delta blobs/files.
3. Create a new tree using current `main` as `base_tree_sha` plus those child blobs.
4. Create a new commit whose only parent is current `main`.
5. Force-update the child branch to the clean commit and retarget its PR to `main`.
6. Run `compare main...branch` and verify there are no unrelated changes/deletions.
7. Update this tracker on the clean branch.
8. Validate the exact final head before merge.

**Do not** manufacture ancestry by making a stale feature tree a merge commit with `main` as an additional parent. That mistake was caught on #225 before merge and would have silently reverted unrelated sandbox-worker files despite Git reporting current ancestry.

## Phase 0 evidence and remaining sign-off

The deterministic `parity-torture-v1` fixture covers 20 seconds at 640×360/30 fps with 103 named frame samples spanning boundaries, rates, transforms, curves, text, shapes, effects, transitions, camera, captions, cursor, and audio.

Retained evidence `32074904557` / artifact `9303432653` established exact frame/audio/delivery identity mechanics. The visual baseline remains intentionally a known-mismatch diagnostic baseline, not a production threshold.

Remaining Phase 0 sign-off:

1. Freeze production visual threshold policy and zero-tolerance structural regions.
2. Define unsupported-audio policy for pitch preservation, custom gain curves, and program processing until Phase 6.
3. Run full evidence on a second supported OS/FFmpeg environment and record deltas.
4. Keep the fixture runnable throughout Phases 2–7.

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

Security unblock during the program:

- #201 replaced reachable-vulnerable `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.

CI reliability unblocks:

- #219 bounded/retried Linux dependency installation and Playwright bootstrap and added job-level timeouts — `a33b32697019b144c9a7d6c7fec277e1cde101b4`.
- #226 quiesced competing apt activity before Playwright retry setup — `ffbf797108c291c927fc7b67f6154b29c1351496`.
- #239 aligned the durable sandbox-worker test SQLite helper with production WAL/busy-timeout behavior, eliminating a false `SQLITE_BUSY` blocker encountered while #237 was being replayed over concurrent sandbox work.
- #246 expanded `frontend/vitest.config.ts` to execute canonical contract tests under `frontend/test/`; the normal frontend unit gate increased from 28 files / 131 tests to 51 files / 248 tests. Its first expanded run exposed and corrected three stale pre-paint transition assertions plus one bit-exact floating-point fixture assertion before merge as `ad7ec51596807293ebb1206ce873088a0ad07da7`.

### Merged PR #245 — canonical cursor renderer state definition

Implemented before this tracker update:

- Go `EvaluatedCursorState` and TypeScript `CanonicalEvaluatedCursorState` projections with contract version `cursor-state-v1`;
- exact rational cursor sampling in both runtimes;
- preview-compatible linear interpolation and endpoint hold;
- strict `<300ms` click-proximity semantics;
- omitted-visible versus explicit-hidden presence semantics in the Go Timeline v2 projection;
- preview-compatible default/range behavior for cursor scale;
- explicit highlight/click-ring state;
- fail-closed smoothing policy until a canonical algorithm exists;
- malformed-event validation for negative time, ordering, finite coordinates, scale, and rational denominator;
- mirrored Go/TypeScript unit coverage;
- two minimal existing Go fixture updates required by `Visible bool` → `*bool`.

Merge record:

- #246 merged first so canonical TypeScript contract tests under `frontend/test/` would be exercised by the normal frontend unit gate;
- the seven-file cursor-contract delta was rebuilt directly on then-current `main` after #246;
- #245 squash-merged as `c428d81f25ce1d85faa6655fb5772430a8fe6b22` after the exact-head hosted matrix completed successfully;
- this PR defines canonical state only: preview and FFmpeg painter behavior remain unchanged until later consumer work.

### Merged PR #247 — cursor FrameState consumption

Implemented in the normalized 12-path delta:

- project evaluated cursor state into Go `FrameLayerState.Cursor` and TypeScript `CanonicalFrameLayerState.cursor`;
- evaluate at exact clip-relative rational output-frame time, remove generic cursor debt only after successful evaluation, and fail the FrameState closed on invalid cursor semantics;
- make explicit-hidden and event-empty metadata authoritative no-paint state;
- add mirrored Go/TypeScript focused tests and shared permanent `visual-frame-state-v1.json` fixture expectations;
- retain the scope boundary: no preview painter, legacy FFmpeg compositor, or Phase 4 browser-renderer behavior changes.

Status at 2026-08-21: PR #247 had no review submissions or inline threads. Its former hosted matrix passed on the prior base, then `main` advanced; the seven-file cursor delta was rebuilt as the intended 12-path normalized head directly on current `main`. Focused Go render-contract tests, frontend lint (9 pre-existing warnings and no errors), 54 frontend unit files / 266 tests, the video-performance fixture, and the production build passed locally. The full Go suite's unrelated Windows symlink limitation, unavailable local GCC race detector, and one unrelated Playwright workspace-collapse timeout were recorded rather than hidden. The complete exact-head hosted Quality, security, container, browser, renderer-parity, and platform/sandbox matrix then passed. The PR was marked ready and squash-merged as `66530a07e3a5585546d978794e24198083bbeaa2`.

### Merged PR #251 — source-provenance FrameState consumption

The current `feat/video-wysiwyg-phase2-source-provenance` branch closes the concrete source-probe and anchor side of the remaining provenance debt without changing preview or FFmpeg painters:

- define pure `source-provenance-v1` from immutable Render Manifest v1 asset identity, clip bindings, content hash, and validated media-probe dimensions;
- add manifest-based Go and TypeScript FrameState entry points that use those source bounds only when authored clip `content_bounds` is absent;
- project serialized source provenance into each media `FrameLayerState`, use it for media geometry and anchor offsets, and never probe a file or guess a canvas-sized source box;
- keep timeline-only FrameState compatibility unchanged, while a manifest call with no eligible source probe remains non-authoritative with explicit source-provenance and anchor debt;
- reject partial/non-positive probe dimensions and a manifest source not bound to the active clip; shared Go/TypeScript fixtures prove success and fail-closed cases.

Local evidence: `go test ./internal/video/...`, `go vet ./...`, focused mirrored Vitest tests (13 tests), the complete frontend unit gate (55 files / 272 tests), the video-performance fixture, TypeScript lint with no errors, and the production frontend build passed. `go test ./...` reached an unrelated `internal/api` sandbox-workspace test but cannot resolve workspace-root symlinks in this Windows sandbox (`Access is denied`); the local race command cannot start because CGO needs a GCC compiler that is not installed. The browser smoke suite first left a test-only SQLite lock, which was released by stopping only its identified orphaned processes; its clean retry is then blocked by a pre-existing Vite development server on port 4173, which is intentionally not terminated. The hosted Quality Gate (including backend tests/race, frontend, Playwright, and video renderer parity), Security Scan, container builds, and platform/sandbox/browser assurances all passed with no review submissions or inline threads; #251 was marked ready and squash-merged as `d49808f1ea7fc23f658e95f52fdbe404bf0be92a` on 2026-08-21.

### Current draft PR #252 — font-resource provenance

The current `feat/video-wysiwyg-phase2-font-provenance` branch in draft PR #252 defines the first bounded font-resource slice without changing preview or FFmpeg painters:

- add optional `font_resources` to Render Manifest v1, with a canonical lowercase resource id, family, static weight/style, format, clean snapshot-relative staged path, content hash, and byte size;
- project the strict schema identically into Go and TypeScript, including required-key and enum drift checks;
- define pure `font-resource-provenance-v1` evaluators that return stable resource-id order and reject duplicate ids, malformed family metadata, unsupported style/format, unsafe paths, malformed hashes, and empty resource bytes;
- use shared Go/TypeScript fixtures to prove successful static-face package identity and fail-closed behavior;
- leave the absence of packaged fonts as explicit empty provenance, without manufacturing a system-font fallback;
- retain the scope boundary: snapshot production packaging, authored text-to-resource selection, variable-font policy, glyph-layout/metric ownership, intrinsic text bounds, preview painting, and FFmpeg composition are not changed or inferred here.

Current local evidence: focused `go test ./internal/video/rendercontract`, complete `go test ./internal/video/...`, and `go vet ./...` passed. Focused Vitest schema/provenance coverage (2 files / 15 tests), the complete frontend unit gate (56 files / 284 tests), the video-performance fixture, TypeScript lint (9 pre-existing warnings and no errors), and the production frontend build passed. `go test ./...` reached all video/contract packages but exits on an unrelated Windows privilege boundary when `internal/gitrepo` attempts to create a symlink (`A required privilege is not held by the client`). `go test -race ./...` cannot start because this environment has `CGO_ENABLED=0`. #252 is open as a draft on its exact head with no reviews/comments; hosted validation is in progress.

### Remaining Phase 2 work

After the current font-resource-contract branch:

1. **Complete font-resource ownership** — wire resources into immutable snapshot creation and require explicit authored text-face selection; then define variable-font policy and glyph-layout/metric ownership. Intrinsic text bounds remain non-canonical until this work exists.
2. **AudioGraph** — define serializable timing/rate/pitch/channel/gain/fade/mute/solo/processing/stem decisions and exact sample-count semantics.
3. Keep all unknown authorable fields fail closed until canonical semantics exist.

### Phase 2 exit gate

Preview and export callers consume identical FrameState/AudioGraph fixtures. No renderer owns separate curve, range, ordering, transform, geometry, projection, transition placement/activity/paint, effect, text, shape, cursor, source-time, or audio semantic math. Go/TypeScript schema/type/fixture drift fails CI.

## Phase 3 — Shared preview composition

Drive the Video Edit Studio program monitor from canonical FrameState/AudioGraph while preserving direct-manipulation UI state separately. Add diagnostics for frame identity, active clip IDs, source time, matrices/bounds/projection, transitions, effects, text, shapes/cursor, and audio graph identity.

## Phase 4 — Shared Chromium render worker

Build a deterministic browser composition entry point accepting immutable Render Manifest v1 + frame index. Package fonts/assets deterministically, manage browser health/concurrency/cancellation, and retain FFmpeg for decode/encode/mux behind a guarded rollout.

## Phase 5 — Visual parity closure

Close decoded output parity for media timing/fit/crop, transforms/anchors/2.5D/camera/z-order, opacity/blending, text metrics/fonts, shapes, transitions, effects, cursor, color space, and deterministic asset loading.

## Phase 6 — Audio parity closure

Build canonical 48-kHz stereo AudioGraph behavior for source time, rate/pitch, channels, mute/solo, gain automation, fades, program processing, processed stems, exact sample counts, and decoded delivery.

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
| Schema/type drift | Go reflection and TypeScript compile/Vitest projection checks fail CI. Canonical `frontend/test/` suites are now included by #246. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState diagnostics. |
| Legacy FFmpeg approximations become de facto contract | Canonical semantics are explicit; legacy renderer is evidence only. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed. |
| Millisecond rounding creates frame/source drift | Canonical rational frame/range/source helpers. |
| Source aspect ratio is guessed from canvas | Explicit source `content_bounds` takes precedence; otherwise only immutable `source-provenance-v1` media probes may supply source bounds, and missing provenance stays unresolved. |
| Crop ordering diverges | Source mask before fit; output crop after fit. |
| Perspective differs between preview/export | One canonical projection contract carried in FrameState. |
| Transition peer/handle inference differs | Explicit placement/peer/real-overlap semantics. |
| Pair transitions are misapplied as independent alpha layers | Pair paint instructions operate on isolated surfaces using canonical weights/transforms. |
| Spatial transition silently inherits sampled FFmpeg approximation | Slide/wipe/zoom use explicit renderer-neutral normalized geometry/scale contracts. |
| Effect ordering/defaults diverge between authoring, preview, and export | `effect-state-v1` preserves authored order and registry-grounded normalized parameters. |
| Unsupported effect metadata is silently ignored | Canonical effect evaluation fails closed for unknown types/parameters or undefined automation. |
| Text defaults drift between browser and export | `text-state-v1` serializes defaults and style intent once; consumers must not re-default independently. |
| Browser/system font fallback changes text metrics | Missing family remains explicit composition-default provenance; deterministic font packaging/metrics are required before parity closure. |
| Intrinsic text bounds are guessed | Only explicit positive box dimensions become canonical bounds until a deterministic glyph-layout contract exists. |
| Shape semantics inherit legacy FFmpeg omissions | `shape-state-v1` is grounded in authored/preview semantics; FFmpeg approximations are not canonical state. |
| Shape defaults drift between preview and export | Shared Go/TypeScript fixture serializes dimensions/style defaults once before FrameState consumption. |
| Shape bounds are defaulted twice | #243 derives shape `content_bounds` from evaluated `shape-state-v1`; FrameState does not independently reinterpret width/height defaults. |
| Invalid shape state is hidden by fallback bounds | Shape evaluation occurs before shape-derived bounds, so invalid authoring fails closed through FrameState. |
| Cursor omission collapses into hidden state | Timeline v2 Go `visible` is optional in #245; omission and explicit false are distinct. |
| Cursor smoothing diverges by renderer | `smoothing:true` fails closed until one versioned canonical algorithm exists. |
| Cursor click-ring timing drifts | `cursor-state-v1` serializes exact sampled click proximity using strict `<300ms` semantics. |
| Cursor FrameState claims authority before evaluation | #247 serializes `cursor-state-v1` only after exact rational evaluation; hidden/empty states are explicit no-paint and invalid semantics fail closed. |
| Source probe and anchor diverge by consumer | Draft #251 serializes immutable asset/clip/hash/source-bounds evidence once, supplies it to FrameState geometry and anchor math, and rejects partial or unbound manifest data. |
| Stacked branch appears current but carries stale tree | Rebuild from actual current `main`; compare every path before merge. |
| CI setup/runner saturation hides code state | Distinguish setup/queue from executed code checks. |
| Browser worker resource cost | Admission control, health checks, cancellation, guarded rollout, FFmpeg retained for media I/O. |
| Audio runtime differences | Explicit AudioGraph and unsupported-boundary policy before default shared export. |

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
- #218 canonicalized perspective projection; manual review caught and corrected the no-camera 1200px compatibility behavior before merge.
- #219 added bounded/retried Linux dependency/Playwright bootstrap and CI timeouts.
- #220 canonicalized transition placement/peer/role/progress state.
- #222 consumed transition state in FrameState and changed paint debt from clip-wide to active-frame scoped.
- #224 defined true fade/crossfade/dip-to-black paint and passed the complete exact-head Quality/Security/container/assurance matrix before merge.
- #225 consumed fade-family paint in FrameState. A pre-merge diff audit caught an unsafe synthetic ancestry/tree merge that would have reverted unrelated sandbox-worker changes; the branch was rebuilt cleanly from current `main` before merge.
- #227, #228, and #229 completed canonical slide, wipe, and zoom transition paint; each was normalized as concurrent work advanced `main` and merged only after exact-head validation.
- #237 defined `effect-state-v1`, projected clip/scene effects into FrameState, and extended permanent cross-runtime parity coverage. Concurrent sandbox work repeatedly advanced `main`; the final branch was replayed directly onto `fc83c95ddcda862a5499cae6898ba0e4623744b9`, passed the complete exact-head matrix, and merged as `22e73cc291a4f8723a99ad123c963aedf0fd0d8a`.
- #241 defined `text-state-v1`, projected canonical text into FrameState, and removed generic text debt. Its first hosted Quality Gate stopped at connector-authored Go formatting before semantic checks; all four files were gofmt-corrected, an undefined test helper was replaced with an explicit value, and the documentation-complete exact head passed the complete Quality/Security/container/platform/parity matrix before merge as `9b072685b689cdb74e0a5590a26478f6a3ef12b4`.

### 2026-08-20

- #242 defined renderer-neutral `shape-state-v1` for all currently authored annotation kinds. Manual review caught and corrected preview-compatible falsy radius behavior and optional speech-bubble/label borders; the final exact head passed the complete Quality/Security/container/platform/parity matrix and merged as `1a25ac0fef217731197169b229bde19aff158c8b`.
- #243 consumed `shape-state-v1` in FrameState, centralized shape-derived bounds on evaluated canonical dimensions, removed generic shape debt, preserved cursor debt, and added mirrored fail-closed/projection coverage. Its first code-only backend run found one stale text-state regression assertion still expecting generic shape debt; the assertion and its TypeScript counterpart were corrected. The final normalized head passed the complete hosted matrix and merged as `111fba5fd7ea73740aee1d92fc1038ee72fda30b`.
- #246 repaired frontend Vitest discovery before cursor work could merge. Activating `frontend/test/` raised the normal unit gate to 51 files / 248 tests and exposed four latent test-only issues: three transition assertions still encoded pre-paint debt, and one zoom fixture used bit-exact floating-point equality. Those assertions were corrected without product-runtime changes; the exact normalized head passed Quality Gate, Security Scan, containers, and all platform/sandbox assurances before merge as `ad7ec51596807293ebb1206ce873088a0ad07da7`.
- #245 defined `cursor-state-v1` in Go and TypeScript, preserved optional visibility semantics, sampled exact rational cursor position/click state, validated malformed authoring, and failed closed on undefined smoothing. It merged as `c428d81f25ce1d85faa6655fb5772430a8fe6b22`.
- #247 cursor FrameState consumption was reviewed with no submitted reviews or inline threads. Its initial hosted matrix was green on the former `c428d81f…` base, then `main` advanced; on 2026-08-21 the exact intended 12-path delta was rebuilt directly on current `main` for fresh validation rather than relying on stale results. The rebuilt exact head passed the complete hosted matrix and squash-merged as `66530a07e3a5585546d978794e24198083bbeaa2`.
- The next branch began directly from merged #247: it derives `source-provenance-v1` from immutable Render Manifest v1 media probes and feeds that state into FrameState geometry and anchors without file probing or canvas-size fallbacks.
- #251 completed that source-provenance/anchor work, passed the exact-head hosted Quality, security, container, browser, renderer-parity, and platform/sandbox matrix without review submissions or inline threads, and squash-merged as `d49808f1ea7fc23f658e95f52fdbe404bf0be92a` on 2026-08-21.
- The next branch began directly from merged #251 and is draft PR #252. It defines fail-closed `font-resource-provenance-v1` from explicitly packaged Render Manifest font faces, while retaining the unimplemented snapshot-package, text-face-selection, and glyph-metric boundaries rather than guessing them.

## Next recommended slice

1. Validate the exact #252 head in hosted CI, address any review feedback, and merge only when it remains current, scoped, and green.
2. Extend the resulting contract through immutable snapshot packaging and explicit text-face selection, then assign glyph-layout/metric ownership without guessing intrinsic text bounds.
3. Define and consume canonical AudioGraph to complete Phase 2 semantic ownership.
4. Continue Phase 0 visual thresholds, unsupported-audio boundary, and second-platform evidence in parallel.
