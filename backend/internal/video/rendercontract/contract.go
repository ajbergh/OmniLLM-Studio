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

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
