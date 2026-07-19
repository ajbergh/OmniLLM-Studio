package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/jobs"
	"github.com/ajbergh/omnillm-studio/internal/music"
)

// MusicGenerateJobTool exposes Music Studio generation through the shared job runtime.
type MusicGenerateJobTool struct {
	manager *jobs.Manager
	service *music.Service
}

func NewMusicGenerateJobTool(manager *jobs.Manager, service *music.Service) *MusicGenerateJobTool {
	return &MusicGenerateJobTool{manager: manager, service: service}
}

func (t *MusicGenerateJobTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "music_generate", Description: "Start an asynchronous Music Studio generation with provider, genre, mood, instrumentation, structure, lyrics, and duration controls.",
		Category: "studio", Enabled: t.manager != nil && t.service != nil, Version: "1",
		Risk: RiskMedium, SideEffecting: true, RequiresNetwork: true,
		DefaultTimeoutMS: 5000, MaxResultBytes: 32768,
		Parameters: json.RawMessage(`{
			"type":"object","required":["provider","prompt"],
			"properties":{
				"provider":{"type":"string","enum":["openrouter","gemini","elevenlabs"]},
				"model":{"type":"string"},"prompt":{"type":"string","minLength":1,"maxLength":12000},
				"lyrics":{"type":"string"},"instrumental":{"type":"boolean"},"vocal_mode":{"type":"string"},
				"session_id":{"type":"string"},"parent_id":{"type":"string"},"title":{"type":"string"},
				"options":{"type":"object","properties":{
					"genre":{"type":"string"},"mood":{"type":"string"},"era":{"type":"string"},
					"instruments":{"type":"array","items":{"type":"string"}},"bpm":{"type":"integer","minimum":20,"maximum":300},
					"scale":{"type":"string"},"duration":{"type":"string"},"structure":{"type":"string"},
					"language":{"type":"string"},"energy_curve":{"type":"string"},
					"production_notes":{"type":"string"},"negative_steer":{"type":"string"}
				}}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"},"status":{"type":"string"}}}`),
	}
}

func (t *MusicGenerateJobTool) Validate(raw json.RawMessage) error {
	var request music.GenerateRequest
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
	if request.Options.BPM != nil && (*request.Options.BPM < 20 || *request.Options.BPM > 300) {
		return fmt.Errorf("bpm must be between 20 and 300")
	}
	return nil
}

func (t *MusicGenerateJobTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var request music.GenerateRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	scope := InvocationScopeFromContext(ctx)
	job, err := t.manager.Start("music_generate", jobs.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, request, func(jobCtx context.Context, progress jobs.Progress) (interface{}, error) {
		progress("preparing music generation", 0.05)
		session, generation, asset, err := t.service.Generate(jobCtx, scope.UserID, request, func(update music.GenerationProgress) {
			switch update.Stage {
			case "started":
				progress(update.Message, 0.2)
			case "done":
				progress(update.Message, 0.95)
			default:
				progress(update.Message, 0.5)
			}
		})
		if err != nil {
			return map[string]interface{}{"session": session, "generation": generation}, err
		}
		result := map[string]interface{}{
			"session": session, "generation": generation, "asset": asset,
		}
		if asset != nil {
			result["artifact"] = ToolArtifact{
				ID: asset.ID, Name: asset.FileName, MimeType: asset.MimeType,
				URL: "/v1/music/assets/" + asset.ID + "/download", Bytes: asset.SizeBytes,
			}
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
	return &ToolResult{
		Content: fmt.Sprintf("Music generation started as job %s. Use job_status to monitor it and retrieve the resulting Music Studio asset.", job.ID),
		Structured: structured, Metadata: map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind},
	}, nil
}
