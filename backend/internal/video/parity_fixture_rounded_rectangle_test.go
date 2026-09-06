package video

import "testing"

func TestParityRoundedRectangleFixtureValid(t *testing.T) {
	doc, assets := ParityRoundedRectangleFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate rounded rectangle fixture: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("rounded rectangle fixture assets = %d, want 0", len(assets))
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 30 {
		t.Fatalf("rounded rectangle fixture canvas drifted: %+v", validated.Canvas)
	}
	if len(validated.Tracks) != 1 || len(validated.Tracks[0].Clips) != 1 {
		t.Fatalf("rounded rectangle fixture topology drifted: %+v", validated.Tracks)
	}
	shape := validated.Tracks[0].Clips[0].Shape
	if shape == nil || shape.Kind != ShapeKindRoundedRectangle || shape.Width != 240 || shape.Height != 120 || shape.CornerRadius != 24 || shape.StrokeWidth != 8 {
		t.Fatalf("rounded rectangle fixture shape drifted: %+v", shape)
	}
	samples := ParityRoundedRectangleFrameSamples()
	if len(samples) != 1 || samples[0].FrameIndex != 15 || samples[0].TimeMS != 500 {
		t.Fatalf("rounded rectangle fixture samples drifted: %+v", samples)
	}
}
