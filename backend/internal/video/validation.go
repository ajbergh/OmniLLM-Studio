package video

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type GenerationValidationSeverity string

const (
	GenerationValidationError         GenerationValidationSeverity = "error"
	GenerationValidationWarning       GenerationValidationSeverity = "warning"
	GenerationValidationNormalization GenerationValidationSeverity = "normalization"
)

type GenerationValidationIssue struct {
	Code       string                       `json:"code"`
	Field      string                       `json:"field,omitempty"`
	Message    string                       `json:"message"`
	Severity   GenerationValidationSeverity `json:"severity"`
	Original   any                          `json:"original,omitempty"`
	Normalized any                          `json:"normalized,omitempty"`
}

type GenerateValidationResult struct {
	Valid             bool                        `json:"valid"`
	Provider          string                      `json:"provider,omitempty"`
	Model             string                      `json:"model,omitempty"`
	Capabilities      []Capability                `json:"capabilities,omitempty"`
	NormalizedRequest GenerateRequest             `json:"normalized_request"`
	Errors            []GenerationValidationIssue `json:"errors"`
	Warnings          []GenerationValidationIssue `json:"warnings"`
	Normalizations    []GenerationValidationIssue `json:"normalizations"`
}

func (r GenerateValidationResult) ErrorMessage() string {
	messages := make([]string, 0, len(r.Errors))
	for _, issue := range r.Errors {
		if strings.TrimSpace(issue.Message) != "" {
			messages = append(messages, issue.Message)
		}
	}
	if len(messages) == 0 {
		return "generation request is invalid"
	}
	return strings.Join(messages, "; ")
}

func (r *ModelRegistry) ValidateGenerateRequest(ctx context.Context, req GenerateRequest) GenerateValidationResult {
	result := GenerateValidationResult{
		Valid:             true,
		NormalizedRequest: req,
	}
	providerKey := NormalizeProvider(req.Provider)
	result.Provider = providerKey
	if providerKey == "" {
		result.addError("provider", "unsupported_provider", "Choose a supported video provider.")
		return result.finalize()
	}
	provider, ok := r.Provider(providerKey)
	if !ok {
		result.addError("provider", "provider_not_registered", fmt.Sprintf("No video adapter is registered for %s.", providerKey))
		return result.finalize()
	}
	modelID := strings.TrimSpace(req.Model)
	if modelID == "" {
		modelID = r.DefaultModel(ctx, providerKey)
	}
	model, ok := r.FindModel(ctx, providerKey, modelID)
	if !ok || strings.TrimSpace(modelID) == "" {
		result.addError("model", "unsupported_model", fmt.Sprintf("%s is not supported by %s.", defaultString(modelID, "Selected model"), providerKey))
		return result.finalize()
	}
	result.Model = model.ID
	result.Capabilities = cloneCapabilities(model.Capabilities)
	if len(result.Capabilities) == 0 {
		result.Capabilities = cloneCapabilities(provider.Capabilities(model.ID))
	}
	result.NormalizedRequest.Provider = providerKey
	result.NormalizedRequest.Model = model.ID
	return validateGenerateRequestForModel(providerKey, model, result)
}

func validateGenerateRequestForModel(providerKey string, model Model, result GenerateValidationResult) GenerateValidationResult {
	req := result.NormalizedRequest
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.NegativePrompt = strings.TrimSpace(req.NegativePrompt)
	req.PersonGeneration = strings.TrimSpace(req.PersonGeneration)
	req.AspectRatio = strings.TrimSpace(req.AspectRatio)
	req.Resolution = strings.TrimSpace(req.Resolution)
	req.GenerationMode = strings.ToLower(strings.TrimSpace(req.GenerationMode))
	req.ReferenceAssetIDs = compactStrings(req.ReferenceAssetIDs)

	if req.Prompt == "" {
		result.addError("prompt", "prompt_required", "Enter a video prompt before generating.")
	}

	hasStartImage := strings.TrimSpace(req.StartImageAssetID) != ""
	hasLastFrame := strings.TrimSpace(req.LastFrameAssetID) != ""
	hasSourceVideo := strings.TrimSpace(req.SourceVideoAssetID) != ""
	hasReferenceImages := len(req.ReferenceAssetIDs) > 0
	isOmni := providerKey == ProviderGemini && isGeminiOmniModel(model.ID)

	if isOmni {
		if req.GenerationMode == "" {
			switch {
			case hasSourceVideo || strings.TrimSpace(req.ParentID) != "":
				req.GenerationMode = "edit"
			case hasReferenceImages:
				req.GenerationMode = "reference_to_video"
			case hasStartImage:
				req.GenerationMode = "image_to_video"
			default:
				req.GenerationMode = "text_to_video"
			}
		}
		switch req.GenerationMode {
		case "text_to_video":
			if hasStartImage || hasReferenceImages || hasSourceVideo {
				result.addError("generation_mode", "omni_text_inputs_invalid", "Text to video does not accept source media. Choose Animate, References, or Edit instead.")
			}
		case "image_to_video":
			if !hasStartImage {
				result.addError("start_image_asset_id", "omni_start_image_required", "Choose a start image for image-to-video generation.")
			}
			if hasReferenceImages || hasSourceVideo {
				result.addError("generation_mode", "omni_image_inputs_invalid", "Image to video accepts one start image. Choose References to combine images.")
			}
		case "reference_to_video":
			if !hasReferenceImages {
				result.addError("reference_asset_ids", "omni_references_required", "Add at least one reference image.")
			}
			if hasSourceVideo {
				result.addError("source_video_asset_id", "omni_reference_video_invalid", "Reference generation cannot include a source video.")
			}
		case "edit":
			if !hasSourceVideo && strings.TrimSpace(req.ParentID) == "" {
				result.addError("source_video_asset_id", "omni_edit_source_required", "Choose a video or continue from a completed Omni generation to edit.")
			}
			if hasSourceVideo && strings.TrimSpace(req.ParentID) != "" {
				result.addError("source_video_asset_id", "omni_edit_source_exclusive", "Choose either an uploaded video or a previous Omni result, not both.")
			}
			if hasStartImage || hasReferenceImages {
				result.addError("generation_mode", "omni_edit_inputs_invalid", "Video editing accepts one video context and an edit instruction.")
			}
		default:
			result.addError("generation_mode", "omni_task_invalid", "Choose a supported Gemini Omni video mode.")
		}
	}

	caps := result.Capabilities
	if !hasStartImage && !hasLastFrame && !hasSourceVideo && !hasReferenceImages && !hasCapability(caps, CapabilityTextToVideo) {
		result.addError("model", "text_to_video_unsupported", fmt.Sprintf("%s does not support text-to-video generation.", model.Name))
	}
	if hasStartImage && !hasCapability(caps, CapabilityImageToVideo) {
		result.addError("start_image_asset_id", "image_to_video_unsupported", fmt.Sprintf("%s does not support start-frame image generation.", model.Name))
	}
	if hasLastFrame {
		if !hasCapability(caps, CapabilityFirstLastFrame) {
			result.addError("last_frame_asset_id", "first_last_frame_unsupported", fmt.Sprintf("%s does not support first/last-frame interpolation.", model.Name))
		}
		if !hasStartImage {
			result.addError("last_frame_asset_id", "last_frame_requires_start_frame", "Choose a start frame before choosing a last frame.")
		}
	}
	if hasSourceVideo {
		if isOmni && !hasCapability(caps, CapabilityVideoToVideo) {
			result.addError("source_video_asset_id", "source_video_unsupported", fmt.Sprintf("%s does not support video editing.", model.Name))
		} else if !isOmni && !hasCapability(caps, CapabilityExtendVideo) {
			result.addError("source_video_asset_id", "source_video_unsupported", fmt.Sprintf("%s does not support source-video extension.", model.Name))
		}
		if !isOmni && (hasStartImage || hasLastFrame || hasReferenceImages) {
			result.addError("source_video_asset_id", "source_video_exclusive", "Source-video extension cannot be combined with start frame, last frame, or reference images.")
		}
	}
	if hasReferenceImages {
		if !hasCapability(caps, CapabilityReferenceImages) {
			result.addError("reference_asset_ids", "reference_images_unsupported", fmt.Sprintf("%s does not support reference images.", model.Name))
		}
		if len(req.ReferenceAssetIDs) > maxReferenceImages(providerKey, model) {
			result.addError("reference_asset_ids", "too_many_reference_images", fmt.Sprintf("Use no more than %d reference images for this model.", maxReferenceImages(providerKey, model)))
		}
	}
	if req.NegativePrompt != "" && !hasCapability(caps, CapabilityNegativePrompt) {
		result.addError("negative_prompt", "negative_prompt_unsupported", fmt.Sprintf("%s does not support negative prompts.", model.Name))
	}
	if req.Seed != nil && !hasCapability(caps, CapabilitySeed) {
		result.addError("seed", "seed_unsupported", fmt.Sprintf("%s does not expose deterministic seed control.", model.Name))
	}
	if req.PersonGeneration != "" {
		if !hasCapability(caps, CapabilityPersonGeneration) {
			result.addError("person_generation", "person_generation_unsupported", fmt.Sprintf("%s does not support person-generation policy controls.", model.Name))
		} else if req.PersonGeneration != "allow" && req.PersonGeneration != "dont_allow" {
			result.addError("person_generation", "person_generation_invalid", "Person generation must be either allow or dont_allow.")
		}
	}

	hasPromptAudioCues := strings.TrimSpace(req.Dialogue) != "" || strings.TrimSpace(req.SoundEffects) != "" || strings.TrimSpace(req.AmbientNoise) != ""
	if hasPromptAudioCues {
		switch {
		case providerKey == ProviderGemini:
			result.addWarning("dialogue", "gemini_audio_prompt_only", "Gemini Veo dialogue and sound cues are added to the prompt; there is no separate native audio toggle.")
		case !hasCapability(caps, CapabilityAudioGeneration):
			result.addError("dialogue", "audio_controls_unsupported", fmt.Sprintf("%s does not support audio or dialogue generation controls.", model.Name))
		}
	}
	if generateAudio, ok := boolSetting(req.Settings, "generate_audio"); ok {
		if providerKey == ProviderGemini {
			result.addWarning("settings.generate_audio", "gemini_generate_audio_ignored", "Gemini Veo ignores generate_audio settings; describe audio in the prompt instead.")
		} else if generateAudio && !hasCapability(caps, CapabilityAudioGeneration) {
			result.addError("settings.generate_audio", "generate_audio_unsupported", fmt.Sprintf("%s does not support native audio generation.", model.Name))
		}
	}

	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = defaultAspectRatioForModel(model)
	}
	if len(model.AspectRatios) > 0 && !containsStringFold(model.AspectRatios, aspectRatio) {
		result.addError("aspect_ratio", "aspect_ratio_unsupported", fmt.Sprintf("%s does not support aspect ratio %s.", model.Name, aspectRatio))
	}
	req.AspectRatio = aspectRatio

	resolution := req.Resolution
	if resolution == "" {
		resolution = defaultResolutionForModel(model)
	}
	if hasSourceVideo && providerKey == ProviderGemini && !strings.EqualFold(resolution, "720p") {
		result.addNormalization("resolution", "source_video_resolution_normalized", "Gemini Veo source-video extension exports at 720p.", resolution, "720p")
		resolution = "720p"
	}
	if len(model.Resolutions) > 0 && !containsStringFold(model.Resolutions, resolution) {
		result.addError("resolution", "resolution_unsupported", fmt.Sprintf("%s does not support %s output.", model.Name, resolution))
	}
	req.Resolution = resolution

	duration := req.DurationSeconds
	if duration <= 0 {
		duration = defaultDurationForModel(model)
	}
	originalDuration := duration
	if model.DurationMinSeconds > 0 && duration < model.DurationMinSeconds {
		duration = model.DurationMinSeconds
		result.addNormalization("duration_seconds", "duration_min_normalized", fmt.Sprintf("Duration was raised to %d seconds for %s.", duration, model.Name), originalDuration, duration)
		originalDuration = duration
	}
	if model.DurationMaxSeconds > 0 && duration > model.DurationMaxSeconds {
		duration = model.DurationMaxSeconds
		result.addNormalization("duration_seconds", "duration_max_normalized", fmt.Sprintf("Duration was capped at %d seconds for %s.", duration, model.Name), originalDuration, duration)
		originalDuration = duration
	}
	if providerKey == ProviderGemini && !isOmni {
		forceEightReason := ""
		switch {
		case hasSourceVideo:
			forceEightReason = "Gemini Veo source-video extension requires 8 seconds."
		case hasLastFrame:
			forceEightReason = "Gemini Veo first/last-frame interpolation requires 8 seconds."
		case hasReferenceImages:
			forceEightReason = "Gemini Veo reference image generation requires 8 seconds."
		case strings.EqualFold(resolution, "1080p") || strings.EqualFold(resolution, "4k"):
			forceEightReason = "Gemini Veo 1080p and 4K generation require 8 seconds."
		}
		if forceEightReason != "" && duration != 8 {
			result.addNormalization("duration_seconds", "gemini_duration_normalized", forceEightReason, duration, 8)
			duration = 8
		}
	}
	req.DurationSeconds = duration

	if req.FPS <= 0 {
		req.FPS = defaultFPSForModel(model)
	} else if len(model.FPSOptions) > 0 && !containsInt(model.FPSOptions, req.FPS) {
		normalized := defaultFPSForModel(model)
		result.addNormalization("fps", "fps_normalized", fmt.Sprintf("%s exports at %d fps.", model.Name, normalized), req.FPS, normalized)
		req.FPS = normalized
	}

	result.NormalizedRequest = req
	return result.finalize()
}

func (r *GenerateValidationResult) addError(field, code, message string) {
	r.Errors = append(r.Errors, GenerationValidationIssue{Field: field, Code: code, Message: message, Severity: GenerationValidationError})
}

func (r *GenerateValidationResult) addWarning(field, code, message string) {
	r.Warnings = append(r.Warnings, GenerationValidationIssue{Field: field, Code: code, Message: message, Severity: GenerationValidationWarning})
}

func (r *GenerateValidationResult) addNormalization(field, code, message string, original, normalized any) {
	r.Normalizations = append(r.Normalizations, GenerationValidationIssue{
		Field:      field,
		Code:       code,
		Message:    message,
		Severity:   GenerationValidationNormalization,
		Original:   original,
		Normalized: normalized,
	})
}

func (r GenerateValidationResult) finalize() GenerateValidationResult {
	r.Valid = len(r.Errors) == 0
	if r.Errors == nil {
		r.Errors = []GenerationValidationIssue{}
	}
	if r.Warnings == nil {
		r.Warnings = []GenerationValidationIssue{}
	}
	if r.Normalizations == nil {
		r.Normalizations = []GenerationValidationIssue{}
	}
	return r
}

func cloneCapabilities(in []Capability) []Capability {
	out := make([]Capability, len(in))
	copy(out, in)
	return out
}

func hasCapability(caps []Capability, target Capability) bool {
	for _, cap := range caps {
		if cap == target {
			return true
		}
	}
	return false
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, value := range in {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func maxReferenceImages(provider string, model Model) int {
	_ = provider
	if model.MaxReferenceImages > 0 {
		return model.MaxReferenceImages
	}
	return 3
}

func defaultAspectRatioForModel(model Model) string {
	if containsStringFold(model.AspectRatios, DefaultAspectRatio) {
		return DefaultAspectRatio
	}
	if len(model.AspectRatios) > 0 {
		return model.AspectRatios[0]
	}
	return DefaultAspectRatio
}

func defaultResolutionForModel(model Model) string {
	if containsStringFold(model.Resolutions, "720p") {
		return "720p"
	}
	if len(model.Resolutions) > 0 {
		return model.Resolutions[0]
	}
	return "720p"
}

func defaultDurationForModel(model Model) int {
	duration := 8
	if model.DurationMinSeconds > 0 && duration < model.DurationMinSeconds {
		duration = model.DurationMinSeconds
	}
	if model.DurationMaxSeconds > 0 && duration > model.DurationMaxSeconds {
		duration = model.DurationMaxSeconds
	}
	return duration
}

func defaultFPSForModel(model Model) int {
	if len(model.FPSOptions) > 0 && model.FPSOptions[0] > 0 {
		return model.FPSOptions[0]
	}
	return DefaultProjectFPS
}

func containsStringFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func boolSetting(raw json.RawMessage, key string) (bool, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return false, false
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return false, false
	}
	value, ok := settings[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	}
	return false, true
}
