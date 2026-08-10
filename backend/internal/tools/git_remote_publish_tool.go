package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type gitRemotePublishBranchTool struct {
	service gitrepo.RemoteBranchPublisher
}

// NewGitRemoteBranchPublishTools returns guarded remote branch-creation tools.
// Registration additionally requires the remote/write/push gates plus the
// independent process-wide branch-creation gate.
func NewGitRemoteBranchPublishTools(svc gitrepo.RemoteBranchPublisher) []Tool {
	if svc == nil {
		return nil
	}
	return []Tool{&gitRemotePublishBranchTool{service: svc}}
}

func (t *gitRemotePublishBranchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "git_publish_branch",
		Description:      "Publish the exact reviewed local HEAD as a new same-named branch on one operator-configured remote. Requires a git_remote_status branch-state digest proving the branch was absent. Existing branches, main/master/default branches, force, delete, tags, arbitrary refspecs, and caller-selected destinations are forbidden.",
		Category:         "git",
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
				"remote":{"type":"string","description":"Configured remote ID from git_remotes"},
				"expected_branch":{"type":"string","description":"Current local branch from git_status; the exact same remote branch must be absent"},
				"expected_head":{"type":"string","description":"Exact local HEAD hash from git_status to publish"},
				"expected_remote_state_digest":{"type":"string","description":"Exact branch_state_digest from the reviewed git_remote_status result"}
			},
			"required":["remote","expected_branch","expected_head","expected_remote_state_digest"],
			"additionalProperties":false
		}`),
	}
}

func (t *gitRemotePublishBranchTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("remote Git branch publication service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("git_publish_branch requires remote and reviewed state preconditions")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(fields) != 4 {
		return fmt.Errorf("git_publish_branch accepts only remote, expected_branch, expected_head, and expected_remote_state_digest")
	}
	allowed := map[string]bool{
		"remote": true, "expected_branch": true, "expected_head": true, "expected_remote_state_digest": true,
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("git_publish_branch does not accept %q", key)
		}
	}
	var decoded struct {
		Remote                    string `json:"remote"`
		ExpectedBranch            string `json:"expected_branch"`
		ExpectedHead              string `json:"expected_head"`
		ExpectedRemoteStateDigest string `json:"expected_remote_state_digest"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid git_publish_branch arguments: %w", err)
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
	return nil
}

func (t *gitRemotePublishBranchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote                    string `json:"remote"`
		ExpectedBranch            string `json:"expected_branch"`
		ExpectedHead              string `json:"expected_head"`
		ExpectedRemoteStateDigest string `json:"expected_remote_state_digest"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	value, err := t.service.PublishBranch(
		ctx,
		strings.TrimSpace(decoded.Remote),
		strings.TrimSpace(decoded.ExpectedBranch),
		strings.TrimSpace(decoded.ExpectedHead),
		strings.TrimSpace(decoded.ExpectedRemoteStateDigest),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(value)
}
