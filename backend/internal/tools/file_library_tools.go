package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/filelibrary"
)

// FileSearchTool provides file-library search as a callable tool.
type FileSearchTool struct {
	svc            *filelibrary.LibraryService
	ownerUserID    string
	conversationID string
	workspaceID    string
}

func NewFileSearchTool(svc *filelibrary.LibraryService, ownerUserID, conversationID, workspaceID string) *FileSearchTool {
	return &FileSearchTool{svc: svc, ownerUserID: ownerUserID, conversationID: conversationID, workspaceID: workspaceID}
}

type fileSearchArgs struct {
	Query            string   `json:"query"`
	Scope            string   `json:"scope,omitempty"`
	ConversationID   string   `json:"conversation_id,omitempty"`
	WorkspaceID      string   `json:"workspace_id,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	FileTypeFilter   []string `json:"file_type_filter,omitempty"`
	SourceFilter     []string `json:"source_filter,omitempty"`
	RequireCitations bool     `json:"require_citations,omitempty"`
}

func (t *FileSearchTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "The user question or search query."},
			"scope": {"type": "string", "enum": ["auto", "conversation", "workspace", "global", "all"], "default": "auto"},
			"conversation_id": {"type": "string"},
			"workspace_id": {"type": "string"},
			"top_k": {"type": "integer", "minimum": 1, "maximum": 30, "default": 8},
			"file_type_filter": {"type": "array", "items": {"type": "string"}},
			"source_filter": {"type": "array", "items": {"type": "string"}},
			"require_citations": {"type": "boolean", "default": true}
		},
		"required": ["query"]
	}`)
	return ToolDefinition{
		Name:        "file_search",
		Description: "Searches the user's indexed file library and returns relevant cited snippets from uploaded files.",
		Parameters:  schema,
		Category:    "search",
		Enabled:     true,
	}
}

func (t *FileSearchTool) Validate(args json.RawMessage) error {
	var a fileSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

func (t *FileSearchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	conversationID := a.ConversationID
	if conversationID == "" {
		conversationID = t.conversationID
	}
	workspaceID := a.WorkspaceID
	if workspaceID == "" {
		workspaceID = t.workspaceID
	}
	req := filelibrary.SearchRequest{
		OwnerUserID:      t.ownerUserID,
		Query:            a.Query,
		Scope:            a.Scope,
		ConversationID:   conversationID,
		WorkspaceID:      workspaceID,
		TopK:             a.TopK,
		FileTypeFilter:   a.FileTypeFilter,
		SourceFilter:     a.SourceFilter,
		RequireCitations: a.RequireCitations,
	}
	resp, err := t.svc.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("file search failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false, Metadata: map[string]interface{}{"result_count": len(resp.Results)}}, nil
}

// FileFetchTool fetches file metadata and optional full text.
type FileFetchTool struct {
	svc         *filelibrary.LibraryService
	ownerUserID string
}

func NewFileFetchTool(svc *filelibrary.LibraryService, ownerUserID string) *FileFetchTool {
	return &FileFetchTool{svc: svc, ownerUserID: ownerUserID}
}

type fileFetchArgs struct {
	LibraryFileID   string `json:"library_file_id,omitempty"`
	ChunkID         string `json:"chunk_id,omitempty"`
	IncludeFullText bool   `json:"include_full_text,omitempty"`
	MaxChars        int    `json:"max_chars,omitempty"`
}

func (t *FileFetchTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"library_file_id": {"type": "string"},
			"chunk_id": {"type": "string"},
			"include_full_text": {"type": "boolean", "default": false},
			"max_chars": {"type": "integer", "default": 20000}
		}
	}`)
	return ToolDefinition{Name: "file_fetch", Description: "Fetches an indexed file or chunk with metadata and text.", Parameters: schema, Category: "fetch", Enabled: true}
}

func (t *FileFetchTool) Validate(args json.RawMessage) error {
	var a fileFetchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.LibraryFileID == "" && a.ChunkID == "" {
		return fmt.Errorf("library_file_id or chunk_id is required")
	}
	return nil
}

func (t *FileFetchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileFetchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	resp, err := t.svc.Fetch(ctx, filelibrary.FetchRequest{
		OwnerUserID:     t.ownerUserID,
		LibraryFileID:   a.LibraryFileID,
		ChunkID:         a.ChunkID,
		IncludeFullText: a.IncludeFullText,
		MaxChars:        a.MaxChars,
	})
	if err != nil {
		return nil, fmt.Errorf("file fetch failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false}, nil
}

// FileIngestTool ingests an attachment into file library.
type FileIngestTool struct {
	svc            *filelibrary.LibraryService
	ownerUserID    string
	conversationID string
	workspaceID    string
}

func NewFileIngestTool(svc *filelibrary.LibraryService, ownerUserID, conversationID, workspaceID string) *FileIngestTool {
	return &FileIngestTool{svc: svc, ownerUserID: ownerUserID, conversationID: conversationID, workspaceID: workspaceID}
}

type fileIngestArgs struct {
	AttachmentID   string                 `json:"attachment_id,omitempty"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Scope          string                 `json:"scope,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

func (t *FileIngestTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"attachment_id": {"type": "string"},
			"display_name": {"type": "string"},
			"scope": {"type": "string", "enum": ["conversation", "workspace", "global"], "default": "conversation"},
			"conversation_id": {"type": "string"},
			"workspace_id": {"type": "string"},
			"metadata": {"type": "object"}
		}
	}`)
	return ToolDefinition{Name: "file_ingest", Description: "Indexes an attachment into the file library.", Parameters: schema, Category: "ingest", Enabled: true}
}

func (t *FileIngestTool) Validate(args json.RawMessage) error {
	var a fileIngestArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.AttachmentID == "" {
		return fmt.Errorf("attachment_id is required")
	}
	return nil
}

func (t *FileIngestTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileIngestArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	conversationID := a.ConversationID
	if conversationID == "" {
		conversationID = t.conversationID
	}
	workspaceID := a.WorkspaceID
	if workspaceID == "" {
		workspaceID = t.workspaceID
	}
	resp, err := t.svc.IngestFile(ctx, filelibrary.IngestFileRequest{
		OwnerUserID:    t.ownerUserID,
		AttachmentID:   a.AttachmentID,
		Scope:          a.Scope,
		ConversationID: conversationID,
		WorkspaceID:    workspaceID,
		DisplayName:    a.DisplayName,
		Metadata:       a.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("file ingest failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false}, nil
}

// FileSummarizeTool summarizes one or more indexed files.
type FileSummarizeTool struct {
	svc         *filelibrary.LibraryService
	ownerUserID string
}

func NewFileSummarizeTool(svc *filelibrary.LibraryService, ownerUserID string) *FileSummarizeTool {
	return &FileSummarizeTool{svc: svc, ownerUserID: ownerUserID}
}

type fileSummarizeArgs struct {
	LibraryFileIDs  []string `json:"library_file_ids"`
	Query           string   `json:"query,omitempty"`
	SummaryStyle    string   `json:"summary_style,omitempty"`
	MaxCharsPerFile int      `json:"max_chars_per_file,omitempty"`
}

func (t *FileSummarizeTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"library_file_ids": {"type": "array", "items": {"type": "string"}},
			"query": {"type": "string"},
			"summary_style": {"type": "string", "enum": ["brief", "detailed", "executive", "technical", "qa"], "default": "detailed"},
			"max_chars_per_file": {"type": "integer", "default": 50000}
		},
		"required": ["library_file_ids"]
	}`)
	return ToolDefinition{Name: "file_summarize", Description: "Summarizes one or more indexed files with citations.", Parameters: schema, Category: "summarize", Enabled: true}
}

func (t *FileSummarizeTool) Validate(args json.RawMessage) error {
	var a fileSummarizeArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(a.LibraryFileIDs) == 0 {
		return fmt.Errorf("library_file_ids is required")
	}
	return nil
}

func (t *FileSummarizeTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileSummarizeArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	resp, err := t.svc.Summarize(ctx, filelibrary.SummarizeRequest{
		OwnerUserID:     t.ownerUserID,
		LibraryFileIDs:  a.LibraryFileIDs,
		Query:           a.Query,
		SummaryStyle:    a.SummaryStyle,
		MaxCharsPerFile: a.MaxCharsPerFile,
	})
	if err != nil {
		return nil, fmt.Errorf("file summarize failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false}, nil
}

// FileCompareTool compares two or more indexed files.
type FileCompareTool struct {
	svc         *filelibrary.LibraryService
	ownerUserID string
}

func NewFileCompareTool(svc *filelibrary.LibraryService, ownerUserID string) *FileCompareTool {
	return &FileCompareTool{svc: svc, ownerUserID: ownerUserID}
}

type fileCompareArgs struct {
	LibraryFileIDs  []string `json:"library_file_ids"`
	ComparisonGoal  string   `json:"comparison_goal,omitempty"`
	OutputFormat    string   `json:"output_format,omitempty"`
	MaxCharsPerFile int      `json:"max_chars_per_file,omitempty"`
}

func (t *FileCompareTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"library_file_ids": {"type": "array", "items": {"type": "string"}, "minItems": 2},
			"comparison_goal": {"type": "string"},
			"output_format": {"type": "string", "enum": ["markdown", "table", "executive_summary"], "default": "markdown"},
			"max_chars_per_file": {"type": "integer", "default": 50000}
		},
		"required": ["library_file_ids"]
	}`)
	return ToolDefinition{Name: "file_compare", Description: "Compares indexed files and returns differences with citations.", Parameters: schema, Category: "compare", Enabled: true}
}

func (t *FileCompareTool) Validate(args json.RawMessage) error {
	var a fileCompareArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(a.LibraryFileIDs) < 2 {
		return fmt.Errorf("at least two library_file_ids are required")
	}
	return nil
}

func (t *FileCompareTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileCompareArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	resp, err := t.svc.Compare(ctx, filelibrary.CompareRequest{
		OwnerUserID:     t.ownerUserID,
		LibraryFileIDs:  a.LibraryFileIDs,
		ComparisonGoal:  a.ComparisonGoal,
		OutputFormat:    a.OutputFormat,
		MaxCharsPerFile: a.MaxCharsPerFile,
	})
	if err != nil {
		return nil, fmt.Errorf("file compare failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false}, nil
}

// FileDeleteTool deletes an indexed file and its vectors/chunks.
type FileDeleteTool struct {
	svc         *filelibrary.LibraryService
	ownerUserID string
}

func NewFileDeleteTool(svc *filelibrary.LibraryService, ownerUserID string) *FileDeleteTool {
	return &FileDeleteTool{svc: svc, ownerUserID: ownerUserID}
}

type fileDeleteArgs struct {
	LibraryFileID string `json:"library_file_id"`
	HardDelete    bool   `json:"hard_delete,omitempty"`
}

func (t *FileDeleteTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"library_file_id": {"type": "string"},
			"hard_delete": {"type": "boolean", "default": false}
		},
		"required": ["library_file_id"]
	}`)
	return ToolDefinition{Name: "file_delete", Description: "Deletes an indexed file and removes associated index artifacts.", Parameters: schema, Category: "manage", Enabled: true}
}

func (t *FileDeleteTool) Validate(args json.RawMessage) error {
	var a fileDeleteArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.LibraryFileID == "" {
		return fmt.Errorf("library_file_id is required")
	}
	return nil
}

func (t *FileDeleteTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileDeleteArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	if err := t.svc.DeleteFile(ctx, filelibrary.DeleteFileRequest{
		OwnerUserID:   t.ownerUserID,
		LibraryFileID: a.LibraryFileID,
		HardDelete:    a.HardDelete,
	}); err != nil {
		return nil, fmt.Errorf("file delete failed: %w", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{"deleted": true, "library_file_id": a.LibraryFileID, "hard_delete": a.HardDelete})
	return &ToolResult{Content: string(payload), IsError: false}, nil
}

// FileReindexTool reindexes an existing indexed file.
type FileReindexTool struct {
	svc         *filelibrary.LibraryService
	ownerUserID string
}

func NewFileReindexTool(svc *filelibrary.LibraryService, ownerUserID string) *FileReindexTool {
	return &FileReindexTool{svc: svc, ownerUserID: ownerUserID}
}

type fileReindexArgs struct {
	LibraryFileID string `json:"library_file_id"`
}

func (t *FileReindexTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"library_file_id": {"type": "string"}
		},
		"required": ["library_file_id"]
	}`)
	return ToolDefinition{Name: "file_reindex", Description: "Re-extracts and reindexes an existing file library entry.", Parameters: schema, Category: "manage", Enabled: true}
}

func (t *FileReindexTool) Validate(args json.RawMessage) error {
	var a fileReindexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.LibraryFileID == "" {
		return fmt.Errorf("library_file_id is required")
	}
	return nil
}

func (t *FileReindexTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fileReindexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	resp, err := t.svc.ReindexFile(ctx, filelibrary.ReindexFileRequest{
		OwnerUserID:   t.ownerUserID,
		LibraryFileID: a.LibraryFileID,
	})
	if err != nil {
		return nil, fmt.Errorf("file reindex failed: %w", err)
	}
	payload, _ := json.Marshal(resp)
	return &ToolResult{Content: string(payload), IsError: false}, nil
}
