package video

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKnownGeminiVideoModelsIncludesOmni(t *testing.T) {
	models := KnownGeminiVideoModels()
	if len(models) < 2 || models[0].ID != "gemini-omni-flash-preview" {
		t.Fatalf("expected Omni to lead the Google video model list, got %#v", models)
	}
	if !hasCapability(models[0].Capabilities, CapabilityVideoToVideo) || models[0].MaxReferenceImages != 6 {
		t.Fatalf("Omni capability metadata is incomplete: %#v", models[0])
	}
}

func TestOmniValidationRequiresTaskInputs(t *testing.T) {
	registry := NewModelRegistry(NewGeminiProvider("", ""))
	result := registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:       ProviderGemini,
		Model:          "gemini-omni-flash-preview",
		Prompt:         "Make the subject wave",
		GenerationMode: "edit",
	})
	if result.Valid || len(result.Errors) == 0 || result.Errors[0].Code != "omni_edit_source_required" {
		t.Fatalf("expected missing edit context error, got %#v", result)
	}

	result = registry.ValidateGenerateRequest(context.Background(), GenerateRequest{
		Provider:       ProviderGemini,
		Model:          "gemini-omni-flash-preview",
		Prompt:         "Make the subject wave",
		GenerationMode: "edit",
		ParentID:       "local-generation-id",
	})
	if !result.Valid {
		t.Fatalf("expected parent-backed edit request to validate, got %#v", result.Errors)
	}
}

func TestGenerateOmniUsesInteractionsAndExtractsInlineVideo(t *testing.T) {
	videoBytes := []byte("not-a-real-mp4-but-valid-provider-bytes")
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/interactions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatal("missing API key header")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "v1_interaction-123",
			"status": "completed",
			"steps": []any{
				map[string]any{"type": "model_output", "content": []any{
					map[string]any{"type": "video", "mime_type": "video/mp4", "data": base64.StdEncoding.EncodeToString(videoBytes)},
				}},
			},
		})
	}))
	defer server.Close()

	provider := NewGeminiProvider(server.URL, "test-key")
	result, err := provider.GenerateOmni(context.Background(), GenerateRequest{
		Model:                 "gemini-omni-flash-preview",
		Prompt:                "Make the lighting dramatic",
		GenerationMode:        "edit",
		PreviousInteractionID: "v1_previous",
		AspectRatio:           "9:16",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Data) != string(videoBytes) || result.UpstreamJobID == nil || *result.UpstreamJobID != "v1_interaction-123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if received["previous_interaction_id"] != "v1_previous" || received["store"] != true {
		t.Fatalf("stateful interaction fields missing: %#v", received)
	}
	format := received["response_format"].(map[string]any)
	if format["delivery"] != "uri" || format["aspect_ratio"] != "9:16" {
		t.Fatalf("unexpected response format: %#v", format)
	}
}

func TestBuildOmniPromptAssignsImageRoles(t *testing.T) {
	prompt := buildOmniPrompt(GenerateRequest{
		Prompt:              "A designer presents a product",
		GenerationMode:      "reference_to_video",
		StartImagePath:      "start.png",
		ReferenceAssetPaths: []string{"person.png", "product.png"},
	})
	for _, expected := range []string{"<FIRST_FRAME>@Image1", "<IMAGE_REF_0>@Image2", "<IMAGE_REF_1>@Image3"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q did not contain %q", prompt, expected)
		}
	}
}
