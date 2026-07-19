package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/artifacts"
	"github.com/ajbergh/omnillm-studio/internal/jobs"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// ArtifactGenerateJobTool converts finished content into a durable downloadable artifact.
type ArtifactGenerateJobTool struct {
	manager     *jobs.Manager
	generator   *artifacts.Generator
	attachments *repository.AttachmentRepo
}

func NewArtifactGenerateJobTool(manager *jobs.Manager, generator *artifacts.Generator, attachments *repository.AttachmentRepo) *ArtifactGenerateJobTool {
	return &ArtifactGenerateJobTool{manager: manager, generator: generator, attachments: attachments}
}

func (t *ArtifactGenerateJobTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "artifact_generate", Description: "Create a downloadable document or data artifact from completed content and save it to the current conversation.",
		Category: "studio", Enabled: t.manager != nil && t.generator != nil && t.attachments != nil,
		Version: "1", Risk: RiskMedium, SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 32768,
		Parameters: json.RawMessage(`{
			"type":"object","required":["content","format"],
			"properties":{
				"content":{"type":"string","minLength":1,"maxLength":500000},
				"format":{"type":"string","enum":["pdf","xlsx","csv","markdown","html","json","yaml"]},
				"filename":{"type":"string","maxLength":200}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"},"status":{"type":"string"}}}`),
	}
}

type artifactJobArgs struct {
	Content  string `json:"content"`
	Format   string `json:"format"`
	Filename string `json:"filename"`
}

func (t *ArtifactGenerateJobTool) Validate(raw json.RawMessage) error {
	var args artifactJobArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Content) == "" {
		return fmt.Errorf("content is required")
	}
	if len(args.Content) > 500000 {
		return fmt.Errorf("content exceeds 500000 characters")
	}
	switch strings.ToLower(args.Format) {
	case "pdf", "xlsx", "csv", "markdown", "html", "json", "yaml":
	default:
		return fmt.Errorf("unsupported artifact format %q", args.Format)
	}
	return nil
}

func (t *ArtifactGenerateJobTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args artifactJobArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	scope := InvocationScopeFromContext(ctx)
	if scope.ConversationID == "" {
		return nil, fmt.Errorf("artifact generation requires a conversation scope")
	}
	format := artifacts.ArtifactFormat(strings.ToLower(args.Format))
	filename := strings.TrimSpace(args.Filename)
	if filename == "" {
		filename = artifacts.SuggestFilename("agent artifact", format)
	}
	job, err := t.manager.Start("artifact_generate", jobs.Scope{
		UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID,
	}, args, func(jobCtx context.Context, progress jobs.Progress) (interface{}, error) {
		progress("rendering artifact", 0.2)
		storagePath, bytes, contentType, err := t.generator.Generate(jobCtx, args.Content, format, filename)
		if err != nil {
			return nil, err
		}
		progress("registering artifact", 0.85)
		attachment := models.Attachment{
			ConversationID: scope.ConversationID, Type: "file", MimeType: contentType,
			StoragePath: storagePath, Bytes: bytes, CreatedAt: time.Now().UTC(),
		}
		if err := t.attachments.Create(&attachment); err != nil {
			return nil, err
		}
		artifact := ToolArtifact{
			ID: attachment.ID, Name: storagePath, MimeType: contentType,
			URL: "/v1/attachments/" + attachment.ID + "/download", Bytes: bytes,
		}
		return map[string]interface{}{"attachment": attachment, "artifact": artifact, "format": format}, nil
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
	return &ToolResult{
		Content:    fmt.Sprintf("Artifact generation started as job %s. Use job_status to retrieve the downloadable attachment.", job.ID),
		Structured: structured, Metadata: map[string]interface{}{"job_id": job.ID, "status": job.Status, "kind": job.Kind},
	}, nil
}
