# Video Rendering

Video Studio separates interactive preview from final export.

## Preview

The frontend preview uses timeline state, browser rendering, and project assets to show the active visual clip at the current playhead. It is intended for responsive editing, not frame-perfect export.

## Export Jobs

Backend render jobs are persisted in `video_render_jobs` and exposed through:

```text
POST /v1/video/projects/{projectId}/render
GET  /v1/video/render-jobs/{jobId}
POST /v1/video/render-jobs/{jobId}/cancel
```

Render outputs become `VideoAsset` rows with `kind = "export"`.

## FFmpeg Renderer

The default renderer is FFmpeg-backed. It creates real MP4/WebM bytes from the neutral timeline canvas and visible text, caption, and callout clips. Outputs are stored as `VideoAsset` rows with `kind = "export"`.

Current FFmpeg export coverage:

- Canvas size, background color, duration, FPS, format, quality, and optional silent audio.
- Text/caption/callout clips with timing, font size, text color, optional background box, stroke, and shadow.
- Clear render failure if `ffmpeg` is unavailable or returns an encoding error.

The `MockRenderer` still exists as a package-local development/test adapter, but it is no longer the default service renderer.

## Production Adapter Direction

A higher-fidelity adapter can still implement `video.Renderer` and delegate to:

- Remotion through a Node render worker.
- A richer FFmpeg timeline composition graph.
- A provider-hosted render API.

The adapter should preserve the neutral OmniLLM timeline as the saved source of truth and treat renderer-specific manifests as derived artifacts.

Remaining render fidelity work:

- Composite generated/imported video and image assets into the exported timeline.
- Mix audio/music clips instead of silent audio.
- Render effect, transition, fade, and keyframe animation parity with the editor model.
