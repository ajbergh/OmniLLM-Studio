# Video Edit Studio WYSIWYG Rendering Implementation Plan

**Status:** In progress  
**Last updated:** 2026-08-19  
**Scope:** Video Edit Studio preview, timeline evaluation, render jobs, visual composition, audio mix, export, validation, packaging, and parity testing.  
**Primary goal:** The authoritative editor preview and final decoded export must represent the same immutable timeline revision with identical frame identity, active-layer ordering, timing, geometry, styling, effects, transitions, camera state, and audio decisions.

> This is the durable execution tracker for the WYSIWYG rendering program. Every implementation PR in this program must update current status, validation evidence, risks, and the next recommended slice before merge.

## Current handoff

Latest merged WYSIWYG PR: **#225 — Consume canonical fade-family transition paint in FrameState** — `a6f9145f92e2342dfa70144a4058bf10f64625da`.

Current PR: **#227 — Define and consume canonical slide transition paint**.  
Current branch: `feat/video-wysiwyg-phase2-slide-transition-paint`.  
Clean rebuild code head: `46e8f66ceb6bb079fdd51a26cf578a6d73fa0c88`.

PR #227 was rebuilt from the actual post-#225 `main` tree rather than retaining its original stacked ancestry. `compare main...branch` shows one commit ahead, zero behind, and exactly eight slide-specific files. The tracker update is the only additional intended file before merge.

The next two transition slices already exist as stacked draft PRs and must be normalized with the same safe replay method after each parent merges:

- **#228 — Define and consume canonical wipe transition paint**.
- **#229 — Define and consume canonical zoom transition paint**.

No preview or FFmpeg compositor behavior changes are included in #225/#227/#228/#229. These are canonical-state slices preparing Phase 3 shared composition.

## Phase tracker

| Phase | Status | Progress / exit work |
|---|---|---|
| Phase 0 — Reproducible parity baseline | In progress | Deterministic 103-frame visual/audio/delivery evidence exists. Production visual thresholds, unsupported-audio policy, and second-platform evidence remain. |
| Phase 1 — Immutable submission | Complete | Revision/hash binding, immutable snapshots/source bytes, decode preflight, snapshot-only execution/recovery, identity metadata, stale rejection, Strict Parity diagnostics, and frontend concurrency/dirty-state behavior are implemented. |
| Phase 2 — Canonical contract | In progress | Timing, curves, v1 adapter, frame/range/source/order, normalization, frame addressing, property evaluation, FrameState, media geometry, perspective, transition state, and fade/crossfade/dip-to-black paint are merged. #227 adds slide; #228 adds wipe; #229 adds zoom. Effects, text/shape/cursor state, remaining provenance edges, and AudioGraph remain. |
| Phase 3 — Shared preview composition | Not started | Program monitor consumes canonical FrameState/AudioGraph instead of preview-local semantic math. |
| Phase 4 — Shared Chromium render worker | Not started | Deterministic browser renderer consumes the same canonical composition package; FFmpeg remains decode/encode/mux where appropriate. |
| Phase 5 — Visual parity closure | Not started | Close text, shapes, effects, transitions, cursor, camera, color, asset loading, and decoded visual thresholds. |
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

The legacy FFmpeg compositor is implementation evidence, not semantic authority. Preview behavior is the Timeline v1 compatibility target where it is actually implemented. Where preview lacks an authorable feature, the canonical contract defines the intended behavior explicitly rather than treating absence as semantics.

### Media geometry

`media-geometry-v1` is authoritative for asset geometry:

- asset source aspect ratio requires explicit `content_bounds` or a future explicitly versioned source-probe projection;
- source dimensions are never guessed from the output canvas;
- `mask_source_crop` operates in source coordinates before fit;
- `contain`, `cover`, `fill`, and `none` are canonical fit modes;
- `transform.crop` is a separate post-fit output viewport clip;
- FrameState carries evaluated painted bounds and source provenance;
- missing source provenance remains explicit unresolved state.

### Perspective and stacking

Perspective is projection state, not paint order. Track/z-index order remains authoritative for stacking; spatial `z` affects projection.

`perspective-projection-v1`:

- preserves the preview-compatible 1200-canvas-pixel distance with no scene camera;
- derives projection distance from evaluated scene-camera FOV when a camera is active;
- allows a positive per-clip perspective override;
- preserves `model_matrix` as camera-relative model transform and serializes projection separately.

### Transition timing/ownership

`transition-state-v1` makes placement explicit:

- `in` is bounded by the owner start;
- `out` is bounded by the owner end;
- `between` requires an explicit, distinct peer and sufficient real temporal overlap;
- no hidden source handles or inferred adjacency are invented;
- owner/peer incoming/outgoing roles are explicit;
- windows use canonical half-open frame mapping;
- progress is sampled at exact frame presentation time;
- inactive authored transitions do not make unrelated frames non-authoritative.

### Transition paint

`transition-paint-v1` is composition state, never renderer-local shorthand.

Merged semantics:

- `fade`: one-sided owner opacity for `in`/`out`.
- `crossfade`: true isolated-surface pair blend using outgoing `1-progress` and incoming `progress`; these are pair-composition weights, not two ordinary stacked alpha values.
- `dip_to_black`: explicit outgoing/black/incoming contribution weights with full black at `progress=0.5`.

Current #227 slide semantics:

- direction names the **entry edge**;
- translation space is `canvas-fraction`;
- `left` slide-in moves X `-1 → 0`; `right` moves `+1 → 0`; `up` moves Y `-1 → 0`; `down` moves `+1 → 0`;
- `out` exits through the opposite edge;
- `between` moves outgoing toward the opposite edge while incoming enters from the chosen edge;
- slide does not implicitly change opacity;
- FrameState consumes the resulting paint through the same support/evaluation path and remains authoritative when no other unresolved family is active.

Prepared #228 wipe semantics:

- direction names the reveal/entry edge;
- wipe clips the isolated layer surface in normalized `layer-fraction` space;
- `in` reveals from the selected edge;
- `out` shrinks toward the opposite edge;
- `between` preserves outgoing as the underlying isolated surface while revealing the incoming peer over it;
- all four clip insets are explicit, including zero values;
- the legacy sampled FFmpeg crop segments are not semantic authority.

Prepared #229 zoom semantics:

- scale is a multiplier of the isolated layer's already-evaluated authored scale around its existing anchor;
- scale space is `layer-multiplier`;
- continuous scale envelope is `0.82 + 0.18 * ease-out(q)` using the shared canonical easing evaluator instead of sampled fidelity segments;
- `in`: `q=progress`, owner opacity=`progress`;
- `out`: `q=1-progress`, owner opacity=`1-progress`;
- `between`: true pair weights plus outgoing scale at `q=1-progress` and incoming scale at `q=progress`;
- zoom has no direction.

Once #229 is merged, every currently authorable Timeline v2 transition type has canonical paint semantics and FrameState consumption. Phase 3 consumers must apply pair operations to isolated surfaces; they must not reinterpret pair paint as independent stacked layer opacity/transform.

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

Security unblock during the program:

- #201 replaced reachable-vulnerable `github.com/ledongthuc/pdf` with `github.com/tsawler/tabula v1.6.14` — `57cb7764a73203fc1194dbe51992e7ee4779817f`.

CI reliability unblock:

- #219 bounded/retried Linux dependency installation and Playwright bootstrap and added job-level timeouts — `a33b32697019b144c9a7d6c7fec277e1cde101b4`.

### Current PR #227 — canonical slide paint

Implemented on clean branch head `46e8f66ceb6bb079fdd51a26cf578a6d73fa0c88` before this tracker commit:

- Go/TypeScript `transition-paint-v1` adds `owner-translate`, `pair-slide`, and `canvas-fraction` translation space.
- Explicit owner/outgoing/incoming X/Y offsets are serialized; zero axes are explicit rather than inferred.
- The shared support predicate now includes slide.
- `transition-paint-v1.json` includes slide-in from left, slide-out through the opposite edge, and between/up pair movement.
- Invalid slide directions fail closed.
- Existing FrameState paint-consumption tests now prove frame 65's 50% `slide-out` resolves to `(0.5, 0)` and restores authority.
- The older frame-scoping regression test is updated so slide is no longer unresolved in this slice.
- `compare main...branch` before this docs commit contained exactly eight intended slide files; no unrelated branch-history changes remained.

Validation status before this tracker commit:

- fresh hosted exact-head jobs were queued behind repository runner demand;
- no code failure had been reported;
- the branch had not yet been marked ready or merged.

Remaining before #227 merge:

1. Validate the documentation-complete exact head.
2. Remediate any formatting/type/test/security finding.
3. Reconfirm `main...branch` contains only eight slide files plus this tracker.
4. Confirm review-thread state.
5. Mark ready and merge.
6. Rebuild #228 from the resulting current `main` tree using only the wipe delta.

### Remaining Phase 2 work

After the transition stack:

1. **Effect stack** — define deterministic effect ordering, enable windows, parameter defaults, animated effect-property evaluation, failure policy, and FrameState projection.
2. **Text / shape / cursor state** — remove those unresolved families by making all renderer-relevant evaluated state explicit.
3. **Provenance edges** — close any remaining anchor/content-bounds/source-probe cases surfaced by parity diagnostics.
4. **AudioGraph** — define serializable timing/rate/pitch/channel/gain/fade/mute/solo/processing/stem decisions and exact sample-count semantics.
5. Keep all unknown authorable fields fail closed until canonical semantics exist.

### Phase 2 exit gate

Preview and export callers consume identical FrameState/AudioGraph fixtures. No renderer owns separate curve, range, ordering, transform, geometry, projection, transition placement/activity/paint, effect, source-time, or audio semantic math. Go/TypeScript schema/type/fixture drift fails CI.

## Phase 3 — Shared preview composition

Drive the Video Edit Studio program monitor from canonical FrameState/AudioGraph while preserving direct-manipulation UI state separately. Add diagnostics for frame identity, active clip IDs, source time, matrices/bounds/projection, transitions, effects, and audio graph identity.

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
- #225 consumed fade-family paint in FrameState. Its first Quality Gate found a stale #222 test expectation, which was fixed. A pre-merge diff audit then caught an unsafe synthetic ancestry/tree merge that would have reverted unrelated sandbox-worker changes. The branch was rebuilt cleanly from current `main`, verified to contain only six intended files, and merged as `a6f9145f92e2342dfa70144a4058bf10f64625da` after documenting exact validation state.
- #227 was rebuilt from the actual post-#225 `main` tree with only eight slide-specific file deltas. The original stacked history is no longer used for merge.
- Draft #228 defines wipe through normalized isolated-layer clipping; draft #229 defines continuous zoom through canonical scale multipliers/easing. Both require the same safe replay onto `main` after their parent merges.

## Next recommended slice

1. Finish exact-head validation and merge #227.
2. Safely replay/update/validate/merge #228.
3. Safely replay/update/validate/merge #229. This completes canonical paint for every current Timeline v2 transition family.
4. Start effect-stack semantics immediately after the transition stack: ordering, enabled windows, parameter defaults, animated effect properties, unresolved/fail-close policy, and FrameState consumption.
5. Continue Phase 0 visual thresholds, unsupported-audio boundary, and second-platform evidence in parallel.
