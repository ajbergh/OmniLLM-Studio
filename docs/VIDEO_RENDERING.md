# Video Rendering

Video Studio separates interactive preview from final export.

## Preview

The frontend preview uses timeline state, browser rendering, and project assets to show the active visual clip at the current playhead. It is intended for responsive editing, not frame-perfect export.

Asset display in the preview canvas:

| Asset type | Rendered as |
|------------|-------------|
| `video/*` MIME | Native `<video>` element with `src` pointing to the download URL |
| `image/*` MIME | `<img>` element |
| Text/caption clip | Inline styled text div |
| `text/plain` (mock dev asset) | Labelled amber development-placeholder card |
| Any other asset | Generic grey placeholder |

## Export Jobs

Backend render jobs are persisted in `video_render_jobs` and exposed through:

```text
POST /v1/video/projects/{projectId}/render
GET  /v1/video/render-jobs/{jobId}
POST /v1/video/render-jobs/{jobId}/cancel
```

Render outputs become `VideoAsset` rows with `kind = "export"`.

## FFmpeg Renderer

The default renderer is FFmpeg-backed. It creates real MP4/WebM bytes from the neutral timeline canvas, compositing real video and image assets alongside text, caption, and callout clips. Outputs are stored as `VideoAsset` rows with `kind = "export"`.

Current FFmpeg export coverage:

- Canvas size, background color, duration, FPS, format, quality, and optional silent audio.
- **Video asset compositing** — video clips with `asset_id` pointing to a real video file are overlaid on the canvas at the correct start/duration using FFmpeg `-itsoffset` and `overlay` filter graph entries.
- **Image asset compositing** — image clips are overlaid on the canvas at the correct start/duration using FFmpeg `overlay` filters.
- Text/caption/callout clips with timing, font size, text color, optional background box, stroke, and shadow.
- Clear render failure if `ffmpeg` is unavailable or returns an encoding error.

### Not yet rendered in export

The following are stored in the timeline JSON and shown in the editor but are **not yet applied by the FFmpeg renderer**:

- Effects (blur, brightness, contrast, saturation, etc.)
- Transitions (cross-fade, wipe, etc.)
- Opacity/fade keyframes
- Audio/music clip mixing
- Per-clip transform (scale, rotation, crop) beyond canvas placement

The inspector panel shows an inline warning when any of these are present on a selected clip, so it is clear that they will not appear in the export.

The `MockRenderer` still exists as a package-local development/test adapter, but it is no longer the default service renderer.

## Production Adapter Direction

A higher-fidelity adapter can still implement `video.Renderer` and delegate to:

- Remotion through a Node render worker.
- A richer FFmpeg timeline composition graph.
- A provider-hosted render API.

The adapter should preserve the neutral OmniLLM timeline as the saved source of truth and treat renderer-specific manifests as derived artifacts.
