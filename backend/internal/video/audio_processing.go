package video

import (
	"fmt"
	"strings"
)

// timelineAudioProcessingFilters returns optional per-input FFmpeg audio
// filters from timeline metadata. The editor writes this object under
// metadata.render_audio_processing.
func timelineAudioProcessingFilters(metadata map[string]any) []string {
	raw, ok := metadata["render_audio_processing"].(map[string]any)
	if !ok {
		return nil
	}
	filters := make([]string, 0, 8)
	if metadataBool(raw, "denoise") {
		filters = append(filters, "highpass=f=70", "afftdn=nr=12:nf=-35")
	}
	switch strings.ToLower(metadataString(raw, "eq_preset")) {
	case "voice":
		filters = append(filters, "equalizer=f=120:t=q:w=1:g=-3", "equalizer=f=3000:t=q:w=1:g=2")
	case "warm":
		filters = append(filters, "equalizer=f=180:t=q:w=1:g=2", "equalizer=f=6000:t=q:w=1:g=-1")
	case "bright":
		filters = append(filters, "equalizer=f=4500:t=q:w=1:g=2")
	}
	if metadataBool(raw, "compressor") {
		filters = append(filters, "acompressor=threshold=-18dB:ratio=3:attack=15:release=180:makeup=2")
	}
	if metadataBool(raw, "normalize") {
		target := metadataNumber(raw, "target_lufs", -16)
		if target > -5 {
			target = -5
		}
		if target < -30 {
			target = -30
		}
		filters = append(filters, fmt.Sprintf("loudnorm=I=%.1f:TP=-1.5:LRA=11", target))
	}
	if metadataBool(raw, "limiter") {
		filters = append(filters, "alimiter=limit=0.95:attack=5:release=50")
	}
	switch strings.ToLower(metadataString(raw, "channels")) {
	case "mono":
		filters = append(filters, "aformat=channel_layouts=mono")
	case "stereo":
		filters = append(filters, "aformat=channel_layouts=stereo")
	}
	return filters
}

func metadataBool(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func metadataString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func metadataNumber(values map[string]any, key string, fallback float64) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case jsonNumber:
		parsed, err := value.Float64()
		if err == nil {
			return parsed
		}
	}
	return fallback
}

// jsonNumber captures the only method needed from encoding/json.Number without
// importing encoding/json solely for a type assertion.
type jsonNumber interface {
	Float64() (float64, error)
}
