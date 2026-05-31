# Video Studio

Video Studio is a first-class project workspace for AI video generation, timeline editing, and export.

## Current Capabilities

- Project-based video workspace gated by the `video_studio` feature flag.
- Mock provider for local development without paid video API keys.
- Real provider adapters for OpenRouter Video and direct Gemini Veo 3.1.
- Provider/model discovery — Gemini performs live discovery against `/v1beta/models` and falls back to a built-in snapshot when the API is unavailable or unconfigured.
- Prompt enhancement, generation history, branching metadata, and durable assets.
- Gemini Veo supports **reference image input** (image-to-video): supply an image asset ID via `ReferenceAssetIDs` and the Gemini adapter base64-encodes the first image and embeds it into the `predictLongRunning` request.
- Neutral OmniLLM timeline JSON with video, image, audio, music, text, caption, shape, and callout track types.
- Asset placement, clip move, trim, split, delete, duplicate, fades, volume, transforms, effects, transitions, text clips, and keyframes.
- Preview canvas shows real `<video>` elements for video assets, `<img>` for image assets, and a labelled amber development-placeholder card for mock (`text/plain`) assets.
- FFmpeg-backed render/export jobs that composite real video and image media alongside text/caption/callout clips into durable MP4/WebM export assets.
- **AI-backed assistant** — storyboard and edit-plan endpoints call the LLM (using the first enabled chat provider) when configured; deterministic fallbacks are used when no LLM is available. Social-variant, timeline-plan, apply-plan, and validate-plan endpoints remain rule-based.
- Crossover translation support for image, music, chat, and video domains.
- **Cross-studio shortcuts** — Image Studio and Music Studio each expose a "Make Video" button that routes the asset (with generated prompt context) into Video Studio via the crossover domain-translation path.
- Backend asset import that copies real bytes from File Library records, Music Studio assets, and Image/attachment-backed sources into Video Studio storage while preserving source metadata.
- **Video-to-Chat** — `POST /v1/video/assets/{assetId}/attach-to-conversation` copies a video asset into a conversation as an attachment, sends it to the chat view, and navigates there.
- **Register in File Library** — `POST /v1/video/assets/{assetId}/register-in-library` ingests a video asset into the global File Library scope so it is available for RAG retrieval and library search across all conversations.

## Development Provider

The built-in `mock` provider exposes `mock-video-v1` and writes deterministic placeholder assets under:

```text
<attachments_dir>/video/<project_id>/<generation_or_job_id>/
```

Mock assets have MIME type `text/plain`. The preview canvas shows a labelled amber card for these, distinct from the normal black "No active visual clip" state.

## Real Providers

OpenRouter and Gemini use encrypted provider profiles from Settings:

- **OpenRouter**: defaults to `https://openrouter.ai/api/v1`, discovers current video models through `/videos/models`, submits jobs through `/videos`, polls the returned URL, and downloads completed `unsigned_urls`.
- **Gemini**: defaults to `https://generativelanguage.googleapis.com/v1beta`, uses direct Veo `predictLongRunning`, polls long-running operations, and downloads generated sample video URIs. When a reference image is supplied, it is embedded as `instance["image"]` with base64-encoded bytes and detected MIME type.

Both providers include a built-in model snapshot so the UI shows expected capabilities before credentials are configured or when model discovery is temporarily unavailable.

## Rendering

Video Studio exports through persisted backend render jobs. The default renderer uses FFmpeg to composite real video and image media, text/caption/callout clips, and canvas settings into durable MP4/WebM export assets. Effects, transitions, fades, opacity keyframes, and audio mixing are stored in the timeline JSON but are not yet applied during FFmpeg export — the inspector shows an inline warning for this.

See [VIDEO_RENDERING.md](VIDEO_RENDERING.md) for the full renderer reference.

## Cross-Studio Imports

`POST /v1/video/projects/{projectId}/assets/import` accepts File Library, Music Studio, Image Studio, and attachment-backed source IDs. The service resolves the original stored file, checks project/source ownership where the source model supports it, copies the bytes into `<attachments_dir>/video/...`, and stores a `VideoAsset` with `source_studio` and `source_id` metadata.

## Cross-Studio Exports

Video assets can be pushed out of Video Studio to other parts of the app:

| Action | Route | Behaviour |
|--------|-------|-----------|
| **Send to Chat** | `POST /v1/video/assets/{assetId}/attach-to-conversation` | Copies the asset file, creates an `Attachment` in the target conversation, and navigates the frontend to that conversation |
| **Register in Library** | `POST /v1/video/assets/{assetId}/register-in-library` | Ingests the asset into the global File Library scope via `filelibrary.IngestFile`; available for RAG retrieval and library search |

Both actions are accessible via icon buttons on each asset card in the Video Studio asset bin.

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

The frontend gates sidebar visibility and settings control through the same feature-flag path used by Music Studio.
