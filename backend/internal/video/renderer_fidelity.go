package video

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// FidelityRenderer expands timeline features that are difficult to express in
// one portable FFmpeg filter graph into deterministic, short static segments.
// The wrapped renderer remains the source of truth for encoding and media I/O.
type FidelityRenderer struct {
	delegate           Renderer
	maxSegmentsPerClip int
}

const (
	rendererEasingLinear    = "linear"
	rendererEasingEaseIn    = "ease-in"
	rendererEasingEaseOut   = "ease-out"
	rendererEasingEaseInOut = "ease-in-out"
	rendererEasingStep      = "step"
)

// NewFidelityRenderer adds eased transform/effect keyframes, wipe/zoom
// transitions, cursor overlays, click rings, letter-spacing approximation, and
// annotation normalization without changing the persisted timeline document.
func NewFidelityRenderer(delegate Renderer) Renderer {
	return &FidelityRenderer{delegate: delegate, maxSegmentsPerClip: 300}
}

// Render expands a copy of the timeline and delegates the actual encode.
func (r *FidelityRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	if r == nil || r.delegate == nil {
		return nil, fmt.Errorf("video renderer is not configured")
	}
	fps := req.Settings.FPS
	if fps <= 0 {
		fps = req.Timeline.Canvas.FPS
	}
	req.Timeline = ExpandTimelineForFidelity(req.Timeline, fps, r.maxSegmentsPerClip)
	return r.delegate.Render(ctx, req, progress)
}

// ExpandTimelineForFidelity returns a render-only timeline. It never mutates
// the persisted editor document.
func ExpandTimelineForFidelity(doc TimelineDocument, fps, maxSegments int) TimelineDocument {
	if fps <= 0 {
		fps = 30
	}
	if fps > 60 {
		fps = 60
	}
	if maxSegments <= 0 {
		maxSegments = 300
	}
	out := cloneTimelineDocument(doc)
	cursorTrack := TimelineTrack{
		ID: uuid.NewString(), Type: TrackTypeLayer, Name: "Renderer cursor overlays",
		Locked: false, Muted: true, Visible: true, Clips: []TimelineClip{},
	}
	for ti := range out.Tracks {
		expanded := make([]TimelineClip, 0, len(out.Tracks[ti].Clips))
		for _, original := range out.Tracks[ti].Clips {
			clip := normalizeRenderClip(original)
			cursorTrack.Clips = append(cursorTrack.Clips, cursorOverlayClips(clip, fps, maxSegments)...)
			clip.Cursor = nil
			if clipNeedsSampling(clip) {
				expanded = append(expanded, sampleRenderClip(clip, fps, maxSegments)...)
			} else {
				expanded = append(expanded, clip)
			}
		}
		out.Tracks[ti].Clips = expanded
	}
	if len(cursorTrack.Clips) > 0 {
		out.Tracks = append(out.Tracks, cursorTrack)
	}
	return out
}

func cloneTimelineDocument(doc TimelineDocument) TimelineDocument {
	out := doc
	out.Tracks = make([]TimelineTrack, len(doc.Tracks))
	for ti, track := range doc.Tracks {
		out.Tracks[ti] = track
		out.Tracks[ti].Clips = make([]TimelineClip, len(track.Clips))
		for ci, clip := range track.Clips {
			out.Tracks[ti].Clips[ci] = cloneTimelineClip(clip)
		}
	}
	out.Markers = append([]TimelineMarker(nil), doc.Markers...)
	if doc.Metadata != nil {
		out.Metadata = cloneAnyMap(doc.Metadata)
	}
	return out
}

func cloneTimelineClip(clip TimelineClip) TimelineClip {
	out := clip
	out.Transform = cloneAnyMap(clip.Transform)
	out.Keyframes = append([]TimelineKeyframe(nil), clip.Keyframes...)
	out.Transitions = append([]TimelineTransition(nil), clip.Transitions...)
	out.Effects = make([]TimelineEffect, len(clip.Effects))
	for i, effect := range clip.Effects {
		out.Effects[i] = effect
		out.Effects[i].Params = cloneAnyMap(effect.Params)
	}
	if clip.Text != nil {
		copied := *clip.Text
		out.Text = &copied
	}
	if clip.Shape != nil {
		copied := *clip.Shape
		out.Shape = &copied
	}
	if clip.Cursor != nil {
		copied := *clip.Cursor
		copied.Events = append([]TimelineCursorEvent(nil), clip.Cursor.Events...)
		out.Cursor = &copied
	}
	return out
}

func cloneAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneAnyMap(nested)
		} else {
			out[key] = value
		}
	}
	return out
}

func normalizeRenderClip(clip TimelineClip) TimelineClip {
	out := cloneTimelineClip(clip)
	if out.Text != nil {
		out.Text.Text = renderTextLayout(out.Text.Text, out.Text.TextAlign, out.Text.LetterSpacing)
	}
	if out.Shape == nil {
		return out
	}
	switch out.Shape.Kind {
	case ShapeKindEllipse:
		out.Shape.Kind = ShapeKindRoundedRectangle
		out.Shape.CornerRadius = float64(maxInt(out.Shape.Width, out.Shape.Height)) / 2
	case ShapeKindLine:
		out.Shape.Kind = ShapeKindRectangle
		if out.Shape.Height <= 0 || out.Shape.Height > 16 {
			out.Shape.Height = maxInt(2, int(out.Shape.StrokeWidth+0.5))
		}
	case ShapeKindArrow:
		out.Shape.Kind = ShapeKindLabel
		ensureAnnotationText(&out, "➜")
	case ShapeKindSpeechBubble:
		out.Shape.Kind = ShapeKindLabel
	case ShapeKindSpotlight:
		out.Shape.Kind = ShapeKindRectangle
		if strings.TrimSpace(out.Shape.Stroke) == "" {
			out.Shape.Stroke = "#facc15"
		}
		if out.Shape.StrokeWidth <= 0 {
			out.Shape.StrokeWidth = 5
		}
	case ShapeKindCheckmark:
		out.Shape.Kind = ShapeKindLabel
		ensureAnnotationText(&out, "✓")
	case ShapeKindXMark:
		out.Shape.Kind = ShapeKindLabel
		ensureAnnotationText(&out, "✕")
	case ShapeKindStepMarker:
		out.Shape.Kind = ShapeKindLabel
		ensureAnnotationText(&out, "1")
	}
	return out
}

func ensureAnnotationText(clip *TimelineClip, fallback string) {
	if clip.Text != nil && strings.TrimSpace(clip.Text.Text) != "" {
		return
	}
	clip.Text = &TimelineText{Text: fallback, FontSize: 44, Color: "#ffffff", TextAlign: "center", Shadow: true}
}

func renderTextLayout(value, align string, letterSpacing float64) string {
	lines := strings.Split(value, "\n")
	if letterSpacing >= 1 {
		for i, line := range lines {
			runes := []rune(line)
			var builder strings.Builder
			for j, r := range runes {
				if j > 0 {
					spaces := int(math.Min(4, math.Max(1, math.Round(letterSpacing/2))))
					builder.WriteString(strings.Repeat(" ", spaces))
				}
				builder.WriteRune(r)
			}
			lines[i] = builder.String()
		}
	}
	maxLen := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > maxLen {
			maxLen = n
		}
	}
	for i, line := range lines {
		width := len([]rune(line))
		switch strings.ToLower(strings.TrimSpace(align)) {
		case "right":
			lines[i] = strings.Repeat(" ", maxLen-width) + line
		case "center":
			lines[i] = strings.Repeat(" ", (maxLen-width)/2) + line
		}
	}
	return strings.Join(lines, "\n")
}

func clipNeedsSampling(clip TimelineClip) bool {
	for _, keyframe := range clip.Keyframes {
		property := strings.ToLower(strings.TrimSpace(keyframe.Property))
		if property == "scale" || property == "opacity" || property == "x" || property == "y" || property == "rotation" || strings.HasPrefix(property, "effect.") || strings.HasPrefix(property, "effect:") {
			return true
		}
		if keyframe.Easing != "" && keyframe.Easing != rendererEasingLinear {
			return true
		}
	}
	for _, transition := range clip.Transitions {
		if transition.Type == TransitionTypeWipe || transition.Type == TransitionTypeZoom {
			return true
		}
	}
	return false
}

func sampleRenderClip(clip TimelineClip, fps, maxSegments int) []TimelineClip {
	frameMS := int64(maxInt(16, int(math.Round(1000/float64(maxInt(1, fps))))))
	segmentCount := int(math.Ceil(float64(clip.DurationMS) / float64(frameMS)))
	if segmentCount > maxSegments {
		segmentCount = maxSegments
		frameMS = int64(maxInt(16, int(math.Ceil(float64(clip.DurationMS)/float64(segmentCount)))))
	}
	result := make([]TimelineClip, 0, segmentCount)
	for offset := int64(0); offset < clip.DurationMS; offset += frameMS {
		duration := minInt64(frameMS, clip.DurationMS-offset)
		if duration <= 0 {
			break
		}
		segment := cloneTimelineClip(clip)
		segment.ID = uuid.NewString()
		segment.StartMS = clip.StartMS + offset
		segment.DurationMS = duration
		sourceOffset := sourceDurationFor(clip, offset)
		sourceDuration := sourceDurationFor(clip, duration)
		segment.TrimInMS = clip.TrimInMS + sourceOffset
		segment.TrimOutMS = segment.TrimInMS + sourceDuration
		sampleTime := offset + duration/2
		if segment.Transform == nil {
			segment.Transform = map[string]any{}
		}
		for _, property := range []string{"x", "y", "scale", "rotation", "opacity"} {
			if value, ok := evaluateTimelineKeyframes(clip.Keyframes, property, sampleTime); ok {
				segment.Transform[property] = value
			}
		}
		segment.Effects = sampleEffects(clip.Effects, clip.Keyframes, sampleTime)
		applySampledTransition(&segment, clip, sampleTime)
		segment.Keyframes = nil
		segment.Transitions = retainNativeTransitions(clip.Transitions)
		segment.Cursor = nil
		result = append(result, segment)
	}
	return result
}

func retainNativeTransitions(transitions []TimelineTransition) []TimelineTransition {
	out := make([]TimelineTransition, 0, len(transitions))
	for _, transition := range transitions {
		if transition.Type != TransitionTypeWipe && transition.Type != TransitionTypeZoom {
			out = append(out, transition)
		}
	}
	return out
}

func applySampledTransition(segment *TimelineClip, original TimelineClip, sampleMS int64) {
	if segment.Transform == nil {
		segment.Transform = map[string]any{}
	}
	for _, transition := range original.Transitions {
		duration := minInt64(transition.DurationMS, original.DurationMS/2)
		if duration <= 0 {
			continue
		}
		inProgress := clamp01(float64(sampleMS) / float64(duration))
		outProgress := clamp01(float64(original.DurationMS-sampleMS) / float64(duration))
		edgeProgress := math.Min(inProgress, outProgress)
		switch transition.Type {
		case TransitionTypeZoom:
			base, _ := numericTransform(segment.Transform, "scale")
			if base <= 0 {
				base = 1
			}
			segment.Transform["scale"] = base * (0.82 + 0.18*easeValue(edgeProgress, rendererEasingEaseOut))
			opacity, ok := numericTransform(segment.Transform, "opacity")
			if !ok {
				opacity = 1
			}
			segment.Transform["opacity"] = opacity * edgeProgress
		case TransitionTypeWipe:
			crop := map[string]any{"top": 0.0, "right": 0.0, "bottom": 0.0, "left": 0.0}
			hidden := 0.95 * (1 - edgeProgress)
			switch strings.ToLower(strings.TrimSpace(transition.Direction)) {
			case "right":
				crop["left"] = hidden
			case "up":
				crop["bottom"] = hidden
			case "down":
				crop["top"] = hidden
			default:
				crop["right"] = hidden
			}
			segment.Transform["crop"] = crop
		}
	}
}

func sampleEffects(effects []TimelineEffect, keyframes []TimelineKeyframe, sampleMS int64) []TimelineEffect {
	out := make([]TimelineEffect, len(effects))
	for i, effect := range effects {
		out[i] = effect
		out[i].Params = cloneAnyMap(effect.Params)
		for _, property := range []string{"effect." + effect.ID + ".amount", "effect:" + effect.ID + ":amount", "effect." + effect.Type + ".amount"} {
			if value, ok := evaluateTimelineKeyframes(keyframes, property, sampleMS); ok {
				out[i].Params["amount"] = value
				break
			}
		}
	}
	return out
}

func evaluateTimelineKeyframes(keyframes []TimelineKeyframe, property string, timeMS int64) (float64, bool) {
	points := make([]TimelineKeyframe, 0)
	for _, keyframe := range keyframes {
		if strings.EqualFold(strings.TrimSpace(keyframe.Property), property) {
			points = append(points, keyframe)
		}
	}
	if len(points) == 0 {
		return 0, false
	}
	sort.Slice(points, func(i, j int) bool { return points[i].TimeMS < points[j].TimeMS })
	if timeMS <= points[0].TimeMS {
		return points[0].Value, true
	}
	for i := 1; i < len(points); i++ {
		next := points[i]
		if timeMS <= next.TimeMS {
			prev := points[i-1]
			span := next.TimeMS - prev.TimeMS
			if span < 1 {
				span = 1
			}
			progress := clamp01(float64(timeMS-prev.TimeMS) / float64(span))
			eased := easeValue(progress, next.Easing)
			return prev.Value + (next.Value-prev.Value)*eased, true
		}
	}
	return points[len(points)-1].Value, true
}

func easeValue(t float64, easing string) float64 {
	t = clamp01(t)
	switch strings.ToLower(strings.TrimSpace(easing)) {
	case rendererEasingStep:
		if t < 1 {
			return 0
		}
		return 1
	case rendererEasingEaseIn:
		return t * t
	case rendererEasingEaseOut:
		return 1 - (1-t)*(1-t)
	case rendererEasingEaseInOut:
		return t * t * (3 - 2*t)
	default:
		return t
	}
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func cursorOverlayClips(clip TimelineClip, fps, maxSegments int) []TimelineClip {
	cursor := clip.Cursor
	if cursor == nil || cursor.Visible == false || len(cursor.Events) == 0 || clip.DurationMS <= 0 {
		return nil
	}
	sampleFPS := minInt(30, maxInt(12, fps))
	step := int64(maxInt(33, int(math.Round(1000/float64(sampleFPS)))))
	count := int(math.Ceil(float64(clip.DurationMS) / float64(step)))
	if count > maxSegments {
		step = int64(maxInt(int(step), int(math.Ceil(float64(clip.DurationMS)/float64(maxSegments)))))
	}
	overlays := make([]TimelineClip, 0, minInt(count*2, maxSegments*2))
	for offset := int64(0); offset < clip.DurationMS; offset += step {
		duration := minInt64(step+8, clip.DurationMS-offset)
		point, ok := interpolateCursor(cursor.Events, offset)
		if !ok {
			continue
		}
		cursorScale := cursor.Scale
		if cursorScale <= 0 {
			cursorScale = 1
		}
		pointer := TimelineClip{
			ID: uuid.NewString(), StartMS: clip.StartMS + offset, DurationMS: duration,
			TrimInMS: 0, TrimOutMS: duration, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
			Transform: map[string]any{"x": point.X, "y": point.Y, "scale": cursorScale, "rotation": 0.0, "opacity": 1.0},
			Text:      &TimelineText{Text: "➤", FontSize: 34, Color: "#ffffff", Stroke: "#111827", StrokeWidth: 2, Shadow: true},
		}
		overlays = append(overlays, pointer)
		if point.Click && cursor.ClickRings {
			size := int(68 * cursorScale)
			ring := TimelineClip{
				ID: uuid.NewString(), StartMS: clip.StartMS + offset, DurationMS: minInt64(220, clip.DurationMS-offset),
				TrimInMS: 0, TrimOutMS: minInt64(220, clip.DurationMS-offset),
				Transform: map[string]any{"x": point.X, "y": point.Y, "scale": 1.0, "rotation": 0.0, "opacity": 0.9},
				Shape:     &TimelineShape{Kind: ShapeKindRectangle, Width: size, Height: size, Stroke: "#f59e0b", StrokeWidth: 5},
				Effects:   []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
			}
			overlays = append(overlays, ring)
		}
	}
	return overlays
}

type sampledCursorPoint struct {
	X, Y  float64
	Click bool
}

func interpolateCursor(events []TimelineCursorEvent, timeMS int64) (sampledCursorPoint, bool) {
	if len(events) == 0 {
		return sampledCursorPoint{}, false
	}
	sorted := append([]TimelineCursorEvent(nil), events...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].TimeMS < sorted[j].TimeMS })
	click := false
	for _, event := range sorted {
		if event.Click && math.Abs(float64(event.TimeMS-timeMS)) <= 160 {
			click = true
		}
	}
	if timeMS <= sorted[0].TimeMS {
		return sampledCursorPoint{X: sorted[0].X, Y: sorted[0].Y, Click: click}, true
	}
	for i := 1; i < len(sorted); i++ {
		next := sorted[i]
		if timeMS <= next.TimeMS {
			prev := sorted[i-1]
			span := next.TimeMS - prev.TimeMS
			if span < 1 {
				span = 1
			}
			progress := clamp01(float64(timeMS-prev.TimeMS) / float64(span))
			return sampledCursorPoint{X: prev.X + (next.X-prev.X)*progress, Y: prev.Y + (next.Y-prev.Y)*progress, Click: click}, true
		}
	}
	last := sorted[len(sorted)-1]
	return sampledCursorPoint{X: last.X, Y: last.Y, Click: click}, true
}
