package video

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

const (
	cursorRasterContractVersion  = "cursor-raster-v1"
	cursorRasterMetadataKey      = "_omnillm_cursor_raster_version"
	cursorRasterScaleMetadataKey = "_omnillm_cursor_raster_scale"
	cursorRasterHighlightKey     = "_omnillm_cursor_raster_highlight"
	cursorRasterClickRingKey     = "_omnillm_cursor_raster_click_ring"
	cursorRasterAssetPrefix      = "__omnillm_cursor_raster_v1_"
	cursorRasterSupersample      = 4
	cursorRasterMaxExactFPS      = 999
	cursorPointerBaseSize        = 64.0
	cursorHighlightRadiusFactor  = 1.1
	cursorClickRingRadiusFactor  = 1.3
	cursorClickRingBorderPixels  = 2.0
)

type cursorRasterSpec struct {
	Scale     float64
	Highlight bool
	ClickRing bool
}

// canonicalCursorRasterOverlayClips converts one supported static-2D cursor
// owner into one exact synthetic image segment per output frame. Cursor x/y and
// click state come from cursor-state-v1 or cursor-state-v2 at the exact rational
// presentation time; the generated full-canvas sprite then inherits the owner's static
// uniform scale, Z rotation, opacity, track visibility, and track ordering.
//
// The function intentionally returns ok=false for combinations that still need
// a renderer-specific approximation (animated/3D/camera/effect/transition
// parents, overlapping same-track siblings, or clips too long for
// the bounded fidelity expansion). Those cases stay on the compatibility path.
func canonicalCursorRasterOverlayClips(
	clip TimelineClip,
	siblings []TimelineClip,
	canvas TimelineCanvas,
	scenes []TimelineScene,
	fps, maxSegments int,
) ([]TimelineClip, bool) {
	if clip.Cursor == nil || len(clip.Cursor.Events) == 0 || clip.DurationMS <= 0 {
		return nil, true
	}
	// Timeline v1 stores visibility as a bool, so false is an explicit no-paint
	// state on this adapter boundary. Do not convert it to the v2 omitted/default
	// visibility semantics used by the renderer-independent contract.
	if !clip.Cursor.Visible {
		return nil, true
	}
	if clip.AssetID == "" || clip.Text != nil || clip.Shape != nil || clip.AudioOnly {
		return nil, false
	}
	if fps <= 0 || fps > cursorRasterMaxExactFPS || maxSegments <= 0 {
		return nil, false
	}
	if clip.FadeInMS > 0 || clip.FadeOutMS > 0 || len(clip.Transitions) > 0 || len(clip.AnimationBlocks) > 0 {
		return nil, false
	}
	for _, effect := range clip.Effects {
		if effect.Enabled {
			return nil, false
		}
	}
	if hasVisualTransformKeyframes(clip.Keyframes) || hasOverlappingSibling(clip, siblings) || hasOverlappingSceneCamera(clip, scenes) {
		return nil, false
	}

	parent, ok := canonicalCursorParentTransform(clip.Transform)
	if !ok {
		return nil, false
	}
	cursor := canonicalCursorStateInput(clip.Cursor)
	startFrame := rendercontract.StartFrame(clip.StartMS, fps)
	endFrame := rendercontract.EndFrame(clip.StartMS+clip.DurationMS, fps)
	if endFrame <= startFrame || endFrame-startFrame > int64(maxSegments) {
		return nil, false
	}

	// Validate the raster extent before generating any clips. Both ring states
	// share the same pointer/highlight geometry; when click rings are authored we
	// reserve the larger ring diameter even if no click lands in this clip.
	spec := cursorRasterSpec{Scale: clip.Cursor.Scale, Highlight: clip.Cursor.Highlight, ClickRing: clip.Cursor.ClickRings}
	if spec.Scale <= 0 {
		spec.Scale = 1
	}
	if !cursorRasterFitsCanvas(canvas.Width, canvas.Height, spec) {
		return nil, false
	}

	overlays := make([]TimelineClip, 0, endFrame-startFrame)
	for frame := startFrame; frame < endFrame; frame++ {
		state, err := rendercontract.EvaluateCursorState(cursor, rendercontract.RationalMilliseconds{
			Numerator:   frame*1000 - clip.StartMS*int64(fps),
			Denominator: int64(fps),
		})
		if err != nil {
			return nil, false
		}
		if state == nil {
			continue
		}
		frameStartMS, frameDurationMS, ok := exactCursorFrameWindow(frame, fps)
		if !ok {
			return nil, false
		}
		x, y := transformCursorOrigin(state.X, state.Y, canvas.Width, canvas.Height, parent)
		frameSpec := cursorRasterSpec{
			Scale:     state.Scale,
			Highlight: state.Highlight,
			ClickRing: state.Click && state.ClickRings,
		}
		assetID := cursorRasterAssetID(canvas.Width, canvas.Height, frameSpec)
		zIndex := cloneIntPointer(clip.ZIndex)
		generated := TimelineClip{
			ID:         fmt.Sprintf("%s__cursor_frame_%d", clip.ID, frame),
			AssetID:    assetID,
			StartMS:    frameStartMS,
			DurationMS: frameDurationMS,
			TrimInMS:   0,
			TrimOutMS:  frameDurationMS,
			ZIndex:     zIndex,
			Muted:      true,
			Transform: map[string]any{
				"x":        x,
				"y":        y,
				"scale":    parent.scale,
				"rotation": parent.rotation,
				"opacity":  parent.opacity,
			},
			Effects:         []TimelineEffect{},
			Transitions:     []TimelineTransition{},
			Keyframes:       []TimelineKeyframe{},
			AnimationBlocks: []TimelineAnimationBlock{},
			Metadata: map[string]any{
				cursorRasterMetadataKey:      cursorRasterContractVersion,
				cursorRasterScaleMetadataKey: frameSpec.Scale,
				cursorRasterHighlightKey:     frameSpec.Highlight,
				cursorRasterClickRingKey:     frameSpec.ClickRing,
			},
		}
		markFidelityGeneratedClip(&generated, rendererFidelityKindCursorPointer, clip.ID)
		overlays = append(overlays, generated)
	}
	return overlays, true
}

type cursorParentTransform struct {
	x, y, scale, rotation, opacity float64
}

func canonicalCursorParentTransform(values map[string]any) (cursorParentTransform, bool) {
	tr := parseClipTransform(values)
	if math.Abs(tr.scaleX-tr.scaleY) > 1e-9 {
		return cursorParentTransform{}, false
	}
	for key, raw := range values {
		switch key {
		case "x", "y", "scale", "scale_x", "scale_y", "rotation", "rotation_z", "opacity", "crop":
			// Supported static 2D fields. Media crop is a child-paint concern in
			// preview and intentionally does not clip the cursor overlay.
		case "z", "rotation_x", "rotation_y", "anchor_x", "anchor_y", "perspective":
			value, numeric := numericTransform(map[string]any{key: raw}, key)
			if !numeric || math.Abs(value) > 1e-9 {
				return cursorParentTransform{}, false
			}
		default:
			return cursorParentTransform{}, false
		}
	}
	return cursorParentTransform{x: tr.x, y: tr.y, scale: tr.scaleX, rotation: tr.rotation, opacity: tr.opacity}, true
}

func hasVisualTransformKeyframes(keyframes []TimelineKeyframe) bool {
	for _, keyframe := range keyframes {
		switch normalizeTimelineToken(keyframe.Property) {
		case "x", "y", "z", "scale", "scale_x", "scale_y", "rotation", "rotation_x", "rotation_y", "rotation_z", "opacity":
			return true
		}
	}
	return false
}

func hasOverlappingSibling(clip TimelineClip, siblings []TimelineClip) bool {
	start, end := clip.StartMS, clip.StartMS+clip.DurationMS
	for _, sibling := range siblings {
		if sibling.ID == clip.ID || sibling.DurationMS <= 0 || sibling.AudioOnly {
			continue
		}
		if sibling.StartMS < end && sibling.StartMS+sibling.DurationMS > start {
			return true
		}
	}
	return false
}

func hasOverlappingSceneCamera(clip TimelineClip, scenes []TimelineScene) bool {
	start, end := clip.StartMS, clip.StartMS+clip.DurationMS
	for _, scene := range scenes {
		if scene.Camera == nil || scene.DurationMS <= 0 {
			continue
		}
		if scene.StartMS < end && scene.StartMS+scene.DurationMS > start {
			return true
		}
	}
	return false
}

func canonicalCursorStateInput(cursor *TimelineCursor) *rendercontract.TimelineV2Cursor {
	if cursor == nil {
		return nil
	}
	scale := cursor.Scale
	if scale <= 0 {
		scale = 1
	}
	var visible *bool
	if cursor.Visible {
		value := true
		visible = &value
	}
	events := make([]rendercontract.TimelineV2CursorEvent, len(cursor.Events))
	for index, event := range cursor.Events {
		events[index] = rendercontract.TimelineV2CursorEvent{TimeMS: event.TimeMS, X: event.X, Y: event.Y, Click: event.Click}
	}
	return &rendercontract.TimelineV2Cursor{
		Visible:    visible,
		Scale:      &scale,
		Highlight:  cursor.Highlight,
		ClickRings: cursor.ClickRings,
		Smoothing:  cursor.Smoothing,
		Events:     events,
	}
}

// exactCursorFrameWindow returns an integer-millisecond interval that contains
// one output-frame presentation time and excludes the next. This is possible
// for the supported <1000 fps domain and prevents FFmpeg's inclusive `between`
// end from double-painting the following frame.
func exactCursorFrameWindow(frame int64, fps int) (int64, int64, bool) {
	if frame < 0 || fps <= 0 || fps > cursorRasterMaxExactFPS {
		return 0, 0, false
	}
	startMS := (frame * 1000) / int64(fps)
	nextNumerator := (frame + 1) * 1000
	nextCeilMS := (nextNumerator + int64(fps) - 1) / int64(fps)
	endInclusiveMS := nextCeilMS - 1
	if endInclusiveMS <= startMS {
		return 0, 0, false
	}
	return startMS, endInclusiveMS - startMS, true
}

func transformCursorOrigin(x, y float64, width, height int, parent cursorParentTransform) (float64, float64) {
	dx := (x - float64(width)/2) * parent.scale
	dy := (y - float64(height)/2) * parent.scale
	radians := parent.rotation * math.Pi / 180
	rotatedX := dx*math.Cos(radians) - dy*math.Sin(radians)
	rotatedY := dx*math.Sin(radians) + dy*math.Cos(radians)
	return parent.x + rotatedX, parent.y + rotatedY
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func normalizeTimelineToken(value string) string {
	return stringLowerTrim(value)
}

func stringLowerTrim(value string) string {
	// Kept local to avoid coupling cursor rasterization to editor-facing token
	// normalization. ASCII timeline property names are the only inputs here.
	out := make([]byte, 0, len(value))
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t' || value[start] == '\n' || value[start] == '\r') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t' || value[end-1] == '\n' || value[end-1] == '\r') {
		end--
	}
	for index := start; index < end; index++ {
		ch := value[index]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		out = append(out, ch)
	}
	return string(out)
}

func cursorRasterAssetID(width, height int, spec cursorRasterSpec) string {
	return fmt.Sprintf("%s%dx%d_s%04d_h%d_r%d", cursorRasterAssetPrefix, width, height, int(math.Round(spec.Scale*1000)), boolInt(spec.Highlight), boolInt(spec.ClickRing))
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func cursorRasterFitsCanvas(width, height int, spec cursorRasterSpec) bool {
	if width < 2 || height < 2 || spec.Scale <= 0 {
		return false
	}
	size := cursorPointerBaseSize * spec.Scale
	negativeExtent := 0.0
	positiveX := 50 * spec.Scale
	positiveY := 58 * spec.Scale
	if spec.Highlight {
		radius := cursorHighlightRadiusFactor * size
		negativeExtent = math.Max(negativeExtent, radius)
		positiveX = math.Max(positiveX, radius)
		positiveY = math.Max(positiveY, radius)
	}
	if spec.ClickRing {
		radius := cursorClickRingRadiusFactor * size
		negativeExtent = math.Max(negativeExtent, radius)
		positiveX = math.Max(positiveX, radius)
		positiveY = math.Max(positiveY, radius)
	}
	cx, cy := float64(width)/2, float64(height)/2
	return negativeExtent+2 <= cx && negativeExtent+2 <= cy && positiveX+2 <= float64(width)-cx && positiveY+2 <= float64(height)-cy
}

// materializeCanonicalCursorRasterAssets writes only the generated cursor
// sprites referenced by the expanded render request. The assets are trusted
// renderer output, live in an isolated temporary directory, and are appended to
// a cloned asset map so the immutable snapshot map supplied by the caller is not
// mutated.
func materializeCanonicalCursorRasterAssets(req *RenderRequest) (func(), error) {
	if req == nil {
		return func() {}, nil
	}
	specs := map[string]cursorRasterSpec{}
	for _, track := range req.Timeline.Tracks {
		for _, clip := range track.Clips {
			if clip.AssetID == "" || clip.Metadata == nil || clip.Metadata[cursorRasterMetadataKey] != cursorRasterContractVersion {
				continue
			}
			scale, ok := numericTransform(clip.Metadata, cursorRasterScaleMetadataKey)
			if !ok || scale <= 0 {
				return func() {}, fmt.Errorf("canonical cursor raster clip %q has invalid scale metadata", clip.ID)
			}
			highlight, _ := clip.Metadata[cursorRasterHighlightKey].(bool)
			clickRing, _ := clip.Metadata[cursorRasterClickRingKey].(bool)
			specs[clip.AssetID] = cursorRasterSpec{Scale: scale, Highlight: highlight, ClickRing: clickRing}
		}
	}
	if len(specs) == 0 {
		return func() {}, nil
	}
	directory, err := os.MkdirTemp("", "omnillm-cursor-raster-*")
	if err != nil {
		return func() {}, fmt.Errorf("create canonical cursor raster directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(directory) }

	assets := make(map[string]models.VideoAsset, len(req.Assets)+len(specs))
	for id, asset := range req.Assets {
		assets[id] = asset
	}
	ids := make([]string, 0, len(specs))
	for id := range specs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := filepath.Join(directory, id+".png")
		if err := writeCursorRasterPNG(path, req.Timeline.Canvas.Width, req.Timeline.Canvas.Height, specs[id]); err != nil {
			cleanup()
			return func() {}, err
		}
		assets[id] = models.VideoAsset{
			ID:         id,
			SourceType: "renderer-generated",
			Kind:       "image",
			FileName:   id + ".png",
			FilePath:   path,
			MimeType:   "image/png",
		}
	}
	req.Assets = assets
	return cleanup, nil
}

func writeCursorRasterPNG(path string, width, height int, spec cursorRasterSpec) error {
	if !cursorRasterFitsCanvas(width, height, spec) {
		return fmt.Errorf("canonical cursor raster %.3fx does not fit %dx%d canvas", spec.Scale, width, height)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	cx, cy := float64(width)/2, float64(height)/2
	size := cursorPointerBaseSize * spec.Scale
	extent := math.Max(60*spec.Scale, 2)
	if spec.Highlight {
		extent = math.Max(extent, cursorHighlightRadiusFactor*size+2)
	}
	if spec.ClickRing {
		extent = math.Max(extent, cursorClickRingRadiusFactor*size+2)
	}
	minX := maxInt(0, int(math.Floor(cx-extent)))
	maxX := minInt(width-1, int(math.Ceil(cx+extent)))
	minY := maxInt(0, int(math.Floor(cy-extent)))
	maxY := minInt(height-1, int(math.Ceil(cy+extent)))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			pixel := supersampledCursorPixel(float64(x), float64(y), cx, cy, spec)
			if pixel.A != 0 {
				img.SetNRGBA(x, y, pixel)
			}
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create canonical cursor raster %q: %w", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode canonical cursor raster %q: %w", path, err)
	}
	return nil
}

type premultipliedPixel struct {
	r, g, b, a float64
}

func supersampledCursorPixel(x, y, cx, cy float64, spec cursorRasterSpec) color.NRGBA {
	var sum premultipliedPixel
	const samples = cursorRasterSupersample * cursorRasterSupersample
	for sy := 0; sy < cursorRasterSupersample; sy++ {
		for sx := 0; sx < cursorRasterSupersample; sx++ {
			px := x + (float64(sx)+0.5)/cursorRasterSupersample - cx
			py := y + (float64(sy)+0.5)/cursorRasterSupersample - cy
			p := cursorSample(px, py, spec)
			sum.r += p.r
			sum.g += p.g
			sum.b += p.b
			sum.a += p.a
		}
	}
	sum.r /= samples
	sum.g /= samples
	sum.b /= samples
	sum.a /= samples
	if sum.a <= 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(math.Round(clampFloat(sum.r/sum.a, 0, 1) * 255)),
		G: uint8(math.Round(clampFloat(sum.g/sum.a, 0, 1) * 255)),
		B: uint8(math.Round(clampFloat(sum.b/sum.a, 0, 1) * 255)),
		A: uint8(math.Round(clampFloat(sum.a, 0, 1) * 255)),
	}
}

func cursorSample(x, y float64, spec cursorRasterSpec) premultipliedPixel {
	var out premultipliedPixel
	size := cursorPointerBaseSize * spec.Scale
	if spec.Highlight && math.Hypot(x, y) <= cursorHighlightRadiusFactor*size {
		// Pinned sRGB cursor palette; keep in sync with CanonicalPreviewCursor.
		out = overPixel(out, 1, 223.0/255, 32.0/255, 0.30)
	}
	if spec.ClickRing {
		radius := cursorClickRingRadiusFactor * size
		distance := math.Hypot(x, y)
		if distance <= radius && distance >= radius-cursorClickRingBorderPixels {
			out = overPixel(out, 0, 188.0/255, 1, 0.80)
		}
	}
	points := cursorPointerPolygon(spec.Scale)
	inside := pointInPolygon(x, y, points)
	stroke := distanceToPolygon(x, y, points) <= 2*spec.Scale
	if inside {
		out = overPixel(out, 1, 1, 1, 1)
	}
	if stroke {
		out = overPixel(out, 17.0/255, 24.0/255, 39.0/255, 1)
	}
	return out
}

func cursorPointerPolygon(scale float64) [][2]float64 {
	unit := 4 * scale
	return [][2]float64{{2 * unit, 1 * unit}, {2 * unit, 12 * unit}, {5.5 * unit, 9.5 * unit}, {7.5 * unit, 14 * unit}, {9.5 * unit, 13 * unit}, {7.5 * unit, 8.8 * unit}, {12 * unit, 8.5 * unit}}
}

func pointInPolygon(x, y float64, points [][2]float64) bool {
	inside := false
	for i, j := 0, len(points)-1; i < len(points); j, i = i, i+1 {
		xi, yi := points[i][0], points[i][1]
		xj, yj := points[j][0], points[j][1]
		intersects := (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi
		if intersects {
			inside = !inside
		}
	}
	return inside
}

func distanceToPolygon(x, y float64, points [][2]float64) float64 {
	minimum := math.Inf(1)
	for index := range points {
		next := (index + 1) % len(points)
		distance := distanceToSegment(x, y, points[index][0], points[index][1], points[next][0], points[next][1])
		if distance < minimum {
			minimum = distance
		}
	}
	return minimum
}

func distanceToSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = clampFloat(t, 0, 1)
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

func overPixel(dst premultipliedPixel, r, g, b, a float64) premultipliedPixel {
	oneMinus := 1 - a
	return premultipliedPixel{
		r: r*a + dst.r*oneMinus,
		g: g*a + dst.g*oneMinus,
		b: b*a + dst.b*oneMinus,
		a: a + dst.a*oneMinus,
	}
}
