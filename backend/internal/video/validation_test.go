package video

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateValidationRejectsLastFrameWithoutStartFrame(t *testing.T) {
	registry := NewModelRegistry(NewGeminiProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:         ProviderGemini,
		Model:            "veo-3.1-generate-preview",
		Prompt:           "A city skyline at sunset",
		LastFrameAssetID: "asset-last",
	})
	if result.Valid {
		t.Fatalf("expected validation to fail")
	}
	if !hasIssue(result.Errors, "last_frame_requires_start_frame") {
		t.Fatalf("expected last_frame_requires_start_frame, got %+v", result.Errors)
	}
}

func TestGenerateValidationNormalizesGeminiReferenceMode(t *testing.T) {
	registry := NewModelRegistry(NewGeminiProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:          ProviderGemini,
		Model:             "veo-3.1-generate-preview",
		Prompt:            "A product hero shot on a glossy table",
		ReferenceAssetIDs: []string{"ref-1"},
		Resolution:        "1080p",
		DurationSeconds:   6,
		FPS:               30,
	})
	if !result.Valid {
		t.Fatalf("expected validation to pass, got %+v", result.Errors)
	}
	if result.NormalizedRequest.DurationSeconds != 8 {
		t.Fatalf("expected duration normalized to 8s, got %d", result.NormalizedRequest.DurationSeconds)
	}
	if result.NormalizedRequest.FPS != 24 {
		t.Fatalf("expected fps normalized to 24, got %d", result.NormalizedRequest.FPS)
	}
	if !hasIssue(result.Normalizations, "gemini_duration_normalized") {
		t.Fatalf("expected gemini duration normalization, got %+v", result.Normalizations)
	}
	if !hasIssue(result.Normalizations, "fps_normalized") {
		t.Fatalf("expected fps normalization, got %+v", result.Normalizations)
	}
}

func TestGenerateValidationRejectsSourceVideoImageMix(t *testing.T) {
	registry := NewModelRegistry(NewGeminiProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:           ProviderGemini,
		Model:              "veo-3.1-fast-generate-preview",
		Prompt:             "Continue the motion",
		SourceVideoAssetID: "source-video",
		StartImageAssetID:  "start-frame",
	})
	if result.Valid {
		t.Fatalf("expected validation to fail")
	}
	if !hasIssue(result.Errors, "source_video_exclusive") {
		t.Fatalf("expected source_video_exclusive, got %+v", result.Errors)
	}
}

func TestGenerateValidationRejectsUnsupportedNegativePrompt(t *testing.T) {
	registry := NewModelRegistry(NewOpenRouterProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:        ProviderOpenRouter,
		Model:           "x-ai/grok-imagine-video",
		Prompt:          "A quick concept clip",
		NegativePrompt:  "text, artifacts",
		DurationSeconds: 4,
		Resolution:      "720p",
		AspectRatio:     "16:9",
		FPS:             24,
	})
	if result.Valid {
		t.Fatalf("expected validation to fail")
	}
	if !hasIssue(result.Errors, "negative_prompt_unsupported") {
		t.Fatalf("expected negative_prompt_unsupported, got %+v", result.Errors)
	}
}

func TestGenerateValidationRejectsOpenRouterNativeAudioOnSilentModel(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{"generate_audio": true})
	registry := NewModelRegistry(NewOpenRouterProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:        ProviderOpenRouter,
		Model:           "x-ai/grok-imagine-video",
		Prompt:          "A quick concept clip",
		Settings:        settings,
		DurationSeconds: 4,
		Resolution:      "720p",
		AspectRatio:     "16:9",
		FPS:             24,
	})
	if result.Valid {
		t.Fatalf("expected validation to fail")
	}
	if !hasIssue(result.Errors, "generate_audio_unsupported") {
		t.Fatalf("expected generate_audio_unsupported, got %+v", result.Errors)
	}
}

func TestGeminiBuildPayloadIncludesStartLastAndReferences(t *testing.T) {
	dir := t.TempDir()
	startPath := writeVideoValidationTestFile(t, dir, "start.png", []byte("start-image"))
	lastPath := writeVideoValidationTestFile(t, dir, "last.jpg", []byte("last-image"))
	refPath := writeVideoValidationTestFile(t, dir, "reference.webp", []byte("ref-image"))

	payload, err := NewGeminiProvider("", "").buildPayload(GenerateRequest{
		Model:               "veo-3.1-generate-preview",
		Prompt:              "A character turns toward camera",
		StartImagePath:      startPath,
		LastFramePath:       lastPath,
		ReferenceAssetPaths: []string{refPath},
		NegativePrompt:      "blur, warped hands",
		DurationSeconds:     6,
		Resolution:          "1080p",
		AspectRatio:         "16:9",
	})
	if err != nil {
		t.Fatalf("buildPayload returned error: %v", err)
	}
	instances := payload["instances"].([]map[string]any)
	instance := instances[0]
	if _, ok := instance["image"]; !ok {
		t.Fatalf("expected image in payload: %+v", instance)
	}
	if _, ok := instance["lastFrame"]; !ok {
		t.Fatalf("expected lastFrame in payload: %+v", instance)
	}
	refs, ok := instance["referenceImages"].([]map[string]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("expected one reference image, got %+v", instance["referenceImages"])
	}
	params := payload["parameters"].(map[string]any)
	if params["durationSeconds"] != 8 {
		t.Fatalf("expected durationSeconds normalized to 8, got %+v", params["durationSeconds"])
	}
	if params["negativePrompt"] != "blur, warped hands" {
		t.Fatalf("expected negativePrompt in payload, got %+v", params["negativePrompt"])
	}
}

func TestGeminiBuildPayloadSourceVideoForces720p(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeVideoValidationTestFile(t, dir, "source.mp4", []byte("mp4-bytes"))

	payload, err := NewGeminiProvider("", "").buildPayload(GenerateRequest{
		Model:           "veo-3.1-generate-preview",
		Prompt:          "Continue the clip",
		SourceVideoPath: sourcePath,
		DurationSeconds: 4,
		Resolution:      "4k",
		AspectRatio:     "9:16",
	})
	if err != nil {
		t.Fatalf("buildPayload returned error: %v", err)
	}
	instances := payload["instances"].([]map[string]any)
	if _, ok := instances[0]["video"]; !ok {
		t.Fatalf("expected video in payload: %+v", instances[0])
	}
	params := payload["parameters"].(map[string]any)
	if params["resolution"] != "720p" {
		t.Fatalf("expected source video resolution normalized to 720p, got %+v", params["resolution"])
	}
	if params["durationSeconds"] != 8 {
		t.Fatalf("expected source video duration normalized to 8, got %+v", params["durationSeconds"])
	}
}

func hasIssue(issues []GenerationValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func writeVideoValidationTestFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}
