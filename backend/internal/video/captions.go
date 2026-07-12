package video

import (
	"fmt"
	"sort"
	"strings"
)

// CaptionCue is a flattened caption entry extracted from caption tracks.
type CaptionCue struct {
	StartMS int64
	EndMS   int64
	Text    string
}

// CaptionCuesFromTimeline flattens caption-track clips with text into sorted
// cues. Mirrors the frontend's exportCaptions selection rules.
func CaptionCuesFromTimeline(doc TimelineDocument) []CaptionCue {
	var cues []CaptionCue
	for _, track := range doc.Tracks {
		if track.Type != TrackTypeCaption {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Text == nil || strings.TrimSpace(clip.Text.Text) == "" {
				continue
			}
			cues = append(cues, CaptionCue{
				StartMS: clip.StartMS,
				EndMS:   clip.StartMS + clip.DurationMS,
				Text:    clip.Text.Text,
			})
		}
	}
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].StartMS < cues[j].StartMS })
	return cues
}

func formatCaptionTimestamp(ms int64, separator string) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3_600_000
	minutes := (ms % 3_600_000) / 60_000
	seconds := (ms % 60_000) / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", hours, minutes, seconds, separator, millis)
}

// SerializeCaptions renders cues as "srt" or "vtt" sidecar content. Returns ""
// when there are no cues or the format is unknown.
func SerializeCaptions(cues []CaptionCue, format string) string {
	if len(cues) == 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "srt":
		var b strings.Builder
		for i, cue := range cues {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1,
				formatCaptionTimestamp(cue.StartMS, ","), formatCaptionTimestamp(cue.EndMS, ","), cue.Text)
		}
		return strings.TrimSuffix(b.String(), "\n")
	case "vtt":
		var b strings.Builder
		b.WriteString("WEBVTT\n\n")
		for _, cue := range cues {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n", formatCaptionTimestamp(cue.StartMS, "."), formatCaptionTimestamp(cue.EndMS, "."), cue.Text)
		}
		return strings.TrimSuffix(b.String(), "\n")
	default:
		return ""
	}
}
