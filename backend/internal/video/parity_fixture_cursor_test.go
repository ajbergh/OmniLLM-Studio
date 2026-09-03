package video

import "testing"

func TestParityCursorFixtureIsNarrowAndBoundaryAddressed(t *testing.T) {
	doc, assets := ParityCursorFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument: %v", err)
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 100 || validated.Canvas.Background != "#000000" {
		t.Fatalf("canvas = %+v", validated.Canvas)
	}
	if len(validated.Tracks) != 1 || len(validated.Tracks[0].Clips) != 1 {
		t.Fatalf("tracks = %+v", validated.Tracks)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.AssetID != ParityCursorBackdropAssetID || clip.Cursor == nil {
		t.Fatalf("cursor fixture clip = %+v", clip)
	}
	if !clip.Cursor.Visible || clip.Cursor.Scale != 1 || !clip.Cursor.Highlight || !clip.Cursor.ClickRings || clip.Cursor.Smoothing {
		t.Fatalf("cursor state = %+v", *clip.Cursor)
	}
	if len(clip.Cursor.Events) != 3 || !clip.Cursor.Events[1].Click || clip.Cursor.Events[1].TimeMS != 500 {
		t.Fatalf("cursor events = %+v", clip.Cursor.Events)
	}
	if len(clip.Keyframes) != 0 || len(clip.AnimationBlocks) != 0 || len(clip.Effects) != 0 || len(clip.Transitions) != 0 || clip.FadeInMS != 0 || clip.FadeOutMS != 0 {
		t.Fatalf("fixture introduced deferred animation/effects: %+v", clip)
	}
	if len(assets) != 1 || assets[0].ID != ParityCursorBackdropAssetID || assets[0].Kind != "image" {
		t.Fatalf("assets = %+v", assets)
	}
	samples := ParityCursorFrameSamples()
	if len(samples) != 5 {
		t.Fatalf("samples = %+v", samples)
	}
	wantFrames := []int64{20, 21, 50, 79, 80}
	wantMS := []int64{200, 210, 500, 790, 800}
	for index := range samples {
		if samples[index].FrameIndex != wantFrames[index] || samples[index].TimeMS != wantMS[index] {
			t.Fatalf("sample %d = %+v", index, samples[index])
		}
	}
}
