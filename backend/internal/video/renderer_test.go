package video

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestMockRendererProducesPlaceholderAsset(t *testing.T) {
	renderer := NewMockRenderer()
	var progressEvents int
	result, err := renderer.Render(context.Background(), RenderRequest{
		Project:  models.VideoProject{ID: "project-1", Title: "Demo", Width: 1920, Height: 1080, FPS: 30},
		Timeline: NewEmptyTimeline(1920, 1080, 30),
		Settings: ExportSettings{
			Format:                 "mp4",
			Resolution:             "project",
			Quality:                "draft",
			IncludeAudio:           true,
			MockRenderDelaySeconds: 0.001,
		},
	}, func(RenderProgress) {
		progressEvents++
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if result.MimeType != "text/plain" || len(result.Data) == 0 {
		t.Fatalf("unexpected render result: %+v", result)
	}
	if progressEvents == 0 {
		t.Fatalf("expected progress events")
	}
}

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
	result, err := renderer.Render(context.Background(), RenderRequest{
		Project:  models.VideoProject{ID: "project-1", Title: "Demo", Width: 320, Height: 180, FPS: 24},
		Timeline: timeline,
		Settings: ExportSettings{
			Format:       "mp4",
			Resolution:   "project",
			Quality:      "draft",
			IncludeAudio: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
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
