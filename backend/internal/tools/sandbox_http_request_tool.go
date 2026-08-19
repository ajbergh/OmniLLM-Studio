package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// SandboxHTTPRequestTool performs destination-enforced HTTP through trusted host
// code. It is intentionally separate from terminal_exec: first-party arbitrary
// process runtimes remain no-network until their OS-level socket allowlisting is
// independently implemented and proven.
type SandboxHTTPRequestTool struct{}

func NewSandboxHTTPRequestTool() *SandboxHTTPRequestTool { return &SandboxHTTPRequestTool{} }

func (t *SandboxHTTPRequestTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "sandbox_http_request",
		Description:      "Make a bounded HTTP(S) request through the trusted sandbox network broker using an owner-bound network_grant_id. DNS answers, destination ports, redirects, private addresses, proxy headers, and response size are enforced by the broker. An optional credential_handle_id is injected only by a trusted destination-bound service adapter; raw secrets are never returned to the sandbox.",
		Category:         "network",
		Enabled:          sandbox.DefaultBrokeredHTTPClient() != nil,
		Version:          "1",
		Risk:             RiskHigh,
		ReadOnly:         false,
		SideEffecting:    true,
		RequiresNetwork:  true,
		SupportsParallel: false,
		DefaultTimeoutMS: 30000,
		MaxResultBytes:   1 << 20,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"network_grant_id":{"type":"string","minLength":1,"maxLength":128},"method":{"type":"string","enum":["GET","HEAD","POST","PUT","PATCH","DELETE"]},"url":{"type":"string","minLength":1,"maxLength":8192},"headers":{"type":"object","maxProperties":32,"additionalProperties":{"type":"string","maxLength":4096}},"body":{"type":"string","maxLength":1048576},"credential_handle_id":{"type":"string","maxLength":128,"description":"Optional opaque host-side credential handle. The registered service adapter must allow the exact destination."}},"required":["network_grant_id","url"],"additionalProperties":false}`),
	}
}

type sandboxHTTPRequestArgs struct {
	NetworkGrantID     string            `json:"network_grant_id"`
	Method             string            `json:"method"`
	URL                string            `json:"url"`
	Headers            map[string]string `json:"headers"`
	Body               string            `json:"body"`
	CredentialHandleID string            `json:"credential_handle_id"`
}

func (t *SandboxHTTPRequestTool) Validate(raw json.RawMessage) error {
	var args sandboxHTTPRequestArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if !strings.HasPrefix(strings.TrimSpace(args.NetworkGrantID), "sng_") {
		return fmt.Errorf("network_grant_id is invalid")
	}
	if len(args.URL) == 0 || len(args.URL) > 8192 || strings.ContainsRune(args.URL, '\x00') {
		return fmt.Errorf("url is invalid")
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return fmt.Errorf("method is not supported")
	}
	if len(args.Headers) > 32 {
		return fmt.Errorf("headers contains too many entries")
	}
	for key, value := range args.Headers {
		if len(key) > 128 || len(value) > 4096 || strings.ContainsAny(key+value, "\r\n\x00") {
			return fmt.Errorf("header %q is invalid", key)
		}
	}
	if len(args.Body) > 1<<20 {
		return fmt.Errorf("body exceeds 1 MiB")
	}
	if handle := strings.TrimSpace(args.CredentialHandleID); handle != "" && (!strings.HasPrefix(handle, "sch_") || len(handle) > 128) {
		return fmt.Errorf("credential_handle_id is invalid")
	}
	return nil
}

func (t *SandboxHTTPRequestTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	owner, err := sandboxOwnerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var args sandboxHTTPRequestArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}
	response, err := sandbox.DefaultBrokeredHTTPClient().Do(ctx, owner, sandbox.BrokeredHTTPRequest{
		GrantID:            strings.TrimSpace(args.NetworkGrantID),
		Method:             method,
		URL:                strings.TrimSpace(args.URL),
		Headers:            cloneToolStringMap(args.Headers),
		Body:               []byte(args.Body),
		CredentialHandleID: strings.TrimSpace(args.CredentialHandleID),
	})
	if err != nil {
		return nil, err
	}
	structured, _ := json.Marshal(response)
	content := string(response.Body)
	if content == "" {
		content = fmt.Sprintf("Brokered HTTP request returned status %d", response.StatusCode)
	}
	return &ToolResult{
		Content:    content,
		Structured: structured,
		IsError:    response.StatusCode >= 400,
		Metadata: map[string]interface{}{
			"status_code": response.StatusCode,
			"network":     "brokered_allowlist",
			"credential":  strings.TrimSpace(args.CredentialHandleID) != "",
		},
	}, nil
}

func cloneToolStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
