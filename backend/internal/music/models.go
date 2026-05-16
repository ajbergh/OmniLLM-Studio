package music

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var KnownLyriaModels = map[string][]Model{
	ProviderOpenRouter: {
		{
			ID: "google/lyria-3-clip-preview", Provider: ProviderOpenRouter, Name: "Lyria 3 Clip (Preview)",
			Capabilities: []Capability{CapabilityTextToMusic}, InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
			SupportedFormats: []string{"mp3"}, SupportsStreaming: true, DefaultOutputFormat: "audio/mpeg",
			Pricing: map[string]string{"request": "0.04"}, Notes: "30 second clips via OpenRouter.",
		},
		{
			ID: "google/lyria-3-pro-preview", Provider: ProviderOpenRouter, Name: "Lyria 3 Pro (Preview)",
			Capabilities: []Capability{CapabilityTextToMusic}, InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
			SupportedFormats: []string{"mp3"}, SupportsStreaming: true, DefaultOutputFormat: "audio/mpeg",
			Notes: "Full-length songs via OpenRouter.",
		},
	},
	ProviderGemini: {
		{
			ID: "lyria-3-clip-preview", Provider: ProviderGemini, Name: "Lyria 3 Clip (Preview)",
			Capabilities: []Capability{CapabilityTextToMusic}, InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
			SupportedFormats: []string{"mp3"}, SupportsStreaming: false, DefaultOutputFormat: "audio/mpeg",
			Notes: "Gemini direct Lyria 3 Clip. Google documents 30 second MP3 output.",
		},
		{
			ID: "lyria-3-pro-preview", Provider: ProviderGemini, Name: "Lyria 3 Pro (Preview)",
			Capabilities: []Capability{CapabilityTextToMusic}, InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
			SupportedFormats: []string{"mp3", "wav"}, SupportsStreaming: false, DefaultOutputFormat: "audio/mpeg",
			Notes: "Gemini direct Lyria 3 Pro. Google documents MP3 by default and WAV via generationConfig.",
		},
	},
}

var geminiOverridePattern = regexp.MustCompile(`^lyria-[a-z0-9._:-]+$`)

type modelCacheEntry struct {
	models    []Model
	expiresAt time.Time
}

type ModelRegistry struct {
	mu         sync.Mutex
	cache      map[string]modelCacheEntry
	httpClient *http.Client
}

func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		cache:      make(map[string]modelCacheEntry),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *ModelRegistry) List(ctx context.Context, provider, baseURL, apiKey, customGeminiOverride string, refresh bool) ([]Model, error) {
	provider = normalizeProvider(provider)
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	cacheKey := provider + "|" + strings.TrimSpace(customGeminiOverride)
	if !refresh {
		r.mu.Lock()
		if entry, ok := r.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
			models := cloneModels(entry.models)
			r.mu.Unlock()
			return models, nil
		}
		r.mu.Unlock()
	}

	models := cloneModels(KnownLyriaModels[provider])
	var discovered []Model
	var err error
	switch provider {
	case ProviderOpenRouter:
		discovered, err = r.discoverOpenRouter(ctx, baseURL, apiKey)
	case ProviderGemini:
		discovered, err = r.discoverGemini(ctx, baseURL, apiKey)
		if override := strings.TrimSpace(customGeminiOverride); override != "" {
			if !geminiOverridePattern.MatchString(override) {
				return models, fmt.Errorf("custom Gemini Lyria model must start with lyria-")
			}
			discovered = append(discovered, Model{
				ID: override, Provider: ProviderGemini, Name: override,
				Capabilities:    []Capability{CapabilityTextToMusic},
				InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
				SupportedFormats: []string{"mp3"}, DefaultOutputFormat: "audio/mpeg",
				Notes: "Custom Gemini Lyria override from Settings.",
			})
		}
	}
	if err == nil {
		models = mergeModels(models, discovered)
	}
	sort.SliceStable(models, func(i, j int) bool { return models[i].ID < models[j].ID })

	r.mu.Lock()
	r.cache[cacheKey] = modelCacheEntry{models: cloneModels(models), expiresAt: time.Now().Add(10 * time.Minute)}
	r.mu.Unlock()
	return models, err
}

func (r *ModelRegistry) Clear(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := normalizeProvider(provider) + "|"
	for key := range r.cache {
		if strings.HasPrefix(key, prefix) {
			delete(r.cache, key)
		}
	}
}

func ValidateModel(provider, modelID string, models []Model) bool {
	provider = normalizeProvider(provider)
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	for _, m := range models {
		if m.Provider == provider && strings.EqualFold(m.ID, modelID) {
			return true
		}
	}
	return false
}

func DefaultModel(provider string) string {
	provider = normalizeProvider(provider)
	switch provider {
	case ProviderOpenRouter:
		return "google/lyria-3-clip-preview"
	case ProviderGemini:
		return "lyria-3-clip-preview"
	default:
		return ""
	}
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderOpenRouter:
		return ProviderOpenRouter
	case ProviderGemini:
		return ProviderGemini
	default:
		return ""
	}
}

func mergeModels(base, discovered []Model) []Model {
	byID := make(map[string]Model, len(base)+len(discovered))
	for _, m := range base {
		byID[strings.ToLower(m.ID)] = m
	}
	for _, m := range discovered {
		if m.ID == "" {
			continue
		}
		key := strings.ToLower(m.ID)
		if _, exists := byID[key]; exists {
			continue
		}
		byID[key] = m
	}
	out := make([]Model, 0, len(byID))
	for _, m := range byID {
		out = append(out, m)
	}
	return out
}

func cloneModels(in []Model) []Model {
	out := make([]Model, len(in))
	copy(out, in)
	return out
}

func (r *ModelRegistry) discoverOpenRouter(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/models?output_modalities=audio"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenRouter models returned status %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			ID           string            `json:"id"`
			Name         string            `json:"name"`
			Pricing      map[string]string `json:"pricing"`
			Architecture struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			SupportedParameters []string `json:"supported_parameters"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	var models []Model
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if !strings.HasPrefix(strings.ToLower(id), "google/lyria") {
			continue
		}
		if !containsFold(item.Architecture.OutputModalities, "audio") {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		models = append(models, Model{
			ID: id, Provider: ProviderOpenRouter, Name: name,
			Capabilities:    []Capability{CapabilityTextToMusic},
			InputModalities: item.Architecture.InputModalities, OutputModalities: item.Architecture.OutputModalities,
			SupportedFormats: []string{"mp3"}, SupportsStreaming: true, DefaultOutputFormat: "audio/mpeg",
			Pricing: item.Pricing,
		})
	}
	return models, nil
}

func (r *ModelRegistry) discoverGemini(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	baseURL = strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/openai")
	endpoint := baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini models returned status %d", resp.StatusCode)
	}
	var parsed struct {
		Models []struct {
			Name        string   `json:"name"`
			DisplayName string   `json:"displayName"`
			Methods     []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	var models []Model
	for _, item := range parsed.Models {
		id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		if !strings.HasPrefix(strings.ToLower(id), "lyria-") {
			continue
		}
		name := strings.TrimSpace(item.DisplayName)
		if name == "" {
			name = id
		}
		models = append(models, Model{
			ID: id, Provider: ProviderGemini, Name: name,
			Capabilities:    []Capability{CapabilityTextToMusic},
			InputModalities: []string{"text"}, OutputModalities: []string{"audio", "text"},
			SupportedFormats: []string{"mp3"}, DefaultOutputFormat: "audio/mpeg",
			Notes: "Discovered from Gemini models.list.",
		})
	}
	return models, nil
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
