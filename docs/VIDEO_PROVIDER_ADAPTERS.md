# Video Provider Adapters

Video providers implement the `video.Provider` interface:

```go
type Provider interface {
    Key() string
    DisplayName() string
    Configured() bool
    ListModels(context.Context) ([]Model, error)
    Capabilities(model string) []Capability
    Generate(context.Context, GenerateRequest, func(GenerationProgress)) (*GenerationResult, error)
}
```

## Configured Providers

Video Studio registers providers from encrypted provider profiles:

- `openrouter` is configured when an enabled OpenRouter provider profile has an API key.
- `gemini` is configured when an enabled Gemini provider profile has an API key.
- `luma` is configured when an enabled Luma provider profile has an API key.

Provider profiles keep API keys server-side. The frontend only receives provider status, model metadata, and capability metadata.
There is no local mock provider fallback; generation requires a configured OpenRouter, Gemini, or Luma profile.

## OpenRouter Video

`NewOpenRouterProvider()` uses the OpenRouter Video API:

- Model discovery: `GET /videos/models`
- Generation submit: `POST /videos`
- Job polling: upstream `polling_url`, falling back to a job-content endpoint when needed
- Output retrieval: first completed `unsigned_urls` entry, with provider metadata and usage JSON preserved

The adapter includes a built-in model snapshot for the current OpenRouter video collection so the UI can show useful model choices before credentials are configured or when model discovery is temporarily unavailable.

## Gemini Omni Flash

`gemini-omni-flash-preview` uses the Gemini Interactions API and is always merged into the direct Gemini video catalog because preview Interactions models are not consistently returned by `models.list`.

Video Studio exposes each supported Omni task as a distinct workflow:

- `text_to_video`: prompt-only generation with native synchronized audio.
- `image_to_video`: a single high-resolution starting image plus a motion-focused prompt.
- `reference_to_video`: up to six reference images, with an optional first frame. The adapter adds Google's `<FIRST_FRAME>` and `<IMAGE_REF_N>` role declarations in upload order.
- `edit`: either one imported video or a stored previous Omni interaction. Follow-up turns use `previous_interaction_id`, so the previous result does not need to be uploaded again.

Interactions are submitted to `POST /v1beta/interactions` with `store: true`, `background: false`, and `stream: false`. Output uses `response_format: {"type":"video","delivery":"uri"}` plus the selected `16:9` or `9:16` aspect ratio. Google-hosted output files are polled until active and downloaded into the project's durable asset storage. Inline base64 output remains supported as a compatibility fallback.

Imported source videos use the resumable Gemini Files API, are polled until active, and are deleted after the interaction completes. Conversational edit interaction IDs are stored in `video_generations.upstream_job_id`; the history panel's **Edit** action loads that context and subsequent turns automatically advance to the newest result.

Omni generates audio from prompt instructions and does not expose duration, resolution, FPS, seed, negative-prompt, sampling, system-instruction, extension, interpolation, multi-video, uploaded-audio-reference, or voice-edit controls. The UI intentionally omits those controls for this model. Generated outputs carry SynthID provenance.

## Gemini Veo

The same `NewGeminiProvider()` adapter uses the direct Gemini prediction API for Veo 3.1:

- **Model discovery**: calls `GET /v1beta/models?pageSize=100` and filters to `veo`-named models that support `predictLongRunning`. Falls back to the built-in `KnownGeminiVeoModels()` snapshot when the API is unavailable or returns an empty list.
- Generation submit: `POST /models/{model}:predictLongRunning`
- Job polling: Gemini long-running operation name
- Output retrieval: generated sample `video.uri`
- Capability validation: `POST /v1/video/generations/validate` and both generation paths run the same provider/model validation before upstream submission.

The direct Gemini payload is constructed with one `instances` entry and a separate `parameters` object:

```json
{
  "instances": [{ "prompt": "..." }],
  "parameters": {
    "aspectRatio": "16:9",
    "durationSeconds": 8,
    "resolution": "720p",
    "negativePrompt": "artifacts to avoid",
    "personGeneration": "allow",
    "seed": 1234
  }
}
```

Image and video inputs are separate fields on the instance:

- **Start image / image-to-video:** `StartImageAssetID` is resolved by the service to `StartImagePath`; the adapter reads the image and sends it as `instance.image`.
- **Last frame / first-last interpolation:** `LastFrameAssetID` is resolved to `LastFramePath` and sent as `instance.lastFrame`. Validation rejects a last frame unless a start frame is also selected.
- **Source-video extension:** `SourceVideoAssetID` is resolved to `SourceVideoPath` and sent as `instance.video`. Validation rejects source-video extension combined with start frame, last frame, or reference images. Gemini extension is normalized to 720p and 8 seconds.
- **Reference images:** up to 3 `ReferenceAssetIDs` are resolved to `ReferenceAssetPaths` and sent as `instance.referenceImages`. Reference-image mode is normalized to 8 seconds.

Supported image MIME types are JPEG, PNG, WebP, and GIF. Supported source-video extensions include MP4, MOV, AVI, WebM, and MKV.

Dialogue, sound effects, ambient noise, continuity notes, and cinematic controls are appended to the prompt text by `assembleProviderPrompt`. Gemini Veo does not expose a separate `generate_audio` parameter in this adapter; validation warns when raw `generate_audio` settings are supplied for Gemini.

Normalization rules currently surfaced by validation:

- Empty duration defaults to the model/provider default.
- Gemini durations are clamped to the model range, currently 4-8 seconds.
- Gemini 1080p/4K, first-last-frame, reference-image, and source-video modes are normalized to 8 seconds.
- Gemini source-video extension is normalized to 720p.
- FPS is normalized to the model's advertised option, currently 24 fps.

**Built-in model list (snapshot):**

- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`
- `veo-3.1-lite-generate-preview`

If a Gemini profile has the OpenAI-compatible `/openai` suffix in its base URL, the video adapter trims that suffix because Veo uses the direct Gemini REST API.

## Luma Dream Machine

`NewLumaProvider()` uses the Luma Dream Machine generations API (`https://api.lumalabs.ai/dream-machine/v1`):

- Generation submit: `POST /generations` with `Authorization: Bearer <key>`
- Job polling: `GET /generations/{id}` until `state` is `completed` or `failed` (`failure_reason` is mapped into the generation error)
- Output retrieval: `assets.video` CDN URL (no auth header), downloaded with the shared retry helper

Request payload:

```json
{
  "prompt": "...",
  "model": "ray-2",
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "duration": "9s"
}
```

Adapter-specific behavior:

- **Static model catalog.** Luma has no model discovery endpoint; `KnownLumaVideoModels()` (ray-2, ray-flash-2, ray-1-6) is the source of truth.
- **Text-to-video only.** Luma image keyframes (`keyframes.frame0/frame1`) require publicly hosted HTTPS URLs, which local Video Studio assets cannot provide, and video extension references prior Luma generation IDs. The adapter therefore advertises only `text_to_video`; capability validation rejects start frame, last frame, source video, and reference images for Luma models before any upstream call.
- **Discrete durations.** ray-2 family models accept `"5s"` or `"9s"`; the requested duration is rounded (≥ 7s → 9s) at payload time, and validation clamps to the 5–9s model range first.
- **Legacy ray-1-6** does not accept `resolution`/`duration` parameters; the adapter omits them.
- An optional `loop` boolean passes through from raw generation settings.

## Generation Validation

The backend exposes `POST /v1/video/generations/validate` for frontend preflight checks. The response contains:

- `valid`: whether the request can be submitted.
- `errors`: hard failures that block generation.
- `warnings`: supported but important provider behavior, such as Gemini audio cues being prompt-only.
- `normalizations`: automatic changes the backend will apply before submission.
- `normalized_request`: the request that generation will use after normalization.

Both synchronous and asynchronous generation paths call the same validation layer and use `normalized_request` before creating provider payloads.

## Adapter Checklist

- Keep API keys and upstream credentials server-side.
- Return capability metadata so the frontend can render controls from model capabilities.
- Validate model IDs and unsupported capability combinations before calling upstream APIs.
- Stream progress through `GenerationProgress` where possible.
- Return raw bytes plus media metadata in `GenerationResult`.
- Preserve upstream request IDs, job IDs, usage JSON, and costs when providers expose them.
- Do not persist provider-native project or timeline formats directly.
