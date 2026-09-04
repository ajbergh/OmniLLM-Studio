package video

const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v5"

// PlaybackParityCase is one live-preview observation window in the retained
// normal-playback canonicalization fixture. FrameIndex is used only to place
// the playhead before playback starts; evidence is collected while playback is
// continuously advancing from that point.
type PlaybackParityCase struct {
	Name                     string   `json:"name"`
	FrameIndex               int64    `json:"frame_index"`
	ObserveMS                int64    `json:"observe_ms"`
	ExpectedMode             string   `json:"expected_mode"`
	ExpectedReason           string   `json:"expected_reason,omitempty"`
	ExpectedTransitionMode   string   `json:"expected_transition_mode"`
	ExpectedWeightedRuntime  string   `json:"expected_weighted_runtime,omitempty"`
	ExpectedWeightedConsumer string   `json:"expected_weighted_consumer,omitempty"`
	ExpectedWeightedPairID   string   `json:"expected_weighted_pair_id,omitempty"`
	ExpectedTextRuntime      string   `json:"expected_text_runtime,omitempty"`
	ExpectedTextConsumer     string   `json:"expected_text_consumer,omitempty"`
	ExpectedTextClipID       string   `json:"expected_text_clip_id,omitempty"`
	ExpectedTextTrace        []string `json:"expected_text_trace,omitempty"`
	ExpectedCursorConsumer   string   `json:"expected_cursor_consumer,omitempty"`
	ExpectedCursorClipID     string   `json:"expected_cursor_clip_id,omitempty"`
	RequireCursorSurface     bool     `json:"require_cursor_surface,omitempty"`
	RequireCursorMotion      bool     `json:"require_cursor_motion,omitempty"`
	RequireCursorHighlight   bool     `json:"require_cursor_highlight,omitempty"`
	RequireCursorClickToggle bool     `json:"require_cursor_click_toggle,omitempty"`
	RequireWeightedCanvas    bool     `json:"require_weighted_canvas,omitempty"`
	RequireTextLayout        bool     `json:"require_text_layout,omitempty"`
	RequireAdvancingFrames   bool     `json:"require_advancing_frames,omitempty"`
	DecoderBudget            int      `json:"decoder_budget,omitempty"`
}

// PlaybackCanonicalParityFixture exercises admitted normal-playback contracts
// and fail-closed classes against one immutable authored timeline. Text cases
// prove resource-backed FontFace/layout readiness, family-name-only deferral,
// font-load failure, and mixed-frame all-or-nothing authority. Weighted cases
// continue to prove the Canvas pair consumer and its independent readiness.
// Mixed v5 cases explicitly prove that supported media/text/cursor and
// weighted/text/cursor surfaces share one canonical frame decision. Exact cursor
// samples must stay frame-addressed while any unsupported cursor parent or runtime
// debt still revokes authority for the complete visual frame.
func PlaybackCanonicalParityFixture() (TimelineDocument, []ParityFixtureAsset, []PlaybackParityCase) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 38000
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
	textClip := func(id, label, family, resourceID string, startMS int64) TimelineClip {
		return TimelineClip{
			ID:         id,
			StartMS:    startMS,
			DurationMS: 1200,
			TrimOutMS:  1200,
			Transform:  transform(),
			Text: &TimelineText{
				Text:           label,
				FontFamily:     family,
				FontResourceID: resourceID,
				FontSize:       36,
				FontWeight:     "700",
				Color:          "#ffffff",
				Background:     "#111827cc",
				TextAlign:      "center",
				LineHeight:     1.2,
				LetterSpacing:  0.5,
				BorderRadius:   8,
			},
			Effects:   []TimelineEffect{},
			Keyframes: []TimelineKeyframe{},
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
	resourceText := textClip(
		"playback-text-resource",
		"Resource-backed playback text",
		"DejaVu Sans",
		"playback-font-v1",
		2800,
	)
	familyText := textClip(
		"playback-text-family",
		"Family-name-only playback fallback",
		"Inter",
		"",
		4200,
	)
	invalidFontText := textClip(
		"playback-text-invalid-font",
		"Invalid resource font fallback",
		"Invalid Playback Font",
		"playback-font-invalid-v1",
		5600,
	)

	cursor := mediaClip("playback-cursor", "asset-landscape", 7000, 1200)
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

	mixedText := textClip(
		"playback-mixed-text",
		"Ready text must not partially promote",
		"DejaVu Sans",
		"playback-font-v1",
		8400,
	)
	mixedCursor := mediaClip("playback-mixed-cursor", "asset-landscape", 8400, 1200)
	mixedCursor.Cursor = &TimelineCursor{
		Visible: true,
		Scale:   1,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 160, Y: 120},
			{TimeMS: 1199, X: 480, Y: 240},
		},
	}

	crossfadeOut, crossfadeIn := pair("weighted-crossfade", TransitionTypeCrossfade, 9800, "asset-square")
	zoomOut, zoomIn := pair("weighted-zoom", TransitionTypeZoom, 12400, "asset-square")
	dipOut, dipIn := pair("weighted-dip", TransitionTypeDipToBlack, 15000, "asset-square")
	budgetOut, budgetIn := pair("weighted-budget-crossfade", TransitionTypeCrossfade, 17600, "asset-landscape")

	mixedSlideOut, mixedSlideIn := pair("mixed-slide", TransitionTypeSlide, 20200, "asset-square")
	mixedSlideOut.Transitions[0].Direction = "left"
	mixedCrossOut, mixedCrossIn := pair("mixed-crossfade", TransitionTypeCrossfade, 20200, "asset-square")

	deferredOut, deferredIn := pair("deferred-slide", TransitionTypeSlide, 22800, "asset-square")
	deferredOut.Transitions[0].Direction = "right"
	deferredBlocker := mediaClip("deferred-blocker", "asset-square", 23300, 1000)

	supportedMediaTextMedia := mediaClip("playback-supported-media-text-media", "asset-landscape", 25600, 1200)
	supportedMediaText := textClip(
		"playback-supported-media-text",
		"Canonical media plus resource text",
		"DejaVu Sans",
		"playback-font-v1",
		25600,
	)

	weightedTextOut, weightedTextIn := pair("weighted-text-crossfade", TransitionTypeCrossfade, 27400, "asset-square")
	weightedText := textClip(
		"playback-weighted-text",
		"Canonical weighted media plus text plus cursor",
		"DejaVu Sans",
		"playback-font-v1",
		28200,
	)
	weightedTextCursor := mediaClip("playback-weighted-text-cursor", "asset-landscape", 28200, 1200)
	weightedTextCursor.Transform["x"] = 180.0
	weightedTextCursor.Transform["y"] = -110.0
	weightedTextCursor.Transform["scale"] = 0.22
	weightedTextCursor.Cursor = &TimelineCursor{
		Visible: true,
		Scale:   0.75,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 80, Y: 80},
			{TimeMS: 600, X: 320, Y: 180},
			{TimeMS: 1199, X: 560, Y: 280},
		},
	}

	weightedInvalidOut, weightedInvalidIn := pair("weighted-invalid-text-crossfade", TransitionTypeCrossfade, 30600, "asset-square")
	weightedInvalidText := textClip(
		"playback-weighted-invalid-text",
		"Weighted media must fall back when text fails",
		"Invalid Playback Font",
		"playback-font-invalid-v1",
		31400,
	)

	weightedBudgetTextOut, weightedBudgetTextIn := pair("weighted-text-budget-crossfade", TransitionTypeCrossfade, 33800, "asset-landscape")
	weightedBudgetText := textClip(
		"playback-weighted-budget-text",
		"Ready text must fall back when weighted runtime defers",
		"DejaVu Sans",
		"playback-font-v1",
		34600,
	)

	unsupportedCursor := mediaClip("playback-cursor-unsupported-fade", "asset-landscape", 36200, 1200)
	unsupportedCursor.FadeInMS = 200
	unsupportedCursor.Cursor = &TimelineCursor{
		Visible: true,
		Scale:   1,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 120, Y: 100},
			{TimeMS: 1199, X: 520, Y: 260},
		},
	}

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
		track("track-text-resource", resourceText),
		track("track-text-family", familyText),
		track("track-text-invalid-font", invalidFontText),
		track("track-cursor", cursor),
		track("track-mixed-text", mixedText),
		track("track-mixed-cursor", mixedCursor),
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
		track("track-supported-media-text-media", supportedMediaTextMedia),
		track("track-supported-media-text", supportedMediaText),
		track("track-weighted-text-out", weightedTextOut),
		track("track-weighted-text-in", weightedTextIn),
		track("track-weighted-text-cursor", weightedTextCursor),
		track("track-weighted-text", weightedText),
		track("track-weighted-invalid-text-out", weightedInvalidOut),
		track("track-weighted-invalid-text-in", weightedInvalidIn),
		track("track-weighted-invalid-text", weightedInvalidText),
		track("track-weighted-budget-text-out", weightedBudgetTextOut),
		track("track-weighted-budget-text-in", weightedBudgetTextIn),
		track("track-weighted-budget-text", weightedBudgetText),
		track("track-cursor-unsupported-fade", unsupportedCursor),
		audio,
	}

	assets := []ParityFixtureAsset{
		{ID: "asset-landscape", Kind: "video", Width: 640, Height: 360, DurationMS: 40000, Description: "deterministic landscape H.264 video"},
		{ID: "asset-square", Kind: "image", Width: 512, Height: 512, DurationMS: 38000, Description: "deterministic square PNG"},
		{ID: "asset-audio", Kind: "audio", DurationMS: 40000, Description: "deterministic continuous mono playback clock"},
		{ID: "asset-font", Kind: "font", Description: "DejaVu Sans Bold resource-backed browser playback font"},
		{ID: "asset-font-invalid", Kind: "font", Description: "intentionally invalid TTF bytes for fail-closed browser font evidence"},
	}
	// Once the whole visual frame fails closed, the established preview pair plan
	// is intentionally legacy. Runtime markers are retained separately so text
	// and weighted readiness remain distinguishable from structural deferrals.
	cases := []PlaybackParityCase{
		{Name: "video-canonical-playback", FrameIndex: 6, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "image-canonical-playback", FrameIndex: 48, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},
		{Name: "resource-text-canonical-playback", FrameIndex: 90, ObserveMS: 450, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-text-resource", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true, RequireAdvancingFrames: true},
		{Name: "family-text-fallback", FrameIndex: 132, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "text-playback-runtime-deferred:playback-text-family:resource-font-required", ExpectedTransitionMode: "legacy", ExpectedTextRuntime: "deferred", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-text-family", ExpectedTextTrace: []string{"deferred:playback-text-family:resource-font-required"}},
		{Name: "invalid-font-text-fallback", FrameIndex: 174, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "text-playback-runtime-failed:playback-text-invalid-font:font-face-load-failed", ExpectedTransitionMode: "legacy", ExpectedTextRuntime: "failed", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-text-invalid-font", ExpectedTextTrace: []string{"font-face-not-ready", "failed:playback-text-invalid-font:font-face-load-failed"}},
		{Name: "cursor-canonical-playback", FrameIndex: 216, ObserveMS: 600, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireCursorHighlight: true, RequireCursorClickToggle: true, RequireAdvancingFrames: true},
		{Name: "mixed-text-cursor-canonical", FrameIndex: 258, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-mixed-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-mixed-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireTextLayout: true, RequireAdvancingFrames: true},
		{Name: "weighted-crossfade-canonical", FrameIndex: 324, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-crossfade", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-zoom-canonical", FrameIndex: 402, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-zoom", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-dip-canonical", FrameIndex: 480, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-dip", RequireWeightedCanvas: true, RequireAdvancingFrames: true},
		{Name: "weighted-decoder-budget-fallback", FrameIndex: 558, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-weighted-runtime-deferred:weighted-budget-crossfade-out:decoder-budget-poster", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "deferred", ExpectedWeightedConsumer: "legacy-time-fallback", ExpectedWeightedPairID: "weighted-budget-crossfade", DecoderBudget: 1},
		{Name: "mixed-transition-fallback", FrameIndex: 636, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-plan-mixed", ExpectedTransitionMode: "legacy"},
		{Name: "deferred-transition-fallback", FrameIndex: 708, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-deferred:deferred-slide:pair-inputs-not-adjacent", ExpectedTransitionMode: "legacy"},
		{Name: "media-text-canonical-playback", FrameIndex: 780, ObserveMS: 450, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-supported-media-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true, RequireAdvancingFrames: true},
		{Name: "weighted-text-canonical-playback", FrameIndex: 822, ObserveMS: 250, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-text-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-weighted-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-weighted-text-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireWeightedCanvas: true, RequireTextLayout: true, RequireAdvancingFrames: true},
		{Name: "weighted-invalid-text-fallback", FrameIndex: 948, ObserveMS: 350, ExpectedMode: "legacy-time-fallback", ExpectedReason: "text-playback-runtime-failed:playback-weighted-invalid-text:font-face-load-failed", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "legacy-time-fallback", ExpectedWeightedPairID: "weighted-invalid-text-crossfade", ExpectedTextRuntime: "failed", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-weighted-invalid-text", ExpectedTextTrace: []string{"font-face-not-ready", "failed:playback-weighted-invalid-text:font-face-load-failed"}, RequireWeightedCanvas: true},
		{Name: "weighted-text-decoder-budget-fallback", FrameIndex: 1044, ObserveMS: 350, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-weighted-runtime-deferred:weighted-text-budget-crossfade-out:decoder-budget-poster", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "deferred", ExpectedWeightedConsumer: "legacy-time-fallback", ExpectedWeightedPairID: "weighted-text-budget-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-weighted-budget-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true, DecoderBudget: 1},
		{Name: "unsupported-cursor-fade-fallback", FrameIndex: 1092, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "cursor-playback-deferred:playback-cursor-unsupported-fade:fade-unsupported", ExpectedTransitionMode: "legacy", ExpectedCursorConsumer: "legacy-time-fallback", ExpectedCursorClipID: "playback-cursor-unsupported-fade", RequireCursorSurface: true},
	}
	return doc, assets, cases
}
