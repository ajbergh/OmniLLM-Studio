package video

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestFFmpegRendererProducesVideoAsset(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	renderer := NewFFmpegRenderer("")
	timeline := NewEmptyTimeline(320, 180, 24)
	timeline.DurationMS = 1000
	timeline.Tracks = append(timeline.Tracks, TimelineTrack{
		ID:      "track-title",
		Type:    TrackTypeText,
		Name:    "Title",
		Visible: true,
		Clips: []TimelineClip{{
			ID:         "clip-title",
			StartMS:    0,
			DurationMS: 1000,
			TrimOutMS:  1000,
			Text: &TimelineText{
				Text:     "Real export",
				FontSize: 24,
				Color:    "#ffffff",
				Shadow:   true,
			},
			Effects:   []TimelineEffect{},
			Keyframes: []TimelineKeyframe{},
		}},
	})
	var progressEvents int
	result, err := renderer.Render(context.Background(), RenderRequest{
		Project:  models.VideoProject{ID: "project-1", Title: "Demo", Width: 320, Height: 180, FPS: 24},
		Timeline: timeline,
		Settings: ExportSettings{
			Format:       "mp4",
			Resolution:   "project",
			Quality:      "draft",
			IncludeAudio: true,
		},
	}, func(RenderProgress) {
		progressEvents++
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if progressEvents == 0 {
		t.Fatalf("expected FFmpeg progress events")
	}
	if result.MimeType != "video/mp4" || len(result.Data) == 0 {
		t.Fatalf("unexpected render result: %+v", result)
	}
	if !bytes.Contains(result.Data[:minInt(len(result.Data), 64)], []byte("ftyp")) {
		t.Fatalf("expected MP4 ftyp box near start")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
