package video

import "testing"

func TestParityCursorSmoothingFixtureIsNarrowAndAsymmetric(t *testing.T) {
	doc, assets := ParityCursorSmoothingFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument: %v", err)
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 100 || validated.Canvas.Background != "#000000" {
		t.Fatalf("canvas = %+v", validated.Canvas)
	}
	if validated.DurationMS != 1001 || len(validated.Tracks) != 1 || len(validated.Tracks[0].Clips) != 1 {
		t.Fatalf("timeline = %+v", validated)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.AssetID != ParityCursorSmoothingBackdropAssetID || clip.Cursor == nil {
		t.Fatalf("cursor smoothing fixture clip = %+v", clip)
	}
	if !clip.Cursor.Visible || clip.Cursor.Scale != 1 || !clip.Cursor.Highlight || clip.Cursor.ClickRings || !clip.Cursor.Smoothing {
		t.Fatalf("cursor smoothing state = %+v", *clip.Cursor)
	}
	if len(clip.Cursor.Events) != 2 || clip.Cursor.Events[0].TimeMS != 0 || clip.Cursor.Events[1].TimeMS != 1000 {
		t.Fatalf("cursor smoothing events = %+v", clip.Cursor.Events)
	}
	if clip.Cursor.Events[0].X != 160 || clip.Cursor.Events[0].Y != 100 || clip.Cursor.Events[1].X != 480 || clip.Cursor.Events[1].Y != 260 {
		t.Fatalf("cursor smoothing coordinates = %+v", clip.Cursor.Events)
	}
	if len(clip.Keyframes) != 0 || len(clip.AnimationBlocks) != 0 || len(clip.Effects) != 0 || len(clip.Transitions) != 0 || clip.FadeInMS != 0 || clip.FadeOutMS != 0 {
		t.Fatalf("fixture introduced unrelated renderer debt: %+v", clip)
	}
	if len(assets) != 1 || assets[0].ID != ParityCursorSmoothingBackdropAssetID || assets[0].Kind != "image" {
		t.Fatalf("assets = %+v", assets)
	}
	samples := ParityCursorSmoothingFrameSamples()
	wantFrames := []int64{25, 50, 75}
	wantMS := []int64{250, 500, 750}
	if len(samples) != len(wantFrames) {
		t.Fatalf("samples = %+v", samples)
	}
	for index := range samples {
		if samples[index].FrameIndex != wantFrames[index] || samples[index].TimeMS != wantMS[index] {
			t.Fatalf("sample %d = %+v", index, samples[index])
		}
	}
}
