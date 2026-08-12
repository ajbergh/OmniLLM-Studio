package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type gitRemoteFetchTool struct {
	service gitrepo.RemoteFetcher
}

// NewGitRemoteMutationTools returns remote operations that mutate local or
// remote Git state. They remain separate from the read/inspection tool family.
func NewGitRemoteMutationTools(svc gitrepo.RemoteFetcher) []Tool {
	if svc == nil {
		return nil
	}
	return []Tool{&gitRemoteFetchTool{service: svc}}
}

func (t *gitRemoteFetchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "git_fetch",
		Description:      "Fetch the exact reviewed head of the current local branch from one remote ID returned by git_remotes into a bounded local object pack and isolated tracking ref. Requires branch/HEAD from git_status and the matching remote branch hash from git_remote_status; never changes HEAD, index, worktree, Git config, tags, submodules, or arbitrary refs.",
		Category:         "git",
		Enabled:          true,
		Version:          "1",
		Risk:             RiskHigh,
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
				"expected_branch":{"type":"string","description":"Current local branch from git_status; fetch operates only on the same-named remote branch"},
				"expected_head":{"type":"string","description":"Current local HEAD hash from git_status"},
				"expected_remote_head":{"type":"string","description":"Hash of expected_branch from a reviewed git_remote_status result"}
			},
			"required":["remote","expected_branch","expected_head","expected_remote_head"],
			"additionalProperties":false
		}`),
	}
}

func (t *gitRemoteFetchTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("remote Git fetch service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("git_fetch requires remote and reviewed state preconditions")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(fields) != 4 {
		return fmt.Errorf("git_fetch accepts only remote, expected_branch, expected_head, and expected_remote_head")
	}
	allowed := map[string]bool{
		"remote": true, "expected_branch": true, "expected_head": true, "expected_remote_head": true,
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("git_fetch does not accept %q", key)
		}
	}
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedHead       string `json:"expected_head"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid git_fetch arguments: %w", err)
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
		return fmt.Errorf("expected_remote_head must be the 40-character branch hash from git_remote_status")
	}
	return nil
}

func (t *gitRemoteFetchTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedHead       string `json:"expected_head"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	value, err := t.service.Fetch(
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