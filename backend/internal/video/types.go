package video

import (
	"encoding/json"
	"time"
)

type Capability string

const (
	ProviderMock       = "mock"
	ProviderOpenRouter = "openrouter"
	ProviderGemini     = "gemini"
	ProviderOpenAI     = "openai"
	ProviderCustom     = "custom"

	CapabilityTextToVideo     Capability = "text_to_video"
	CapabilityImageToVideo    Capability = "image_to_video"
	CapabilityVideoToVideo    Capability = "video_to_video"
	CapabilityExtendVideo     Capability = "extend_video"
	CapabilityReferenceImages Capability = "reference_images"
	CapabilityReferenceVideo  Capability = "reference_video"
	CapabilityNegativePrompt  Capability = "negative_prompt"
	CapabilitySeed            Capability = "seed"
	CapabilityCameraMotion    Capability = "camera_motion"
	CapabilityAudioGeneration Capability = "audio_generation"
)

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
	Notes              string       `json:"notes,omitempty"`
}

type ProviderInfo struct {
	Key         string  `json:"key"`
	DisplayName string  `json:"display_name"`
	Configured  bool    `json:"configured"`
	Mock        bool    `json:"mock"`
	Models      []Model `json:"models,omitempty"`
}

type GenerateRequest struct {
	ProjectID         string          `json:"project_id,omitempty"`
	ParentID          string          `json:"parent_id,omitempty"`
	Title             string          `json:"title,omitempty"`
	Provider          string          `json:"provider"`
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt"`
	Enhance           bool            `json:"enhance,omitempty"`
	EnhancedPrompt    string          `json:"enhanced_prompt,omitempty"`
	NegativePrompt    string          `json:"negative_prompt,omitempty"`
	AspectRatio       string          `json:"aspect_ratio,omitempty"`
	DurationSeconds   int             `json:"duration_seconds,omitempty"`
	Resolution        string          `json:"resolution,omitempty"`
	FPS               int             `json:"fps,omitempty"`
	Seed              *int64          `json:"seed,omitempty"`
	ReferenceAssetIDs []string        `json:"reference_asset_ids,omitempty"`
	CameraMotion      string          `json:"camera_motion,omitempty"`
	ShotType          string          `json:"shot_type,omitempty"`
	StylePreset       string          `json:"style_preset,omitempty"`
	ProductionNotes   string          `json:"production_notes,omitempty"`
	Settings          json.RawMessage `json:"settings,omitempty"`
	PlaceOnTimeline   bool            `json:"place_on_timeline,omitempty"`
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
	OutputAssetID     *string    `json:"output_asset_id,omitempty"`
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
}

type EnhancePromptResponse struct {
	Prompt string `json:"prompt"`
}
