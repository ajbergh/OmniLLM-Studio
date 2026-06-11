# Video Rendering

Video Edit Studio separates interactive timeline preview from final export. Video Studio remains focused on AI video creation and selected-output playback.

## Preview

The frontend preview composites **all** active visual clips at the current playhead, stacked by track order (later tracks on top) and per-clip `z_index`, with transforms, fades, effects, and text styling applied. It is intended for responsive editing, not frame-perfect export. Track solo is a preview-only monitoring control (only the soloed track contributes preview audio); exports ignore it.

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
- **Per-clip transform** — `x`/`y` position offset, `scale`, **`rotation`** (via `rotate` with transparent fill), and fractional `crop` (`{top, right, bottom, left}`, 0–0.95 each).
- **Position keyframes** — keyframed `x`/`y` animate via piecewise-linear `overlay` time expressions (keyframe `time_ms` is clip-relative; easing curves are approximated linearly).
- **Volume keyframes** — keyframed `volume` exports as a frame-evaluated `volume` filter expression and overrides the static clip volume.
- **Opacity** — applied via `colorchannelmixer`.
- **Fades** — video fade in/out as alpha fades; audio fade in/out via `afade`.
- **Transitions** — `fade`, `crossfade`, and `dip_to_black` render as alpha fades; `slide` renders as an animated overlay position (enters from the chosen edge, exits the opposite edge).
- **Effects** — `brightness`, `contrast`, `saturation`, `blur`, `grayscale`, `sharpen` (`unsharp`), `vignette`, and `chroma_key` (`chromakey`, default green with `color`/`similarity`/`blend` params) map to FFmpeg filters.
- **Text styling** — font family (fontconfig best match), stroke color + width, line spacing, plus the existing size/color/background box/shadow.
- **Audio/music mixing** — per-clip `volume`, timeline placement via `adelay`, and multi-track `amix` mixdown. Audio from video clips joins the mix when the asset carries an audio stream (`has_audio` recorded at ingest, ffprobe fallback at render).
- **Clip mute & detached audio** — `clip.muted` silences a clip without changing volume; `clip.audio_only` suppresses a video clip's visuals so it acts as detached audio (the editor's "Detach audio" command pairs a muted original with an audio-only twin).
- **Track semantics** — hidden tracks drop their video (their video clips' audio still mixes); muted tracks drop their audio.
- **Layer-order compositing** — visual clips (media and text alike) composite bottom-to-top by track array order, then `z_index`, then start time, matching the preview. Start time controls only when a clip is enabled, never its stacking. Text clips on any visible track (including generic `layer` tracks) interleave into the same compositing chain, so a text clip on a lower layer renders beneath media on a higher layer.
- Text/caption/callout clips with timing, font size, text color, optional background box, stroke, and shadow.
- Render diagnostics — the FFmpeg command (and stderr on failure) is persisted in `video_render_jobs.metadata_json`.
- Clear render failure if `ffmpeg` is unavailable or returns an encoding error.

### Not yet rendered in export

The following are stored in the timeline JSON and shown in the editor but are **not yet applied by the FFmpeg renderer**:

- Keyframes for `scale`, `rotation`, and `opacity` (position and volume keyframes render; easing curves are linearized)
- `wipe`/`zoom` transitions (true `xfade` directional transitions)
- `shadow` and `background_blur` effects
- Text letter spacing, border radius, and alignment (preview-only)
- Track solo (preview-only monitoring control; exports mix all unmuted tracks)
- Chroma key in the preview (CSS cannot key colors — the canvas shows the unkeyed frame; export applies it)

Renderer support is reported by `GET /v1/video/render/capabilities` (see `backend/internal/video/renderer_capabilities.go`). The inspector and render panel derive their export-fidelity warnings from that endpoint, so warnings stay accurate as renderer support evolves.

Render/export uses `NewFFmpegRenderer("")` by default. There is no package-local mock renderer.

## Production Adapter Direction

A higher-fidelity adapter can still implement `video.Renderer` and delegate to:

- Remotion through a Node render worker.
- A richer FFmpeg timeline composition graph.
- A provider-hosted render API.

The adapter should preserve the neutral OmniLLM timeline as the saved source of truth and treat renderer-specific manifests as derived artifacts.
