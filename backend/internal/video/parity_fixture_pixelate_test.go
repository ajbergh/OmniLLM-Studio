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
	if len(assets) != 2 || assets[0].ID != "asset-square" || assets[1].ID != "asset-audio" {
		t.Fatalf("assets = %#v, want square image plus audio harness asset", assets)
	}

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
	if len(samples) != len(wantFrames) {
		t.Fatalf("samples = %d, want %d", len(samples), len(wantFrames))
	}
	for i, want := range wantFrames {
		if samples[i].FrameIndex != want {
			t.Fatalf("samples[%d].frame_index = %d, want %d", i, samples[i].FrameIndex, want)
		}
	}

	bounds := ParityPixelateOpaqueRegionBounds()
	wantBounds := (ParityBounds{MinX: 71, MinY: 94, MaxX: 474, MaxY: 401})
	if bounds != wantBounds {
		t.Fatalf("bounds = %#v, want %#v", bounds, wantBounds)
	}
	frames := ParityPixelateOpaqueRegionFrames(samples)
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
