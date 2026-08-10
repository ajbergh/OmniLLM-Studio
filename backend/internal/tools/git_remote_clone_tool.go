package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type gitRemoteCloneTool struct {
	service gitrepo.RemoteCloner
}

// NewGitRemoteCloneTools returns remote repository-creation tools. Registration
// is additionally gated by explicit clone enablement, valid storage budgets,
// remote access, and the local Git write gate.
func NewGitRemoteCloneTools(svc gitrepo.RemoteCloner) []Tool {
	if svc == nil {
		return nil
	}
	return []Tool{&gitRemoteCloneTool{service: svc}}
}

func (t *gitRemoteCloneTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "git_clone",
		Description:      "Clone one reviewed branch from an operator-configured remote into that remote's preconfigured repository destination. The destination must not already exist. Clone is quota-enforced during pack ingestion and checkout; raw URLs, filesystem paths, credentials, arbitrary refspecs, submodule recursion, and caller-selected quotas are forbidden.",
		Category:         "git",
		Enabled:          true,
		Version:          "1",
		Risk:             RiskCritical,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  true,
		SupportsParallel: false,
		DefaultTimeoutMS: 120_000,
		MaxResultBytes:   gitRemoteToolResultLimit,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured remote ID from git_remotes; it also determines the operator-configured destination repository ID"},
				"expected_branch":{"type":"string","description":"Exact branch name reviewed in git_remote_status"},
				"expected_remote_head":{"type":"string","description":"Exact 40-character branch hash reviewed in git_remote_status"}
			},
			"required":["remote","expected_branch","expected_remote_head"],
			"additionalProperties":false
		}`),
	}
}

func (t *gitRemoteCloneTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("remote Git clone service is unavailable")
	}
	if len(args) == 0 {
		return fmt.Errorf("git_clone requires a remote and reviewed branch state")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(fields) != 3 {
		return fmt.Errorf("git_clone accepts only remote, expected_branch, and expected_remote_head")
	}
	allowed := map[string]bool{
		"remote": true, "expected_branch": true, "expected_remote_head": true,
	}
	for key := range fields {
		if !allowed[key] {
			return fmt.Errorf("git_clone does not accept %q", key)
		}
	}
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid git_clone arguments: %w", err)
	}
	if !gitrepo.ValidRepositoryID(strings.TrimSpace(decoded.Remote)) {
		return fmt.Errorf("remote must be a configured remote ID")
	}
	branch := strings.TrimSpace(decoded.ExpectedBranch)
	if branch == "" || len(branch) > 200 {
		return fmt.Errorf("expected_branch must be a branch name from git_remote_status")
	}
	if !validHex(decoded.ExpectedRemoteHead, 40) {
		return fmt.Errorf("expected_remote_head must be the 40-character branch hash from git_remote_status")
	}
	return nil
}

func (t *gitRemoteCloneTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded struct {
		Remote             string `json:"remote"`
		ExpectedBranch     string `json:"expected_branch"`
		ExpectedRemoteHead string `json:"expected_remote_head"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	value, err := t.service.Clone(
		ctx,
		strings.TrimSpace(decoded.Remote),
		strings.TrimSpace(decoded.ExpectedBranch),
		strings.TrimSpace(decoded.ExpectedRemoteHead),
	)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(value)
}
