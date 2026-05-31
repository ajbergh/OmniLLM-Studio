package video

import (
	"context"
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
