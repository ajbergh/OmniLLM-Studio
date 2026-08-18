package rendercontract

import "testing"

func TestTimelineV2ConstantsMatchSchema(t *testing.T) {
	schema := loadProjectionSchema(t, "timeline-v2.schema.json")
	version, ok := schema.Properties["version"].Const.(float64)
	if !ok || int(version) != TimelineV2Version {
		t.Fatalf("timeline version const = %v, want %d", schema.Properties["version"].Const, TimelineV2Version)
	}
	workingColor := schema.Properties["working_color_space"].Enum
	if len(workingColor) != 1 || workingColor[0] != RenderWorkingColorSpaceSRGB {
		t.Fatalf("timeline working color space = %v, want %q", workingColor, RenderWorkingColorSpaceSRGB)
	}
}

func TestRenderManifestConstantsMatchSchema(t *testing.T) {
	schema := loadProjectionSchema(t, "render-manifest-v1.schema.json")
	version, ok := schema.Properties["version"].Const.(float64)
	if !ok || int(version) != RenderManifestV1Version {
		t.Fatalf("manifest version const = %v, want %d", schema.Properties["version"].Const, RenderManifestV1Version)
	}
	if got := schema.Properties["contract_version"].Const; got != RenderContractTimelineV2 {
		t.Fatalf("contract version = %v, want %q", got, RenderContractTimelineV2)
	}
	settings := schema.Defs["settings"]
	sampleRate, ok := settings.Properties["audio_sample_rate"].Const.(float64)
	if !ok || int(sampleRate) != RenderAudioSampleRate {
		t.Fatalf("audio sample rate = %v, want %d", settings.Properties["audio_sample_rate"].Const, RenderAudioSampleRate)
	}
	channels, ok := settings.Properties["audio_channels"].Const.(float64)
	if !ok || int(channels) != RenderAudioChannels {
		t.Fatalf("audio channels = %v, want %d", settings.Properties["audio_channels"].Const, RenderAudioChannels)
	}
	if got := settings.Properties["working_color_space"].Const; got != RenderWorkingColorSpaceSRGB {
		t.Fatalf("manifest working color space = %v, want %q", got, RenderWorkingColorSpaceSRGB)
	}
}
