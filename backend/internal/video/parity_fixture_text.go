package video

const (
	ParityResourceTextFixtureName = "parity-resource-text-v1"
	ParityResourceTextFontID      = "parity-text-face-v1"
	parityResourceTextWidth       = 640
	parityResourceTextHeight      = 360
	parityResourceTextFPS         = 30
	parityResourceTextDurationMS  = int64(2000)
)

// ParityResourceTextFixture isolates the smallest useful resource-backed text
// contract. It deliberately excludes decoration, animation, explicit line-height,
// fallback families, and media underneath the glyphs so retained differences
// measure browser/FFmpeg glyph rasterization against one immutable font face.
func ParityResourceTextFixture() (TimelineDocument, []ParityFixtureAsset) {
	doc := NewEmptyTimeline(parityResourceTextWidth, parityResourceTextHeight, parityResourceTextFPS)
	doc.DurationMS = parityResourceTextDurationMS
	doc.Canvas.Background = "#182230"
	doc.Metadata = map[string]any{"fixture": ParityResourceTextFixtureName}

	textZ := 10
	doc.Tracks = []TimelineTrack{
		{
			ID:      "track-resource-text",
			Type:    TrackTypeText,
			Name:    "Immutable project font text",
			Visible: true,
			Clips: []TimelineClip{{
				ID:         "clip-resource-text",
				StartMS:    0,
				DurationMS: parityResourceTextDurationMS,
				TrimOutMS:  parityResourceTextDurationMS,
				ZIndex:     &textZ,
				Transform: map[string]any{
					"x": 0.0, "y": 0.0, "z": 0.0,
					"scale": 1.0, "rotation": 0.0, "opacity": 1.0,
				},
				Text: &TimelineText{
					Text:           "WYSIWYG 42",
					FontFamily:     "DejaVu Sans",
					FontResourceID: ParityResourceTextFontID,
					FontSize:       48,
					FontWeight:     "400",
					Color:          "#f7f8fa",
				},
				Effects:   []TimelineEffect{},
				Keyframes: []TimelineKeyframe{},
			}},
		},
		{
			ID:      "track-resource-text-audio",
			Type:    TrackTypeAudio,
			Name:    "Capture harness audio",
			Visible: true,
			Clips: []TimelineClip{{
				ID:           "clip-resource-text-audio",
				AssetID:      "asset-audio",
				StartMS:      0,
				DurationMS:   parityResourceTextDurationMS,
				TrimOutMS:    parityResourceTextDurationMS,
				PlaybackRate: 1,
				Effects:      []TimelineEffect{},
				Keyframes:    []TimelineKeyframe{},
			}},
		},
	}

	return doc, []ParityFixtureAsset{
		{ID: "asset-font", Kind: "font", Description: "immutable regular DejaVu Sans face bound to parity-text-face-v1"},
		{ID: "asset-audio", Kind: "audio", DurationMS: parityResourceTextDurationMS, Description: "deterministic capture harness tone"},
	}
}

// ParityResourceTextFrameSamples binds the measurement to one stable interior
// output frame. The fixture is static, so a single frame removes temporal noise
// while still proving the canonical frame-addressed preview/export path.
func ParityResourceTextFrameSamples() []ParityFrameSample {
	return []ParityFrameSample{{
		Name:       "resource-text-static",
		FrameIndex: 15,
		TimeMS:     500,
		Reason:     "single immutable project-font glyph raster sample",
	}}
}
