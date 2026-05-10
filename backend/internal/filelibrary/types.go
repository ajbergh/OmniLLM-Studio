package filelibrary

import (
	"context"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// TimeFilter represents optional date bounds for file searches.
type TimeFilter struct {
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// FileIntent captures deterministic preflight routing hints for file-grounded requests.
type FileIntent struct {
	RequiresFileSearch bool        `json:"requires_file_search"`
	SearchQuery        string      `json:"search_query,omitempty"`
	Scope              string      `json:"scope,omitempty"`
	FileTypeHints      []string    `json:"file_type_hints,omitempty"`
	TimeHints          *TimeFilter `json:"time_hints,omitempty"`
	CompareIntent      bool        `json:"compare_intent"`
	SummarizeIntent    bool        `json:"summarize_intent"`
	FetchSpecificFile  bool        `json:"fetch_specific_file"`
	Confidence         float64     `json:"confidence"`
	Reason             string      `json:"reason,omitempty"`
}

// IngestFileRequest describes an ingestion request for a library file.
type IngestFileRequest struct {
	OwnerUserID    string                 `json:"owner_user_id,omitempty"`
	AttachmentID   string                 `json:"attachment_id,omitempty"`
	Scope          string                 `json:"scope,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	WorkspaceID    string                 `json:"workspace_id,omitempty"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRequest describes a file-library search request.
type SearchRequest struct {
	OwnerUserID      string      `json:"owner_user_id,omitempty"`
	Query            string      `json:"query"`
	Scope            string      `json:"scope,omitempty"`
	ConversationID   string      `json:"conversation_id,omitempty"`
	WorkspaceID      string      `json:"workspace_id,omitempty"`
	TopK             int         `json:"top_k,omitempty"`
	FileTypeFilter   []string    `json:"file_type_filter,omitempty"`
	SourceFilter     []string    `json:"source_filter,omitempty"`
	TimeFilter       *TimeFilter `json:"time_filter,omitempty"`
	RequireCitations bool        `json:"require_citations,omitempty"`
}

// FetchRequest describes a fetch request by file or chunk id.
type FetchRequest struct {
	OwnerUserID     string `json:"owner_user_id,omitempty"`
	LibraryFileID   string `json:"library_file_id,omitempty"`
	ChunkID         string `json:"chunk_id,omitempty"`
	IncludeFullText bool   `json:"include_full_text,omitempty"`
	MaxChars        int    `json:"max_chars,omitempty"`
}

// SummarizeRequest describes a request to summarize one or more files.
type SummarizeRequest struct {
	OwnerUserID     string   `json:"owner_user_id,omitempty"`
	LibraryFileIDs  []string `json:"library_file_ids"`
	Query           string   `json:"query,omitempty"`
	SummaryStyle    string   `json:"summary_style,omitempty"`
	MaxCharsPerFile int      `json:"max_chars_per_file,omitempty"`
	ConversationID  string   `json:"conversation_id,omitempty"`
}

// CompareRequest describes a request to compare multiple files.
type CompareRequest struct {
	OwnerUserID     string   `json:"owner_user_id,omitempty"`
	LibraryFileIDs  []string `json:"library_file_ids"`
	ComparisonGoal  string   `json:"comparison_goal,omitempty"`
	OutputFormat    string   `json:"output_format,omitempty"`
	MaxCharsPerFile int      `json:"max_chars_per_file,omitempty"`
	ConversationID  string   `json:"conversation_id,omitempty"`
}

// UpdateFileRequest describes metadata updates for a library file.
type UpdateFileRequest struct {
	OwnerUserID   string                 `json:"owner_user_id,omitempty"`
	LibraryFileID string                 `json:"library_file_id"`
	DisplayName   *string                `json:"display_name,omitempty"`
	Scope         *string                `json:"scope,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// DeleteFileRequest describes a delete operation for a library file.
type DeleteFileRequest struct {
	OwnerUserID   string `json:"owner_user_id,omitempty"`
	LibraryFileID string `json:"library_file_id"`
	HardDelete    bool   `json:"hard_delete,omitempty"`
}

// ReindexFileRequest describes a reindex operation for a single file.
type ReindexFileRequest struct {
	OwnerUserID   string `json:"owner_user_id,omitempty"`
	LibraryFileID string `json:"library_file_id"`
}

// FileCitation references a specific file/chunk for UI and answer citations.
type FileCitation struct {
	Label        string `json:"label"`
	FileID       string `json:"file_id"`
	ChunkID      string `json:"chunk_id"`
	PageNumber   *int   `json:"page_number,omitempty"`
	SectionTitle string `json:"section_title,omitempty"`
}

// FileSearchResult is a single citation-ready search match.
type FileSearchResult struct {
	ChunkID       string       `json:"chunk_id"`
	LibraryFileID string       `json:"library_file_id"`
	FileName      string       `json:"file_name"`
	DisplayName   string       `json:"display_name"`
	MimeType      string       `json:"mime_type"`
	Scope         string       `json:"scope"`
	SourceType    string       `json:"source_type"`
	SourceURL     string       `json:"source_url,omitempty"`
	PageNumber    *int         `json:"page_number,omitempty"`
	SectionTitle  string       `json:"section_title,omitempty"`
	Snippet       string       `json:"snippet"`
	Score         float64      `json:"score"`
	Citation      FileCitation `json:"citation"`
}

// SearchMetadata captures summary counters for debugging/telemetry.
type SearchMetadata struct {
	SearchedCollections []string `json:"searched_collections,omitempty"`
	VectorResults       int      `json:"vector_results"`
	KeywordResults      int      `json:"keyword_results"`
	MergedResults       int      `json:"merged_results"`
}

// SearchResponse returns results and ranking metadata.
type SearchResponse struct {
	Query    string             `json:"query"`
	Scope    string             `json:"scope"`
	Results  []FileSearchResult `json:"results"`
	Metadata SearchMetadata     `json:"metadata"`
}

// SummaryResponse returns a citation-aware summary across selected files.
type SummaryResponse struct {
	Summary string         `json:"summary"`
	Sources []FileCitation `json:"sources,omitempty"`
}

// CompareResponse returns a citation-aware comparison across selected files.
type CompareResponse struct {
	Comparison string         `json:"comparison"`
	Sources    []FileCitation `json:"sources,omitempty"`
}

// ReindexFileResponse returns summary information for a file reindex run.
type ReindexFileResponse struct {
	LibraryFileID    string `json:"library_file_id"`
	ChunksCreated    int    `json:"chunks_created"`
	EmbeddingsStored int    `json:"embeddings_stored"`
}

// FetchedFile is returned by Fetch.
type FetchedFile struct {
	File     *models.LibraryFile   `json:"file,omitempty"`
	Chunk    *models.DocumentChunk `json:"chunk,omitempty"`
	FullText string                `json:"full_text,omitempty"`
}

// Service defines file-library ingestion/search/fetch capabilities.
type Service interface {
	IngestFile(ctx context.Context, req IngestFileRequest) (*models.LibraryFile, error)
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
	Fetch(ctx context.Context, req FetchRequest) (*FetchedFile, error)
	ListFiles(ctx context.Context, ownerUserID string, scope string, query string) ([]models.LibraryFile, error)
	Summarize(ctx context.Context, req SummarizeRequest) (*SummaryResponse, error)
	Compare(ctx context.Context, req CompareRequest) (*CompareResponse, error)
	UpdateFile(ctx context.Context, req UpdateFileRequest) (*models.LibraryFile, error)
	DeleteFile(ctx context.Context, req DeleteFileRequest) error
	ReindexFile(ctx context.Context, req ReindexFileRequest) (*ReindexFileResponse, error)
}
