package video

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

type Renderer interface {
	Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error)
}

type RenderRequest struct {
	Project  models.VideoProject `json:"project"`
	Timeline TimelineDocument    `json:"timeline"`
	Settings ExportSettings      `json:"settings"`
}

type RenderProgress struct {
	Stage    string  `json:"stage"`
	Message  string  `json:"message"`
	Progress float64 `json:"progress"`
}

type RenderResult struct {
	MimeType   string         `json:"mime_type"`
	FileName   string         `json:"file_name"`
	Data       []byte         `json:"-"`
	DurationMS int64          `json:"duration_ms"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	FPS        float64        `json:"fps"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type MockRenderer struct{}

func NewMockRenderer() *MockRenderer {
	return &MockRenderer{}
}

func (r *MockRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	if req.Project.ID == "" {
		return nil, fmt.Errorf("project is required")
	}
	req.Timeline, _ = ValidateTimelineDocument(req.Timeline)
	steps := []RenderProgress{
		{Stage: "preparing", Message: "Preparing timeline composition", Progress: 0.15},
		{Stage: "compositing", Message: "Compositing clips and overlays", Progress: 0.45},
		{Stage: "encoding", Message: "Encoding mock export", Progress: 0.75},
		{Stage: "finalizing", Message: "Finalizing render asset", Progress: 0.95},
	}
	delay := 160 * time.Millisecond
	if req.Settings.MockRenderDelaySeconds > 0 {
		delay = time.Duration(req.Settings.MockRenderDelaySeconds * float64(time.Second))
	}
	for _, step := range steps {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			if progress != nil {
				progress(step)
			}
		}
	}

	payload := map[string]any{
		"kind":      "omnillm-video-studio-mock-export",
		"project":   req.Project.Title,
		"settings":  req.Settings,
		"timeline":  summarizeTimeline(req.Timeline),
		"createdAt": time.Now().UTC().Format(time.RFC3339),
		"note":      "This is a deterministic development export placeholder. Wire a Remotion or FFmpeg adapter behind Renderer for production video bytes.",
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	format := strings.ToLower(strings.TrimSpace(req.Settings.Format))
	if format == "" {
		format = "mp4"
	}
	return &RenderResult{
		MimeType:   "text/plain",
		FileName:   fmt.Sprintf("mock-render-%s.txt", sanitizePathSegment(req.Project.ID)),
		Data:       data,
		DurationMS: req.Timeline.DurationMS,
		Width:      req.Timeline.Canvas.Width,
		Height:     req.Timeline.Canvas.Height,
		FPS:        float64(req.Timeline.Canvas.FPS),
		Metadata: map[string]any{
			"renderer":         "mock",
			"requested_format": format,
			"quality":          req.Settings.Quality,
			"include_audio":    req.Settings.IncludeAudio,
		},
	}, nil
}

func summarizeTimeline(doc TimelineDocument) map[string]any {
	clips := 0
	for _, track := range doc.Tracks {
		clips += len(track.Clips)
	}
	return map[string]any{
		"version":     doc.Version,
		"duration_ms": doc.DurationMS,
		"width":       doc.Canvas.Width,
		"height":      doc.Canvas.Height,
		"fps":         doc.Canvas.FPS,
		"track_count": len(doc.Tracks),
		"clip_count":  clips,
	}
}
