package video

const (
	ParityPixelateOpaqueFixtureName = "parity-pixelate-opaque-v1"

	parityPixelateOpaqueCanvasWidth  = 512
	parityPixelateOpaqueCanvasHeight = 512
	parityPixelateOpaqueFPS          = 30
	parityPixelateOpaqueDurationMS   = int64(2000)
	parityPixelateOpaqueRegionWidth  = 403
	parityPixelateOpaqueRegionHeight = 307
	parityPixelateOpaqueRegionX      = 17
	parityPixelateOpaqueRegionY      = -8
	parityPixelateOpaqueBlockSize    = 20
)

// ParityFixtureRegionFrame binds exact structural-region policy to canonical
// output frame identity. It is shared by focused fixture emitters and the
// parity-report manifest without relying on mutable screenshot filenames.
type ParityFixtureRegionFrame struct {
	FrameIndex int64          `json:"frame_index"`
	Regions    []ParityRegion `json:"regions"`
}

// ParityPixelateOpaqueFixture isolates the first preview-pixelate-raster-v1
// consumer path from the larger torture fixture. A 1:1 opaque PNG backdrop
// avoids codec/color-conversion noise while the non-divisible 403x307 region
// exercises the same floor downsample grid used by the legacy FFmpeg renderer.
func ParityPixelateOpaqueFixture() (TimelineDocument, []ParityFixtureAsset) {
	doc := NewEmptyTimeline(
		parityPixelateOpaqueCanvasWidth,
		parityPixelateOpaqueCanvasHeight,
		parityPixelateOpaqueFPS,
	)
	doc.DurationMS = parityPixelateOpaqueDurationMS
	doc.Canvas.Background = "#000000"
	doc.Metadata = map[string]any{"fixture": ParityPixelateOpaqueFixtureName}

	mediaZ := 1
	pixelateZ := 20
	doc.Tracks = []TimelineTrack{
		{
			ID:      "track-pixelate-source",
			Type:    TrackTypeLayer,
			Name:    "Opaque pixelate source",
			Visible: true,
			Clips: []TimelineClip{{
				ID:           "clip-pixelate-source",
				AssetID:      "asset-square",
				StartMS:      0,
				DurationMS:   parityPixelateOpaqueDurationMS,
				TrimOutMS:    parityPixelateOpaqueDurationMS,
				PlaybackRate: 1,
				ZIndex:       &mediaZ,
				Transform: map[string]any{
					"x": 0.0, "y": 0.0, "z": 0.0,
					"scale": 1.0, "rotation": 0.0, "opacity": 1.0,
				},
				Effects:   []TimelineEffect{},
				Keyframes: []TimelineKeyframe{},
			}},
		},
		{
			ID:      "track-pixelate-region",
			Type:    TrackTypeShape,
			Name:    "Exact opaque pixelate region",
			Visible: true,
			Clips: []TimelineClip{{
				ID:         "clip-pixelate-region",
				StartMS:    0,
				DurationMS: parityPixelateOpaqueDurationMS,
				TrimOutMS:  parityPixelateOpaqueDurationMS,
				ZIndex:     &pixelateZ,
				Transform: map[string]any{
					"x": float64(parityPixelateOpaqueRegionX),
					"y": float64(parityPixelateOpaqueRegionY),
					"z": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
				},
				Shape: &TimelineShape{
					Kind:       ShapeKindPixelate,
					Width:      parityPixelateOpaqueRegionWidth,
					Height:     parityPixelateOpaqueRegionHeight,
					BlurRadius: parityPixelateOpaqueBlockSize,
				},
				Effects:   []TimelineEffect{},
				Keyframes: []TimelineKeyframe{},
			}},
		},
		{
			ID:      "track-pixelate-audio",
			Type:    TrackTypeAudio,
			Name:    "Capture harness audio",
			Visible: true,
			Clips: []TimelineClip{{
				ID:           "clip-pixelate-audio",
				AssetID:      "asset-audio",
				StartMS:      0,
				DurationMS:   parityPixelateOpaqueDurationMS,
				TrimOutMS:    parityPixelateOpaqueDurationMS,
				PlaybackRate: 1,
				Effects:      []TimelineEffect{},
				Keyframes:    []TimelineKeyframe{},
			}},
		},
	}

	assets := []ParityFixtureAsset{
		{ID: "asset-square", Kind: "image", Width: 512, Height: 512, DurationMS: parityPixelateOpaqueDurationMS, Description: "opaque 512x512 deterministic PNG"},
		{ID: "asset-audio", Kind: "audio", DurationMS: parityPixelateOpaqueDurationMS, Description: "deterministic mono swept tone and impulses"},
	}
	return doc, assets
}

// ParityPixelateOpaqueFrameSamples keeps the focused evidence fixture small and
// deterministic while sampling the start, two interior frames, and final frame.
func ParityPixelateOpaqueFrameSamples() []ParityFrameSample {
	return []ParityFrameSample{
		{Name: "pixelate-start", FrameIndex: 0, TimeMS: 0, Reason: "pixelate region start"},
		{Name: "pixelate-quarter", FrameIndex: 15, TimeMS: 500, Reason: "opaque pixelate interior"},
		{Name: "pixelate-half", FrameIndex: 30, TimeMS: 1000, Reason: "opaque pixelate interior"},
		{Name: "pixelate-final", FrameIndex: 59, TimeMS: 1967, Reason: "final active pixelate frame"},
	}
}

// ParityPixelateOpaqueRegionBounds mirrors the legacy renderer's integer
// center-relative region placement for this fixed fixture.
func ParityPixelateOpaqueRegionBounds() ParityBounds {
	minX := (parityPixelateOpaqueCanvasWidth-parityPixelateOpaqueRegionWidth)/2 + parityPixelateOpaqueRegionX
	minY := (parityPixelateOpaqueCanvasHeight-parityPixelateOpaqueRegionHeight)/2 + parityPixelateOpaqueRegionY
	return ParityBounds{
		MinX: minX,
		MinY: minY,
		MaxX: minX + parityPixelateOpaqueRegionWidth,
		MaxY: minY + parityPixelateOpaqueRegionHeight,
	}
}

// ParityPixelateOpaqueRegionFrames requires an exact structural comparison of
// every sampled pixelate output rectangle while leaving whole-frame codec/style
// thresholds independently diagnostic.
func ParityPixelateOpaqueRegionFrames(samples []ParityFrameSample) []ParityFixtureRegionFrame {
	bounds := ParityPixelateOpaqueRegionBounds()
	frames := make([]ParityFixtureRegionFrame, 0, len(samples))
	for _, sample := range samples {
		frames = append(frames, ParityFixtureRegionFrame{
			FrameIndex: sample.FrameIndex,
			Regions: []ParityRegion{{
				Name:   "pixelate-output",
				Bounds: bounds,
			}},
		})
	}
	return frames
}
