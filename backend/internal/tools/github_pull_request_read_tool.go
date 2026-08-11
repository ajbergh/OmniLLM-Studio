package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestReadTool struct {
	service gitrepo.GitHubPullRequestReader
	name    string
}

// NewGitHubPullRequestReadTools returns read-only GitHub collaboration tools.
// Registration is independently gated from draft-PR creation and Git writes.
func NewGitHubPullRequestReadTools(service gitrepo.GitHubPullRequestReader) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{
		&githubPullRequestReadTool{service: service, name: "github_get_pull_request"},
		&githubPullRequestReadTool{service: service, name: "github_list_pull_requests"},
		&githubPullRequestReadTool{service: service, name: "github_get_pull_request_checks"},
	}
}

func (t *githubPullRequestReadTool) Definition() ToolDefinition {
	definition := ToolDefinition{
		Name:                t.name,
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
	}
	switch t.name {
	case "github_get_pull_request":
		definition.Description = "Read bounded metadata for one pull request in the repository derived from an operator-configured github.com remote. Repository, API host, token, and arbitrary commit refs are never accepted from the model."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"number":{"type":"integer","minimum":1,"description":"Pull request number"}
			},
			"required":["remote","number"],
			"additionalProperties":false
		}`)
	case "github_list_pull_requests":
		definition.Description = "List a bounded first page of pull requests for the repository derived from an operator-configured github.com remote. Results are sorted by most recently updated and may optionally be restricted to one same-repository head branch."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"state":{"type":"string","enum":["open","closed","all"],"description":"Pull request state; defaults to open"},
				"head_branch":{"type":"string","maxLength":200,"description":"Optional same-repository head branch filter"},
				"limit":{"type":"integer","minimum":1,"maximum":20,"description":"Maximum results; defaults to 10"}
			},
			"required":["remote"],
			"additionalProperties":false
		}`)
	case "github_get_pull_request_checks":
		definition.Description = "Read bounded GitHub check runs and combined commit-status contexts for one pull request. The service fetches the pull request first and binds inspection to its exact returned head SHA; the model cannot choose a commit ref."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured github.com remote ID from git_remotes"},
				"number":{"type":"integer","minimum":1,"description":"Pull request number"}
			},
			"required":["remote","number"],
			"additionalProperties":false
		}`)
	}
	return definition
}

func (t *githubPullRequestReadTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request read service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("%s requires a configured remote", t.name)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := validateGitHubReadRemote(fields); err != nil {
		return err
	}
	switch t.name {
	case "github_get_pull_request", "github_get_pull_request_checks":
		if len(fields) != 2 {
			return fmt.Errorf("%s accepts only remote and number", t.name)
		}
		return validateGitHubPullRequestNumber(fields["number"])
	case "github_list_pull_requests":
		for key := range fields {
			switch key {
			case "remote", "state", "head_branch", "limit":
			default:
				return fmt.Errorf("github_list_pull_requests does not accept %q", key)
			}
		}
		var decoded struct {
			State      string `json:"state"`
			HeadBranch string `json:"head_branch"`
			Limit      int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &decoded); err != nil {
			return fmt.Errorf("invalid pull request list arguments: %w", err)
		}
		state := strings.ToLower(strings.TrimSpace(decoded.State))
		if state != "" && state != "open" && state != "closed" && state != "all" {
			return fmt.Errorf("state must be open, closed, or all")
		}
		if len(decoded.HeadBranch) > 200 {
			return fmt.Errorf("head_branch exceeds 200 characters")
		}
		if rawLimit, ok := fields["limit"]; ok {
			var limit int
			if err := json.Unmarshal(rawLimit, &limit); err != nil || limit < 1 || limit > 20 {
				return fmt.Errorf("limit must be between 1 and 20")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown GitHub pull request read tool %q", t.name)
	}
}

func validateGitHubReadRemote(fields map[string]json.RawMessage) error {
	raw, ok := fields["remote"]
	if !ok {
		return fmt.Errorf("remote is required")
	}
	var remote string
	if err := json.Unmarshal(raw, &remote); err != nil || !gitrepo.ValidRepositoryID(strings.TrimSpace(remote)) {
		return fmt.Errorf("remote must be a configured remote ID")
	}
	return nil
}

func validateGitHubPullRequestNumber(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("number is required")
	}
	var number int
	if err := json.Unmarshal(raw, &number); err != nil || number <= 0 {
		return fmt.Errorf("number must be a positive pull request number")
	}
	return nil
}

func (t *githubPullRequestReadTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	switch t.name {
	case "github_get_pull_request":
		var decoded struct {
			Remote string `json:"remote"`
			Number int    `json:"number"`
		}
		if err := json.Unmarshal(args, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
		result, err := t.service.GetPullRequest(ctx, strings.TrimSpace(decoded.Remote), decoded.Number)
		if err != nil {
			return &ToolResult{Content: err.Error(), IsError: true}, nil
		}
		return structuredToolResult(result)
	case "github_list_pull_requests":
		var decoded struct {
			Remote     string `json:"remote"`
			State      string `json:"state"`
			HeadBranch string `json:"head_branch"`
			Limit      int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
		result, err := t.service.ListPullRequests(ctx, strings.TrimSpace(decoded.Remote), decoded.State, decoded.HeadBranch, decoded.Limit)
		if err != nil {
			return &ToolResult{Content: err.Error(), IsError: true}, nil
		}
		return structuredToolResult(result)
	case "github_get_pull_request_checks":
		var decoded struct {
			Remote string `json:"remote"`
			Number int    `json:"number"`
		}
		if err := json.Unmarshal(args, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
		result, err := t.service.GetPullRequestChecks(ctx, strings.TrimSpace(decoded.Remote), decoded.Number)
		if err != nil {
			return &ToolResult{Content: err.Error(), IsError: true}, nil
		}
		return structuredToolResult(result)
	default:
		return nil, fmt.Errorf("unknown GitHub pull request read tool %q", t.name)
	}
}
