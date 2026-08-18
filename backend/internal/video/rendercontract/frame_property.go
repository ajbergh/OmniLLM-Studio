package rendercontract

import "sort"

// RationalMilliseconds represents an exact millisecond-domain time without
// rounding an output frame through integer milliseconds first.
type RationalMilliseconds struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

// FrameRelativeMilliseconds returns the exact frame presentation time relative
// to an authored millisecond origin: (frameIndex*1000 - originMS*fps) / fps.
func FrameRelativeMilliseconds(frameIndex int64, fps int, originMS int64) RationalMilliseconds {
	if frameIndex < 0 {
		frameIndex = 0
	}
	if fps <= 0 {
		fps = 1
	}
	return RationalMilliseconds{
		Numerator:   frameIndex*1000 - originMS*int64(fps),
		Denominator: int64(fps),
	}
}

// SamplePropertyKeyframesAtRationalMS is the exact-time sibling of
// SamplePropertyKeyframes. Keyframe timestamps stay authored integer
// milliseconds while the sampled presentation time remains rational.
func SamplePropertyKeyframesAtRationalMS(keyframes []PropertyKeyframe, property string, time RationalMilliseconds) (float64, bool) {
	property = normalizePropertyName(property)
	if property == "" {
		return 0, false
	}
	if time.Denominator <= 0 {
		time.Denominator = 1
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
	at := func(ms int64) int64 { return ms * time.Denominator }
	if time.Numerator <= at(points[0].TimeMS) {
		return points[0].Value, true
	}
	for i := 1; i < len(points); i++ {
		next := points[i]
		if time.Numerator <= at(next.TimeMS) {
			prev := points[i-1]
			span := next.TimeMS - prev.TimeMS
			progress := 1.0
			if span > 0 {
				progress = float64(time.Numerator-at(prev.TimeMS)) / float64(span*time.Denominator)
			}
			eased := CurveProgress(progress, next.Curve, next.Easing)
			return prev.Value + (next.Value-prev.Value)*eased, true
		}
	}
	return points[len(points)-1].Value, true
}

// EvaluateClipPropertyAtFrame resolves one semantic clip property at the exact
// output-frame presentation time, relative to clip start.
func EvaluateClipPropertyAtFrame(clip TimelineV2Clip, property string, frameIndex int64, fps int) (float64, error) {
	property = normalizePropertyName(property)
	base, err := ClipPropertyBaseValue(clip, property)
	if err != nil {
		return 0, err
	}
	time := FrameRelativeMilliseconds(frameIndex, fps, clip.StartMS)
	if value, ok := SamplePropertyKeyframesAtRationalMS(timelineV2PropertyKeyframes(clip.Keyframes), property, time); ok {
		return value, nil
	}
	return base, nil
}

// EvaluateCameraPropertyAtFrame resolves one semantic camera property at the
// exact output-frame presentation time, relative to scene start.
func EvaluateCameraPropertyAtFrame(camera *TimelineV2Camera, property string, frameIndex int64, fps int, sceneStartMS int64) (float64, error) {
	property = normalizePropertyName(property)
	base, err := CameraPropertyBaseValue(camera, property)
	if err != nil {
		return 0, err
	}
	if camera != nil {
		time := FrameRelativeMilliseconds(frameIndex, fps, sceneStartMS)
		if value, ok := SamplePropertyKeyframesAtRationalMS(timelineV2PropertyKeyframes(camera.Keyframes), property, time); ok {
			return value, nil
		}
	}
	return base, nil
}
