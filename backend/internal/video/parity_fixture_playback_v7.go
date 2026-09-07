package video

const PlaybackCanonicalParityFixtureV7Name = "parity-playback-canonical-v7"

// PlaybackCanonicalParityFixtureV7 extends the immutable v6 playback fixture
// with the first canonical annotation playback slice. It deliberately retains
// every v6 case and adds only:
//   - one export-proven static-2D rounded_rectangle mixed with canonical media;
//   - one unsupported ellipse in an otherwise empty visual window to prove the
//     complete frame returns to the established time-domain painter.
func PlaybackCanonicalParityFixtureV7() (TimelineDocument, []ParityFixtureAsset, []PlaybackParityCase) {
	doc, assets, cases := PlaybackCanonicalParityFixture()
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	doc.Metadata["fixture"] = PlaybackCanonicalParityFixtureV7Name

	rounded := TimelineClip{
		ID:         "playback-shape-rounded-rectangle",
		StartMS:    0,
		DurationMS: 1200,
		TrimOutMS:  1200,
		Transform: map[string]any{
			"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
		},
		Shape: &TimelineShape{
			Kind: ShapeKindRoundedRectangle, Width: 240, Height: 120,
			Fill: "rgba(10,20,30,0.5)", Stroke: "#f59e0b", StrokeWidth: 8, CornerRadius: 24,
		},
		Effects:   []TimelineEffect{},
		Keyframes: []TimelineKeyframe{},
	}
	unsupportedEllipse := TimelineClip{
		ID:         "playback-shape-ellipse",
		StartMS:    26800,
		DurationMS: 600,
		TrimOutMS:  600,
		Transform: map[string]any{
			"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
		},
		Shape: &TimelineShape{
			Kind: ShapeKindEllipse, Width: 220, Height: 110,
			Fill: "#22d3ee55", Stroke: "#f8fafc", StrokeWidth: 6,
		},
		Effects:   []TimelineEffect{},
		Keyframes: []TimelineKeyframe{},
	}
	doc.Tracks = append(doc.Tracks,
		TimelineTrack{
			ID: "track-playback-shape-rounded-rectangle", Type: TrackTypeLayer,
			Name: "Canonical rounded rectangle playback", Visible: true,
			Clips: []TimelineClip{rounded},
		},
		TimelineTrack{
			ID: "track-playback-shape-ellipse", Type: TrackTypeLayer,
			Name: "Unsupported shape playback fallback", Visible: true,
			Clips: []TimelineClip{unsupportedEllipse},
		},
	)

	cases = append(cases,
		PlaybackParityCase{
			Name: "rounded-rectangle-canonical-playback", FrameIndex: 18, ObserveMS: 350,
			ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none",
			ExpectedShapeConsumer: "canonical-inline", ExpectedShapeClipID: rounded.ID,
			RequireShapeSurface: true, RequireShapeGeometry: true, RequireAdvancingFrames: true,
		},
		PlaybackParityCase{
			Name: "unsupported-ellipse-fallback", FrameIndex: 807, ObserveMS: 300,
			ExpectedMode: "legacy-time-fallback",
			ExpectedReason: "shape-playback-deferred:playback-shape-ellipse:shape-kind-unsupported",
			ExpectedTransitionMode: "legacy",
			ExpectedShapeConsumer: "legacy-time-fallback", ExpectedShapeClipID: unsupportedEllipse.ID,
			RequireShapeSurface: true,
		},
	)
	return doc, assets, cases
}
