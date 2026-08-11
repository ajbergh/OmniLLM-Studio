package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type githubPullRequestReviewThreadResolutionTool struct {
	service gitrepo.GitHubPullRequestReviewThreadResolver
}

// NewGitHubPullRequestReviewThreadResolutionTools returns the independently
// gated hosted mutation for changing one reviewed thread's resolved state.
func NewGitHubPullRequestReviewThreadResolutionTools(service gitrepo.GitHubPullRequestReviewThreadResolver) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubPullRequestReviewThreadResolutionTool{service: service}}
}

func (t *githubPullRequestReviewThreadResolutionTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_set_pull_request_review_thread_resolved",
		Description:         "Change one existing GitHub pull request review thread between resolved and unresolved after revalidating the exact reviewed PR head, opaque thread ID, resolved/outdated state, repository/PR ownership, and configured viewer capability. This is a hosted collaboration mutation. GitHub viewer capability is not OmniLLM authorization; the independent operator gate and tool approval remain authoritative. Repository, API host, token, GraphQL query text, reviewer prose, and other PR controls are not accepted.",
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
				"number":{"type":"integer","minimum":1,"description":"Pull request number"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"Exact current PR head returned by GitHub PR/thread inspection"},
				"thread_id":{"type":"string","minLength":1,"maxLength":256,"description":"Opaque review thread ID returned by github_get_pull_request_review_threads"},
				"expected_is_resolved":{"type":"boolean","description":"Exact is_resolved value from the reviewed thread state"},
				"expected_is_outdated":{"type":"boolean","description":"Exact is_outdated value from the reviewed thread state"},
				"resolved":{"type":"boolean","description":"Desired resolved state; must differ from expected_is_resolved"}
			},
			"required":["remote","number","expected_head","thread_id","expected_is_resolved","expected_is_outdated","resolved"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubPullRequestReviewThreadResolutionTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request review thread resolution service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_set_pull_request_review_thread_resolved requires valid JSON arguments")
	}
	allowed := map[string]bool{
		"remote": true, "number": true, "expected_head": true, "thread_id": true,
		"expected_is_resolved": true, "expected_is_outdated": true, "resolved": true,
	}
	if len(fields) != len(allowed) {
		return fmt.Errorf("github_set_pull_request_review_thread_resolved requires only reviewed PR/thread state and desired resolution")
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("github_set_pull_request_review_thread_resolved does not accept %q", key)
		}
	}
	var decoded struct {
		Remote             string `json:"remote"`
		Number             int    `json:"number"`
		ExpectedHead       string `json:"expected_head"`
		ThreadID           string `json:"thread_id"`
		ExpectedIsResolved bool   `json:"expected_is_resolved"`
		ExpectedIsOutdated bool   `json:"expected_is_outdated"`
		Resolved           bool   `json:"resolved"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid pull request review thread resolution arguments: %w", err)
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
	threadID := strings.TrimSpace(decoded.ThreadID)
	if threadID == "" || len([]byte(threadID)) > 256 {
		return fmt.Errorf("thread_id must be the bounded opaque ID from GitHub review thread inspection")
	}
	if decoded.Resolved == decoded.ExpectedIsResolved {
		return fmt.Errorf("resolved must differ from expected_is_resolved")
	}
	return nil
}

func (t *githubPullRequestReviewThreadResolutionTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote             string `json:"remote"`
		Number             int    `json:"number"`
		ExpectedHead       string `json:"expected_head"`
		ThreadID           string `json:"thread_id"`
		ExpectedIsResolved bool   `json:"expected_is_resolved"`
		ExpectedIsOutdated bool   `json:"expected_is_outdated"`
		Resolved           bool   `json:"resolved"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.SetPullRequestReviewThreadResolved(
		ctx,
		strings.TrimSpace(decoded.Remote),
		decoded.Number,
		strings.ToLower(strings.TrimSpace(decoded.ExpectedHead)),
		strings.TrimSpace(decoded.ThreadID),
		decoded.ExpectedIsResolved,
		decoded.ExpectedIsOutdated,
		decoded.Resolved,
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
