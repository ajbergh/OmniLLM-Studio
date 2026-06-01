package video

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

const (
	maxProviderJSONBytes     = 8 << 20
	maxProviderDownloadBytes = 512 << 20
)

func assembleProviderPrompt(req GenerateRequest) string {
	prompt := strings.TrimSpace(req.EnhancedPrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	var parts []string
	if prompt != "" {
		parts = append(parts, prompt)
	}
	if value := strings.TrimSpace(req.StylePreset); value != "" {
		parts = append(parts, "Style: "+value)
	}
	if value := strings.TrimSpace(req.CameraMotion); value != "" {
		parts = append(parts, "Camera motion: "+value)
	}
	if value := strings.TrimSpace(req.ShotType); value != "" {
		parts = append(parts, "Shot type: "+value)
	}
	if value := strings.TrimSpace(req.Composition); value != "" {
		parts = append(parts, "Composition: "+value)
	}
	if value := strings.TrimSpace(req.LensEffect); value != "" {
		parts = append(parts, "Lens/focus: "+value)
	}
	if value := strings.TrimSpace(req.Lighting); value != "" {
		parts = append(parts, "Lighting: "+value)
	}
	if value := strings.TrimSpace(req.Dialogue); value != "" {
		parts = append(parts, "Dialogue: "+value)
	}
	if value := strings.TrimSpace(req.SoundEffects); value != "" {
		parts = append(parts, "Sound effects: "+value)
	}
	if value := strings.TrimSpace(req.AmbientNoise); value != "" {
		parts = append(parts, "Ambient noise: "+value)
	}
	if value := strings.TrimSpace(req.ContinuityNotes); value != "" {
		parts = append(parts, "Continuity: "+value)
	}
	if value := strings.TrimSpace(req.ProductionNotes); value != "" {
		parts = append(parts, "Production notes: "+value)
	}
	return strings.Join(parts, "\n\n")
}

// assembleCinematicNotes builds a compact structured summary of the cinematic
// detail fields on req (style, composition, lens, lighting, audio cues, etc.)
// for use as the ProductionNotes hint to the LLM prompt enhancer.
func assembleCinematicNotes(req GenerateRequest) string {
	type kv struct{ k, v string }
	fields := []kv{
		{"Style", req.StylePreset},
		{"Camera", req.CameraMotion},
		{"Shot type", req.ShotType},
		{"Composition", req.Composition},
		{"Lens/focus", req.LensEffect},
		{"Lighting", req.Lighting},
		{"Dialogue", req.Dialogue},
		{"Sound effects", req.SoundEffects},
		{"Ambient noise", req.AmbientNoise},
		{"Continuity", req.ContinuityNotes},
		{"Production notes", req.ProductionNotes},
	}
	var parts []string
	for _, f := range fields {
		if v := strings.TrimSpace(f.v); v != "" {
			parts = append(parts, f.k+": "+v)
		}
	}
	return strings.Join(parts, "\n")
}

func mergeAllowedVideoSettings(target map[string]any, raw json.RawMessage, allowed map[string]bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return
	}
	for key, value := range settings {
		if allowed[key] {
			target[key] = value
		}
	}
}

func modelsWithDiscoveryNote(models []Model, note string) []Model {
	out := make([]Model, len(models))
	copy(out, models)
	for i := range out {
		if out[i].Notes == "" {
			out[i].Notes = note
		} else if !strings.Contains(out[i].Notes, note) {
			out[i].Notes += " " + note
		}
	}
	return out
}

func cloneStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 1 {
		return value[:max]
	}
	return strings.TrimSpace(value[:max-1]) + "..."
}

func responseSnippet(body []byte) string {
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		return "<empty response>"
	}
	return truncateString(snippet, 500)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstRawMessage(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		trimmed := strings.TrimSpace(string(value))
		if trimmed != "" && trimmed != "null" {
			return value
		}
	}
	return nil
}

func stringPtrIfNotEmpty(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func costFromUsage(raw json.RawMessage) *float64 {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	var usage map[string]any
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil
	}
	for _, key := range []string{"cost", "total_cost", "upstream_inference_cost"} {
		if value, ok := usage[key]; ok {
			if cost, ok := floatFromAny(value); ok {
				return &cost
			}
		}
	}
	return nil
}

func floatFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
			return typed, true
		}
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		if f, err := typed.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
