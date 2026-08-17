# Video Timeline Schema

Video timelines persist one renderer-neutral JSON document. The current schema version remains **1**; motion-design additions are optional and additive.

```json
{
  "version": 1,
  "canvas": { "width": 1920, "height": 1080, "fps": 30, "background": "#000000" },
  "duration_ms": 30000,
  "tracks": [],
  "markers": [],
  "scenes": [],
  "metadata": {}
}
```

`ValidateTimelineDocument` is authoritative for defaults and rejection. Older v1 documents require no migration. A document with a future version is rejected with an actionable upgrade error; `UpgradeTimelineDocument` is reserved for a future breaking reinterpretation.

## Compatibility and downgrade behavior

- Missing `scenes` means one implicit full-timeline scene in scene-aware UI. Nothing is persisted until the user edits scenes.
- Missing spatial transform fields keep the existing 2D defaults.
- `keyframe.curve` is optional. `curve` wins when present; `easing` remains a nearest-equivalent fallback for older builds (`spring` → `ease-out`, Bezier → `ease-in-out`).
- Semantic animation blocks are provenance only. Ordinary keyframes remain rendering truth, so builds that ignore `animation_blocks` still render the animation.
- `template_slot` is ignored by renderers and older builds; replacing a slot changes only `asset_id`.
- Partial preview/export fidelity is reported by `GET /v1/video/render/capabilities` and timeline analysis. Optional fields are never used to conceal a visible downgrade.

## Tracks and stacking

Supported track types are `layer`, `video`, `image`, `audio`, `music`, `text`, `caption`, `shape`, and `callout`. `layer` is primary and accepts any clip kind; legacy typed tracks remain valid.

Track array order controls visual stacking: later tracks render above earlier tracks. `z_index` orders clips within a track; spatial `z` changes camera projection, not stack order. Each track stores `locked`, `muted`, `visible`, optional row `height` (32–160), and optional persisted `solo`. If any track is soloed, export audio is limited to solo tracks; visual visibility is still controlled only by `visible`.

## Scenes and cameras

`scenes` is an optional flat macro-range collection:

```json
{
  "id": "scene-1", "name": "Opening", "start_ms": 0, "duration_ms": 5000,
  "camera": {
    "x": 0, "y": 0, "z": 0,
    "rotation_x": 0, "rotation_y": 0, "rotation_z": 0,
    "field_of_view": 50, "focus_depth": 0, "keyframes": []
  },
  "effects": [], "metadata": {}
}
```

Scene starts are non-negative, durations are positive, ranges cannot overlap or exceed the document, and IDs are unique. Camera keyframes are scene-relative and have their own allowlist: `x`, `y`, `z`, `rotation_x`, `rotation_y`, `rotation_z`, `field_of_view`, `focus_depth`.

## Clips

Important clip fields:

- `asset_id`: optional for generated text, shapes, and unfilled template slots.
- `start_ms`, `duration_ms`: output placement and length.
- `trim_in_ms`, `trim_out_ms`: source-media window. `playback_rate` is 0.25×–4× and validation canonicalizes `trim_out_ms = trim_in_ms + duration_ms × playback_rate`.
- `z_index`, `group_id`, `muted`, and `audio_only`: ordering, grouping, and audio controls.
- `template_slot`: persisted replaceable role such as `Logo`, `Hero Image`, or `Screenshot 1`. Empty non-text slots are validation issues in the editor/agent inspection surface.
- `metadata`: templates use `required_asset_kind` for slot guidance.
- `transform`: `x`, `y`, `z`, `scale`, `scale_x`, `scale_y`, `rotation`, `rotation_x`, `rotation_y`, `rotation_z`, `anchor_x`, `anchor_y`, `perspective`, `opacity`, and fractional `crop` sides (0–0.95). Legacy `scale`/`rotation` remain the fallback.
- `text`, `shape`, and `cursor`: styled overlay, annotation, and captured cursor data. Cursor paths/click rings export through sampled overlays; click audio is not synthesized.
- `transitions`: `fade`, `crossfade`, `dip_to_black`, `slide`, `wipe`, `zoom` with positive duration.

Allowed clip and scene effects are:

```text
blur, brightness, contrast, saturation, grayscale, shadow,
background_blur, chroma_key, sharpen, vignette,
film_grain, bloom, color_grade, edge_fade, rgb_split,
ghost_trail, motion_blur, depth_of_field, rack_focus
```

Effect parameters are bounded by the authoring registry; the backend rejects non-finite numeric values. Capability metadata identifies unsupported or approximated filters. Scene effects are timeline-gated after the composited frame.

## Keyframes and motion curves

Clip keyframes are clip-relative scalar samples. Allowed base properties are:

```text
x, y, z, scale, scale_x, scale_y,
rotation, rotation_x, rotation_y, rotation_z,
opacity, volume
```

For an effect already present on the clip, `effect.<effect-id>.amount` and `effect.<effect-type>.amount` are also valid. This makes animated effect amount authorable without opening arbitrary dotted properties.

Legacy easing values are `linear`, `ease-in`, `ease-out`, `ease-in-out`, and `step`. The additive curve sibling is:

```json
{ "type": "bezier", "x1": 0.42, "y1": 0, "x2": 0.58, "y2": 1 }
{ "type": "spring", "stiffness": 170, "damping": 18, "mass": 1 }
```

Bezier and spring curves are deterministically sampled in preview and the FFmpeg fidelity expander. Springs are segment-local with zero incoming velocity at each keyframe boundary.

## Semantic animation blocks

`animation_blocks` records enough provenance to replace, edit, remove, and undo generated motion without deleting manual keyframes:

```json
{
  "id": "animation-block-1", "block_key": "pop", "family": "in",
  "start_ms": 0, "duration_ms": 650, "delay_ms": 0,
  "params": { "overshoot": 1.06 },
  "generated_keyframe_ids": ["keyframe-a", "keyframe-b", "keyframe-c"]
}
```

Block ranges must fit within the clip and every generated ID must reference a keyframe on that clip. The single frontend registry absorbs the legacy motion presets; applying one block is one undo/save mutation.

## Validation summary

The backend validates schema version, canvas defaults, unique IDs, finite values, clip/source timing, scene overlap/bounds, camera and clip property allowlists, effect/transition types, curve parameters, animation-block provenance, and edit-plan operations. Unknown effects, transitions, keyframe properties, and invalid scene ranges reject the document rather than being silently dropped.
