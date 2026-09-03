package video

import "testing"

func TestParityResourceTextFixtureIsNarrowAndResourceBacked(t *testing.T) {
	doc, assets := ParityResourceTextFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument: %v", err)
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 30 || validated.Canvas.Background != "#000000" {
		t.Fatalf("canvas = %+v", validated.Canvas)
	}
	if len(validated.Tracks) != 1 || len(validated.Tracks[0].Clips) != 1 {
		t.Fatalf("tracks = %+v", validated.Tracks)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.Text == nil {
		t.Fatal("resource text clip has no text state")
	}
	if clip.Text.FontResourceID != ParityResourceTextFontID || clip.Text.FontFamily != "DejaVu Sans" {
		t.Fatalf("font binding = %+v", *clip.Text)
	}
	if clip.Text.FontSize != 48 || clip.Text.FontWeight != "400" || clip.Text.Color != "#ffffff" {
		t.Fatalf("text style = %+v", *clip.Text)
	}
	if clip.Text.Background != "" || clip.Text.Stroke != "" || clip.Text.StrokeWidth != 0 || clip.Text.Shadow || clip.Text.LineHeight != 0 || clip.Text.LetterSpacing != 0 {
		t.Fatalf("fixture introduced deferred text decoration/metrics: %+v", *clip.Text)
	}
	if len(clip.Keyframes) != 0 || len(clip.Effects) != 0 {
		t.Fatalf("fixture introduced animation/effects: %+v", clip)
	}
	if len(assets) != 1 || assets[0].ID != "asset-font" || assets[0].Kind != "font" {
		t.Fatalf("assets = %+v", assets)
	}
	samples := ParityResourceTextFrameSamples()
	if len(samples) != 1 || samples[0].FrameIndex != 15 || samples[0].TimeMS != 500 {
		t.Fatalf("samples = %+v", samples)
	}
}
