package llm

import "testing"

func TestOllamaAPIRoot(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "default with v1", baseURL: "http://localhost:11434/v1", want: "http://localhost:11434"},
		{name: "default no suffix", baseURL: "http://localhost:11434", want: "http://localhost:11434"},
		{name: "trailing slash", baseURL: "http://localhost:11434/v1/", want: "http://localhost:11434"},
		{name: "nested path", baseURL: "http://localhost:11434/custom/v1", want: "http://localhost:11434/custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ollamaAPIRoot(tt.baseURL)
			if got != tt.want {
				t.Fatalf("ollamaAPIRoot(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestOllamaModelNameMatches(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		requested string
		want      bool
	}{
		{name: "exact", installed: "nomic-embed-text", requested: "nomic-embed-text", want: true},
		{name: "installed latest", installed: "nomic-embed-text:latest", requested: "nomic-embed-text", want: true},
		{name: "requested latest", installed: "nomic-embed-text", requested: "nomic-embed-text:latest", want: true},
		{name: "different model", installed: "all-minilm", requested: "nomic-embed-text", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ollamaModelNameMatches(tt.installed, tt.requested)
			if got != tt.want {
				t.Fatalf("ollamaModelNameMatches(%q, %q) = %v, want %v", tt.installed, tt.requested, got, tt.want)
			}
		})
	}
}

func TestOpenRouterEffectiveImageCapabilities(t *testing.T) {
	providerCaps := GetImageCapabilities("openrouter")
	if providerCaps.SupportsEditing {
		t.Fatal("openrouter provider default should stay generation-only unless a selected model is known to edit")
	}

	knownEditModels := []string{
		"google/gemini-2.5-flash-image",
		"google/gemini-3.1-flash-image-preview",
		"google/gemini-3-pro-image-preview",
		"openai/gpt-5.4-image-2",
		"openai/gpt-5-image",
		"openai/gpt-5-image-mini",
		"black-forest-labs/flux.2-pro",
		"black-forest-labs/flux.2-max",
		"black-forest-labs/flux.2-flex",
		"black-forest-labs/flux.2-klein-4b",
		"recraft/recraft-v3",
		"recraft/recraft-v4",
		"recraft/recraft-v4-pro",
		"sourceful/riverflow-v2-fast",
		"sourceful/riverflow-v2-fast-preview",
		"sourceful/riverflow-v2-pro",
		"sourceful/riverflow-v2-max-preview",
		"sourceful/riverflow-v2-standard-preview",
		"bytedance-seed/seedream-4.5",
	}

	for _, model := range knownEditModels {
		t.Run(model, func(t *testing.T) {
			caps := GetEffectiveImageCapabilities("openrouter", model)
			if !caps.SupportsEditing {
				t.Fatalf("expected %s to support OpenRouter image editing", model)
			}
			if !caps.SupportsContentReference {
				t.Fatalf("expected %s to accept an input/base image", model)
			}
			if caps.SupportsMasking {
				t.Fatalf("expected %s masking to remain disabled until OpenRouter exposes a mask contract", model)
			}
		})
	}

	unknown := GetEffectiveImageCapabilities("openrouter", "openrouter/auto")
	if unknown.SupportsEditing {
		t.Fatal("openrouter/auto should not enable editing without a concrete model capability")
	}
}

func TestOpenRouterImageContentIncludesReferenceImage(t *testing.T) {
	req := ImageRequest{
		Prompt: "make the sky sunset orange",
		ReferenceImage: &ReferenceImage{
			Data:     "abc123",
			MimeType: "image/png",
		},
	}

	content, ok := openrouterImageContent(req).([]map[string]interface{})
	if !ok {
		t.Fatal("expected multimodal content array when reference image is present")
	}
	if len(content) != 2 {
		t.Fatalf("expected text plus image content, got %d parts", len(content))
	}
	if content[0]["type"] != "text" || content[0]["text"] != req.Prompt {
		t.Fatalf("unexpected first content part: %#v", content[0])
	}
	imageURL, ok := content[1]["image_url"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image_url content part, got %#v", content[1])
	}
	if imageURL["url"] != "data:image/png;base64,abc123" {
		t.Fatalf("unexpected image data URL: %#v", imageURL["url"])
	}
}

func TestOpenRouterImageContentLeavesTextOnlyRequestsAsString(t *testing.T) {
	req := ImageRequest{Prompt: "generate a product photo"}

	content, ok := openrouterImageContent(req).(string)
	if !ok {
		t.Fatalf("expected text-only content string, got %T", openrouterImageContent(req))
	}
	if content != req.Prompt {
		t.Fatalf("content = %q, want %q", content, req.Prompt)
	}
}

func TestGeminiMusicRequestBodyForProDoesNotForceResponseFormat(t *testing.T) {
	temperature := 0.7
	req := MusicRequest{
		Prompt: "An atmospheric ambient track.",
		Options: MusicOptions{
			Temperature: &temperature,
		},
	}

	body := geminiMusicRequestBody(req)
	genConfig, ok := body["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %T, want map[string]interface{}", body["generationConfig"])
	}
	if _, ok := genConfig["responseFormat"]; ok {
		t.Fatal("expected responseFormat to be omitted for Gemini Lyria Pro payload")
	}
	modalities, ok := genConfig["responseModalities"].([]string)
	if !ok {
		t.Fatalf("responseModalities = %T, want []string", genConfig["responseModalities"])
	}
	if len(modalities) != 2 || modalities[0] != "AUDIO" || modalities[1] != "TEXT" {
		t.Fatalf("responseModalities = %#v, want [AUDIO TEXT]", modalities)
	}
	if got := genConfig["temperature"]; got != temperature {
		t.Fatalf("temperature = %#v, want %v", got, temperature)
	}

	contents, ok := body["contents"].([]map[string]interface{})
	if !ok {
		t.Fatalf("contents = %T, want []map[string]interface{}", body["contents"])
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(contents))
	}
	parts, ok := contents[0]["parts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("parts = %T, want []map[string]interface{}", contents[0]["parts"])
	}
	if len(parts) != 1 || parts[0]["text"] != req.Prompt {
		t.Fatalf("unexpected text payload: %#v", parts)
	}
}

func TestGeminiMusicRequestBodyForClipDoesNotForceResponseFormat(t *testing.T) {
	req := MusicRequest{
		Prompt: "A bright 30-second lo-fi loop with mellow Rhodes and vinyl crackle.",
	}

	body := geminiMusicRequestBody(req)
	genConfig, ok := body["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("generationConfig = %T, want map[string]interface{}", body["generationConfig"])
	}
	if _, ok := genConfig["responseFormat"]; ok {
		t.Fatal("expected responseFormat to be omitted for Gemini Lyria Clip payload")
	}
	modalities, ok := genConfig["responseModalities"].([]string)
	if !ok {
		t.Fatalf("responseModalities = %T, want []string", genConfig["responseModalities"])
	}
	if len(modalities) != 2 || modalities[0] != "AUDIO" || modalities[1] != "TEXT" {
		t.Fatalf("responseModalities = %#v, want [AUDIO TEXT]", modalities)
	}

	contents, ok := body["contents"].([]map[string]interface{})
	if !ok {
		t.Fatalf("contents = %T, want []map[string]interface{}", body["contents"])
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(contents))
	}
	parts, ok := contents[0]["parts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("parts = %T, want []map[string]interface{}", contents[0]["parts"])
	}
	if len(parts) != 1 || parts[0]["text"] != req.Prompt {
		t.Fatalf("unexpected text payload: %#v", parts)
	}
}
