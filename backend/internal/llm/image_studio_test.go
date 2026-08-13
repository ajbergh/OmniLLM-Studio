package llm

import "testing"

func TestResolveImageStudioGeometryPreservesEditSource(t *testing.T) {
	tests := []struct {
		provider string
		wantSize string
	}{
		{provider: "openai", wantSize: "auto"},
		{provider: "gemini", wantSize: ""},
		{provider: "openrouter", wantSize: ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got, err := resolveImageStudioGeometry(tt.provider, "edit", "", ImageGeometry{})
			if err != nil {
				t.Fatalf("resolveImageStudioGeometry: %v", err)
			}
			if got.Mode != ImageGeometryPreserveSource {
				t.Fatalf("mode = %q, want %q", got.Mode, ImageGeometryPreserveSource)
			}
			if got.LegacySize != tt.wantSize {
				t.Fatalf("legacy size = %q, want %q", got.LegacySize, tt.wantSize)
			}
		})
	}
}

func TestResolveImageStudioGeometryExplicit(t *testing.T) {
	got, err := resolveImageStudioGeometry("openrouter", "edit", "1536x1024", ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatalf("resolveImageStudioGeometry: %v", err)
	}
	if got.Size != "1536x1024" || got.AspectRatio != "3:2" {
		t.Fatalf("resolution = %#v, want 1536x1024 / 3:2", got)
	}
}

func TestResolveImageStudioGeometryRejectsInvalidExplicitSize(t *testing.T) {
	if _, err := resolveImageStudioGeometry("gemini", "edit", "banana", ImageGeometry{Mode: ImageGeometryExplicit}); err == nil {
		t.Fatal("expected invalid explicit size to fail")
	}
}

func TestNormalizeImageStudioModelMigratesRetiredImageModels(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		want     string
	}{
		{"gemini", "gemini-3.1-flash-image-preview", "gemini-3.1-flash-image"},
		{"gemini", "gemini-3-pro-image-preview", "gemini-3-pro-image"},
		{"gemini", "imagen-4.0-generate-001", "gemini-3.1-flash-image"},
		{"gemini", "imagen-4.0-ultra-generate-001", "gemini-3.1-flash-image"},
		{"gemini", "imagen-4.0-fast-generate-001", "gemini-3.1-flash-image"},
		{"openrouter", "google/gemini-3.1-flash-image-preview", "google/gemini-3.1-flash-image"},
		{"openrouter", "google/gemini-3-pro-image-preview", "google/gemini-3-pro-image"},
		{"openrouter", "openai/gpt-5-image", "openai/gpt-5-image"},
	}
	for _, tt := range tests {
		if got := normalizeImageStudioModel(tt.provider, tt.model); got != tt.want {
			t.Fatalf("normalizeImageStudioModel(%q, %q) = %q, want %q", tt.provider, tt.model, got, tt.want)
		}
	}
}

func TestBuildOpenRouterStudioImageBodyUsesPortableGeometry(t *testing.T) {
	seed := 42
	req := ImageStudioRequest{
		ImageRequest: ImageRequest{
			Prompt:  "watercolor cabin",
			Size:    "1536x1024",
			N:       2,
			Quality: "high",
			ReferenceImage: &ReferenceImage{
				Data:     "YWJj",
				MimeType: "image/png",
			},
		},
		Seed: &seed,
	}
	geometry, err := resolveImageStudioGeometry("openrouter", "edit", req.Size, ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatal(err)
	}
	body := buildOpenRouterStudioImageBody("openai/gpt-image-1", req, geometry)
	if _, exists := body["size"]; exists {
		t.Fatalf("OpenRouter body must not send explicit pixel size with aspect ratio: %#v", body)
	}
	if body["aspect_ratio"] != "3:2" {
		t.Fatalf("unexpected geometry body: %#v", body)
	}
	if body["seed"] != 42 || body["n"] != 2 || body["quality"] != "high" {
		t.Fatalf("advanced fields missing: %#v", body)
	}
	refs, ok := body["input_references"].([]map[string]interface{})
	if !ok || len(refs) != 1 {
		t.Fatalf("input_references = %#v", body["input_references"])
	}
}

func TestBuildOpenRouterStudioMinimalBodyDropsOptionalControls(t *testing.T) {
	seed := 42
	req := ImageStudioRequest{
		ImageRequest: ImageRequest{Prompt: "watercolor cabin", N: 2, Quality: "high"},
		Seed:         &seed,
	}
	body := buildOpenRouterStudioMinimalBody("google/gemini-2.5-flash-image", req)
	if body["model"] != "google/gemini-2.5-flash-image" || body["prompt"] != req.Prompt {
		t.Fatalf("required fields missing: %#v", body)
	}
	for _, key := range []string{"n", "quality", "seed", "size", "aspect_ratio"} {
		if _, exists := body[key]; exists {
			t.Fatalf("minimal body unexpectedly contains %s: %#v", key, body)
		}
	}
}

func TestBuildTogetherStudioImageBodyUsesDocumentedControls(t *testing.T) {
	seed := 7
	guidance := 4.5
	req := ImageStudioRequest{
		ImageRequest: ImageRequest{Prompt: "mountains", N: 3, Size: "1024x768"},
		Seed:         &seed,
		Guidance:     &guidance,
	}
	geometry, err := resolveImageStudioGeometry("together", "generate", req.Size, ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatal(err)
	}
	body, err := buildTogetherStudioImageBody("black-forest-labs/FLUX.1-pro", req, geometry)
	if err != nil {
		t.Fatal(err)
	}
	if body["width"] != 1024 || body["height"] != 768 {
		t.Fatalf("width/height missing: %#v", body)
	}
	if body["seed"] != 7 || body["guidance_scale"] != 4.5 || body["n"] != 3 {
		t.Fatalf("advanced fields missing: %#v", body)
	}
}

func TestBuildTogetherStudioImageBodyUsesAspectRatioForSchnell(t *testing.T) {
	req := ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "mountains", Size: "1344x768"}}
	geometry, err := resolveImageStudioGeometry("together", "generate", req.Size, ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatal(err)
	}
	body, err := buildTogetherStudioImageBody("black-forest-labs/FLUX.1-schnell", req, geometry)
	if err != nil {
		t.Fatal(err)
	}
	if body["aspect_ratio"] != "7:4" {
		t.Fatalf("aspect_ratio = %#v, want 7:4", body["aspect_ratio"])
	}
	if _, ok := body["width"]; ok {
		t.Fatalf("Schnell body should not include width: %#v", body)
	}
}
