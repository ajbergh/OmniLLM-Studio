package video

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Provider interface {
	Key() string
	DisplayName() string
	ListModels(ctx context.Context) ([]Model, error)
	Capabilities(model string) []Capability
	Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error)
}

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Key() string {
	return ProviderMock
}

func (p *MockProvider) DisplayName() string {
	return "Mock Video"
}

func (p *MockProvider) ListModels(ctx context.Context) ([]Model, error) {
	return []Model{mockModel()}, nil
}

func (p *MockProvider) Capabilities(model string) []Capability {
	return mockModel().Capabilities
}

func (p *MockProvider) Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error) {
	steps := []GenerationProgress{
		{Stage: "planning", Message: "Creating mock shot plan", Progress: 0.25},
		{Stage: "rendering", Message: "Rendering deterministic placeholder asset", Progress: 0.7},
		{Stage: "packaging", Message: "Packaging mock video asset", Progress: 0.95},
	}
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if progress != nil {
				progress(step)
			}
			time.Sleep(75 * time.Millisecond)
		}
	}

	width, height := dimensionsForResolution(req.Resolution, req.AspectRatio)
	fps := float64(defaultInt(req.FPS, DefaultProjectFPS))
	duration := int64(defaultInt(req.DurationSeconds, 6) * 1000)
	prompt := strings.TrimSpace(req.EnhancedPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	body := fmt.Sprintf(
		"OmniLLM-Studio mock video placeholder\n\nPrompt:\n%s\n\nGenerated: %s\n",
		prompt,
		time.Now().UTC().Format(time.RFC3339),
	)
	return &GenerationResult{
		MimeType:   "text/plain; charset=utf-8",
		FileName:   "mock-video-placeholder.txt",
		Data:       []byte(body),
		DurationMS: &duration,
		Width:      &width,
		Height:     &height,
		FPS:        &fps,
		Metadata: map[string]any{
			"mock":             true,
			"placeholder_kind": "video",
			"note":             "Replace the mock provider with a real video adapter when credentials are available.",
		},
	}, nil
}

func mockModel() Model {
	return Model{
		ID:       "mock-video-v1",
		Provider: ProviderMock,
		Name:     "Mock Video v1",
		Capabilities: []Capability{
			CapabilityTextToVideo,
			CapabilityImageToVideo,
			CapabilityReferenceImages,
			CapabilityNegativePrompt,
			CapabilitySeed,
			CapabilityCameraMotion,
		},
		AspectRatios:       []string{"16:9", "9:16", "1:1", "4:3"},
		Resolutions:        []string{"720p", "1080p"},
		DurationMinSeconds: 2,
		DurationMaxSeconds: 12,
		FPSOptions:         []int{24, 30},
		MaxPromptChars:     4000,
		Notes:              "Local deterministic placeholder provider for Video Studio development.",
	}
}

func dimensionsForResolution(resolution, aspectRatio string) (int, int) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "720p":
		if aspectRatio == "9:16" {
			return 720, 1280
		}
		return 1280, 720
	case "1080p":
		if aspectRatio == "9:16" {
			return 1080, 1920
		}
		return 1920, 1080
	default:
		if aspectRatio == "9:16" {
			return 1080, 1920
		}
		return DefaultProjectWidth, DefaultProjectHeight
	}
}

func defaultInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
