package video

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

const (
	MaxAgentInspectionLayers = 50
	MaxAgentInspectionClips  = 200
	MaxDiagnosticDurationMS  = 5000
	MaxDiagnosticWidth       = 1280
	MaxDiagnosticHeight      = 720
)

type AgentLayerSummary struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	ClipCount int      `json:"clip_count"`
	ClipIDs   []string `json:"clip_ids"`
}

type AgentProjectInspection struct {
	ProjectID           string               `json:"project_id"`
	TimelineID          string               `json:"timeline_id"`
	Revision            string               `json:"revision"`
	Canvas              TimelineCanvas       `json:"canvas"`
	DurationMS          int64                `json:"duration_ms"`
	SceneCount          int                  `json:"scene_count"`
	LayerCount          int                  `json:"layer_count"`
	ClipCount           int                  `json:"clip_count"`
	KeyframeCount       int                  `json:"keyframe_count"`
	AnimationBlockCount int                  `json:"animation_block_count"`
	EffectCount         int                  `json:"effect_count"`
	Layers              []AgentLayerSummary  `json:"layers"`
	Issues              []string             `json:"issues"`
	Truncated           bool                 `json:"truncated"`
	Capabilities        RendererCapabilities `json:"renderer_capabilities"`
}

func (s *Service) ValidateTimelineBinding(ctx context.Context, userID, projectID, timelineID, revision string) (*models.VideoTimeline, TimelineDocument, error) {
	timeline, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if strings.TrimSpace(timelineID) == "" || timeline.ID != timelineID {
		return nil, TimelineDocument{}, fmt.Errorf("timeline binding mismatch; inspect the project again")
	}
	current, err := TimelineRevision(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	if strings.TrimSpace(revision) == "" || revision != current {
		return nil, TimelineDocument{}, fmt.Errorf("stale timeline revision; inspect the project again (current %s)", current)
	}
	return timeline, doc, nil
}

func (s *Service) InspectMotionProject(ctx context.Context, userID, projectID string) (*AgentProjectInspection, error) {
	timeline, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	revision, err := TimelineRevision(doc)
	if err != nil {
		return nil, err
	}
	result := &AgentProjectInspection{ProjectID: projectID, TimelineID: timeline.ID, Revision: revision, Canvas: doc.Canvas, DurationMS: doc.DurationMS, SceneCount: len(doc.Scenes), LayerCount: len(doc.Tracks), Layers: []AgentLayerSummary{}, Issues: []string{}, Capabilities: FFmpegRendererCapabilities()}
	remaining := MaxAgentInspectionClips
	for _, track := range doc.Tracks {
		result.ClipCount += len(track.Clips)
		for _, clip := range track.Clips {
			result.KeyframeCount += len(clip.Keyframes)
			result.AnimationBlockCount += len(clip.AnimationBlocks)
			result.EffectCount += len(clip.Effects)
			if clip.TemplateSlot != "" && clip.AssetID == "" && clip.Text == nil {
				result.Issues = append(result.Issues, fmt.Sprintf("template slot %q is empty", clip.TemplateSlot))
			}
		}
		if len(result.Layers) >= MaxAgentInspectionLayers {
			result.Truncated = true
			continue
		}
		take := len(track.Clips)
		if take > remaining {
			take = remaining
			result.Truncated = true
		}
		ids := make([]string, 0, take)
		for i := 0; i < take; i++ {
			ids = append(ids, track.Clips[i].ID)
		}
		result.Layers = append(result.Layers, AgentLayerSummary{ID: track.ID, Name: track.Name, Type: track.Type, ClipCount: len(track.Clips), ClipIDs: ids})
		remaining -= take
		if remaining < 0 {
			remaining = 0
		}
	}
	for _, scene := range doc.Scenes {
		result.EffectCount += len(scene.Effects)
	}
	if len(result.Issues) > 50 {
		result.Issues = result.Issues[:50]
		result.Truncated = true
	}
	return result, nil
}

type DiagnosticRenderRequest struct {
	TimelineID string `json:"timeline_id"`
	Revision   string `json:"revision"`
	TimeMS     int64  `json:"time_ms,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Iteration  int    `json:"iteration,omitempty"`
}

func (s *Service) StartDiagnosticRender(ctx context.Context, userID, projectID string, request DiagnosticRenderRequest, frameOnly bool) (*models.VideoRenderJob, error) {
	if request.Iteration < 1 || request.Iteration > 3 {
		return nil, fmt.Errorf("diagnostic iteration must be between 1 and 3")
	}
	_, doc, err := s.ValidateTimelineBinding(ctx, userID, projectID, request.TimelineID, request.Revision)
	if err != nil {
		return nil, err
	}
	start := request.TimeMS
	if start < 0 {
		start = 0
	}
	if start >= doc.DurationMS {
		return nil, fmt.Errorf("diagnostic time is outside the timeline")
	}
	duration := request.DurationMS
	if frameOnly {
		duration = int64(1000 / maxInt(1, doc.Canvas.FPS))
	}
	if duration <= 0 {
		duration = 2000
	}
	if duration > MaxDiagnosticDurationMS {
		duration = MaxDiagnosticDurationMS
	}
	end := minInt64(doc.DurationMS, start+duration)
	if end <= start {
		end = minInt64(doc.DurationMS, start+1)
	}
	width, height := request.Width, request.Height
	if width <= 0 {
		width = 960
	}
	if height <= 0 {
		height = 540
	}
	width = minInt(MaxDiagnosticWidth, maxInt(2, width))
	height = minInt(MaxDiagnosticHeight, maxInt(2, height))
	if width%2 != 0 {
		width--
	}
	if height%2 != 0 {
		height--
	}
	return s.StartRender(ctx, userID, projectID, ExportSettings{Format: "mp4", Codec: "h264", Resolution: "custom", Width: width, Height: height, FPS: minInt(30, maxInt(1, doc.Canvas.FPS)), Quality: "draft", IncludeAudio: false, RangeStartMS: start, RangeEndMS: end, EstimatedDurationMS: end - start, Priority: 10})
}
