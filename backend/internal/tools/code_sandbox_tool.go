package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type codeSandboxArgs struct {
	Language  string `json:"language"`
	Code      string `json:"code"`
	SessionID string `json:"session_id,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

// CodeSandboxTool executes code only through the application-owned sandbox
// Broker. Session IDs are issued by Broker and ownership is revalidated against
// the current tool invocation scope on every reuse.
type CodeSandboxTool struct{ broker *sandbox.Broker }

func NewCodeSandboxTool(broker *sandbox.Broker) *CodeSandboxTool {
	return &CodeSandboxTool{broker: broker}
}

func (t *CodeSandboxTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "code_execute",
		Description:      "Execute Python, JavaScript, or shell code inside the configured OS-isolated sandbox. Sessions are application-issued, ownership-bound, and have network access disabled by default.",
		Category:         "compute",
		Enabled:          t != nil && t.broker != nil,
		Risk:             RiskHigh,
		SideEffecting:    true,
		ReadOnly:         false,
		RequiresNetwork:  false,
		SupportsParallel: false,
		DefaultTimeoutMS: 60000,
		MaxResultBytes:   1 << 20,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"language":{"type":"string","enum":["python","javascript","shell"]},"code":{"type":"string","minLength":1,"maxLength":100000},"session_id":{"type":"string","description":"Optional application-issued sandbox session ID returned by an earlier code_execute call in the same ownership scope."},"timeout_ms":{"type":"integer","minimum":100,"maximum":60000}},"required":["language","code"],"additionalProperties":false}`),
	}
}

func (t *CodeSandboxTool) Validate(args json.RawMessage) error {
	if t == nil || t.broker == nil {
		return fmt.Errorf("code sandbox is not configured")
	}
	var in codeSandboxArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(in.Language)) {
	case "python", "javascript", "shell":
	default:
		return fmt.Errorf("unsupported language")
	}
	if strings.TrimSpace(in.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if len(in.Code) > 100000 {
		return fmt.Errorf("code is too large")
	}
	if in.TimeoutMS != 0 && (in.TimeoutMS < 100 || in.TimeoutMS > 60000) {
		return fmt.Errorf("timeout_ms must be between 100 and 60000")
	}
	if in.SessionID != "" && !strings.HasPrefix(in.SessionID, "sbx_") {
		return fmt.Errorf("session_id must be an application-issued sandbox session")
	}
	return nil
}

func (t *CodeSandboxTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t == nil || t.broker == nil {
		return nil, fmt.Errorf("code sandbox is not configured")
	}
	var in codeSandboxArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, err
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 30000
	}
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}

	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		session, createErr := t.broker.Create(ctx, owner, defaultCodeSandboxSpec(in.TimeoutMS))
		if createErr != nil {
			return nil, createErr
		}
		sessionID = session.ID
	}

	out, err := t.broker.Exec(ctx, owner, sessionID, sandbox.ExecRequest{
		Language:  strings.ToLower(strings.TrimSpace(in.Language)),
		Code:      in.Code,
		TimeoutMS: in.TimeoutMS,
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(out)
	content := out.Stdout
	if out.Stderr != "" {
		if content != "" {
			content += "\n"
		}
		content += "stderr:\n" + out.Stderr
	}
	if content == "" {
		content = fmt.Sprintf("Sandbox process exited with code %d", out.ExitCode)
	}
	artifacts := make([]ToolArtifact, 0, len(out.Artifacts))
	for _, artifact := range out.Artifacts {
		artifacts = append(artifacts, ToolArtifact{ID: artifact.ID, Name: artifact.Name, MimeType: artifact.MimeType, Bytes: artifact.Bytes})
	}
	metadata := map[string]interface{}{
		"session_id":   sessionID,
		"execution_id": out.ExecutionID,
		"exit_code":    out.ExitCode,
		"network":      "none",
	}
	for key, value := range out.Metadata {
		metadata[key] = value
	}
	return &ToolResult{
		Content:    content,
		Structured: structured,
		Artifacts:  artifacts,
		IsError:    out.ExitCode != 0,
		Metadata:   metadata,
	}, nil
}
