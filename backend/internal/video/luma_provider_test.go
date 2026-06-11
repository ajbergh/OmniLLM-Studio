package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func shortenLumaPollInterval(t *testing.T) {
	t.Helper()
	previous := lumaPollInterval
	lumaPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { lumaPollInterval = previous })
}

func TestLumaBuildPayloadRay2(t *testing.T) {
	provider := NewLumaProvider("", "key")
	payload := provider.buildPayload(GenerateRequest{
		Model:           "ray-2",
		Prompt:          "A drone shot over a fjord",
		AspectRatio:     "16:9",
		Resolution:      "1080p",
		DurationSeconds: 8,
	})
	if payload["model"] != "ray-2" {
		t.Errorf("model = %v", payload["model"])
	}
	if payload["aspect_ratio"] != "16:9" {
		t.Errorf("aspect_ratio = %v", payload["aspect_ratio"])
	}
	if payload["resolution"] != "1080p" {
		t.Errorf("resolution = %v", payload["resolution"])
	}
	if payload["duration"] != "9s" {
		t.Errorf("duration should round 8 up to 9s, got %v", payload["duration"])
	}
	prompt, _ := payload["prompt"].(string)
	if !strings.Contains(prompt, "drone shot") {
		t.Errorf("prompt missing content: %q", prompt)
	}
}

func TestLumaBuildPayloadLegacyModelOmitsRay2Params(t *testing.T) {
	provider := NewLumaProvider("", "key")
	payload := provider.buildPayload(GenerateRequest{
		Model:           "ray-1-6",
		Prompt:          "City timelapse",
		AspectRatio:     "9:16",
		Resolution:      "720p",
		DurationSeconds: 5,
	})
	if _, exists := payload["resolution"]; exists {
		t.Errorf("ray-1-6 should not send resolution")
	}
	if _, exists := payload["duration"]; exists {
		t.Errorf("ray-1-6 should not send duration")
	}
	if payload["aspect_ratio"] != "9:16" {
		t.Errorf("aspect_ratio = %v", payload["aspect_ratio"])
	}
}

func TestLumaDurationParamRounding(t *testing.T) {
	cases := map[int]string{1: "5s", 5: "5s", 6: "5s", 7: "9s", 9: "9s", 30: "9s"}
	for input, expected := range cases {
		if got := lumaDurationParam(input); got != expected {
			t.Errorf("lumaDurationParam(%d) = %s, want %s", input, got, expected)
		}
	}
}

func TestLumaGenerateSubmitPollDownload(t *testing.T) {
	shortenLumaPollInterval(t)
	var pollCount int32
	videoBytes := []byte("fake-mp4-bytes")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/generations":
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
				t.Errorf("unexpected auth header %q", auth)
			}
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["duration"] != "5s" {
				t.Errorf("expected duration 5s in submit payload, got %v", payload["duration"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-123", "state": "queued"})
		case r.Method == http.MethodGet && r.URL.Path == "/generations/gen-123":
			count := atomic.AddInt32(&pollCount, 1)
			if count < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-123", "state": "dreaming"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    "gen-123",
				"state": "completed",
				"assets": map[string]any{
					"video": "http://" + r.Host + "/cdn/output.mp4",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/cdn/output.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(videoBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewLumaProvider(server.URL, "test-key")
	var stages []string
	result, err := provider.Generate(context.Background(), GenerateRequest{
		Model:           "ray-2",
		Prompt:          "Ocean waves at golden hour",
		AspectRatio:     "16:9",
		Resolution:      "720p",
		DurationSeconds: 5,
	}, func(p GenerationProgress) {
		stages = append(stages, p.Stage)
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if string(result.Data) != string(videoBytes) {
		t.Errorf("unexpected video bytes: %q", result.Data)
	}
	if result.MimeType != "video/mp4" {
		t.Errorf("mime type = %s", result.MimeType)
	}
	if result.UpstreamJobID == nil || *result.UpstreamJobID != "gen-123" {
		t.Errorf("upstream job id = %v", result.UpstreamJobID)
	}
	if result.DurationMS == nil || *result.DurationMS != 5000 {
		t.Errorf("duration = %v", result.DurationMS)
	}
	if len(stages) == 0 {
		t.Errorf("expected progress stages")
	}
}

func TestLumaGenerateFailedStateMapsError(t *testing.T) {
	shortenLumaPollInterval(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/generations":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-err", "state": "queued"})
		case r.Method == http.MethodGet && r.URL.Path == "/generations/gen-err":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "gen-err",
				"state":          "failed",
				"failure_reason": "content policy violation",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewLumaProvider(server.URL, "test-key")
	_, err := provider.Generate(context.Background(), GenerateRequest{Model: "ray-2", Prompt: "test"}, nil)
	if err == nil || !strings.Contains(err.Error(), "content policy violation") {
		t.Fatalf("expected failure_reason in error, got %v", err)
	}
}

func TestLumaGenerateRequiresAPIKey(t *testing.T) {
	provider := NewLumaProvider("", "")
	_, err := provider.Generate(context.Background(), GenerateRequest{Model: "ray-2", Prompt: "test"}, nil)
	if err == nil || !strings.Contains(err.Error(), "Luma provider profile") {
		t.Fatalf("expected configuration error, got %v", err)
	}
}

func TestLumaValidationRejectsImageInputs(t *testing.T) {
	registry := NewModelRegistry(NewLumaProvider("", "key"))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:          ProviderLuma,
		Model:             "ray-2",
		Prompt:            "A test prompt",
		AspectRatio:       "16:9",
		Resolution:        "720p",
		DurationSeconds:   5,
		StartImageAssetID: "asset-1",
	})
	if result.Valid {
		t.Fatalf("start frame should be rejected for Luma models: %+v", result)
	}
	found := false
	for _, issue := range result.Errors {
		if issue.Code == "image_to_video_unsupported" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected image_to_video_unsupported error, got %+v", result.Errors)
	}
}

func TestLumaValidationNormalizesDuration(t *testing.T) {
	registry := NewModelRegistry(NewLumaProvider("", "key"))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:        ProviderLuma,
		Model:           "ray-2",
		Prompt:          "A test prompt",
		AspectRatio:     "16:9",
		Resolution:      "720p",
		DurationSeconds: 30,
	})
	if !result.Valid {
		t.Fatalf("expected valid result, got errors %+v", result.Errors)
	}
	if result.NormalizedRequest.DurationSeconds != 9 {
		t.Errorf("duration should cap at 9 for ray-2, got %d", result.NormalizedRequest.DurationSeconds)
	}
	if len(result.Normalizations) == 0 {
		t.Errorf("expected a duration normalization entry")
	}
}

func TestNormalizeProviderLuma(t *testing.T) {
	if NormalizeProvider("Luma") != ProviderLuma {
		t.Errorf("NormalizeProvider should accept luma")
	}
}

func TestDownloadWithRetryRecoversFromTransientFailure(t *testing.T) {
	previousWait := downloadRetryBaseWait
	downloadRetryBaseWait = time.Millisecond
	t.Cleanup(func() { downloadRetryBaseWait = previousWait })
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			http.Error(w, "upstream hiccup", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("ok-bytes"))
	}))
	defer server.Close()

	data, mimeType, err := downloadWithRetry(context.Background(), server.Client(), server.URL, "Test", nil)
	if err != nil {
		t.Fatalf("expected retry to recover, got %v", err)
	}
	if string(data) != "ok-bytes" || mimeType != "video/mp4" {
		t.Errorf("unexpected result %q %q", data, mimeType)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDownloadWithRetryDoesNotRetryClientErrors(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, _, err := downloadWithRetry(context.Background(), server.Client(), server.URL, "Test", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("404 should not retry, got %d attempts", attempts)
	}
}
