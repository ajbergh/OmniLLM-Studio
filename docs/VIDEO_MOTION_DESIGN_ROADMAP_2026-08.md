# Video Motion Design Roadmap

> **Status:** PLANNED
>
> **Purpose:** Durable implementation plan for evolving OmniLLM-Studio Video Edit Studio from a capable nonlinear editor into a combined NLE + motion-design + agentic video-composition workspace.
>
> **Baseline:** Reviewed against `main` at `11dfab99e73fe414e45cc44b0f33d4c80789295a` on 2026-08-16.
>
> **Product reference:** Raylight (`https://www.raylight.app`) was reviewed as a motion-design reference. The goal is not product cloning. The useful concepts are its Design/Animate separation, semantic animation blocks, scene/shot navigation, depth/camera model, cinematic effects, reusable motion templates, and agent-addressable editing model.

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

Confirmed current capabilities include:

- Neutral timeline JSON using generic ordered layers.
- Clip move, trim, split, speed, ripple, overwrite, grouping, snapping, markers, copy/paste, undo/redo, and layer operations.
- Preview compositing with transforms, opacity, crop, effects, keyframes, styled text, direct manipulation, smart guides, inline text editing, and crop mode.
- Keyframes for `x`, `y`, `scale`, `rotation`, `opacity`, and `volume` with linear/ease/step interpolation.
- Effects and transitions with preview/export capability reporting.
- Captions, transcription, screen/camera/voice recording, audio ducking, fades, waveform display, render queue, export validation, codec options, and durable FFmpeg jobs.
- AI storyboard/edit-plan/social-variant endpoints.
- A conservative renderer capability matrix in `backend/internal/video/renderer_capabilities.go`.
- Existing editor modes in `frontend/src/components/video/editorModes.ts`.

The current transform and animation model is still fundamentally 2D:

```text
Transform:
  x
  y
  scale
  rotation
  opacity
  crop

Keyframe properties:
  x
  y
  scale
  rotation
  opacity
  volume
```

The roadmap below adds motion-design semantics while preserving existing timelines and current NLE behavior.

## 3. Architectural principles

### 3.1 Preserve the neutral timeline

Do not create a second incompatible motion-project format. New motion-design concepts must extend the existing `VideoTimelineDocument` schema through normal versioned upgrade logic.

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

This keeps manual keyframes editable and avoids two competing animation systems.

### 3.3 Preview support does not imply export support

Every new transform, effect, camera, and animation capability must remain conservative in `renderer_capabilities.go` until the FFmpeg path is implemented and covered by renderer/golden-media tests.

### 3.4 Backward compatibility is mandatory

Existing timelines must continue to load without destructive rewrites. New fields must be optional with stable defaults. Timeline upgrade functions must normalize older documents explicitly.

### 3.5 UI simplification is additive

Motion Design mode should hide irrelevant NLE controls without changing persisted timeline semantics, matching the current editor-mode strategy.

### 3.6 Agent mutations use existing governance

Video-editing tools exposed to agents must use the existing Omni tool registry, permissions, approval policy, ownership checks, bounded inputs/outputs, and audit paths. Do not create a privileged side channel specifically for Video Studio AI.

## 4. Execution order

1. Finish the current Video Studio performance/export validation prerequisite.
2. Add Motion Design UI mode and Design/Animate inspector organization.
3. Add semantic animation blocks that compile to editable keyframes.
4. Expand curve semantics and introduce scene/shot ranges.
5. Add 2.5D transforms and camera semantics, preview first.
6. Add renderer parity for the new spatial/cinematic features.
7. Add remixable motion templates.
8. Expand the assistant into a governed agentic motion director with render-and-inspect loops.
9. Consider optional collaborative review/share surfaces only after the local-first editing path is mature.

---

# Phase 0 - Existing renderer and performance validation

## Objective

Complete the reliability work already identified in `docs/MASTER_PLAN.md` before significantly increasing animation complexity.

## Scope

- Add representative large-project browser performance fixtures.
- Capture frame-time / React commit / memory evidence in CI where practical.
- Preserve conservative renderer-capability reporting.
- Select and close at least one existing renderer-fidelity gap with golden-media coverage before marking new motion features export-capable.

Current known fidelity debt includes:

- true two-clip crossfades,
- rounded/geometric annotation fidelity,
- drop shadow/background blur,
- continuous-curve fidelity,
- track-solo export semantics,
- click-audio synthesis.

## Primary existing files

- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_capabilities.go`
- `frontend/src/components/video/`
- existing `tests/video-editor-*.spec.ts` coverage
- `.github/workflows/ci.yml`

## Acceptance criteria

- A reproducible large-timeline fixture exists.
- CI has a documented performance budget or at minimum produces stable performance evidence for regression review.
- No capability is upgraded in `renderer_capabilities.go` without renderer tests.
- At least one current fidelity gap is closed with golden-media or deterministic output assertions.

---

# Phase 1 - Motion Design editor mode

## Objective

Create a focused motion-design workspace using the current Video Edit Studio shell and timeline semantics.

## User experience

Add a new editor mode:

```text
Full editor
Simple trim
Captions only
Social clip
Motion design
```

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
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/components/video/timeline/KeyframeLane.tsx`
- `frontend/src/stores/videoStudio.ts`

## Data model

No timeline schema change is required for the first Motion Design UI release.

## Tests

- Unit coverage for mode feature gates.
- Playwright smoke coverage proving the mode can be entered and exited without changing persisted timeline content.
- Regression coverage proving a timeline edited in Motion Design mode opens identically in Full Editor mode.

## Acceptance criteria

- Motion Design mode is available without duplicating editor state.
- Static and animated properties are clearly separated in the inspector.
- No existing editor mode loses functionality.
- Existing timelines require no migration merely to enter Motion Design mode.

---

# Phase 2 - Semantic animation blocks

## Objective

Add higher-level motion behaviors so users do not need to author every animation with individual keyframes.

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
- Slow Zoom
- Pan
- Gentle Rotate
- Ken Burns

### Out

- Fade Out
- Slide Out
- Scale Out
- Drop Away
- Blur Out

## Proposed frontend files

The following are **new proposed files**, not current repository files:

- `frontend/src/components/video/motion/animationBlockRegistry.ts`
- `frontend/src/components/video/motion/AnimationBlockPicker.tsx`
- `frontend/src/components/video/motion/AnimationBlockEditor.tsx`
- `frontend/src/components/video/motion/animationBlockCompiler.ts`

Extend existing:

- `frontend/src/components/video/effects/motionPresets.ts`
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

## Undo/save behavior

Follow the existing mutation convention:

```text
clone -> mutate -> one undo snapshot -> autosave
```

Applying or modifying one animation block should produce one undoable user action.

## Acceptance criteria

- Blocks generate deterministic keyframes.
- Generated keyframes remain editable in the existing keyframe lane.
- Reapplying a block has explicit replace/stack behavior; it never silently destroys unrelated manual keyframes.
- Undo removes the entire block mutation in one step.
- Preview and export behavior remain identical to manually-authored equivalent keyframes.

---

# Phase 3 - Motion curves and scene/shot ranges

## 3A. Richer easing and springs

### Objective

Replace the current limited easing vocabulary with a forward-compatible curve model while preserving old values.

### Proposed concept

```ts
type MotionCurve =
  | { type: 'linear' }
  | { type: 'ease-in' }
  | { type: 'ease-out' }
  | { type: 'ease-in-out' }
  | { type: 'step' }
  | { type: 'bezier'; x1: number; y1: number; x2: number; y2: number }
  | { type: 'spring'; stiffness: number; damping: number; mass: number };
```

Exact persisted shape should be finalized in a schema-design PR before implementation.

### Primary files

- `frontend/src/types/video.ts`
- `frontend/src/components/video/effects/keyframeUtils.ts`
- `frontend/src/components/video/timeline/KeyframeLane.tsx`
- `backend/internal/video/timeline.go`
- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_capabilities.go`

### Renderer strategy

Bezier and spring motion should be deterministically sampled to renderer segments first. Continuous/native curve rendering can be considered later if FFmpeg support and parity are reliable.

### Acceptance criteria

- Existing string easing values upgrade losslessly.
- Frontend preview and backend sampling share tested curve semantics.
- Spring motion is deterministic for the same parameters and FPS/time window.
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

The exact names/types above are proposed and must mirror the eventual Go structs.

### Behavior

- A scene represents a time range over the current flat timeline.
- Existing timelines upgrade to one implicit/default scene spanning the document duration when scene-aware UI is required.
- Reordering a scene moves all clips wholly contained by that scene range.
- Cross-boundary clips must have explicit behavior; do not silently split or reassign them.
- Scene duplicate/delete operations must be deterministic and undoable.

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
- `backend/internal/video/timeline.go`
- `docs/VIDEO_TIMELINE_SCHEMA.md`

### Acceptance criteria

- Existing timelines load without semantic change.
- Scene operations produce deterministic timeline transforms.
- Scene boundaries are visible and snap-capable.
- Scene operations are covered by frontend store tests and backend timeline validation tests.

---

# Phase 4 - 2.5D transforms and camera model

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

This phase may initially be preview-capable while export remains conservative.

### Primary files

- `frontend/src/types/video.ts`
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/stores/videoStudio.ts`
- `backend/internal/video/timeline.go`
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

Camera values should be animatable.

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

Only move to `supported` after FFmpeg parity is implemented and covered by golden-media tests.

## Acceptance criteria

- 2.5D preview is deterministic and performant on representative projects.
- Z-order semantics are clearly defined relative to existing layer order / `z_index` behavior.
- Camera motion produces parallax when layers have different depth.
- Existing 2D timelines render identically after schema upgrade.
- Renderer capability metadata remains conservative.

---

# Phase 5 - Scene-level cinematic effects and motion templates

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

Depth-aware effects must wait for a stable spatial/camera model.

### Primary files

- `frontend/src/components/video/effects/effectRegistry.ts`
- `frontend/src/components/video/EffectBrowser.tsx`
- `frontend/src/components/video/VideoPreviewCanvas.tsx`
- `backend/internal/video/renderer.go`
- `backend/internal/video/renderer_capabilities.go`

### Acceptance criteria

- Every effect has one registry definition for UI/support metadata.
- Preview/export support labels are derived from actual capabilities.
- Golden-media tests cover every effect advertised as export-supported.

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

### Primary files

- `frontend/src/components/video/templates/timelineTemplates.ts`
- proposed template slot types in `frontend/src/types/video.ts` or a dedicated template type file
- `frontend/src/stores/videoStudio.ts`
- backend changes only if template persistence/import requires them

### Acceptance criteria

- Template content can be replaced without losing motion design.
- Missing slots produce clear validation rather than broken references.
- Template application is deterministic and undoable where applied to an existing timeline.

---

# Phase 6 - Agentic Motion Director

## Objective

Expand the current one-shot assistant plan model into a governed editing loop that can inspect, mutate, render, visually evaluate, and refine a video project.

## Current limitation

The current assistant contract is useful but intentionally narrow. Existing edit-plan operations include concepts such as:

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

The next stage should not merely add dozens of strings to the same prompt. It should create bounded domain tools with explicit validation and authorization.

## Proposed governed tool families

The following names are proposed, not existing tools:

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
apply changes
      ->
render one or more diagnostic frames / short preview
      ->
vision-capable model evaluates actual output
      ->
refine if necessary within bounded iteration limits
      ->
present result and change summary
```

## Safety and governance requirements

- Reuse existing tool permission/scoped permission infrastructure.
- Bind every mutation to the authenticated user/workspace/project.
- Validate tool inputs against the current timeline revision where practical.
- Prefer optimistic revision checks so the agent does not overwrite newer user edits.
- Bound rendered diagnostic frame count, preview duration, resolution, storage, and iteration count.
- Do not expose raw filesystem paths or FFmpeg command lines.
- Preserve existing approval behavior for side-effecting tool calls.
- Record enough metadata for diagnostics/audit without storing secrets or sensitive local paths.

## Primary existing files

- `backend/internal/video/assistant.go`
- `backend/internal/video/service.go`
- `backend/internal/video/timeline.go`
- `backend/internal/api/router.go`
- `backend/internal/tools/`
- `frontend/src/components/video/VideoInspector.tsx`
- `frontend/src/stores/videoStudio.ts`

Potential new backend files should follow existing tool naming and package conventions after inspecting the exact implementation pattern at that time.

## Render-and-inspect seam

A frame-render endpoint/tool should be designed separately from full export jobs if that produces materially lower latency and resource use. It must still use the real renderer/composition path so the agent is judging export-relevant output rather than a different browser-only preview.

## Acceptance criteria

- Agent can inspect current timeline state without receiving unbounded project data.
- Agent can make approved, validated edits through normal domain operations.
- Agent can render a bounded diagnostic frame or preview artifact.
- A vision-capable model can use the diagnostic artifact to decide whether another bounded edit is needed.
- Iteration has a hard maximum and cancellation support.
- User receives a concise list/diff of changes that were actually applied.
- Manual edits remain first-class; the agent never owns a hidden timeline state.

---

# Phase 7 - Optional review/share workflow

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

- `motion_design` editor mode
- Design / Animate inspector organization
- animation block registry/compiler
- initial In/During/Out blocks compiling to current keyframes

Expected risk: low to medium.

No backend schema change should be required initially.

## PR family B - Timeline motion model

Scope:

- richer easing representation
- spring/Bezier sampling
- scene/shot ranges
- schema upgrade + validation
- scene strip UI

Expected risk: medium.

Requires backend/frontend contract changes and migration tests.

## PR family C - Spatial/camera system

Scope:

- 2.5D transform fields
- 3D browser preview
- camera model + camera lane
- parallax
- conservative renderer capability reporting

Expected risk: high.

Preview support may land before export parity if clearly labeled.

## PR family D - Cinematic renderer parity and templates

Scope:

- renderer implementation for selected 2.5D/camera/effects
- golden-media tests
- motion templates / slot replacement
- capability upgrades only when proven

Expected risk: high.

## PR family E - Agentic Motion Director

Scope:

- bounded video inspection/edit tools
- revision-aware mutations
- frame/preview diagnostic rendering
- vision inspection loop
- iteration/cancellation/approval semantics

Expected risk: high and security-sensitive.

Requires explicit tool-permission and adversarial test review.

---

# 6. Schema and compatibility strategy

## Timeline versions

Every persisted timeline change must:

1. increment the timeline schema version only when required,
2. update Go structs,
3. update `frontend/src/types/video.ts` to mirror the JSON contract,
4. update `ValidateTimelineDocument`,
5. update `UpgradeTimelineDocument`,
6. add upgrade tests from representative older versions,
7. update `docs/VIDEO_TIMELINE_SCHEMA.md`.

## Compatibility defaults

Recommended defaults:

- missing Z = `0`
- missing X/Y tilt = `0`
- missing perspective = scene/canvas default
- missing scenes = implicit full-duration scene for scene-aware UI
- missing camera = identity camera
- old `scale` maps to uniform X/Y scale
- old `rotation` maps to Z rotation
- old easing strings preserve current behavior exactly

Do not mutate persisted files solely to add default-valued optional fields unless a normal save occurs.

---

# 7. Renderer fidelity policy

The renderer capability matrix remains the contract between timeline features and user expectations.

For every new feature:

1. preview implementation,
2. backend schema validation,
3. renderer implementation or explicit unsupported/partial status,
4. deterministic renderer tests,
5. golden-media test where visual fidelity matters,
6. only then capability promotion.

No UI feature should hide export limitations.

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

Names above are proposed and should be finalized with the renderer implementation.

---

# 8. Performance requirements

Motion-design features can increase browser and render cost materially.

Required safeguards:

- preserve timeline virtualization,
- avoid mounting hidden animation/property editors for every clip,
- memoize sampled curve data,
- batch pointer-drag preview updates and commit once on pointer-up,
- avoid creating excessive undo snapshots during animation drags,
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

On Linux environments where Wails desktop packages are included and WebKit2GTK 4.1 is required, preserve the repository's documented build tags.

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

## Feature-specific validation

Add as phases require:

- timeline schema upgrade tests,
- animation compiler unit tests,
- spring/Bezier deterministic sampling tests,
- preview interaction tests,
- scene reorder/boundary tests,
- camera/parallax visual tests,
- renderer capability tests,
- golden-media output tests,
- agent mutation authorization/ownership tests,
- agent iteration and cancellation tests,
- large-project performance evidence.

---

# 10. Risks and mitigations

## Risk: two animation truth models

**Mitigation:** semantic blocks compile to keyframes/scene properties; manual keyframes remain authoritative/editable.

## Risk: preview/export divergence

**Mitigation:** conservative capability matrix, renderer-first tests, golden-media evidence.

## Risk: schema complexity

**Mitigation:** additive optional fields, explicit upgrade functions, no unnecessary nested storage redesign.

## Risk: 3D transforms destabilize layer ordering

**Mitigation:** define interaction between timeline layer order, `z_index`, and spatial Z before implementation; test overlap and negative/positive depth cases.

## Risk: camera features create an After Effects-sized UI

**Mitigation:** keep Motion Design mode preset-first; advanced numeric/keyframe controls remain available but not mandatory.

## Risk: animation sampling causes performance regressions

**Mitigation:** cache sampled curves, limit recalculation to affected properties/time windows, preserve virtualization, add large-project budgets.

## Risk: agent edits overwrite newer user work

**Mitigation:** revision-aware operations and/or timeline hash binding before mutation.

## Risk: agent render-inspect loops consume unbounded resources

**Mitigation:** hard iteration/render/resolution/duration/storage limits plus cancellation and normal tool approval policy.

---

# 11. Definition of completion

This roadmap is complete when OmniLLM-Studio Video Edit Studio can truthfully provide all of the following:

- A focused Motion Design editor mode.
- Clear Design vs Animate property workflows.
- Reusable In/During/Out animation blocks.
- Rich motion curves including deterministic spring/Bezier behavior.
- Scene/shot-level navigation over the existing timeline.
- 2.5D layer composition with depth and tilt.
- Camera animation with real parallax in supported compositions.
- A useful set of cinematic scene effects with honest export support.
- Remixable motion templates with replaceable content slots.
- Renderer capability metadata backed by tests and golden-media evidence.
- A governed agent tool surface that can inspect and edit video projects.
- A bounded render-frame/preview -> visual inspection -> refinement loop.
- No regression in existing NLE, caption, audio, recording, generation, or export workflows.

## 12. Explicit non-goals

Unless separately chartered, this roadmap does not commit to:

- replacing FFmpeg with a different primary renderer,
- replacing the neutral timeline with a Raylight-specific project model,
- implementing full arbitrary 3D geometry or a general 3D scene engine,
- implementing a creator marketplace,
- mandatory cloud hosting,
- real-time multi-user collaborative editing,
- copying Raylight's visual design or proprietary implementation.

## 13. Source references

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
