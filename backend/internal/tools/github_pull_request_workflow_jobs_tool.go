package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestWorkflowJobsTool struct {
	service gitrepo.GitHubPullRequestWorkflowJobsReader
}

// NewGitHubPullRequestWorkflowJobsTool returns the bounded, read-only exact-head
// GitHub Actions metadata surface. Registration remains under the existing
// operator-controlled pull-request/CI read authorization boundary.
func NewGitHubPullRequestWorkflowJobsTool(service gitrepo.GitHubPullRequestWorkflowJobsReader) Tool {
	if service == nil {
		return nil
	}
	return &githubPullRequestWorkflowJobsTool{service: service}
}

func (t *githubPullRequestWorkflowJobsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_get_pull_request_workflow_jobs",
		Description:         "Read bounded GitHub Actions workflow, job, and step status metadata for one pull request. The service fetches the pull request first and derives its exact head plus all run/job IDs internally; commit refs, run IDs, job IDs, API URLs, tokens, logs, artifacts, runner names, and command output are never accepted from or exposed to the model.",
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

func (t *githubPullRequestWorkflowJobsTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request workflow job service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("github_get_pull_request_workflow_jobs requires remote and number")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(fields) != 2 {
		return fmt.Errorf("github_get_pull_request_workflow_jobs accepts only remote and number")
	}
	if err := validateGitHubReadRemote(fields); err != nil {
		return err
	}
	return validateGitHubPullRequestNumber(fields["number"])
}

func (t *githubPullRequestWorkflowJobsTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote string `json:"remote"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.GetPullRequestWorkflowJobs(ctx, strings.TrimSpace(decoded.Remote), decoded.Number)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
