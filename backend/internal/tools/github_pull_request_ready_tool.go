package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestReadyTool struct {
	service gitrepo.GitHubPullRequestReadyMarker
}

// NewGitHubPullRequestReadyTools returns the independently gated hosted
// mutation for advancing one exact reviewed draft PR to ready-for-review state.
func NewGitHubPullRequestReadyTools(service gitrepo.GitHubPullRequestReadyMarker) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubPullRequestReadyTool{service: service}}
}

func (t *githubPullRequestReadyTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_mark_pull_request_ready_for_review",
		Description:         "Mark one existing GitHub draft pull request ready for review after revalidating that it remains open, unmerged, draft, on the exact reviewed head, and targeted at the configured remote default branch. This is a hosted collaboration mutation. Repository, API host, token, pull request node ID, base branch, reviewer controls, merge controls, and GraphQL text are application/operator-controlled and are not accepted.",
		Category:            "github",
		Enabled:             true,
		Version:             "1",
		Risk:                RiskHigh,
		ReadOnly:            false,
		SideEffecting:       true,
		RequiresNetwork:     true,
		RequiresCredentials: true,
		SupportsParallel:    false,
		DefaultTimeoutMS:    30_000,
		MaxResultBytes:      gitRemoteToolResultLimit,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"number":{"type":"integer","minimum":1,"description":"Draft pull request number"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"Exact current PR head returned by GitHub PR inspection"}
			},
			"required":["remote","number","expected_head"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubPullRequestReadyTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request ready-for-review service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_mark_pull_request_ready_for_review requires valid JSON arguments")
	}
	allowed := map[string]bool{"remote": true, "number": true, "expected_head": true}
	if len(fields) != len(allowed) {
		return fmt.Errorf("github_mark_pull_request_ready_for_review requires only remote, number, and expected_head")
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("github_mark_pull_request_ready_for_review does not accept %q", key)
		}
	}
	var decoded struct {
		Remote       string `json:"remote"`
		Number       int    `json:"number"`
		ExpectedHead string `json:"expected_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid pull request ready-for-review arguments: %w", err)
	}
	if !gitrepo.ValidRepositoryID(strings.TrimSpace(decoded.Remote)) {
		return fmt.Errorf("remote must be a configured remote ID")
	}
	if decoded.Number <= 0 {
		return fmt.Errorf("number must be a positive pull request number")
	}
	if !validHex(strings.TrimSpace(decoded.ExpectedHead), 40) {
		return fmt.Errorf("expected_head must be the exact 40-character PR head from GitHub inspection")
	}
	return nil
}

func (t *githubPullRequestReadyTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote       string `json:"remote"`
		Number       int    `json:"number"`
		ExpectedHead string `json:"expected_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.MarkPullRequestReadyForReview(
		ctx,
		strings.TrimSpace(decoded.Remote),
		decoded.Number,
		strings.ToLower(strings.TrimSpace(decoded.ExpectedHead)),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
