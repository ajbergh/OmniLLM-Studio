package music

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsOpenRouterMusicModel(t *testing.T) {
	tests := []struct {
		id          string
		name        string
		description string
		want        bool
	}{
		{
			id:          "google/lyria-3-clip-preview",
			name:        "Google: Lyria 3 Clip Preview",
			description: "30 second duration clips are priced at $0.04 per clip.",
			want:        true,
		},
		{
			id:          "google/lyria-3-pro-preview",
			name:        "Google: Lyria 3 Pro Preview",
			description: "Full-length songs are priced at $0.08 per song. Lyria 3 is Google's family of music generation models.",
			want:        true,
		},
		{
			id:          "suno/v3.5",
			name:        "Suno v3.5",
			description: "Text to music generation model",
			want:        true,
		},
		{
			id:          "stabilityai/stable-audio-open-1.0",
			name:        "Stability AI: Stable Audio Open 1.0",
			description: "Generate music and sound effects",
			want:        true,
		},
		{
			id:          "openai/gpt-audio",
			name:        "OpenAI: GPT Audio",
			description: "The gpt-audio model is OpenAI's first generally available audio model with natural sounding voices.",
			want:        false,
		},
		{
			id:          "openai/gpt-audio-mini",
			name:        "OpenAI: GPT Audio Mini",
			description: "A cost-efficient version of GPT Audio for voice consistency.",
			want:        false,
		},
		{
			id:          "mistralai/voxtral-small-24b-2507",
			name:        "Mistral: Voxtral Small",
			description: "Audio input transcription and audio understanding",
			want:        false,
		},
		{
			id:          "meta/audioldm-2",
			name:        "Meta: AudioLDM 2",
			description: "Audio generation model for music and sound",
			want:        true,
		},
	}

	for _, tt := range tests {
		got := IsOpenRouterMusicModel(tt.id, tt.name, tt.description)
		if got != tt.want {
			t.Errorf("IsOpenRouterMusicModel(%q, %q, %q) = %v; want %v", tt.id, tt.name, tt.description, got, tt.want)
		}
	}
}

func TestDiscoverOpenRouterModels(t *testing.T) {
	mockResponse := map[string]any{
		"data": []map[string]any{
			{
				"id":   "google/lyria-3-clip-preview",
				"name": "Google: Lyria 3 Clip Preview",
				"architecture": map[string]any{
					"input_modalities":  []string{"text", "image"},
					"output_modalities": []string{"text", "audio"},
				},
				"description": "30 second duration clips are priced at $0.04 per clip.",
				"pricing": map[string]string{
					"request": "0.04",
				},
			},
			{
				"id":   "google/lyria-3-pro-preview",
				"name": "Google: Lyria 3 Pro Preview",
				"architecture": map[string]any{
					"input_modalities":  []string{"text", "image"},
					"output_modalities": []string{"text", "audio"},
				},
				"description": "Full-length songs are priced at $0.08 per song.",
				"pricing": map[string]string{
					"request": "0.08",
				},
			},
			{
				"id":   "openai/gpt-audio",
				"name": "OpenAI: GPT Audio",
				"architecture": map[string]any{
					"input_modalities":  []string{"text", "audio"},
					"output_modalities": []string{"text", "audio"},
				},
				"description": "Voice output model from OpenAI.",
			},
			{
				"id":   "suno/bark-music-v2",
				"name": "Suno Bark Music v2",
				"architecture": map[string]any{
					"input_modalities":  []string{"text"},
					"output_modalities": []string{"audio"},
				},
				"description": "Music generation and song synthesizer.",
			},
			{
				"id":   "text-only/model",
				"name": "Text Only Model",
				"architecture": map[string]any{
					"input_modalities":  []string{"text"},
					"output_modalities": []string{"text"},
				},
				"description": "Just a text model about music theory.",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	registry := NewModelRegistry()
	models, err := registry.discoverOpenRouter(context.Background(), server.URL, "fake-key")
	if err != nil {
		t.Fatalf("discoverOpenRouter returned error: %v", err)
	}

	// Should contain Lyria Clip, Lyria Pro, and Suno Bark Music, but NOT GPT Audio or Text Only
	expectedIDs := map[string]bool{
		"google/lyria-3-clip-preview": true,
		"google/lyria-3-pro-preview":  true,
		"suno/bark-music-v2":          true,
	}

	if len(models) != len(expectedIDs) {
		t.Fatalf("expected %d models, got %d: %+v", len(expectedIDs), len(models), models)
	}

	for _, m := range models {
		if !expectedIDs[m.ID] {
			t.Errorf("unexpected model discovered: %s", m.ID)
		}
		if m.Provider != ProviderOpenRouter {
			t.Errorf("expected provider openrouter, got %s", m.Provider)
		}
		if !m.SupportsStreaming {
			t.Errorf("expected supports_streaming to be true for openrouter model")
		}
	}
}
