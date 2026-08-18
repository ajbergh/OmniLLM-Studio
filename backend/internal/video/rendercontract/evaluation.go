package rendercontract

import "sort"

// FrameRange is a half-open output-frame range [start, end).
type FrameRange struct {
	StartFrame int64 `json:"start_frame"`
	EndFrame   int64 `json:"end_frame"`
}

// ActiveClip identifies one temporally active clip at an output frame. The
// indices are retained so every runtime can reproduce the canonical stable
// composition order without depending on an interval-index implementation.
type ActiveClip struct {
	TrackIndex   int     `json:"track_index"`
	ClipIndex    int     `json:"clip_index"`
	TrackID      string  `json:"track_id"`
	ClipID       string  `json:"clip_id"`
	ZIndex       int     `json:"z_index"`
	StartFrame   int64   `json:"start_frame"`
	EndFrame     int64   `json:"end_frame"`
	SourceTimeMS float64 `json:"source_time_ms"`
}

// FrameRangeFromMS maps an explicit authored millisecond interval to the
// canonical half-open output-frame interval. Invalid/empty ranges stay empty;
// callers decide whether an omitted export range means the whole timeline.
func FrameRangeFromMS(startMS, endMS int64, fps int) FrameRange {
	if fps <= 0 || endMS <= startMS {
		return FrameRange{}
	}
	if startMS < 0 {
		startMS = 0
	}
	if endMS < 0 {
		return FrameRange{}
	}
	start := StartFrame(startMS, fps)
	end := EndFrame(endMS, fps)
	if end < start {
		end = start
	}
	return FrameRange{StartFrame: start, EndFrame: end}
}

// Contains reports whether frameIndex belongs to the half-open range.
func (r FrameRange) Contains(frameIndex int64) bool {
	return frameIndex >= r.StartFrame && frameIndex < r.EndFrame
}

// SourceTimeAtFrameMS derives clip source time directly from output-frame
// identity. The computation does not round-trip the frame through integer
// milliseconds. Before the authored clip start, source time clamps to trim-in.
func SourceTimeAtFrameMS(frameIndex int64, fps int, clipStartMS, trimInMS int64, playbackRate float64) float64 {
	if playbackRate == 0 {
		playbackRate = 1
	}
	if frameIndex < 0 || fps <= 0 {
		return float64(trimInMS)
	}
	elapsedNumerator := frameIndex*1000 - clipStartMS*int64(fps)
	if elapsedNumerator <= 0 {
		return float64(trimInMS)
	}
	return float64(trimInMS) + (float64(elapsedNumerator)*playbackRate)/float64(fps)
}

// ActiveClipsAtFrame returns every temporally active clip in canonical stable
// order: track array index, z_index (default 0), then clip array index. Track
// visibility/mute/solo are composition/audio concerns and are intentionally not
// folded into temporal activity here.
func ActiveClipsAtFrame(doc TimelineV2Document, frameIndex int64) []ActiveClip {
	fps := doc.Canvas.FPS
	if frameIndex < 0 || fps <= 0 {
		return nil
	}
	active := make([]ActiveClip, 0)
	for trackIndex, track := range doc.Tracks {
		for clipIndex, clip := range track.Clips {
			if !ActiveAtFrame(frameIndex, clip.StartMS, clip.DurationMS, fps) {
				continue
			}
			zIndex := 0
			if clip.ZIndex != nil {
				zIndex = *clip.ZIndex
			}
			playbackRate := 1.0
			if clip.PlaybackRate != nil && *clip.PlaybackRate != 0 {
				playbackRate = *clip.PlaybackRate
			}
			active = append(active, ActiveClip{
				TrackIndex:   trackIndex,
				ClipIndex:    clipIndex,
				TrackID:      track.ID,
				ClipID:       clip.ID,
				ZIndex:       zIndex,
				StartFrame:   StartFrame(clip.StartMS, fps),
				EndFrame:     EndFrame(clip.StartMS+clip.DurationMS, fps),
				SourceTimeMS: SourceTimeAtFrameMS(frameIndex, fps, clip.StartMS, clip.TrimInMS, playbackRate),
			})
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		left, right := active[i], active[j]
		if left.TrackIndex != right.TrackIndex {
			return left.TrackIndex < right.TrackIndex
		}
		if left.ZIndex != right.ZIndex {
			return left.ZIndex < right.ZIndex
		}
		return left.ClipIndex < right.ClipIndex
	})
	return active
}
