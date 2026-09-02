package video

import "testing"

func TestPlaybackCanonicalParityFixtureValid(t *testing.T) {
	doc, assets, cases := PlaybackCanonicalParityFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate playback parity fixture: %v", err)
	}
	if validated.DurationMS != 22000 || validated.Canvas.FPS != 30 {
		t.Fatalf("unexpected playback parity canvas/duration: %+v / %d", validated.Canvas, validated.DurationMS)
	}
	if len(assets) != 3 {
		t.Fatalf("playback parity assets = %d, want 3", len(assets))
	}
	if len(cases) != 10 {
		t.Fatalf("playback parity cases = %d, want 10", len(cases))
	}
	seen := map[string]bool{}
	for _, testCase := range cases {
		if testCase.Name == "" || seen[testCase.Name] {
			t.Fatalf("invalid duplicate playback parity case %q", testCase.Name)
		}
		seen[testCase.Name] = true
		if testCase.FrameIndex < 0 || testCase.ObserveMS < 250 {
			t.Fatalf("invalid playback parity case timing: %+v", testCase)
		}
		if testCase.ExpectedMode != "canonical-playback" && testCase.ExpectedMode != "legacy-time-fallback" {
			t.Fatalf("unexpected playback parity mode %q", testCase.ExpectedMode)
		}
		if testCase.ExpectedMode == "legacy-time-fallback" && testCase.ExpectedReason == "" {
			t.Fatalf("fallback case %q is missing an expected reason", testCase.Name)
		}
		if testCase.ExpectedWeightedRuntime != "" && testCase.ExpectedWeightedPairID == "" {
			t.Fatalf("weighted runtime case %q is missing an expected pair id", testCase.Name)
		}
		if testCase.RequireWeightedCanvas {
			if testCase.ExpectedMode != "canonical-playback" {
				t.Fatalf("weighted Canvas case %q must be canonical playback", testCase.Name)
			}
			if testCase.ExpectedWeightedRuntime != "ready" || testCase.ExpectedWeightedConsumer != "canonical-weighted-canvas" {
				t.Fatalf("weighted Canvas case %q is missing ready consumer expectations", testCase.Name)
			}
			if testCase.ExpectedWeightedPairID == "" {
				t.Fatalf("weighted Canvas case %q is missing pair identity", testCase.Name)
			}
		}
		if testCase.DecoderBudget < 0 {
			t.Fatalf("playback parity case %q has invalid decoder budget %d", testCase.Name, testCase.DecoderBudget)
		}
	}
	for _, required := range []string{
		"video-canonical-playback",
		"image-canonical-playback",
		"text-fallback",
		"cursor-fallback",
		"weighted-crossfade-canonical",
		"weighted-zoom-canonical",
		"weighted-dip-canonical",
		"weighted-decoder-budget-fallback",
		"mixed-transition-fallback",
		"deferred-transition-fallback",
	} {
		if !seen[required] {
			t.Fatalf("playback parity case %q is missing", required)
		}
	}
}
