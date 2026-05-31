package video

import (
	"context"
	"testing"
)

func TestModelRegistryIncludesMockProvider(t *testing.T) {
	registry := NewModelRegistry(NewMockProvider())
	providers, err := registry.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders returned error: %v", err)
	}
	if len(providers) != 1 || providers[0].Key != ProviderMock || !providers[0].Configured {
		t.Fatalf("mock provider missing or not configured: %+v", providers)
	}

	models, err := registry.ListModels(context.Background(), ProviderMock)
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "mock-video-v1" {
		t.Fatalf("unexpected mock models: %+v", models)
	}
	if !registry.ValidateModel(context.Background(), ProviderMock, "mock-video-v1") {
		t.Fatalf("expected mock-video-v1 to validate")
	}
	if registry.ValidateModel(context.Background(), ProviderMock, "missing") {
		t.Fatalf("unexpected missing model validation")
	}
}

func TestModelRegistryIncludesRealProviderSnapshots(t *testing.T) {
	registry := NewModelRegistry(
		NewMockProvider(),
		NewOpenRouterProvider("", ""),
		NewGeminiProvider("", ""),
	)

	providers, err := registry.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders returned error: %v", err)
	}
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %+v", providers)
	}

	var openRouter, gemini *ProviderInfo
	for i := range providers {
		switch providers[i].Key {
		case ProviderOpenRouter:
			openRouter = &providers[i]
		case ProviderGemini:
			gemini = &providers[i]
		}
	}
	if openRouter == nil || openRouter.Configured || len(openRouter.Models) == 0 {
		t.Fatalf("expected unconfigured OpenRouter snapshot models, got %+v", openRouter)
	}
	if gemini == nil || gemini.Configured || len(gemini.Models) == 0 {
		t.Fatalf("expected unconfigured Gemini snapshot models, got %+v", gemini)
	}
	if !registry.ValidateModel(context.Background(), ProviderOpenRouter, "google/veo-3.1") {
		t.Fatalf("expected OpenRouter Veo 3.1 model to validate")
	}
	if !registry.ValidateModel(context.Background(), ProviderGemini, "veo-3.1-generate-preview") {
		t.Fatalf("expected Gemini Veo 3.1 model to validate")
	}
}
