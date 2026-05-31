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

- `mock` is always configured and produces local placeholder assets.
- `openrouter` is configured when an enabled OpenRouter provider profile has an API key.
- `gemini` is configured when an enabled Gemini provider profile has an API key.

Provider profiles keep API keys server-side. The frontend only receives provider status, model metadata, and capability metadata.

## OpenRouter Video

`NewOpenRouterProvider()` uses the OpenRouter Video API:

- Model discovery: `GET /videos/models`
- Generation submit: `POST /videos`
- Job polling: upstream `polling_url`, falling back to a job-content endpoint when needed
- Output retrieval: first completed `unsigned_urls` entry, with provider metadata and usage JSON preserved

The adapter includes a built-in model snapshot for the current OpenRouter video collection so the UI can show useful model choices before credentials are configured or when model discovery is temporarily unavailable.

## Gemini Veo

`NewGeminiProvider()` uses the direct Gemini API for Veo 3.1:

- Generation submit: `POST /models/{model}:predictLongRunning`
- Job polling: Gemini long-running operation name
- Output retrieval: generated sample `video.uri`

The adapter currently exposes:

- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`

If a Gemini profile has the OpenAI-compatible `/openai` suffix in its base URL, the video adapter trims that suffix because Veo uses the direct Gemini REST API.

## Adapter Checklist

- Keep API keys and upstream credentials server-side.
- Return capability metadata so the frontend can render controls from model capabilities.
- Validate model IDs and unsupported capability combinations before calling upstream APIs.
- Stream progress through `GenerationProgress` where possible.
- Return raw bytes plus media metadata in `GenerationResult`.
- Preserve upstream request IDs, job IDs, usage JSON, and costs when providers expose them.
- Do not persist provider-native project or timeline formats directly.
