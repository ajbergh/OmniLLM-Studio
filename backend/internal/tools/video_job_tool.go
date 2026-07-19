package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/jobs"
	"github.com/ajbergh/omnillm-studio/internal/video"
)

// VideoGenerateJobTool exposes Video Studio generation through durable jobs.
type VideoGenerateJobTool struct {
	manager *jobs.Manager
	service *video.Service
}

func NewVideoGenerateJobTool(manager *jobs.Manager, service *video.Service) *VideoGenerateJobTool {
	return &VideoGenerateJobTool{manager: manager, service: service}
}

func (t *VideoGenerateJobTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "video_generate", Description: "Start an asynchronous Video Studio generation from text, image assets, source video, first/last frames, or reference assets.",
		Category: "studio", Enabled: t.manager != nil && t.service != nil, Version: "1",
		Risk: RiskMedium, SideEffecting: true, RequiresNetwork: true,
		DefaultTimeoutMS: 5000, MaxResultBytes: 32768,
		Parameters: json.RawMessage(`{
			"type":"object","required":["provider","prompt"],
			"properties":{
				"project_id":{"type":"string"},"title":{"type":"string"},
				"provider":{"type":"string","enum":["openrouter","gemini","luma"]},"model":{"type":"string"},
				"prompt":{"type":"string","minLength":1,"maxLength":12000},
				"generation_mode":{"type":"string"},"enhance":{"type":"boolean"},"negative_prompt":{"type":"string"},
				"aspect_ratio":{"type":"string"},"duration_seconds":{"type":"integer","minimum":1,"maximum":60},
				"resolution":{"type":"string"},"fps":{"type":"integer","minimum":1,"maximum":120},
				"start_image_asset_id":{"type":"string"},"last_frame_asset_id":{"type":"string"},
				"source_video_asset_id":{"type":"string"},
				"reference_asset_ids":{"type":"array","items":{"type":"string"},"maxItems":8},
				"camera_motion":{"type":"string"},"shot_type":{"type":"string"},"style_preset":{"type":"string"},
				"composition":{"type":"string"},"lens_effect":{"type":"string"},"lighting":{"type":"string"},
				"dialogue":{"type":"string"},"sound_effects":{"type":"string"},"ambient_noise":{"type":"string"},
				"continuity_notes":{"type":"string"},"production_notes":{"type":"string"},
				"place_on_timeline":{"type":"boolean"}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"},"status":{"type":"string"}}}`),
	}
}

func (t *VideoGenerateJobTool) Validate(raw json.RawMessage) error {
	var request video.GenerateRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return err
	}
	if strings.TrimSpace(request.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if len(request.Prompt) > 12000 {
		return fmt.Errorf("prompt exceeds 12000 characters")
	}
	if request.DurationSeconds < 0 || request.DurationSeconds > 60 {
		return fmt.Errorf("duration_seconds must be between 1 and 60")
	}
	if len(request.ReferenceAssetIDs) > 8 {
		return fmt.Errorf("reference_asset_ids supports at most 8 items")
	}
	return nil
}

func (t *VideoGenerateJobTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var request video.GenerateRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	scope := InvocationScopeFromContext(ctx)
	job, err := t.manager.Start("video_generate", jobs.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, request, func(jobCtx context.Context, progress jobs.Progress) (interface{}, error) {
		progress("validating video generation", 0.05)
		project, generation, asset, err := t.service.Generate(jobCtx, scope.UserID, request, func(update video.GenerationProgress) {
			fraction := update.Progress
			if fraction <= 0 {
				switch update.Stage {
				case "started":
					fraction = 0.15
				case "done":
					fraction = 0.95
				default:
					fraction = 0.5
				}
			}
			progress(update.Message, fraction)
		})
		if err != nil {
			return map[string]interface{}{"project": project, "generation": generation}, err
		}
		result := map[string]interface{}{"project": project, "generation": generation, "asset": asset}
		if asset != nil {
			result["artifact"] = ToolArtifact{
				ID: asset.ID, Name: asset.FileName, MimeType: asset.MimeType,
				URL: "/v1/video/assets/" + asset.ID + "/download", Bytes: asset.SizeBytes,
			}
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
	return &ToolResult{
		Content: fmt.Sprintf("Video generation started as job %s. Use job_status to monitor it and retrieve the resulting Video Studio project, generation, and asset.", job.ID),
		Structured: structured, Metadata: map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind},
	}, nil
}
