// Package rendercontract defines renderer-independent, deterministic timeline
// evaluation primitives. It deliberately contains no media I/O and no FFmpeg
// or browser-specific behavior so preview and export paths can share the same
// frame-boundary and interpolation semantics.
package rendercontract

import (
	"math"
	"strings"
)

const (
	EasingLinear    = "linear"
	EasingEaseIn    = "ease-in"
	EasingEaseOut   = "ease-out"
	EasingEaseInOut = "ease-in-out"
	EasingStep      = "step"
)

// RationalTime represents an exact timeline time in seconds. FrameTime returns
// frame_index / fps without converting through milliseconds or floating point.
type RationalTime struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

// MotionCurve is the renderer-independent interpolation contract for one
// keyframe segment. Springs are segment-local and reset velocity at each pair.
type MotionCurve struct {
	Type      string  `json:"type"`
	X1        float64 `json:"x1,omitempty"`
	Y1        float64 `json:"y1,omitempty"`
	X2        float64 `json:"x2,omitempty"`
	Y2        float64 `json:"y2,omitempty"`
	Stiffness float64 `json:"stiffness,omitempty"`
	Damping   float64 `json:"damping,omitempty"`
	Mass      float64 `json:"mass,omitempty"`
}

// FrameTime returns the exact presentation time of frameIndex at fps.
func FrameTime(frameIndex int64, fps int) RationalTime {
	if frameIndex < 0 {
		frameIndex = 0
	}
	if fps <= 0 {
		fps = 1
	}
	return RationalTime{Numerator: frameIndex, Denominator: int64(fps)}
}

// StartFrame maps an authored millisecond start to the canonical first output
// frame using floor(ms * fps / 1000).
func StartFrame(ms int64, fps int) int64 {
	if ms <= 0 || fps <= 0 {
		return 0
	}
	return (ms * int64(fps)) / 1000
}

// EndFrame maps an authored millisecond end to the canonical exclusive output
// frame using ceil(ms * fps / 1000).
func EndFrame(ms int64, fps int) int64 {
	if ms <= 0 || fps <= 0 {
		return 0
	}
	product := ms * int64(fps)
	return (product + 999) / 1000
}

// FrameCount returns the number of output frames needed to cover durationMS.
func FrameCount(durationMS int64, fps int) int64 {
	return EndFrame(durationMS, fps)
}

// ActiveAtFrame applies the canonical half-open activity rule after authored
// millisecond boundaries have been mapped to frames.
func ActiveAtFrame(frameIndex, startMS, durationMS int64, fps int) bool {
	if frameIndex < 0 || durationMS <= 0 || fps <= 0 {
		return false
	}
	start := StartFrame(startMS, fps)
	end := EndFrame(startMS+durationMS, fps)
	return start <= frameIndex && frameIndex < end
}

// SourceTimeMS maps an output timeline time to source time for a clip with a
// constant playback rate. It is deterministic and clamps before the clip start
// to trimInMS. Callers validate the playback-rate bounds separately.
func SourceTimeMS(timelineMS, clipStartMS, trimInMS int64, playbackRate float64) float64 {
	if playbackRate == 0 {
		playbackRate = 1
	}
	elapsed := timelineMS - clipStartMS
	if elapsed < 0 {
		elapsed = 0
	}
	return float64(trimInMS) + float64(elapsed)*playbackRate
}

// EaseProgress evaluates the canonical v1-compatible easing function. The
// ease-in-out curve intentionally matches the editor preview's piecewise
// quadratic behavior rather than the legacy FFmpeg fidelity smoothstep.
func EaseProgress(t float64, easing string) float64 {
	t = clamp01(t)
	switch strings.ToLower(strings.TrimSpace(easing)) {
	case EasingStep:
		if t < 1 {
			return 0
		}
		return 1
	case EasingEaseIn:
		return t * t
	case EasingEaseOut:
		return 1 - (1-t)*(1-t)
	case EasingEaseInOut:
		if t < 0.5 {
			return 2 * t * t
		}
		return 1 - math.Pow(-2*t+2, 2)/2
	default:
		return t
	}
}

// CurveProgress evaluates the canonical segment curve. Curve wins when
// present; fallback easing preserves v1 documents that only authored Easing.
func CurveProgress(t float64, curve *MotionCurve, fallback string) float64 {
	t = clamp01(t)
	if curve == nil {
		return EaseProgress(t, fallback)
	}
	switch strings.ToLower(strings.TrimSpace(curve.Type)) {
	case "bezier":
		return cubicBezierProgress(t, curve.X1, curve.Y1, curve.X2, curve.Y2)
	case "spring":
		return springProgress(t, curve.Stiffness, curve.Damping, curve.Mass)
	default:
		return EaseProgress(t, curve.Type)
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

func springProgress(t, stiffness, damping, mass float64) float64 {
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
