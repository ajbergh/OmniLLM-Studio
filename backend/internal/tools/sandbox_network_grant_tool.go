package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// SandboxNetworkGrantTool creates a short-lived owner-bound destination grant.
// The normal Executor Ask/Allow/Deny policy is the user-approval boundary; the
// runtime must independently advertise enforceable allowlist support before the
// grant can be consumed by a sandbox session.
type SandboxNetworkGrantTool struct{}

func NewSandboxNetworkGrantTool() *SandboxNetworkGrantTool { return &SandboxNetworkGrantTool{} }

func (t *SandboxNetworkGrantTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "sandbox_network_grant",
		Description:      "Request a short-lived sandbox network grant for exact operator-permitted DNS domains and ports. This does not itself make a connection; consuming runtimes must enforce destination allowlisting.",
		Category:         "network",
		Enabled:          sandbox.DefaultBroker() != nil,
		Version:          "1",
		Risk:             RiskHigh,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  true,
		SupportsParallel: false,
		DefaultTimeoutMS: 5000,
		MaxResultBytes:   16 << 10,
		Parameters: json.RawMessage(`{"type":"object","properties":{"domains":{"type":"array","minItems":1,"maxItems":16,"items":{"type":"string"}},"ports":{"type":"array","maxItems":16,"items":{"type":"integer","minimum":1,"maximum":65535}},"ttl_seconds":{"type":"integer","minimum":60,"maximum":1800}},"required":["domains"],"additionalProperties":false}`),
	}
}

type networkGrantArgs struct {
	Domains    []string `json:"domains"`
	Ports      []int    `json:"ports"`
	TTLSeconds int      `json:"ttl_seconds"`
}

func (t *SandboxNetworkGrantTool) Validate(raw json.RawMessage) error {
	if sandbox.DefaultBroker() == nil {
		return fmt.Errorf("sandbox runtime is not configured")
	}
	var args networkGrantArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if len(args.Domains) == 0 || len(args.Domains) > 16 {
		return fmt.Errorf("domains must contain 1-16 entries")
	}
	if len(args.Ports) > 16 {
		return fmt.Errorf("ports must contain at most 16 entries")
	}
	if args.TTLSeconds != 0 && (args.TTLSeconds < 60 || args.TTLSeconds > 1800) {
		return fmt.Errorf("ttl_seconds must be between 60 and 1800")
	}
	return nil
}

func (t *SandboxNetworkGrantTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	if sandbox.DefaultBroker() == nil {
		return nil, fmt.Errorf("sandbox runtime is not configured")
	}
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var args networkGrantArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	ttl := 15 * time.Minute
	if args.TTLSeconds > 0 {
		ttl = time.Duration(args.TTLSeconds) * time.Second
	}
	grant, err := sandbox.DefaultNetworkGrantStore().Create(owner, args.Domains, args.Ports, ttl)
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(grant)
	return &ToolResult{
		Content:    string(structured),
		Structured: structured,
		Metadata: map[string]interface{}{
			"grant_id":   grant.ID,
			"expires_at": grant.ExpiresAt,
		},
	}, nil
}
