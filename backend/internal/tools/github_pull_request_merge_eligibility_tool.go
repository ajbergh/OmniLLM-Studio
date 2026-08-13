package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestMergeEligibilityTool struct {
	service gitrepo.GitHubPullRequestMergeEligibilityReader
}

func newGitHubPullRequestMergeEligibilityTool(service gitrepo.GitHubPullRequestMergeEligibilityReader) Tool {
	if service == nil {
		return nil
	}
	return &githubPullRequestMergeEligibilityTool{service: service}
}

func (t *githubPullRequestMergeEligibilityTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_get_pull_request_merge_eligibility",
		Description:         "Read M3A current-state merge eligibility evidence for one GitHub pull request. The service starts from fresh fail-closed merge-policy evidence, binds evaluation to GitHub's exact current head and base, and verifies current mergeability, default-base state, required checks including app binding, review decision, required review-thread resolution, required deployments, and signature evidence where policy requires them. This tool never merges, changes policy, chooses a merge method, or accepts arbitrary refs, repositories, actors, checks, deployments, or API endpoints.",
		Category:            "github",
		Enabled:             true,
		Version:             "1",
		Risk:                RiskLow,
		ReadOnly:            true,
		SideEffecting:       false,
		RequiresNetwork:     true,
		RequiresCredentials: true,
		SupportsParallel:    false,
		DefaultTimeoutMS:    60_000,
		MaxResultBytes:      gitRemoteToolResultLimit,
		Parameters: json.RawMessage(`{
            "type":"object",
            "properties":{
                "remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
                "number":{"type":"integer","minimum":1,"description":"Pull request number"}
            },
            "required":["remote","number"],
            "additionalProperties":false
        }`),
	}
}

func (t *githubPullRequestMergeEligibilityTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request merge eligibility service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_get_pull_request_merge_eligibility requires valid JSON arguments")
	}
	if len(fields) != 2 {
		return fmt.Errorf("github_get_pull_request_merge_eligibility accepts only remote and number")
	}
	if err := validateGitHubReadRemote(fields); err != nil {
		return err
	}
	if _, ok := fields["number"]; !ok {
		return fmt.Errorf("number is required")
	}
	return validateGitHubPullRequestNumber(fields["number"])
}

func (t *githubPullRequestMergeEligibilityTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote string `json:"remote"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.GetPullRequestMergeEligibility(ctx, strings.TrimSpace(decoded.Remote), decoded.Number)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
