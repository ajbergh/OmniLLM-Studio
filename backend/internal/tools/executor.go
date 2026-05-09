// Package tools provides the dynamic tool registry and execution framework
// for OmniLLM-Studio, supporting both local built-in tools and remote MCP tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultTimeout is the default per-tool execution timeout.
const DefaultTimeout = 30 * time.Second

// PermissionResolver looks up the policy for a given tool.
// Return "allow", "deny", or "ask".  A zero-value ("") is treated as "allow".
type PermissionResolver func(toolName string) string

type approvalContextKey struct{}

const ApprovalStatusMetadataKey = "approval_status"

// ContextWithApprovalHandler attaches a per-request approval handler used for
// tools with "ask" policy.
func ContextWithApprovalHandler(ctx context.Context, handler ApprovalHandler) context.Context {
	if handler == nil {
		return ctx
	}
	return context.WithValue(ctx, approvalContextKey{}, handler)
}

// Executor orchestrates tool lookup, permission checks, and execution.
type Executor struct {
	registry    *Registry
	permissions PermissionResolver
	timeout     time.Duration
}

// NewExecutor creates an Executor with the given registry and permission
// resolver.  If timeout is 0 the DefaultTimeout is used.
func NewExecutor(registry *Registry, permissions PermissionResolver, timeout time.Duration) *Executor {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &Executor{
		registry:    registry,
		permissions: permissions,
		timeout:     timeout,
	}
}

// Execute runs a single tool call.  It validates permissions, validates
// arguments, and executes the tool within the configured timeout.
func (e *Executor) Execute(ctx context.Context, call ToolCall) *ToolResult {
	// 1. Lookup tool
	tool, ok := e.registry.Get(call.Name)
	if !ok {
		return &ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("unknown tool: %s", call.Name),
			IsError:    true,
		}
	}

	// 2. Check if tool definition is enabled
	if !tool.Definition().Enabled {
		return &ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("tool %q is disabled", call.Name),
			IsError:    true,
		}
	}

	// 3. Check permission policy
	if e.permissions != nil {
		policy := e.permissions(call.Name)
		switch policy {
		case "deny":
			return &ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("tool %q is denied by policy", call.Name),
				IsError:    true,
			}
		case "ask":
			handler, _ := ctx.Value(approvalContextKey{}).(ApprovalHandler)
			if handler == nil {
				return &ToolResult{
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool %q requires user approval", call.Name),
					IsError:    true,
					Metadata: map[string]interface{}{
						ApprovalStatusMetadataKey: "required",
					},
				}
			}
			approved, err := handler(ctx, ApprovalRequest{
				ToolCallID:  call.ID,
				ToolName:    call.Name,
				Description: tool.Definition().Description,
				Arguments:   call.Arguments,
			})
			if err != nil {
				return &ToolResult{
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool %q approval failed: %v", call.Name, err),
					IsError:    true,
					Metadata: map[string]interface{}{
						ApprovalStatusMetadataKey: "error",
					},
				}
			}
			if !approved {
				return &ToolResult{
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool %q was rejected by the user", call.Name),
					IsError:    true,
					Metadata: map[string]interface{}{
						ApprovalStatusMetadataKey: "rejected",
					},
				}
			}
			// "allow" or "" → continue
		}
	}

	// 4. Validate arguments
	if err := tool.Validate(call.Arguments); err != nil {
		return &ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("invalid arguments for %q: %v", call.Name, err),
			IsError:    true,
		}
	}

	// 5. Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	result, err := tool.Execute(execCtx, call.Arguments)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				ToolCallID: call.ID,
				Content:    fmt.Sprintf("tool %q timed out after %s", call.Name, e.timeout),
				IsError:    true,
			}
		}
		return &ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("tool %q failed: %v", call.Name, err),
			IsError:    true,
		}
	}

	result.ToolCallID = call.ID
	return result
}

// ExecuteBatch runs multiple tool calls concurrently and returns results
// in the same order as the input calls.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCall) []*ToolResult {
	results := make([]*ToolResult, len(calls))
	var wg sync.WaitGroup
	wg.Add(len(calls))

	for i, call := range calls {
		go func(idx int, c ToolCall) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, c)
		}(i, call)
	}

	wg.Wait()
	return results
}

// ExecuteJSON is a convenience wrapper that accepts a raw JSON tool call,
// unmarshals it, executes, and returns the result.
func (e *Executor) ExecuteJSON(ctx context.Context, raw json.RawMessage) *ToolResult {
	var call ToolCall
	if err := json.Unmarshal(raw, &call); err != nil {
		log.Printf("[tools/executor] failed to unmarshal tool call: %v", err)
		return &ToolResult{
			Content: fmt.Sprintf("malformed tool call: %v", err),
			IsError: true,
		}
	}
	return e.Execute(ctx, call)
}
