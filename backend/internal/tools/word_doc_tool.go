package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// WordDocGenerator is the interface for converting Markdown to .docx.
// Implemented by wordgen.Generator; declared here to keep the tool testable
// without importing the wordgen package.
type WordDocGenerator interface {
	Generate(markdownContent, filename string) (storagePath string, bytes int64, err error)
}

// WordDocAttachmentCreator is the minimal attachment-repo interface needed by WordDocTool.
type WordDocAttachmentCreator interface {
	Create(a *models.Attachment) error
}

// WordDocTool generates a Word (.docx) document from Markdown content.
// It saves the file via WordDocGenerator, registers an Attachment record, and
// returns a download URL through ToolResult.Metadata.
type WordDocTool struct {
	generator  WordDocGenerator
	attachRepo WordDocAttachmentCreator
}

// NewWordDocTool creates a WordDocTool.
func NewWordDocTool(generator WordDocGenerator, attachRepo WordDocAttachmentCreator) *WordDocTool {
	return &WordDocTool{
		generator:  generator,
		attachRepo: attachRepo,
	}
}

type wordDocArgs struct {
	Content        string `json:"content"`         // Markdown content to convert
	Filename       string `json:"filename"`        // Optional desired filename, e.g. "report.docx"
	ConversationID string `json:"conversation_id"` // Owning conversation ID
}

func (t *WordDocTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {
				"type": "string",
				"description": "The full Markdown content to render as a Word document."
			},
			"filename": {
				"type": "string",
				"description": "Optional filename for the Word document, e.g. 'report.docx'. Defaults to a timestamped name."
			},
			"conversation_id": {
				"type": "string",
				"description": "The ID of the conversation this document belongs to."
			}
		},
		"required": ["content", "conversation_id"]
	}`)

	return ToolDefinition{
		Name:        "generate_word_doc",
		Description: "Convert Markdown content into a properly formatted Word (.docx) document and return a download link.",
		Parameters:  schema,
		Category:    "document",
		Enabled:     true,
	}
}

func (t *WordDocTool) Validate(args json.RawMessage) error {
	var a wordDocArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.Content == "" {
		return fmt.Errorf("content is required")
	}
	if a.ConversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	return nil
}

func (t *WordDocTool) Execute(_ context.Context, args json.RawMessage) (*ToolResult, error) {
	var a wordDocArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	// Generate the .docx file.
	storagePath, bytes, err := t.generator.Generate(a.Content, a.Filename)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Failed to generate Word document: %v", err),
			IsError: true,
		}, nil
	}

	// Register the attachment in the database.
	attachment := &models.Attachment{
		ConversationID: a.ConversationID,
		Type:           "file",
		MimeType:       "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		StoragePath:    storagePath,
		Bytes:          bytes,
		CreatedAt:      time.Now().UTC(),
	}
	if err := t.attachRepo.Create(attachment); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Document generated but failed to save record: %v", err),
			IsError: true,
		}, nil
	}

	downloadURL := fmt.Sprintf("/v1/attachments/%s/download", attachment.ID)
	markdownLink := fmt.Sprintf("[📄 Download %s](%s)", storagePath, downloadURL)

	return &ToolResult{
		Content: markdownLink,
		Metadata: map[string]interface{}{
			"attachment_id": attachment.ID,
			"download_url":  downloadURL,
			"filename":      storagePath,
			"bytes":         bytes,
		},
	}, nil
}
