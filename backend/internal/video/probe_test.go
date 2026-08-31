package video

import (
	"encoding/json"
	"testing"
)

func TestParseProbePayload(t *testing.T) {
	payload := []byte(`{
		"format": {"duration": "12.480000"},
		"streams": [
			{"codec_type": "audio", "codec_name": "aac", "duration": "12.48", "channels": 2, "sample_rate": "48000"},
			{"codec_type": "video", "codec_name": "h264", "pix_fmt": "yuv420p", "width": 1920, "height": 1080, "r_frame_rate": "30000/1001", "avg_frame_rate": "30000/1001"}
		]
	}`)
	probe, err := parseProbePayload(payload)
	if err != nil {
		t.Fatalf("parseProbePayload returned error: %v", err)
	}
	if probe == nil {
		t.Fatal("expected probe data")
	}
	if probe.DurationMS != 12480 {
		t.Errorf("duration = %d, want 12480", probe.DurationMS)
	}
	if probe.Width != 1920 || probe.Height != 1080 {
		t.Errorf("dimensions = %dx%d, want 1920x1080", probe.Width, probe.Height)
	}
	if probe.FPS < 29.9 || probe.FPS > 30 {
		t.Errorf("fps = %f, want ≈29.97", probe.FPS)
	}
	if probe.VideoCodec != "h264" || probe.VideoPixelFormat != "yuv420p" || probe.AudioCodec != "aac" {
		t.Errorf("stream metadata = %+v, want h264/yuv420p and aac", probe)
	}
	if !probe.HasAudio || probe.AudioChannels != 2 || probe.AudioSampleRate != 48000 {
		t.Errorf("audio stream = %+v, want 2ch @ 48000", probe)
	}
}

func TestParseProbePayloadAudioOnlyAndEmpty(t *testing.T) {
	probe, err := parseProbePayload([]byte(`{"format": {"duration": "3.5"}, "streams": [{"codec_type": "audio"}]}`))
	if err != nil || probe == nil || probe.DurationMS != 3500 {
		t.Fatalf("audio-only probe = %+v err=%v, want duration 3500", probe, err)
	}
	probe, err = parseProbePayload([]byte(`{"format": {}, "streams": []}`))
	if err != nil || probe != nil {
		t.Fatalf("empty payload should yield nil probe without error, got %+v err=%v", probe, err)
	}
}

func TestParseProbePayloadPreservesVideoAlphaFacts(t *testing.T) {
	payload := []byte(`{
		"format": {"duration": "2.0"},
		"streams": [{
			"codec_type": "video",
			"codec_name": "vp9",
			"pix_fmt": "yuv420p",
			"width": 512,
			"height": 512,
			"avg_frame_rate": "30/1",
			"tags": {"alpha_mode": "1"}
		}]
	}`)
	probe, err := parseProbePayload(payload)
	if err != nil || probe == nil {
		t.Fatalf("alpha probe = %+v err=%v", probe, err)
	}
	if probe.VideoCodec != "vp9" || probe.VideoPixelFormat != "yuv420p" || probe.VideoAlphaMode != "1" || !probe.VideoHasAlpha() {
		t.Fatalf("alpha stream facts = %+v", probe)
	}
	metadata := mergeProbeMetadataJSON(`{"source":"fixture","video_codec":"stale"}`, probe)
	var got map[string]any
	if err := json.Unmarshal([]byte(metadata), &got); err != nil {
		t.Fatalf("merged metadata: %v", err)
	}
	if got["source"] != "fixture" || got["video_codec"] != "vp9" || got["video_alpha_mode"] != "1" {
		t.Fatalf("merged metadata = %#v", got)
	}
}

func TestVideoPixelFormatHasAlpha(t *testing.T) {
	for _, pixelFormat := range []string{"yuva420p", "gbrap", "rgba", "bgra"} {
		if !videoPixelFormatHasAlpha(pixelFormat) {
			t.Errorf("videoPixelFormatHasAlpha(%q) = false, want true", pixelFormat)
		}
	}
	for _, pixelFormat := range []string{"yuv420p", "rgb24", ""} {
		if videoPixelFormatHasAlpha(pixelFormat) {
			t.Errorf("videoPixelFormatHasAlpha(%q) = true, want false", pixelFormat)
		}
	}
}

func TestParseFrameRate(t *testing.T) {
	cases := map[string]float64{
		"30/1":       30,
		"30000/1001": 29.97002997002997,
		"25":         25,
		"0/0":        0,
		"":           0,
		"abc":        0,
		"30/0":       0,
	}
	for input, want := range cases {
		if got := parseFrameRate(input); got != want {
			t.Errorf("parseFrameRate(%q) = %f, want %f", input, got, want)
		}
	}
}
