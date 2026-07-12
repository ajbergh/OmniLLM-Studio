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
- `start_ms`, `duration_ms`: output-timeline placement and duration.
- `trim_in_ms`, `trim_out_ms`: source-media trim window.
- `playback_rate`: optional constant playback rate for time-based media,
  from **0.25× through 4×** (defaults to `1`). The selected source span is
  `duration_ms × playback_rate`; validation canonicalizes `trim_out_ms` from
  that relationship. Images, text, and shapes ignore the field.
- `z_index`: optional compositing order among overlapping clips (default 0).
- `group_id`: optional grouping handle for multi-select operations.
- `muted`: silences the clip's audio contribution without touching `volume`.
- `audio_only`: suppresses a clip's visual output so a video asset acts as
  detached audio (created by the editor's "Detach audio" command).
- `shape`: parameterized callout/annotation box — `kind` (`rectangle` |
  `highlight` | `blur` | `rounded_rectangle` | `ellipse` | `arrow` | `line` |
  `speech_bubble` | `spotlight` | `pixelate` | `checkmark` | `x_mark` |
  `step_marker` | `label`), `width`/`height` in canvas pixels (defaults
  320×180), `fill`, `stroke` + `stroke_width` (clamped 0–100), `blur_radius`
  (blur radius or pixelate block size, clamped 1–50, default 12),
  `corner_radius` (clamped 0–200, preview-only at export). Position/scale/
  opacity come from the clip transform; a clip may carry both a shape and
  `text` (the label draws above its box). Blur regions redact whatever
  composites beneath them; pixelate regions export as a true mosaic.
  Export support per kind: rectangle/highlight/blur/pixelate export fully;
  rounded_rectangle and label export with square corners; the remaining
  annotation kinds are preview-only (see `GET /v1/video/render/capabilities`).
- `cursor`: optional cursor metadata captured with screen recordings —
  `visible`, `scale` (clamped 0.25–4, default 1), `highlight`, `click_rings`,
  `smoothing`, and `events` (sorted samples `{time_ms, x, y, click?}` with
  clip-relative **output-timeline** times and canvas-pixel coordinates from
  the top-left). Retime and split operations scale or rebase the samples so
  the path stays synchronized with its clip. Preview-only: the editor overlays
  a cursor, but exports do not draw it yet. The built-in recorder does not
  currently collect browser cursor coordinates.
- `transform`: x, y, scale, rotation, opacity, and optional fractional crop (`{top, right, bottom, left}`, each 0–0.95).
- `text`: text payload and styling — `font_family`, `font_size`, `font_weight`, `color`, `background`, `stroke`, `stroke_width`, `shadow`, `text_align`, `line_height`, `letter_spacing`, `border_radius`. Some styling fields are preview-only; export support is reported by `GET /v1/video/render/capabilities`.
- `effects`: ordered effect definitions. Allowed types: `blur`, `brightness`, `contrast`, `saturation`, `grayscale`, `shadow`, `background_blur`, `chroma_key`, `sharpen`, `vignette`.
- `transitions`: clip transition definitions. Allowed types: `fade`, `crossfade`, `dip_to_black`, `slide`, `wipe`, `zoom`. Duration must be positive; IDs are unique per clip.
- `keyframes`: animated scalar values over time. Allowed properties: `x`, `y`, `scale`, `rotation`, `opacity`, `volume`. Easing: `linear`, `ease-in`, `ease-out`, `ease-in-out`, `step` (unknown easings normalize to `linear`). **`time_ms` is clip-relative** (measured from the clip start). The preview canvas animates keyframed `x`/`y`/`scale`/`rotation`/`opacity` (interpolation in `frontend/src/components/video/effects/keyframeUtils.ts`); FFmpeg export applies `x`/`y`, `rotation`, and `volume` keyframes with linear interpolation; `scale`/`opacity` keyframes — and easing curves — are preview-only (see `GET /v1/video/render/capabilities`). The editor's motion presets (zoom, pan, Ken Burns) generate ordinary `x`/`y`/`scale` keyframes.
- `metadata`: free-form document metadata. The editor records `default_caption_preset` here (set when a caption style preset is applied) so new and imported captions inherit the project's caption style.

The backend validates version, canvas defaults, unique track/clip/marker IDs,
clip durations, trim values, finite 0.25×–4× playback rates, effect/
transition/keyframe types, per-clip effect/transition/keyframe ID uniqueness,
and operation types before saving or applying AI edit plans. Unknown effect/
transition types and unknown keyframe properties are rejected.
