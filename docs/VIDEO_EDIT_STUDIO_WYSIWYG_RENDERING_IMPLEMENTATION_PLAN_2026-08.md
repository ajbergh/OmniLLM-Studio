# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-20  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG PR: **#241 — Define canonical text renderer state** — `9b072685b689cdb74e0a5590a26478f6a3ef12b4`.

Current PR: **#242 — Define canonical shape renderer state**.  
Current branch: `feat/video-wysiwyg-phase2-shape-state`.  
Code-only validation head before tracker/final normalization work: `5958857ef48e50d75913ca1e014b3cccd23773f0`.

PR #241 merged only after its documentation-complete exact head `d51c02cd0710120fc895567ad81a2c456f297773` was one intended commit ahead/zero behind current `main`, had no review threads, and passed the complete hosted assurance matrix: Go formatting/vet/unit/integration/race, frontend lint/unit/performance/build, Playwright smoke, deterministic renderer parity, Security Scan including both CodeQL languages and dependency audit, all platform/sandbox assurances, and frontend/backend container builds.

PR #242 starts directly from post-#241 `main` merge `9b072685b689cdb74e0a5590a26478f6a3ef12b4`. It deliberately defines canonical shape semantics only. FrameState consumption/removal of generic shape debt is the immediate follow-on slice so evaluator definition and consumption remain independently reviewable.

No preview CSS compositor or FFmpeg composition behavior changes are included in #242. Legacy FFmpeg shape approximations remain implementation evidence, not semantic authority.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Timing, curves, v1 adapter, frame/range/source/order, normalization, frame addressing, property evaluation, FrameState, media geometry, perspective, all current transition state/paint families, effect stack state, and canonical text state are merged. #242 defines canonical shape state. Shape FrameState consumption, cursor state, remaining provenance edges, and AudioGraph remain. |
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

- source aspect ratio requires explicit `content_bounds` or a future versioned source-probe projection;
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

`shape-state-v1` is the renderer-independent static annotation contract introduced by #242.

Current #242 semantics:

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

Compatibility boundary for #242:

- current preview supports all authored shape kinds, but several legacy FFmpeg paths are approximations or preview-only;
- #242 defines authored/preview semantic intent so shared Phase 3/4 composition can close export parity without preserving legacy approximations;
- FrameState shape projection/removal of generic `shape` debt is intentionally deferred to the immediate follow-on consumption PR.

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

Security unblock during the program:

- #201 replaced reachable-vulnerable `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.

CI reliability unblocks:

- #219 bounded/retried Linux dependency installation and Playwright bootstrap and added job-level timeouts — `a33b32697019b144c9a7d6c7fec277e1cde101b4`.
- #226 quiesced competing apt activity before Playwright retry setup — `ffbf797108c291c927fc7b67f6154b29c1351496`.
- #239 aligned the durable sandbox-worker test SQLite helper with production WAL/busy-timeout behavior, eliminating a false `SQLITE_BUSY` blocker encountered while #237 was being replayed over concurrent sandbox work.

### Current PR #242 — canonical shape renderer state definition

Implemented on code-only validation head `5958857ef48e50d75913ca1e014b3cccd23773f0` before tracker/final normalization work:

- shared Go/TypeScript `shape-state-v1` evaluator;
- all 14 authorable shape kinds with preview-grounded defaults;
- explicit dimensions, fill/stroke, stroke width, blur/pixel block radius, and corner-radius state;
- fail-closed unsupported-kind/non-finite/invalid-dimension policy;
- shared `shape-state-v1.json` Go↔TypeScript fixture;
- mirrored focused Go/TypeScript tests;
- no FrameState consumption in this definition slice.

Validation history before final normalized head:

- code-only Go formatting/vet/unit/integration/race: PASS;
- code-only frontend lint/unit/performance/build: PASS;
- code-only core platform/sandbox assurance: PASS for completed jobs;
- manual review of `ShapePreview.tsx` caught the falsy zero-radius fallback for rounded rectangle, speech bubble, and label; both evaluators and the shared fixture were corrected;
- follow-up review against `ShapePreview.tsx`, `VideoInspector.tsx`, and `annotationRegistry.ts` caught that speech bubble/label default borders are absent unless `shape.stroke` is authored; both evaluators and shared fixtures were corrected so empty stroke means no default callout border;
- these compatibility corrections and tracker update change the head, so every required hosted gate must pass again on the final documentation-complete normalized head before merge.

Remaining before #242 merge:

1. Rebuild/squash the corrected documentation-complete intended tree as one commit directly on current `main`.
2. Reconfirm `main...branch` contains only the five shape code/test/fixture paths plus this tracker.
3. Refresh #242 body with both compatibility corrections and exact final head; keep ready for review.
4. Validate formatting, vet, unit/integration, race, frontend lint/unit/performance/build, Playwright smoke, deterministic renderer parity, Security Scan, container builds, and all platform/sandbox assurances on the exact final head.
5. Inspect and resolve any actionable review thread.
6. Merge only when the exact head is current, one intended commit ahead, zero behind, mergeable, and clean.
7. Immediately create the separate shape-state FrameState consumption slice from resulting current `main`.

### Remaining Phase 2 work

After #242:

1. **Shape state consumption** — project `shape-state-v1` into FrameState, add permanent Go↔TypeScript parity coverage, and remove generic `shape` debt.
2. **Cursor state** — canonicalize sampled cursor position, visibility, scale, highlight/click-ring semantics, smoothing policy, and any asset/provenance needs.
3. **Provenance edges** — close remaining anchor/content-bounds/source-probe/font-resource cases surfaced by parity diagnostics.
4. **AudioGraph** — define serializable timing/rate/pitch/channel/gain/fade/mute/solo/processing/stem decisions and exact sample-count semantics.
5. Keep all unknown authorable fields fail closed until canonical semantics exist.

### Phase 2 exit gate

Preview and export callers consume identical FrameState/AudioGraph fixtures. No renderer owns separate curve, range, ordering, transform, geometry, projection, transition placement/activity/paint, effect, text, shape, source-time, or audio semantic math. Go/TypeScript schema/type/fixture drift fails CI.

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
| Schema/type drift | Go reflection and TypeScript compile/Vitest projection checks fail CI. |
| Browser/Go evaluator drift | Shared fixtures plus permanent FrameState diagnostics. |
| Legacy FFmpeg approximations become de facto contract | Canonical semantics are explicit; legacy renderer is evidence only. |
| v1 adapter guesses ambiguous behavior | Ambiguous state fails closed. |
| Millisecond rounding creates frame/source drift | Canonical rational frame/range/source helpers. |
| Source aspect ratio is guessed from canvas | Explicit source `content_bounds`; missing provenance stays unresolved. |
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
| Falsy authored shape defaults drift from preview | Zero stroke/blur/radius values follow current preview truthy-fallback rules and are covered by shared fixtures. |
| Optional callout borders are invented | Speech bubble/label serialize empty default stroke; only an authored non-empty stroke enables their border. |
| Stacked branch appears current but carries stale tree | Rebuild from actual current `main`; compare every path before merge. |
| FrameState claims authority too early | Explicit unresolved sets until canonical family semantics exist. |
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

- #242 defines renderer-neutral `shape-state-v1` for all currently authored annotation kinds. The code-only evaluator passed formatting, vet, unit/integration, race, and the full frontend pipeline. Manual review then caught two preview-compatibility details before merge: falsy zero corner radii must preserve the kind fallback for rounded rectangle/speech bubble/label, and speech bubble/label have no default border unless a stroke is authored. Both runtimes and shared fixtures were corrected before final normalization.

## Next recommended slice

1. Normalize, validate, and merge #242 from its corrected documentation-complete exact head.
2. Immediately consume `shape-state-v1` in FrameState and remove generic shape debt in a separate small PR.
3. Follow with canonical cursor renderer state, then remaining provenance/font-resource edges and AudioGraph.
4. Continue Phase 0 visual thresholds, unsupported-audio boundary, and second-platform evidence in parallel.