package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/jobs"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

// ImageGenerateJobTool starts image generation as durable asynchronous work.
type ImageGenerateJobTool struct {
	manager     *jobs.Manager
	llm         *llm.Service
	attachments *repository.AttachmentRepo
	storageDir  string
}

func NewImageGenerateJobTool(manager *jobs.Manager, llmService *llm.Service, attachments *repository.AttachmentRepo, storageDir string) *ImageGenerateJobTool {
	return &ImageGenerateJobTool{manager: manager, llm: llmService, attachments: attachments, storageDir: storageDir}
}

func (t *ImageGenerateJobTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "image_generate", Description: "Start an asynchronous image generation job and save generated images as conversation attachments.",
		Category: "studio", Enabled: t.manager != nil && t.llm != nil && t.attachments != nil && t.storageDir != "",
		Version: "1", Risk: RiskMedium, SideEffecting: true, RequiresNetwork: true,
		DefaultTimeoutMS: 5000, MaxResultBytes: 32768,
		Parameters: json.RawMessage(`{
			"type":"object","required":["prompt"],
			"properties":{
				"prompt":{"type":"string","minLength":1,"maxLength":12000},
				"provider":{"type":"string"},"model":{"type":"string"},
				"size":{"type":"string"},"quality":{"type":"string"},
				"n":{"type":"integer","minimum":1,"maximum":4,"default":1}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"},"status":{"type":"string"}}}`),
		Examples:     []ToolExample{{Description: "Generate one image", Arguments: json.RawMessage(`{"prompt":"A modern local-first AI studio workspace","size":"1024x1024","n":1}`)}},
	}
}

type imageJobArgs struct {
	Prompt   string `json:"prompt"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Size     string `json:"size"`
	Quality  string `json:"quality"`
	N        int    `json:"n"`
}

func (t *ImageGenerateJobTool) Validate(raw json.RawMessage) error {
	var args imageJobArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if len(args.Prompt) > 12000 {
		return fmt.Errorf("prompt exceeds 12000 characters")
	}
	if args.N < 0 || args.N > 4 {
		return fmt.Errorf("n must be between 1 and 4")
	}
	return nil
}

func (t *ImageGenerateJobTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args imageJobArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.N == 0 {
		args.N = 1
	}
	scope := InvocationScopeFromContext(ctx)
	if scope.ConversationID == "" {
		return nil, fmt.Errorf("image generation requires a conversation scope")
	}
	job, err := t.manager.Start("image_generate", jobs.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, args, func(jobCtx context.Context, progress jobs.Progress) (interface{}, error) {
		progress("requesting provider", 0.1)
		response, err := t.llm.ImageGenerate(jobCtx, llm.ImageRequest{
			Provider: args.Provider, Model: args.Model, Prompt: args.Prompt,
			Size: args.Size, Quality: args.Quality, N: args.N,
		})
		if err != nil {
			return nil, err
		}
		progress("saving generated images", 0.75)
		artifacts := make([]ToolArtifact, 0, len(response.Images))
		attachments := make([]models.Attachment, 0, len(response.Images))
		for index, image := range response.Images {
			if jobCtx.Err() != nil {
				return nil, jobCtx.Err()
			}
			data, err := base64.StdEncoding.DecodeString(image.B64JSON)
			if err != nil {
				return nil, fmt.Errorf("decode generated image %d: %w", index+1, err)
			}
			filename := uuid.NewString() + ".png"
			path := filepath.Join(t.storageDir, filename)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return nil, fmt.Errorf("save generated image %d: %w", index+1, err)
			}
			metadata, _ := json.Marshal(map[string]interface{}{
				"revised_prompt": image.RevisedPrompt, "generator_model": response.Model,
				"generator_provider": response.Provider, "agent_tool": true,
			})
			attachment := models.Attachment{
				ConversationID: scope.ConversationID, Type: "image", MimeType: "image/png",
				StoragePath: filename, Bytes: int64(len(data)), CreatedAt: time.Now().UTC(), MetadataJSON: string(metadata),
			}
			if err := t.attachments.Create(&attachment); err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			attachments = append(attachments, attachment)
			artifacts = append(artifacts, ToolArtifact{
				ID: attachment.ID, Name: filename, MimeType: attachment.MimeType,
				URL: "/v1/attachments/" + attachment.ID + "/download", Bytes: attachment.Bytes,
			})
			progress("saving generated images", 0.75+0.2*float64(index+1)/float64(len(response.Images)))
		}
		return map[string]interface{}{
			"provider": response.Provider, "model": response.Model,
			"attachments": attachments, "artifacts": artifacts,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
	return &ToolResult{
		Content:    fmt.Sprintf("Image generation started as job %s. Use job_status with operation=await or get to retrieve the saved image attachments.", job.ID),
		Structured: structured, Metadata: map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind},
	}, nil
}
