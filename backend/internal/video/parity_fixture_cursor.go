package video

const (
	ParityCursorFixtureName           = "parity-cursor-v1"
	ParityCursorBackdropAssetID       = "asset-cursor-backdrop"
	parityCursorWidth                 = 640
	parityCursorHeight                = 360
	parityCursorFPS                   = 100
	parityCursorDurationMS      int64 = 1000
)

// ParityCursorFixture isolates cursor-state-v1 visual semantics on a lossless,
// flat backdrop. The preview and export paths both pin the current cursor
// highlight/ring palette to explicit sRGB values, so this fixture measures that
// renderer contract rather than whichever Tailwind palette happens to be active.
// It deliberately excludes audio, effects, transitions, camera, clip animation,
// crop, opacity, and non-identity parent transforms so retained differences
// measure pointer/highlight/click-ring rasterization plus exact output-frame
// cursor sampling rather than unrelated media or geometry noise.
func ParityCursorFixture() (TimelineDocument, []ParityFixtureAsset) {
	doc := NewEmptyTimeline(parityCursorWidth, parityCursorHeight, parityCursorFPS)
	doc.DurationMS = parityCursorDurationMS
	doc.Canvas.Background = "#000000"
	doc.Metadata = map[string]any{"fixture": ParityCursorFixtureName}

	z := 10
	doc.Tracks = []TimelineTrack{{
		ID:      "track-cursor",
		Type:    TrackTypeLayer,
		Name:    "Canonical cursor control",
		Visible: true,
		Clips: []TimelineClip{{
			ID:         "clip-cursor",
			AssetID:    ParityCursorBackdropAssetID,
			StartMS:    0,
			DurationMS: parityCursorDurationMS,
			TrimOutMS:  parityCursorDurationMS,
			ZIndex:     &z,
			Transform: map[string]any{
				"x": 0.0, "y": 0.0, "z": 0.0,
				"scale": 1.0, "rotation": 0.0, "opacity": 1.0,
			},
			Cursor: &TimelineCursor{
				Visible:    true,
				Scale:      1,
				Highlight:  true,
				ClickRings: true,
				Events: []TimelineCursorEvent{
					{TimeMS: 0, X: 160, Y: 120},
					{TimeMS: 500, X: 320, Y: 180, Click: true},
					{TimeMS: 999, X: 480, Y: 240},
				},
			},
			Effects:         []TimelineEffect{},
			Transitions:     []TimelineTransition{},
			Keyframes:       []TimelineKeyframe{},
			AnimationBlocks: []TimelineAnimationBlock{},
		}},
	}}
	return doc, []ParityFixtureAsset{{
		ID:          ParityCursorBackdropAssetID,
		Kind:        "image",
		Description: "lossless 640x360 black PNG cursor backdrop",
	}}
}

// ParityCursorFrameSamples surrounds the strict <300 ms click window at 100
// fps. Frame 20 is exactly 300 ms before the 500 ms click and frame 80 exactly
// 300 ms after it, so both must omit the ring; frames 21/50/79 must include it.
func ParityCursorFrameSamples() []ParityFrameSample {
	return []ParityFrameSample{
		{Name: "cursor-click-boundary-before", FrameIndex: 20, TimeMS: 200, Reason: "exactly 300ms before click; strict boundary must omit ring"},
		{Name: "cursor-click-inside-before", FrameIndex: 21, TimeMS: 210, Reason: "290ms before click; ring must be present"},
		{Name: "cursor-click-center", FrameIndex: 50, TimeMS: 500, Reason: "click presentation frame and interpolated cursor center"},
		{Name: "cursor-click-inside-after", FrameIndex: 79, TimeMS: 790, Reason: "290ms after click; ring must be present"},
		{Name: "cursor-click-boundary-after", FrameIndex: 80, TimeMS: 800, Reason: "exactly 300ms after click; strict boundary must omit ring"},
	}
}
