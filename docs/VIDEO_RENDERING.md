# Video Rendering

Video Edit Studio separates interactive timeline preview from final export. Video Studio remains focused on AI video creation and selected-output playback.

## Preview

The frontend preview uses timeline state, browser rendering, and project assets to show the active visual clip at the current playhead. It is intended for responsive editing, not frame-perfect export.

Asset display in the preview canvas:

| Asset type | Rendered as |
|------------|-------------|
| `video/*` MIME | Native `<video>` element with `src` pointing to the download URL |
| `image/*` MIME | `<img>` element |
| Text/caption clip | Inline styled text div |
| `text/plain` asset | Compact text asset card |
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
- **Per-clip transform** — `x`/`y` position offset, `scale`, and fractional `crop` (`{top, right, bottom, left}`, 0–0.95 each).
- **Opacity** — applied via `colorchannelmixer`.
- **Fades** — video fade in/out as alpha fades; audio fade in/out via `afade`.
- **Transitions (fade-style)** — `fade`, `crossfade`, and `dip_to_black` are rendered as alpha fades.
- **Effects** — `brightness`, `contrast`, `saturation`, `blur`, and `grayscale` map to FFmpeg filters.
- **Audio/music mixing** — per-clip `volume`, timeline placement via `adelay`, and multi-track `amix` mixdown.
- **Track semantics** — hidden tracks drop their video; muted tracks drop their audio.
- Text/caption/callout clips with timing, font size, text color, optional background box, stroke, and shadow.
- Render diagnostics — the FFmpeg command (and stderr on failure) is persisted in `video_render_jobs.metadata_json`.
- Clear render failure if `ffmpeg` is unavailable or returns an encoding error.

### Not yet rendered in export

The following are stored in the timeline JSON and shown in the editor but are **not yet applied by the FFmpeg renderer**:

- Keyframe animation
- Rotation
- `slide`/`wipe`/`zoom` transitions (true `xfade` directional transitions)
- `chroma_key`, `shadow`, and `background_blur` effects
- Track solo (not yet in the timeline schema)

Renderer support is reported by `GET /v1/video/render/capabilities` (see `backend/internal/video/renderer_capabilities.go`). The inspector and render panel derive their export-fidelity warnings from that endpoint, so warnings stay accurate as renderer support evolves.

Render/export uses `NewFFmpegRenderer("")` by default. There is no package-local mock renderer.

## Production Adapter Direction

A higher-fidelity adapter can still implement `video.Renderer` and delegate to:

- Remotion through a Node render worker.
- A richer FFmpeg timeline composition graph.
- A provider-hosted render API.

The adapter should preserve the neutral OmniLLM timeline as the saved source of truth and treat renderer-specific manifests as derived artifacts.
