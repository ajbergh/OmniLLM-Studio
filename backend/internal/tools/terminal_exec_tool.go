package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// TerminalExecTool runs discrete argv commands inside the shared OS sandbox.
// Granted project workspaces are deliberately mounted read-only in this phase;
// source mutations remain journaled workspace-tool operations. Network remains
// disabled unless the caller supplies an owner-bound grant and the runtime
// advertises enforceable destination allowlisting.
type TerminalExecTool struct{}

func NewTerminalExecTool() *TerminalExecTool { return &TerminalExecTool{} }

func (t *TerminalExecTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "terminal_exec",
		Description:      "Run a command with explicit argv inside the configured OS sandbox. An optional granted filesystem workspace is mounted read-only; use workspace_write/workspace_apply_patch for journaled source changes. Network is disabled unless network_grant_id references an approved owner-bound destination grant and the runtime can enforce the allowlist.",
		Category:         "compute",
		Enabled:          sandbox.DefaultBroker() != nil,
		Version:          "1",
		Risk:             RiskHigh,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  false,
		SupportsParallel: false,
		DefaultTimeoutMS: 60000,
		MaxResultBytes:   1 << 20,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","minLength":1},"args":{"type":"array","maxItems":128,"items":{"type":"string","maxLength":8192}},"workspace_id":{"type":"string"},"directory":{"type":"string"},"stdin":{"type":"string","maxLength":1048576},"timeout_ms":{"type":"integer","minimum":100,"maximum":60000},"network_grant_id":{"type":"string","maxLength":128,"description":"Optional owner-bound grant returned by sandbox_network_grant"}},"required":["command"],"additionalProperties":false}`),
	}
}

type terminalExecArgs struct {
	Command        string   `json:"command"`
	Args           []string `json:"args"`
	WorkspaceID    string   `json:"workspace_id"`
	Directory      string   `json:"directory"`
	Stdin          string   `json:"stdin"`
	TimeoutMS      int      `json:"timeout_ms"`
	NetworkGrantID string   `json:"network_grant_id"`
}

func (t *TerminalExecTool) Validate(raw json.RawMessage) error {
	if sandbox.DefaultBroker() == nil {
		return fmt.Errorf("sandbox runtime is not configured")
	}
	var args terminalExecArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Command) == "" || strings.ContainsRune(args.Command, '\x00') {
		return fmt.Errorf("command is required")
	}
	if len(args.Args) > 128 {
		return fmt.Errorf("too many command arguments")
	}
	for _, arg := range args.Args {
		if len(arg) > 8192 || strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("invalid command argument")
		}
	}
	if len(args.Stdin) > 1<<20 {
		return fmt.Errorf("stdin exceeds 1 MiB")
	}
	if args.TimeoutMS != 0 && (args.TimeoutMS < 100 || args.TimeoutMS > 60000) {
		return fmt.Errorf("timeout_ms must be between 100 and 60000")
	}
	if grantID := strings.TrimSpace(args.NetworkGrantID); grantID != "" {
		if len(grantID) > 128 || !strings.HasPrefix(grantID, "sng_") {
			return fmt.Errorf("network_grant_id is invalid")
		}
	}
	return nil
}

func (t *TerminalExecTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	broker := sandbox.DefaultBroker()
	if broker == nil {
		return nil, fmt.Errorf("sandbox runtime is not configured")
	}
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var args terminalExecArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.TimeoutMS == 0 {
		args.TimeoutMS = 30000
	}

	spec := sandbox.CreateRequest{
		Network: sandbox.NetworkPolicy{Mode: sandbox.NetworkNone},
		Resources: sandbox.ResourceLimits{
			WallTimeMS:     args.TimeoutMS,
			MaxStdoutBytes: 1 << 20,
			MaxStderrBytes: 1 << 20,
		},
		TTLSeconds: 120,
		Requirements: sandbox.RuntimeRequirements{
			OSIsolation:          true,
			FilesystemIsolation:  true,
			NetworkIsolation:     true,
			ProcessTreeIsolation: true,
		},
	}
	if workspaceID := strings.TrimSpace(args.WorkspaceID); workspaceID != "" {
		// Never request a writable terminal mount in this phase. The Broker also
		// independently prevents grant widening.
		spec.Mounts = []sandbox.WorkspaceMount{{WorkspaceID: workspaceID, Mode: sandbox.MountReadOnly}}
	}

	networkMode := string(sandbox.NetworkNone)
	if grantID := strings.TrimSpace(args.NetworkGrantID); grantID != "" {
		grant, err := sandbox.DefaultNetworkGrantStore().Resolve(owner, grantID)
		if err != nil {
			return nil, err
		}
		spec.Network = sandbox.NetworkPolicy{
			Mode:           sandbox.NetworkAllowlist,
			AllowedDomains: append([]string(nil), grant.Domains...),
			AllowedPorts:   append([]int(nil), grant.Ports...),
		}
		spec.Requirements.NetworkAllowlist = true
		networkMode = string(sandbox.NetworkAllowlist)
	}

	session, err := broker.Create(ctx, owner, spec)
	if err != nil {
		return nil, err
	}
	out, execErr := broker.Exec(ctx, owner, session.ID, sandbox.ExecRequest{
		Command:   strings.TrimSpace(args.Command),
		Args:      append([]string(nil), args.Args...),
		Directory: args.Directory,
		Stdin:     []byte(args.Stdin),
		TimeoutMS: args.TimeoutMS,
	})
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cleanupErr := broker.Destroy(cleanupCtx, owner, session.ID)
	cancel()
	if execErr != nil {
		return nil, execErr
	}
	if cleanupErr != nil {
		return nil, fmt.Errorf("terminal sandbox cleanup failed: %w", cleanupErr)
	}
	if out == nil {
		return nil, fmt.Errorf("terminal sandbox returned no result")
	}

	content := out.Stdout
	if out.Stderr != "" {
		if content != "" {
			content += "\n"
		}
		content += "stderr:\n" + out.Stderr
	}
	if content == "" {
		content = fmt.Sprintf("Sandbox command exited with code %d", out.ExitCode)
	}
	structured, _ := json.Marshal(out)
	metadata := map[string]interface{}{
		"session_id":      session.ID,
		"execution_id":    out.ExecutionID,
		"exit_code":       out.ExitCode,
		"network":         networkMode,
		"workspace_write": false,
	}
	for key, value := range out.Metadata {
		metadata[key] = value
	}
	return &ToolResult{
		Content:    content,
		Structured: structured,
		IsError:    out.ExitCode != 0,
		Metadata:   metadata,
	}, nil
}
