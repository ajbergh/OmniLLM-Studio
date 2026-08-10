package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func (t *gitRepositoryTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("git repository service is unavailable")
	}
	var decoded gitToolArgs
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if t.name == "git_repositories" {
		return nil
	}
	decoded.Repository = strings.TrimSpace(decoded.Repository)
	if !gitrepo.ValidRepositoryID(decoded.Repository) {
		return fmt.Errorf("repository must be a configured repository ID")
	}
	if t.name == "git_diff" && strings.TrimSpace(decoded.From) == "" && strings.TrimSpace(decoded.To) != "" {
		return fmt.Errorf("from is required when to is provided")
	}
	if t.name == "git_log" && (decoded.Limit < 0 || decoded.Limit > 100) {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	if t.name == "git_blame" {
		if strings.TrimSpace(decoded.Path) == "" {
			return fmt.Errorf("path is required")
		}
		if decoded.StartLine < 0 || decoded.EndLine < 0 {
			return fmt.Errorf("line numbers must be positive")
		}
		if decoded.StartLine > 0 && decoded.EndLine > 0 && decoded.EndLine < decoded.StartLine {
			return fmt.Errorf("end_line must be greater than or equal to start_line")
		}
	}
	return nil
}

func (t *gitRepositoryTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var decoded gitToolArgs
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	var value any
	var err error
	switch t.name {
	case "git_repositories":
		value = map[string]any{"repositories": t.service.Repositories(ctx)}
	case "git_status":
		value, err = t.service.Status(ctx, decoded.Repository)
	case "git_diff":
		value, err = t.service.Diff(ctx, decoded.Repository, decoded.From, decoded.To)
	case "git_log":
		value, err = t.service.Log(ctx, decoded.Repository, decoded.Revision, decoded.Limit)
	case "git_show":
		value, err = t.service.Show(ctx, decoded.Repository, decoded.Revision)
	case "git_branches":
		value, err = t.service.Branches(ctx, decoded.Repository)
	case "git_blame":
		value, err = t.service.Blame(ctx, decoded.Repository, decoded.Path, decoded.Revision, decoded.StartLine, decoded.EndLine)
	default:
		return nil, fmt.Errorf("unknown git tool %q", t.name)
	}
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return structuredToolResult(value)
}

type gitToolArgs struct {
	Repository string `json:"repository"`
	From       string `json:"from"`
	To         string `json:"to"`
	Revision   string `json:"revision"`
	Limit      int    `json:"limit"`
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
}

func repositoryOnlySchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"repository":{"type":"string","description":"Configured repository ID from git_repositories"}
		},
		"required":["repository"],
		"additionalProperties":false
	}`)
}

func structuredToolResult(value any) (*ToolResult, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal git result: %w", err)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format git result: %w", err)
	}
	return &ToolResult{Content: string(pretty), Structured: payload}, nil
}
