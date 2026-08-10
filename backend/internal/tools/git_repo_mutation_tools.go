package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

const gitMutationToolResultLimit = 16 << 10

type gitRepositoryMutationTool struct {
	service gitrepo.Writer
	name    string
}

// NewGitRepositoryMutationTools returns the local Git mutation tools backed by
// svc. They are high-risk, side-effecting, non-network operations and should be
// registered only when the operator has explicitly enabled Git writes.
func NewGitRepositoryMutationTools(svc gitrepo.Writer) []Tool {
	if svc == nil {
		return nil
	}
	names := []string{"git_create_branch", "git_checkout", "git_stage", "git_commit"}
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, &gitRepositoryMutationTool{service: svc, name: name})
	}
	return out
}

func (t *gitRepositoryMutationTool) Definition() ToolDefinition {
	definition := ToolDefinition{
		Name:             t.name,
		Category:         "git",
		Enabled:          true,
		Version:          "1",
		Risk:             RiskHigh,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  false,
		SupportsParallel: false,
		DefaultTimeoutMS: 10_000,
		MaxResultBytes:   gitMutationToolResultLimit,
	}
	switch t.name {
	case "git_create_branch":
		definition.Description = "Create a local branch in a configured repository without switching HEAD. Requires the HEAD observed from git_status so stale approvals fail safely."
		definition.Parameters = json.RawMessage(`{
			"type":"object","required":["repository","name","expected_head"],
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"name":{"type":"string","minLength":1,"maxLength":200,"description":"New local branch name"},
				"from":{"type":"string","description":"Revision to branch from; defaults to HEAD"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"HEAD hash observed from git_status"}
			},"additionalProperties":false
		}`)
	case "git_checkout":
		definition.Description = "Switch a configured repository to an existing local branch. Checkout is refused unless the worktree is clean and HEAD still matches git_status."
		definition.Parameters = json.RawMessage(`{
			"type":"object","required":["repository","branch","expected_head"],
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"branch":{"type":"string","minLength":1,"maxLength":200,"description":"Existing local branch name"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"HEAD hash observed from git_status"}
			},"additionalProperties":false
		}`)
	case "git_stage":
		definition.Description = "Stage exact changed repository-relative files after review. Requires branch, HEAD, and index_digest from git_status plus worktree_digest from the matching worktree git_diff; any changed worktree content invalidates the request."
		definition.Parameters = json.RawMessage(`{
			"type":"object","required":["repository","paths","expected_branch","expected_head","expected_index_digest","expected_worktree_digest"],
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"paths":{"type":"array","minItems":1,"maxItems":50,"uniqueItems":true,"items":{"type":"string","minLength":1},"description":"Exact repository-relative file paths"},
				"expected_branch":{"type":"string","minLength":1,"maxLength":200,"description":"Local branch observed from git_status"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"HEAD hash observed from git_status"},
				"expected_index_digest":{"type":"string","minLength":64,"maxLength":64,"description":"index_digest observed from the same git_status snapshot"},
				"expected_worktree_digest":{"type":"string","minLength":64,"maxLength":64,"description":"worktree_digest from a worktree git_diff reviewed after that git_status"}
			},"additionalProperties":false
		}`)
	case "git_commit":
		definition.Description = "Commit the already-staged index only. Does not auto-stage, amend, create empty commits, or operate on detached HEAD. Requires branch, HEAD, and index_digest from a fresh git_status after staging."
		definition.Parameters = json.RawMessage(`{
			"type":"object","required":["repository","message","expected_branch","expected_head","expected_index_digest"],
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"message":{"type":"string","minLength":1,"maxLength":4000,"description":"Commit message"},
				"expected_branch":{"type":"string","minLength":1,"maxLength":200,"description":"Local branch observed from git_status after staging"},
				"expected_head":{"type":"string","minLength":40,"maxLength":40,"description":"HEAD hash observed from git_status after staging"},
				"expected_index_digest":{"type":"string","minLength":64,"maxLength":64,"description":"index_digest observed from git_status after staging"}
			},"additionalProperties":false
		}`)
	}
	return definition
}

func (t *gitRepositoryMutationTool) Validate(raw json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("git repository mutation service is unavailable")
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var args gitMutationArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	args.Repository = strings.TrimSpace(args.Repository)
	if !gitrepo.ValidRepositoryID(args.Repository) {
		return fmt.Errorf("repository must be a configured repository ID")
	}
	if !validHex(args.ExpectedHead, 40) {
		return fmt.Errorf("expected_head must be a 40-character commit hash from git_status")
	}
	switch t.name {
	case "git_create_branch":
		if name := strings.TrimSpace(args.Name); name == "" || len(name) > 200 {
			return fmt.Errorf("name must be between 1 and 200 bytes")
		}
	case "git_checkout":
		if branch := strings.TrimSpace(args.Branch); branch == "" || len(branch) > 200 {
			return fmt.Errorf("branch must be between 1 and 200 bytes")
		}
	case "git_stage":
		if err := validateExpectedBranchAndIndex(args); err != nil {
			return err
		}
		if !validHex(args.ExpectedWorktreeDigest, 64) {
			return fmt.Errorf("expected_worktree_digest must be the 64-character digest from a worktree git_diff")
		}
		if len(args.Paths) == 0 || len(args.Paths) > 50 {
			return fmt.Errorf("paths must contain between 1 and 50 files")
		}
		seen := map[string]struct{}{}
		for _, filePath := range args.Paths {
			filePath = strings.TrimSpace(filePath)
			if filePath == "" {
				return fmt.Errorf("paths cannot contain empty values")
			}
			if _, ok := seen[filePath]; ok {
				return fmt.Errorf("paths must be unique")
			}
			seen[filePath] = struct{}{}
		}
	case "git_commit":
		if err := validateExpectedBranchAndIndex(args); err != nil {
			return err
		}
		message := strings.TrimSpace(args.Message)
		if message == "" || len(message) > 4000 {
			return fmt.Errorf("message must be between 1 and 4000 bytes")
		}
	default:
		return fmt.Errorf("unknown git mutation tool %q", t.name)
	}
	return nil
}

func (t *gitRepositoryMutationTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var args gitMutationArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	var value any
	var err error
	switch t.name {
	case "git_create_branch":
		value, err = t.service.CreateBranch(ctx, args.Repository, args.Name, args.From, args.ExpectedHead)
	case "git_checkout":
		value, err = t.service.Checkout(ctx, args.Repository, args.Branch, args.ExpectedHead)
	case "git_stage":
		value, err = t.service.Stage(ctx, args.Repository, args.Paths, args.ExpectedBranch, args.ExpectedHead, args.ExpectedIndexDigest, args.ExpectedWorktreeDigest)
	case "git_commit":
		value, err = t.service.Commit(ctx, args.Repository, args.Message, args.ExpectedBranch, args.ExpectedHead, args.ExpectedIndexDigest)
	default:
		return nil, fmt.Errorf("unknown git mutation tool %q", t.name)
	}
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(value)
}

type gitMutationArgs struct {
	Repository             string   `json:"repository"`
	Name                   string   `json:"name"`
	From                   string   `json:"from"`
	Branch                 string   `json:"branch"`
	Paths                  []string `json:"paths"`
	Message                string   `json:"message"`
	ExpectedBranch         string   `json:"expected_branch"`
	ExpectedHead           string   `json:"expected_head"`
	ExpectedIndexDigest    string   `json:"expected_index_digest"`
	ExpectedWorktreeDigest string   `json:"expected_worktree_digest"`
}

func validateExpectedBranchAndIndex(args gitMutationArgs) error {
	branch := strings.TrimSpace(args.ExpectedBranch)
	if branch == "" || len(branch) > 200 {
		return fmt.Errorf("expected_branch must be the local branch from git_status")
	}
	if !validHex(args.ExpectedIndexDigest, 64) {
		return fmt.Errorf("expected_index_digest must be the 64-character digest from git_status")
	}
	return nil
}

func validHex(value string, size int) bool {
	value = strings.TrimSpace(value)
	if len(value) != size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
