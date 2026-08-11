package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestReviewThreadsTool struct {
	service gitrepo.GitHubPullRequestReviewThreadReader
}

// NewGitHubPullRequestReviewThreadTools returns the independently bounded,
// read-only GitHub review-thread state tool under the existing PR read gate.
func NewGitHubPullRequestReviewThreadTools(service gitrepo.GitHubPullRequestReviewThreadReader) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubPullRequestReviewThreadsTool{service: service}}
}

func (t *githubPullRequestReviewThreadsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_get_pull_request_review_threads",
		Description:         "Read one bounded page of GitHub pull request review-thread state from an operator-configured github.com remote. Returns thread node IDs, resolved/outdated/collapsed state, bounded file/line metadata, resolver identity, and viewer capability flags without review bodies. The service fetches the PR first and requires GraphQL to report the same current head SHA. Repository, API host, token, arbitrary GraphQL query text, and thread mutations are never accepted from the model.",
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
				"number":{"type":"integer","minimum":1,"description":"Pull request number"},
				"after":{"type":"string","maxLength":512,"description":"Optional opaque next_cursor returned by the previous call to this same tool"},
				"limit":{"type":"integer","minimum":1,"maximum":20,"description":"Maximum threads for this page; defaults to 10"}
			},
			"required":["remote","number"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubPullRequestReviewThreadsTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request review thread service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_get_pull_request_review_threads requires valid JSON arguments")
	}
	for key := range fields {
		switch key {
		case "remote", "number", "after", "limit":
		default:
			return fmt.Errorf("github_get_pull_request_review_threads does not accept %q", key)
		}
	}
	if err := validateGitHubReadRemote(fields); err != nil {
		return err
	}
	if err := validateGitHubPullRequestNumber(fields["number"]); err != nil {
		return err
	}
	if rawAfter, ok := fields["after"]; ok {
		var after string
		if err := json.Unmarshal(rawAfter, &after); err != nil || len([]byte(strings.TrimSpace(after))) > 512 {
			return fmt.Errorf("after must be an opaque cursor no longer than 512 bytes")
		}
	}
	if rawLimit, ok := fields["limit"]; ok {
		var limit int
		if err := json.Unmarshal(rawLimit, &limit); err != nil || limit < 1 || limit > 20 {
			return fmt.Errorf("limit must be between 1 and 20")
		}
	}
	return nil
}

func (t *githubPullRequestReviewThreadsTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote string `json:"remote"`
		Number int    `json:"number"`
		After  string `json:"after"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.GetPullRequestReviewThreads(ctx, strings.TrimSpace(decoded.Remote), decoded.Number, strings.TrimSpace(decoded.After), decoded.Limit)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
