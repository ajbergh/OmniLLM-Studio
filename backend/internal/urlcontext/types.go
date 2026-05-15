package urlcontext

import "time"

// URLKind classifies the type of a URL.
type URLKind string

const (
	URLKindUnknown         URLKind = "unknown"
	URLKindWebPage         URLKind = "webpage"
	URLKindPDF             URLKind = "pdf"
	URLKindGitHubRepo      URLKind = "github_repo"
	URLKindGitHubFile      URLKind = "github_file"
	URLKindGitHubDirectory URLKind = "github_directory"
	URLKindGitHubRaw       URLKind = "github_raw"
)

// AnalysisGoal describes what the user wants to do with the linked source.
type AnalysisGoal string

const (
	GoalUnknown            AnalysisGoal = "unknown"
	GoalSummarize          AnalysisGoal = "summarize"
	GoalReview             AnalysisGoal = "review"
	GoalFeatureGapReview   AnalysisGoal = "feature_gap_review"
	GoalArchitectureReview AnalysisGoal = "architecture_review"
	GoalSecurityReview     AnalysisGoal = "security_review"
	GoalCodeReview         AnalysisGoal = "code_review"
	GoalExplain            AnalysisGoal = "explain"
	GoalCompare            AnalysisGoal = "compare"
)

// ResolveRequest is the input to Service.Resolve.
type ResolveRequest struct {
	ConversationID string
	UserMessage    string
	URLs           []string // if nil, URLs are extracted from UserMessage
	Force          bool     // bypass intent check
	StreamStatus   func(event string, payload any)
}

// ResolveResult is the output from Service.Resolve.
type ResolveResult struct {
	Handled               bool
	RequiresLLM           bool
	UsedDirectContext     bool
	UsedRAG               bool
	PromptContext         string
	Sources               []SourceRef
	ResolvedSources       []ResolvedSource
	Warnings              []string
	Metadata              map[string]any
	ShouldBypassWebSearch bool
}

// SourceRef is minimal source attribution stored in message metadata.
type SourceRef struct {
	ID               string    `json:"id"`
	URL              string    `json:"url"`
	FinalURL         string    `json:"final_url,omitempty"`
	Title            string    `json:"title,omitempty"`
	Kind             URLKind   `json:"kind"`
	Path             string    `json:"path,omitempty"`
	FetchedAt        time.Time `json:"fetched_at"`
	ContentHash      string    `json:"content_hash,omitempty"`
	LoadedViaBrowser bool      `json:"loaded_via_browser,omitempty"`
}

// ResolvedSource contains the full resolved content for a single URL.
type ResolvedSource struct {
	SourceRef
	ContentMarkdown string             `json:"content_markdown,omitempty"`
	ContentText     string             `json:"content_text,omitempty"`
	Summary         string             `json:"summary,omitempty"`
	Repo            *GitHubRepoContext `json:"repo,omitempty"`
	Metadata        map[string]any     `json:"metadata,omitempty"`
	Warnings        []string           `json:"warnings,omitempty"`
}

// GitHubRepoContext holds structured repository context.
type GitHubRepoContext struct {
	Owner               string              `json:"owner"`
	Repo                string              `json:"repo"`
	DefaultBranch       string              `json:"default_branch"`
	CommitSHA           string              `json:"commit_sha,omitempty"`
	Description         string              `json:"description,omitempty"`
	README              string              `json:"readme,omitempty"`
	FileTree            []GitHubTreeEntry   `json:"file_tree,omitempty"`
	SelectedFiles       []GitHubFileContext `json:"selected_files,omitempty"`
	Manifests           []GitHubFileContext `json:"manifests,omitempty"`
	Docs                []GitHubFileContext `json:"docs,omitempty"`
	ArchitectureSignals []string            `json:"architecture_signals,omitempty"`
	Warnings            []string            `json:"warnings,omitempty"`
}

// GitHubTreeEntry is a single entry in the repository file tree.
type GitHubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// GitHubFileContext holds the content of a selected repository file.
type GitHubFileContext struct {
	Path      string `json:"path"`
	Language  string `json:"language,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SHA       string `json:"sha,omitempty"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// GitHubParseResult holds the parsed components of a GitHub URL.
type GitHubParseResult struct {
	Owner string
	Repo  string
	Ref   string // branch or tag
	Path  string // file or directory path within the repo
	Kind  URLKind
}

// ReadableDocument holds extracted content from a web page.
type ReadableDocument struct {
	Title       string
	Markdown    string
	Text        string
	Description string
	FinalURL    string
}
