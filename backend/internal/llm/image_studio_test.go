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

func TestBuildGeminiStudioImageBodyUsesCurrentResponseFormat(t *testing.T) {
	req := ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "watercolor cabin", Size: "1536x1024", N: 1}}
	geometry, err := resolveImageStudioGeometry("gemini", "generate", req.Size, ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatal(err)
	}
	body := buildGeminiStudioImageBody(req, geometry)
	generationConfig, ok := body["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %#v", body["generationConfig"])
	}
	if _, exists := generationConfig["imageConfig"]; exists {
		t.Fatalf("legacy imageConfig must not be sent: %#v", generationConfig)
	}
	responseFormat, ok := generationConfig["responseFormat"].(map[string]interface{})
	if !ok {
		t.Fatalf("responseFormat = %#v", generationConfig["responseFormat"])
	}
	imageFormat, ok := responseFormat["image"].(map[string]interface{})
	if !ok || imageFormat["aspectRatio"] != "3:2" {
		t.Fatalf("image response format = %#v", responseFormat["image"])
	}
}

func TestBuildGeminiStudioImageBodyProviderAutoOmitsGeometry(t *testing.T) {
	req := ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "watercolor cabin"}}
	geometry, err := resolveImageStudioGeometry("gemini", "generate", "", ImageGeometry{Mode: ImageGeometryProviderAuto})
	if err != nil {
		t.Fatal(err)
	}
	body := buildGeminiStudioImageBody(req, geometry)
	generationConfig := body["generationConfig"].(map[string]interface{})
	if _, exists := generationConfig["responseFormat"]; exists {
		t.Fatalf("provider_auto must not force image geometry: %#v", generationConfig)
	}
}

func TestBuildImagenStudioImageBodyUsesPredictParameters(t *testing.T) {
	req := ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "product scene", Size: "1024x768", N: 3}}
	geometry, err := resolveImageStudioGeometry("gemini", "generate", req.Size, ImageGeometry{Mode: ImageGeometryExplicit})
	if err != nil {
		t.Fatal(err)
	}
	body := buildImagenStudioImageBody(req, geometry)
	instances, ok := body["instances"].([]map[string]interface{})
	if !ok || len(instances) != 1 || instances[0]["prompt"] != req.Prompt {
		t.Fatalf("instances = %#v", body["instances"])
	}
	parameters, ok := body["parameters"].(map[string]interface{})
	if !ok || parameters["sampleCount"] != 3 || parameters["aspectRatio"] != "4:3" {
		t.Fatalf("parameters = %#v", body["parameters"])
	}
}

func TestImagenSupportsOnlyDocumentedAspectRatios(t *testing.T) {
	for _, ratio := range []string{"1:1", "3:4", "4:3", "9:16", "16:9"} {
		if !imagenSupportsAspectRatio(ratio) {
			t.Fatalf("expected %s to be supported", ratio)
		}
	}
	if imagenSupportsAspectRatio("3:2") {
		t.Fatal("Imagen 4 must not receive unsupported 3:2 aspect ratio")
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
