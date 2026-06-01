package video

import (
	"context"
	"testing"
)

func TestModelRegistryIncludesRealProviderSnapshots(t *testing.T) {
	registry := NewModelRegistry(
		NewOpenRouterProvider("", ""),
		NewGeminiProvider("", ""),
	)

	providers, err := registry.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders returned error: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %+v", providers)
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
	if NormalizeProvider("") != "" {
		t.Fatalf("empty provider should not normalize to a concrete adapter")
	}
}
