package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/urlcontext"
)

// FetchURLContextTool fetches a URL and returns structured Markdown content
// with source metadata. It uses the URL context resolver for readability
// extraction and GitHub-aware handling — a superset of the basic url_fetch tool.
type FetchURLContextTool struct {
	svc *urlcontext.Service
}

// NewFetchURLContextTool creates a FetchURLContextTool backed by the given service.
func NewFetchURLContextTool(svc *urlcontext.Service) *FetchURLContextTool {
	return &FetchURLContextTool{svc: svc}
}

type fetchURLContextArgs struct {
	URL  string `json:"url"`
	Goal string `json:"goal,omitempty"`
}

func (t *FetchURLContextTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch and extract content from"
			},
			"goal": {
				"type": "string",
				"description": "Optional analysis goal: summarize, review, explain, compare, architecture_review, security_review, code_review, feature_gap_review",
				"enum": ["summarize", "review", "explain", "compare", "architecture_review", "security_review", "code_review", "feature_gap_review"]
			}
		},
		"required": ["url"]
	}`)

	return ToolDefinition{
		Name:        "fetch_url_context",
		Description: "Fetch a URL and extract its readable content as structured Markdown. Supports web pages, GitHub repos, GitHub files, and PDFs. Returns source metadata and cleaned content suitable for analysis.",
		Parameters:  schema,
		Category:    "fetch",
		Enabled:     true,
	}
}

func (t *FetchURLContextTool) Validate(args json.RawMessage) error {
	var a fetchURLContextArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(a.URL, "http://") && !strings.HasPrefix(a.URL, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}
	return nil
}

func (t *FetchURLContextTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a fetchURLContextArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	result, err := t.svc.Resolve(ctx, urlcontext.ResolveRequest{
		UserMessage: a.URL,
		URLs:        []string{a.URL},
		Force:       true,
	})
	if err != nil {
		return &ToolResult{
			Content: urlcontext.UserFacingErrorMessage(err),
			IsError: true,
		}, nil
	}
	if result == nil || !result.Handled || len(result.ResolvedSources) == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("No content could be extracted from %s", a.URL),
			IsError: true,
		}, nil
	}

	src := result.ResolvedSources[0]
	var content string
	if src.ContentMarkdown != "" {
		content = src.ContentMarkdown
	} else if src.ContentText != "" {
		content = src.ContentText
	} else if src.Repo != nil {
		content = result.PromptContext
	}

	if content == "" {
		content = result.PromptContext
	}

	meta := map[string]interface{}{
		"url":       src.URL,
		"final_url": src.FinalURL,
		"title":     src.Title,
		"kind":      string(src.Kind),
	}
	if !src.FetchedAt.IsZero() {
		meta["fetched_at"] = src.FetchedAt.Format("2006-01-02T15:04:05Z")
	}
	if len(src.Warnings) > 0 {
		meta["warnings"] = src.Warnings
	}

	return &ToolResult{
		Content:  content,
		Metadata: meta,
	}, nil
}

// GitHubRepoInspectTool inspects a GitHub repository and returns structured
// context: metadata, file tree, architecture signals, and key file contents.
type GitHubRepoInspectTool struct {
	svc *urlcontext.Service
}

// NewGitHubRepoInspectTool creates a GitHubRepoInspectTool backed by the given service.
func NewGitHubRepoInspectTool(svc *urlcontext.Service) *GitHubRepoInspectTool {
	return &GitHubRepoInspectTool{svc: svc}
}

type githubRepoInspectArgs struct {
	URL  string `json:"url"`
	Goal string `json:"goal,omitempty"`
}

func (t *GitHubRepoInspectTool) Definition() ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "GitHub repository URL (e.g. https://github.com/owner/repo)"
			},
			"goal": {
				"type": "string",
				"description": "Analysis goal which controls file selection: architecture_review, security_review, code_review, feature_gap_review",
				"enum": ["architecture_review", "security_review", "code_review", "feature_gap_review"]
			}
		},
		"required": ["url"]
	}`)

	return ToolDefinition{
		Name:        "github_repo_inspect",
		Description: "Inspect a GitHub repository: fetches metadata, file tree, README, dependency manifests, architecture signals, and goal-specific source files. Use this for code review, architecture analysis, security review, or feature gap analysis of a GitHub repository.",
		Parameters:  schema,
		Category:    "fetch",
		Enabled:     true,
	}
}

func (t *GitHubRepoInspectTool) Validate(args json.RawMessage) error {
	var a githubRepoInspectArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if a.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.Contains(a.URL, "github.com") {
		return fmt.Errorf("url must be a GitHub repository URL")
	}
	return nil
}

func (t *GitHubRepoInspectTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var a githubRepoInspectArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	// Build a synthetic user message that triggers GitHub repo goal detection
	userMsg := a.URL
	if a.Goal != "" {
		switch a.Goal {
		case "architecture_review":
			userMsg = "Review the architecture of " + a.URL
		case "security_review":
			userMsg = "Security review of " + a.URL
		case "code_review":
			userMsg = "Code review of " + a.URL
		case "feature_gap_review":
			userMsg = "Feature gap analysis of " + a.URL
		}
	}

	result, err := t.svc.Resolve(ctx, urlcontext.ResolveRequest{
		UserMessage: userMsg,
		URLs:        []string{a.URL},
		Force:       true,
	})
	if err != nil {
		return &ToolResult{
			Content: urlcontext.UserFacingErrorMessage(err),
			IsError: true,
		}, nil
	}
	if result == nil || !result.Handled || len(result.ResolvedSources) == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("Could not inspect repository at %s", a.URL),
			IsError: true,
		}, nil
	}

	src := result.ResolvedSources[0]
	meta := map[string]interface{}{
		"url":  src.URL,
		"kind": string(src.Kind),
	}
	if src.Repo != nil {
		meta["owner"] = src.Repo.Owner
		meta["repo"] = src.Repo.Repo
		meta["default_branch"] = src.Repo.DefaultBranch
		if src.Repo.CommitSHA != "" {
			meta["commit_sha"] = src.Repo.CommitSHA
		}
		meta["file_count"] = len(src.Repo.FileTree)
		meta["selected_files"] = len(src.Repo.SelectedFiles)
	}
	if len(src.Warnings) > 0 {
		meta["warnings"] = src.Warnings
	}

	return &ToolResult{
		Content:  result.PromptContext,
		Metadata: meta,
	}, nil
}
