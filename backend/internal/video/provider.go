package video

import (
	"context"
	"strings"
)

type Provider interface {
	Key() string
	DisplayName() string
	Configured() bool
	ListModels(ctx context.Context) ([]Model, error)
	Capabilities(model string) []Capability
	Generate(ctx context.Context, req GenerateRequest, progress func(GenerationProgress)) (*GenerationResult, error)
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
	case "4k", "2160p":
		if aspectRatio == "9:16" {
			return 2160, 3840
		}
		return 3840, 2160
	case "480p":
		if aspectRatio == "9:16" {
			return 480, 854
		}
		return 854, 480
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
