package video

import (
	"encoding/json"
	"time"
)

type Capability string

const (
	ProviderOpenRouter = "openrouter"
	ProviderGemini     = "gemini"
	ProviderLuma       = "luma"
	ProviderOpenAI     = "openai"
	ProviderCustom     = "custom"

	CapabilityTextToVideo      Capability = "text_to_video"
	CapabilityImageToVideo     Capability = "image_to_video"
	CapabilityVideoToVideo     Capability = "video_to_video"
	CapabilityExtendVideo      Capability = "extend_video"
	CapabilityFirstLastFrame   Capability = "first_last_frame"
	CapabilityReferenceImages  Capability = "reference_images"
	CapabilityReferenceVideo   Capability = "reference_video"
	CapabilityNegativePrompt   Capability = "negative_prompt"
	CapabilityPersonGeneration Capability = "person_generation"
	CapabilitySeed             Capability = "seed"
	CapabilityCameraMotion     Capability = "camera_motion"
	CapabilityAudioGeneration  Capability = "audio_generation"
)

// InputAssetRole categorises the role of an input image/video asset in a generation.
type InputAssetRole string

const (
	RoleStartFrame  InputAssetRole = "start_frame"
	RoleLastFrame   InputAssetRole = "last_frame"
	RoleReference   InputAssetRole = "reference_image"
	RoleSourceVideo InputAssetRole = "source_video"
)

// InputAsset pairs an asset ID with its intended role.  These are stored as
// a JSON array in video_generations.input_assets_json for structured lookups.
type InputAsset struct {
	AssetID string         `json:"asset_id"`
	Role    InputAssetRole `json:"role"`
}

type Model struct {
	ID                 string       `json:"id"`
	Provider           string       `json:"provider"`
	Name               string       `json:"name"`
	Capabilities       []Capability `json:"capabilities"`
	AspectRatios       []string     `json:"aspect_ratios,omitempty"`
	Resolutions        []string     `json:"resolutions,omitempty"`
	DurationMinSeconds int          `json:"duration_min_seconds,omitempty"`
	DurationMaxSeconds int          `json:"duration_max_seconds,omitempty"`
	FPSOptions         []int        `json:"fps_options,omitempty"`
	MaxPromptChars     int          `json:"max_prompt_chars,omitempty"`
	MaxReferenceImages int          `json:"max_reference_images,omitempty"`
	Notes              string       `json:"notes,omitempty"`
}

type ProviderInfo struct {
	Key         string  `json:"key"`
	DisplayName string  `json:"display_name"`
	Configured  bool    `json:"configured"`
	Models      []Model `json:"models,omitempty"`
}

type GenerateRequest struct {
	ProjectID string `json:"project_id,omitempty"`
	ParentID  string `json:"parent_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	// GenerationMode maps directly to provider task semantics when available.
	// Gemini Omni accepts text_to_video, image_to_video, reference_to_video, and edit.
	GenerationMode  string `json:"generation_mode,omitempty"`
	Enhance         bool   `json:"enhance,omitempty"`
	EnhancedPrompt  string `json:"enhanced_prompt,omitempty"`
	NegativePrompt  string `json:"negative_prompt,omitempty"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	FPS             int    `json:"fps,omitempty"`
	Seed            *int64 `json:"seed,omitempty"`
	// PersonGeneration controls Veo's person generation policy ("allow"|"dont_allow").
	PersonGeneration string `json:"person_generation,omitempty"`
	// StartImageAssetID is the video asset ID to use as the starting frame (image-to-video).
	StartImageAssetID string `json:"start_image_asset_id,omitempty"`
	// LastFrameAssetID is the video asset ID for the last frame (first/last-frame interpolation).
	LastFrameAssetID string `json:"last_frame_asset_id,omitempty"`
	// SourceVideoAssetID is the video asset ID to extend (video extension mode).
	SourceVideoAssetID string `json:"source_video_asset_id,omitempty"`
	// ReferenceAssetIDs holds style/character/product reference image asset IDs.
	ReferenceAssetIDs   []string `json:"reference_asset_ids,omitempty"`
	ReferenceAssetPaths []string `json:"-"` // resolved by service, not from JSON
	// StartImagePath / LastFramePath / SourceVideoPath are resolved by service.
	StartImagePath  string `json:"-"`
	LastFramePath   string `json:"-"`
	SourceVideoPath string `json:"-"`
	// PreviousInteractionID is resolved from ParentID by the service for stateful
	// Gemini Omni edits. It is never accepted directly from an HTTP request.
	PreviousInteractionID string          `json:"-"`
	CameraMotion          string          `json:"camera_motion,omitempty"`
	ShotType              string          `json:"shot_type,omitempty"`
	StylePreset           string          `json:"style_preset,omitempty"`
	Composition           string          `json:"composition,omitempty"`
	LensEffect            string          `json:"lens_effect,omitempty"`
	Lighting              string          `json:"lighting,omitempty"`
	Dialogue              string          `json:"dialogue,omitempty"`
	SoundEffects          string          `json:"sound_effects,omitempty"`
	AmbientNoise          string          `json:"ambient_noise,omitempty"`
	ContinuityNotes       string          `json:"continuity_notes,omitempty"`
	ProductionNotes       string          `json:"production_notes,omitempty"`
	Settings              json.RawMessage `json:"settings,omitempty"`
	PlaceOnTimeline       bool            `json:"place_on_timeline,omitempty"`
}

// GenerateAsyncResponse is returned by the non-blocking POST /v1/video/generations endpoint.
type GenerateAsyncResponse struct {
	GenerationID string `json:"generation_id"`
	ProjectID    string `json:"project_id"`
	Status       string `json:"status"`
}

type GenerationProgress struct {
	Stage        string  `json:"stage"`
	Message      string  `json:"message"`
	ProjectID    string  `json:"project_id,omitempty"`
	GenerationID string  `json:"generation_id,omitempty"`
	Progress     float64 `json:"progress,omitempty"`
}

type GenerationResult struct {
	MimeType      string          `json:"mime_type"`
	FileName      string          `json:"file_name"`
	Data          []byte          `json:"-"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	Width         *int            `json:"width,omitempty"`
	Height        *int            `json:"height,omitempty"`
	FPS           *float64        `json:"fps,omitempty"`
	UpstreamJobID *string         `json:"upstream_job_id,omitempty"`
	UpstreamReqID *string         `json:"upstream_request_id,omitempty"`
	UsageJSON     json.RawMessage `json:"usage_json,omitempty"`
	CostUSD       *float64        `json:"cost_usd,omitempty"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
}

type GenerationDetail struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	ParentID          *string    `json:"parent_id,omitempty"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	Prompt            string     `json:"prompt"`
	EnhancedPrompt    *string    `json:"enhanced_prompt,omitempty"`
	NegativePrompt    *string    `json:"negative_prompt,omitempty"`
	SettingsJSON      string     `json:"settings_json,omitempty"`
	InputAssetIDsJSON string     `json:"input_asset_ids_json,omitempty"`
	InputAssetsJSON   string     `json:"input_assets_json,omitempty"`
	OutputAssetID     *string    `json:"output_asset_id,omitempty"`
	UpstreamJobID     *string    `json:"upstream_job_id,omitempty"`
	UpstreamReqID     *string    `json:"upstream_request_id,omitempty"`
	UsageJSON         *string    `json:"usage_json,omitempty"`
	AssetURL          string     `json:"asset_url,omitempty"`
	MimeType          string     `json:"mime_type,omitempty"`
	CostUSD           *float64   `json:"cost_usd,omitempty"`
	Error             *string    `json:"error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type EnhancePromptRequest struct {
	Prompt          string `json:"prompt"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	NegativePrompt  string `json:"negative_prompt,omitempty"`
	// InputMode hints at the generation mode (e.g. "image-to-video", "extend", "first_last_frame").
	InputMode string `json:"input_mode,omitempty"`
	// ProductionNotes carries cinematic detail directives (style, composition, lighting, audio cues, etc.)
	// to inform the LLM enhancer when building a structured prompt.
	ProductionNotes string `json:"production_notes,omitempty"`
}

type EnhancePromptResponse struct {
	Prompt string `json:"prompt"`
}

type ExternalAssetImportRequest struct {
	SourceStudio string         `json:"source_studio"`
	SourceID     string         `json:"source_id"`
	SourceType   string         `json:"source_type,omitempty"`
	Kind         string         `json:"kind"`
	FileName     string         `json:"file_name"`
	MimeType     string         `json:"mime_type"`
	SizeBytes    int64          `json:"size_bytes,omitempty"`
	DurationMS   *int64         `json:"duration_ms,omitempty"`
	Width        *int           `json:"width,omitempty"`
	Height       *int           `json:"height,omitempty"`
	FPS          *float64       `json:"fps,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ExportSettings struct {
	Format     string `json:"format"`
	Codec      string `json:"codec,omitempty"`
	Resolution string `json:"resolution"`
	// Preset is an advisory label for the chosen export preset
	// (e.g. "youtube_16_9", "shorts_9_16", "square_1_1", "custom").
	Preset string `json:"preset,omitempty"`
	// Width/Height override Resolution when both are set (custom export size).
	Width                 int    `json:"width,omitempty"`
	Height                int    `json:"height,omitempty"`
	FPS                   int    `json:"fps,omitempty"`
	Quality               string `json:"quality,omitempty"`
	IncludeAudio          bool   `json:"include_audio"`
	RegisterInFileLibrary bool   `json:"register_in_file_library,omitempty"`
	EstimatedDurationMS   int64  `json:"estimated_duration_ms,omitempty"`
	// BurnInCaptions controls whether caption-track text draws into the frame.
	// Nil preserves the historical always-on behavior.
	BurnInCaptions *bool `json:"burn_in_captions,omitempty"`
	// SidecarCaptions additionally writes the captions as a sibling asset:
	// "" (none), "srt", or "vtt".
	SidecarCaptions string `json:"sidecar_captions,omitempty"`
	// RangeStartMS/RangeEndMS export only that timeline window when end > start.
	RangeStartMS int64 `json:"range_start_ms,omitempty"`
	RangeEndMS   int64 `json:"range_end_ms,omitempty"`
	// AudioBitrateKbps overrides the encoder's default audio bitrate (32–512).
	AudioBitrateKbps int `json:"audio_bitrate_kbps,omitempty"`
}
