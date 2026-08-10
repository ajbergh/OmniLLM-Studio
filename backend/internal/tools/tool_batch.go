package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

const maxBatchToolCalls = 8

// ToolBatch executes a bounded sequence of already-registered tools through the
// same Executor. Child calls therefore retain policy, approval, idempotency,
// timeout, result-limit, audit, and request-scope restrictions.
type ToolBatch struct{ executor *Executor }

func NewToolBatch(executor *Executor) *ToolBatch { return &ToolBatch{executor: executor} }

func (t *ToolBatch) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "tool_batch",
		Description:      "Run a bounded ordered batch of registered tools. Every child call still passes through normal tool policy, approvals, and request restrictions.",
		Category:         "orchestration",
		Enabled:          t != nil && t.executor != nil,
		Risk:             RiskHigh,
		SideEffecting:    true,
		ReadOnly:         false,
		SupportsParallel: false,
		DefaultTimeoutMS: 60000,
		MaxResultBytes:   1 << 20,
		Parameters:       json.RawMessage(`{"type":"object","properties":{"calls":{"type":"array","minItems":1,"maxItems":8,"items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string","minLength":1},"arguments":{"type":"object"}},"required":["name","arguments"],"additionalProperties":false}},"continue_on_error":{"type":"boolean","default":false}},"required":["calls"],"additionalProperties":false}`),
	}
}

type batchRequest struct {
	Calls []struct {
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"calls"`
	ContinueOnError bool `json:"continue_on_error"`
}

func (t *ToolBatch) Validate(args json.RawMessage) error {
	var req batchRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return err
	}
	if len(req.Calls) == 0 || len(req.Calls) > maxBatchToolCalls {
		return fmt.Errorf("calls must contain between 1 and %d entries", maxBatchToolCalls)
	}
	for _, call := range req.Calls {
		if call.Name == "" {
			return fmt.Errorf("child tool name is required")
		}
		if call.Name == "tool_batch" {
			return fmt.Errorf("nested tool_batch calls are not allowed")
		}
	}
	return nil
}

func (t *ToolBatch) Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	if t == nil || t.executor == nil {
		return nil, fmt.Errorf("tool executor unavailable")
	}
	var req batchRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	results := make([]*ToolResult, 0, len(req.Calls))
	for index, child := range req.Calls {
		if !ToolAllowedByContext(ctx, child.Name) {
			result := &ToolResult{ToolCallID: child.ID, Content: fmt.Sprintf("tool %q is excluded by the current request restriction", child.Name), IsError: true, Metadata: map[string]interface{}{"error_code": "TOOL_RESTRICTED"}}
			results = append(results, result)
			if !req.ContinueOnError {
				break
			}
			continue
		}
		encoded, _ := json.Marshal(child.Arguments)
		id := child.ID
		if id == "" {
			id = fmt.Sprintf("batch-%d", index+1)
		}
		result := t.executor.Execute(ctx, ToolCall{ID: id, Name: child.Name, Arguments: encoded})
		results = append(results, result)
		if result != nil && result.IsError && !req.ContinueOnError {
			break
		}
	}
	structured, _ := json.Marshal(results)
	contentBytes, _ := json.MarshalIndent(results, "", "  ")
	isError := false
	for _, result := range results {
		if result != nil && result.IsError {
			isError = true
			break
		}
	}
	return &ToolResult{Content: string(contentBytes), Structured: structured, IsError: isError, Metadata: map[string]interface{}{"executed_calls": len(results), "requested_calls": len(req.Calls)}}, nil
}
