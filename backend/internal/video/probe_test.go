package video

import "testing"

func TestParseProbePayload(t *testing.T) {
	payload := []byte(`{
		"format": {"duration": "12.480000"},
		"streams": [
			{"codec_type": "audio", "duration": "12.48"},
			{"codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30000/1001", "avg_frame_rate": "30000/1001"}
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
