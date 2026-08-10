package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

const gitRemoteToolResultLimit = 64 << 10

type gitRemoteTool struct {
	service gitrepo.RemoteReader
	name    string
}

// NewGitRemoteTools returns the configured remote Git inspection tools.
func NewGitRemoteTools(svc gitrepo.RemoteReader) []Tool {
	if svc == nil {
		return nil
	}
	return []Tool{
		&gitRemoteTool{service: svc, name: "git_remotes"},
		&gitRemoteTool{service: svc, name: "git_remote_status"},
	}
}

func (t *gitRemoteTool) Definition() ToolDefinition {
	definition := ToolDefinition{
		Name:             t.name,
		Category:         "git",
		Enabled:          true,
		Version:          "1",
		ReadOnly:         true,
		SideEffecting:    false,
		SupportsParallel: false,
		MaxResultBytes:   gitRemoteToolResultLimit,
	}
	switch t.name {
	case "git_remotes":
		definition.Description = "List operator-configured Git remotes by stable remote ID, repository ID, host, authentication presence, and push eligibility. Remote URLs and credential references are never exposed."
		definition.Risk = RiskLow
		definition.RequiresNetwork = false
		definition.SupportsParallel = true
		definition.DefaultTimeoutMS = 5_000
		definition.Parameters = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	case "git_remote_status":
		definition.Description = "Inspect advertised branch heads for one operator-configured Git remote. This performs approval-gated outbound HTTPS access and returns bounded remote branch hashes for later fetch/push preconditions."
		definition.Risk = RiskHigh
		definition.RequiresNetwork = true
		definition.DefaultTimeoutMS = 30_000
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"remote":{"type":"string","description":"Configured remote ID from git_remotes"}
			},
			"required":["remote"],
			"additionalProperties":false
		}`)
	}
	return definition
}

func (t *gitRemoteTool) Validate(args json.RawMessage) error {
	if t == nil || t.service == nil {
		return fmt.Errorf("remote Git service is unavailable")
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	switch t.name {
	case "git_remotes":
		if len(fields) != 0 {
			return fmt.Errorf("git_remotes does not accept arguments")
		}
		return nil
	case "git_remote_status":
		if len(fields) != 1 {
			return fmt.Errorf("git_remote_status accepts only the remote ID")
		}
		rawRemote, ok := fields["remote"]
		if !ok {
			return fmt.Errorf("remote is required")
		}
		var remoteID string
		if err := json.Unmarshal(rawRemote, &remoteID); err != nil {
			return fmt.Errorf("remote must be a configured remote ID")
		}
		remoteID = strings.TrimSpace(remoteID)
		if !gitrepo.ValidRepositoryID(remoteID) {
			return fmt.Errorf("remote must be a configured remote ID")
		}
		return nil
	default:
		return fmt.Errorf("unknown remote Git tool %q", t.name)
	}
}

func (t *gitRemoteTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	switch t.name {
	case "git_remotes":
		return structuredToolResult(map[string]any{"remotes": t.service.Remotes(ctx)})
	case "git_remote_status":
		var decoded struct {
			Remote string `json:"remote"`
		}
		if err := json.Unmarshal(args, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
		value, err := t.service.RemoteStatus(ctx, strings.TrimSpace(decoded.Remote))
		if err != nil {
			return &ToolResult{Content: err.Error(), IsError: true}, nil
		}
		return structuredToolResult(value)
	default:
		return nil, fmt.Errorf("unknown remote Git tool %q", t.name)
	}
}
