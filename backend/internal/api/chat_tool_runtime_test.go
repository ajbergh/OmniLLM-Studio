package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type chatRuntimeTestTool struct{ def tools.ToolDefinition }

func (t chatRuntimeTestTool) Definition() tools.ToolDefinition { return t.def }
func (t chatRuntimeTestTool) Validate(json.RawMessage) error   { return nil }
func (t chatRuntimeTestTool) Execute(context.Context, json.RawMessage) (*tools.ToolResult, error) {
	return &tools.ToolResult{Content: "ok"}, nil
}

func TestSelectChatToolsExcludesDeniedPolicies(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatRuntimeTestTool{def: tools.ToolDefinition{Name: "calculator", Description: "calculate numbers", Parameters: json.RawMessage(`{"type":"object"}`), Enabled: true}})
	registry.MustRegister(chatRuntimeTestTool{def: tools.ToolDefinition{Name: "memory_save", Description: "save a memory", Parameters: json.RawMessage(`{"type":"object"}`), Enabled: true, SideEffecting: true}})
	executor := tools.NewExecutor(registry, func(name string) string {
		if name == "memory_save" {
			return "deny"
		}
		return "allow"
	}, 0)

	selected := selectChatTools(registry, executor, "calculate this and save it to memory")
	seen := map[string]bool{}
	for _, tool := range selected {
		seen[tool.Function.Name] = true
	}
	if !seen["calculator"] {
		t.Fatal("calculator was not selected")
	}
	if seen["memory_save"] {
		t.Fatal("denied memory_save tool was advertised")
	}
}

func TestOrderedToolCallsSupportsSparseIndexes(t *testing.T) {
	first := llm.ToolCall{Index: 5, ID: "five"}
	second := llm.ToolCall{Index: 2, ID: "two"}
	ordered := orderedToolCalls(map[int]*llm.ToolCall{5: &first, 2: &second})
	if len(ordered) != 2 || ordered[0].ID != "two" || ordered[1].ID != "five" {
		t.Fatalf("unexpected order: %+v", ordered)
	}
}

func TestSafeToolResultSanitizesErrors(t *testing.T) {
	result := safeToolResultForMetadata("calculator", &tools.ToolResult{ToolCallID: "c1", Content: `tool "calculator" timed out after 30s`, IsError: true})
	if result.Content != "The tool did not finish before its timeout." {
		t.Fatalf("unexpected user message: %q", result.Content)
	}
	if result.Metadata["error_code"] != "TOOL_TIMEOUT" || result.Metadata["retryable"] != true {
		t.Fatalf("unexpected metadata: %+v", result.Metadata)
	}
}

func TestRequiresComposableToolLoop(t *testing.T) {
	if !requiresComposableToolLoop("Find the latest exchange rate and calculate the converted total") {
		t.Fatal("expected compound current-information request to use composable tools")
	}
	if requiresComposableToolLoop("What is the latest exchange rate?") {
		t.Fatal("simple lookup should retain optimized grounded-search orchestration")
	}
}
