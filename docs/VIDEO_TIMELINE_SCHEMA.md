# Video Timeline Schema

Video timelines persist a renderer-neutral OmniLLM JSON document.

```json
{
  "version": 1,
  "canvas": {
    "width": 1920,
    "height": 1080,
    "fps": 30,
    "background": "#000000"
  },
  "duration_ms": 30000,
  "tracks": [],
  "markers": [],
  "metadata": {}
}
```

The current document version is **1** (`CurrentTimelineVersion` in `backend/internal/video/timeline.go`). All recent schema additions are optional fields, so older v1 documents remain valid without migration. Documents with a future version fail with an actionable error; `UpgradeTimelineDocument` is the entry point for stepwise upgrades when a breaking version lands.

## Tracks

Supported track types:

```text
layer, video, image, audio, music, text, caption, shape, callout
```

**`layer` is the primary track type**: a generic ordered layer that accepts any
clip kind. Media behavior (visual output, audio contribution, default duration,
default transform/volume) comes from the clip's asset kind and MIME type, not
the track type. `NewEmptyTimeline` creates four generic layers (`Layer 1`–`Layer 4`).

The legacy typed tracks (`video`, `image`, …) remain valid and loadable. When an
asset is added without an explicit `track_id`, legacy typed tracks are still
preferred for their matching media kind; an explicit `track_id` accepts any
asset kind on any unlocked track of any type.

**Stacking order:** track array order controls visual compositing — **later
tracks render on top** of earlier ones, in both the preview compositor and the
FFmpeg export. The editor displays the track list reversed (foreground layer at
the top of the list, Camtasia/Premiere style); array index 0 is the background.
Within a track, `z_index` then start time break ties.

Each track stores lock, mute, visibility, optional `height` (UI row height in
px, clamped 32–160), and clip state. `visible: false` suppresses the track's
visual output only; `muted: true` suppresses its audio contribution only.
Solo is an ephemeral preview-side monitoring control and is **not** persisted
in the document.

## Markers

Markers (`id`, `time_ms`, `label`) are normalized on validation: IDs are auto-generated, negative times clamp to 0, labels are trimmed, and markers are sorted by time.

## Clips

Clips include timing, asset references, transforms, fades, effects, transitions, text payloads, and keyframes.

Important fields:

- `asset_id`: optional for generated text/callout clips.
- `start_ms`, `duration_ms`: timeline placement.
- `trim_in_ms`, `trim_out_ms`: source trim window.
- `z_index`: optional compositing order among overlapping clips (default 0).
- `group_id`: optional grouping handle for multi-select operations.
- `transform`: x, y, scale, rotation, opacity, and optional fractional crop (`{top, right, bottom, left}`, each 0–0.95).
- `text`: text payload and styling — `font_family`, `font_size`, `font_weight`, `color`, `background`, `stroke`, `stroke_width`, `shadow`, `text_align`, `line_height`, `letter_spacing`, `border_radius`. Some styling fields are preview-only; export support is reported by `GET /v1/video/render/capabilities`.
- `effects`: ordered effect definitions. Allowed types: `blur`, `brightness`, `contrast`, `saturation`, `grayscale`, `shadow`, `background_blur`, `chroma_key`, `sharpen`, `vignette`.
- `transitions`: clip transition definitions. Allowed types: `fade`, `crossfade`, `dip_to_black`, `slide`, `wipe`, `zoom`. Duration must be positive; IDs are unique per clip.
- `keyframes`: animated scalar values over time. Allowed properties: `x`, `y`, `scale`, `rotation`, `opacity`, `volume`. Easing: `linear`, `ease-in`, `ease-out`, `ease-in-out`, `step` (unknown easings normalize to `linear`). **`time_ms` is clip-relative** (measured from the clip start). The preview canvas animates keyframed `x`/`y`/`scale`/`rotation`/`opacity` (interpolation in `frontend/src/components/video/effects/keyframeUtils.ts`); FFmpeg export applies `x`/`y` position keyframes with linear interpolation, the rest are preview-only (see `GET /v1/video/render/capabilities`).

The backend validates version, canvas defaults, unique track/clip/marker IDs, clip durations, trim values, effect/transition/keyframe types, per-clip effect/transition/keyframe ID uniqueness, and operation types before saving or applying AI edit plans. Unknown effect/transition types and unknown keyframe properties are rejected.
