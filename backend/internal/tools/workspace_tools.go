package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

const workspaceToolResultLimit = 256 << 10

type workspaceTool struct{ name string }

// NewWorkspaceTools returns the generic filesystem workspace tool family. The
// tools resolve only application-owned workspace IDs; no tool accepts a host
// filesystem root.
func NewWorkspaceTools() []Tool {
	names := []string{
		"workspace_list",
		"workspace_read",
		"workspace_search",
		"workspace_write",
		"workspace_apply_patch",
		"workspace_delete",
		"workspace_changes",
		"workspace_revert",
	}
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, &workspaceTool{name: name})
	}
	return out
}

func (t *workspaceTool) Definition() ToolDefinition {
	def := ToolDefinition{
		Name:             t.name,
		Category:         "workspace",
		Enabled:          true,
		Version:          "1",
		RequiresNetwork:  false,
		SupportsParallel: false,
		DefaultTimeoutMS: 15_000,
		MaxResultBytes:   workspaceToolResultLimit,
	}
	switch t.name {
	case "workspace_list":
		def.Description = "List filesystem workspaces explicitly granted to the current user by opaque workspace ID and access mode. Host paths are never returned."
		def.Risk, def.ReadOnly, def.SideEffecting, def.SupportsParallel = RiskLow, true, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	case "workspace_read":
		def.Description = "Read a bounded regular file from an explicitly granted filesystem workspace and return its current SHA-256 for state-bound edits. Symlinks and .git paths are rejected."
		def.Risk, def.ReadOnly, def.SideEffecting, def.SupportsParallel = RiskLow, true, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"path":{"type":"string"},"max_bytes":{"type":"integer","minimum":1,"maximum":8388608}},"required":["workspace_id","path"],"additionalProperties":false}`)
	case "workspace_search":
		def.Description = "Search bounded text files in an explicitly granted filesystem workspace without following symlinks or reading .git metadata."
		def.Risk, def.ReadOnly, def.SideEffecting, def.SupportsParallel = RiskLow, true, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"query":{"type":"string","minLength":1},"max_matches":{"type":"integer","minimum":1,"maximum":200}},"required":["workspace_id","query"],"additionalProperties":false}`)
	case "workspace_write":
		def.Description = "Create or atomically replace a small regular file in a writable workspace. For an existing file, pass the SHA-256 returned by workspace_read so execution fails if the file changed. Changes are durably journaled."
		def.Risk, def.ReadOnly, def.SideEffecting = RiskHigh, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"path":{"type":"string"},"content":{"type":"string","maxLength":2097152},"expected_sha256":{"type":"string","pattern":"^[0-9a-fA-F]{64}$"}},"required":["workspace_id","path","content"],"additionalProperties":false}`)
	case "workspace_apply_patch":
		def.Description = "Apply 1-32 exact-text replacements to a small regular workspace file. Requires the reviewed SHA-256 and fails unless every old_text matches exactly once. Changes are atomic and journaled."
		def.Risk, def.ReadOnly, def.SideEffecting = RiskHigh, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"path":{"type":"string"},"expected_sha256":{"type":"string","pattern":"^[0-9a-fA-F]{64}$"},"edits":{"type":"array","minItems":1,"maxItems":32,"items":{"type":"object","properties":{"old_text":{"type":"string","minLength":1},"new_text":{"type":"string"}},"required":["old_text","new_text"],"additionalProperties":false}}},"required":["workspace_id","path","expected_sha256","edits"],"additionalProperties":false}`)
	case "workspace_delete":
		def.Description = "Delete one small regular file from a read_write workspace. Requires the exact current reviewed SHA-256; read_write_no_delete grants cannot use this tool. The delete is journaled and can be reverted while state remains unchanged."
		def.Risk, def.ReadOnly, def.SideEffecting = RiskHigh, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"path":{"type":"string"},"expected_sha256":{"type":"string","pattern":"^[0-9a-fA-F]{64}$"}},"required":["workspace_id","path","expected_sha256"],"additionalProperties":false}`)
	case "workspace_changes":
		def.Description = "List bounded recent change-journal records for one owned filesystem workspace, including hashes and revertability without exposing host paths."
		def.Risk, def.ReadOnly, def.SideEffecting, def.SupportsParallel = RiskLow, true, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"workspace_id":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":200}},"required":["workspace_id"],"additionalProperties":false}`)
	case "workspace_revert":
		def.Description = "Revert one journaled workspace change only if the current file still exactly matches that change's recorded after-state. The revert itself is journaled."
		def.Risk, def.ReadOnly, def.SideEffecting = RiskHigh, false, true
		def.Parameters = json.RawMessage(`{"type":"object","properties":{"change_id":{"type":"string","pattern":"^wch_"}},"required":["change_id"],"additionalProperties":false}`)
	}
	return def
}

type workspaceArgs struct {
	WorkspaceID    string             `json:"workspace_id"`
	Path           string             `json:"path"`
	Query          string             `json:"query"`
	Content        string             `json:"content"`
	ExpectedSHA256 string             `json:"expected_sha256"`
	MaxBytes       int64              `json:"max_bytes"`
	MaxMatches     int                `json:"max_matches"`
	Limit          int                `json:"limit"`
	ChangeID       string             `json:"change_id"`
	Edits          []sandbox.TextEdit `json:"edits"`
}

func (t *workspaceTool) Validate(raw json.RawMessage) error {
	var args workspaceArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	workspaceRequired := t.name != "workspace_list" && t.name != "workspace_revert"
	if workspaceRequired && strings.TrimSpace(args.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	switch t.name {
	case "workspace_list":
		return nil
	case "workspace_read":
		if strings.TrimSpace(args.Path) == "" {
			return fmt.Errorf("path is required")
		}
		if args.MaxBytes < 0 || args.MaxBytes > 8<<20 {
			return fmt.Errorf("max_bytes must be between 1 and 8388608")
		}
	case "workspace_search":
		if strings.TrimSpace(args.Query) == "" {
			return fmt.Errorf("query is required")
		}
		if args.MaxMatches < 0 || args.MaxMatches > 200 {
			return fmt.Errorf("max_matches must be between 1 and 200")
		}
	case "workspace_write":
		if strings.TrimSpace(args.Path) == "" {
			return fmt.Errorf("path is required")
		}
		if len(args.Content) > 2<<20 {
			return fmt.Errorf("content exceeds 2 MiB")
		}
		if args.ExpectedSHA256 != "" && !isSHA256(args.ExpectedSHA256) {
			return fmt.Errorf("expected_sha256 must be 64 hex characters")
		}
	case "workspace_apply_patch":
		if strings.TrimSpace(args.Path) == "" || !isSHA256(args.ExpectedSHA256) {
			return fmt.Errorf("path and expected_sha256 are required")
		}
		if len(args.Edits) == 0 || len(args.Edits) > 32 {
			return fmt.Errorf("edits must contain 1-32 entries")
		}
	case "workspace_delete":
		if strings.TrimSpace(args.Path) == "" || !isSHA256(args.ExpectedSHA256) {
			return fmt.Errorf("path and expected_sha256 are required")
		}
	case "workspace_changes":
		if args.Limit < 0 || args.Limit > 200 {
			return fmt.Errorf("limit must be between 1 and 200")
		}
	case "workspace_revert":
		if !strings.HasPrefix(strings.TrimSpace(args.ChangeID), "wch_") {
			return fmt.Errorf("valid change_id is required")
		}
	default:
		return fmt.Errorf("unknown workspace tool")
	}
	return nil
}

func (t *workspaceTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	fs, registry, err := sandbox.WorkspaceFSForOwner(owner.UserID)
	if err != nil {
		return nil, err
	}
	var args workspaceArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	var value any
	switch t.name {
	case "workspace_list":
		workspaces, err := registry.List(owner.UserID)
		if err != nil {
			return nil, err
		}
		type visibleWorkspace struct {
			ID   string            `json:"id"`
			Mode sandbox.MountMode `json:"mode"`
		}
		visible := make([]visibleWorkspace, 0, len(workspaces))
		for _, workspace := range workspaces {
			visible = append(visible, visibleWorkspace{ID: workspace.ID, Mode: workspace.Mode})
		}
		value = visible
	case "workspace_read":
		data, sha, err := fs.ReadFile(owner.UserID, args.WorkspaceID, args.Path, args.MaxBytes)
		if err != nil {
			return nil, err
		}
		value = map[string]any{"workspace_id": args.WorkspaceID, "path": args.Path, "content": string(data), "sha256": sha, "bytes": len(data)}
	case "workspace_search":
		matches, err := fs.Search(owner.UserID, args.WorkspaceID, args.Query, args.MaxMatches)
		if err != nil {
			return nil, err
		}
		value = map[string]any{"workspace_id": args.WorkspaceID, "matches": matches}
	case "workspace_write":
		value, err = fs.WriteFile(ctx, owner, args.WorkspaceID, args.Path, []byte(args.Content), args.ExpectedSHA256)
		if err != nil {
			return nil, err
		}
	case "workspace_apply_patch":
		value, err = fs.ApplyExactPatch(ctx, owner, args.WorkspaceID, args.Path, args.ExpectedSHA256, args.Edits)
		if err != nil {
			return nil, err
		}
	case "workspace_delete":
		value, err = fs.DeleteFile(ctx, owner, args.WorkspaceID, args.Path, args.ExpectedSHA256)
		if err != nil {
			return nil, err
		}
	case "workspace_changes":
		value, err = registry.ListWorkspaceChanges(ctx, owner.UserID, args.WorkspaceID, args.Limit)
		if err != nil {
			return nil, err
		}
	case "workspace_revert":
		value, err = fs.RevertChange(ctx, owner, args.ChangeID)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown workspace tool")
	}
	structured, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &ToolResult{Content: string(structured), Structured: structured}, nil
}

func isSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
