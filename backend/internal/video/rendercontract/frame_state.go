package rendercontract

import (
	"fmt"
	"math"
	"sort"
)

const VisualFrameStateContractV1 = "visual-frame-state-v1"

// Matrix4 is a row-major 4x4 matrix multiplied by column vectors. Layer local
// coordinates are centered on the content box. Model and perspective matrices
// remain separate so consumers can preserve the authored camera-relative model
// transform while applying canonical projection explicitly.
type Matrix4 [16]float64

type EvaluatedCamera struct {
	X                   float64 `json:"x"`
	Y                   float64 `json:"y"`
	Z                   float64 `json:"z"`
	RotationX           float64 `json:"rotation_x"`
	RotationY           float64 `json:"rotation_y"`
	RotationZ           float64 `json:"rotation_z"`
	FieldOfView         float64 `json:"field_of_view"`
	FocusDepth          float64 `json:"focus_depth"`
	PerspectiveDistance float64 `json:"perspective_distance"`
}

type EvaluatedTransform struct {
	X           float64         `json:"x"`
	Y           float64         `json:"y"`
	Z           float64         `json:"z"`
	ScaleX      float64         `json:"scale_x"`
	ScaleY      float64         `json:"scale_y"`
	RotationX   float64         `json:"rotation_x"`
	RotationY   float64         `json:"rotation_y"`
	RotationZ   float64         `json:"rotation_z"`
	Opacity     float64         `json:"opacity"`
	AnchorX     float64         `json:"anchor_x"`
	AnchorY     float64         `json:"anchor_y"`
	Perspective *float64        `json:"perspective,omitempty"`
	Crop        *TimelineV2Crop `json:"crop,omitempty"`
}

type FrameLayerState struct {
	TrackIndex            int                            `json:"track_index"`
	ClipIndex             int                            `json:"clip_index"`
	TrackID               string                         `json:"track_id"`
	ClipID                string                         `json:"clip_id"`
	ZIndex                int                            `json:"z_index"`
	StartFrame            int64                          `json:"start_frame"`
	EndFrame              int64                          `json:"end_frame"`
	SourceTimeMS          float64                        `json:"source_time_ms"`
	MediaFit              string                         `json:"media_fit,omitempty"`
	ContentBounds         *TimelineV2ContentBounds       `json:"content_bounds,omitempty"`
	MediaGeometry         *EvaluatedMediaGeometry        `json:"media_geometry,omitempty"`
	Transform             EvaluatedTransform             `json:"transform"`
	ViewTransform         EvaluatedTransform             `json:"view_transform"`
	ModelMatrix           Matrix4                        `json:"model_matrix"`
	PerspectiveProjection EvaluatedPerspectiveProjection `json:"perspective_projection"`
	Transitions           []EvaluatedTransitionState     `json:"transitions,omitempty"`
	Unresolved            []string                       `json:"unresolved"`
	Authoritative         bool                           `json:"authoritative"`
}

type VisualFrameState struct {
	ContractVersion string            `json:"contract_version"`
	FrameIndex      int64             `json:"frame_index"`
	FrameTime       RationalTime      `json:"frame_time"`
	Canvas          TimelineV2Canvas  `json:"canvas"`
	ActiveSceneID   string            `json:"active_scene_id,omitempty"`
	Camera          EvaluatedCamera   `json:"camera"`
	Layers          []FrameLayerState `json:"layers"`
	Unresolved      []string          `json:"unresolved"`
	Authoritative   bool              `json:"authoritative"`
}

// EvaluateVisualFrameState produces the renderer-independent visual FrameState
// projection. It evaluates exact-frame clip/camera properties, visibility/order,
// source time, canonical media geometry, perspective projection, and transition
// timing/peer state. Paint families not yet canonicalized are explicitly listed
// in Unresolved and make only the affected frame state non-authoritative.
func EvaluateVisualFrameState(doc TimelineV2Document, frameIndex int64) (VisualFrameState, error) {
	normalized, err := NormalizeTimelineV2EvaluationInputs(doc)
	if err != nil {
		return VisualFrameState{}, err
	}
	fps := normalized.Canvas.FPS
	if frameIndex < 0 || frameIndex >= FrameCount(normalized.DurationMS, fps) {
		return VisualFrameState{}, fmt.Errorf("frame index %d is outside timeline frame range", frameIndex)
	}

	scene := sceneAtFramePresentation(normalized.Scenes, frameIndex, fps)
	camera, err := evaluateFrameCamera(scene, frameIndex, fps, normalized.Canvas.Height)
	if err != nil {
		return VisualFrameState{}, err
	}
	state := VisualFrameState{
		ContractVersion: VisualFrameStateContractV1,
		FrameIndex:      frameIndex,
		FrameTime:       FrameTime(frameIndex, fps),
		Canvas:          normalized.Canvas,
		Camera:          camera,
		Layers:          []FrameLayerState{},
		Unresolved:      []string{},
	}
	if scene != nil {
		state.ActiveSceneID = scene.ID
		if len(scene.Effects) > 0 {
			state.Unresolved = append(state.Unresolved, "scene_effects")
		}
	}

	for _, active := range ActiveClipsAtFrame(normalized, frameIndex) {
		track := normalized.Tracks[active.TrackIndex]
		clip := track.Clips[active.ClipIndex]
		if !track.Visible || clip.AudioOnly || track.Type == "audio" || track.Type == "music" {
			continue
		}
		transitions, err := evaluateClipTransitionsAtFrameNormalized(normalized, active.TrackIndex, active.ClipIndex, frameIndex)
		if err != nil {
			return VisualFrameState{}, err
		}
		layer, err := evaluateFrameLayer(normalized.Canvas, track, clip, active, camera, transitions, frameIndex)
		if err != nil {
			return VisualFrameState{}, err
		}
		state.Layers = append(state.Layers, layer)
		for _, unresolved := range layer.Unresolved {
			state.Unresolved = append(state.Unresolved, clip.ID+":"+unresolved)
		}
	}
	state.Unresolved = uniqueStrings(state.Unresolved)
	state.Authoritative = len(state.Unresolved) == 0
	return state, nil
}

func evaluateFrameCamera(scene *TimelineV2Scene, frameIndex int64, fps, canvasHeight int) (EvaluatedCamera, error) {
	camera := (*TimelineV2Camera)(nil)
	sceneStartMS := int64(0)
	if scene != nil {
		camera = scene.Camera
		sceneStartMS = scene.StartMS
	}
	properties := []string{"x", "y", "z", "scale_x", "scale_y", "rotation_x", "rotation_y", "rotation_z", "opacity"}
	_ = properties
	cameraProperties := []string{"x", "y", "z", "rotation_x", "rotation_y", "rotation_z", "field_of_view", "focus_depth"}
	values := make(map[string]float64, len(cameraProperties))
	for _, property := range cameraProperties {
		value, err := EvaluateCameraPropertyAtFrame(camera, property, frameIndex, fps, sceneStartMS)
		if err != nil {
			return EvaluatedCamera{}, err
		}
		values[property] = value
	}
	fieldOfView := clampFloat64(values["field_of_view"], 1, 179)
	perspectiveDistance := 1200.0
	if camera != nil {
		perspectiveDistance = float64(maxIntValue(canvasHeight, 1)) / (2 * math.Tan(fieldOfView*math.Pi/360))
	}
	return EvaluatedCamera{
		X: values["x"], Y: values["y"], Z: values["z"],
		RotationX: values["rotation_x"], RotationY: values["rotation_y"], RotationZ: values["rotation_z"],
		FieldOfView: fieldOfView, FocusDepth: values["focus_depth"], PerspectiveDistance: perspectiveDistance,
	}, nil
}

func evaluateFrameLayer(canvas TimelineV2Canvas, track TimelineV2Track, clip TimelineV2Clip, active ActiveClip, camera EvaluatedCamera, transitions []EvaluatedTransitionState, frameIndex int64) (FrameLayerState, error) {
	properties := []string{"x", "y", "z", "scale_x", "scale_y", "rotation_x", "rotation_y", "rotation_z", "opacity"}
	values := make(map[string]float64, len(properties))
	for _, property := range properties {
		value, err := EvaluateClipPropertyAtFrame(clip, property, frameIndex, canvas.FPS)
		if err != nil {
			return FrameLayerState{}, err
		}
		values[property] = value
	}
	transform := EvaluatedTransform{
		X: values["x"], Y: values["y"], Z: values["z"],
		ScaleX: values["scale_x"], ScaleY: values["scale_y"],
		RotationX: values["rotation_x"], RotationY: values["rotation_y"], RotationZ: values["rotation_z"],
		Opacity: values["opacity"] * fadeFactorAtFrame(clip, frameIndex, canvas.FPS),
		Crop:    cloneCrop(clip.Transform),
	}
	if clip.Transform != nil {
		if clip.Transform.AnchorX != nil {
			transform.AnchorX = *clip.Transform.AnchorX
		}
		if clip.Transform.AnchorY != nil {
			transform.AnchorY = *clip.Transform.AnchorY
		}
		if clip.Transform.Perspective != nil {
			value := *clip.Transform.Perspective
			transform.Perspective = &value
		}
	}
	view := transform
	view.X -= camera.X
	view.Y -= camera.Y
	view.Z -= camera.Z
	view.RotationX -= camera.RotationX
	view.RotationY -= camera.RotationY
	view.RotationZ -= camera.RotationZ

	bounds := effectiveContentBounds(clip)
	unresolved := unresolvedLayerFeatures(clip, bounds)
	for _, transition := range transitions {
		if transition.Active {
			unresolved = append(unresolved, "transition_paint:"+transition.ID)
		}
	}
	var mediaGeometry *EvaluatedMediaGeometry
	if clip.AssetID != "" && clip.ContentBounds != nil {
		geometry, err := EvaluateMediaGeometry(canvas, clip)
		if err != nil {
			return FrameLayerState{}, err
		}
		mediaGeometry = &geometry
	}
	anchorOffsetX, anchorOffsetY := 0.0, 0.0
	if mediaGeometry != nil {
		anchorOffsetX = transform.AnchorX * mediaGeometry.PaintedBounds.Width / float64(maxIntValue(canvas.Width, 1))
		anchorOffsetY = transform.AnchorY * mediaGeometry.PaintedBounds.Height / float64(maxIntValue(canvas.Height, 1))
	} else if bounds != nil {
		anchorOffsetX = transform.AnchorX * bounds.Width / float64(maxIntValue(canvas.Width, 1))
		anchorOffsetY = transform.AnchorY * bounds.Height / float64(maxIntValue(canvas.Height, 1))
	} else if transform.AnchorX != 0 || transform.AnchorY != 0 {
		unresolved = append(unresolved, "content_bounds_for_anchor")
	}
	matrix := composeModelMatrix(view, anchorOffsetX, anchorOffsetY)
	projection, err := EvaluatePerspectiveProjection(camera, view)
	if err != nil {
		return FrameLayerState{}, err
	}
	unresolved = uniqueStrings(unresolved)
	return FrameLayerState{
		TrackIndex: active.TrackIndex, ClipIndex: active.ClipIndex, TrackID: track.ID, ClipID: clip.ID,
		ZIndex: active.ZIndex, StartFrame: active.StartFrame, EndFrame: active.EndFrame, SourceTimeMS: active.SourceTimeMS,
		MediaFit: clip.MediaFit, ContentBounds: bounds, MediaGeometry: mediaGeometry, Transform: transform, ViewTransform: view,
		ModelMatrix: matrix, PerspectiveProjection: projection, Transitions: transitions, Unresolved: unresolved, Authoritative: len(unresolved) == 0,
	}, nil
}

func sceneAtFramePresentation(scenes []TimelineV2Scene, frameIndex int64, fps int) *TimelineV2Scene {
	presentation := frameIndex * 1000
	for i := range scenes {
		start := scenes[i].StartMS * int64(fps)
		end := (scenes[i].StartMS + scenes[i].DurationMS) * int64(fps)
		if start <= presentation && presentation < end {
			return &scenes[i]
		}
	}
	return nil
}

func effectiveContentBounds(clip TimelineV2Clip) *TimelineV2ContentBounds {
	if clip.ContentBounds != nil {
		bounds := *clip.ContentBounds
		return &bounds
	}
	if clip.Shape != nil {
		width, height := 320.0, 180.0
		if clip.Shape.Width != nil && *clip.Shape.Width > 0 {
			width = float64(*clip.Shape.Width)
		}
		if clip.Shape.Height != nil && *clip.Shape.Height > 0 {
			height = float64(*clip.Shape.Height)
		}
		return &TimelineV2ContentBounds{Width: width, Height: height}
	}
	if clip.Text != nil && clip.Text.BoxWidth != nil && clip.Text.BoxHeight != nil && *clip.Text.BoxWidth > 0 && *clip.Text.BoxHeight > 0 {
		return &TimelineV2ContentBounds{Width: *clip.Text.BoxWidth, Height: *clip.Text.BoxHeight}
	}
	return nil
}

func unresolvedLayerFeatures(clip TimelineV2Clip, bounds *TimelineV2ContentBounds) []string {
	unresolved := []string{}
	if len(clip.Effects) > 0 {
		unresolved = append(unresolved, "effects")
	}
	if clip.Text != nil {
		unresolved = append(unresolved, "text")
	}
	if clip.Shape != nil {
		unresolved = append(unresolved, "shape")
	}
	if clip.Cursor != nil {
		unresolved = append(unresolved, "cursor")
	}
	if clip.AssetID != "" && bounds == nil {
		unresolved = append(unresolved, "media_geometry:content_bounds")
	}
	return unresolved
}

func fadeFactorAtFrame(clip TimelineV2Clip, frameIndex int64, fps int) float64 {
	time := FrameRelativeMilliseconds(frameIndex, fps, clip.StartMS)
	elapsed := float64(time.Numerator) / float64(time.Denominator)
	remaining := float64(clip.DurationMS) - elapsed
	factor := 1.0
	if clip.FadeInMS > 0 {
		factor = math.Min(factor, elapsed/float64(clip.FadeInMS))
	}
	if clip.FadeOutMS > 0 {
		factor = math.Min(factor, remaining/float64(clip.FadeOutMS))
	}
	return clampFloat64(factor, 0, 1)
}

func cloneCrop(transform *TimelineV2Transform) *TimelineV2Crop {
	if transform == nil || transform.Crop == nil {
		return nil
	}
	crop := *transform.Crop
	return &crop
}

func composeModelMatrix(transform EvaluatedTransform, anchorX, anchorY float64) Matrix4 {
	matrix := translationMatrix(transform.X, transform.Y, transform.Z)
	matrix = multiplyMatrix(matrix, translationMatrix(anchorX, anchorY, 0))
	matrix = multiplyMatrix(matrix, rotationXMatrix(transform.RotationX))
	matrix = multiplyMatrix(matrix, rotationYMatrix(transform.RotationY))
	matrix = multiplyMatrix(matrix, rotationZMatrix(transform.RotationZ))
	matrix = multiplyMatrix(matrix, scaleMatrix(transform.ScaleX, transform.ScaleY, 1))
	matrix = multiplyMatrix(matrix, translationMatrix(-anchorX, -anchorY, 0))
	return matrix
}

func identityMatrix() Matrix4 {
	return Matrix4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func multiplyMatrix(left, right Matrix4) Matrix4 {
	var out Matrix4
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			for k := 0; k < 4; k++ {
				out[row*4+col] += left[row*4+k] * right[k*4+col]
			}
		}
	}
	return out
}

func translationMatrix(x, y, z float64) Matrix4 {
	matrix := identityMatrix()
	matrix[3], matrix[7], matrix[11] = x, y, z
	return matrix
}

func scaleMatrix(x, y, z float64) Matrix4 {
	matrix := identityMatrix()
	matrix[0], matrix[5], matrix[10] = x, y, z
	return matrix
}

func rotationXMatrix(degrees float64) Matrix4 {
	radians := degrees * math.Pi / 180
	c, s := math.Cos(radians), math.Sin(radians)
	return Matrix4{1, 0, 0, 0, 0, c, -s, 0, 0, s, c, 0, 0, 0, 0, 1}
}

func rotationYMatrix(degrees float64) Matrix4 {
	radians := degrees * math.Pi / 180
	c, s := math.Cos(radians), math.Sin(radians)
	return Matrix4{c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1}
}

func rotationZMatrix(degrees float64) Matrix4 {
	radians := degrees * math.Pi / 180
	c, s := math.Cos(radians), math.Sin(radians)
	return Matrix4{c, -s, 0, 0, s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func clampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxIntValue(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}
