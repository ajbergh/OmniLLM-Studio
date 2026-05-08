package llm

import "strings"

// ImageCapabilities describes what image operations a provider/model supports.
type ImageCapabilities struct {
	SupportsGeneration       bool     `json:"supports_generation"`
	SupportsEditing          bool     `json:"supports_editing"`
	SupportsMasking          bool     `json:"supports_masking"`
	SupportsVariations       bool     `json:"supports_variations"`
	SupportsSeed             bool     `json:"supports_seed"`
	SupportsGuidance         bool     `json:"supports_guidance"`
	SupportsStyleReference   bool     `json:"supports_style_reference"`
	SupportsContentReference bool     `json:"supports_content_reference"`
	MaxReferenceImages       int      `json:"max_reference_images"`
	MaxVariants              int      `json:"max_variants"`
	SupportedSizes           []string `json:"supported_sizes"`
	ImageModels              []string `json:"image_models"`
	DefaultImageModel        string   `json:"default_image_model"`

	// Per-model overrides keyed by model name. Only fields that differ from the provider default need to be set.
	ModelOverrides map[string]ModelImageCapabilities `json:"model_overrides,omitempty"`
}

// ModelImageCapabilities holds per-model capability overrides.
type ModelImageCapabilities struct {
	SupportsEditing          *bool    `json:"supports_editing,omitempty"`
	SupportsMasking          *bool    `json:"supports_masking,omitempty"`
	SupportsContentReference *bool    `json:"supports_content_reference,omitempty"`
	MaxVariants              *int     `json:"max_variants,omitempty"`
	SupportedSizes           []string `json:"supported_sizes,omitempty"`
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// GetEffectiveImageCapabilities returns provider capabilities with any
// model-specific overrides applied.
func GetEffectiveImageCapabilities(providerType, model string) ImageCapabilities {
	caps := GetImageCapabilities(providerType)
	model = strings.TrimSpace(model)
	if model == "" || caps.ModelOverrides == nil {
		return caps
	}
	overrides, ok := caps.ModelOverrides[model]
	if !ok {
		return caps
	}
	if overrides.SupportsEditing != nil {
		caps.SupportsEditing = *overrides.SupportsEditing
	}
	if overrides.SupportsMasking != nil {
		caps.SupportsMasking = *overrides.SupportsMasking
	}
	if overrides.SupportsContentReference != nil {
		caps.SupportsContentReference = *overrides.SupportsContentReference
	}
	if overrides.MaxVariants != nil {
		caps.MaxVariants = *overrides.MaxVariants
	}
	if overrides.SupportedSizes != nil {
		caps.SupportedSizes = overrides.SupportedSizes
	}
	return caps
}

// GetImageCapabilities returns the capability matrix for a known provider type.
func GetImageCapabilities(providerType string) ImageCapabilities {
	switch providerType {
	case "openai-dall-e-3", "dall-e-3":
		return ImageCapabilities{
			SupportsGeneration: true,
			MaxVariants:        1,
			SupportedSizes:     []string{"1024x1024", "1792x1024", "1024x1792"},
			ImageModels:        []string{"gpt-image-2", "gpt-image-1.5", "chatgpt-image-latest", "gpt-image-1", "gpt-image-1-mini", "dall-e-3"},
			DefaultImageModel:  "gpt-image-2",
		}
	case "openai-dall-e-2", "dall-e-2":
		return ImageCapabilities{
			SupportsGeneration: true,
			SupportsEditing:    true,
			SupportsMasking:    true,
			SupportsVariations: true,
			MaxReferenceImages: 1,
			MaxVariants:        10,
			SupportedSizes:     []string{"256x256", "512x512", "1024x1024"},
			ImageModels:        []string{"dall-e-2"},
			DefaultImageModel:  "dall-e-2",
		}
	case "openai":
		return ImageCapabilities{
			SupportsGeneration:       true,
			SupportsEditing:          true,
			SupportsMasking:          true,
			SupportsContentReference: true,
			MaxReferenceImages:       1,
			MaxVariants:              4,
			SupportedSizes:           []string{"1024x1024", "1536x1024", "1024x1536", "auto"},
			ImageModels:              []string{"gpt-image-2", "gpt-image-1.5", "chatgpt-image-latest", "gpt-image-1", "gpt-image-1-mini", "dall-e-3", "dall-e-2"},
			DefaultImageModel:        "gpt-image-2",
			ModelOverrides: map[string]ModelImageCapabilities{
				"dall-e-2": {
					SupportedSizes:           []string{"256x256", "512x512", "1024x1024"},
					MaxVariants:              intPtr(10),
					SupportsContentReference: boolPtr(false),
				},
				"dall-e-3": {
					SupportedSizes:           []string{"1024x1024", "1792x1024", "1024x1792"},
					MaxVariants:              intPtr(1),
					SupportsEditing:          boolPtr(false),
					SupportsMasking:          boolPtr(false),
					SupportsContentReference: boolPtr(false),
				},
			},
		}
	case "gemini", "imagen":
		return ImageCapabilities{
			SupportsGeneration:       true,
			SupportsEditing:          true,
			SupportsMasking:          true,
			SupportsSeed:             true,
			SupportsContentReference: true,
			SupportsStyleReference:   true,
			MaxReferenceImages:       14,
			MaxVariants:              4,
			// WxH sizes that map to Gemini aspect ratios common to all 3 models.
			SupportedSizes: []string{
				"1024x1024", // 1:1
				"1024x1536", // 2:3
				"1536x1024", // 3:2
				"768x1024",  // 3:4
				"1024x768",  // 4:3
				"1024x1280", // 4:5
				"1280x1024", // 5:4
				"576x1024",  // 9:16
				"1024x576",  // 16:9
				"1344x576",  // 21:9
			},
			ImageModels: []string{
				"gemini-3.1-flash-image-preview",
				"gemini-3-pro-image-preview",
				"gemini-2.5-flash-image",
				"imagen-4.0-generate-001",
				"imagen-4.0-ultra-generate-001",
				"imagen-4.0-fast-generate-001",
				"imagen-3.0-generate-002",
				"imagen-3.0-fast-generate-001",
			},
			DefaultImageModel: "gemini-2.5-flash-image",
			ModelOverrides: map[string]ModelImageCapabilities{
				"gemini-3.1-flash-image-preview": {
					// Gemini 3.1 Flash adds 1:4, 4:1, 1:8, 8:1 aspect ratios.
					SupportedSizes: []string{
						"1024x1024", // 1:1
						"1024x1536", // 2:3
						"1536x1024", // 3:2
						"768x1024",  // 3:4
						"1024x768",  // 4:3
						"1024x1280", // 4:5
						"1280x1024", // 5:4
						"576x1024",  // 9:16
						"1024x576",  // 16:9
						"1344x576",  // 21:9
						"512x2048",  // 1:4
						"2048x512",  // 4:1
						"384x3072",  // 1:8
						"3072x384",  // 8:1
					},
				},
				"gemini-2.5-flash-image": {
					MaxVariants: intPtr(4),
					// gemini-2.5-flash-image works best with up to 3 reference images.
					SupportsContentReference: boolPtr(true),
				},
			},
		}
	case "stable-diffusion", "stability":
		return ImageCapabilities{
			SupportsGeneration:       true,
			SupportsEditing:          true,
			SupportsMasking:          true,
			SupportsSeed:             true,
			SupportsGuidance:         true,
			SupportsStyleReference:   true,
			SupportsContentReference: true,
			MaxReferenceImages:       1,
			MaxVariants:              4,
			SupportedSizes:           []string{"512x512", "768x768", "1024x1024"},
			ImageModels:              []string{"stable-diffusion-xl-1024-v1-0", "stable-diffusion-v1-6", "stable-diffusion-xl-beta-v2-2-2"},
			DefaultImageModel:        "stable-diffusion-xl-1024-v1-0",
		}
	case "together":
		return ImageCapabilities{
			SupportsGeneration: true,
			MaxVariants:        4,
			SupportedSizes:     []string{"1024x1024", "1024x768", "768x1024"},
			ImageModels: []string{
				"google/imagen-4.0-preview",
				"google/imagen-4.0-fast",
				"google/imagen-4.0-ultra",
				"google/flash-image-2.5",
				"google/gemini-3-pro-image",
				"black-forest-labs/FLUX.1-schnell-Free",
				"black-forest-labs/FLUX.1-schnell",
				"black-forest-labs/FLUX.1.1-pro",
				"black-forest-labs/FLUX.1-kontext-pro",
				"black-forest-labs/FLUX.1-kontext-max",
				"black-forest-labs/FLUX.1-krea-dev",
				"black-forest-labs/FLUX.2-pro",
				"black-forest-labs/FLUX.2-dev",
				"black-forest-labs/FLUX.2-flex",
				"ByteDance-Seed/Seedream-3.0",
				"ByteDance-Seed/Seedream-4.0",
				"Qwen/Qwen-Image",
				"RunDiffusion/Juggernaut-pro-flux",
				"Rundiffusion/Juggernaut-Lightning-Flux",
				"HiDream-ai/HiDream-I1-Full",
				"HiDream-ai/HiDream-I1-Dev",
				"HiDream-ai/HiDream-I1-Fast",
				"ideogram/ideogram-3.0",
				"Lykon/DreamShaper",
				"stabilityai/stable-diffusion-3-medium",
				"stabilityai/stable-diffusion-xl-base-1.0",
			},
			DefaultImageModel: "black-forest-labs/FLUX.1-schnell-Free",
		}
	case "openrouter":
		openRouterEditModel := ModelImageCapabilities{
			SupportsEditing:          boolPtr(true),
			SupportsContentReference: boolPtr(true),
		}
		return ImageCapabilities{
			SupportsGeneration: true,
			MaxVariants:        1,
			// Sizes map to OpenRouter aspect_ratio values via sizeToOpenRouterAspectRatio.
			SupportedSizes: []string{
				"1024x1024", // 1:1
				"832x1248",  // 2:3
				"1248x832",  // 3:2
				"864x1184",  // 3:4
				"1184x864",  // 4:3
				"896x1152",  // 4:5
				"1152x896",  // 5:4
				"768x1344",  // 9:16
				"1344x768",  // 16:9
				"1536x672",  // 21:9
			},
			ImageModels: []string{
				// Google / Gemini (text+image output)
				"google/gemini-2.5-flash-image",
				"google/gemini-3.1-flash-image-preview",
				"google/gemini-3-pro-image-preview",
				// OpenAI (text+image output)
				"openai/gpt-5.4-image-2",
				"openai/gpt-5-image",
				"openai/gpt-5-image-mini",
				// Black Forest Labs / FLUX (image-only; note: dot notation)
				"black-forest-labs/flux.2-pro",
				"black-forest-labs/flux.2-max",
				"black-forest-labs/flux.2-flex",
				"black-forest-labs/flux.2-klein-4b",
				// Recraft (image-only)
				"recraft/recraft-v3",
				"recraft/recraft-v4",
				"recraft/recraft-v4-pro",
				// Sourceful (image-only)
				"sourceful/riverflow-v2-fast",
				"sourceful/riverflow-v2-fast-preview",
				"sourceful/riverflow-v2-pro",
				"sourceful/riverflow-v2-max-preview",
				"sourceful/riverflow-v2-standard-preview",
				// ByteDance (image-only)
				"bytedance-seed/seedream-4.5",
			},
			DefaultImageModel: "google/gemini-2.5-flash-image",
			ModelOverrides: map[string]ModelImageCapabilities{
				// OpenRouter image models that accept image input and can perform
				// image-to-image edits after an initial generation.
				"google/gemini-2.5-flash-image":           openRouterEditModel,
				"google/gemini-3.1-flash-image-preview":   openRouterEditModel,
				"google/gemini-3-pro-image-preview":       openRouterEditModel,
				"openai/gpt-5.4-image-2":                  openRouterEditModel,
				"openai/gpt-5-image":                      openRouterEditModel,
				"openai/gpt-5-image-mini":                 openRouterEditModel,
				"black-forest-labs/flux.2-pro":            openRouterEditModel,
				"black-forest-labs/flux.2-max":            openRouterEditModel,
				"black-forest-labs/flux.2-flex":           openRouterEditModel,
				"black-forest-labs/flux.2-klein-4b":       openRouterEditModel,
				"recraft/recraft-v3":                      openRouterEditModel,
				"recraft/recraft-v4":                      openRouterEditModel,
				"recraft/recraft-v4-pro":                  openRouterEditModel,
				"sourceful/riverflow-v2-fast":             openRouterEditModel,
				"sourceful/riverflow-v2-fast-preview":     openRouterEditModel,
				"sourceful/riverflow-v2-pro":              openRouterEditModel,
				"sourceful/riverflow-v2-max-preview":      openRouterEditModel,
				"sourceful/riverflow-v2-standard-preview": openRouterEditModel,
				"bytedance-seed/seedream-4.5":             openRouterEditModel,
			},
		}
	default:
		// Default: generation only
		return ImageCapabilities{
			SupportsGeneration: true,
			MaxVariants:        1,
			SupportedSizes:     []string{"1024x1024", "1792x1024", "1024x1792"},
		}
	}
}
