package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestMergeTool struct {
	service gitrepo.GitHubPullRequestMerger
}

// NewGitHubPullRequestMergeTools returns the independently gated M3B merge
// mutation. It is deliberately critical-risk and never accepts a merge method,
// repository selector, base branch, commit message, or branch-deletion option.
func NewGitHubPullRequestMergeTools(service gitrepo.GitHubPullRequestMerger) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubPullRequestMergeTool{service: service}}
}

func (t *githubPullRequestMergeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_merge_pull_request",
		Description:         "Merge one configured GitHub pull request only after a fresh fail-closed M3A eligibility preflight validates the exact reviewed head, current default base, required checks/reviews/threads/deployments/signatures, and the operator-configured merge method. The server sends one exact-head merge request and never deletes the source branch or automatically retries an ambiguous outcome. Repository identity, API host, token, base branch, merge method, commit message, and branch deletion remain application/operator-controlled.",
		Category:            "github",
		Enabled:             true,
		Version:             "1",
		Risk:                RiskCritical,
		ReadOnly:            false,
		SideEffecting:       true,
		RequiresNetwork:     true,
		RequiresCredentials: true,
		SupportsParallel:    false,
		DefaultTimeoutMS:    45_000,
		MaxResultBytes:      gitRemoteToolResultLimit,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"number":{"type":"integer","minimum":1,"description":"Pull request number"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"Exact current PR head returned by GitHub inspection"}
			},
			"required":["remote","number","expected_head"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubPullRequestMergeTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request merge service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_merge_pull_request requires valid JSON arguments")
	}
	allowed := map[string]bool{"remote": true, "number": true, "expected_head": true}
	if len(fields) != len(allowed) {
		return fmt.Errorf("github_merge_pull_request requires only remote, number, and expected_head")
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("github_merge_pull_request does not accept %q", key)
		}
	}
	var decoded struct {
		Remote       string `json:"remote"`
		Number       int    `json:"number"`
		ExpectedHead string `json:"expected_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid pull request merge arguments: %w", err)
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

func (t *githubPullRequestMergeTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote       string `json:"remote"`
		Number       int    `json:"number"`
		ExpectedHead string `json:"expected_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.MergePullRequest(
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
