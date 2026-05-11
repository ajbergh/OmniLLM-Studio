package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// providerWithCapabilities wraps a ProviderProfile with computed capabilities.
type providerWithCapabilities struct {
	models.ProviderProfile
	ImageCapable bool `json:"image_capable"`
}

// ProviderHandler handles provider profile API endpoints.
type ProviderHandler struct {
	repo *repository.ProviderRepo
}

// NewProviderHandler creates a new ProviderHandler.
func NewProviderHandler(repo *repository.ProviderRepo) *ProviderHandler {
	return &ProviderHandler{repo: repo}
}

func withProviderCapabilities(provider models.ProviderProfile) providerWithCapabilities {
	return providerWithCapabilities{
		ProviderProfile: provider,
		ImageCapable:    llm.IsImageCapableProvider(provider.Type),
	}
}

func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.repo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if providers == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	// Enrich with computed capabilities
	result := make([]providerWithCapabilities, len(providers))
	for i, p := range providers {
		result[i] = withProviderCapabilities(p)
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input repository.CreateProviderInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name == "" || input.Type == "" {
		respondError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	provider, err := h.repo.Create(input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, withProviderCapabilities(*provider))
}

func (h *ProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "providerId")

	var upd repository.ProviderUpdate
	if err := decodeJSON(r, &upd); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	provider, err := h.repo.Update(id, upd)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == nil {
		respondError(w, http.StatusNotFound, "provider not found")
		return
	}
	respondJSON(w, http.StatusOK, withProviderCapabilities(*provider))
}

func (h *ProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "providerId")
	if err := h.repo.Delete(id); err != nil {
		respondInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// FetchOllamaModels proxies a model-list request to an Ollama instance so
// the frontend doesn't need direct cross-origin access (required for the
// Wails desktop build where the WebView2 origin is not http://localhost).
func (h *ProviderHandler) FetchOllamaModels(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(r.URL.Query().Get("base_url"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid base_url")
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusBadGateway, "cannot reach Ollama at "+baseURL)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB limit
	if err != nil {
		respondError(w, http.StatusBadGateway, "error reading Ollama response")
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondError(w, http.StatusBadGateway, "Ollama returned status "+resp.Status)
		return
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		respondError(w, http.StatusBadGateway, "invalid response from Ollama")
		return
	}

	names := make([]string, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		names = append(names, m.Name)
	}
	respondJSON(w, http.StatusOK, names)
}

// FetchOpenRouterModels fetches available models from OpenRouter API.
// GET /v1/providers/openrouter/models?provider_id=...
func (h *ProviderHandler) FetchOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimSpace(r.URL.Query().Get("api_key")) // fallback for compatibility
	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	if apiKey == "" && providerID != "" {
		key, err := h.repo.GetAPIKey(providerID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		apiKey = strings.TrimSpace(key)
	}
	if apiKey == "" {
		respondError(w, http.StatusBadRequest, "provider_id (or api_key) is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusBadGateway, "cannot reach OpenRouter API")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB limit
	if err != nil {
		respondError(w, http.StatusBadGateway, "error reading OpenRouter response")
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondError(w, http.StatusBadGateway, "OpenRouter returned status "+resp.Status)
		return
	}

	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		respondError(w, http.StatusBadGateway, "invalid response from OpenRouter")
		return
	}

	// Return simplified model list (id and name)
	type modelInfo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	models := make([]modelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, modelInfo{ID: m.ID, Name: m.Name})
	}

	respondJSON(w, http.StatusOK, models)
}

// GetImageCapabilities returns the image capabilities for a provider.
// GET /v1/providers/{providerId}/image-capabilities
func (h *ProviderHandler) GetImageCapabilities(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "providerId")

	provider, err := h.repo.GetByID(providerID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == nil {
		respondError(w, http.StatusNotFound, "provider not found")
		return
	}

	caps := llm.GetImageCapabilities(provider.Type)
	if provider.DefaultImageModel != nil {
		model := strings.TrimSpace(*provider.DefaultImageModel)
		if model != "" {
			caps.DefaultImageModel = model
			found := false
			for _, existing := range caps.ImageModels {
				if existing == model {
					found = true
					break
				}
			}
			if !found {
				caps.ImageModels = append(caps.ImageModels, model)
			}
		}
	}
	respondJSON(w, http.StatusOK, caps)
}
