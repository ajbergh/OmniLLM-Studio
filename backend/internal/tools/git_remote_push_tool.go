package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type gitRemotePushTool struct {
	service gitrepo.RemotePusher
}

type gitRemoteBranchPublisherService interface {
	gitrepo.RemotePusher
	gitrepo.RemoteBranchPublisher
	BranchCreateMutationEnabled() bool
}

// NewGitRemotePushTools returns remote-side Git mutation tools. Registration is
// additionally gated by the process-wide remote push and local write settings.
// A branch-publication tool is included only when its separate creation gate is
// also enabled on a service that implements the guarded publication contract.
func NewGitRemotePushTools(svc gitrepo.RemotePusher) []Tool {
	if svc == nil {
		return nil
	}
	out := []Tool{&gitRemotePushTool{service: svc}}
	if publisher, ok := svc.(gitRemoteBranchPublisherService); ok && publisher.BranchCreateMutationEnabled() {
		out = append(out, &gitRemotePublishBranchTool{service: publisher})
	}
	return out
}

func (t *gitRemotePushTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "git_push",
		Description:      "Push the exact reviewed local HEAD to the same-named existing branch on one operator-configured remote. Requires a prior git_remote_status and git_fetch at the same remote head. Only fast-forward updates are permitted; force, delete, tag, arbitrary refspec, branch creation, and default-branch pushes without operator opt-in are forbidden.",
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
				"expected_branch":{"type":"string","description":"Current local branch from git_status; only the same-named existing remote branch may be updated"},
				"expected_head":{"type":"string","description":"Exact local HEAD hash from git_status to push"},
				"expected_remote_head":{"type":"string","description":"Reviewed remote branch hash used by the immediately preceding git_fetch"}
			},
			"required":["remote","expected_branch","expected_head","expected_remote_head"],
			"additionalProperties":false
		}`),
	}
}

func (t *gitRemotePushTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("remote Git push service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("git_push requires remote and reviewed state preconditions")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(fields) != 4 {
		return fmt.Errorf("git_push accepts only remote, expected_branch, expected_head, and expected_remote_head")
	}
	allowed := map[string]bool{
		"remote": true, "expected_branch": true, "expected_head": true, "expected_remote_head": true,
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("git_push does not accept %q", key)
		}
	}
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedHead       string `json:"expected_head"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid git_push arguments: %w", err)
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
	if !validHex(decoded.ExpectedRemoteHead, 40) {
		return fmt.Errorf("expected_remote_head must be the 40-character branch hash used by git_fetch")
	}
	return nil
}

func (t *gitRemotePushTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedHead       string `json:"expected_head"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	value, err := t.service.Push(
		ctx,
		strings.TrimSpace(decoded.Remote),
		strings.TrimSpace(decoded.ExpectedBranch),
		strings.TrimSpace(decoded.ExpectedHead),
		strings.TrimSpace(decoded.ExpectedRemoteHead),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(value)
}
