# Video Motion Design Roadmap

> **Status:** IMPLEMENTED — Phases 0–6 complete; Phase 7 intentionally not activated because it is an optional consideration with no selected hosted workflow.
>
> **Purpose:** Durable implementation plan for evolving OmniLLM-Studio Video Edit Studio from a capable nonlinear editor into a combined NLE + motion-design + agentic video-composition workspace.
>
> **Baseline:** Re-verified against `main` at `b4520f83ea5d8eb5a3c13692c57e59f4e04462b8` on 2026-08-17. The original draft cited `11dfab99e73fe414e45cc44b0f33d4c80789295a`, which is `feat(sandbox): enforce Windows process-count quotas (#171)` — a sandbox commit 15 revisions behind current `main`. `git diff 11dfab99..HEAD -- backend/internal/video frontend/src/components/video frontend/src/stores/videoStudio.ts frontend/src/types/video.ts` is empty, so no video code changed in that span and the technical baseline below holds. Prefer re-verifying the claims in §2 over trusting the pinned hash.
>
> **Product reference:** Raylight (`https://www.raylight.app`) was reviewed as a motion-design reference. The goal is not product cloning. The useful concepts are its Design/Animate separation, semantic animation blocks, scene/shot navigation, depth/camera model, cinematic effects, reusable motion templates, and agent-addressable editing model.

## Implementation status — 2026-08-17

| Phase | Status | Shipped evidence |
| --- | --- | --- |
| 0 — Renderer/performance validation | **Complete** | 2,000-clip/16,000-keyframe fixture, CI budgets and artifact, real FFmpeg golden coverage, effect-amount validation/authoring, and persisted track-solo export semantics. |
| 1 — Motion Design mode | **Complete** | Reachable fifth mode with `Design / Animate / Effects / AI / Export`; mode changes are non-mutating and covered by store and Playwright smoke tests. |
| 2 — Animation blocks | **Complete** | One 21-block In/During/Out registry and compiler; replace/stack semantics, stored provenance, editable generated keyframes, and atomic undo/autosave mutations. |
| 3 — Curves and scenes | **Complete** | Additive curve schema, deterministic Bezier/spring sampling in preview and export, scene strip/operations/snapping, per-scene cameras/effects, validation, and undo tests. |
| 4 — 2.5D/camera | **Complete** | Spatial transforms and keyframes, camera lane/projection/parallax, unchanged additive v1 compatibility, and explicit partial-fidelity capability notes for 3D tilt. |
| 5 — Effects/templates | **Complete** | Nine cinematic scene effects through preview and FFmpeg, runtime fidelity badges, 12 motion templates, persistent replaceable slots, and missing-slot analysis. |
| 6 — Agentic Motion Director | **Complete** | Bounded inspect/mutate/frame/preview/status/cancel tools; timeline/revision binding; atomic rejection; real renderer diagnostics; ownership, cancellation, and three-iteration limit. |
| 7 — Optional review/share | **Not activated** | This phase only asks the product to consider deployment-specific sharing. No deployment model was selected, so local-first export remains the deliberate behavior. |

Validation recorded for this implementation:

- `go test ./internal/video` passes, including real FFmpeg golden renders for motion curves and all nine scene effects.
- The new governed video-tool contract tests pass. The full backend suite passes every package except one pre-existing `internal/gitrepo` Windows symlink test, which requires the OS `Create symbolic links` privilege; no motion test fails.
- Frontend unit tests: 19 files / 80 tests pass; TypeScript/Vite production build passes.
- Performance fixture: 2,000 clips, 480 active-time queries, 2,472,801-byte document, 1.81 ms indexing, 19.07 ms total querying, 0.072 ms p95 frame computation, 22.85 MB heap delta, and 1,758-byte representative animation-block patch.
- Motion Design Playwright smoke test passes against the built backend/frontend servers.

The baseline and phase-scope prose below is retained as an implementation record. Where it describes the pre-implementation state, the status table and phase-status callouts above are authoritative for the current branch.

## 1. Goal

Extend the existing Video Edit Studio without replacing its current architecture.

The target product should combine:

- OmniLLM-Studio's existing nonlinear editing, trimming, ripple/gap editing, captions, audio workflows, recording, annotations, effects, transitions, media management, AI generation, and FFmpeg export.
- A simpler motion-design mental model for users who want animated compositions rather than traditional clip editing.
- Richer animation semantics above raw keyframes.
- Optional 2.5D spatial composition and camera motion.
- A governed video-editing tool surface that allows Omni agents to inspect a project, propose edits, apply approved edits, render frames, visually inspect results, and iterate.

The end state is intentionally broader than a Raylight-style motion editor:

```text
Traditional NLE
     +
Motion Design
     +
Generative Video
     +
Agentic Editing / Visual Evaluation
```

## 2. Current confirmed baseline

The current repository already has a substantial Video Edit Studio and should be extended rather than rewritten.

### 2.1 Confirmed capabilities

- Neutral timeline JSON using generic ordered layers (`TrackTypeLayer`; higher array indices stack on top).
- Clip move, trim, split, speed, ripple, overwrite, grouping, snapping, markers, copy/paste, undo/redo, and layer operations.
- Preview compositing with transforms, opacity, crop, effects, keyframes, styled text, direct manipulation, smart guides, inline text editing, and crop mode — all in `VideoPreviewCanvas.tsx`.
- Keyframes for `x`, `y`, `scale`, `rotation`, `opacity`, and `volume` with `linear`, `ease-in`, `ease-out`, `ease-in-out`, and `step` interpolation.
- Effects and transitions with preview/export capability reporting.
- Captions, transcription, screen/camera/voice recording, audio ducking, fades, waveform display, render queue, export validation, codec options, and durable FFmpeg jobs.
- One-shot AI storyboard / timeline-plan / edit-plan / apply-edit-plan / social-variant endpoints (`router.go:567-571`).
- A conservative renderer capability matrix in `backend/internal/video/renderer_capabilities.go`.
- Four editor modes in `frontend/src/components/video/editorModes.ts`: `full`, `simple_trim`, `captions_only`, `social_clip`.
- One existing agent-facing video tool: `video_generate` (`backend/internal/tools/video_job_tool.go:25`), an asynchronous `jobs.Manager` tool.

### 2.2 The editor is a three-layer wrapper chain, not one component

This materially affects every frontend phase below. `App.tsx:11` mounts the outermost shell:

```text
App.tsx
  -> VideoEditStudioUltimate.tsx   (268 LOC)  byte-budgeted PatchHistory undo
       -> VideoEditStudioEnhanced.tsx (499 LOC)  analyzeTimeline health/issue surface
            -> VideoEditStudio.tsx    (1270 LOC) core editor, inspector, timeline
```

Any new editor mode, inspector tab, or store field must be threaded through all three. Treating `VideoEditStudio.tsx` as "the" studio file — as the original draft did — will produce a mode that cannot be reached from the running app.

### 2.3 The `pro/` layer is live and load-bearing

`frontend/src/components/video/pro/` was omitted from the original draft entirely. It is wired into production paths, not dead code, and several roadmap requirements silently depend on it:

| File | Live consumer | Roadmap relevance |
| --- | --- | --- |
| `pro/timelineIndex.ts` | `TimelineTrack.tsx:281` (`visibleClips`), `VideoPreviewCanvas.tsx:200-208` (`buildTimelineIntervalIndex`, `queryActiveClips`, `applyDecoderBudget`) | This *is* the timeline virtualization and decoder budget that §8 requires preserving. |
| `pro/timelineAnalysis.ts` | `VideoEditStudioEnhanced.tsx:161` — `analyzeTimeline(timeline, assets, rendererCapabilities, undoDepth)` | This *is* where export-fidelity warnings reach the user. §7's "no UI feature should hide export limitations" is unenforceable without updating this file per feature. |
| `pro/patchHistory.ts` | `VideoEditStudioUltimate.tsx:41` — `new PatchHistory(8–256 MB, default 32 MB)` | Undo is a **byte-budgeted JSON patch log**, not an unbounded snapshot stack. Animation drags that emit large patches evict older history rather than growing memory. |
| `pro/timelineCommandEngine.ts` | `pro/audioTools.ts`, `pro/mediaTools.ts` (`commitTimelineCommand:92`) | An existing labelled-mutation abstraction with `TimelineBranchRecord` branches and a 50-entry history limit. New motion mutations should evaluate this before inventing another commit path. |

The store's own convention (`stores/videoStudio.ts:4`) is `clone -> mutate -> one undo snapshot via withTimelineHistory -> autosave`. `PatchHistory` is a second, coexisting history mechanism at the Ultimate layer. **Do not add a third.** Decide per phase which of the two owns a given mutation.

### 2.4 The persisted animation model

```text
TimelineClip.Transform  (timeline.go:201)
  map[string]any        <- NOT a typed Go struct
  defaulted only        <- keys are never validated (timeline.go:634, :840, :947)

TimelineKeyframe        (timeline.go:288-294)
  ID       string
  Property string       <- strict allowlist, hard reject
  TimeMS   int64
  Value    float64      <- one scalar per property per time
  Easing   string       <- bare string, unknown values silently coerced
```

Two asymmetries drive most of the compatibility work in this roadmap:

1. **Transform keys are unvalidated.** Adding `z`, `rotation_x`, or `perspective` to the transform map requires **no Go struct change and no validator change**. The frontend `VideoTimelineTransform` interface (`types/video.ts:222-229`) is typed, so the mirror obligation is frontend-only. The tradeoff is that there is no backend safety net against a malformed transform today.
2. **Keyframe properties are strictly allowlisted and hard-fail.** `knownKeyframeProperties` (`timeline.go:137-144`) is checked at `timeline.go:735-737` and returns an error for the **entire document** on an unknown property. Unknown *easing* values instead degrade silently to `linear` (`timeline.go:741-744`).

The current transform and animation model is fundamentally 2D. The roadmap below adds motion-design semantics while preserving existing timelines and current NLE behavior.

### 2.5 Known inaccuracy in the current capability matrix

`renderer_capabilities.go:67` advertises that "effect-amount keyframes export". That claim is **unreachable today**:

- `sampleEffects` (`renderer_fidelity.go:338-351`) looks for keyframe properties named `effect.<id>.amount`, `effect:<id>:amount`, and `effect.<type>.amount`.
- `ValidateTimelineDocument` rejects every one of those names via `knownKeyframeProperties`, and `renderer.go:102` validates before rendering.
- No frontend code path creates such a keyframe, and no test covers the branch.

So the sampling code is dead and the capability note overstates support. Closing this is Phase 0 work: either allowlist the synthetic property names and wire an authoring path, or remove the claim from the notes. This matters because §7 makes the capability matrix the contract, and Phase 5A's acceptance criteria assume the matrix is already honest.

## 3. Architectural principles

### 3.1 Preserve the neutral timeline

Do not create a second incompatible motion-project format. New motion-design concepts must extend the existing `TimelineDocument` schema through normal versioned upgrade logic.

### 3.2 Semantic animation compiles down to deterministic timeline behavior

Animation blocks should initially compile to existing or extended keyframes rather than becoming an independent rendering truth model.

```text
Animation block
      ->
registry / compiler
      ->
keyframes and/or scene properties
      ->
existing preview + renderer paths
```

This keeps manual keyframes editable and avoids two competing animation systems. The pattern is already proven in-repo by `effects/motionPresets.ts` (see Phase 2).

### 3.3 Preview support does not imply export support

Every new transform, effect, camera, and animation capability must remain conservative in `renderer_capabilities.go` **and** must surface through `pro/timelineAnalysis.ts` until the FFmpeg path is implemented and covered by renderer/golden-media tests. A conservative matrix that the UI never reads is not a safeguard.

### 3.4 Backward compatibility is mandatory

Existing timelines must continue to load without destructive rewrites. New fields must be optional with stable defaults. Timeline upgrade functions must normalize older documents explicitly.

### 3.5 Forward compatibility is a real constraint, not an afterthought

This principle was missing from the original draft and it changes phase sequencing.

- `CurrentTimelineVersion = 1` (`timeline.go:43`). Documents from a future version **fail with an actionable error** rather than degrading (`docs/VIDEO_TIMELINE_SCHEMA.md:21`).
- Any new keyframe property rejects the whole document on an older build.

Consequences:

- Every new animatable property must land **backend-first**, in a release that ships before the frontend that authors it. A frontend-first rollout produces timelines the user's own prior build cannot open.
- Desktop users can downgrade. A version bump for scenes (Phase 3B) makes every post-bump project unopenable on older desktop builds. Prefer additive optional fields at version 1 for as long as the semantics genuinely permit it, and bump only when a real breaking reinterpretation lands.
- Prefer silent-degrade semantics over hard-reject semantics for *new* optional fields, but never silently degrade something the user can see in preview without also reporting it through the fidelity surface.

### 3.6 UI simplification is additive

Motion Design mode should hide irrelevant NLE controls without changing persisted timeline semantics, matching the current editor-mode strategy in `editorModes.ts`.

### 3.7 Agent mutations use existing governance

Video-editing tools exposed to agents must use the existing Omni tool registry, permissions, approval policy, ownership checks, bounded inputs/outputs, and audit paths. Do not create a privileged side channel specifically for Video Studio AI. `video_generate` is the existing precedent for a long-running, job-backed video tool.

## 4. Execution order

1. Finish the current Video Studio performance/export validation prerequisite, including the effect-amount capability discrepancy in §2.5.
2. Add Motion Design UI mode and Design/Animate inspector organization.
3. Add semantic animation blocks that compile to editable keyframes, absorbing the existing `MOTION_PRESETS`.
4. Expand curve semantics and introduce scene/shot ranges.
5. Add 2.5D transforms and camera semantics, preview first, backend allowlists first.
6. Author scene-level cinematic effects (preview + registry) — omitted from the original ordering.
7. Add renderer parity for the new spatial/cinematic features.
8. Add remixable motion templates.
9. Expand the assistant into a governed agentic motion director with render-and-inspect loops.
10. Consider optional collaborative review/share surfaces only after the local-first editing path is mature.

---

# Phase 0 - Existing renderer and performance validation

> **Implementation status:** COMPLETE — performance budgets, CI evidence, golden renderer coverage, effect-amount keyframes, and persisted solo export are implemented and tested.

## Objective

Complete the reliability work already identified in `docs/MASTER_PLAN.md:66-67` before significantly increasing animation complexity.

## Scope

- Add representative large-project browser performance fixtures.
- Capture frame-time / React commit / memory evidence in CI. **At the roadmap baseline, `.github/workflows/ci.yml` had no performance job at all** — there was no budget to preserve, only one to create. Phase 0 added the current evidence job and thresholds.
- Preserve conservative renderer-capability reporting.
- Resolve the effect-amount keyframe discrepancy in §2.5 — the matrix must not advertise an unreachable path.
- Fix the stale comment in `effects/motionPresets.ts:13-17` claiming "Scale keyframes are preview-only at export today"; `renderer_capabilities.go:67` reports sampled scale keyframes as exporting. One of the two is wrong and users read the badge derived from the matrix.
- Select and close at least one existing renderer-fidelity gap with golden-media coverage before marking new motion features export-capable.

Current known fidelity debt, confirmed verbatim against `renderer_capabilities.go` and `MASTER_PLAN.md:66`:

- true two-clip crossfades (transitions are alpha-fade approximations),
- rounded/geometric annotation fidelity (rounded corners preview-only; ellipse/arrow/speech-bubble normalize to primitives),
- drop shadow / background blur (unsupported),
- continuous-curve fidelity (sampled segments only),
- ~~track-solo export semantics (preview-only monitoring)~~ — resolved in Phase 0; `solo` is persisted and constrains the export audio mix,
- click-audio synthesis (not synthesized).

## Primary existing files

- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_fidelity.go`
- `backend/internal/video/renderer_capabilities.go`
- `frontend/src/components/video/pro/timelineIndex.ts` (virtualization/decoder budget under measurement)
- `frontend/src/components/video/pro/timelineAnalysis.ts` (fidelity surface)
- existing `tests/video-editor-*.smoke.spec.ts` coverage (`video-editor`, `video-editor-advanced`, `video-editor-source-media`, `video-editor-recording-lab`)
- `backend/internal/video/renderer_golden_test.go`, `renderer_fidelity_test.go`
- `.github/workflows/ci.yml`

## Acceptance criteria

- A reproducible large-timeline fixture exists.
- CI produces stable performance evidence for regression review, with a documented budget where a threshold is defensible.
- No capability is upgraded in `renderer_capabilities.go` without renderer tests.
- No capability *note* claims a path that validation rejects.
- At least one current fidelity gap is closed with golden-media or deterministic output assertions.

---

# Phase 1 - Motion Design editor mode

> **Implementation status:** COMPLETE — the fifth mode is threaded through the mounted shell with a non-mutating five-tab workflow and smoke coverage.

## Objective

Create a focused motion-design workspace using the current Video Edit Studio shell and timeline semantics.

## User experience

Add a fifth editor mode:

```text
Full editor      (full)
Simple trim      (simple_trim)
Captions only    (captions_only)
Social clip      (social_clip)
Motion design    (motion_design)   <- new
```

`EditorModeKey` is a closed union and `EditorModeFeatures` is a fixed struct of eight booleans. Motion Design needs feature flags that do not exist yet (a Design/Animate split, an animation-block picker), so this phase extends `EditorModeFeatures` — which means every one of the four existing mode definitions must be updated in the same change or TypeScript will fail the build. That is a feature, not a burden: it forces an explicit decision per mode.

In Motion Design mode, emphasize a simpler right rail:

```text
Design | Animate | Effects | AI | Export
```

### Design tab

Expose static composition controls:

- X/Y position
- scale
- rotation
- crop
- opacity
- alignment
- layer order
- text/shape appearance
- fit/fill/center/reset

Future spatial fields such as Z and tilt should appear here when Phase 4 lands.

### Animate tab

Expose motion behavior:

- In
- During
- Out
- motion presets
- property keyframes
- easing
- duration/delay
- animation stack

## Primary existing files

- `frontend/src/components/video/editorModes.ts`
- `frontend/src/components/video/VideoEditStudio.tsx`
- `frontend/src/components/video/VideoEditStudioEnhanced.tsx` (mode-aware health/issue surface)
- `frontend/src/components/video/VideoEditStudioUltimate.tsx` (outermost mounted shell)
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/components/video/timeline/VideoTimeline.tsx`
- `frontend/src/components/video/timeline/KeyframeLane.tsx`
- `frontend/src/stores/videoStudio.ts`

The five current `editorMode` consumers are `editorModes.ts`, `VideoTimeline.tsx`, `VideoEditStudio.tsx`, `VideoInspector.tsx`, and `videoStudio.ts`.

## Data model

No timeline schema change is required for the first Motion Design UI release. Mode selection is UI state, consistent with the existing four modes.

## Tests

- Unit coverage for mode feature gates, including the new flags on all five modes.
- Playwright smoke coverage proving the mode can be entered and exited without changing persisted timeline content.
- Regression coverage proving a timeline edited in Motion Design mode opens identically in Full Editor mode.

## Acceptance criteria

- Motion Design mode is reachable from the mounted `VideoEditStudioUltimate` shell, not only from the inner component.
- Motion Design mode is available without duplicating editor state.
- Static and animated properties are clearly separated in the inspector.
- No existing editor mode loses functionality.
- Existing timelines require no migration merely to enter Motion Design mode.

---

# Phase 2 - Semantic animation blocks

> **Implementation status:** COMPLETE — the legacy presets now derive from one provenance-aware 21-block registry with replace/stack and atomic undo behavior.

## Objective

Add higher-level motion behaviors so users do not need to author every animation with individual keyframes.

## This extends an existing system — it does not introduce one

`frontend/src/components/video/effects/motionPresets.ts` already implements exactly the compile-to-keyframes pattern this phase proposes, with a registry shape of `{ key, label, description, build(clip, canvas) => MotionKeyframeSpec[] }` and a `motionPreset(key)` lookup.

It already ships six presets: `zoom_in`, `zoom_out`, `pan_left`, `pan_right`, `ken_burns`, and `restore`. Three of those overlap the "During" family proposed below (Slow Zoom, Pan, Ken Burns). **Phase 2 must absorb or migrate `MOTION_PRESETS`, not add a parallel registry beside it.** Two motion registries is the same failure mode as two animation truth models.

Two existing behaviors must change deliberately rather than being assumed away:

1. `MOTION_PRESETS` **replaces all existing `x`/`y`/`scale` keyframes** when applied (documented at `motionPresets.ts:9`). That directly contradicts this phase's acceptance criterion about never destroying unrelated manual keyframes. Changing it is a user-visible behavior change to specify, and `restore` (whose entire purpose is clearing motion keyframes) needs a defined role in the new model.
2. Blocks must record enough provenance to be re-editable and re-appliable. A generated keyframe is indistinguishable from a manual one in the current schema — `TimelineKeyframe` has no origin field. Either store block definitions in `TimelineClip` metadata, or accept that blocks are fire-and-forget generators and drop the "modify a block" requirement. Choose explicitly; do not leave it implied.

## Initial block families

### In

- Fade In
- Slide In
- Scale In
- Pop
- Blur Reveal
- Rise
- Drop

### During

- Float
- Drift
- Pulse
- Breathe
- Slow Zoom (absorbs `zoom_in`)
- Pan (absorbs `pan_left` / `pan_right`)
- Gentle Rotate
- Ken Burns (absorbs `ken_burns`)

### Out

- Fade Out
- Slide Out
- Scale Out
- Drop Away
- Blur Out

**Blur Reveal and Blur Out cannot be delivered in this phase as animated blur.** Blur is an effect parameter, not a keyframable property, and effect-amount keyframes do not work today (§2.5). Either sequence them behind the Phase 0 fix, or ship them as opacity/scale approximations with honest labelling.

## Proposed frontend files

The following are **new proposed files**, not current repository files:

- `frontend/src/components/video/motion/animationBlockRegistry.ts`
- `frontend/src/components/video/motion/AnimationBlockPicker.tsx`
- `frontend/src/components/video/motion/AnimationBlockEditor.tsx`
- `frontend/src/components/video/motion/animationBlockCompiler.ts`

Extend existing:

- `frontend/src/components/video/effects/motionPresets.ts` (source of the existing registry)
- `frontend/src/components/video/effects/keyframeUtils.ts`
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/stores/videoStudio.ts`

## Compilation strategy

For the first implementation, blocks compile to normal timeline keyframes.

Example:

```text
Pop In
  duration: 650ms
  overshoot: 1.06
  start scale: 0.80

compiles to approximately:
  scale 0.80 @ t0
  scale 1.06 @ t0 + 420ms
  scale 1.00 @ t0 + 650ms
```

Users must still be able to reveal and manually edit generated keyframes.

**Hard constraint that keeps this phase backend-free:** every block must compile only to the six allowlisted properties (`x`, `y`, `scale`, `rotation`, `opacity`, `volume`) and the five allowlisted easings. Any block that needs a seventh property becomes a backend-first change under §3.5 and belongs in Phase 3 or 4, not here.

## Undo/save behavior

Follow the existing store convention (`stores/videoStudio.ts:4`):

```text
clone -> mutate -> one undo snapshot via withTimelineHistory -> autosave
```

Applying or modifying one animation block should produce one undoable user action. Note that the outer `PatchHistory` budget (`VideoEditStudioUltimate.tsx:41`, 8–256 MB, default 32 MB) also records the change; a block that rewrites many keyframes produces a proportionally large patch and evicts older history sooner. Measure patch size for the worst-case block against a large fixture.

## Acceptance criteria

- Blocks generate deterministic keyframes.
- `MOTION_PRESETS` is migrated or absorbed; exactly one motion registry exists after this phase.
- Generated keyframes remain editable in the existing keyframe lane.
- Reapplying a block has explicit replace/stack behavior; it never silently destroys unrelated manual keyframes.
- Undo removes the entire block mutation in one step.
- Preview and export behavior remain identical to manually-authored equivalent keyframes.
- No block emits a keyframe property or easing value the backend validator would reject.

---

# Phase 3 - Motion curves and scene/shot ranges

> **Implementation status:** COMPLETE — additive curves and validated scene ranges/cameras/effects are authored, persisted, previewed, exported, snapped, reordered, duplicated, and tested.

## 3A. Richer easing and springs

### Objective

Extend the current easing vocabulary with a forward-compatible curve model while preserving old values.

### The original proposal was schema-breaking — use an additive sibling field

The draft proposed replacing easing with a discriminated-union object:

```ts
type MotionCurve =
  | { type: 'linear' } | { type: 'ease-in' } | { type: 'ease-out' }
  | { type: 'ease-in-out' } | { type: 'step' }
  | { type: 'bezier'; x1: number; y1: number; x2: number; y2: number }
  | { type: 'spring'; stiffness: number; damping: number; mass: number };
```

`TimelineKeyframe.Easing` is a bare `string` today (`timeline.go:293`). Changing that field to an object is a breaking JSON shape change on a version-1 document, which contradicts §3.4 and §3.5 and this roadmap's own "additive optional fields" mitigation in §10. It would also require a custom `UnmarshalJSON` accepting both shapes forever.

Recommended instead — an additive optional sibling, no version bump:

```go
type TimelineKeyframe struct {
    ID       string        `json:"id"`
    Property string        `json:"property"`
    TimeMS   int64         `json:"time_ms"`
    Value    float64       `json:"value"`
    Easing   string        `json:"easing,omitempty"` // retained, authoritative when Curve is nil
    Curve    *MotionCurve  `json:"curve,omitempty"`  // new, wins when present
}
```

Resolution rule: `Curve` when present, else `Easing`, else `linear`. Old documents are untouched. Documents with `Curve` opened by an older build lose the curve — but note the failure mode carefully: **the old build coerces silently.** `timeline.go:741-744` maps unknown easing to `linear` without error, so a spring degrades to linear with no signal. Decide whether that silent degrade is acceptable, or whether `Curve` should also write a nearest-equivalent `Easing` value as a graceful fallback. Writing the fallback is strongly preferred.

Do **not** attempt to express new curves as new `Easing` strings. The allowlist at `timeline.go:146-152` silently rewrites them to `linear`, producing an unreported preview/export divergence.

### Primary files

- `frontend/src/types/video.ts`
- `frontend/src/components/video/effects/keyframeUtils.ts`
- `frontend/src/components/video/timeline/KeyframeLane.tsx`
- `backend/internal/video/timeline.go` (`TimelineKeyframe`, `knownKeyframeEasings`, validation)
- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_fidelity.go` (`evaluateTimelineKeyframes` is the shared sampling seam)
- `backend/internal/video/renderer_capabilities.go`
- `docs/VIDEO_TIMELINE_SCHEMA.md`

### Renderer strategy

Bezier and spring motion should be deterministically sampled to renderer segments first, through the existing `evaluateTimelineKeyframes` / `sampleEffects` path. Continuous/native curve rendering can be considered later if FFmpeg support and parity are reliable.

Springs need one decision the draft omitted: `TimelineKeyframe.Value` is a single scalar with no velocity term, so a spring between three keyframes has no defined incoming velocity at the middle point. Specify whether springs are segment-local (velocity resets at every keyframe, simplest and deterministic) or continuous across a property track (requires state and is order-dependent). Segment-local is recommended.

### Acceptance criteria

- Existing string easing values upgrade losslessly, and no version bump is required.
- Frontend preview and backend sampling share tested curve semantics.
- Spring motion is deterministic for the same parameters and FPS/time window, with documented velocity semantics at keyframe boundaries.
- A document containing `curve` also carries a nearest-equivalent `easing` so older builds degrade predictably.
- Golden-media coverage exists before spring/Bezier export is advertised as supported.

## 3B. Scene / shot ranges

### Objective

Add a macro timeline abstraction without physically nesting all clips into a new storage hierarchy.

### Proposed schema

```ts
interface VideoTimelineScene {
  id: string;
  name: string;
  start_ms: number;
  duration_ms: number;
  camera?: VideoSceneCamera;
  effects?: VideoSceneEffects;
  metadata?: Record<string, unknown>;
}
```

`TimelineDocument` (`timeline.go:159-166`) has no `Scenes` field, so unlike transform additions this **is** a real Go struct change plus new validation. The exact names/types above are proposed and must mirror the eventual Go structs.

### Version decision

Adding `scenes` as an optional field with an implicit-full-duration default does **not** require a version bump, and per §3.5 it should not take one. `CurrentTimelineVersion` stays at `1` unless scene semantics reinterpret existing fields. Reserve the bump for a genuine breaking change; a bump makes every subsequent project unopenable on older desktop builds.

### Behavior

- A scene represents a time range over the current flat timeline.
- Existing timelines present as one implicit full-duration scene when scene-aware UI is required; nothing is persisted until the user edits scenes.
- Reordering a scene moves all clips wholly contained by that scene range.
- Cross-boundary clips must have explicit behavior; do not silently split or reassign them. `SliceTimelineRange` (`timeline.go:323`) is the existing pure range transform and should inform the containment rules rather than a new parallel implementation.
- Scene duplicate/delete operations must be deterministic and undoable.
- Scenes must not overlap, and validation must reject overlaps explicitly rather than normalizing them.

### UI

Provide a macro strip above or within the timeline:

```text
[Scene 1] [Scene 2] [Scene 3] [Scene 4]
```

Selecting a scene focuses the timeline viewport and scene-level controls without hiding the underlying clips.

### Primary files

- `frontend/src/types/video.ts`
- `frontend/src/stores/videoStudio.ts`
- `frontend/src/components/video/timeline/VideoTimeline.tsx`
- proposed `frontend/src/components/video/timeline/SceneStrip.tsx`
- `frontend/src/components/video/pro/timelineIndex.ts` (scene-aware windowing)
- `backend/internal/video/timeline.go`
- `docs/VIDEO_TIMELINE_SCHEMA.md`

### Acceptance criteria

- Existing timelines load without semantic change and without a version bump.
- Scene operations produce deterministic timeline transforms.
- Overlapping or negative-duration scenes are rejected by `ValidateTimelineDocument`.
- Scene boundaries are visible and snap-capable.
- Scene operations are covered by frontend store tests and backend timeline validation tests.

---

# Phase 4 - 2.5D transforms and camera model

> **Implementation status:** COMPLETE — spatial transforms/keyframes and depth-based camera parallax are implemented; true X/Y tilt remains explicitly disclosed as an approximate/partial export feature.

## Objective

Enable dimensional compositions and parallax while retaining the current layer timeline.

## 4A. Spatial transforms

### Proposed transform expansion

```text
x
y
z
rotation_x
rotation_y
rotation_z
scale_x
scale_y
anchor_x
anchor_y
perspective
opacity
crop
```

The existing `scale` and `rotation` fields require a compatibility strategy. Prefer additive fields and upgrade/default logic rather than rewriting all old documents immediately.

### The static half is cheap; the animated half is the gate

Because `TimelineClip.Transform` is `map[string]any` with no key validation, adding these **static** fields needs no Go change at all — only the frontend `VideoTimelineTransform` interface and the renderer's `numericTransform` reads.

Making them **animatable** is the expensive part. Each of `z`, `rotation_x`, `rotation_y`, `scale_x`, `scale_y` must be added to `knownKeyframeProperties` (`timeline.go:137-144`) or the document is rejected outright. Under §3.5 that allowlist expansion must ship in a backend release **before** any frontend that authors such keyframes. Sequence this phase as:

1. Backend release: expand `knownKeyframeProperties`, keep renderer conservative, add validation tests.
2. Frontend release: author static spatial fields.
3. Frontend release: author spatial keyframes.

Skipping step 1 produces projects the user's previous build refuses to open.

### Preview implementation

`frontend/src/components/video/VideoPreviewCanvas.tsx` can initially use browser transforms such as:

```text
perspective(...)
translate3d(...)
rotateX(...)
rotateY(...)
rotateZ(...)
scale3d(...)
```

This phase may initially be preview-capable while export remains conservative. Note that `VideoPreviewCanvas.tsx` already runs an interval index, active-clip query, and decoder budget per frame (`:200-208`); 3D transforms must not bypass `applyDecoderBudget`, or a depth-heavy composition will exceed the decoder limit that currently protects preview.

### Primary files

- `frontend/src/types/video.ts`
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/components/video/pro/timelineIndex.ts`
- `frontend/src/components/video/pro/timelineAnalysis.ts` (new fidelity warnings)
- `frontend/src/stores/videoStudio.ts`
- `backend/internal/video/timeline.go` (keyframe allowlist, not struct fields)
- `backend/internal/video/renderer.go` (`numericTransform` reads)
- `backend/internal/video/renderer_capabilities.go`

## 4B. Camera track

### Proposed scene camera properties

```text
x
y
z
rotation_x
rotation_y
rotation_z
field_of_view
focus_depth
```

Camera values should be animatable. Since a camera is scene-scoped rather than clip-scoped, camera animation does **not** fit `TimelineClip.Keyframes` and therefore does not fit `knownKeyframeProperties`. Specify the camera animation container explicitly — a `VideoSceneCamera.keyframes` collection with its own validated property set is the cleaner option, and it avoids overloading the clip keyframe allowlist with dotted pseudo-properties (the same mistake that left effect-amount sampling dead in §2.5).

### Initial camera presets

- Push In
- Pull Out
- Pan Left
- Pan Right
- Crane Up
- Crane Down
- Dolly Left
- Dolly Right
- Orbit
- Gentle Handheld Drift
- Rack Focus (only after focus-depth rendering exists)

### Timeline UI

A selected scene may expose a dedicated camera lane:

```text
Camera  --- Push In ------ Orbit ------ Focus ---
```

Do not implement camera as a fake media clip unless that clearly simplifies the schema and validation model. Prefer explicit scene/camera semantics.

## Export policy

Spatial/camera features must initially report one of:

- preview only,
- partial,
- supported.

Only move to `supported` after FFmpeg parity is implemented and covered by golden-media tests. `RendererFeatureSupport` encodes this as `Supported bool` + `Partial bool`, so "preview only" is `Supported: false` with an explanatory note. Track solo used this shape at the roadmap baseline and was promoted only after Phase 0 added its tested export path.

## Acceptance criteria

- 2.5D preview is deterministic and performant on representative projects, and still respects the existing decoder budget.
- Z-order semantics are clearly defined relative to existing layer order (array index) and `TimelineClip.ZIndex` (`timeline.go:194`).
- Camera motion produces parallax when layers have different depth.
- Existing 2D timelines render identically.
- New animatable properties shipped backend-first per §3.5.
- Renderer capability metadata remains conservative, and `timelineAnalysis.ts` surfaces the preview-only status to the user.

---

# Phase 5 - Scene-level cinematic effects and motion templates

> **Implementation status:** COMPLETE — all nine effects have real preview/export paths and all requested templates have persistent replaceable slot identity.

## 5A. Scene-level cinematic effects

### Priority effects

Implement renderer-friendly effects first:

1. Film grain
2. Bloom
3. Color-grade presets
4. Edge fade
5. RGB/color split
6. Ghost/trail
7. Motion blur
8. Depth-of-field
9. Rack focus

None of these existed at the roadmap baseline. Phase 5 added all nine beside the original ten effect types (`blur`, `brightness`, `contrast`, `saturation`, `grayscale`, `shadow`, `background_blur`, `chroma_key`, `sharpen`, and `vignette`). The original `shadow` and `background_blur` limitations remain honestly reported rather than being conflated with the new tested scene effects.

Depth-aware effects (7–9) must wait for a stable spatial/camera model.

Scene-scoped effects also need a container decision: `TimelineEffect` is clip-scoped. `VideoSceneEffects` in Phase 3B's schema is the natural home, but its param validation must be defined rather than inheriting `TimelineEffect.Params map[string]any` unvalidated.

### Primary files

- `frontend/src/components/video/effects/effectRegistry.ts`
- `frontend/src/components/video/EffectBrowser.tsx`
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_fidelity.go`
- `backend/internal/video/renderer_capabilities.go`

### Acceptance criteria

- Every effect has one registry definition for UI/support metadata.
- Preview/export support labels are derived from actual capabilities, and `timelineAnalysis.ts` reports them.
- Golden-media tests cover every effect advertised as export-supported.
- Animated effect parameters are only offered after §2.5 is resolved.

## 5B. Remixable motion templates

### Objective

Evolve starter templates from fixed timeline scaffolds into reusable compositions with replaceable content slots while preserving motion, timing, effects, and camera behavior.

### Example templates

- App Demo
- Product Hero
- Floating Screens
- Feature Carousel
- Logo Reveal
- Social Quote
- Podcast Clip
- Vertical Product Ad
- Lower Third Pack
- Before / After
- Cinematic Title
- Photo Slideshow

### Slot model

Templates should identify replaceable roles such as:

```text
Logo
Hero Image
Screenshot 1
Screenshot 2
Headline
Body Copy
CTA
Music
```

Replacing a slot must not destroy unrelated animation/camera/effect data.

Slot identity needs a persistence decision. `TimelineClip` has no slot/role field; the available hooks are `TimelineClip.GroupID` (already used for grouping, so overloading it is a conflict) or document/clip `metadata`. Choose one and document it in `docs/VIDEO_TIMELINE_SCHEMA.md`, because a template whose slots are only tracked in frontend state cannot survive a save/reload — which is the entire point of remixability.

### Primary files

- `frontend/src/components/video/templates/timelineTemplates.ts`
- proposed template slot types in `frontend/src/types/video.ts` or a dedicated template type file
- `frontend/src/stores/videoStudio.ts`
- `backend/internal/video/timeline.go` if slot identity is persisted (it should be)

### Acceptance criteria

- Template content can be replaced without losing motion design.
- Slot identity survives save, reload, and export.
- Missing slots produce clear validation rather than broken references.
- Template application is deterministic and undoable where applied to an existing timeline.

---

# Phase 6 - Agentic Motion Director

> **Implementation status:** COMPLETE — governed, revision-bound inspect/mutate/render/status/cancel tools support a bounded three-pass visual refinement loop without hidden timeline state.

## Objective

Expand the current one-shot assistant plan model into a governed editing loop that can inspect, mutate, render, visually evaluate, and refine a video project.

## Current limitation

The current assistant contract is useful but intentionally narrow. Confirmed edit-plan operations (`types/video.ts:481`, mirrored in `assistant.go`):

- `set_canvas`
- `set_duration`
- `trim_clip`
- `add_text_clip`
- `move_clip`
- `delete_clip`
- `set_volume`
- `add_marker`
- `add_asset_clip`
- `set_transform`

Note the union ends in `| string`, so the frontend type does not constrain the operation set.

Three concrete weaknesses in `ApplyEditPlan` (`assistant.go:373-402`) that Phase 6 must fix rather than inherit:

1. **No timeline binding.** It calls `GetOrCreateTimeline(ctx, userID, projectID)` and mutates whatever timeline is active. A caller cannot say "apply to the timeline I inspected."
2. **No revision check.** It saves last-write-wins, so a plan generated against a stale document silently overwrites concurrent user edits.
3. **Silent partial application.** `ValidateEditPlanOperations` returns `issues`, but `ApplyEditPlan` discards them and applies the valid subset — reporting nothing about what was dropped unless every operation failed.

The next stage should not merely add dozens of strings to the same prompt. It should create bounded domain tools with explicit validation and authorization.

## Proposed governed tool families

The following names are proposed, not existing tools. The `video_*` prefix follows the existing `video_generate` convention (`tools/video_job_tool.go:25`).

```text
video_project_inspect
video_scene_list
video_layer_inspect
video_layer_update
video_animation_apply
video_camera_update
video_effect_apply
video_timeline_mutate
video_render_frame
video_render_preview
```

Tool granularity should be reviewed during implementation; avoid both a dangerous unrestricted `video_execute_json` tool and an excessively fragmented tool surface.

`video_render_preview` should follow the existing async job pattern (`video_generate` + `jobs.Manager`) rather than blocking a tool call, and should reuse `internal/jobs` progress/cancellation rather than inventing a second mechanism.

For inspection, `pro/timelineAnalysis.ts` already computes bounded timeline metrics, issues, and health from a document plus renderer capabilities. Its Go equivalent, or a shared contract, is the natural basis for `video_project_inspect` — an agent should receive the same honest fidelity picture the user sees, not a raw document dump.

## Required workflow

The target agent loop is:

```text
User instruction
      ->
inspect current project/timeline/selection
      ->
propose bounded changes
      ->
approval when required
      ->
apply changes  (bound to timeline id + revision)
      ->
render one or more diagnostic frames / short preview
      ->
vision-capable model evaluates actual output
      ->
refine if necessary within bounded iteration limits
      ->
present result and change summary (including dropped operations)
```

## Safety and governance requirements

- Reuse existing tool permission/scoped permission infrastructure in `backend/internal/tools/`.
- Bind every mutation to the authenticated user/workspace/project **and to an explicit timeline ID**.
- Validate tool inputs against the current timeline revision. `models.VideoTimeline` has `UpdatedAt` but no revision counter or content hash; add one, or bind on a hash of `TimelineJSON`.
- Reject rather than silently subset. If an agent plan contains invalid operations, return them; do not apply a partial plan without reporting the remainder.
- Bound rendered diagnostic frame count, preview duration, resolution, storage, and iteration count.
- Do not expose raw filesystem paths or FFmpeg command lines.
- Preserve existing approval behavior for side-effecting tool calls.
- Record enough metadata for diagnostics/audit without storing secrets or sensitive local paths.

## Primary existing files

- `backend/internal/video/assistant.go`
- `backend/internal/video/service.go`
- `backend/internal/video/timeline.go`
- `backend/internal/video/artifacts.go`
- `backend/internal/api/router.go` (video assistant routes at `:567-571`)
- `backend/internal/tools/video_job_tool.go` (existing precedent)
- `backend/internal/tools/`
- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/stores/videoStudio.ts`

Potential new backend files should follow existing tool naming and package conventions after inspecting the exact implementation pattern at that time.

## Render-and-inspect seam

No composite frame-render path exists today. `GenerateAssetArtifacts` (`artifacts.go:16`) produces single-asset thumbnails and waveforms, not a composited timeline frame — so this seam is genuinely new work.

Two existing pure transforms are the right foundation and should be reused rather than reimplemented:

- `SliceTimelineRange(doc, startMS, endMS)` (`timeline.go:323`) derives a bounded time window without mutating the source document.
- `StripCaptionOverlays(doc)` (`timeline.go:424`) derives a caption-free variant.

A frame-render endpoint/tool should be designed separately from full export jobs if that produces materially lower latency and resource use. It must still route through `renderer.go` so the agent judges export-relevant output rather than a browser-only preview — and it must run `ValidateTimelineDocument` like `renderer.go:102` does, so a diagnostic render cannot bypass validation the export path enforces.

## Acceptance criteria

- Agent can inspect current timeline state without receiving unbounded project data.
- Agent can make approved, validated edits through normal domain operations, bound to a timeline ID and revision.
- A stale plan is rejected with an actionable error, not applied over newer user edits.
- Agent can render a bounded diagnostic frame or preview artifact through the real renderer path.
- A vision-capable model can use the diagnostic artifact to decide whether another bounded edit is needed.
- Iteration has a hard maximum and cancellation support.
- User receives a concise list/diff of changes that were actually applied **and of operations that were rejected**.
- Manual edits remain first-class; the agent never owns a hidden timeline state.

---

# Phase 7 - Optional review/share workflow

> **Implementation status:** NOT ACTIVATED — this phase defines no required feature and explicitly makes sharing optional. No hosted deployment model was selected; local-first export is unchanged.

## Objective

Consider collaboration only after core motion-design and agentic editing are mature.

Do not turn OmniLLM-Studio into a mandatory hosted video service.

Potential deployment-aware model:

```text
Desktop/local
  -> export review package

Server deployment
  -> optional authenticated, expiring review URL

Workspace deployment
  -> member/comment review tied to workspace permissions
```

This phase is explicitly lower priority than editing, rendering, and agent reliability.

---

# 5. Recommended PR sequence

Keep implementation reviewable. Do not land the entire roadmap in one branch.

## PR family A - Motion UX foundation

Scope:

- `motion_design` editor mode, threaded through the Ultimate/Enhanced/base shell chain
- `EditorModeFeatures` expansion across all five modes
- Design / Animate inspector organization
- animation block registry/compiler, absorbing `MOTION_PRESETS`
- initial In/During/Out blocks compiling to the six allowlisted keyframe properties

Expected risk: low to medium.

No backend schema change is required **provided** every block stays inside the existing keyframe property and easing allowlists. That constraint is what makes this family backend-free; a single block needing a new property moves it into family B.

## PR family B - Timeline motion model

Scope:

- additive `curve` field alongside `easing` (no version bump)
- spring/Bezier deterministic sampling with defined boundary velocity
- scene/shot ranges as an optional `scenes` field
- validation, overlap rejection, and upgrade tests
- scene strip UI

Expected risk: medium.

Requires backend/frontend contract changes and compatibility tests in both directions.

## PR family C - Spatial/camera system

Scope:

- **C1 (backend-first):** `knownKeyframeProperties` expansion for spatial properties, scene camera keyframe container and validation, renderer left conservative
- **C2:** static 2.5D transform fields and inspector controls
- **C3:** 3D browser preview honoring the existing decoder budget
- **C4:** camera model, camera lane, parallax
- fidelity warnings via `timelineAnalysis.ts`

Expected risk: high.

C1 must ship in a release before C2–C4 per §3.5. Preview support may land before export parity if clearly labeled.

## PR family D - Cinematic renderer parity and templates

Scope:

- renderer implementation for selected 2.5D/camera/effects
- golden-media tests
- motion templates with persisted slot identity
- capability upgrades only when proven

Expected risk: high.

## PR family E - Agentic Motion Director

Scope:

- bounded video inspection/edit tools
- timeline-ID-bound, revision-aware mutations
- rejected-operation reporting
- frame/preview diagnostic rendering through the real renderer
- vision inspection loop
- iteration/cancellation/approval semantics

Expected risk: high and security-sensitive.

Requires explicit tool-permission and adversarial test review.

---

# 6. Schema and compatibility strategy

## Timeline versions

Every persisted timeline change must:

1. increment the timeline schema version **only when a breaking reinterpretation lands** — not for additive optional fields,
2. update Go structs where the field is typed (note: `TimelineClip.Transform` is `map[string]any`, so transform additions have no Go struct step),
3. update `frontend/src/types/video.ts` to mirror the JSON contract,
4. update `ValidateTimelineDocument`, including `knownKeyframeProperties` for any new animatable property,
5. update `UpgradeTimelineDocument` when a version step exists,
6. add upgrade tests from representative older versions,
7. add a **downgrade** test asserting the older validator's behavior on the new document — reject or degrade — and confirming that behavior is the intended one,
8. update `docs/VIDEO_TIMELINE_SCHEMA.md`.

## Compatibility defaults

Recommended defaults:

- missing Z = `0`
- missing X/Y tilt = `0`
- missing perspective = scene/canvas default
- missing scenes = implicit full-duration scene for scene-aware UI, unpersisted
- missing camera = identity camera
- old `scale` maps to uniform X/Y scale
- old `rotation` maps to Z rotation
- old easing strings preserve current behavior exactly; `curve` wins when present and should carry a nearest-equivalent `easing` fallback

Do not mutate persisted files solely to add default-valued optional fields unless a normal save occurs.

## Validation asymmetry to keep in mind

| Surface | Current behavior | Consequence for new fields |
| --- | --- | --- |
| `TimelineClip.Transform` keys | untyped map, never validated | additive and downgrade-safe; no backend gate, but also no safety net |
| `TimelineKeyframe.Property` | strict allowlist, **rejects whole document** | must ship backend-first; hard downgrade break |
| `TimelineKeyframe.Easing` | allowlist, **silently coerced to `linear`** | silent fidelity loss on downgrade; pair new curves with a fallback |
| `TimelineEffect.Params` | untyped map, never validated | additive, but scene effects need explicit param validation |
| `TimelineDocument.Version` | future versions **hard fail** | a bump makes projects unopenable on older desktop builds |

---

# 7. Renderer fidelity policy

The renderer capability matrix remains the contract between timeline features and user expectations — but only because `pro/timelineAnalysis.ts` reads it and renders warnings. Both halves are required.

For every new feature:

1. preview implementation,
2. backend schema validation,
3. renderer implementation or explicit unsupported/partial status,
4. deterministic renderer tests,
5. golden-media test where visual fidelity matters,
6. fidelity surfacing through `timelineAnalysis.ts`,
7. only then capability promotion.

No UI feature should hide export limitations, and no capability note should describe a path that validation rejects (see §2.5).

Potential capability additions may include:

```text
spatial_3d_transform
camera_motion
camera_parallax
bezier_curves
spring_curves
film_grain
bloom
motion_blur
depth_of_field
rack_focus
```

Names above are proposed and should be finalized with the renderer implementation, following the existing `RendererFeature*` constant convention.

---

# 8. Performance requirements

Motion-design features can increase browser and render cost materially.

Required safeguards, stated against the mechanisms that actually exist:

- preserve timeline virtualization — `visibleClips` in `pro/timelineIndex.ts`, consumed by `TimelineTrack.tsx:281`,
- preserve preview windowing and the decoder budget — `buildTimelineIntervalIndex` / `queryActiveClips` / `applyDecoderBudget` in `VideoPreviewCanvas.tsx:200-208`,
- avoid mounting hidden animation/property editors for every clip,
- memoize sampled curve data,
- batch pointer-drag preview updates and commit once on pointer-up,
- keep animation-drag patches small enough not to thrash the `PatchHistory` byte budget (`VideoEditStudioUltimate.tsx:41`, 8–256 MB, default 32 MB) — measure worst-case patch size per block,
- respect the `timelineCommandEngine` 50-entry history limit where mutations route through it,
- keep preview frame computation bounded,
- use poster/proxy behavior where existing decoder budgets require it,
- cap agent diagnostic renders,
- add large-project fixtures before spatial/camera work is declared complete.

Representative performance fixtures should include:

- many layers,
- many keyframes,
- several animation blocks,
- multiple effects,
- audio tracks,
- captions,
- at least one camera-enabled scene once supported.

---

# 9. Validation matrix

Every applicable implementation PR should run the repository's confirmed validation commands.

## Backend

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/server
```

On Linux environments where Wails desktop packages are included and WebKit2GTK 4.1 is required, preserve the repository's documented build tags (`GOFLAGS=-tags=webkit2_41`).

## Frontend

```bash
cd frontend
npm ci
npm run lint
npm run test:unit
npm run build
```

## Browser smoke tests

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:smoke
```

Existing video specs: `tests/video-editor.smoke.spec.ts`, `video-editor-advanced.smoke.spec.ts`, `video-editor-source-media.smoke.spec.ts`, `video-editor-recording-lab.smoke.spec.ts`, `video-omni.smoke.spec.ts`.

## Feature-specific validation

Add as phases require:

- timeline schema upgrade tests,
- timeline **downgrade** behavior tests (§6 step 7),
- animation compiler unit tests,
- spring/Bezier deterministic sampling tests including boundary velocity,
- preview interaction tests,
- scene reorder/boundary/overlap-rejection tests,
- camera/parallax visual tests,
- renderer capability tests,
- golden-media output tests,
- agent mutation authorization/ownership tests,
- agent stale-revision rejection tests,
- agent iteration and cancellation tests,
- large-project performance evidence.

---

# 10. Risks and mitigations

## Risk: two animation truth models

**Mitigation:** semantic blocks compile to keyframes/scene properties; manual keyframes remain authoritative/editable.

## Risk: two motion registries

**Mitigation:** Phase 2 absorbs or migrates the existing `MOTION_PRESETS`; exactly one registry survives the phase.

## Risk: a third history mechanism

**Mitigation:** two already exist (`withTimelineHistory` in the store, `PatchHistory` at the Ultimate shell, plus `timelineCommandEngine`'s own commit path). Every new mutation declares which one owns it.

## Risk: preview/export divergence

**Mitigation:** conservative capability matrix, renderer-first tests, golden-media evidence, and mandatory surfacing through `timelineAnalysis.ts`.

## Risk: capability matrix claims an unreachable path

**Mitigation:** Phase 0 closes the known effect-amount case (§2.5); every capability note must be backed by a test that exercises the path end to end through `ValidateTimelineDocument`.

## Risk: schema complexity

**Mitigation:** additive optional fields, explicit upgrade functions, no unnecessary nested storage redesign.

## Risk: new fields make projects unopenable on the user's previous build

**Mitigation:** §3.5 forward-compatibility rules — backend-first allowlist releases, no gratuitous version bump, nearest-equivalent fallbacks for new curve data, and an explicit downgrade test per schema change.

## Risk: 3D transforms destabilize layer ordering

**Mitigation:** define interaction between timeline layer order (array index), `TimelineClip.ZIndex`, and spatial Z before implementation; test overlap and negative/positive depth cases.

## Risk: 3D preview bypasses the decoder budget

**Mitigation:** spatial compositing must route through `applyDecoderBudget`; add a fixture with more depth layers than the decoder limit.

## Risk: camera features create an After Effects-sized UI

**Mitigation:** keep Motion Design mode preset-first; advanced numeric/keyframe controls remain available but not mandatory.

## Risk: animation sampling causes performance regressions

**Mitigation:** cache sampled curves, limit recalculation to affected properties/time windows, preserve virtualization, add large-project budgets.

## Risk: agent edits overwrite newer user work

**Mitigation:** `ApplyEditPlan` today has no timeline binding and no revision check. Add a revision counter or `TimelineJSON` hash and bind every mutation to it.

## Risk: agent silently applies a partial plan

**Mitigation:** return rejected operations to the caller and surface them in the change summary; never drop `issues` at the service layer.

## Risk: agent render-inspect loops consume unbounded resources

**Mitigation:** hard iteration/render/resolution/duration/storage limits plus cancellation and normal tool approval policy; reuse `internal/jobs` rather than a bespoke loop.

---

# 11. Definition of completion

This roadmap is complete when OmniLLM-Studio Video Edit Studio can truthfully provide all of the following:

- A focused Motion Design editor mode reachable from the mounted shell.
- Clear Design vs Animate property workflows.
- Reusable In/During/Out animation blocks in a single motion registry.
- Rich motion curves including deterministic spring/Bezier behavior.
- Scene/shot-level navigation over the existing timeline.
- 2.5D layer composition with depth and tilt.
- Camera animation with real parallax in supported compositions.
- A useful set of cinematic scene effects with honest export support.
- Remixable motion templates with persisted, replaceable content slots.
- Renderer capability metadata backed by tests and golden-media evidence, with every claim reachable through validation.
- A governed agent tool surface that can inspect and edit video projects, bound to timeline revisions.
- A bounded render-frame/preview -> visual inspection -> refinement loop.
- Documented forward- and backward-compatibility behavior for every schema change.
- No regression in existing NLE, caption, audio, recording, generation, or export workflows.

# 12. Explicit non-goals

Unless separately chartered, this roadmap does not commit to:

- replacing FFmpeg with a different primary renderer,
- replacing the neutral timeline with a Raylight-specific project model,
- implementing full arbitrary 3D geometry or a general 3D scene engine,
- implementing a creator marketplace,
- mandatory cloud hosting,
- real-time multi-user collaborative editing,
- copying Raylight's visual design or proprietary implementation.

# 13. Source references

External product-reference material used when developing this roadmap:

- `https://www.raylight.app/`
- `https://www.raylight.app/academy/design-and-animate`
- `https://www.raylight.app/academy/the-timeline`
- `https://www.raylight.app/academy/your-first-shot`
- `https://www.raylight.app/academy/cinematic-effects`
- `https://www.raylight.app/academy/export-and-share`
- `https://www.raylight.app/agents`
- `https://www.raylight.app/mcp`

Repository documents that remain authoritative for current Omni implementation details:

- `docs/MASTER_PLAN.md`
- `docs/VIDEO_STUDIO.md`
- `docs/VIDEO_STUDIO_ARCHITECTURE.md`
- `docs/VIDEO_TIMELINE_SCHEMA.md`
- `docs/VIDEO_RENDERING.md`
- `docs/VIDEO_RENDERER_RELIABILITY_TRANSCRIPTION_SCALABILITY_2026-07-20.md`
- `CLAUDE.md`

When this roadmap and current implementation diverge, update this document as phases land rather than leaving completed or superseded items ambiguous.

# 14. Review log

## 2026-08-17 — accuracy and completeness review against `b4520f8`

Verified as accurate: the four existing editor modes; the six keyframe properties and five easings; the ten edit-plan operations; the fidelity debt list (matches `renderer_capabilities.go` and `MASTER_PLAN.md:66` verbatim); all seven referenced repository documents exist; the store's `clone -> mutate -> snapshot -> autosave` convention; the assistant endpoint set; the absence of any CI performance job.

Corrected or added:

1. Baseline commit was a sandbox commit 15 revisions behind `main`; re-pinned and noted that no video code changed in the interval.
2. The editor is a three-shell wrapper chain (`Ultimate -> Enhanced -> VideoEditStudio`); the draft named only the inner component.
3. Added the `pro/` subsystem, which was absent entirely and which owns virtualization, the decoder budget, the fidelity-warning surface, patch-based undo, and an existing command engine.
4. `TimelineClip.Transform` is `map[string]any` with unvalidated keys — spatial transform additions need no Go struct change, contradicting the draft's blanket "update Go structs" step.
5. `knownKeyframeProperties` hard-rejects unknown properties, making every new animatable property a backend-first, downgrade-breaking change. Added §3.5 forward compatibility and resequenced PR family C into C1–C4.
6. The proposed `MotionCurve` union replacing `Easing string` was schema-breaking and contradicted the roadmap's own additive principle; replaced with an additive `curve` sibling field plus a fallback `easing`, and flagged that unknown easings coerce silently to `linear`.
7. `renderer_capabilities.go:67` advertises effect-amount keyframe export, which validation rejects and no code authors; added to Phase 0 and used to gate the Blur Reveal / Blur Out blocks.
8. `motionPresets.ts` already implements the Phase 2 pattern and ships three overlapping presets that replace all x/y/scale keyframes; Phase 2 now absorbs it rather than adding a second registry.
9. `ApplyEditPlan` has no timeline binding, no revision check, and silently applies a partial plan; documented all three as Phase 6 requirements.
10. No composite frame-render path exists; named `SliceTimelineRange` and `StripCaptionOverlays` as the reuse seam and `artifacts.go` as the single-asset precedent.
11. `video_generate` already establishes the `video_*` tool naming and async job pattern.
12. Phase 5A (authoring cinematic effects) was missing from the execution order.
13. Scenes do not need a version bump; a bump is a downgrade hazard for desktop users.
14. Added missing container decisions: scene camera keyframes, scene effect params, and template slot identity — none of which have a home in the current schema.

## 2026-08-17 — implementation completion review

Phases 0–6 were implemented on the current branch and verified using the evidence table at the top of this document. The implementation kept timeline schema version 1 additive, preserved the existing ordered-layer/patch-history architecture, used one animation registry, and did not introduce a hidden agent timeline. Phase 7 remains an explicit product/deployment decision rather than silently adding a hosted collaboration surface.
