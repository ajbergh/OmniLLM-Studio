package video

const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v1"

// PlaybackParityCase is one live-preview observation window in the retained
// normal-playback canonicalization fixture. FrameIndex is used only to place
// the playhead before playback starts; evidence is collected while playback is
// continuously advancing from that point.
type PlaybackParityCase struct {
	Name                   string `json:"name"`
	FrameIndex             int64  `json:"frame_index"`
	ObserveMS              int64  `json:"observe_ms"`
	ExpectedMode           string `json:"expected_mode"`
	ExpectedReason         string `json:"expected_reason,omitempty"`
	ExpectedTransitionMode string `json:"expected_transition_mode"`
	RequireAdvancingFrames bool   `json:"require_advancing_frames,omitempty"`
}

// PlaybackCanonicalParityFixture exercises the first admitted normal-playback
// contract and every fail-closed class that must remain on legacy time. Each
// case occupies a separate timeline window so the browser harness can seek,
// start real playback, and retain observations without mutating authored state.
func PlaybackCanonicalParityFixture() (TimelineDocument, []ParityFixtureAsset, []PlaybackParityCase) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 18000
	doc.Canvas.Background = "#10131a"
	doc.Metadata = map[string]any{"fixture": PlaybackCanonicalParityFixtureName}

	transform := func() map[string]any {
		return map[string]any{
			"x":        0.0,
			"y":        0.0,
			"scale":    1.0,
			"rotation": 0.0,
			"opacity":  1.0,
		}
	}
	mediaClip := func(id, assetID string, startMS, durationMS int64) TimelineClip {
		return TimelineClip{
			ID:           id,
			AssetID:      assetID,
			StartMS:      startMS,
			DurationMS:   durationMS,
			TrimOutMS:    durationMS,
			PlaybackRate: 1,
			Transform:    transform(),
			Effects:      []TimelineEffect{},
			Keyframes:    []TimelineKeyframe{},
		}
	}
	track := func(id string, clip TimelineClip) TimelineTrack {
		return TimelineTrack{
			ID:      id,
			Type:    TrackTypeLayer,
			Name:    id,
			Visible: true,
			Clips:   []TimelineClip{clip},
		}
	}

	video := mediaClip("playback-video", "asset-landscape", 0, 1800)
	image := mediaClip("playback-image", "asset-square", 2000, 1800)

	text := TimelineClip{
		ID:         "playback-text",
		StartMS:    4000,
		DurationMS: 1800,
		TrimOutMS:  1800,
		Transform:  transform(),
		Text: &TimelineText{
			Text:       "Playback fallback text",
			FontFamily: "Inter",
			FontSize:   36,
			FontWeight: "700",
			Color:      "#ffffff",
			TextAlign:  "center",
		},
		Effects:   []TimelineEffect{},
		Keyframes: []TimelineKeyframe{},
	}

	cursor := mediaClip("playback-cursor", "asset-landscape", 6000, 1800)
	cursor.Cursor = &TimelineCursor{
		Visible:    true,
		Scale:      1,
		Highlight:  true,
		ClickRings: true,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 100, Y: 100},
			{TimeMS: 900, X: 320, Y: 180, Click: true},
			{TimeMS: 1799, X: 540, Y: 260},
		},
	}

	weightedOut := mediaClip("weighted-out", "asset-landscape", 8000, 2000)
	weightedOut.Transitions = []TimelineTransition{{
		ID:         "weighted-crossfade",
		Type:       TransitionTypeCrossfade,
		DurationMS: 800,
		Placement:  "between",
		PeerClipID: "weighted-in",
	}}
	weightedIn := mediaClip("weighted-in", "asset-square", 9000, 2000)

	mixedSlideOut := mediaClip("mixed-slide-out", "asset-landscape", 12000, 2000)
	mixedSlideOut.Transitions = []TimelineTransition{{
		ID:         "mixed-slide",
		Type:       TransitionTypeSlide,
		DurationMS: 800,
		Direction:  "left",
		Placement:  "between",
		PeerClipID: "mixed-slide-in",
	}}
	mixedSlideIn := mediaClip("mixed-slide-in", "asset-square", 12500, 2000)
	mixedCrossOut := mediaClip("mixed-cross-out", "asset-landscape", 12000, 2000)
	mixedCrossOut.Transitions = []TimelineTransition{{
		ID:         "mixed-crossfade",
		Type:       TransitionTypeCrossfade,
		DurationMS: 800,
		Placement:  "between",
		PeerClipID: "mixed-cross-in",
	}}
	mixedCrossIn := mediaClip("mixed-cross-in", "asset-square", 12500, 2000)

	deferredOut := mediaClip("deferred-out", "asset-landscape", 15000, 2000)
	deferredOut.Transitions = []TimelineTransition{{
		ID:         "deferred-slide",
		Type:       TransitionTypeSlide,
		DurationMS: 800,
		Direction:  "right",
		Placement:  "between",
		PeerClipID: "deferred-in",
	}}
	deferredBlocker := mediaClip("deferred-blocker", "asset-square", 15400, 1200)
	deferredIn := mediaClip("deferred-in", "asset-square", 15500, 2000)

	audio := TimelineTrack{
		ID:      "playback-audio",
		Type:    TrackTypeAudio,
		Name:    "Continuous playback audio clock",
		Visible: true,
		Clips: []TimelineClip{{
			ID:         "playback-audio-clip",
			AssetID:    "asset-audio",
			StartMS:    0,
			DurationMS: doc.DurationMS,
			TrimOutMS:  doc.DurationMS,
			Effects:    []TimelineEffect{},
			Keyframes:  []TimelineKeyframe{},
		}},
	}

	doc.Tracks = []TimelineTrack{
		track("track-video", video),
		track("track-image", image),
		track("track-text", text),
		track("track-cursor", cursor),
		track("track-weighted-out", weightedOut),
		track("track-weighted-in", weightedIn),
		track("track-mixed-slide-out", mixedSlideOut),
		track("track-mixed-slide-in", mixedSlideIn),
		track("track-mixed-cross-out", mixedCrossOut),
		track("track-mixed-cross-in", mixedCrossIn),
		track("track-deferred-out", deferredOut),
		track("track-deferred-blocker", deferredBlocker),
		track("track-deferred-in", deferredIn),
		audio,
	}

	assets := []ParityFixtureAsset{
		{ID: "asset-landscape", Kind: "video", Width: 640, Height: 360, DurationMS: 24000, Description: "deterministic landscape H.264 video"},
		{ID: "asset-square", Kind: "image", Width: 512, Height: 512, DurationMS: 18000, Description: "deterministic square PNG"},
		{ID: "asset-audio", Kind: "audio", DurationMS: 24000, Description: "deterministic continuous mono playback clock"},
	}
	// Once the whole visual frame fails closed, the consumer pair plan is
	// intentionally legacy. The admission decision's deferred reason remains the
	// authoritative proof of the canonical plan that caused fallback.
	cases := []PlaybackParityCase{
		{Name: "video-canonical-playback", FrameIndex: 6, ObserveMS: 650, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "image-canonical-playback", FrameIndex: 66, ObserveMS: 650, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "text-fallback", FrameIndex: 126, ObserveMS: 450, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-text", ExpectedTransitionMode: "legacy"},
		{Name: "cursor-fallback", FrameIndex: 186, ObserveMS: 450, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-cursor", ExpectedTransitionMode: "legacy"},
		{Name: "weighted-transition-fallback", FrameIndex: 276, ObserveMS: 450, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-plan-weighted-deferred", ExpectedTransitionMode: "legacy"},
		{Name: "mixed-transition-fallback", FrameIndex: 381, ObserveMS: 450, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-plan-mixed", ExpectedTransitionMode: "legacy"},
		// The canonical between-transition window is frames [465, 489). Start
		// comfortably inside it and retain only 300ms so ordinary playback startup
		// latency cannot carry the observation across frame 489 on a busy runner.
		{Name: "deferred-transition-fallback", FrameIndex: 466, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-deferred:deferred-slide:pair-inputs-not-adjacent", ExpectedTransitionMode: "legacy"},
	}
	return doc, assets, cases
}
