package video

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

const ParityTortureFixtureName = "parity-torture-v1"

type ParityFixtureAsset struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
	Description string `json:"description"`
}

type ParityFrameSample struct {
	Name       string `json:"name"`
	FrameIndex int64  `json:"frame_index"`
	TimeMS     int64  `json:"time_ms"`
	Reason     string `json:"reason"`
}

// ParityTortureFixture is the deterministic Phase 0 contract fixture. Source
// media bytes are generated from Assets by the fixture command/test harness;
// the timeline itself contains no host-specific paths or random identifiers.
func ParityTortureFixture() (TimelineDocument, []ParityFixtureAsset) {
	duration := int64(20000)
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = duration
	doc.Canvas.Background = "#10131a"
	doc.Metadata = map[string]any{
		"fixture":          ParityTortureFixtureName,
		"range_export":     map[string]any{"start_ms": 2250, "end_ms": 17750},
		"audio_processing": map[string]any{"sample_rate": 48000, "input_channels": 1, "output_channels": 2, "normalize": true},
	}
	doc.Markers = []TimelineMarker{
		{ID: "marker-clip-boundary", TimeMS: 4000, Label: "Exact clip boundary"},
		{ID: "marker-transition", TimeMS: 7500, Label: "Transition midpoint"},
		{ID: "marker-range-in", TimeMS: 2250, Label: "Range in"},
		{ID: "marker-range-out", TimeMS: 17750, Label: "Range out"},
	}
	doc.Scenes = []TimelineScene{
		{ID: "scene-a", Name: "Camera approach", StartMS: 0, DurationMS: 10000, Camera: &TimelineCamera{FieldOfView: 50, FocusDepth: 0.4, Keyframes: []TimelineKeyframe{
			{ID: "camera-x-0", Property: "x", TimeMS: 0, Value: -18, Easing: "linear"},
			{ID: "camera-x-1", Property: "x", TimeMS: 10000, Value: 22, Easing: "ease-in-out", Curve: &MotionCurve{Type: "bezier", X1: .42, Y1: 0, X2: .58, Y2: 1}},
			{ID: "camera-z-0", Property: "z", TimeMS: 0, Value: -30, Easing: "ease-out"},
			{ID: "camera-z-1", Property: "z", TimeMS: 10000, Value: 40, Easing: "ease-in", Curve: &MotionCurve{Type: "spring", Stiffness: 170, Damping: 26, Mass: 1}},
		}}},
		{ID: "scene-b", Name: "Effects and rack focus", StartMS: 10000, DurationMS: 10000, Camera: &TimelineCamera{RotationX: 5, RotationY: -7, RotationZ: 2, FieldOfView: 42, FocusDepth: .7}, Effects: parityEffects("scene")},
	}

	baseTransform := func(x, y float64) map[string]any {
		return map[string]any{"x": x, "y": y, "z": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0}
	}
	media := TimelineTrack{ID: "track-media", Type: TrackTypeLayer, Name: "Media rates and trims", Visible: true}
	rates := []float64{.25, .5, 1, 2, 4}
	for i, rate := range rates {
		start := int64(i * 4000)
		assetID := []string{"asset-landscape", "asset-portrait", "asset-square", "asset-landscape", "asset-portrait"}[i]
		transform := baseTransform(float64((i%3-1)*55), float64((i%2)*24-12))
		if i == 1 {
			transform["crop"] = map[string]any{"top": .08, "right": .12, "bottom": .16, "left": .04}
			transform["anchor_x"] = .2
			transform["anchor_y"] = .8
		}
		if i == 2 {
			transform["scale_x"] = 1.25
			transform["scale_y"] = .75
			transform["rotation"] = 13.0
			transform["rotation_x"] = 8.0
			transform["rotation_y"] = -11.0
			transform["z"] = 30.0
		}
		z := 4
		clip := TimelineClip{ID: fmt.Sprintf("clip-rate-%s", strings.ReplaceAll(fmt.Sprintf("%.2f", rate), ".", "_")), AssetID: assetID, StartMS: start, DurationMS: 4000, TrimInMS: int64(i * 100), TrimOutMS: int64(i*100) + int64(math.Round(4000*rate)), PlaybackRate: rate, ZIndex: &z, Transform: transform, FadeInMS: 350, FadeOutMS: 425, Effects: []TimelineEffect{}, Keyframes: parityKeyframes()}
		if i == 1 {
			volume := 1.35
			clip.Volume = &volume
		}
		if i == 2 {
			clip.Cursor = &TimelineCursor{Visible: true, Scale: 1.25, Highlight: true, ClickRings: true, Smoothing: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 80, Y: 80}, {TimeMS: 1800, X: 320, Y: 180, Click: true}, {TimeMS: 3999, X: 560, Y: 280}}}
			clip.Effects = parityEffects("clip")
		}
		if i == 3 {
			clip.Transitions = parityTransitions()
		}
		media.Clips = append(media.Clips, clip)
	}

	equalZ := 4
	overlay := TimelineTrack{ID: "track-overlay", Type: TrackTypeLayer, Name: "Overlaps and equal z-index", Visible: true, Clips: []TimelineClip{
		{ID: "clip-square-overlay", AssetID: "asset-square", StartMS: 3000, DurationMS: 6000, TrimOutMS: 6000, PlaybackRate: 1, ZIndex: &equalZ, Transform: map[string]any{"x": 170.0, "y": -80.0, "scale": .55, "rotation": -9.0, "opacity": .82}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
	}}
	hidden := TimelineTrack{ID: "track-hidden", Type: TrackTypeLayer, Name: "Hidden visual", Visible: false, Clips: []TimelineClip{{ID: "clip-hidden", AssetID: "asset-landscape", StartMS: 0, DurationMS: duration, TrimOutMS: duration, Transform: baseTransform(0, 0), Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}}}}
	muted := TimelineTrack{ID: "track-muted", Type: TrackTypeAudio, Name: "Muted audio", Muted: true, Visible: true, Clips: []TimelineClip{{ID: "clip-muted-audio", AssetID: "asset-audio", StartMS: 0, DurationMS: duration, TrimOutMS: duration, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}}}}
	solo := TimelineTrack{ID: "track-solo", Type: TrackTypeAudio, Name: "Solo audio and channel conversion", Solo: true, Visible: true, Clips: []TimelineClip{{ID: "clip-solo-audio", AssetID: "asset-audio", StartMS: 500, DurationMS: 19000, TrimInMS: 250, TrimOutMS: 19250, FadeInMS: 500, FadeOutMS: 750, Volume: floatPtr(1.4), Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{{ID: "audio-volume-0", Property: "volume", TimeMS: 0, Value: .25, Easing: "linear"}, {ID: "audio-volume-1", Property: "volume", TimeMS: 9500, Value: 1.5, Easing: "ease-in-out"}, {ID: "audio-volume-2", Property: "volume", TimeMS: 19000, Value: .5, Easing: "ease-out"}}, Metadata: map[string]any{"channel_conversion": "mono_to_stereo", "normalize_lufs": -14}}}}
	audioOnly := TimelineTrack{ID: "track-audio-only-video", Type: TrackTypeLayer, Name: "Audio-only video", Visible: true, Clips: []TimelineClip{{ID: "clip-audio-only-video", AssetID: "asset-landscape", StartMS: 2500, DurationMS: 9000, TrimInMS: 500, TrimOutMS: 9500, AudioOnly: true, Volume: floatPtr(.8), Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}}}}

	text := TimelineTrack{ID: "track-text", Type: TrackTypeText, Name: "Text shaping", Visible: true, Clips: []TimelineClip{
		{ID: "text-bundled", StartMS: 1000, DurationMS: 7000, TrimOutMS: 7000, Transform: baseTransform(0, -105), Text: &TimelineText{Text: "Multiline parity\nTypography  AV", FontFamily: "Inter", FontSize: 34, FontWeight: "700", Color: "#ffffff", Background: "#111827cc", Stroke: "#000000", StrokeWidth: 2, Shadow: true, TextAlign: "center", LineHeight: 1.25, LetterSpacing: 1.5, BorderRadius: 12}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
		{ID: "text-missing-emoji", StartMS: 8500, DurationMS: 8500, TrimOutMS: 8500, Transform: baseTransform(0, 100), Text: &TimelineText{Text: "Missing Font → fallback 🚀\nمرحبا · こんにちは · नमस्ते", FontFamily: "ParityMissingFont", FontSize: 27, FontWeight: "500", Color: "#fde68a", Background: "#00000099", Stroke: "#4c1d95", StrokeWidth: 1, Shadow: true, TextAlign: "left", LineHeight: 1.4, LetterSpacing: .5, BorderRadius: 6}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
	}}

	shapeTrack := TimelineTrack{ID: "track-shapes", Type: TrackTypeShape, Name: "Every shape and annotation", Visible: true}
	shapeKinds := []string{ShapeKindRectangle, ShapeKindHighlight, ShapeKindBlur, ShapeKindRoundedRectangle, ShapeKindEllipse, ShapeKindArrow, ShapeKindLine, ShapeKindSpeechBubble, ShapeKindSpotlight, ShapeKindPixelate, ShapeKindCheckmark, ShapeKindXMark, ShapeKindStepMarker, ShapeKindLabel}
	for i, kind := range shapeKinds {
		shapeTrack.Clips = append(shapeTrack.Clips, TimelineClip{ID: "shape-" + kind, StartMS: int64(i * 1250), DurationMS: 2500, TrimOutMS: 2500, ZIndex: intPtr(20 + i), Transform: map[string]any{"x": float64((i%5 - 2) * 105), "y": float64((i%3 - 1) * 78), "scale": 1.0, "rotation": float64(i*3 - 12), "opacity": .72}, Shape: &TimelineShape{Kind: kind, Width: 110, Height: 64, Fill: "#22d3ee55", Stroke: "#f8fafc", StrokeWidth: 3, BlurRadius: 10, CornerRadius: 18}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}})
	}

	caption := TimelineTrack{ID: "track-captions", Type: TrackTypeCaption, Name: "Burn-in captions", Visible: true, Clips: []TimelineClip{
		{ID: "caption-1", StartMS: 2000, DurationMS: 3000, TrimOutMS: 3000, Transform: baseTransform(0, 140), Text: &TimelineText{Text: "Caption boundary one", FontFamily: "Inter", FontSize: 24, FontWeight: "600", Color: "#ffffff", Background: "#000000cc", TextAlign: "center", LineHeight: 1.2, BorderRadius: 5}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
		{ID: "caption-2", StartMS: 5000, DurationMS: 3000, TrimOutMS: 3000, Transform: baseTransform(0, 140), Text: &TimelineText{Text: "字幕 · الترجمة · captions", FontFamily: "Inter", FontSize: 24, FontWeight: "600", Color: "#ffffff", Background: "#000000cc", TextAlign: "center", LineHeight: 1.2, BorderRadius: 5}, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
	}}
	doc.Tracks = []TimelineTrack{media, overlay, hidden, muted, solo, audioOnly, text, shapeTrack, caption}

	assets := []ParityFixtureAsset{
		{ID: "asset-landscape", Kind: "video", Width: 640, Height: 360, DurationMS: 24000, Description: "testsrc2 landscape video with stereo tone"},
		{ID: "asset-portrait", Kind: "video", Width: 360, Height: 640, DurationMS: 24000, Description: "portrait color bars with mono tone"},
		{ID: "asset-square", Kind: "image", Width: 512, Height: 512, DurationMS: 20000, Description: "square checkerboard still"},
		{ID: "asset-audio", Kind: "audio", DurationMS: 24000, Description: "deterministic mono swept tone and impulses"},
	}
	return doc, assets
}

// ParityFrameSamples selects boundary, keyframe, transition, scene, marker,
// and seeded-random samples. Times map to frames using floor semantics, which
// matches the browser playhead interval [frame/fps, (frame+1)/fps).
func ParityFrameSamples(doc TimelineDocument, seed int64, randomCount int) []ParityFrameSample {
	fps := doc.Canvas.FPS
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	totalFrames := int64(math.Ceil(float64(doc.DurationMS) * float64(fps) / 1000))
	if totalFrames < 1 {
		totalFrames = 1
	}
	frameDurationMS := 1000 / float64(fps)
	samples := map[int64]ParityFrameSample{}
	add := func(timeMS int64, name, reason string) {
		if timeMS < 0 {
			timeMS = 0
		}
		if timeMS >= doc.DurationMS {
			timeMS = maxInt64(0, doc.DurationMS-1)
		}
		frame := int64(math.Floor(float64(timeMS) * float64(fps) / 1000))
		if frame >= totalFrames {
			frame = totalFrames - 1
		}
		if existing, exists := samples[frame]; exists {
			if !strings.Contains(existing.Reason, reason) {
				existing.Reason += "; " + reason
			}
			if !strings.Contains(existing.Name, name) {
				existing.Name += "+" + name
			}
			samples[frame] = existing
			return
		}
		samples[frame] = ParityFrameSample{Name: name, FrameIndex: frame, TimeMS: int64(math.Round(float64(frame) * frameDurationMS)), Reason: reason}
	}
	add(0, "timeline-start", "timeline boundary")
	add(doc.DurationMS-1, "timeline-end", "last timeline frame")
	for _, track := range doc.Tracks {
		for _, clip := range track.Clips {
			add(clip.StartMS, "clip-start-"+clip.ID, "clip start")
			add(clip.StartMS+clip.DurationMS-1, "clip-end-"+clip.ID, "last frame before clip end")
			for _, kf := range clip.Keyframes {
				add(clip.StartMS+kf.TimeMS, "keyframe-"+kf.ID, "keyframe")
			}
			for _, transition := range clip.Transitions {
				add(clip.StartMS+transition.DurationMS/2, "transition-in-"+transition.ID, "transition midpoint")
				add(clip.StartMS+clip.DurationMS-transition.DurationMS/2, "transition-out-"+transition.ID, "transition midpoint")
			}
		}
	}
	for _, scene := range doc.Scenes {
		add(scene.StartMS, "scene-start-"+scene.ID, "scene boundary")
		add(scene.StartMS+scene.DurationMS-1, "scene-end-"+scene.ID, "last frame before scene end")
		if scene.Camera != nil {
			for _, kf := range scene.Camera.Keyframes {
				add(scene.StartMS+kf.TimeMS, "camera-keyframe-"+kf.ID, "camera keyframe")
			}
		}
	}
	for _, marker := range doc.Markers {
		add(marker.TimeMS, "marker-"+marker.ID, "timeline marker")
	}
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < randomCount; i++ {
		frame := rng.Int63n(totalFrames)
		add(int64(math.Round(float64(frame)*frameDurationMS)), fmt.Sprintf("seeded-random-%02d", i+1), "seeded random")
	}
	out := make([]ParityFrameSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FrameIndex < out[j].FrameIndex })
	return out
}

func parityKeyframes() []TimelineKeyframe {
	return []TimelineKeyframe{
		{ID: "kf-x-linear", Property: "x", TimeMS: 0, Value: -35, Easing: "linear"},
		{ID: "kf-x-ease-in", Property: "x", TimeMS: 700, Value: -10, Easing: "ease-in"},
		{ID: "kf-x-ease-out", Property: "x", TimeMS: 1400, Value: 12, Easing: "ease-out"},
		{ID: "kf-x-ease-in-out", Property: "x", TimeMS: 2100, Value: 30, Easing: "ease-in-out"},
		{ID: "kf-x-step", Property: "x", TimeMS: 2800, Value: -4, Easing: "step", Curve: &MotionCurve{Type: "step"}},
		{ID: "kf-x-bezier", Property: "x", TimeMS: 3400, Value: 24, Easing: "ease-in-out", Curve: &MotionCurve{Type: "bezier", X1: .25, Y1: .1, X2: .25, Y2: 1}},
		{ID: "kf-x-spring", Property: "x", TimeMS: 4000, Value: 0, Easing: "ease-out", Curve: &MotionCurve{Type: "spring", Stiffness: 170, Damping: 26, Mass: 1}},
		{ID: "kf-rotation-x", Property: "rotation_x", TimeMS: 1200, Value: 12, Easing: "ease-in-out"},
		{ID: "kf-rotation-y", Property: "rotation_y", TimeMS: 2400, Value: -14, Easing: "ease-in-out"},
		{ID: "kf-scale-x", Property: "scale_x", TimeMS: 1800, Value: 1.3, Easing: "ease-out"},
		{ID: "kf-scale-y", Property: "scale_y", TimeMS: 3000, Value: .7, Easing: "ease-in"},
	}
}

func parityEffects(prefix string) []TimelineEffect {
	types := []string{EffectTypeBlur, EffectTypeBrightness, EffectTypeContrast, EffectTypeSaturation, EffectTypeGrayscale, EffectTypeShadow, EffectTypeBackgroundBlur, EffectTypeChromaKey, EffectTypeSharpen, EffectTypeVignette, EffectTypeFilmGrain, EffectTypeBloom, EffectTypeColorGrade, EffectTypeEdgeFade, EffectTypeRGBSplit, EffectTypeGhostTrail, EffectTypeMotionBlur, EffectTypeDepthOfField, EffectTypeRackFocus}
	effects := make([]TimelineEffect, 0, len(types))
	for _, kind := range types {
		effects = append(effects, TimelineEffect{ID: prefix + "-effect-" + kind, Type: kind, Enabled: true, Params: map[string]any{"amount": .35, "color": "#00ff00", "threshold": .2}})
	}
	return effects
}
func parityTransitions() []TimelineTransition {
	types := []string{TransitionTypeFade, TransitionTypeCrossfade, TransitionTypeDipToBlack, TransitionTypeSlide, TransitionTypeWipe, TransitionTypeZoom}
	result := make([]TimelineTransition, 0, len(types))
	for i, kind := range types {
		result = append(result, TimelineTransition{ID: "transition-" + kind, Type: kind, DurationMS: int64(300 + i*60), Direction: []string{"left", "right", "up", "down"}[i%4]})
	}
	return result
}
func floatPtr(value float64) *float64 { return &value }
func intPtr(value int) *int           { return &value }
