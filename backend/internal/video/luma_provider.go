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

const defaultLumaBaseURL = "https://api.lumalabs.ai/dream-machine/v1"

// LumaProvider adapts the Luma Dream Machine generations API to the
// video.Provider interface.
//
// Integration notes:
//   - Luma image keyframes (start/end frame) require publicly hosted HTTPS
//     URLs; local Video Studio assets cannot be sent, so image-to-video and
//     first/last-frame capabilities are intentionally not advertised.
//   - Video extension references prior Luma generation IDs, which do not map
//     to local assets either, so extend_video is not advertised.
//   - ray-2 family models accept discrete durations ("5s"/"9s"); the requested
//     duration is rounded to the nearest supported value at payload time.
type LumaProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewLumaProvider(baseURL, apiKey string) *LumaProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultLumaBaseURL
	}
	return &LumaProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (p *LumaProvider) Key() string {
	return ProviderLuma
}

func (p *LumaProvider) DisplayName() string {
	return "Luma Dream Machine"
}

func (p *LumaProvider) Configured() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

// ListModels returns the static Luma model catalog. Luma does not expose a
// model discovery endpoint, so the snapshot is the source of truth.
func (p *LumaProvider) ListModels(ctx context.Context) ([]Model, error) {
	_ = ctx
	return KnownLumaVideoModels(), nil
}

func (p *LumaProvider) Capabilities(model string) []Capability {
	return lumaCapabilitiesForID(model)
}

func KnownLumaVideoModels() []Model {
	return []Model{
		{
			ID:                 "ray-2",
			Provider:           ProviderLuma,
			Name:               "Luma: Ray 2",
			Capabilities:       lumaCapabilitiesForID("ray-2"),
			AspectRatios:       []string{"1:1", "16:9", "9:16", "4:3", "3:4", "21:9", "9:21"},
			Resolutions:        []string{"540p", "720p", "1080p", "4k"},
			DurationMinSeconds: 5,
			DurationMaxSeconds: 9,
			FPSOptions:         []int{24},
			MaxPromptChars:     4000,
			Notes:              "Luma Ray 2 text-to-video. Durations are rounded to 5s or 9s. Image keyframes require publicly hosted URLs and are not yet supported through OmniLLM Studio.",
		},
		{
			ID:                 "ray-flash-2",
			Provider:           ProviderLuma,
			Name:               "Luma: Ray Flash 2",
			Capabilities:       lumaCapabilitiesForID("ray-flash-2"),
			AspectRatios:       []string{"1:1", "16:9", "9:16", "4:3", "3:4", "21:9", "9:21"},
			Resolutions:        []string{"540p", "720p", "1080p"},
			DurationMinSeconds: 5,
			DurationMaxSeconds: 9,
			FPSOptions:         []int{24},
			MaxPromptChars:     4000,
			Notes:              "Luma Ray Flash 2 trades quality for speed and cost. Durations are rounded to 5s or 9s.",
		},
		{
			ID:                 "ray-1-6",
			Provider:           ProviderLuma,
			Name:               "Luma: Ray 1.6",
			Capabilities:       lumaCapabilitiesForID("ray-1-6"),
			AspectRatios:       []string{"1:1", "16:9", "9:16", "4:3", "3:4", "21:9", "9:21"},
			Resolutions:        []string{"540p", "720p"},
			DurationMinSeconds: 5,
			DurationMaxSeconds: 5,
			FPSOptions:         []int{24},
			MaxPromptChars:     4000,
			Notes:              "Legacy Luma Ray 1.6. Fixed ~5 second clips; resolution and duration parameters are not sent.",
		},
	}
}

func lumaCapabilitiesForID(id string) []Capability {
	_ = id
	// Image/video keyframe inputs require public URLs (see LumaProvider doc
	// comment), so only text-to-video is advertised for every Ray model.
	return []Capability{
		CapabilityTextToVideo,
		CapabilityCameraMotion,
	}
}

// lumaRay2Family reports whether the model accepts resolution/duration params.
func lumaRay2Family(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(lower, "ray-2") || strings.HasPrefix(lower, "ray-flash-2")
}

// lumaDurationParam rounds a duration in seconds to Luma's discrete values.
func lumaDurationParam(seconds int) string {
	if seconds >= 7 {
		return "9s"
	}
	return "5s"
}

// buildPayload constructs the Luma generations request body.
func (p *LumaProvider) buildPayload(req GenerateRequest) map[string]any {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "ray-2"
	}
	payload := map[string]any{
		"prompt": assembleProviderPrompt(req),
		"model":  model,
	}
	if value := strings.TrimSpace(req.AspectRatio); value != "" {
		payload["aspect_ratio"] = value
	}
	if lumaRay2Family(model) {
		if value := strings.TrimSpace(req.Resolution); value != "" {
			payload["resolution"] = strings.ToLower(value)
		}
		payload["duration"] = lumaDurationParam(defaultInt(req.DurationSeconds, 5))
	}
	mergeAllowedVideoSettings(payload, req.Settings, map[string]bool{
		"loop": true,
	})
	return payload
}

func (p *LumaProvider) Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error) {
	if !p.Configured() {
		return nil, fmt.Errorf("%w: no enabled Luma provider profile with an API key", ErrProviderUnavailable)
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "submitting", Message: "Submitting Luma Dream Machine generation", Progress: 0.08})
	}

	payload := p.buildPayload(req)
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit Luma generation: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("read Luma submit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Luma returned %s: %s", resp.Status, responseSnippet(respBody))
	}

	var submit lumaGenerationResponse
	if err := json.Unmarshal(respBody, &submit); err != nil {
		return nil, fmt.Errorf("decode Luma submit response: %w", err)
	}
	if strings.TrimSpace(submit.ID) == "" {
		return nil, errors.New("Luma submit response did not include a generation id")
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "queued", Message: "Luma generation accepted", Progress: 0.15})
	}

	status, err := p.pollGeneration(ctx, submit.ID, progress)
	if err != nil {
		return nil, err
	}
	videoURL := strings.TrimSpace(status.Assets.Video)
	if videoURL == "" {
		return nil, errors.New("Luma generation completed without a video asset URL")
	}
	if progress != nil {
		progress(GenerationProgress{Stage: "downloading", Message: "Downloading Luma video output", Progress: 0.95})
	}
	// Luma serves outputs from a public CDN; no auth header is required.
	data, mimeType, err := downloadWithRetry(ctx, p.client, videoURL, "Luma", nil)
	if err != nil {
		return nil, err
	}
	if mimeType == "" || !strings.HasPrefix(mimeType, "video/") {
		mimeType = "video/mp4"
	}

	width, height := dimensionsForResolution(req.Resolution, req.AspectRatio)
	fps := float64(24)
	durationSeconds := defaultInt(req.DurationSeconds, 5)
	if lumaRay2Family(req.Model) && lumaDurationParam(durationSeconds) == "9s" {
		durationSeconds = 9
	} else if lumaRay2Family(req.Model) {
		durationSeconds = 5
	}
	durationMS := int64(durationSeconds * 1000)
	fileName := "luma-" + sanitizePathSegment(defaultString(req.Model, "ray-2")) + extensionForMimeType(mimeType)
	return &GenerationResult{
		MimeType:      mimeType,
		FileName:      fileName,
		Data:          data,
		DurationMS:    &durationMS,
		Width:         &width,
		Height:        &height,
		FPS:           &fps,
		UpstreamJobID: stringPtrIfNotEmpty(status.ID),
		Metadata: map[string]any{
			"provider":        ProviderLuma,
			"model":           defaultString(req.Model, "ray-2"),
			"luma_generation": status.ID,
			"download_source": "luma_dream_machine_api",
		},
	}, nil
}

func (p *LumaProvider) pollGeneration(ctx context.Context, generationID string, progress func(GenerationProgress)) (*lumaGenerationResponse, error) {
	pollURL := p.baseURL + "/generations/" + generationID
	var last lumaGenerationResponse
	for attempt := 0; attempt < 120; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(lumaPollInterval):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Accept", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("poll Luma generation: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderJSONBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Luma poll response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Luma poll returned %s: %s", resp.Status, responseSnippet(body))
		}
		if err := json.Unmarshal(body, &last); err != nil {
			return nil, fmt.Errorf("decode Luma poll response: %w", err)
		}
		state := strings.ToLower(strings.TrimSpace(last.State))
		if progress != nil {
			progress(GenerationProgress{
				Stage:    defaultString(state, "polling"),
				Message:  "Luma generation " + defaultString(state, "in progress"),
				Progress: minFloat(0.9, 0.2+(float64(attempt)*0.03)),
			})
		}
		switch state {
		case "completed":
			return &last, nil
		case "failed", "rejected":
			return nil, fmt.Errorf("Luma generation %s: %s", state, defaultString(last.FailureReason, "unknown upstream error"))
		}
	}
	return nil, errors.New("Luma generation timed out while polling")
}

// lumaPollInterval is a variable so tests can shorten the poll loop.
var lumaPollInterval = 10 * time.Second

type lumaGenerationResponse struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	FailureReason string `json:"failure_reason"`
	Model         string `json:"model"`
	Assets        struct {
		Video string `json:"video"`
		Image string `json:"image"`
	} `json:"assets"`
}
