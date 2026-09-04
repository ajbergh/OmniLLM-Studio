package video

const (
	ParityCursorSmoothingFixtureName           = "parity-cursor-smoothing-v2"
	ParityCursorSmoothingBackdropAssetID       = "asset-cursor-smoothing-backdrop"
	parityCursorSmoothingWidth                 = 640
	parityCursorSmoothingHeight                = 360
	parityCursorSmoothingFPS                   = 100
	parityCursorSmoothingDurationMS      int64 = 1001
)

// ParityCursorSmoothingFixture isolates cursor-state-v2 smoothstep semantics on
// a lossless flat backdrop. The two authored endpoints intentionally span 1000
// ms so frames 25/50/75 land at exact 25/50/75 percent segment progress. The
// asymmetric samples distinguish cubic smoothstep from cursor-state-v1 linear
// interpolation while keeping cursor rasterization, parent transform, effects,
// transitions, camera, and click semantics out of the measurement.
func ParityCursorSmoothingFixture() (TimelineDocument, []ParityFixtureAsset) {
	doc := NewEmptyTimeline(parityCursorSmoothingWidth, parityCursorSmoothingHeight, parityCursorSmoothingFPS)
	doc.DurationMS = parityCursorSmoothingDurationMS
	doc.Canvas.Background = "#000000"
	doc.Metadata = map[string]any{"fixture": ParityCursorSmoothingFixtureName}

	z := 10
	doc.Tracks = []TimelineTrack{{
		ID:      "track-cursor-smoothing",
		Type:    TrackTypeLayer,
		Name:    "Canonical smoothed cursor control",
		Visible: true,
		Clips: []TimelineClip{{
			ID:         "clip-cursor-smoothing",
			AssetID:    ParityCursorSmoothingBackdropAssetID,
			StartMS:    0,
			DurationMS: parityCursorSmoothingDurationMS,
			TrimOutMS:  parityCursorSmoothingDurationMS,
			ZIndex:     &z,
			Transform: map[string]any{
				"x": 0.0, "y": 0.0, "z": 0.0,
				"scale": 1.0, "rotation": 0.0, "opacity": 1.0,
			},
			Cursor: &TimelineCursor{
				Visible:    true,
				Scale:      1,
				Highlight:  true,
				ClickRings: false,
				Smoothing:  true,
				Events: []TimelineCursorEvent{
					{TimeMS: 0, X: 160, Y: 100},
					{TimeMS: 1000, X: 480, Y: 260},
				},
			},
			Effects:         []TimelineEffect{},
			Transitions:     []TimelineTransition{},
			Keyframes:       []TimelineKeyframe{},
			AnimationBlocks: []TimelineAnimationBlock{},
		}},
	}}
	return doc, []ParityFixtureAsset{{
		ID:          ParityCursorSmoothingBackdropAssetID,
		Kind:        "image",
		Description: "lossless 640x360 black PNG cursor smoothing backdrop",
	}}
}

// ParityCursorSmoothingFrameSamples exercise both asymmetric smoothstep points
// plus the midpoint control. Expected cursor centers are (210,125), (320,180),
// and (430,235); linear interpolation would instead place the asymmetric
// samples at (240,140) and (400,220).
func ParityCursorSmoothingFrameSamples() []ParityFrameSample {
	return []ParityFrameSample{
		{Name: "cursor-smoothing-quarter", FrameIndex: 25, TimeMS: 250, Reason: "25% progress; smoothstep=0.15625, distinct from linear"},
		{Name: "cursor-smoothing-midpoint", FrameIndex: 50, TimeMS: 500, Reason: "50% progress; smoothstep midpoint control"},
		{Name: "cursor-smoothing-three-quarter", FrameIndex: 75, TimeMS: 750, Reason: "75% progress; smoothstep=0.84375, distinct from linear"},
	}
}
