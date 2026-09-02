package video

const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v2"

// PlaybackParityCase is one live-preview observation window in the retained
// normal-playback canonicalization fixture. FrameIndex is used only to place
// the playhead before playback starts; evidence is collected while playback is
// continuously advancing from that point.
type PlaybackParityCase struct {
	Name                     string `json:"name"`
	FrameIndex               int64  `json:"frame_index"`
	ObserveMS                int64  `json:"observe_ms"`
	ExpectedMode             string `json:"expected_mode"`
	ExpectedReason           string `json:"expected_reason,omitempty"`
	ExpectedTransitionMode   string `json:"expected_transition_mode"`
	ExpectedWeightedRuntime  string `json:"expected_weighted_runtime,omitempty"`
	ExpectedWeightedConsumer string `json:"expected_weighted_consumer,omitempty"`
	RequireWeightedCanvas    bool   `json:"require_weighted_canvas,omitempty"`
	RequireAdvancingFrames   bool   `json:"require_advancing_frames,omitempty"`
	DecoderBudget            int    `json:"decoder_budget,omitempty"`
}

// PlaybackCanonicalParityFixture exercises admitted normal-playback contracts
// and fail-closed classes against one immutable authored timeline. Weighted
// cases prove the Canvas pair consumer during ordinary free-running playback;
// the decoder-budget case deliberately withholds one required raster source so
// readiness revocation can be retained independently from mixed/structural
// transition fallback.
func PlaybackCanonicalParityFixture() (TimelineDocument, []ParityFixtureAsset, []PlaybackParityCase) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 22000
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
	pair := func(idPrefix, transitionType string, startMS int64, incomingAsset string) (TimelineClip, TimelineClip) {
		outgoing := mediaClip(idPrefix+"-out", "asset-landscape", startMS, 2200)
		incoming := mediaClip(idPrefix+"-in", incomingAsset, startMS+800, 2200)
		outgoing.Transitions = []TimelineTransition{{
			ID:         idPrefix,
			Type:       transitionType,
			DurationMS: 1200,
			Placement:  "between",
			PeerClipID: incoming.ID,
		}}
		return outgoing, incoming
	}

	video := mediaClip("playback-video", "asset-landscape", 0, 1200)
	image := mediaClip("playback-image", "asset-square", 1400, 1200)

	text := TimelineClip{
		ID:         "playback-text",
		StartMS:    2800,
		DurationMS: 1200,
		TrimOutMS:  1200,
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

	cursor := mediaClip("playback-cursor", "asset-landscape", 4200, 1200)
	cursor.Cursor = &TimelineCursor{
		Visible:    true,
		Scale:      1,
		Highlight:  true,
		ClickRings: true,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 100, Y: 100},
			{TimeMS: 600, X: 320, Y: 180, Click: true},
			{TimeMS: 1199, X: 540, Y: 260},
		},
	}

	crossfadeOut, crossfadeIn := pair("weighted-crossfade", TransitionTypeCrossfade, 5600, "asset-square")
	zoomOut, zoomIn := pair("weighted-zoom", TransitionTypeZoom, 8200, "asset-square")
	dipOut, dipIn := pair("weighted-dip", TransitionTypeDipToBlack, 10800, "asset-square")
	budgetOut, budgetIn := pair("weighted-budget-crossfade", TransitionTypeCrossfade, 13400, "asset-landscape")

	mixedSlideOut, mixedSlideIn := pair("mixed-slide", TransitionTypeSlide, 16000, "asset-square")
	mixedSlideOut.Transitions[0].Direction = "left"
	mixedCrossOut, mixedCrossIn := pair("mixed-crossfade", TransitionTypeCrossfade, 16000, "asset-square")

	deferredOut, deferredIn := pair("deferred-slide", TransitionTypeSlide, 18600, "asset-square")
	deferredOut.Transitions[0].Direction = "right"
	deferredBlocker := mediaClip("deferred-blocker", "asset-square", 19100, 1000)

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
		track("track-weighted-crossfade-out", crossfadeOut),
		track("track-weighted-crossfade-in", crossfadeIn),
		track("track-weighted-zoom-out", zoomOut),
		track("track-weighted-zoom-in", zoomIn),
		track("track-weighted-dip-out", dipOut),
		track("track-weighted-dip-in", dipIn),
		track("track-weighted-budget-out", budgetOut),
		track("track-weighted-budget-in", budgetIn),
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
		{ID: "asset-square", Kind: "image", Width: 512, Height: 512, DurationMS: 22000, Description: "deterministic square PNG"},
		{ID: "asset-audio", Kind: "audio", DurationMS: 24000, Description: "deterministic continuous mono playback clock"},
	}
	// Once the whole visual frame fails closed, the established preview pair plan
	// is intentionally legacy. Weighted runtime markers are retained separately so
	// readiness failures remain distinguishable from mixed/structural deferrals.
	cases := []PlaybackParityCase{
		{Name: "video-canonical-playback", FrameIndex: 6, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "image-canonical-playback", FrameIndex: 48, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "text-fallback", FrameIndex: 90, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-text", ExpectedTransitionMode: "legacy"},
		{Name: "cursor-fallback", FrameIndex: 132, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-cursor", ExpectedTransitionMode: "legacy"},
		{Name: "weighted-crossfade-canonical", FrameIndex: 198, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-zoom-canonical", FrameIndex: 276, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-dip-canonical", FrameIndex: 354, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-decoder-budget-fallback", FrameIndex: 432, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-weighted-runtime-deferred:weighted-budget-crossfade-out:decoder-budget-poster", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "deferred", ExpectedWeightedConsumer: "legacy-time-fallback", DecoderBudget: 1},
		{Name: "mixed-transition-fallback", FrameIndex: 510, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-plan-mixed", ExpectedTransitionMode: "legacy"},
		{Name: "deferred-transition-fallback", FrameIndex: 588, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-deferred:deferred-slide:pair-inputs-not-adjacent", ExpectedTransitionMode: "legacy"},
	}
	return doc, assets, cases
}
