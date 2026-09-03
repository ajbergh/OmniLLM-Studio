package video

import "testing"

func TestPlaybackCanonicalParityFixtureValid(t *testing.T) {
	doc, assets, cases := PlaybackCanonicalParityFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate playback parity fixture: %v", err)
	}
	if validated.DurationMS != 38000 || validated.Canvas.FPS != 30 {
		t.Fatalf("unexpected playback parity canvas/duration: %+v / %d", validated.Canvas, validated.DurationMS)
	}
	if len(assets) != 5 {
		t.Fatalf("playback parity assets = %d, want 5", len(assets))
	}
	if len(cases) != 17 {
		t.Fatalf("playback parity cases = %d, want 17", len(cases))
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
			if testCase.ExpectedWeightedRuntime != "ready" {
				t.Fatalf("weighted Canvas case %q must expect a ready weighted runtime", testCase.Name)
			}
			if testCase.ExpectedWeightedPairID == "" {
				t.Fatalf("weighted Canvas case %q is missing pair identity", testCase.Name)
			}
			if testCase.ExpectedMode == "canonical-playback" && testCase.ExpectedWeightedConsumer != "canonical-weighted-canvas" {
				t.Fatalf("canonical weighted Canvas case %q is missing canonical consumer expectation", testCase.Name)
			}
			if testCase.ExpectedMode == "legacy-time-fallback" && testCase.ExpectedWeightedConsumer != "legacy-time-fallback" {
				t.Fatalf("fallback weighted Canvas case %q must keep the consumer hidden", testCase.Name)
			}
		}
		if testCase.ExpectedTextRuntime != "" && testCase.ExpectedTextClipID == "" {
			t.Fatalf("text runtime case %q is missing an expected clip id", testCase.Name)
		}
		if testCase.RequireTextLayout {
			if testCase.ExpectedTextRuntime != "ready" {
				t.Fatalf("text layout case %q must expect a ready text runtime", testCase.Name)
			}
			if testCase.ExpectedTextConsumer != "canonical-text-dom" && testCase.ExpectedTextConsumer != "legacy-time-fallback" {
				t.Fatalf("text layout case %q has invalid consumer %q", testCase.Name, testCase.ExpectedTextConsumer)
			}
			if len(testCase.ExpectedTextTrace) == 0 {
				t.Fatalf("text layout case %q is missing readiness trace expectations", testCase.Name)
			}
		}
		if testCase.DecoderBudget < 0 {
			t.Fatalf("playback parity case %q has invalid decoder budget %d", testCase.Name, testCase.DecoderBudget)
		}
	}
	for _, required := range []string{
		"video-canonical-playback",
		"image-canonical-playback",
		"resource-text-canonical-playback",
		"family-text-fallback",
		"invalid-font-text-fallback",
		"cursor-fallback",
		"mixed-text-cursor-fallback",
		"weighted-crossfade-canonical",
		"weighted-zoom-canonical",
		"weighted-dip-canonical",
		"weighted-decoder-budget-fallback",
		"mixed-transition-fallback",
		"deferred-transition-fallback",
		"media-text-canonical-playback",
		"weighted-text-canonical-playback",
		"weighted-invalid-text-fallback",
		"weighted-text-decoder-budget-fallback",
	} {
		if !seen[required] {
			t.Fatalf("playback parity case %q is missing", required)
		}
	}

	resourceIDs := map[string]bool{}
	for _, track := range validated.Tracks {
		for _, clip := range track.Clips {
			if clip.Text != nil && clip.Text.FontResourceID != "" {
				resourceIDs[clip.Text.FontResourceID] = true
			}
		}
	}
	for _, resourceID := range []string{"playback-font-v1", "playback-font-invalid-v1"} {
		if !resourceIDs[resourceID] {
			t.Fatalf("playback parity font resource %q is missing", resourceID)
		}
	}
}
