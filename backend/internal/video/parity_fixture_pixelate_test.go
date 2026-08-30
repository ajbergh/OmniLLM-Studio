package video

import "testing"

func TestParityPixelateOpaqueFixtureIsValidAndIsolated(t *testing.T) {
	doc, assets := ParityPixelateOpaqueFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument() error = %v", err)
	}
	if validated.Canvas.Width != 512 || validated.Canvas.Height != 512 || validated.Canvas.FPS != 30 {
		t.Fatalf("canvas = %dx%d@%d, want 512x512@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)
	}
	if validated.DurationMS != 2000 {
		t.Fatalf("duration_ms = %d, want 2000", validated.DurationMS)
	}
	if len(assets) != 2 || assets[0].ID != "asset-square" || assets[0].Kind != "image" || assets[1].ID != "asset-audio" {
		t.Fatalf("assets = %#v, want square image plus audio harness asset", assets)
	}
	assertIsolatedPixelateFixture(t, validated)
}

func TestParityPixelateDecodedVideoFixtureIsValidAndUnscaled(t *testing.T) {
	doc, assets := ParityPixelateDecodedVideoFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument() error = %v", err)
	}
	if validated.Canvas.Width != 640 || validated.Canvas.Height != 360 || validated.Canvas.FPS != 30 {
		t.Fatalf("canvas = %dx%d@%d, want 640x360@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)
	}
	if validated.DurationMS != 2000 {
		t.Fatalf("duration_ms = %d, want 2000", validated.DurationMS)
	}
	if len(assets) != 2 || assets[0].ID != "asset-landscape" || assets[0].Kind != "video" || assets[0].Width != 640 || assets[0].Height != 360 || assets[1].ID != "asset-audio" {
		t.Fatalf("assets = %#v, want 640x360 video plus audio harness asset", assets)
	}
	assertIsolatedPixelateFixture(t, validated)
}

func TestParityPixelateAlphaFixtureIsValidAndUsesNonBlackBackground(t *testing.T) {
	doc, assets := ParityPixelateAlphaFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument() error = %v", err)
	}
	if validated.Canvas.Width != 512 || validated.Canvas.Height != 512 || validated.Canvas.FPS != 30 {
		t.Fatalf("canvas = %dx%d@%d, want 512x512@30", validated.Canvas.Width, validated.Canvas.Height, validated.Canvas.FPS)
	}
	if validated.Canvas.Background != "#19324A" {
		t.Fatalf("background = %q, want #19324A", validated.Canvas.Background)
	}
	if len(assets) != 2 || assets[0].ID != "asset-alpha" || assets[0].Kind != "image" || assets[0].Width != 512 || assets[0].Height != 512 || assets[1].ID != "asset-audio" {
		t.Fatalf("assets = %#v, want alpha PNG plus audio harness asset", assets)
	}
	assertIsolatedPixelateFixture(t, validated)
}

func assertIsolatedPixelateFixture(t *testing.T, validated TimelineDocument) {
	t.Helper()
	visualClips := 0
	pixelateClips := 0
	for _, track := range validated.Tracks {
		if !track.Visible || track.Type == TrackTypeAudio {
			continue
		}
		for _, clip := range track.Clips {
			visualClips++
			if clip.Shape != nil && clip.Shape.Kind == ShapeKindPixelate {
				pixelateClips++
				if clip.Shape.Width != 403 || clip.Shape.Height != 307 || clip.Shape.BlurRadius != 20 {
					t.Fatalf("pixelate shape = %#v, want 403x307 block size 20", clip.Shape)
				}
				if len(clip.Keyframes) != 0 {
					t.Fatalf("pixelate keyframes = %d, want renderer-static fixture", len(clip.Keyframes))
				}
			}
		}
	}
	if visualClips != 2 || pixelateClips != 1 {
		t.Fatalf("visual clips = %d pixelate clips = %d, want isolated source+pixelate", visualClips, pixelateClips)
	}
}

func TestParityPixelateOpaqueSamplesAndRegionsStayFrameBound(t *testing.T) {
	samples := ParityPixelateOpaqueFrameSamples()
	wantFrames := []int64{0, 15, 30, 59}
	assertPixelateFrames(t, samples, wantFrames)

	bounds := ParityPixelateOpaqueRegionBounds()
	wantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})
	if bounds != wantBounds {
		t.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)
	}
	assertPixelateRegionFrames(t, samples, bounds, ParityPixelateOpaqueRegionFrames(samples))
}

func TestParityPixelateDecodedVideoSamplesAndRegionsStayFrameBound(t *testing.T) {
	samples := ParityPixelateDecodedVideoFrameSamples()
	wantFrames := []int64{0, 15, 30, 59}
	assertPixelateFrames(t, samples, wantFrames)

	bounds := ParityPixelateDecodedVideoRegionBounds()
	wantBounds := (ParityBounds{MinX: 135, MinY: 18, MaxX: 538, MaxY: 325})
	if bounds != wantBounds {
		t.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)
	}
	assertPixelateRegionFrames(t, samples, bounds, ParityPixelateDecodedVideoRegionFrames(samples))
}

func TestParityPixelateAlphaSamplesAndRegionsStayFrameBound(t *testing.T) {
	samples := ParityPixelateAlphaFrameSamples()
	wantFrames := []int64{0, 15, 30, 59}
	assertPixelateFrames(t, samples, wantFrames)

	bounds := ParityPixelateAlphaRegionBounds()
	wantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})
	if bounds != wantBounds {
		t.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)
	}
	assertPixelateRegionFrames(t, samples, bounds, ParityPixelateAlphaRegionFrames(samples))
}

func assertPixelateFrames(t *testing.T, samples []ParityFrameSample, wantFrames []int64) {
	t.Helper()
	if len(samples) != len(wantFrames) {
		t.Fatalf("samples = %d, want %d", len(samples), len(wantFrames))
	}
	for i, want := range wantFrames {
		if samples[i].FrameIndex != want {
			t.Fatalf("samples[%d].frame_index = %d, want %d", i, samples[i].FrameIndex, want)
		}
	}
}

func assertPixelateRegionFrames(t *testing.T, samples []ParityFrameSample, bounds ParityBounds, frames []ParityFixtureRegionFrame) {
	t.Helper()
	if len(frames) != len(samples) {
		t.Fatalf("region frames = %d, want %d", len(frames), len(samples))
	}
	for i, frame := range frames {
		if frame.FrameIndex != samples[i].FrameIndex {
			t.Fatalf("region frame %d = %d, want %d", i, frame.FrameIndex, samples[i].FrameIndex)
		}
		if len(frame.Regions) != 1 || frame.Regions[0].Name != "pixelate-output" || frame.Regions[0].Bounds != bounds {
			t.Fatalf("region frame %d = %#v", i, frame)
		}
	}
}
