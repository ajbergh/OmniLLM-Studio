package video

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

const (
	shapeRasterContractVersion = "shape-raster-v1"
	shapeRasterMetadataKey     = "_omnillm_shape_raster_version"
	shapeRasterWidthKey        = "_omnillm_shape_raster_width"
	shapeRasterHeightKey       = "_omnillm_shape_raster_height"
	shapeRasterFillKey         = "_omnillm_shape_raster_fill"
	shapeRasterStrokeKey       = "_omnillm_shape_raster_stroke"
	shapeRasterStrokeWidthKey  = "_omnillm_shape_raster_stroke_width"
	shapeRasterCornerRadiusKey = "_omnillm_shape_raster_corner_radius"
	shapeRasterAssetPrefix     = "__omnillm_shape_raster_v1_"
	shapeRasterSupersample     = 4
)

type roundedRectangleRasterSpec struct {
	Width        float64
	Height       float64
	Fill         string
	Stroke       string
	StrokeWidth  float64
	CornerRadius float64
}

// canonicalRoundedRectangleRasterClip replaces only the proven static-2D
// rounded-rectangle subset with a transparent full-canvas image. Existing
// media transform/compositing then owns x/y, uniform scale, Z rotation,
// opacity, timing, track visibility, z-index, and track ordering.
func canonicalRoundedRectangleRasterClip(clip TimelineClip, canvas TimelineCanvas, scenes []TimelineScene) (TimelineClip, bool) {
	if clip.Shape == nil || normalizeTimelineToken(clip.Shape.Kind) != ShapeKindRoundedRectangle {
		return clip, false
	}
	if clip.AssetID != "" || clip.Text != nil || clip.Cursor != nil || clip.AudioOnly || clip.DurationMS <= 0 {
		return clip, false
	}
	if clip.FadeInMS > 0 || clip.FadeOutMS > 0 || len(clip.Transitions) > 0 || len(clip.AnimationBlocks) > 0 {
		return clip, false
	}
	for _, effect := range clip.Effects {
		if effect.Enabled {
			return clip, false
		}
	}
	if len(clip.Keyframes) > 0 || hasOverlappingSceneCamera(clip, scenes) {
		return clip, false
	}
	parent, ok := canonicalCursorParentTransform(clip.Transform)
	if !ok || parent.scale <= 0 || parent.opacity < 0 || parent.opacity > 1 {
		return clip, false
	}
	if tr := parseClipTransform(clip.Transform); tr.hasCrop {
		return clip, false
	}

	state, ok := evaluatedRoundedRectangleState(clip.Shape)
	if !ok || state.Width > float64(canvas.Width) || state.Height > float64(canvas.Height) {
		return clip, false
	}
	if _, ok := parseShapeRasterColor(state.Fill); !ok {
		return clip, false
	}
	if _, ok := parseShapeRasterColor(state.Stroke); !ok && strings.TrimSpace(state.Stroke) != "" {
		return clip, false
	}

	spec := roundedRectangleRasterSpec{
		Width:        state.Width,
		Height:       state.Height,
		Fill:         state.Fill,
		Stroke:       state.Stroke,
		StrokeWidth:  state.StrokeWidth,
		CornerRadius: state.CornerRadius,
	}
	generated := cloneTimelineClip(clip)
	generated.AssetID = roundedRectangleRasterAssetID(canvas.Width, canvas.Height, spec)
	generated.Shape = nil
	generated.Muted = true
	if generated.Metadata == nil {
		generated.Metadata = map[string]any{}
	}
	generated.Metadata[shapeRasterMetadataKey] = shapeRasterContractVersion
	generated.Metadata[shapeRasterWidthKey] = spec.Width
	generated.Metadata[shapeRasterHeightKey] = spec.Height
	generated.Metadata[shapeRasterFillKey] = spec.Fill
	generated.Metadata[shapeRasterStrokeKey] = spec.Stroke
	generated.Metadata[shapeRasterStrokeWidthKey] = spec.StrokeWidth
	generated.Metadata[shapeRasterCornerRadiusKey] = spec.CornerRadius
	markFidelityGeneratedClip(&generated, rendererFidelityKindShapeRaster, clip.ID)
	return generated, true
}

func evaluatedRoundedRectangleState(shape *TimelineShape) (*rendercontract.EvaluatedShapeState, bool) {
	if shape == nil {
		return nil, false
	}
	input := &rendercontract.TimelineV2Shape{Kind: shape.Kind, Fill: shape.Fill, Stroke: shape.Stroke}
	if shape.Width > 0 {
		value := shape.Width
		input.Width = &value
	}
	if shape.Height > 0 {
		value := shape.Height
		input.Height = &value
	}
	if shape.StrokeWidth > 0 {
		value := shape.StrokeWidth
		input.StrokeWidth = &value
	}
	if shape.BlurRadius > 0 {
		value := shape.BlurRadius
		input.BlurRadius = &value
	}
	if shape.CornerRadius > 0 {
		value := shape.CornerRadius
		input.CornerRadius = &value
	}
	state, err := rendercontract.EvaluateShapeState(input)
	if err != nil || state == nil || state.Kind != ShapeKindRoundedRectangle {
		return nil, false
	}
	return state, true
}

func roundedRectangleRasterAssetID(canvasWidth, canvasHeight int, spec roundedRectangleRasterSpec) string {
	payload := fmt.Sprintf("%dx%d|%.6f|%.6f|%s|%s|%.6f|%.6f", canvasWidth, canvasHeight, spec.Width, spec.Height, strings.TrimSpace(spec.Fill), strings.TrimSpace(spec.Stroke), spec.StrokeWidth, spec.CornerRadius)
	sum := sha256.Sum256([]byte(payload))
	return shapeRasterAssetPrefix + hex.EncodeToString(sum[:12])
}

func materializeCanonicalShapeRasterAssets(req *RenderRequest) (func(), error) {
	if req == nil {
		return func() {}, nil
	}
	specs := map[string]roundedRectangleRasterSpec{}
	for _, track := range req.Timeline.Tracks {
		for _, clip := range track.Clips {
			if clip.AssetID == "" || clip.Metadata == nil || clip.Metadata[shapeRasterMetadataKey] != shapeRasterContractVersion {
				continue
			}
			width, widthOK := numericTransform(clip.Metadata, shapeRasterWidthKey)
			height, heightOK := numericTransform(clip.Metadata, shapeRasterHeightKey)
			strokeWidth, strokeOK := numericTransform(clip.Metadata, shapeRasterStrokeWidthKey)
			cornerRadius, cornerOK := numericTransform(clip.Metadata, shapeRasterCornerRadiusKey)
			fill, fillOK := clip.Metadata[shapeRasterFillKey].(string)
			stroke, strokeColorOK := clip.Metadata[shapeRasterStrokeKey].(string)
			if !widthOK || !heightOK || !strokeOK || !cornerOK || !fillOK || !strokeColorOK || width <= 0 || height <= 0 || strokeWidth < 0 || cornerRadius < 0 {
				return func() {}, fmt.Errorf("canonical rounded rectangle clip %q has invalid raster metadata", clip.ID)
			}
			specs[clip.AssetID] = roundedRectangleRasterSpec{Width: width, Height: height, Fill: fill, Stroke: stroke, StrokeWidth: strokeWidth, CornerRadius: cornerRadius}
		}
	}
	if len(specs) == 0 {
		return func() {}, nil
	}

	directory, err := os.MkdirTemp("", "omnillm-shape-raster-*")
	if err != nil {
		return func() {}, fmt.Errorf("create canonical shape raster directory: %w", err)
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
		if err := writeRoundedRectangleRasterPNG(path, req.Timeline.Canvas.Width, req.Timeline.Canvas.Height, specs[id]); err != nil {
			cleanup()
			return func() {}, err
		}
		assets[id] = models.VideoAsset{ID: id, SourceType: "renderer-generated", Kind: "image", FileName: id + ".png", FilePath: path, MimeType: "image/png"}
	}
	req.Assets = assets
	return cleanup, nil
}

func writeRoundedRectangleRasterPNG(path string, canvasWidth, canvasHeight int, spec roundedRectangleRasterSpec) error {
	if canvasWidth < 2 || canvasHeight < 2 || spec.Width <= 0 || spec.Height <= 0 || spec.Width > float64(canvasWidth) || spec.Height > float64(canvasHeight) {
		return fmt.Errorf("canonical rounded rectangle %.3fx%.3f does not fit %dx%d canvas", spec.Width, spec.Height, canvasWidth, canvasHeight)
	}
	fill, ok := parseShapeRasterColor(spec.Fill)
	if !ok {
		return fmt.Errorf("unsupported canonical rounded rectangle fill %q", spec.Fill)
	}
	stroke := color.NRGBA{}
	if strings.TrimSpace(spec.Stroke) != "" {
		var strokeOK bool
		stroke, strokeOK = parseShapeRasterColor(spec.Stroke)
		if !strokeOK {
			return fmt.Errorf("unsupported canonical rounded rectangle stroke %q", spec.Stroke)
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	cx, cy := float64(canvasWidth)/2, float64(canvasHeight)/2
	halfW, halfH := spec.Width/2, spec.Height/2
	minX := maxInt(0, int(math.Floor(cx-halfW-2)))
	maxX := minInt(canvasWidth-1, int(math.Ceil(cx+halfW+2)))
	minY := maxInt(0, int(math.Floor(cy-halfH-2)))
	maxY := minInt(canvasHeight-1, int(math.Ceil(cy+halfH+2)))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			pixel := supersampledRoundedRectanglePixel(float64(x), float64(y), cx, cy, spec, fill, stroke)
			if pixel.A != 0 {
				img.SetNRGBA(x, y, pixel)
			}
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create canonical rounded rectangle raster %q: %w", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode canonical rounded rectangle raster %q: %w", path, err)
	}
	return nil
}

func supersampledRoundedRectanglePixel(x, y, cx, cy float64, spec roundedRectangleRasterSpec, fill, stroke color.NRGBA) color.NRGBA {
	var sum premultipliedPixel
	const samples = shapeRasterSupersample * shapeRasterSupersample
	for sy := 0; sy < shapeRasterSupersample; sy++ {
		for sx := 0; sx < shapeRasterSupersample; sx++ {
			px := x + (float64(sx)+0.5)/shapeRasterSupersample - cx
			py := y + (float64(sy)+0.5)/shapeRasterSupersample - cy
			p := roundedRectangleSample(px, py, spec, fill, stroke)
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
	return color.NRGBA{R: uint8(math.Round(clampFloat(sum.r/sum.a, 0, 1) * 255)), G: uint8(math.Round(clampFloat(sum.g/sum.a, 0, 1) * 255)), B: uint8(math.Round(clampFloat(sum.b/sum.a, 0, 1) * 255)), A: uint8(math.Round(clampFloat(sum.a, 0, 1) * 255))}
}

func roundedRectangleSample(x, y float64, spec roundedRectangleRasterSpec, fill, stroke color.NRGBA) premultipliedPixel {
	halfW, halfH := spec.Width/2, spec.Height/2
	radius := math.Min(math.Max(spec.CornerRadius, 0), math.Min(halfW, halfH))
	if !insideRoundedRectangle(x, y, halfW, halfH, radius) {
		return premultipliedPixel{}
	}
	out := overNRGBAPixel(premultipliedPixel{}, fill)
	if stroke.A == 0 || spec.StrokeWidth <= 0 {
		return out
	}
	innerHalfW := halfW - spec.StrokeWidth
	innerHalfH := halfH - spec.StrokeWidth
	innerRadius := math.Max(0, radius-spec.StrokeWidth)
	insideInner := innerHalfW > 0 && innerHalfH > 0 && insideRoundedRectangle(x, y, innerHalfW, innerHalfH, math.Min(innerRadius, math.Min(innerHalfW, innerHalfH)))
	if !insideInner {
		out = overNRGBAPixel(out, stroke)
	}
	return out
}

func insideRoundedRectangle(x, y, halfW, halfH, radius float64) bool {
	ax, ay := math.Abs(x), math.Abs(y)
	if ax > halfW || ay > halfH {
		return false
	}
	if radius <= 0 || ax <= halfW-radius || ay <= halfH-radius {
		return true
	}
	dx := ax - (halfW - radius)
	dy := ay - (halfH - radius)
	return dx*dx+dy*dy <= radius*radius
}

func overNRGBAPixel(dst premultipliedPixel, src color.NRGBA) premultipliedPixel {
	alpha := float64(src.A) / 255
	return overPixel(dst, float64(src.R)/255, float64(src.G)/255, float64(src.B)/255, alpha)
}

func parseShapeRasterColor(value string) (color.NRGBA, bool) {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "transparent" || s == "" {
		return color.NRGBA{}, true
	}
	if strings.HasPrefix(s, "#") {
		hexValue := strings.TrimPrefix(s, "#")
		switch len(hexValue) {
		case 3, 4:
			expanded := make([]byte, 0, len(hexValue)*2)
			for i := range hexValue {
				expanded = append(expanded, hexValue[i], hexValue[i])
			}
			hexValue = string(expanded)
		case 6, 8:
		default:
			return color.NRGBA{}, false
		}
		parsed, err := hex.DecodeString(hexValue)
		if err != nil || (len(parsed) != 3 && len(parsed) != 4) {
			return color.NRGBA{}, false
		}
		result := color.NRGBA{R: parsed[0], G: parsed[1], B: parsed[2], A: 255}
		if len(parsed) == 4 {
			result.A = parsed[3]
		}
		return result, true
	}
	if strings.HasPrefix(s, "rgb(") || strings.HasPrefix(s, "rgba(") {
		open := strings.IndexByte(s, '(')
		if open < 0 || !strings.HasSuffix(s, ")") {
			return color.NRGBA{}, false
		}
		parts := strings.Split(s[open+1:len(s)-1], ",")
		if len(parts) != 3 && len(parts) != 4 {
			return color.NRGBA{}, false
		}
		channels := [3]uint8{}
		for i := 0; i < 3; i++ {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
			if err != nil || parsed < 0 || parsed > 255 {
				return color.NRGBA{}, false
			}
			channels[i] = uint8(math.Round(parsed))
		}
		alpha := uint8(255)
		if len(parts) == 4 {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				return color.NRGBA{}, false
			}
			alpha = uint8(math.Round(parsed * 255))
		}
		return color.NRGBA{R: channels[0], G: channels[1], B: channels[2], A: alpha}, true
	}
	return color.NRGBA{}, false
}
