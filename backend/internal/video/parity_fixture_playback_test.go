package video

import "testing"

func TestPlaybackCanonicalParityFixtureValid(t *testing.T) {
	doc, assets, cases := PlaybackCanonicalParityFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate playback parity fixture: %v", err)
	}
	if validated.DurationMS != 18000 || validated.Canvas.FPS != 30 {
		t.Fatalf("unexpected playback parity canvas/duration: %+v / %d", validated.Canvas, validated.DurationMS)
	}
	if len(assets) != 3 {
		t.Fatalf("playback parity assets = %d, want 3", len(assets))
	}
	if len(cases) != 7 {
		t.Fatalf("playback parity cases = %d, want 7", len(cases))
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
	}
	for _, required := range []string{
		"video-canonical-playback",
		"image-canonical-playback",
		"text-fallback",
		"cursor-fallback",
		"weighted-transition-fallback",
		"mixed-transition-fallback",
		"deferred-transition-fallback",
	} {
		if !seen[required] {
			t.Fatalf("playback parity case %q is missing", required)
		}
	}
}
