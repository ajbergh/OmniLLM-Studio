package rendercontract

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	TimelineV2RuntimeInvalidCode = "TIMELINE_V2_RUNTIME_INVALID"
	minTimelineV2PlaybackRate    = 0.25
	maxTimelineV2PlaybackRate    = 4.0
)

var timelineV2TrackTypes = map[string]bool{
	"layer": true, "video": true, "image": true, "audio": true, "music": true,
	"text": true, "caption": true, "shape": true, "callout": true,
}

var timelineV2MediaFits = map[string]bool{
	"contain": true, "cover": true, "fill": true, "none": true,
}

// TimelineV2RuntimeError is a path-addressed semantic validation failure found
// after schema projection but before canonical frame evaluation.
type TimelineV2RuntimeError struct {
	Code        string
	Path        string
	Message     string
	Remediation string
}

func (e *TimelineV2RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// NormalizeTimelineV2EvaluationInputs returns a deep-copied Timeline v2 with
// deterministic defaults required by frame activity/source-time evaluation.
// It deliberately does not claim to validate feature-specific text, shape,
// effect, transition, or camera semantics; those remain owned by the evaluator
// slices that implement them.
func NormalizeTimelineV2EvaluationInputs(doc TimelineV2Document) (TimelineV2Document, error) {
	normalized, err := cloneTimelineV2(doc)
	if err != nil {
		return TimelineV2Document{}, err
	}
	if normalized.Version != TimelineV2Version {
		return TimelineV2Document{}, runtimeTimelineError("version", fmt.Sprintf("version must be %d", TimelineV2Version), "adapt or upgrade the timeline to Timeline v2 before evaluation")
	}
	if normalized.Canvas.Width < 1 {
		return TimelineV2Document{}, runtimeTimelineError("canvas.width", "width must be at least 1", "provide a positive canvas width")
	}
	if normalized.Canvas.Height < 1 {
		return TimelineV2Document{}, runtimeTimelineError("canvas.height", "height must be at least 1", "provide a positive canvas height")
	}
	if normalized.Canvas.FPS < 1 || normalized.Canvas.FPS > 120 {
		return TimelineV2Document{}, runtimeTimelineError("canvas.fps", "fps must be between 1 and 120", "choose a supported integer output frame rate")
	}
	if strings.TrimSpace(normalized.Canvas.Background) == "" {
		return TimelineV2Document{}, runtimeTimelineError("canvas.background", "background must not be empty", "provide an explicit canvas background")
	}
	if normalized.DurationMS < 0 {
		return TimelineV2Document{}, runtimeTimelineError("duration_ms", "duration_ms cannot be negative", "provide a non-negative timeline duration")
	}
	if normalized.WorkingColorSpace == "" {
		normalized.WorkingColorSpace = RenderWorkingColorSpaceSRGB
	}
	if normalized.WorkingColorSpace != RenderWorkingColorSpaceSRGB {
		return TimelineV2Document{}, runtimeTimelineError("working_color_space", fmt.Sprintf("working color space %q is unsupported", normalized.WorkingColorSpace), "use srgb until another canonical working color space is versioned")
	}
	if normalized.Metadata == nil {
		normalized.Metadata = Metadata{}
	}
	if normalized.Tracks == nil {
		normalized.Tracks = []TimelineV2Track{}
	}
	if normalized.Markers == nil {
		normalized.Markers = []TimelineV2Marker{}
	}

	trackIDs := make(map[string]bool, len(normalized.Tracks))
	clipIDs := map[string]bool{}
	maxClipEnd := int64(0)
	for trackIndex := range normalized.Tracks {
		track := &normalized.Tracks[trackIndex]
		trackPath := fmt.Sprintf("tracks[%d]", trackIndex)
		track.ID = strings.TrimSpace(track.ID)
		if track.ID == "" {
			return TimelineV2Document{}, runtimeTimelineError(trackPath+".id", "track id must not be empty", "provide a stable track id")
		}
		if trackIDs[track.ID] {
			return TimelineV2Document{}, runtimeTimelineError(trackPath+".id", fmt.Sprintf("duplicate track id %q", track.ID), "use a unique track id")
		}
		trackIDs[track.ID] = true
		track.Type = strings.ToLower(strings.TrimSpace(track.Type))
		if !timelineV2TrackTypes[track.Type] {
			return TimelineV2Document{}, runtimeTimelineError(trackPath+".type", fmt.Sprintf("unsupported track type %q", track.Type), "use a Timeline v2 track type")
		}
		if track.Height != nil && (*track.Height < 32 || *track.Height > 160) {
			return TimelineV2Document{}, runtimeTimelineError(trackPath+".height", "track height must be between 32 and 160", "choose a supported track height")
		}
		if track.Clips == nil {
			track.Clips = []TimelineV2Clip{}
		}
		for clipIndex := range track.Clips {
			clip := &track.Clips[clipIndex]
			clipPath := fmt.Sprintf("%s.clips[%d]", trackPath, clipIndex)
			clip.ID = strings.TrimSpace(clip.ID)
			if clip.ID == "" {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".id", "clip id must not be empty", "provide a stable clip id")
			}
			if clipIDs[clip.ID] {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".id", fmt.Sprintf("duplicate clip id %q", clip.ID), "use a globally unique clip id")
			}
			clipIDs[clip.ID] = true
			if clip.StartMS < 0 {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".start_ms", "start_ms cannot be negative", "place the clip at or after timeline time zero")
			}
			if clip.DurationMS < 1 {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".duration_ms", "duration_ms must be at least 1", "provide a positive output-timeline duration")
			}
			if clip.TrimInMS < 0 {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".trim_in_ms", "trim_in_ms cannot be negative", "provide a non-negative source trim-in point")
			}
			if clip.TrimOutMS < 0 {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".trim_out_ms", "trim_out_ms cannot be negative", "provide a non-negative source trim-out point")
			}

			rate := 1.0
			if clip.PlaybackRate != nil {
				rate = *clip.PlaybackRate
			}
			if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < minTimelineV2PlaybackRate || rate > maxTimelineV2PlaybackRate {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".playback_rate", "playback_rate must be finite and between 0.25 and 4", "choose a supported constant playback rate")
			}
			clip.PlaybackRate = float64Pointer(rate)
			clip.TrimOutMS = clip.TrimInMS + sourceDurationForTimelineV2(clip.DurationMS, rate)

			visual := track.Type != "audio" && track.Type != "music" && !clip.AudioOnly
			if visual {
				if clip.Transform == nil {
					clip.Transform = defaultTimelineV2Transform()
				} else if err := normalizeTimelineV2Transform(clip.Transform, clipPath+".transform"); err != nil {
					return TimelineV2Document{}, err
				}
				if clip.AssetID != "" && clip.MediaFit == "" {
					clip.MediaFit = "contain"
				}
			}
			if clip.MediaFit != "" && !timelineV2MediaFits[clip.MediaFit] {
				return TimelineV2Document{}, runtimeTimelineError(clipPath+".media_fit", fmt.Sprintf("unsupported media_fit %q", clip.MediaFit), "use contain, cover, fill, or none")
			}
			if clip.Effects == nil {
				clip.Effects = []TimelineV2Effect{}
			}
			if clip.Keyframes == nil {
				clip.Keyframes = []TimelineV2Keyframe{}
			}
			clipEnd := clip.StartMS + clip.DurationMS
			if clipEnd > maxClipEnd {
				maxClipEnd = clipEnd
			}
		}
	}
	if normalized.DurationMS < maxClipEnd {
		normalized.DurationMS = maxClipEnd
	}
	return normalized, nil
}

func cloneTimelineV2(doc TimelineV2Document) (TimelineV2Document, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return TimelineV2Document{}, fmt.Errorf("clone Timeline v2: %w", err)
	}
	var cloned TimelineV2Document
	if err := json.Unmarshal(data, &cloned); err != nil {
		return TimelineV2Document{}, fmt.Errorf("clone Timeline v2: %w", err)
	}
	return cloned, nil
}

func normalizeTimelineV2Transform(transform *TimelineV2Transform, path string) error {
	defaults := defaultTimelineV2Transform()
	if transform.X == nil {
		transform.X = defaults.X
	}
	if transform.Y == nil {
		transform.Y = defaults.Y
	}
	if transform.Scale == nil {
		transform.Scale = defaults.Scale
	}
	if transform.Rotation == nil {
		transform.Rotation = defaults.Rotation
	}
	if transform.Opacity == nil {
		transform.Opacity = defaults.Opacity
	}
	values := []struct {
		name  string
		value *float64
	}{
		{"x", transform.X}, {"y", transform.Y}, {"z", transform.Z}, {"scale", transform.Scale},
		{"scale_x", transform.ScaleX}, {"scale_y", transform.ScaleY}, {"rotation", transform.Rotation},
		{"rotation_x", transform.RotationX}, {"rotation_y", transform.RotationY}, {"rotation_z", transform.RotationZ},
		{"opacity", transform.Opacity}, {"anchor_x", transform.AnchorX}, {"anchor_y", transform.AnchorY}, {"perspective", transform.Perspective},
	}
	for _, entry := range values {
		if entry.value != nil && (math.IsNaN(*entry.value) || math.IsInf(*entry.value, 0)) {
			return runtimeTimelineError(path+"."+entry.name, "transform value must be finite", "provide a finite numeric transform value")
		}
	}
	if transform.Opacity != nil && (*transform.Opacity < 0 || *transform.Opacity > 1) {
		return runtimeTimelineError(path+".opacity", "opacity must be between 0 and 1", "provide normalized opacity")
	}
	if transform.Crop != nil {
		crop := transform.Crop
		cropValues := []struct {
			name  string
			value float64
		}{{"top", crop.Top}, {"right", crop.Right}, {"bottom", crop.Bottom}, {"left", crop.Left}}
		for _, entry := range cropValues {
			if math.IsNaN(entry.value) || math.IsInf(entry.value, 0) || entry.value < 0 || entry.value > 1 {
				return runtimeTimelineError(path+".crop."+entry.name, "crop value must be finite and between 0 and 1", "provide a normalized crop edge")
			}
		}
	}
	return nil
}

func defaultTimelineV2Transform() *TimelineV2Transform {
	return &TimelineV2Transform{
		X: float64Pointer(0), Y: float64Pointer(0), Scale: float64Pointer(1),
		Rotation: float64Pointer(0), Opacity: float64Pointer(1),
	}
}

func sourceDurationForTimelineV2(durationMS int64, playbackRate float64) int64 {
	duration := int64(math.Round(float64(durationMS) * playbackRate))
	if duration < 1 {
		return 1
	}
	return duration
}

func float64Pointer(value float64) *float64 {
	copied := value
	return &copied
}

func runtimeTimelineError(path, message, remediation string) *TimelineV2RuntimeError {
	return &TimelineV2RuntimeError{Code: TimelineV2RuntimeInvalidCode, Path: path, Message: message, Remediation: remediation}
}
