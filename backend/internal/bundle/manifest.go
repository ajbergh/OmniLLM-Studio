package bundle

import (
	"encoding/json"
	"fmt"
	"time"
)

// CurrentFormatVersion is the bundle format version produced by this build.
const CurrentFormatVersion = 2

// Manifest describes an export bundle.
type Manifest struct {
	FormatVersion int           `json:"format_version"`
	AppVersion    string        `json:"app_version"`
	SchemaVersion int           `json:"schema_version"`
	CreatedAt     time.Time     `json:"created_at"`
	Stats         ManifestStats `json:"stats"`
}

// ManifestStats contains counts of exported entities.
type ManifestStats struct {
	Conversations int `json:"conversations"`
	Messages      int `json:"messages"`
	Attachments   int `json:"attachments"`
	Providers     int `json:"providers"`
}

// ValidateCompatibility checks whether this bundle can be imported.
func (m *Manifest) ValidateCompatibility(currentSchemaVersion int) []string {
	var warnings []string

	if m.FormatVersion > CurrentFormatVersion {
		warnings = append(warnings, fmt.Sprintf(
			"bundle format version %d is newer than supported version %d; some data may be skipped",
			m.FormatVersion, CurrentFormatVersion,
		))
	}

	if m.SchemaVersion > currentSchemaVersion {
		warnings = append(warnings, fmt.Sprintf(
			"bundle schema version %d is newer than current %d; some tables may not exist",
			m.SchemaVersion, currentSchemaVersion,
		))
	}

	return warnings
}

// MarshalManifest produces pretty-printed JSON.
func MarshalManifest(m *Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// UnmarshalManifest parses a manifest from JSON.
func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if m.FormatVersion == 0 {
		return nil, fmt.Errorf("manifest missing format_version")
	}
	return &m, nil
}

// ValidationReport is returned by the validate-before-import endpoint.
type ValidationReport struct {
	Manifest *Manifest `json:"manifest"`
	Valid    bool      `json:"valid"`
	Warnings []string  `json:"warnings,omitempty"`
	Errors   []string  `json:"errors,omitempty"`
}
