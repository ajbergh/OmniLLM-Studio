package llm

import "strings"

// ImageMaskingMode distinguishes exact alpha/pixel masks from conversational
// semantic edit guidance. SupportsMasking remains during the compatibility
// window for existing frontend/API consumers.
type ImageMaskingMode string

const (
	ImageMaskingNone     ImageMaskingMode = "none"
	ImageMaskingSemantic ImageMaskingMode = "semantic"
	ImageMaskingPixel    ImageMaskingMode = "pixel"
)

// ImageCapabilities describes what image operations a provider/model supports.
type ImageCapabilities struct {
	SupportsGeneration       bool             `json:"supports_generation"`
	SupportsEditing          bool             `json:"supports_editing"`
	SupportsMasking          bool             `json:"supports_masking"`
	MaskingMode              ImageMaskingMode `json:"masking_mode"`
	SupportsVariations       bool             `json:"supports_variations"`
	SupportsSeed             bool             `json:"supports_seed"`
	SupportsGuidance         bool             `json:"supports_guidance"`
	SupportsStyleReference   bool             `json:"supports_style_reference"`
	SupportsContentReference bool             `json:"supports_content_reference"`
	MaxReferenceImages       int              `json:"max_reference_images"`
	MaxVariants              int              `json:"max_variants"`
	SupportedSizes           []string         `json:"supported_sizes"`
	ImageModels              []string         `json:"image_models"`
	DefaultImageModel        string           `json:"default_image_model"`

	// Per-model overrides keyed by model name. Only fields that differ from the provider default need to be set.
	ModelOverrides map[string]ModelImageCapabilities `json:"model_overrides,omitempty"`
}

// ModelImageCapabilities holds per-model capability overrides.
type ModelImageCapabilities struct {
	SupportsEditing          *bool             `json:"supports_editing,omitempty"`
	SupportsMasking          *bool             `json:"supports_masking,omitempty"`
	MaskingMode              *ImageMaskingMode `json:"masking_mode,omitempty"`
	SupportsContentReference *bool             `json:"supports_content_reference,omitempty"`
	SupportsStyleReference   *bool             `json:"supports_style_reference,omitempty"`
	SupportsSeed             *bool             `json:"supports_seed,omitempty"`
	SupportsGuidance         *bool             `json:"supports_guidance,omitempty"`
	MaxReferenceImages       *int              `json:"max_reference_images,omitempty"`
	MaxVariants              *int              `json:"max_variants,omitempty"`
	SupportedSizes           []string          `json:"supported_sizes,omitempty"`
}

func boolPtr(b bool) *bool                                   { return &b }
func intPtr(i int) *int                                      { return &i }
func maskingModePtr(mode ImageMaskingMode) *ImageMaskingMode { return &mode }

// GetEffectiveImageCapabilities returns provider capabilities with any
// model-specific overrides applied.
func GetEffectiveImageCapabilities(providerType, model string) ImageCapabilities {
	caps := GetImageCapabilities(strings.ToLower(strings.TrimSpace(providerType)))
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
	if overrides.MaskingMode != nil {
		caps.MaskingMode = *overrides.MaskingMode
	}
	if overrides.SupportsContentReference != nil {
		caps.SupportsContentReference = *overrides.SupportsContentReference
	}
	if overrides.SupportsStyleReference != nil {
		caps.SupportsStyleReference = *overrides.SupportsStyleReference
	}
	if overrides.SupportsSeed != nil {
		caps.SupportsSeed = *overrides.SupportsSeed
	}
	if overrides.SupportsGuidance != nil {
		caps.SupportsGuidance = *overrides.SupportsGuidance
	}
	if overrides.MaxReferenceImages != nil {
		caps.MaxReferenceImages = *overrides.MaxReferenceImages
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
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai-dall-e-3", "dall-e-3":
		return ImageCapabilities{
			SupportsGeneration: true,
			MaskingMode:        ImageMaskingNone,
			MaxVariants:        1,
			SupportedSizes:     []string{"1024x1024", "1792x1024", "1024x1792"},
			ImageModels:        []string{"dall-e-3"},
			DefaultImageModel:  "dall-e-3",
		}
	case "openai-dall-e-2", "dall-e-2":
		return ImageCapabilities{
			SupportsGeneration: true,
			SupportsEditing:    true,
			SupportsMasking:    true,
			MaskingMode:        ImageMaskingPixel,
			SupportsVariations: true,
			MaxVariants:        10,
			SupportedSizes:     []string{"256x256", "512x512", "1024x1024"},
			ImageModels:        []string{"dall-e-2"},
			DefaultImageModel:  "dall-e-2",
		}
	case "openai":
		return ImageCapabilities{
			SupportsGeneration: true,
			SupportsEditing:    true,
			SupportsMasking:    true,
			MaskingMode:        ImageMaskingPixel,
			MaxVariants:        4,
			SupportedSizes:     []string{"1024x1024", "1536x1024", "1024x1536", "auto"},
			ImageModels:        []string{"gpt-image-2", "gpt-image-1.5", "chatgpt-image-latest", "gpt-image-1", "gpt-image-1-mini", "dall-e-3", "dall-e-2"},
			DefaultImageModel:  "gpt-image-2",
			ModelOverrides: map[string]ModelImageCapabilities{
				"dall-e-2": {
					SupportedSizes: []string{"256x256", "512x512", "1024x1024"},
					MaxVariants:    intPtr(10),
				},
				"dall-e-3": {
					SupportedSizes:           []string{"1024x1024", "1792x1024", "1024x1792"},
					MaxVariants:              intPtr(1),
					SupportsEditing:          boolPtr(false),
					SupportsMasking:          boolPtr(false),
					MaskingMode:              maskingModePtr(ImageMaskingNone),
					SupportsContentReference: boolPtr(false),
				},
			},
		}
	case "gemini", "imagen":
		imagenGenerationOnly := ModelImageCapabilities{
			SupportsEditing:          boolPtr(false),
			SupportsMasking:          boolPtr(false),
			MaskingMode:              maskingModePtr(ImageMaskingNone),
			SupportsContentReference: boolPtr(false),
			SupportsStyleReference:   boolPtr(false),
			SupportsSeed:             boolPtr(false),
		}
		return ImageCapabilities{
			SupportsGeneration:       true,
			SupportsEditing:          true,
			SupportsMasking:          true,
			MaskingMode:              ImageMaskingSemantic,
			SupportsContentReference: true,
			SupportsStyleReference:   true,
			MaxReferenceImages:       14,
			MaxVariants:              4,
			SupportedSizes: []string{
				"1024x1024", "1024x1536", "1536x1024", "768x1024", "1024x768",
				"1024x1280", "1280x1024", "576x1024", "1024x576", "1344x576",
			},
			ImageModels: []string{
				"gemini-3.1-flash-image",
				"gemini-3.1-flash-lite-image",
				"gemini-3-pro-image",
				"gemini-2.5-flash-image",
				"imagen-4.0-generate-001",
				"imagen-4.0-ultra-generate-001",
				"imagen-4.0-fast-generate-001",
			},
			DefaultImageModel: "gemini-3.1-flash-image",
			ModelOverrides: map[string]ModelImageCapabilities{
				"gemini-3.1-flash-image": {
					SupportedSizes: []string{
						"1024x1024", "1024x1536", "1536x1024", "768x1024", "1024x768",
						"1024x1280", "1280x1024", "576x1024", "1024x576", "1344x576",
						"512x2048", "2048x512", "384x3072", "3072x384",
					},
				},
				"gemini-2.5-flash-image":        {MaxReferenceImages: intPtr(3)},
				"imagen-4.0-generate-001":       imagenGenerationOnly,
				"imagen-4.0-ultra-generate-001": imagenGenerationOnly,
				"imagen-4.0-fast-generate-001":  imagenGenerationOnly,
			},
		}
	case "stable-diffusion", "stability":
		// No Stability-specific transport exists in Service.ImageGenerate.
		// Do not advertise controls that will be rejected by the service.
		return ImageCapabilities{MaskingMode: ImageMaskingNone}
	case "together":
		return ImageCapabilities{
			SupportsGeneration: true,
			SupportsSeed:       true,
			SupportsGuidance:   true,
			MaskingMode:        ImageMaskingNone,
			MaxVariants:        4,
			SupportedSizes:     []string{"1024x1024", "1024x768", "768x1024"},
			ImageModels: []string{
				"google/imagen-4.0-preview", "google/imagen-4.0-fast", "google/imagen-4.0-ultra",
				"google/flash-image-2.5", "google/gemini-3-pro-image",
				"black-forest-labs/FLUX.1-schnell-Free", "black-forest-labs/FLUX.1-schnell",
				"black-forest-labs/FLUX.1.1-pro", "black-forest-labs/FLUX.1-kontext-pro",
				"black-forest-labs/FLUX.1-kontext-max", "black-forest-labs/FLUX.1-krea-dev",
				"black-forest-labs/FLUX.2-pro", "black-forest-labs/FLUX.2-dev", "black-forest-labs/FLUX.2-flex",
				"ByteDance-Seed/Seedream-3.0", "ByteDance-Seed/Seedream-4.0", "Qwen/Qwen-Image",
				"RunDiffusion/Juggernaut-pro-flux", "Rundiffusion/Juggernaut-Lightning-Flux",
				"HiDream-ai/HiDream-I1-Full", "HiDream-ai/HiDream-I1-Dev", "HiDream-ai/HiDream-I1-Fast",
				"ideogram/ideogram-3.0", "Lykon/DreamShaper", "stabilityai/stable-diffusion-3-medium",
				"stabilityai/stable-diffusion-xl-base-1.0",
			},
			DefaultImageModel: "black-forest-labs/FLUX.1-schnell-Free",
		}
	case "openrouter":
		openRouterEditModel := ModelImageCapabilities{
			SupportsEditing:          boolPtr(true),
			SupportsContentReference: boolPtr(true),
			SupportsMasking:          boolPtr(false),
			MaskingMode:              maskingModePtr(ImageMaskingNone),
		}
		return ImageCapabilities{
			SupportsGeneration: true,
			MaskingMode:        ImageMaskingNone,
			MaxReferenceImages: 1,
			MaxVariants:        1,
			SupportedSizes: []string{
				"1024x1024", "832x1248", "1248x832", "864x1184", "1184x864",
				"896x1152", "1152x896", "768x1344", "1344x768", "1536x672",
			},
			ImageModels: []string{
				"google/gemini-2.5-flash-image", "google/gemini-3.1-flash-image-preview", "google/gemini-3-pro-image-preview",
				"openai/gpt-5.4-image-2", "openai/gpt-5-image", "openai/gpt-5-image-mini",
				"black-forest-labs/flux.2-pro", "black-forest-labs/flux.2-max", "black-forest-labs/flux.2-flex", "black-forest-labs/flux.2-klein-4b",
				"recraft/recraft-v3", "recraft/recraft-v4", "recraft/recraft-v4-pro",
				"sourceful/riverflow-v2-fast", "sourceful/riverflow-v2-fast-preview", "sourceful/riverflow-v2-pro", "sourceful/riverflow-v2-max-preview", "sourceful/riverflow-v2-standard-preview",
				"bytedance-seed/seedream-4.5",
			},
			DefaultImageModel: "google/gemini-2.5-flash-image",
			ModelOverrides: map[string]ModelImageCapabilities{
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
		// Unknown or unrouted providers must not be advertised as image-capable.
		return ImageCapabilities{MaskingMode: ImageMaskingNone}
	}
}
