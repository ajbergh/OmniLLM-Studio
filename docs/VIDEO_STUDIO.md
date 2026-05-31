# Video Studio

Video Studio is a first-class project workspace for AI video generation, timeline editing, and export.

## Current Capabilities

- Project-based video workspace gated by the `video_studio` feature flag.
- Mock provider for local development without paid video API keys.
- Real provider adapters for OpenRouter Video and direct Gemini Veo 3.1.
- Provider/model discovery, prompt enhancement, generation history, branching metadata, and durable assets.
- Neutral OmniLLM timeline JSON with video, image, audio, music, text, caption, shape, and callout track types.
- Asset placement, clip move, trim, split, delete, duplicate, fades, volume, transforms, effects, transitions, text clips, and keyframes.
- Preview canvas driven by browser state.
- Mock render/export jobs that create durable export assets.
- Assistant storyboard, timeline-plan, edit-plan, apply-plan, and social-variant endpoints.
- Crossover translation support for image, music, chat, and video domains.

## Development Provider

The built-in `mock` provider exposes `mock-video-v1` and writes deterministic placeholder assets under:

```text
<attachments_dir>/video/<project_id>/<generation_or_job_id>/
```

## Real Providers

OpenRouter and Gemini use encrypted provider profiles from Settings:

- OpenRouter: defaults to `https://openrouter.ai/api/v1`, discovers current video models through `/videos/models`, submits jobs through `/videos`, polls the returned URL, and downloads completed `unsigned_urls`.
- Gemini: defaults to `https://generativelanguage.googleapis.com/v1beta`, uses direct Veo `predictLongRunning`, polls long-running operations, and downloads generated sample video URIs.

The model list still includes a built-in snapshot when credentials are not configured so the UI can present expected capabilities before generation is available.

## Feature Flag

Video Studio is enabled by migration with:

```text
video_studio
```

The frontend gates sidebar visibility and settings control through the same feature-flag path used by Music Studio.
