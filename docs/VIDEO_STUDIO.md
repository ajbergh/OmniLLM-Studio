# Video Studio

Video Studio is a first-class project workspace for AI video creation. Timeline composition, multi-asset editing, assistant edit planning, and render/export live in the separate **Video Edit Studio** workspace.

## Video Studio Capabilities

- Project-based video workspace gated by the `video_studio` feature flag.
- Real provider adapters for OpenRouter Video and direct Gemini Veo 3.1.
- Provider/model discovery — Gemini performs live discovery against `/v1beta/models` and falls back to a built-in snapshot when the API is unavailable or unconfigured.
- Prompt enhancement, generation history, branching metadata, and durable assets.
- Gemini Veo supports **reference image input** (image-to-video): supply an image asset ID via `ReferenceAssetIDs` and the Gemini adapter base64-encodes the first image and embeds it into the `predictLongRunning` request.
- Single-output preview and download controls for generated video, image, and audio outputs.
- **Cross-studio shortcuts** — Image Studio and Music Studio each expose a "Make Video" button that routes the asset (with generated prompt context) into Video Studio via the crossover domain-translation path. Music Studio additionally has an **"Add to Video Project"** button on completed tracks that imports the audio asset directly into the active video project's media bin (creating a project when none is active) via `POST /v1/video/projects/{id}/assets/import` with `source_studio: "music"` — ready to drag onto a music/audio track in Video Edit Studio.

## Creation Panel UI

The creation panel is organized into individually collapsible sections so you can focus on what matters:

| Section | Default | Contents |
|---------|---------|----------|
| **Prompt** | Open | Main prompt textarea + negative prompt (when supported) |
| **Start / Last Frame** | Open | Start frame image, last frame image (Veo interpolation), source video to extend — shown only when the selected model supports these capabilities |
| **Reference Images** | Closed | Up to 3 reference images for style/subject guidance — shown only for models with `reference_images` capability |
| **Output Format** | Open | Aspect ratio, duration, resolution, FPS |
| **Cinematic Controls** | Closed | Full cinematic detail dropdowns (see below) |
| **Advanced** | Closed | Person generation and future tunables |

## Cinematic Controls

The **Cinematic Controls** section provides Music-Studio-style dropdown selectors for every major creative dimension. Each control offers preset choices plus a **Custom…** free-text entry option.

| Control | Presets | Example custom |
|---------|---------|----------------|
| **Style** | cinematic, documentary, animation, hyperrealistic, vintage film, sci-fi, horror, fantasy, noir, nature documentary, vlog, commercial, abstract | `impressionist painting` |
| **Camera motion** | static locked-off, slow push-in, slow pull-out, dolly forward/backward, dolly zoom, pan left/right, tilt up/down, orbit/arc, tracking follow, crane up/down, handheld shake, whip pan, dutch tilt, drone aerial | `rotating crane` |
| **Shot type** | ECU through extreme wide shot, OTS, POV, high angle, low angle, bird's eye, worm's eye | `two-shot` |
| **Composition** | rule of thirds, centered symmetry, leading lines, negative space, framing, golden ratio, diagonal, layered depth, overhead flat lay | `dynamic diagonal` |
| **Lens / focus** | standard 50mm, wide angle, telephoto, macro, fisheye, tilt-shift, anamorphic, shallow DOF bokeh, deep focus, rack focus | `anamorphic streak` |
| **Lighting / ambiance** | golden hour, blue hour, midday sun, overcast diffuse, studio three-point, neon night, candlelight, silhouette backlit, fog/mist, underwater caustics, fire glow, moonlight | `hard rim backlight` |
| **Dialogue** (audio models) | Free text — wrap dialogue in quotes: `"Hello," she whispered.` | |
| **Sound effects** (audio models) | footsteps on gravel, rain on glass, crowd murmur, door creak, explosion, thunder crack, city traffic, fire crackling, wind, ocean waves, chirping birds, keyboard typing | `thunder crack` |
| **Ambient noise** (audio models) | forest ambience, urban street, coffee shop, empty concert hall, underwater, wind in trees, night crickets, industrial factory, distant traffic hum | `distant traffic hum` |
| **Continuity notes** | Free text — shown when the model supports image-to-video, first/last frame, or extend. E.g. `Maintain character outfit, match exit direction, seamless loop…` | |
| **Production notes** | Free text — additional directives appended to the assembled prompt. | |

All cinematic controls are assembled into the final prompt at generation time — nothing is sent as a separate API parameter. Controls left empty (or set to **Auto**) are omitted.

## Asset Pickers — Thumbnails & Local Upload

Every image/video asset selector in the creation panel includes:

- **Inline thumbnail** — once an asset is selected, its preview renders below the dropdown (images as `max-h-32`, video as a `max-h-20` muted poster frame).
- **Local file upload (`+` button)** — click the `+` button next to the dropdown to open a native file picker scoped to the appropriate type (`image/*` or `video/*`). The file is uploaded to the project via `POST /v1/video/projects/{projectId}/assets/upload`, automatically selected, and the asset list updates immediately — no page refresh required.

The upload button is available for Start frame, Last frame, Source video to extend, and all three Reference image slots.


## Video Edit Studio Capabilities

- Neutral OmniLLM timeline JSON built on **generic ordered layers** (`layer` track type) that accept any clip kind — media behavior comes from the asset, not the track. Legacy typed tracks (video, image, audio, music, text, caption, shape, callout) remain loadable. The layer list displays foreground-on-top; later tracks in the document stack above earlier ones in preview and export.
- Layer-based timeline: clip move (group-aware), trim, split (playhead, blade tool, or split-all), duplicate, delete, copy/cut/paste (Ctrl+C/X/V, paste-at-playhead or paste-here), copy/paste clip attributes, multi-select (modifiers, Ctrl+A, before/after playhead, all-on-layer), grouping, align/distribute, markers, snapping with drop guides, undo/redo, zoom (buttons, `+`/`-`/`F`, Ctrl+wheel), layer add/remove/rename/reorder/duplicate/clear/solo (preview audio), sticky headers, drag-to-resize row height, and a `?` shortcut overlay.
- **Context menus everywhere** via a shared accessible component (`frontend/src/components/common/ContextMenu.tsx`, portal-based, keyboard-navigable, Shift+F10): timeline clips, layer headers, empty lanes (time-aware paste/insert), the ruler (markers, split-all, duration, selection), the preview canvas (fit/fill/center/reset, z-order, select-underneath), media-bin assets (add/rename/download/send-to-chat/register-in-library/delete with in-use warnings), inspector effect/transition/keyframe rows, and render jobs (copy error/diagnostics, retry, cancel).
- Preview canvas composites **all visible tracks** at the playhead (track order + z-index) with transforms, opacity, fades, crop, CSS-filter effects, keyframe animation, and styled text; on-canvas drag/scale/rotate/crop editing with snap-to-center, safe-area/grid guides, and a fullscreen toggle.
- Inspector with timecode-capable timing fields (start/end/duration/trim-in), numeric X/Y + fit/fill/center/reset transform actions, text styling controls (size, weight, color, background, alignment, shadow, title/lower-third/subtitle presets), registry-driven effect/transition/keyframe editors (param sliders, reorder, per-type export-support chips), and a project Canvas section (presets, dimensions, FPS, background) when nothing is selected.
- Captions panel: segment editor (retime/split/merge/delete), SRT/WebVTT import/export, and style presets — captions are ordinary clips on caption tracks.
- Media bin: search, sort, type filters, grid/list views, server-generated poster thumbnails and audio waveform images (FFmpeg, best-effort, served from `GET /v1/video/assets/{assetId}/artifacts/{thumbnail|waveform}`), in-use badges, rename/delete, button or drag-and-drop local upload (uploads are ffprobe-enriched with duration/dimensions/FPS/codec/audio details when ffprobe is installed), and drag-to-timeline. Audio clips show their waveform in the timeline.
- Starter templates (16:9/9:16/1:1, title + lower third, captioned talking head, slideshow) that create real timeline JSON via the header Templates menu.
- Audio workflows: per-clip volume with exportable volume keyframes, clip mute (independent of volume), detach-audio-from-video (audio-only twin on a new layer), one-click full-length music beds from audio assets, ephemeral layer solo for preview monitoring, and waveforms on timeline clips.
- FFmpeg-backed render/export jobs that composite real video and image media alongside text/caption/callout clips into durable MP4/WebM export assets — layer-ordered stacking, slide transitions, chroma key, and audio mixdown that includes video-clip soundtracks.
- **AI-backed assistant** — storyboard and edit-plan endpoints call the LLM (using the first enabled chat provider) when configured; deterministic fallbacks are used when no LLM is available. Social-variant, timeline-plan, apply-plan, and validate-plan endpoints remain rule-based.
- Crossover translation support for image, music, chat, and video domains.
- Backend asset import that copies real bytes from File Library records, Music Studio assets, and Image/attachment-backed sources into Video Studio storage while preserving source metadata.
- **Video-to-Chat** — `POST /v1/video/assets/{assetId}/attach-to-conversation` copies a video asset into a conversation as an attachment, sends it to the chat view, and navigates there.
- **Register in File Library** — `POST /v1/video/assets/{assetId}/register-in-library` ingests a video asset into the global File Library scope so it is available for RAG retrieval and library search across all conversations.

## Provider Requirement

Video generation requires a configured OpenRouter or Gemini provider profile with an API key. There is no local mock provider fallback; if neither profile is configured, the frontend keeps generation disabled and prompts for provider configuration.

## Real Providers

OpenRouter and Gemini use encrypted provider profiles from Settings:

- **OpenRouter**: defaults to `https://openrouter.ai/api/v1`, discovers current video models through `/videos/models`, submits jobs through `/videos`, polls the returned URL, and downloads completed `unsigned_urls`.
- **Gemini**: defaults to `https://generativelanguage.googleapis.com/v1beta`, uses direct Veo `predictLongRunning`, polls long-running operations, and downloads generated sample video URIs. When a reference image is supplied, it is embedded as `instance["image"]` with base64-encoded bytes and detected MIME type.

Both providers include a built-in model snapshot so the UI shows expected capabilities before credentials are configured or when model discovery is temporarily unavailable.

## Rendering

Video Edit Studio exports through persisted backend render jobs. The default renderer uses FFmpeg to composite real video and image media, text/caption/callout clips, and canvas settings into durable MP4/WebM export assets — including transforms (position/scale/rotation/crop), opacity, fades, fade-style transitions, most effects, position keyframes, audio mixing, and text styling. Remaining gaps (e.g. scale/opacity keyframes, slide/wipe/zoom transitions) are reported by `GET /v1/video/render/capabilities`, which drives the inspector's export-fidelity warnings.

See [VIDEO_RENDERING.md](VIDEO_RENDERING.md) for the full renderer reference.

## Cross-Studio Imports

`POST /v1/video/projects/{projectId}/assets/import` accepts File Library, Music Studio, Image Studio, and attachment-backed source IDs. The service resolves the original stored file, checks project/source ownership where the source model supports it, copies the bytes into `<attachments_dir>/video/...`, and stores a `VideoAsset` with `source_studio` and `source_id` metadata.

`POST /v1/video/projects/{projectId}/assets/upload` accepts a raw `multipart/form-data` file upload (field name `file`; per-kind limits: image 25 MB, audio 100 MB, video 500 MB). The backend sniffs the MIME type from the file content (rejecting kind mismatches), derives `kind` (`image`, `video`, `audio`), saves the file under a UUID filename in video storage, enriches video/audio assets with duration/dimensions/FPS via `ffprobe` when available, and creates a `VideoAsset` record with `source_type = "upload"`. This endpoint powers the creation-panel asset pickers' `+` buttons and the Edit Studio media bin's upload button and drag-and-drop.

## Cross-Studio Exports

Video assets can be pushed out of the video project to other parts of the app:

| Action | Route | Behaviour |
|--------|-------|-----------|
| **Send to Chat** | `POST /v1/video/assets/{assetId}/attach-to-conversation` | Copies the asset file, creates an `Attachment` in the target conversation, and navigates the frontend to that conversation |
| **Register in Library** | `POST /v1/video/assets/{assetId}/register-in-library` | Ingests the asset into the global File Library scope via `filelibrary.IngestFile`; available for RAG retrieval and library search |

Both actions are accessible from video project asset cards in Video Edit Studio.

## AI Assistant

The assistant endpoints are:

| Endpoint | Behaviour |
|----------|-----------|
| `POST /v1/video/projects/{projectId}/assistant/storyboard` | Generates a multi-scene storyboard. Calls the LLM with a structured JSON schema prompt; falls back to a deterministic 3-scene template if no LLM is available. |
| `POST /v1/video/projects/{projectId}/assistant/plan` | Returns a new empty timeline plan. |
| `POST /v1/video/projects/{projectId}/assistant/edit-plan` | Generates timeline edit operations from a natural-language instruction. Calls the LLM to produce typed `EditOperation` objects; falls back to keyword matching. |
| `POST /v1/video/projects/{projectId}/assistant/apply-plan` | Applies a validated `EditPlan` to the live timeline. |
| `POST /v1/video/projects/{projectId}/assistant/validate-plan` | Validates edit operations against the timeline without applying them. |
| `POST /v1/video/projects/{projectId}/assistant/social-variants` | Generates aspect-ratio variants from a storyboard. Rule-based. |

## Feature Flag

Video Studio is enabled by migration with:

```text
video_studio
```

The frontend gates both Video Studio and Video Edit Studio sidebar visibility through the same feature-flag path used by Music Studio.
