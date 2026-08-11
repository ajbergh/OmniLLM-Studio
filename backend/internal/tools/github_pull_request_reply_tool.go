package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

const maxGitHubReviewReplyBodyBytes = 8 << 10

type githubPullRequestReviewReplyTool struct {
	service gitrepo.GitHubPullRequestReviewReplier
}

// NewGitHubPullRequestReplyTools returns the independently gated hosted
// communication mutation for replying to an existing inline review comment.
func NewGitHubPullRequestReplyTools(service gitrepo.GitHubPullRequestReviewReplier) []Tool {
	if service == nil {
		return nil
	}
	return []Tool{&githubPullRequestReviewReplyTool{service: service}}
}

func (t *githubPullRequestReviewReplyTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:                "github_reply_to_pull_request_review_comment",
		Description:         "Reply to one existing top-level inline GitHub pull request review comment after revalidating the exact reviewed PR head, review ID, comment ID, and comment updated_at timestamp. This is an external communication mutation that triggers GitHub notifications. Repository, API host, token, review state, thread resolution, and other PR controls are not accepted.",
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
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"Exact current PR head returned by github_get_pull_request_feedback or github_get_pull_request"},
				"comment_id":{"type":"integer","minimum":1,"description":"Top-level inline review comment ID returned by feedback kind review_comments"},
				"expected_review_id":{"type":"integer","minimum":1,"description":"Exact review_id returned with the reviewed comment"},
				"expected_updated_at":{"type":"string","description":"Exact updated_at timestamp returned with the reviewed comment"},
				"body":{"type":"string","minLength":1,"maxLength":8192,"description":"Reply text; maximum 8192 UTF-8 bytes"}
			},
			"required":["remote","number","expected_head","comment_id","expected_review_id","expected_updated_at","body"],
			"additionalProperties":false
		}`),
	}
}

func (t *githubPullRequestReviewReplyTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("GitHub pull request review reply service is unavailable")
	}
	var fields map[string]json.RawMessage
	if len(args) == 0 || json.Unmarshal(args, &fields) != nil {
		return fmt.Errorf("github_reply_to_pull_request_review_comment requires valid JSON arguments")
	}
	allowed := map[string]bool{
		"remote": true, "number": true, "expected_head": true, "comment_id": true,
		"expected_review_id": true, "expected_updated_at": true, "body": true,
	}
	if len(fields) != len(allowed) {
		return fmt.Errorf("github_reply_to_pull_request_review_comment requires only the reviewed PR/comment state and reply body")
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("github_reply_to_pull_request_review_comment does not accept %q", key)
		}
	}
	var decoded struct {
		Remote            string `json:"remote"`
		Number            int    `json:"number"`
		ExpectedHead      string `json:"expected_head"`
		CommentID         int64  `json:"comment_id"`
		ExpectedReviewID  int64  `json:"expected_review_id"`
		ExpectedUpdatedAt string `json:"expected_updated_at"`
		Body              string `json:"body"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid pull request review reply arguments: %w", err)
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
	if decoded.CommentID <= 0 || decoded.ExpectedReviewID <= 0 {
		return fmt.Errorf("comment_id and expected_review_id must be positive")
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(decoded.ExpectedUpdatedAt)); err != nil {
		return fmt.Errorf("expected_updated_at must be the reviewed GitHub comment timestamp")
	}
	if strings.TrimSpace(decoded.Body) == "" || len([]byte(decoded.Body)) > maxGitHubReviewReplyBodyBytes || !utf8.ValidString(decoded.Body) {
		return fmt.Errorf("body must contain valid UTF-8 text within 8192 bytes")
	}
	return nil
}

func (t *githubPullRequestReviewReplyTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote            string `json:"remote"`
		Number            int    `json:"number"`
		ExpectedHead      string `json:"expected_head"`
		CommentID         int64  `json:"comment_id"`
		ExpectedReviewID  int64  `json:"expected_review_id"`
		ExpectedUpdatedAt string `json:"expected_updated_at"`
		Body              string `json:"body"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	result, err := t.service.ReplyToPullRequestReviewComment(
		ctx,
		strings.TrimSpace(decoded.Remote),
		decoded.Number,
		strings.ToLower(strings.TrimSpace(decoded.ExpectedHead)),
		decoded.CommentID,
		decoded.ExpectedReviewID,
		strings.TrimSpace(decoded.ExpectedUpdatedAt),
		decoded.Body,
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(result)
}
