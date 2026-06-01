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

## Tracks

Supported track types:

```text
video, image, audio, music, text, caption, shape, callout
```

Each track stores lock, mute, visibility, and clip state.

## Clips

Clips include timing, asset references, transforms, fades, effects, transitions, text payloads, and keyframes.

Important fields:

- `asset_id`: optional for generated text/callout clips.
- `start_ms`, `duration_ms`: timeline placement.
- `trim_in_ms`, `trim_out_ms`: source trim window.
- `transform`: x, y, scale, rotation, opacity, and optional crop.
- `effects`: ordered enabled effect definitions.
- `transitions`: clip transition definitions.
- `keyframes`: animated scalar values over time.

The backend validates version, canvas defaults, unique IDs, clip durations, trim values, and operation types before saving or applying AI edit plans.
