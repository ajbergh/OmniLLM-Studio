package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
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

	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Type) == "" {
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

// FetchOllamaModels proxies model discovery for an Ollama endpoint. Provider
// discovery is an administrative network action: in multi-user mode only an
// admin may invoke it. Solo mode remains available for local desktop use.
func (h *ProviderHandler) FetchOllamaModels(w http.ResponseWriter, r *http.Request) {
	if !requireAdminOrSolo(w, r) {
		return
	}

	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	if providerID == "" {
		respondError(w, http.StatusBadRequest, "provider_id is required")
		return
	}
	provider, err := h.repo.GetByID(providerID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == nil || !strings.EqualFold(provider.Type, "ollama") {
		respondError(w, http.StatusNotFound, "Ollama provider not found")
		return
	}

	baseURL := ""
	if provider.BaseURL != nil && strings.TrimSpace(*provider.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*provider.BaseURL), "/")
	}
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}

	parsed, err := validateProviderDiscoveryURL(baseURL)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, err := providerDiscoveryClient(r.Context(), parsed)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid or unreachable provider host")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(parsed.String(), "/")+"/api/tags", nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid provider URL")
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusBadGateway, "cannot reach configured Ollama provider")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
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
		if name := strings.TrimSpace(m.Name); name != "" {
			names = append(names, name)
		}
	}
	respondJSON(w, http.StatusOK, names)
}

// FetchOpenRouterModels fetches available models using only the encrypted key
// associated with a stored provider profile. Credentials are never accepted in
// query strings because URLs are commonly logged by browsers and proxies.
func (h *ProviderHandler) FetchOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	if providerID == "" {
		respondError(w, http.StatusBadRequest, "provider_id is required")
		return
	}
	provider, err := h.repo.GetByID(providerID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == nil || !strings.EqualFold(provider.Type, "openrouter") {
		respondError(w, http.StatusNotFound, "OpenRouter provider not found")
		return
	}
	apiKey, err := h.repo.GetAPIKey(providerID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		respondError(w, http.StatusBadRequest, "provider has no configured API key")
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

	client := tools.NewSSRFSafeClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusBadGateway, "cannot reach OpenRouter API")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
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
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		respondError(w, http.StatusBadGateway, "invalid response from OpenRouter")
		return
	}

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

func requireAdminOrSolo(w http.ResponseWriter, r *http.Request) bool {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return true
	}
	if user.Role != "admin" {
		respondError(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

func validateProviderDiscoveryURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid provider URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("provider URL must use http or https")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("provider URL must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("provider URL must not contain a query or fragment")
	}
	return parsed, nil
}

func providerDiscoveryClient(ctx context.Context, target *url.URL) (*http.Client, error) {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, target.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("resolve provider host")
	}
	local := true
	for _, addr := range ips {
		ip := addr.IP
		if !(ip.IsLoopback() || ip.IsPrivate()) {
			local = false
			break
		}
	}
	if !local {
		return tools.NewSSRFSafeClient(10 * time.Second), nil
	}

	host := strings.ToLower(target.Hostname())
	scheme := target.Scheme
	port := target.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
				dialHost, dialPort, splitErr := net.SplitHostPort(address)
				if splitErr != nil || !strings.EqualFold(dialHost, host) || dialPort != port {
					return nil, fmt.Errorf("provider connection changed origin")
				}
				resolved, resolveErr := net.DefaultResolver.LookupIPAddr(dialCtx, host)
				if resolveErr != nil || len(resolved) == 0 {
					return nil, fmt.Errorf("resolve provider host")
				}
				for _, addr := range resolved {
					if !(addr.IP.IsLoopback() || addr.IP.IsPrivate()) {
						return nil, fmt.Errorf("provider host no longer resolves to a private address")
					}
				}
				return dialer.DialContext(dialCtx, network, net.JoinHostPort(resolved[0].IP.String(), port))
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if !strings.EqualFold(req.URL.Hostname(), host) || req.URL.Scheme != scheme {
				return fmt.Errorf("provider redirect changed origin")
			}
			return nil
		},
	}, nil
}
