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
video, image, audio, music, text, caption, shape, callout
```

Each track stores lock, mute, visibility, optional `height` (UI row height in px, clamped 32–160), and clip state. Track order is the array order.

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
- `keyframes`: animated scalar values over time. Allowed properties: `x`, `y`, `scale`, `rotation`, `opacity`, `volume`. Easing: `linear`, `ease-in`, `ease-out`, `ease-in-out`, `step` (unknown easings normalize to `linear`). Keyframe time semantics (clip-relative vs timeline-absolute) will be fixed when keyframe rendering lands; today they are stored but not rendered.

The backend validates version, canvas defaults, unique track/clip/marker IDs, clip durations, trim values, effect/transition/keyframe types, per-clip effect/transition/keyframe ID uniqueness, and operation types before saving or applying AI edit plans. Unknown effect/transition types and unknown keyframe properties are rejected.
