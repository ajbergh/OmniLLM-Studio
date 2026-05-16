package music

import "time"

type Capability string

const (
	ProviderOpenRouter = "openrouter"
	ProviderGemini     = "gemini"

	CapabilityTextToMusic Capability = "text_to_music"
)

type Model struct {
	ID                  string            `json:"id"`
	Provider            string            `json:"provider"`
	Name                string            `json:"name"`
	Capabilities        []Capability      `json:"capabilities"`
	InputModalities     []string          `json:"input_modalities,omitempty"`
	OutputModalities    []string          `json:"output_modalities,omitempty"`
	SupportedFormats    []string          `json:"supported_formats,omitempty"`
	SupportsStreaming   bool              `json:"supports_streaming"`
	DefaultOutputFormat string            `json:"default_output_format,omitempty"`
	Pricing             map[string]string `json:"pricing,omitempty"`
	Notes               string            `json:"notes,omitempty"`
}

type GenerateRequest struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Prompt       string  `json:"prompt"`
	Lyrics       string  `json:"lyrics,omitempty"`
	Instrumental bool    `json:"instrumental,omitempty"`
	VocalMode    string  `json:"vocal_mode,omitempty"`
	Options      Options `json:"options,omitempty"`
	SessionID    string  `json:"session_id,omitempty"`
	ParentID     string  `json:"parent_id,omitempty"`
	Title        string  `json:"title,omitempty"`
	Enhance      bool    `json:"enhance,omitempty"`
}

type Options struct {
	Genre           string   `json:"genre,omitempty"`
	Mood            string   `json:"mood,omitempty"`
	Era             string   `json:"era,omitempty"`
	Instruments     []string `json:"instruments,omitempty"`
	BPM             *int     `json:"bpm,omitempty"`
	Scale           string   `json:"scale,omitempty"`
	Duration        string   `json:"duration,omitempty"`
	Structure       string   `json:"structure,omitempty"`
	Language        string   `json:"language,omitempty"`
	EnergyCurve     string   `json:"energy_curve,omitempty"`
	ProductionNotes string   `json:"production_notes,omitempty"`
	NegativeSteer   string   `json:"negative_steer,omitempty"`
	Seed            *int64   `json:"seed,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type GenerationProgress struct {
	Stage        string `json:"stage"`
	Message      string `json:"message"`
	SessionID    string `json:"session_id,omitempty"`
	GenerationID string `json:"generation_id,omitempty"`
}

type GenerateResponse struct {
	Session    interface{} `json:"session"`
	Generation interface{} `json:"generation"`
	Asset      interface{} `json:"asset,omitempty"`
}

type ProvidersResponse struct {
	OpenRouter bool `json:"openrouter"`
	Gemini     bool `json:"gemini"`
}

type GenerationDetail struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"session_id"`
	ParentID        *string    `json:"parent_id,omitempty"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	Provider        string     `json:"provider"`
	Model           string     `json:"model"`
	Prompt          string     `json:"prompt"`
	AssembledPrompt string     `json:"assembled_prompt"`
	Lyrics          string     `json:"lyrics,omitempty"`
	Structure       string     `json:"structure,omitempty"`
	Error           *string    `json:"error,omitempty"`
	AssetID         string     `json:"asset_id,omitempty"`
	AssetURL        string     `json:"asset_url,omitempty"`
	MimeType        string     `json:"mime_type,omitempty"`
	CostUSD         *float64   `json:"cost_usd,omitempty"`
	DurationMS      *int64     `json:"duration_ms,omitempty"`
	OutputBytes     int64      `json:"output_bytes"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}
