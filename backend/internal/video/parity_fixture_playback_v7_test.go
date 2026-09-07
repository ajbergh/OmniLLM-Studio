package video

import "testing"

func TestPlaybackCanonicalParityFixtureV7Valid(t *testing.T) {
	doc, assets, cases := PlaybackCanonicalParityFixtureV7()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate playback parity v7 fixture: %v", err)
	}
	if validated.DurationMS != 39600 || validated.Canvas.FPS != 30 {
		t.Fatalf("unexpected playback parity v7 canvas/duration: %+v / %d", validated.Canvas, validated.DurationMS)
	}
	if len(assets) != 5 {
		t.Fatalf("playback parity v7 assets = %d, want 5", len(assets))
	}
	if len(cases) != 21 {
		t.Fatalf("playback parity v7 cases = %d, want 21", len(cases))
	}
	if got, _ := validated.Metadata["fixture"].(string); got != PlaybackCanonicalParityFixtureV7Name {
		t.Fatalf("playback parity v7 metadata fixture = %q, want %q", got, PlaybackCanonicalParityFixtureV7Name)
	}

	seenCases := map[string]PlaybackParityCase{}
	for _, testCase := range cases {
		seenCases[testCase.Name] = testCase
	}
	roundedCase, ok := seenCases["rounded-rectangle-canonical-playback"]
	if !ok {
		t.Fatal("rounded rectangle canonical playback case is missing")
	}
	if roundedCase.ExpectedMode != "canonical-playback" || roundedCase.ExpectedTransitionMode != "canonical-none" || !roundedCase.RequireAdvancingFrames {
		t.Fatalf("unexpected rounded rectangle playback case: %+v", roundedCase)
	}
	ellipseCase, ok := seenCases["unsupported-ellipse-fallback"]
	if !ok {
		t.Fatal("unsupported ellipse playback case is missing")
	}
	if ellipseCase.ExpectedMode != "legacy-time-fallback" || ellipseCase.ExpectedReason != "shape-playback-deferred:playback-shape-ellipse:shape-kind-unsupported" || ellipseCase.ExpectedTransitionMode != "legacy" {
		t.Fatalf("unexpected ellipse fallback case: %+v", ellipseCase)
	}

	rounded := findPlaybackFixtureClip(validated, "playback-shape-rounded-rectangle")
	if rounded == nil || rounded.Shape == nil {
		t.Fatal("rounded rectangle playback clip is missing")
	}
	if rounded.Shape.Kind != ShapeKindRoundedRectangle || rounded.Shape.Width != 240 || rounded.Shape.Height != 120 || rounded.Shape.CornerRadius != 24 || rounded.Shape.StrokeWidth != 8 {
		t.Fatalf("unexpected rounded rectangle playback geometry: %+v", rounded.Shape)
	}
	if rounded.AssetID != "" || rounded.Text != nil || rounded.Cursor != nil || rounded.FadeInMS != 0 || rounded.FadeOutMS != 0 || len(rounded.Effects) != 0 || len(rounded.Keyframes) != 0 || len(rounded.Transitions) != 0 || len(rounded.AnimationBlocks) != 0 {
		t.Fatalf("rounded rectangle playback clip escaped static standalone subset: %+v", rounded)
	}

	ellipse := findPlaybackFixtureClip(validated, "playback-shape-ellipse")
	if ellipse == nil || ellipse.Shape == nil || ellipse.Shape.Kind != ShapeKindEllipse {
		t.Fatalf("unsupported ellipse playback clip is missing or invalid: %+v", ellipse)
	}
}

func findPlaybackFixtureClip(doc TimelineDocument, clipID string) *TimelineClip {
	for trackIndex := range doc.Tracks {
		for clipIndex := range doc.Tracks[trackIndex].Clips {
			clip := &doc.Tracks[trackIndex].Clips[clipIndex]
			if clip.ID == clipID {
				return clip
			}
		}
	}
	return nil
}
