package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBuildGeminiStudioImageBodyUsesImageConfigGeometry(t *testing.T) {
	req := ImageStudioRequest{
		ImageRequest: ImageRequest{
			Prompt: "edit the room",
			N:      4,
			ReferenceImage: &ReferenceImage{
				Data:     "YmFzZQ==",
				MimeType: "image/jpeg",
			},
			MaskImage: &ReferenceImage{
				Data:     "bWFzaw==",
				MimeType: "image/png",
			},
			ReferenceImages:      []ReferenceImage{{Data: "cmVm", MimeType: "image/webp"}},
			StyleReferenceImages: []ReferenceImage{{Data: "c3R5bGU=", MimeType: "image/png"}},
		},
	}
	geometry := imageStudioGeometryResolution{
		Mode:        ImageGeometryExplicit,
		Size:        "1536x1024",
		AspectRatio: "3:2",
	}

	body := buildGeminiStudioImageBody(req, geometry)
	generationConfig, ok := body["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %#v", body["generationConfig"])
	}
	if _, exists := generationConfig["responseFormat"]; exists {
		t.Fatalf("protobuf enum responseFormat must not be serialized: %#v", generationConfig)
	}
	if _, exists := generationConfig["candidateCount"]; exists {
		t.Fatalf("candidateCount must not be serialized for Image Studio: %#v", generationConfig)
	}
	imageConfig, ok := generationConfig["imageConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig.imageConfig = %#v", generationConfig["imageConfig"])
	}
	if imageConfig["aspectRatio"] != "3:2" {
		t.Fatalf("aspectRatio = %#v, want 3:2", imageConfig["aspectRatio"])
	}
	if _, exists := imageConfig["imageSize"]; exists {
		t.Fatalf("pixel dimensions must not be sent as a Gemini imageSize tier: %#v", imageConfig)
	}

	contents, ok := body["contents"].([]map[string]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("contents = %#v", body["contents"])
	}
	parts, ok := contents[0]["parts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("parts = %#v", contents[0]["parts"])
	}
	if len(parts) != 7 {
		t.Fatalf("parts count = %d, want 7 (prompt, base, ref, mask guidance+mask, style guidance+style)", len(parts))
	}
	inlineCount := 0
	for _, part := range parts {
		if _, ok := part["inlineData"].(map[string]interface{}); ok {
			inlineCount++
		}
	}
	if inlineCount != 4 {
		t.Fatalf("inlineData count = %d, want 4", inlineCount)
	}
}

func TestBuildGeminiStudioImageBodyOmitsImageConfigForInferredGeometry(t *testing.T) {
	for _, mode := range []ImageGeometryMode{ImageGeometryProviderAuto, ImageGeometryPreserveSource} {
		t.Run(string(mode), func(t *testing.T) {
			body := buildGeminiStudioImageBody(
				ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "preserve the room"}},
				imageStudioGeometryResolution{Mode: mode},
			)
			generationConfig := body["generationConfig"].(map[string]interface{})
			if _, exists := generationConfig["imageConfig"]; exists {
				t.Fatalf("imageConfig must be omitted for %s: %#v", mode, generationConfig)
			}
		})
	}
}

func TestGeminiStudioImageGenerateFansOutVariants(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("x-goog-api-key"); got != "secret" {
			t.Errorf("x-goog-api-key = %q, want secret", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		generationConfig, _ := body["generationConfig"].(map[string]interface{})
		if _, exists := generationConfig["candidateCount"]; exists {
			t.Errorf("candidateCount unexpectedly serialized: %#v", generationConfig)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []interface{}{
				map[string]interface{}{
					"content": map[string]interface{}{
						"parts": []interface{}{
							map[string]interface{}{
								"inlineData": map[string]interface{}{
									"mimeType": "image/png",
									"data":     "aW1hZ2U=",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	service := &Service{httpClient: server.Client()}
	response, err := service.geminiStudioImageGenerate(
		context.Background(),
		server.URL,
		"secret",
		"gemini-3.1-flash-image",
		ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "three rooms", N: 3}},
		imageStudioGeometryResolution{Mode: ImageGeometryProviderAuto},
	)
	if err != nil {
		t.Fatalf("geminiStudioImageGenerate: %v", err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("request count = %d, want 3", got)
	}
	if len(response.Images) != 3 {
		t.Fatalf("image count = %d, want 3", len(response.Images))
	}
}

func TestGeminiStudioImageGenerateSurfacesProviderErrors(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "provider rejected request", status)
			}))
			defer server.Close()

			service := &Service{httpClient: server.Client()}
			_, err := service.geminiStudioImageGenerate(
				context.Background(),
				server.URL,
				"secret",
				"gemini-3.1-flash-image",
				ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "room", N: 1}},
				imageStudioGeometryResolution{Mode: ImageGeometryProviderAuto},
			)
			if err == nil {
				t.Fatal("expected provider error")
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("status %d", status)) {
				t.Fatalf("error = %q, want status %d", err, status)
			}
		})
	}
}
