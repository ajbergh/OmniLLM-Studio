package llm

import "testing"

func TestImageStudioGeometryRegressionMatrix(t *testing.T) {
	tests := []struct {
		size  string
		ratio string
	}{
		{size: "1024x1024", ratio: "1:1"},
		{size: "1536x1024", ratio: "3:2"},
		{size: "1024x1536", ratio: "2:3"},
		{size: "1024x768", ratio: "4:3"},
		{size: "768x1024", ratio: "3:4"},
		{size: "576x1024", ratio: "9:16"},
		{size: "1344x576", ratio: "7:3"},
		{size: "3072x384", ratio: "8:1"},
	}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			got, err := resolveImageStudioGeometry("gemini", "edit", tt.size, ImageGeometry{Mode: ImageGeometryExplicit})
			if err != nil {
				t.Fatalf("resolve explicit geometry: %v", err)
			}
			if got.AspectRatio != tt.ratio {
				t.Fatalf("aspect ratio = %q, want %q", got.AspectRatio, tt.ratio)
			}
		})
	}
}

func TestImageStudioPreserveSourceNeverInventsSquareGeometry(t *testing.T) {
	for _, provider := range []string{"openai", "gemini", "openrouter"} {
		t.Run(provider, func(t *testing.T) {
			got, err := resolveImageStudioGeometry(provider, "edit", "", ImageGeometry{Mode: ImageGeometryPreserveSource})
			if err != nil {
				t.Fatal(err)
			}
			if got.LegacySize == "1024x1024" || got.Size == "1024x1024" {
				t.Fatalf("preserve_source invented square geometry: %#v", got)
			}
		})
	}
}

func TestImageCapabilityRegressionMatrix(t *testing.T) {
	tests := []struct {
		name            string
		provider        string
		model           string
		wantGeneration  bool
		wantEditing     bool
		wantMaskMode    ImageMaskingMode
		wantSeed        bool
		wantGuidance    bool
		wantMaxRefs     int
		wantContentRefs bool
		wantStyleRefs   bool
	}{
		{
			name: "OpenAI GPT Image uses pixel masks", provider: "openai", model: "gpt-image-2",
			wantGeneration: true, wantEditing: true, wantMaskMode: ImageMaskingPixel,
		},
		{
			name: "DALL-E 3 remains generation only", provider: "openai", model: "dall-e-3",
			wantGeneration: true, wantEditing: false, wantMaskMode: ImageMaskingNone,
		},
		{
			name: "Gemini edit selection is semantic", provider: "gemini", model: "gemini-3.1-flash-image",
			wantGeneration: true, wantEditing: true, wantMaskMode: ImageMaskingSemantic,
			wantMaxRefs: 14, wantContentRefs: true, wantStyleRefs: true,
		},
		{
			name: "Gemini 2.5 has conservative reference limit", provider: "gemini", model: "gemini-2.5-flash-image",
			wantGeneration: true, wantEditing: true, wantMaskMode: ImageMaskingSemantic,
			wantMaxRefs: 3, wantContentRefs: true, wantStyleRefs: true,
		},
		{
			name: "Imagen 4 is text generation only", provider: "gemini", model: "imagen-4.0-generate-001",
			wantGeneration: true, wantEditing: false, wantMaskMode: ImageMaskingNone,
		},
		{
			name: "OpenRouter edits use references but no pixel mask claim", provider: "openrouter", model: "google/gemini-2.5-flash-image",
			wantGeneration: true, wantEditing: true, wantMaskMode: ImageMaskingNone,
			wantMaxRefs: 1, wantContentRefs: true,
		},
		{
			name: "Together exposes implemented advanced controls", provider: "together", model: "black-forest-labs/FLUX.1-schnell",
			wantGeneration: true, wantEditing: false, wantMaskMode: ImageMaskingNone,
			wantSeed: true, wantGuidance: true,
		},
		{
			name: "Stability is not advertised without a transport", provider: "stability", model: "stable-diffusion-xl-1024-v1-0",
			wantGeneration: false, wantEditing: false, wantMaskMode: ImageMaskingNone,
		},
		{
			name: "Unknown provider is not image capable", provider: "unknown-provider", model: "anything",
			wantGeneration: false, wantEditing: false, wantMaskMode: ImageMaskingNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEffectiveImageCapabilities(tt.provider, tt.model)
			if got.SupportsGeneration != tt.wantGeneration || got.SupportsEditing != tt.wantEditing {
				t.Fatalf("generation/edit = %v/%v, want %v/%v", got.SupportsGeneration, got.SupportsEditing, tt.wantGeneration, tt.wantEditing)
			}
			if got.MaskingMode != tt.wantMaskMode {
				t.Fatalf("masking mode = %q, want %q", got.MaskingMode, tt.wantMaskMode)
			}
			if got.SupportsSeed != tt.wantSeed || got.SupportsGuidance != tt.wantGuidance {
				t.Fatalf("seed/guidance = %v/%v, want %v/%v", got.SupportsSeed, got.SupportsGuidance, tt.wantSeed, tt.wantGuidance)
			}
			if got.MaxReferenceImages != tt.wantMaxRefs {
				t.Fatalf("max refs = %d, want %d", got.MaxReferenceImages, tt.wantMaxRefs)
			}
			if got.SupportsContentReference != tt.wantContentRefs || got.SupportsStyleReference != tt.wantStyleRefs {
				t.Fatalf("content/style refs = %v/%v, want %v/%v", got.SupportsContentReference, got.SupportsStyleReference, tt.wantContentRefs, tt.wantStyleRefs)
			}
		})
	}
}
