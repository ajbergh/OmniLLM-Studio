# Video Provider Adapters

Video providers implement the `video.Provider` interface:

```go
type Provider interface {
    Info(context.Context) ProviderInfo
    ListModels(context.Context) ([]Model, error)
    Generate(context.Context, GenerateRequest, func(GenerationProgress)) (*GenerationResult, error)
}
```

## Adapter Checklist

- Keep API keys and upstream credentials server-side.
- Return capability metadata so the frontend can render controls from model capabilities.
- Validate model IDs and unsupported capability combinations before calling upstream APIs.
- Stream progress through `GenerationProgress` where possible.
- Return raw bytes plus media metadata in `GenerationResult`.
- Preserve upstream request IDs, job IDs, usage JSON, and costs when providers expose them.
- Do not persist provider-native project or timeline formats directly.

## Mock Provider

`NewMockProvider()` is the local development adapter. It supports text-to-video, prompt enhancement workflows, and deterministic placeholder assets.

## Future Providers

Provider adapters can be registered in `NewModelRegistry(...)`. The current registry is intentionally small so provider-specific API work stays isolated from the Video Studio domain model.
