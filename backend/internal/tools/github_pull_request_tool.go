package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubDraftPullRequestTool struct {
	service gitrepo.GitHubPullRequestCreator
}

// NewGitHubPullRequestTools returns the GitHub collaboration mutation tools.
// Registration is separately gated from Git push/branch creation and this slice
// exposes draft PR creation only.
func NewGitHubPullRequestTools(service gitrepo.GitHubPullRequestCreator) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubDraftPullRequestTool{service: service}}
}

func (t *githubDraftPullRequestTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "github_create_draft_pull_request",
		Description:      "Create a draft GitHub pull request for the exact reviewed current branch on an operator-configured github.com remote. The published source branch must still equal local HEAD, the reviewed branch-state digest must still match, and the base is always the remote's advertised default branch. Repository, API URL, token, base branch, reviewers, labels, merge, and ready-for-review controls are not accepted.",
		Category:         "github",
		Enabled:          true,
		Version:          "1",
		Risk:             RiskCritical,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  true,
		SupportsParallel: false,
		DefaultTimeoutMS: 30_000,
		MaxResultBytes:   gitRemoteToolResultLimit,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"expected_branch":{"type":"string","description":"Exact current local branch from git_status; the same remote branch must exist at expected_head"},
				"expected_head":{"type":"string","description":"Exact 40-character local HEAD hash from git_status"},
				"expected_remote_state_digest":{"type":"string","description":"Exact 64-character branch_state_digest from a reviewed git_remote_status after branch publication"},
				"title":{"type":"string","minLength":1,"maxLength":256,"description":"Draft pull request title"},
				"body":{"type":"string","maxLength":32768,"description":"Optional draft pull request body"}
			},
			"required":["remote","expected_branch","expected_head","expected_remote_state_digest","title"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubDraftPullRequestTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("github_create_draft_pull_request requires reviewed Git state and a title")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	allowed := map[string]bool{
		"remote": true, "expected_branch": true, "expected_head": true,
		"expected_remote_state_digest": true, "title": true, "body": true,
	}
	if len(fields) < 5 || len(fields) > 6 {
		return fmt.Errorf("github_create_draft_pull_request accepts only remote, reviewed state, title, and optional body")
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("github_create_draft_pull_request does not accept %q", key)
		}
	}
	var decoded struct {
		Remote                    string `json:"remote"`
		ExpectedBranch            string `json:"expected_branch"`
		ExpectedHead              string `json:"expected_head"`
		ExpectedRemoteStateDigest string `json:"expected_remote_state_digest"`
		Title                     string `json:"title"`
		Body                      string `json:"body"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid draft pull request arguments: %w", err)
	}
	if !gitrepo.ValidRepositoryID(strings.TrimSpace(decoded.Remote)) {
		return fmt.Errorf("remote must be a configured remote ID")
	}
	branch := strings.TrimSpace(decoded.ExpectedBranch)
	if branch == "" || len(branch) > 200 {
		return fmt.Errorf("expected_branch must be the current local branch from git_status")
	}
	if !validHex(decoded.ExpectedHead, 40) {
		return fmt.Errorf("expected_head must be the 40-character HEAD hash from git_status")
	}
	if !validHex(decoded.ExpectedRemoteStateDigest, 64) {
		return fmt.Errorf("expected_remote_state_digest must be the 64-character branch_state_digest from git_remote_status")
	}
	title := strings.TrimSpace(decoded.Title)
	if title == "" || utf8.RuneCountInString(title) > 256 {
		return fmt.Errorf("title must contain 1-256 characters")
	}
	if len([]byte(decoded.Body)) > 32<<10 {
		return fmt.Errorf("body exceeds the guarded 32768-byte limit")
	}
	return nil
}

func (t *githubDraftPullRequestTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote                    string `json:"remote"`
		ExpectedBranch            string `json:"expected_branch"`
		ExpectedHead              string `json:"expected_head"`
		ExpectedRemoteStateDigest string `json:"expected_remote_state_digest"`
		Title                     string `json:"title"`
		Body                      string `json:"body"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.CreateDraftPullRequest(
		ctx,
		strings.TrimSpace(decoded.Remote),
		strings.TrimSpace(decoded.ExpectedBranch),
		strings.TrimSpace(decoded.ExpectedHead),
		strings.TrimSpace(decoded.ExpectedRemoteStateDigest),
		strings.TrimSpace(decoded.Title),
		decoded.Body,
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
