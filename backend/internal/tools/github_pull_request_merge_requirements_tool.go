package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestMergeRequirementsTool struct {
	service gitrepo.GitHubPullRequestMergeRequirementsReader
}

// NewGitHubPullRequestMergeRequirementsTools returns bounded read-only
// merge-policy and merge-eligibility inspection tools under the existing
// GitHub pull-request read gate. Later evidence layers are additive only when
// the service implements their reader interfaces.
func NewGitHubPullRequestMergeRequirementsTools(service gitrepo.GitHubPullRequestMergeRequirementsReader) []Tool {
	if service == nil {
		return nil
	}
	out := []Tool{&githubPullRequestMergeRequirementsTool{service: service}}
	if evidenceService, ok := service.(gitrepo.GitHubPullRequestMergePolicyEvidenceReader); ok {
		if evidenceTool := newGitHubPullRequestMergePolicyEvidenceTool(evidenceService); evidenceTool != nil {
			out = append(out, evidenceTool)
		}
	}
	if eligibilityService, ok := service.(gitrepo.GitHubPullRequestMergeEligibilityReader); ok {
		if eligibilityTool := newGitHubPullRequestMergeEligibilityTool(eligibilityService); eligibilityTool != nil {
			out = append(out, eligibilityTool)
		}
	}
	return out
}

func (t *githubPullRequestMergeRequirementsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_get_pull_request_merge_requirements",
		Description:         "Read a bounded, fail-closed normalized view of merge requirements for one GitHub pull request. The service fetches the pull request first and derives its exact head, base branch, repository, API host, token, active branch rules, classic protection, and repository merge-method settings internally. This tool never merges, changes policy, selects a merge method, or accepts arbitrary refs, repositories, rulesets, or API endpoints.",
		Category:            "github",
		Enabled:             true,
		Version:             "1",
		Risk:                RiskLow,
		ReadOnly:            true,
		SideEffecting:       false,
		RequiresNetwork:     true,
		RequiresCredentials: true,
		SupportsParallel:    true,
		DefaultTimeoutMS:    30_000,
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

func (t *githubPullRequestMergeRequirementsTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request merge requirements service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_get_pull_request_merge_requirements requires valid JSON arguments")
	}
	if len(fields) != 2 {
		return fmt.Errorf("github_get_pull_request_merge_requirements accepts only remote and number")
	}
	if err := validateGitHubReadRemote(fields); err != nil {
		return err
	}
	if _, ok := fields["number"]; !ok {
		return fmt.Errorf("number is required")
	}
	return validateGitHubPullRequestNumber(fields["number"])
}

func (t *githubPullRequestMergeRequirementsTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote string `json:"remote"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.GetPullRequestMergeRequirements(ctx, strings.TrimSpace(decoded.Remote), decoded.Number)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
