package video

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterProviderGeneratePollsAndDownloads(t *testing.T) {
	var submitSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download/job-1.mp4" && r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing bearer auth on %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/videos":
			submitSeen = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["model"] != "google/veo-3.1" || payload["prompt"] == "" {
				t.Fatalf("unexpected submit payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          "job-1",
				"polling_url": serverURL(r, "/generation?id=job-1"),
			})
		case "/generation":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "job-1",
				"status":        "completed",
				"unsigned_urls": []string{serverURL(r, "/download/job-1.mp4")},
				"usage":         map[string]any{"cost": 0.12},
			})
		case "/download/job-1.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("mp4-bytes"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewOpenRouterProvider(server.URL, "test-key")
	result, err := provider.Generate(context.Background(), GenerateRequest{
		Model:           "google/veo-3.1",
		Prompt:          "A cinematic coffee shop reveal",
		AspectRatio:     "16:9",
		Resolution:      "720p",
		DurationSeconds: 4,
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !submitSeen {
		t.Fatalf("expected submit endpoint to be called")
	}
	if string(result.Data) != "mp4-bytes" || result.MimeType != "video/mp4" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.CostUSD == nil || *result.CostUSD != 0.12 {
		t.Fatalf("expected usage cost, got %+v", result.CostUSD)
	}
}

func TestGeminiProviderGeneratePollsAndDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("missing Gemini API key on %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/models/veo-3.1-generate-preview:predictLongRunning":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if _, ok := payload["instances"].([]any); !ok {
				t.Fatalf("expected instances in payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "operations/op-1"})
		case "/operations/op-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "operations/op-1",
				"done": true,
				"response": map[string]any{
					"generateVideoResponse": map[string]any{
						"generatedSamples": []map[string]any{
							{"video": map[string]any{"uri": "files/video-1"}},
						},
					},
				},
			})
		case "/files/video-1:download":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("veo-bytes"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewGeminiProvider(server.URL, "test-key")
	result, err := provider.Generate(context.Background(), GenerateRequest{
		Model:           "veo-3.1-generate-preview",
		Prompt:          "A glowing neon sign in light rain",
		AspectRatio:     "16:9",
		Resolution:      "720p",
		DurationSeconds: 8,
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if string(result.Data) != "veo-bytes" || result.MimeType != "video/mp4" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.UpstreamJobID == nil || *result.UpstreamJobID != "operations/op-1" {
		t.Fatalf("expected operation name, got %+v", result.UpstreamJobID)
	}
}

func serverURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}
