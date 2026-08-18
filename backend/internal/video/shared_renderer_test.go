package video

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestSharedCompositionRenderer(t *testing.T) {
	renderer := NewSharedCompositionRenderer("video-render-worker", "ffmpeg", "ffprobe")
	if renderer == nil {
		t.Fatal("expected non-nil SharedCompositionRenderer")
	}

	snapshot := &models.VideoRenderSnapshot{
		ID: "test-snapshot-123",
	}

	var progressSteps []string
	err := renderer.RenderVideo(context.Background(), snapshot, "/tmp/output.mp4", func(progress float64, step string) {
		progressSteps = append(progressSteps, step)
	})

	if err != nil {
		t.Fatalf("unexpected error rendering video: %v", err)
	}

	if len(progressSteps) == 0 {
		t.Fatal("expected progress steps to be recorded")
	}
}
