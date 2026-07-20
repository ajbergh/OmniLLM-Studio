package models

import "time"

// VideoTranscript is one durable provider-backed speech-to-text result.
type VideoTranscript struct {
	ID                 string                   `json:"id"`
	ProjectID          string                   `json:"project_id"`
	AssetID            string                   `json:"asset_id"`
	UserID             *string                  `json:"user_id,omitempty"`
	ProviderProfileID  string                   `json:"provider_profile_id"`
	Provider           string                   `json:"provider"`
	Model              string                   `json:"model"`
	Status             string                   `json:"status"`
	Language           string                   `json:"language,omitempty"`
	TranslatedLanguage string                   `json:"translated_language,omitempty"`
	Text               string                   `json:"text,omitempty"`
	CostUSD            *float64                 `json:"cost_usd,omitempty"`
	PrivacyJSON        string                   `json:"privacy_json,omitempty"`
	MetadataJSON       string                   `json:"metadata_json,omitempty"`
	Error              *string                  `json:"error,omitempty"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
	CompletedAt        *time.Time               `json:"completed_at,omitempty"`
	Segments           []VideoTranscriptSegment `json:"segments,omitempty"`
}

// VideoTranscriptSegment stores segment and optional word timing data.
type VideoTranscriptSegment struct {
	ID           string   `json:"id"`
	TranscriptID string   `json:"transcript_id"`
	SegmentIndex int      `json:"segment_index"`
	StartMS      int      `json:"start_ms"`
	EndMS        int      `json:"end_ms"`
	Text         string   `json:"text"`
	Speaker      string   `json:"speaker,omitempty"`
	Confidence   *float64 `json:"confidence,omitempty"`
	WordsJSON    string   `json:"words_json,omitempty"`
}
