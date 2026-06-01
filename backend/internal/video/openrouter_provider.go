package video

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenRouterVideoBaseURL = "https://openrouter.ai/api/v1"

type OpenRouterProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenRouterProvider(baseURL, apiKey string) *OpenRouterProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenRouterVideoBaseURL
	}
	return &OpenRouterProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (p *OpenRouterProvider) Key() string {
	return ProviderOpenRouter
}

func (p *OpenRouterProvider) DisplayName() string {
	return "OpenRouter Video"
}

func (p *OpenRouterProvider) Configured() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]Model, error) {
	fallback := KnownOpenRouterVideoModels()
	if !p.Configured() {
		return fallback, nil
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(discoveryCtx, http.MethodGet, p.baseURL+"/videos/models", nil)
	if err != nil {
		return fallback, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return modelsWithDiscoveryNote(fallback, "OpenRouter model discovery failed; showing built-in May 2026 snapshot."), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return modelsWithDiscoveryNote(fallback, "OpenRouter model discovery failed; showing built-in May 2026 snapshot."), nil
	}

	var payload openRouterModelsResponse
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Data) == 0 {
		return modelsWithDiscoveryNote(fallback, "OpenRouter model discovery returned no video models; showing built-in May 2026 snapshot."), nil
	}
	models := make([]Model, 0, len(payload.Data))
	for _, upstream := range payload.Data {
		model := openRouterModelFromAPI(upstream)
		if model.ID != "" {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return fallback, nil
	}
	return models, nil
}

func (p *OpenRouterProvider) Capabilities(model string) []Capability {
	return openRouterCapabilitiesForID(model)
}

func (p *OpenRouterProvider) Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error) {
	if !p.Configured() {
		return nil, fmt.Errorf("%w: no enabled OpenRouter provider profile with an API key", ErrProviderUnavailable)
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "submitting", Message: "Submitting OpenRouter video generation job", Progress: 0.08})
	}

	payload := map[string]any{
		"model":  strings.TrimSpace(req.Model),
		"prompt": assembleProviderPrompt(req),
	}
	if req.DurationSeconds > 0 {
		payload["duration"] = req.DurationSeconds
	}
	if value := strings.TrimSpace(req.Resolution); value != "" {
		payload["resolution"] = value
	}
	if value := strings.TrimSpace(req.AspectRatio); value != "" {
		payload["aspect_ratio"] = value
	}
	if req.Seed != nil {
		payload["seed"] = *req.Seed
	}
	mergeAllowedVideoSettings(payload, req.Settings, map[string]bool{
		"callback_url":     true,
		"frame_images":     true,
		"generate_audio":   true,
		"input_references": true,
		"provider":         true,
		"size":             true,
	})
	if strings.TrimSpace(req.NegativePrompt) != "" && strings.HasPrefix(strings.ToLower(req.Model), "google/") {
		applyOpenRouterGoogleNegativePrompt(payload, req.NegativePrompt)
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/videos", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit OpenRouter video job: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("read OpenRouter submit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenRouter returned %s: %s", resp.Status, responseSnippet(respBody))
	}

	var submit openRouterJobResponse
	if err := json.Unmarshal(respBody, &submit); err != nil {
		return nil, fmt.Errorf("decode OpenRouter submit response: %w", err)
	}
	jobID := firstNonEmpty(submit.ID, submit.GenerationID, submit.Data.ID, submit.Data.GenerationID)
	pollURL := firstNonEmpty(submit.PollingURL, submit.Data.PollingURL)
	if jobID == "" && pollURL == "" {
		return nil, errors.New("OpenRouter submit response did not include a job id or polling URL")
	}
	if pollURL == "" {
		pollURL = p.baseURL + "/videos/" + jobID
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "queued", Message: "OpenRouter video job accepted", Progress: 0.15})
	}

	status, err := p.pollJob(ctx, pollURL, progress)
	if err != nil {
		return nil, err
	}
	downloadURL := firstString(status.UnsignedURLs)
	if downloadURL == "" {
		downloadURL = firstString(status.Data.UnsignedURLs)
	}
	if downloadURL == "" && jobID != "" {
		downloadURL = p.baseURL + "/videos/" + jobID + "/content?index=0"
	}
	if downloadURL == "" {
		return nil, errors.New("OpenRouter job completed without a downloadable video URL")
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "downloading", Message: "Downloading OpenRouter video output", Progress: 0.95})
	}
	data, mimeType, err := p.downloadVideo(ctx, downloadURL)
	if err != nil {
		return nil, err
	}
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	width, height := dimensionsForResolution(req.Resolution, req.AspectRatio)
	fps := float64(24)
	duration := int64(defaultInt(req.DurationSeconds, 8) * 1000)
	usage := firstRawMessage(status.Usage, status.Data.Usage)
	cost := costFromUsage(usage)
	upstreamJobID := firstNonEmpty(status.ID, status.GenerationID, status.Data.ID, status.Data.GenerationID, jobID)
	fileName := "openrouter-" + sanitizePathSegment(req.Model) + extensionForMimeType(mimeType)
	return &GenerationResult{
		MimeType:      mimeType,
		FileName:      fileName,
		Data:          data,
		DurationMS:    &duration,
		Width:         &width,
		Height:        &height,
		FPS:           &fps,
		UpstreamJobID: stringPtrIfNotEmpty(upstreamJobID),
		UsageJSON:     usage,
		CostUSD:       cost,
		Metadata: map[string]any{
			"provider":        ProviderOpenRouter,
			"model":           req.Model,
			"polling_url":     pollURL,
			"openrouter_job":  upstreamJobID,
			"download_source": "openrouter_video_api",
		},
	}, nil
}

func (p *OpenRouterProvider) pollJob(ctx context.Context, pollingURL string, progress func(GenerationProgress)) (*openRouterJobResponse, error) {
	var last openRouterJobResponse
	for attempt := 0; attempt < 120; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollingURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Accept", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("poll OpenRouter video job: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read OpenRouter poll response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("OpenRouter poll returned %s: %s", resp.Status, responseSnippet(body))
		}
		if err := json.Unmarshal(body, &last); err != nil {
			return nil, fmt.Errorf("decode OpenRouter poll response: %w", err)
		}
		status := strings.ToLower(firstNonEmpty(last.Status, last.Data.Status))
		if progress != nil {
			progress(GenerationProgress{
				Stage:    defaultString(status, "polling"),
				Message:  "OpenRouter video job " + defaultString(status, "in progress"),
				Progress: minFloat(0.9, 0.2+(float64(attempt)*0.03)),
			})
		}
		switch status {
		case "completed", "succeeded", "success", "done":
			return &last, nil
		case "failed", "cancelled", "canceled", "expired":
			return nil, fmt.Errorf("OpenRouter video job %s: %s", status, openRouterErrorMessage(last.Error, last.Data.Error))
		}
	}
	return nil, errors.New("OpenRouter video job timed out while polling")
}

func (p *OpenRouterProvider) downloadVideo(ctx context.Context, downloadURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(downloadURL, "/api/v1/videos/") {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download OpenRouter video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		return nil, "", fmt.Errorf("OpenRouter video download returned %s: %s", resp.Status, responseSnippet(body))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderDownloadBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read OpenRouter video download: %w", err)
	}
	return data, strings.TrimSpace(resp.Header.Get("Content-Type")), nil
}

type openRouterModelsResponse struct {
	Data []openRouterModelResponse `json:"data"`
}

type openRouterModelResponse struct {
	ID                           string          `json:"id"`
	CanonicalSlug                string          `json:"canonical_slug"`
	Name                         string          `json:"name"`
	Description                  string          `json:"description"`
	SupportedResolutions         []string        `json:"supported_resolutions"`
	SupportedAspectRatios        []string        `json:"supported_aspect_ratios"`
	AllowedPassthroughParameters []string        `json:"allowed_passthrough_parameters"`
	SupportedDurations           []int           `json:"supported_durations"`
	PricingSKUs                  json.RawMessage `json:"pricing_skus"`
}

type openRouterJobResponse struct {
	ID           string          `json:"id"`
	GenerationID string          `json:"generation_id"`
	PollingURL   string          `json:"polling_url"`
	Status       string          `json:"status"`
	UnsignedURLs []string        `json:"unsigned_urls"`
	Usage        json.RawMessage `json:"usage"`
	Error        any             `json:"error"`
	Data         struct {
		ID           string          `json:"id"`
		GenerationID string          `json:"generation_id"`
		PollingURL   string          `json:"polling_url"`
		Status       string          `json:"status"`
		UnsignedURLs []string        `json:"unsigned_urls"`
		Usage        json.RawMessage `json:"usage"`
		Error        any             `json:"error"`
	} `json:"data"`
}

func KnownOpenRouterVideoModels() []Model {
	return []Model{
		openRouterKnownModel("google/veo-3.1", "Google: Veo 3.1", []string{"720p", "1080p", "4k"}, []string{"16:9", "9:16", "1:1"}, 4, 8, "OpenRouter Veo 3.1 text/image video generation with native audio and Google Vertex passthrough support."),
		openRouterKnownModel("google/veo-3.1-fast", "Google: Veo 3.1 Fast", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 4, 8, "OpenRouter Veo 3.1 Fast balances latency and quality with native audio."),
		openRouterKnownModel("google/veo-3.1-lite", "Google: Veo 3.1 Lite", []string{"720p", "1080p"}, []string{"16:9", "9:16"}, 4, 8, "OpenRouter Veo 3.1 Lite for lower-cost short clips."),
		openRouterKnownModel("x-ai/grok-imagine-video", "xAI: Grok Imagine Video", []string{"480p", "720p"}, []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}, 1, 15, "OpenRouter Grok Imagine Video supports text, image, and reference-conditioned clips."),
		openRouterKnownModel("kwaivgi/kling-v3.0-pro", "Kling: Video v3.0 Pro", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 15, "OpenRouter Kling v3.0 Pro supports text-to-video, image-to-video, and optional native audio."),
		openRouterKnownModel("kwaivgi/kling-v3.0-std", "Kling: Video v3.0 Standard", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 15, "OpenRouter Kling v3.0 Standard supports text-to-video, image-to-video, and optional native audio."),
		openRouterKnownModel("kwaivgi/kling-video-o1", "Kling: Video O1", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 5, 10, "OpenRouter Kling Video O1 supports text and image inputs."),
		openRouterKnownModel("minimax/hailuo-2.3", "MiniMax: Hailuo 2.3", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 10, "OpenRouter Hailuo 2.3 supports text and reference image workflows."),
		openRouterKnownModel("bytedance/seedance-2.0-fast", "ByteDance: Seedance 2.0 Fast", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 10, "OpenRouter Seedance 2.0 Fast supports text, first/last-frame, and reference workflows."),
		openRouterKnownModel("bytedance/seedance-2.0", "ByteDance: Seedance 2.0", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 10, "OpenRouter Seedance 2.0 supports text, first/last-frame, and reference workflows."),
		openRouterKnownModel("alibaba/wan-2.7", "Alibaba: Wan 2.7", []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 10, "OpenRouter Wan 2.7 supports text-to-video, image-to-video, and reference-to-video."),
	}
}

func openRouterKnownModel(id, name string, resolutions, ratios []string, minDuration, maxDuration int, notes string) Model {
	return Model{
		ID:                 id,
		Provider:           ProviderOpenRouter,
		Name:               name,
		Capabilities:       openRouterCapabilitiesForID(id),
		AspectRatios:       cloneStrings(ratios),
		Resolutions:        cloneStrings(resolutions),
		DurationMinSeconds: minDuration,
		DurationMaxSeconds: maxDuration,
		FPSOptions:         []int{24},
		MaxPromptChars:     4000,
		Notes:              notes,
	}
}

func openRouterModelFromAPI(upstream openRouterModelResponse) Model {
	id := firstNonEmpty(upstream.ID, upstream.CanonicalSlug)
	known := openRouterKnownByID(id)
	if known.ID == "" {
		known = openRouterKnownModel(id, firstNonEmpty(upstream.Name, id), []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, 3, 10, "Discovered from OpenRouter video model API.")
	}
	if upstream.Name != "" {
		known.Name = upstream.Name
	}
	if len(upstream.SupportedResolutions) > 0 {
		known.Resolutions = cloneStrings(upstream.SupportedResolutions)
	}
	if len(upstream.SupportedAspectRatios) > 0 {
		known.AspectRatios = cloneStrings(upstream.SupportedAspectRatios)
	}
	if len(upstream.SupportedDurations) > 0 {
		min, max := upstream.SupportedDurations[0], upstream.SupportedDurations[0]
		for _, value := range upstream.SupportedDurations[1:] {
			if value < min {
				min = value
			}
			if value > max {
				max = value
			}
		}
		known.DurationMinSeconds = min
		known.DurationMaxSeconds = max
	}
	if upstream.Description != "" {
		known.Notes = truncateString(upstream.Description, 260)
	}
	return known
}

func openRouterKnownByID(id string) Model {
	for _, model := range KnownOpenRouterVideoModels() {
		if strings.EqualFold(model.ID, id) {
			return model
		}
	}
	return Model{}
}

func openRouterCapabilitiesForID(id string) []Capability {
	caps := []Capability{
		CapabilityTextToVideo,
		CapabilityImageToVideo,
		CapabilityReferenceImages,
		CapabilitySeed,
		CapabilityCameraMotion,
	}
	lower := strings.ToLower(id)
	if strings.HasPrefix(lower, "google/") {
		caps = append(caps, CapabilityNegativePrompt, CapabilityAudioGeneration)
		if lower == "google/veo-3.1" || lower == "google/veo-3.1-fast" {
			caps = append(caps, CapabilityVideoToVideo)
		}
	}
	if strings.Contains(lower, "kling") {
		caps = append(caps, CapabilityAudioGeneration)
	}
	return caps
}

func applyOpenRouterGoogleNegativePrompt(payload map[string]any, negativePrompt string) {
	if _, exists := payload["provider"]; exists {
		return
	}
	payload["provider"] = map[string]any{
		"options": map[string]any{
			"google-vertex": map[string]any{
				"parameters": map[string]any{
					"negativePrompt": strings.TrimSpace(negativePrompt),
				},
			},
		},
	}
}

func openRouterErrorMessage(value any, fallback any) string {
	for _, candidate := range []any{value, fallback} {
		switch typed := candidate.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case map[string]any:
			for _, key := range []string{"message", "error", "detail"} {
				if msg, ok := typed[key].(string); ok && strings.TrimSpace(msg) != "" {
					return msg
				}
			}
		}
	}
	return "unknown upstream error"
}
