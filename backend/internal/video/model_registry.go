package video

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type ModelRegistry struct {
	providers map[string]Provider
}

func NewModelRegistry(providers ...Provider) *ModelRegistry {
	registry := &ModelRegistry{providers: map[string]Provider{}}
	for _, provider := range providers {
		registry.Register(provider)
	}
	return registry
}

func (r *ModelRegistry) Register(provider Provider) {
	if provider == nil {
		return
	}
	r.providers[provider.Key()] = provider
}

func (r *ModelRegistry) Provider(key string) (Provider, bool) {
	provider, ok := r.providers[NormalizeProvider(key)]
	return provider, ok
}

func (r *ModelRegistry) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	keys := make([]string, 0, len(r.providers))
	for key := range r.providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]ProviderInfo, 0, len(keys))
	for _, key := range keys {
		provider := r.providers[key]
		models, err := provider.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, ProviderInfo{
			Key:         provider.Key(),
			DisplayName: provider.DisplayName(),
			Configured:  provider.Configured(),
			Models:      models,
		})
	}
	return out, nil
}

func (r *ModelRegistry) ListModels(ctx context.Context, provider string) ([]Model, error) {
	p, ok := r.Provider(provider)
	if !ok {
		return nil, fmt.Errorf("unsupported video provider: %s", provider)
	}
	return p.ListModels(ctx)
}

func (r *ModelRegistry) ValidateModel(ctx context.Context, provider, modelID string) bool {
	models, err := r.ListModels(ctx, provider)
	if err != nil {
		return false
	}
	modelID = strings.TrimSpace(modelID)
	for _, model := range models {
		if strings.EqualFold(model.ID, modelID) {
			return true
		}
	}
	return false
}

func (r *ModelRegistry) DefaultModel(ctx context.Context, provider string) string {
	models, err := r.ListModels(ctx, provider)
	if err != nil || len(models) == 0 {
		return ""
	}
	return models[0].ID
}

func NormalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "":
		return ""
	case ProviderOpenRouter:
		return ProviderOpenRouter
	case ProviderGemini:
		return ProviderGemini
	case ProviderOpenAI:
		return ProviderOpenAI
	case ProviderCustom:
		return ProviderCustom
	default:
		return ""
	}
}
