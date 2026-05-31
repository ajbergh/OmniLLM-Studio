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

## Mock Renderer

The current `MockRenderer` creates a deterministic text placeholder asset. This keeps the job lifecycle, UI, polling, and storage path testable without requiring Remotion, FFmpeg, or GPU/video tooling.

## Production Adapter Direction

A production adapter should implement `video.Renderer` and can delegate to:

- Remotion through a Node render worker.
- FFmpeg for timeline composition.
- A provider-hosted render API.

The adapter should preserve the neutral OmniLLM timeline as the saved source of truth and treat renderer-specific manifests as derived artifacts.
