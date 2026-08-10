package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/codesandbox"
)

// CodeSandboxTool executes code only through the configured external sandbox service.
type CodeSandboxTool struct{ client *codesandbox.Client }

func NewCodeSandboxTool(client *codesandbox.Client) *CodeSandboxTool {
	return &CodeSandboxTool{client: client}
}

func (t *CodeSandboxTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "code_execute",
		Description:      "Execute Python, JavaScript, or shell code inside the configured isolated code sandbox. Never runs code in the OmniLLM backend process.",
		Category:         "compute",
		Enabled:          t != nil && t.client != nil,
		Risk:             RiskHigh,
		SideEffecting:    true,
		ReadOnly:         false,
		RequiresNetwork:  true,
		SupportsParallel: false,
		DefaultTimeoutMS: 60000,
		MaxResultBytes:   1 << 20,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"language":{"type":"string","enum":["python","javascript","shell"]},"code":{"type":"string","minLength":1,"maxLength":100000},"session_id":{"type":"string"},"timeout_ms":{"type":"integer","minimum":100,"maximum":60000}},"required":["language","code"],"additionalProperties":false}`),
	}
}

func (t *CodeSandboxTool) Validate(args json.RawMessage) error {
	var in codesandbox.ExecuteRequest
	if err := json.Unmarshal(args, &in); err != nil {
		return err
	}
	switch in.Language {
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
	if in.TimeoutMS < 0 || in.TimeoutMS > 60000 {
		return fmt.Errorf("timeout_ms must be between 100 and 60000")
	}
	return nil
}

func (t *CodeSandboxTool) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t == nil || t.client == nil {
		return nil, fmt.Errorf("code sandbox is not configured")
	}
	var in codesandbox.ExecuteRequest
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, err
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 30000
	}
	out, err := t.client.Execute(ctx, in)
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
	for _, a := range out.Artifacts {
		artifacts = append(artifacts, ToolArtifact{Name: a.Name, MimeType: a.MimeType, URL: a.URL, Bytes: a.Bytes})
	}
	return &ToolResult{Content: content, Structured: structured, Artifacts: artifacts, IsError: out.ExitCode != 0, Metadata: map[string]interface{}{"session_id": out.SessionID, "exit_code": out.ExitCode}}, nil
}
