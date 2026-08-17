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
	if req.Settings.DiagnosticAudio {
		// The underlying audio graph evaluates rate, gain keyframes, fades,
		// track mute/solo, and timeline processing directly. Visual fidelity
		// expansion would split transform-animated video into hundreds of audio
		// inputs and can exceed Windows' process command-line limit.
		return r.delegate.Render(ctx, req, progress)
	}
	fps := req.Settings.FPS
	if fps <= 0 {
		fps = req.Timeline.Canvas.FPS
	}
	req.Timeline = ExpandTimelineForFidelity(req.Timeline, fps, r.maxSegmentsPerClip)
	if req.Settings.DiagnosticFrameIndex != nil {
		// Fidelity expansion can create hundreds of short sampled segments per
		// clip. A one-frame diagnostic needs only the segments active at that
		// output timestamp. Filtering after expansion preserves the exact
		// legacy sampling result while keeping the FFmpeg graph bounded.
		timeMS := int64(math.Floor(float64(*req.Settings.DiagnosticFrameIndex) * 1000 / float64(fps)))
		if req.Settings.RangeEndMS > req.Settings.RangeStartMS {
			timeMS += req.Settings.RangeStartMS
		}
		req.Timeline = FilterTimelineAtDiagnosticTime(req.Timeline, timeMS)
	}
	return r.delegate.Render(ctx, req, progress)
}

// FilterTimelineAtDiagnosticTime keeps the original timeline clock and only
// the expanded clips active in the requested half-open frame interval. This
// is render-only and never mutates the persisted document.
func FilterTimelineAtDiagnosticTime(doc TimelineDocument, timeMS int64) TimelineDocument {
	out := doc
	out.Tracks = make([]TimelineTrack, len(doc.Tracks))
	for ti, track := range doc.Tracks {
		copied := track
		copied.Clips = make([]TimelineClip, 0, len(track.Clips))
		for _, clip := range track.Clips {
			if timeMS >= clip.StartMS && timeMS < clip.StartMS+clip.DurationMS {
				copied.Clips = append(copied.Clips, clip)
			}
		}
		out.Tracks[ti] = copied
	}
	return out
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
			if clipNeedsSampling(clip) || clipNeedsCameraSampling(out.Scenes, clip) {
				expanded = append(expanded, sampleRenderClip(clip, out.Scenes, out.Canvas, fps, maxSegments)...)
			} else {
				expanded = append(expanded, applySceneCamera(clip, out.Scenes, out.Canvas, clip.StartMS+clip.DurationMS/2))
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
	out.Scenes = make([]TimelineScene, len(doc.Scenes))
	for i, scene := range doc.Scenes {
		out.Scenes[i] = scene
		out.Scenes[i].Metadata = cloneAnyMap(scene.Metadata)
		out.Scenes[i].Effects = make([]TimelineEffect, len(scene.Effects))
		for ei, effect := range scene.Effects {
			out.Scenes[i].Effects[ei] = effect
			out.Scenes[i].Effects[ei].Params = cloneAnyMap(effect.Params)
		}
		if scene.Camera != nil {
			camera := *scene.Camera
			camera.Keyframes = append([]TimelineKeyframe(nil), scene.Camera.Keyframes...)
			out.Scenes[i].Camera = &camera
		}
	}
	if doc.Metadata != nil {
		out.Metadata = cloneAnyMap(doc.Metadata)
	}
	return out
}

func cloneTimelineClip(clip TimelineClip) TimelineClip {
	out := clip
	out.Transform = cloneAnyMap(clip.Transform)
	out.Keyframes = append([]TimelineKeyframe(nil), clip.Keyframes...)
	out.AnimationBlocks = make([]TimelineAnimationBlock, len(clip.AnimationBlocks))
	for i, block := range clip.AnimationBlocks {
		out.AnimationBlocks[i] = block
		out.AnimationBlocks[i].Params = cloneAnyMap(block.Params)
		out.AnimationBlocks[i].GeneratedKeyframeIDs = append([]string(nil), block.GeneratedKeyframeIDs...)
	}
	out.Metadata = cloneAnyMap(clip.Metadata)
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
		if knownKeyframeProperties[property] || strings.HasPrefix(property, "effect.") || strings.HasPrefix(property, "effect:") {
			return true
		}
		if keyframe.Curve != nil || (keyframe.Easing != "" && keyframe.Easing != rendererEasingLinear) {
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

func sampleRenderClip(clip TimelineClip, scenes []TimelineScene, canvas TimelineCanvas, fps, maxSegments int) []TimelineClip {
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
		for _, property := range []string{"x", "y", "z", "scale", "scale_x", "scale_y", "rotation", "rotation_x", "rotation_y", "rotation_z", "opacity"} {
			if value, ok := evaluateTimelineKeyframes(clip.Keyframes, property, sampleTime); ok {
				segment.Transform[property] = value
			}
		}
		segment.Effects = sampleEffects(clip.Effects, clip.Keyframes, sampleTime)
		segment = applySceneCamera(segment, scenes, canvas, segment.StartMS+duration/2)
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
			eased := curveProgress(progress, next.Curve, next.Easing)
			return prev.Value + (next.Value-prev.Value)*eased, true
		}
	}
	return points[len(points)-1].Value, true
}

func clipNeedsCameraSampling(scenes []TimelineScene, clip TimelineClip) bool {
	for _, scene := range scenes {
		if scene.Camera == nil || len(scene.Camera.Keyframes) == 0 {
			continue
		}
		if clip.StartMS < scene.StartMS+scene.DurationMS && clip.StartMS+clip.DurationMS > scene.StartMS {
			return true
		}
	}
	return false
}

// applySceneCamera projects a 2.5D layer into the 2D FFmpeg composition. A
// positive layer Z moves toward the camera and grows; timeline layer order is
// still authoritative, with Z affecting projection rather than stack order.
func applySceneCamera(clip TimelineClip, scenes []TimelineScene, canvas TimelineCanvas, timelineMS int64) TimelineClip {
	var scene *TimelineScene
	for i := range scenes {
		if timelineMS >= scenes[i].StartMS && timelineMS < scenes[i].StartMS+scenes[i].DurationMS {
			scene = &scenes[i]
			break
		}
	}
	if scene == nil || scene.Camera == nil {
		return clip
	}
	result := cloneTimelineClip(clip)
	if result.Transform == nil {
		result.Transform = map[string]any{}
	}
	camera := *scene.Camera
	relativeMS := timelineMS - scene.StartMS
	for property, target := range map[string]*float64{
		"x": &camera.X, "y": &camera.Y, "z": &camera.Z,
		"rotation_x": &camera.RotationX, "rotation_y": &camera.RotationY, "rotation_z": &camera.RotationZ,
		"field_of_view": &camera.FieldOfView, "focus_depth": &camera.FocusDepth,
	} {
		if value, ok := evaluateTimelineKeyframes(camera.Keyframes, property, relativeMS); ok {
			*target = value
		}
	}
	layerX := numericOrZero(result.Transform, "x")
	layerY := numericOrZero(result.Transform, "y")
	layerZ := numericOrZero(result.Transform, "z")
	fieldOfView := camera.FieldOfView
	if fieldOfView <= 0 {
		fieldOfView = 50
	}
	focal := float64(maxInt(1, canvas.Height)) / (2 * math.Tan(clampFloat(fieldOfView, 1, 179)*math.Pi/360))
	relativeZ := layerZ - camera.Z
	denominator := math.Max(focal*0.1, focal-relativeZ)
	projection := clampFloat(focal/denominator, 0.1, 10)
	result.Transform["x"] = (layerX - camera.X) * projection
	result.Transform["y"] = (layerY - camera.Y) * projection
	baseScale, ok := numericTransform(result.Transform, "scale")
	if !ok || baseScale <= 0 {
		baseScale = 1
	}
	result.Transform["scale"] = baseScale * projection
	baseRotation := numericOrZero(result.Transform, "rotation_z")
	if baseRotation == 0 {
		baseRotation = numericOrZero(result.Transform, "rotation")
	}
	result.Transform["rotation_z"] = baseRotation - camera.RotationZ
	return result
}

// curveProgress evaluates the additive curve model. Springs are segment-local:
// position starts at zero with zero velocity for every keyframe pair, so a
// middle keyframe never inherits velocity from the previous segment.
func curveProgress(t float64, curve *MotionCurve, fallback string) float64 {
	t = clamp01(t)
	if curve == nil {
		return easeValue(t, fallback)
	}
	switch strings.ToLower(strings.TrimSpace(curve.Type)) {
	case "bezier":
		return cubicBezierProgress(t, curve.X1, curve.Y1, curve.X2, curve.Y2)
	case "spring":
		stiffness, damping, mass := curve.Stiffness, curve.Damping, curve.Mass
		if stiffness <= 0 {
			stiffness = 170
		}
		if damping <= 0 {
			damping = 26
		}
		if mass <= 0 {
			mass = 1
		}
		response := func(at float64) float64 {
			omega0 := math.Sqrt(stiffness / mass)
			zeta := damping / (2 * math.Sqrt(stiffness*mass))
			if zeta < 1-1e-6 {
				omegaD := omega0 * math.Sqrt(1-zeta*zeta)
				return 1 - math.Exp(-zeta*omega0*at)*(math.Cos(omegaD*at)+(zeta*omega0/omegaD)*math.Sin(omegaD*at))
			}
			if zeta > 1+1e-6 {
				s := math.Sqrt(zeta*zeta - 1)
				r1 := -omega0 * (zeta - s)
				r2 := -omega0 * (zeta + s)
				return 1 - (r2*math.Exp(r1*at)-r1*math.Exp(r2*at))/(r2-r1)
			}
			return 1 - math.Exp(-omega0*at)*(1+omega0*at)
		}
		end := response(1)
		if math.Abs(end) < 1e-9 {
			return t
		}
		return response(t) / end
	default:
		return easeValue(t, curve.Type)
	}
}

func cubicBezierProgress(x, x1, y1, x2, y2 float64) float64 {
	x = clamp01(x)
	bezier := func(t, a, b float64) float64 {
		u := 1 - t
		return 3*u*u*t*a + 3*u*t*t*b + t*t*t
	}
	derivative := func(t, a, b float64) float64 {
		u := 1 - t
		return 3*u*u*a + 6*u*t*(b-a) + 3*t*t*(1-b)
	}
	t := x
	for i := 0; i < 8; i++ {
		d := derivative(t, x1, x2)
		if math.Abs(d) < 1e-7 {
			break
		}
		t = clamp01(t - (bezier(t, x1, x2)-x)/d)
	}
	low, high := 0.0, 1.0
	for i := 0; i < 12; i++ {
		value := bezier(t, x1, x2)
		if math.Abs(value-x) < 1e-7 {
			break
		}
		if value < x {
			low = t
		} else {
			high = t
		}
		t = (low + high) / 2
	}
	return bezier(t, y1, y2)
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
