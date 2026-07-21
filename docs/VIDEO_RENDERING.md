# Video Rendering

Video Edit Studio separates responsive interactive preview from final FFmpeg export. Video Studio remains focused on AI video creation and selected-output playback, while Video Edit Studio owns timeline composition, recording, transcription, audio finishing, durable rendering, and export.

## Preview

The frontend preview composites active visual clips at the current playhead, stacked by track order (later tracks on top) and per-clip `z_index`, with transforms, fades, effects, annotations, cursor overlays, and text styling applied. It uses muted visual `<video>` elements plus managed audio elements, so video soundtracks and audio assets both audition clip volume, volume keyframes, fades, track solo, constant 0.25×–4× speed, and the preview-only master volume.

Preview is optimized for editing responsiveness rather than frame-perfect export. Track solo and master volume are monitoring controls; exports ignore both.

Large projects use an interval index for active-clip lookup and a configurable decoder budget. Active videos outside the decoder budget display generated poster/thumbnail frames instead of mounting another decoder. The selected video is promoted into the budget so direct manipulation stays interactive. The local-storage key `omnillm-video-decoder-budget` accepts 1–12 mounted video decoders and defaults to 4.

Asset display in the preview canvas:

| Asset type | Rendered as |
|------------|-------------|
| `video/*` MIME within the decoder budget | Native muted `<video>` element with soundtrack routed through managed audio |
| `video/*` MIME outside the decoder budget | Generated thumbnail/poster frame |
| `image/*` MIME | `<img>` element |
| Text/caption clip | Inline styled text |
| Shape, annotation, or cursor data | Preview overlay |
| `audio/*` MIME or `audio_only` video clip | No visual element; managed hidden audio |
| Other asset without visual timeline data | No visual output |

Timeline rows are horizontally virtualized from the visible scroll window with overscan. Undo/redo uses reversible patch commands with a bounded memory budget instead of retaining an unbounded sequence of full timeline documents. `omnillm-video-history-budget-mb` defaults to 32 MiB and is clamped to 8–256 MiB. Playback UI updates are coalesced to 30 Hz by default through `omnillm-video-playback-ui-hz`, while media elements continue native playback.

## Export Jobs

Backend render jobs are persisted in `video_render_jobs` and exposed through:

```text
POST /v1/video/projects/{projectId}/render
GET    /v1/video/render-jobs/{jobId}
POST   /v1/video/render-jobs/{jobId}/cancel
DELETE /v1/video/render-jobs/{jobId}
```

Deleting a render job removes only a terminal job record; cancel active work first. The output export asset is an independent `VideoAsset` row with `kind = "export"` and survives job deletion.

The production renderer wraps the FFmpeg renderer with fidelity expansion and a durable scheduler. The scheduler provides:

- Configurable global concurrency.
- Per-user and per-workspace admission limits.
- Priority-aware queued-job ordering with FIFO order inside one priority.
- Queued and active cancellation.
- Startup recovery for interrupted persisted jobs.
- Progress persistence and stalled-job detection.
- Temporary-disk preflight and stale temporary-file cleanup.
- Graceful application shutdown that cancels active FFmpeg processes and waits for workers.

Scheduler environment variables:

| Variable | Default | Accepted range | Purpose |
|----------|---------|----------------|---------|
| `OMNILLM_VIDEO_RENDER_CONCURRENCY` | `1` | 1–16 | Global simultaneous FFmpeg jobs |
| `OMNILLM_VIDEO_RENDER_PER_USER` | `1` | 1–8 | Simultaneous jobs admitted for one user |
| `OMNILLM_VIDEO_RENDER_PER_WORKSPACE` | `1` | 1–8 | Simultaneous jobs admitted for one workspace |
| `OMNILLM_VIDEO_RENDER_STALL_SECONDS` | `300` | 30–3600 | Cancel a job that emits no progress for this period |
| `OMNILLM_VIDEO_RENDER_MIN_FREE_BYTES` | `536870912` | 0–2^50 | Free-space reserve before estimated temporary render usage |
| `OMNILLM_VIDEO_RENDER_TEMP_MAX_HOURS` | `24` | 1–720 | Age threshold for stale `omnillm-video-render-*` temporary files |

Desktop defaults remain deliberately conservative: one FFmpeg render at a time unless explicitly configured otherwise.

## FFmpeg Renderer

The default renderer creates real MP4/WebM bytes from the neutral timeline canvas, compositing real video and image assets alongside text, captions, callouts, annotations, cursor effects, and mixed audio.

Current export coverage includes:

- Canvas size, background, duration, FPS, format, quality, codec, optional audio, range rendering, caption burn-in, and SRT/VTT sidecar captions.
- Real video and image compositing with timeline placement, trim, layer ordering, `z_index`, scaling, position, fractional crop, rotation, opacity, and fades.
- Constant 0.25×–4× video/audio retiming with pitch-preserving `atempo` filters.
- Position, scale, rotation, opacity, volume, and effect-amount keyframes expanded into deterministic sampled segments. Linear, ease-in, ease-out, ease-in-out, and step interpolation are supported; continuous curves remain approximations.
- Fade, dip, slide, sampled zoom, and directional wipe transitions. Crossfade remains an alpha-fade approximation rather than a true two-input blend.
- Brightness, contrast, saturation, blur, grayscale, sharpen, vignette, chroma key, and sampled effect-amount animation.
- Text/caption/callout styling including font, size, color, line height, stroke, shadow, background, transform, opacity, fades, and deterministic alignment/letter-spacing approximation.
- Rectangle, highlight, pixelate, blur-region, and normalized fallback annotation output. Complex geometry such as ellipse, arrow, and speech bubble currently exports as simpler deterministic primitives.
- Cursor paths, highlights, and click rings through sampled overlays. Click audio is not synthesized.
- Multi-track audio mix with video soundtracks, per-clip volume/mute, volume keyframes, fades, timeline delay, constant speed, and `amix` mixdown.
- Optional final audio processing: denoise, EQ preset, compression, LUFS normalization, limiting, and mono/stereo conversion.
- Render diagnostics persisted in `video_render_jobs.metadata_json` and explicit failures when FFmpeg is unavailable or encoding fails.

### Known preview/export differences

The capability endpoint intentionally reports partial support where output is deterministic but approximate:

- Crop values are source-frame fractions; wipe transitions are sampled crop segments.
- Rounded text-box corners remain preview-only.
- Crossfade is an alpha-fade approximation.
- Drop shadow and background-blur effects remain unsupported by export.
- Keyframe curves are sampled rather than continuously evaluated.
- Complex annotation geometry normalizes to simpler primitives.
- Cursor click audio is not synthesized.
- Track solo is preview-only; exports mix all unmuted tracks.
- Chroma key is applied by FFmpeg but cannot be represented faithfully by the CSS preview.

Renderer support is reported by `GET /v1/video/render/capabilities`. `backend/internal/video/renderer_capabilities.go` is the source of truth used by the inspector, assistant, and render panel. Upgrade a capability only after the corresponding renderer path and tests are complete.

## Transcription and Caption Regeneration

Video Edit Studio includes a versioned, provider-neutral transcription contract with durable transcript and segment persistence. The current OpenAI/OpenAI-compatible adapter supports optional source language, word and segment timestamps, speaker-label requests when the provider supports them, and translation to English.

Remote processing requires explicit user consent. Provider privacy terms and charges still apply; the UI exposes the selected provider profile, retention choice, returned language, speaker availability, and reported cost when available. Interrupted queued/running jobs are marked failed on startup with a retryable message because the neutral contract does not persist a resumable provider operation ID.

Completed transcripts can regenerate ordinary caption clips from persisted segments without retranscribing the source asset.

## Windows Native Capture

The Wails Windows build can record the desktop through FFmpeg `gdigrab`, optionally capture a selected DirectShow microphone or loopback device, and store cursor/click telemetry plus explicitly opted-in virtual-key timing. Typed text is not reconstructed.

DirectShow devices are enumerated and validated again when capture starts. Seamless device reconnect is not implemented: stop the take, reconnect the device, refresh the list, and start a new take. FFmpeg receives a graceful `q` stop first and is forcibly terminated after the shutdown timeout if necessary. Completed captures import only into their originating project, then temporary capture and telemetry files are removed.

## Validation

Renderer reliability is covered by unit tests for fidelity expansion, capability reporting, scheduler limits/priorities/cancellation/stalls/shutdown, audio-processing filters, transcription parsing/recovery, interval indexing, decoder budgeting, and patch history.

`TestRendererGoldenMedia` generates deterministic source video and audio at test time, performs a real FFmpeg round trip, and validates decoded frame composition, dimensions, duration, audio-stream presence, and non-silent samples. It uses semantic pixel/audio thresholds rather than encoded-file hashes because codec bytes can differ across FFmpeg builds.

The Windows CI job compiles and tests the desktop capture contract on a native Windows runner. The standard pull-request gate also runs Go formatting/vet/tests/race detection, frontend lint/unit/build, the complete Chromium Playwright suite, and Helm validation.

## Adapter Boundary

`video.Renderer` remains the neutral adapter boundary. A future renderer may delegate to Remotion, a richer FFmpeg graph, or a provider-hosted render API, but the saved OmniLLM timeline remains the source of truth and renderer-specific manifests remain derived artifacts.
