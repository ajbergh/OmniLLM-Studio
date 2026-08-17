package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

type VideoProjectInspectTool struct{ service *video.Service }

func NewVideoProjectInspectTool(service *video.Service) *VideoProjectInspectTool {
	return &VideoProjectInspectTool{service: service}
}
func (t *VideoProjectInspectTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "video_project_inspect", Description: "Inspect a Video Studio project's bounded timeline structure, revision, issues, and renderer fidelity without returning raw project JSON.", Category: "studio", Enabled: t.service != nil, Version: "1", Risk: RiskLow, DefaultTimeoutMS: 5000, MaxResultBytes: 65536, Parameters: json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","minLength":1}}}`)}
}
func (t *VideoProjectInspectTool) Validate(raw json.RawMessage) error {
	var input struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	return nil
}
func (t *VideoProjectInspectTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var input struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	inspection, err := t.service.InspectMotionProject(ctx, InvocationScopeFromContext(ctx).UserID, input.ProjectID)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(inspection)
	return &ToolResult{Content: fmt.Sprintf("Inspected timeline %s at revision %s: %d layers, %d clips, %d scenes, %d issue(s).", inspection.TimelineID, inspection.Revision, inspection.LayerCount, inspection.ClipCount, inspection.SceneCount, len(inspection.Issues)), Structured: structured}, nil
}

type VideoTimelineMutateTool struct{ service *video.Service }

func NewVideoTimelineMutateTool(service *video.Service) *VideoTimelineMutateTool {
	return &VideoTimelineMutateTool{service: service}
}

type videoTimelineMutateInput struct {
	ProjectID  string                `json:"project_id"`
	TimelineID string                `json:"timeline_id"`
	Revision   string                `json:"revision"`
	Summary    string                `json:"summary,omitempty"`
	Operations []video.EditOperation `json:"operations"`
}

func (t *VideoTimelineMutateTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "video_timeline_mutate", Description: "Apply a bounded, validated Video Studio edit plan to an explicit timeline revision. The entire mutation is rejected if any operation is invalid or stale.", Category: "studio", Enabled: t.service != nil, Version: "1", Risk: RiskMedium, SideEffecting: true, DefaultTimeoutMS: 10000, MaxResultBytes: 65536, Parameters: json.RawMessage(`{"type":"object","required":["project_id","timeline_id","revision","operations"],"properties":{"project_id":{"type":"string"},"timeline_id":{"type":"string"},"revision":{"type":"string"},"summary":{"type":"string","maxLength":1000},"operations":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"object","required":["type"],"properties":{"type":{"type":"string"},"clip_id":{"type":"string"},"track_id":{"type":"string"},"scene_id":{"type":"string"},"asset_id":{"type":"string"},"start_ms":{"type":"integer"},"duration_ms":{"type":"integer"},"text":{"type":"string","maxLength":12000},"width":{"type":"integer"},"height":{"type":"integer"},"fps":{"type":"integer"},"volume":{"type":"number"},"x":{"type":"number"},"y":{"type":"number"},"z":{"type":"number"},"scale":{"type":"number"},"scale_x":{"type":"number"},"scale_y":{"type":"number"},"rotation_x":{"type":"number"},"rotation_y":{"type":"number"},"rotation_z":{"type":"number"},"opacity":{"type":"number"},"field_of_view":{"type":"number"},"focus_depth":{"type":"number"},"property":{"type":"string"},"value":{"type":"number"},"easing":{"type":"string"},"curve":{"type":"object"},"effect_type":{"type":"string"},"params":{"type":"object"}}}}}}`)}
}
func (t *VideoTimelineMutateTool) Validate(raw json.RawMessage) error {
	var input videoTimelineMutateInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.TimelineID) == "" || strings.TrimSpace(input.Revision) == "" {
		return fmt.Errorf("project_id, timeline_id, and revision are required")
	}
	if len(input.Operations) == 0 || len(input.Operations) > 50 {
		return fmt.Errorf("operations must contain 1 to 50 items")
	}
	return nil
}
func (t *VideoTimelineMutateTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var input videoTimelineMutateInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	timeline, doc, err := t.service.ApplyEditPlan(ctx, InvocationScopeFromContext(ctx).UserID, input.ProjectID, video.EditPlan{TimelineID: input.TimelineID, Revision: input.Revision, Summary: input.Summary, Operations: input.Operations})
	if err != nil {
		return nil, err
	}
	revision, err := video.TimelineRevision(doc)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]any{"timeline_id": timeline.ID, "revision": revision, "applied_operations": len(input.Operations), "summary": input.Summary})
	return &ToolResult{Content: fmt.Sprintf("Applied %d operation(s) to timeline %s. New revision: %s.", len(input.Operations), timeline.ID, revision), Structured: structured}, nil
}

type VideoDiagnosticRenderTool struct {
	service   *video.Service
	frameOnly bool
}

func NewVideoRenderFrameTool(service *video.Service) *VideoDiagnosticRenderTool {
	return &VideoDiagnosticRenderTool{service: service, frameOnly: true}
}
func NewVideoRenderPreviewTool(service *video.Service) *VideoDiagnosticRenderTool {
	return &VideoDiagnosticRenderTool{service: service, frameOnly: false}
}
func (t *VideoDiagnosticRenderTool) Definition() ToolDefinition {
	name, description := "video_render_preview", "Queue a bounded diagnostic preview (up to five seconds) through the real Video Studio renderer."
	if t.frameOnly {
		name = "video_render_frame"
		description = "Queue a one-frame-duration diagnostic render through the real Video Studio renderer."
	}
	return ToolDefinition{Name: name, Description: description + " The caller must pass refinement iteration 1, 2, or 3; a fourth iteration is rejected.", Category: "studio", Enabled: t.service != nil, Version: "1", Risk: RiskMedium, SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 32768, Parameters: json.RawMessage(`{"type":"object","required":["project_id","timeline_id","revision","iteration"],"properties":{"project_id":{"type":"string"},"timeline_id":{"type":"string"},"revision":{"type":"string"},"time_ms":{"type":"integer","minimum":0},"duration_ms":{"type":"integer","minimum":1,"maximum":5000},"width":{"type":"integer","minimum":2,"maximum":1280},"height":{"type":"integer","minimum":2,"maximum":720},"iteration":{"type":"integer","minimum":1,"maximum":3}}}`)}
}
func (t *VideoDiagnosticRenderTool) Validate(raw json.RawMessage) error {
	var input struct {
		ProjectID string `json:"project_id"`
		video.DiagnosticRenderRequest
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.TimelineID) == "" || strings.TrimSpace(input.Revision) == "" {
		return fmt.Errorf("project_id, timeline_id, and revision are required")
	}
	if input.Iteration < 1 || input.Iteration > 3 {
		return fmt.Errorf("iteration must be between 1 and 3")
	}
	if input.DurationMS > video.MaxDiagnosticDurationMS {
		return fmt.Errorf("duration_ms exceeds %d", video.MaxDiagnosticDurationMS)
	}
	return nil
}
func (t *VideoDiagnosticRenderTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var input struct {
		ProjectID string `json:"project_id"`
		video.DiagnosticRenderRequest
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	job, err := t.service.StartDiagnosticRender(ctx, InvocationScopeFromContext(ctx).UserID, input.ProjectID, input.DiagnosticRenderRequest, t.frameOnly)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]any{"render_job_id": job.ID, "status": job.Status, "timeline_id": job.TimelineID})
	return &ToolResult{Content: fmt.Sprintf("Diagnostic render queued as %s. Use the render-job status/cancel endpoints to monitor or cancel it.", job.ID), Structured: structured, Metadata: map[string]any{"render_job_id": job.ID, "status": job.Status}}, nil
}

type VideoRenderJobStatusTool struct{ service *video.Service }

func NewVideoRenderJobStatusTool(service *video.Service) *VideoRenderJobStatusTool {
	return &VideoRenderJobStatusTool{service: service}
}
func (t *VideoRenderJobStatusTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "video_render_status", Description: "Inspect an owned Video Studio diagnostic render job and retrieve its artifact when complete.", Category: "studio", Enabled: t.service != nil, Version: "1", Risk: RiskLow, DefaultTimeoutMS: 5000, MaxResultBytes: 32768, Parameters: json.RawMessage(`{"type":"object","required":["render_job_id"],"properties":{"render_job_id":{"type":"string"}}}`)}
}
func (t *VideoRenderJobStatusTool) Validate(raw json.RawMessage) error {
	var input struct {
		JobID string `json:"render_job_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.JobID) == "" {
		return fmt.Errorf("render_job_id is required")
	}
	return nil
}
func (t *VideoRenderJobStatusTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var input struct {
		JobID string `json:"render_job_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	job, err := t.service.GetRenderJob(InvocationScopeFromContext(ctx).UserID, input.JobID)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(job)
	result := &ToolResult{Content: fmt.Sprintf("Render job %s is %s (%.0f%%).", job.ID, job.Status, job.Progress), Structured: structured}
	if job.OutputAssetID != nil && *job.OutputAssetID != "" {
		result.Artifacts = []ToolArtifact{{ID: *job.OutputAssetID, Name: "diagnostic-render.mp4", MimeType: "video/mp4", URL: "/v1/video/assets/" + *job.OutputAssetID + "/download"}}
	}
	return result, nil
}

type VideoRenderJobCancelTool struct{ service *video.Service }

func NewVideoRenderJobCancelTool(service *video.Service) *VideoRenderJobCancelTool {
	return &VideoRenderJobCancelTool{service: service}
}
func (t *VideoRenderJobCancelTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "video_render_cancel", Description: "Cancel an owned queued or running Video Studio diagnostic render job.", Category: "studio", Enabled: t.service != nil, Version: "1", Risk: RiskMedium, SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 16384, Parameters: json.RawMessage(`{"type":"object","required":["render_job_id"],"properties":{"render_job_id":{"type":"string"}}}`)}
}
func (t *VideoRenderJobCancelTool) Validate(raw json.RawMessage) error {
	var input struct {
		JobID string `json:"render_job_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.JobID) == "" {
		return fmt.Errorf("render_job_id is required")
	}
	return nil
}
func (t *VideoRenderJobCancelTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var input struct {
		JobID string `json:"render_job_id"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	job, err := t.service.CancelRenderJob(InvocationScopeFromContext(ctx).UserID, input.JobID)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(job)
	return &ToolResult{Content: fmt.Sprintf("Render job %s is %s.", job.ID, job.Status), Structured: structured}, nil
}
