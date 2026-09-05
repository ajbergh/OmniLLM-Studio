package video

const (
	ParityRoundedRectangleFixtureName      = "parity-rounded-rectangle-v1"
	parityRoundedRectangleWidth            = 640
	parityRoundedRectangleHeight           = 360
	parityRoundedRectangleFPS              = 30
	parityRoundedRectangleDurationMS int64 = 1000
)

// ParityRoundedRectangleFixture isolates one static shape-state-v1 rounded
// rectangle on a flat black canvas. No media, text, cursor, effects,
// transitions, camera, crop, keyframes, animation, or audio are present, so
// retained browser/export differences measure only CSS versus Go raster edge
// coverage and alpha composition for the proven static-2D subset.
func ParityRoundedRectangleFixture() (TimelineDocument, []ParityFixtureAsset) {
	doc := NewEmptyTimeline(parityRoundedRectangleWidth, parityRoundedRectangleHeight, parityRoundedRectangleFPS)
	doc.DurationMS = parityRoundedRectangleDurationMS
	doc.Canvas.Background = "#000000"
	doc.Metadata = map[string]any{"fixture": ParityRoundedRectangleFixtureName}
	z := 10
	doc.Tracks = []TimelineTrack{{
		ID:      "track-rounded-rectangle",
		Type:    TrackTypeShape,
		Name:    "Canonical rounded rectangle control",
		Visible: true,
		Clips: []TimelineClip{{
			ID:         "clip-rounded-rectangle",
			StartMS:    0,
			DurationMS: parityRoundedRectangleDurationMS,
			TrimOutMS:  parityRoundedRectangleDurationMS,
			ZIndex:     &z,
			Transform: map[string]any{
				"x": 0.0, "y": 0.0, "z": 0.0,
				"scale": 1.0, "rotation": 0.0, "opacity": 1.0,
			},
			Shape: &TimelineShape{
				Kind:         ShapeKindRoundedRectangle,
				Width:        240,
				Height:       120,
				Fill:         "rgba(10,20,30,0.5)",
				Stroke:       "#f59e0b",
				StrokeWidth:  8,
				CornerRadius: 24,
			},
			Effects:         []TimelineEffect{},
			Transitions:     []TimelineTransition{},
			Keyframes:       []TimelineKeyframe{},
			AnimationBlocks: []TimelineAnimationBlock{},
		}},
	}}
	return doc, []ParityFixtureAsset{}
}

func ParityRoundedRectangleFrameSamples() []ParityFrameSample {
	return []ParityFrameSample{{
		Name:       "rounded-rectangle-static",
		FrameIndex: 15,
		TimeMS:     500,
		Reason:     "static midpoint isolates shape-state-v1 radius, fill, stroke, and edge antialiasing",
	}}
}
