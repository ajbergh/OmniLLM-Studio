package rendercontract

import (
	"fmt"
	"sort"
	"strings"
)

// PropertyKeyframe is the renderer-independent input shape for sampling one
// numeric timeline property. It intentionally does not encode which property
// names are semantically supported so effect automation can reuse the same
// interpolation primitive while visual/camera evaluators fail closed.
type PropertyKeyframe struct {
	Property string       `json:"property"`
	TimeMS   int64        `json:"time_ms"`
	Value    float64      `json:"value"`
	Easing   string       `json:"easing,omitempty"`
	Curve    *MotionCurve `json:"curve,omitempty"`
}

var canonicalClipProperties = map[string]bool{
	"x": true, "y": true, "z": true,
	"scale": true, "scale_x": true, "scale_y": true,
	"rotation": true, "rotation_x": true, "rotation_y": true, "rotation_z": true,
	"opacity": true, "volume": true,
}

var canonicalCameraProperties = map[string]bool{
	"x": true, "y": true, "z": true,
	"rotation_x": true, "rotation_y": true, "rotation_z": true,
	"field_of_view": true, "focus_depth": true,
}

// SamplePropertyKeyframes samples one property at clip/scene-relative time.
// Property matching is trimmed and case-insensitive. Values hold flat before
// the first and after the last keyframe, and each segment uses the LATER
// keyframe's curve/easing. Sorting is stable so duplicate authored times remain
// deterministic without inventing a secondary semantic ordering rule.
func SamplePropertyKeyframes(keyframes []PropertyKeyframe, property string, timeMS int64) (float64, bool) {
	property = normalizePropertyName(property)
	if property == "" {
		return 0, false
	}
	points := make([]PropertyKeyframe, 0)
	for _, keyframe := range keyframes {
		if normalizePropertyName(keyframe.Property) == property {
			points = append(points, keyframe)
		}
	}
	if len(points) == 0 {
		return 0, false
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].TimeMS < points[j].TimeMS })
	if timeMS <= points[0].TimeMS {
		return points[0].Value, true
	}
	for i := 1; i < len(points); i++ {
		next := points[i]
		if timeMS <= next.TimeMS {
			prev := points[i-1]
			span := next.TimeMS - prev.TimeMS
			progress := 1.0
			if span > 0 {
				progress = float64(timeMS-prev.TimeMS) / float64(span)
			}
			eased := CurveProgress(progress, next.Curve, next.Easing)
			return prev.Value + (next.Value-prev.Value)*eased, true
		}
	}
	return points[len(points)-1].Value, true
}

// EvaluateClipProperty resolves a supported numeric clip property from its
// normalized static/base value plus optional keyframes. Unsupported property
// requests return an error instead of silently manufacturing semantics.
func EvaluateClipProperty(clip TimelineV2Clip, property string, timeMS int64) (float64, error) {
	property = normalizePropertyName(property)
	base, err := ClipPropertyBaseValue(clip, property)
	if err != nil {
		return 0, err
	}
	if value, ok := SamplePropertyKeyframes(timelineV2PropertyKeyframes(clip.Keyframes), property, timeMS); ok {
		return value, nil
	}
	return base, nil
}

// ClipPropertyBaseValue returns the static/default value for a canonical clip
// property. Axis-specific scale falls back to uniform scale; rotation_z falls
// back to the legacy 2D rotation. These are compatibility semantics, not final
// matrix-composition semantics (owned by the later transform evaluator).
func ClipPropertyBaseValue(clip TimelineV2Clip, property string) (float64, error) {
	property = normalizePropertyName(property)
	if !canonicalClipProperties[property] {
		return 0, unsupportedProperty("clip", property)
	}
	transform := clip.Transform
	switch property {
	case "x":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.X }, 0), nil
	case "y":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Y }, 0), nil
	case "z":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Z }, 0), nil
	case "scale":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Scale }, 1), nil
	case "scale_x":
		if transform != nil && transform.ScaleX != nil {
			return *transform.ScaleX, nil
		}
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Scale }, 1), nil
	case "scale_y":
		if transform != nil && transform.ScaleY != nil {
			return *transform.ScaleY, nil
		}
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Scale }, 1), nil
	case "rotation":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Rotation }, 0), nil
	case "rotation_x":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.RotationX }, 0), nil
	case "rotation_y":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.RotationY }, 0), nil
	case "rotation_z":
		if transform != nil && transform.RotationZ != nil {
			return *transform.RotationZ, nil
		}
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Rotation }, 0), nil
	case "opacity":
		return transformValue(transform, func(value *TimelineV2Transform) *float64 { return value.Opacity }, 1), nil
	case "volume":
		if clip.Volume != nil {
			return *clip.Volume, nil
		}
		return 1, nil
	default:
		return 0, unsupportedProperty("clip", property)
	}
}

// EvaluateCameraProperty resolves a supported numeric camera property from its
// static/default value plus optional camera keyframes.
func EvaluateCameraProperty(camera *TimelineV2Camera, property string, timeMS int64) (float64, error) {
	property = normalizePropertyName(property)
	base, err := CameraPropertyBaseValue(camera, property)
	if err != nil {
		return 0, err
	}
	if camera != nil {
		if value, ok := SamplePropertyKeyframes(timelineV2PropertyKeyframes(camera.Keyframes), property, timeMS); ok {
			return value, nil
		}
	}
	return base, nil
}

// CameraPropertyBaseValue returns the v1-preview-compatible static/default
// camera value. field_of_view defaults to 50 degrees; all other supported
// camera properties default to zero.
func CameraPropertyBaseValue(camera *TimelineV2Camera, property string) (float64, error) {
	property = normalizePropertyName(property)
	if !canonicalCameraProperties[property] {
		return 0, unsupportedProperty("camera", property)
	}
	if camera == nil {
		if property == "field_of_view" {
			return 50, nil
		}
		return 0, nil
	}
	var value *float64
	switch property {
	case "x":
		value = camera.X
	case "y":
		value = camera.Y
	case "z":
		value = camera.Z
	case "rotation_x":
		value = camera.RotationX
	case "rotation_y":
		value = camera.RotationY
	case "rotation_z":
		value = camera.RotationZ
	case "field_of_view":
		value = camera.FieldOfView
	case "focus_depth":
		value = camera.FocusDepth
	}
	if value != nil {
		return *value, nil
	}
	if property == "field_of_view" {
		return 50, nil
	}
	return 0, nil
}

func timelineV2PropertyKeyframes(keyframes []TimelineV2Keyframe) []PropertyKeyframe {
	out := make([]PropertyKeyframe, len(keyframes))
	for i, keyframe := range keyframes {
		out[i] = PropertyKeyframe{
			Property: keyframe.Property,
			TimeMS:   keyframe.TimeMS,
			Value:    keyframe.Value,
			Easing:   keyframe.Easing,
			Curve:    keyframe.Curve,
		}
	}
	return out
}

func transformValue(transform *TimelineV2Transform, field func(*TimelineV2Transform) *float64, fallback float64) float64 {
	if transform == nil {
		return fallback
	}
	value := field(transform)
	if value == nil {
		return fallback
	}
	return *value
}

func normalizePropertyName(property string) string {
	return strings.ToLower(strings.TrimSpace(property))
}

func unsupportedProperty(scope, property string) error {
	return fmt.Errorf("unsupported canonical %s property %q", scope, property)
}
