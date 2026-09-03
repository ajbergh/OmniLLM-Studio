package video

import "testing"

func TestParityResourceTextFixtureIsNarrowAndResourceBacked(t *testing.T) {
	doc, assets := ParityResourceTextFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument: %v", err)
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 30 {
		t.Fatalf("canvas = %+v", validated.Canvas)
	}
	if len(validated.Tracks) != 2 || len(validated.Tracks[0].Clips) != 1 {
		t.Fatalf("tracks = %+v", validated.Tracks)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.Text == nil {
		t.Fatal("resource text clip has no text state")
	}
	if clip.Text.FontResourceID != ParityResourceTextFontID {
		t.Fatalf("font resource = %q", clip.Text.FontResourceID)
	}
	if clip.Text.FontSize != 48 || clip.Text.FontWeight != "400" || clip.Text.Color != "#f7f8fa" {
		t.Fatalf("text style = %+v", *clip.Text)
	}
	if clip.Text.Background != "" || clip.Text.Stroke != "" || clip.Text.StrokeWidth != 0 || clip.Text.Shadow || clip.Text.LineHeight != 0 || clip.Text.LetterSpacing != 0 {
		t.Fatalf("fixture introduced deferred text decoration/metrics: %+v", *clip.Text)
	}
	if len(clip.Keyframes) != 0 || len(clip.Effects) != 0 {
		t.Fatalf("fixture introduced animation/effects: %+v", clip)
	}
	if len(assets) != 2 || assets[0].ID != "asset-font" || assets[0].Kind != "font" || assets[1].ID != "asset-audio" {
		t.Fatalf("assets = %+v", assets)
	}
	samples := ParityResourceTextFrameSamples()
	if len(samples) != 1 || samples[0].FrameIndex != 15 || samples[0].TimeMS != 500 {
		t.Fatalf("samples = %+v", samples)
	}
}
