package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type chatExecutionTestTool struct {
	name             string
	readOnly         bool
	sideEffecting    bool
	supportsParallel bool
}

func (t chatExecutionTestTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:             t.name,
		Description:      "chat execution adapter test tool",
		Parameters:       json.RawMessage(`{"type":"object"}`),
		Category:         "test",
		Enabled:          true,
		ReadOnly:         t.readOnly,
		SideEffecting:    t.sideEffecting,
		SupportsParallel: t.supportsParallel,
	}
}

func (t chatExecutionTestTool) Validate(json.RawMessage) error { return nil }
func (t chatExecutionTestTool) Execute(context.Context, json.RawMessage) (*tools.ToolResult, error) {
	return &tools.ToolResult{Content: t.name}, nil
}

func llmTestToolCall(id, name, arguments string) llm.ToolCall {
	call := llm.ToolCall{ID: id, Type: "function"}
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call
}

func TestNewChatToolExecutionPreservesProviderOrderAndArguments(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatExecutionTestTool{name: "first", readOnly: true})
	registry.MustRegister(chatExecutionTestTool{name: "second", readOnly: true})
	executor := tools.NewExecutor(registry, nil, 0)
	calls := []llm.ToolCall{
		llmTestToolCall("call-1", "first", `{"value":1}`),
		llmTestToolCall("call-2", "second", ""),
	}

	execution := newChatToolExecution(executor, calls)
	if len(execution.ProviderCalls) != 2 || len(execution.RuntimeCalls) != 2 {
		t.Fatalf("execution cardinality = provider:%d runtime:%d", len(execution.ProviderCalls), len(execution.RuntimeCalls))
	}
	if execution.RuntimeCalls[0].ID != "call-1" || execution.RuntimeCalls[0].Name != "first" || string(execution.RuntimeCalls[0].Arguments) != `{"value":1}` {
		t.Fatalf("first runtime call = %#v", execution.RuntimeCalls[0])
	}
	if execution.RuntimeCalls[1].ID != "call-2" || execution.RuntimeCalls[1].Name != "second" || string(execution.RuntimeCalls[1].Arguments) != `{}` {
		t.Fatalf("second runtime call = %#v", execution.RuntimeCalls[1])
	}
}

func TestChatToolExecutionKeepsBrowserManagedRoundsSequential(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatExecutionTestTool{name: "calculator", readOnly: true, supportsParallel: true})
	registry.MustRegister(chatExecutionTestTool{name: "browser_navigate", readOnly: true, supportsParallel: true})
	executor := tools.NewExecutor(registry, nil, 0)

	generic := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("1", "calculator", `{}`),
		llmTestToolCall("2", "calculator", `{}`),
	})
	if !generic.genericRuntimeEligible() {
		t.Fatal("expected generic-only round to use ordered runtime")
	}

	browserManaged := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("1", "calculator", `{}`),
		llmTestToolCall("2", "browser_navigate", `{"url":"https://example.com"}`),
	})
	if browserManaged.genericRuntimeEligible() {
		t.Fatal("browser-managed round must remain on existing sequential handler path")
	}
}

func TestNewChatToolExecutionUsesPolicyAwarePlan(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatExecutionTestTool{name: "allowed_read", readOnly: true, supportsParallel: true})
	registry.MustRegister(chatExecutionTestTool{name: "approval_read", readOnly: true, supportsParallel: true})
	executor := tools.NewExecutor(registry, func(name string) string {
		if name == "approval_read" {
			return "ask"
		}
		return "allow"
	}, 0)

	execution := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("1", "allowed_read", `{}`),
		llmTestToolCall("2", "allowed_read", `{}`),
		llmTestToolCall("3", "approval_read", `{}`),
		llmTestToolCall("4", "allowed_read", `{}`),
		llmTestToolCall("5", "allowed_read", `{}`),
	})
	if len(execution.Plan) != 3 {
		t.Fatalf("plan length = %d, want 3: %#v", len(execution.Plan), execution.Plan)
	}
	if !execution.Plan[0].Parallel || len(execution.Plan[0].Calls) != 2 {
		t.Fatalf("first parallel batch = %#v", execution.Plan[0])
	}
	if execution.Plan[1].Parallel || len(execution.Plan[1].Calls) != 1 || execution.Plan[1].Calls[0].Name != "approval_read" {
		t.Fatalf("approval barrier = %#v", execution.Plan[1])
	}
	if !execution.Plan[2].Parallel || len(execution.Plan[2].Calls) != 2 {
		t.Fatalf("last parallel batch = %#v", execution.Plan[2])
	}
}

func TestExecuteChatToolStepPreservesResultOrder(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatExecutionTestTool{name: "read_a", readOnly: true, supportsParallel: true})
	registry.MustRegister(chatExecutionTestTool{name: "read_b", readOnly: true, supportsParallel: true})
	executor := tools.NewExecutor(registry, nil, 0)
	execution := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("call-a", "read_a", `{}`),
		llmTestToolCall("call-b", "read_b", `{}`),
	})
	if len(execution.Plan) != 1 || !execution.Plan[0].Parallel {
		t.Fatalf("parallel plan = %#v", execution.Plan)
	}

	results := executeChatToolStep(context.Background(), executor, execution.Plan[0])
	if len(results) != 2 || results[0].ToolCallID != "call-a" || results[0].Content != "read_a" || results[1].ToolCallID != "call-b" || results[1].Content != "read_b" {
		t.Fatalf("ordered results = %#v", results)
	}
}

func TestExecuteChatToolStepReturnsNilForEmptyInput(t *testing.T) {
	if results := executeChatToolStep(context.Background(), nil, tools.ExecutionStep{}); results != nil {
		t.Fatalf("nil executor results = %#v", results)
	}
}
