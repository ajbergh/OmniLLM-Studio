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
