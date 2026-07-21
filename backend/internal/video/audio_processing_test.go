package video

import (
	"reflect"
	"strings"
	"testing"
)

func TestTimelineAudioProcessingFilters(t *testing.T) {
	metadata := map[string]any{
		"render_audio_processing": map[string]any{
			"denoise":     true,
			"eq_preset":   "voice",
			"compressor":  true,
			"normalize":   true,
			"target_lufs": -14.0,
			"limiter":     true,
			"channels":    "stereo",
		},
	}
	filters := timelineAudioProcessingFilters(metadata)
	joined := strings.Join(filters, ",")
	for _, expected := range []string{
		"highpass=f=70",
		"afftdn=nr=12:nf=-35",
		"equalizer=f=120",
		"acompressor=",
		"loudnorm=I=-14.0",
		"alimiter=",
		"aformat=channel_layouts=stereo",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in filter chain %q", expected, joined)
		}
	}
}

func TestTimelineAudioProcessingFiltersClampLoudness(t *testing.T) {
	tooLoud := timelineAudioProcessingFilters(map[string]any{
		"render_audio_processing": map[string]any{"normalize": true, "target_lufs": 20.0},
	})
	if !strings.Contains(strings.Join(tooLoud, ","), "loudnorm=I=-5.0") {
		t.Fatalf("expected upper LUFS clamp, got %v", tooLoud)
	}
	tooQuiet := timelineAudioProcessingFilters(map[string]any{
		"render_audio_processing": map[string]any{"normalize": true, "target_lufs": -80.0},
	})
	if !strings.Contains(strings.Join(tooQuiet, ","), "loudnorm=I=-30.0") {
		t.Fatalf("expected lower LUFS clamp, got %v", tooQuiet)
	}
}

func TestTimelineAudioProcessingFiltersIgnoreUnknownMetadata(t *testing.T) {
	if filters := timelineAudioProcessingFilters(nil); filters != nil {
		t.Fatalf("expected nil filters, got %v", filters)
	}
	if filters := timelineAudioProcessingFilters(map[string]any{
		"render_audio_processing": "invalid",
	}); filters != nil {
		t.Fatalf("expected nil filters for invalid metadata, got %v", filters)
	}
	if filters := timelineAudioProcessingFilters(map[string]any{
		"render_audio_processing": map[string]any{"eq_preset": "unknown", "channels": "source"},
	}); !reflect.DeepEqual(filters, []string{}) {
		t.Fatalf("expected empty filter chain, got %v", filters)
	}
}
