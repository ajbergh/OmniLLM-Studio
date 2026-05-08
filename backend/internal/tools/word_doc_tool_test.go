package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// ---- Mocks ----------------------------------------------------------------

type mockGenerator struct {
	storagePath string
	bytes       int64
	err         error
}

func (m *mockGenerator) Generate(content, filename string) (string, int64, error) {
	return m.storagePath, m.bytes, m.err
}

type mockAttachRepo struct {
	err error
	got *models.Attachment
}

func (m *mockAttachRepo) Create(a *models.Attachment) error {
	if m.err != nil {
		return m.err
	}
	// Simulate the repo assigning an ID.
	if a.ID == "" {
		a.ID = "test-attachment-id"
	}
	m.got = a
	return nil
}

// ---- Tests -----------------------------------------------------------------

func TestWordDocTool_Definition(t *testing.T) {
	tool := NewWordDocTool(&mockGenerator{}, &mockAttachRepo{})
	def := tool.Definition()

	if def.Name != "generate_word_doc" {
		t.Errorf("Name = %q; want generate_word_doc", def.Name)
	}
	if !def.Enabled {
		t.Error("Enabled = false; want true")
	}
	if def.Category != "document" {
		t.Errorf("Category = %q; want document", def.Category)
	}
}

func TestWordDocTool_Validate(t *testing.T) {
	tool := NewWordDocTool(&mockGenerator{}, &mockAttachRepo{})

	valid := wordDocArgs{Content: "# Hello", ConversationID: "conv-1"}
	b, _ := json.Marshal(valid)
	if err := tool.Validate(b); err != nil {
		t.Errorf("expected no error for valid args; got %v", err)
	}

	noContent := wordDocArgs{Content: "", ConversationID: "conv-1"}
	b, _ = json.Marshal(noContent)
	if err := tool.Validate(b); err == nil {
		t.Error("expected error for empty content; got nil")
	}

	noConvo := wordDocArgs{Content: "# Hi", ConversationID: ""}
	b, _ = json.Marshal(noConvo)
	if err := tool.Validate(b); err == nil {
		t.Error("expected error for empty conversation_id; got nil")
	}
}

func TestWordDocTool_Execute_Success(t *testing.T) {
	gen := &mockGenerator{storagePath: "report.docx", bytes: 1234}
	repo := &mockAttachRepo{}
	tool := NewWordDocTool(gen, repo)

	args := wordDocArgs{Content: "# Report\n\nHello", Filename: "report.docx", ConversationID: "conv-1"}
	b, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), b)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true; content = %q", result.Content)
	}
	if result.Metadata["attachment_id"] == "" {
		t.Error("attachment_id missing from metadata")
	}
	if result.Metadata["download_url"] == "" {
		t.Error("download_url missing from metadata")
	}

	// Download URL should contain the attachment ID.
	wantURLSuffix := "/download"
	url, _ := result.Metadata["download_url"].(string)
	if len(url) == 0 || url[len(url)-len(wantURLSuffix):] != wantURLSuffix {
		t.Errorf("download_url = %q; want suffix %q", url, wantURLSuffix)
	}
}

func TestWordDocTool_Execute_GeneratorError(t *testing.T) {
	gen := &mockGenerator{err: errors.New("disk full")}
	repo := &mockAttachRepo{}
	tool := NewWordDocTool(gen, repo)

	args := wordDocArgs{Content: "# Hi", ConversationID: "conv-1"}
	b, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), b)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false; expected generator error to be surfaced")
	}
}

func TestWordDocTool_Execute_RepoError(t *testing.T) {
	gen := &mockGenerator{storagePath: "doc.docx", bytes: 500}
	repo := &mockAttachRepo{err: errors.New("db unavailable")}
	tool := NewWordDocTool(gen, repo)

	args := wordDocArgs{Content: "# Hi", ConversationID: "conv-1"}
	b, _ := json.Marshal(args)

	result, err := tool.Execute(context.Background(), b)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false; expected repo error to be surfaced")
	}
}
