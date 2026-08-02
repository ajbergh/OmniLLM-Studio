package api

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type chatRoundTestTool struct {
	name             string
	content          string
	readOnly         bool
	sideEffecting    bool
	supportsParallel bool
	delay            time.Duration
	executions       *atomic.Int32
}

func (t chatRoundTestTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:             t.name,
		Description:      "chat round integration test tool",
		Parameters:       json.RawMessage(`{"type":"object"}`),
		Category:         "test",
		Enabled:          true,
		ReadOnly:         t.readOnly,
		SideEffecting:    t.sideEffecting,
		SupportsParallel: t.supportsParallel,
	}
}

func (t chatRoundTestTool) Validate(json.RawMessage) error { return nil }

func (t chatRoundTestTool) Execute(ctx context.Context, _ json.RawMessage) (*tools.ToolResult, error) {
	if t.executions != nil {
		t.executions.Add(1)
	}
	if t.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(t.delay):
		}
	}
	return &tools.ToolResult{Content: t.content}, nil
}

func TestExecuteGenericChatToolRoundStopsBeforeLaterSideEffectAfterLimit(t *testing.T) {
	registry := tools.NewRegistry()
	var writeExecutions atomic.Int32
	registry.MustRegister(chatRoundTestTool{
		name:      "large_read",
		content:   "123456",
		readOnly: true,
	})
	registry.MustRegister(chatRoundTestTool{
		name:          "write_action",
		content:       "written",
		sideEffecting: true,
		executions:    &writeExecutions,
	})
	executor := tools.NewExecutor(registry, nil, 0)
	execution := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("read-call", "large_read", `{}`),
		llmTestToolCall("write-call", "write_action", `{}`),
	})

	outcome := executeGenericChatToolRound(context.Background(), executor, execution.Plan, 0, 4)
	if !outcome.LimitReached {
		t.Fatal("expected result context limit")
	}
	if got := writeExecutions.Load(); got != 0 {
		t.Fatalf("write executions = %d, want 0", got)
	}
	if len(outcome.Processed) != 2 {
		t.Fatalf("processed results = %d, want 2", len(outcome.Processed))
	}
	if outcome.Processed[0].ToolCallID != "read-call" || outcome.Processed[1].ToolCallID != "write-call" {
		t.Fatalf("processed order = %#v", outcome.Processed)
	}
	if code, _ := outcome.Processed[1].MetadataResult.Metadata["error_code"].(string); code != "TOOL_RESULT_LIMIT" {
		t.Fatalf("skipped write error code = %q", code)
	}
}

func TestExecuteGenericChatToolRoundPreservesParallelResultOrder(t *testing.T) {
	registry := tools.NewRegistry()
	registry.MustRegister(chatRoundTestTool{
		name:             "slow_read",
		content:          "slow",
		readOnly:         true,
		supportsParallel: true,
		delay:            20 * time.Millisecond,
	})
	registry.MustRegister(chatRoundTestTool{
		name:             "fast_read",
		content:          "fast",
		readOnly:         true,
		supportsParallel: true,
	})
	executor := tools.NewExecutor(registry, nil, 0)
	execution := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("slow-call", "slow_read", `{}`),
		llmTestToolCall("fast-call", "fast_read", `{}`),
	})
	if len(execution.Plan) != 1 || !execution.Plan[0].Parallel {
		t.Fatalf("plan = %#v, want one parallel step", execution.Plan)
	}

	outcome := executeGenericChatToolRound(context.Background(), executor, execution.Plan, 0, 1000)
	if outcome.LimitReached {
		t.Fatal("unexpected result context limit")
	}
	if len(outcome.Processed) != 2 {
		t.Fatalf("processed results = %d, want 2", len(outcome.Processed))
	}
	if outcome.Processed[0].ToolCallID != "slow-call" || outcome.Processed[0].Message.Content != "slow" {
		t.Fatalf("first result = %#v", outcome.Processed[0])
	}
	if outcome.Processed[1].ToolCallID != "fast-call" || outcome.Processed[1].Message.Content != "fast" {
		t.Fatalf("second result = %#v", outcome.Processed[1])
	}
}

func TestExecuteGenericChatToolRoundSkipsAllWhenBudgetAlreadyExhausted(t *testing.T) {
	registry := tools.NewRegistry()
	var executions atomic.Int32
	registry.MustRegister(chatRoundTestTool{
		name:       "read_once",
		content:    "unused",
		readOnly:   true,
		executions: &executions,
	})
	executor := tools.NewExecutor(registry, nil, 0)
	execution := newChatToolExecution(executor, []llm.ToolCall{
		llmTestToolCall("read-call", "read_once", `{}`),
	})

	outcome := executeGenericChatToolRound(context.Background(), executor, execution.Plan, 10, 10)
	if !outcome.LimitReached {
		t.Fatal("expected exhausted budget")
	}
	if got := executions.Load(); got != 0 {
		t.Fatalf("executions = %d, want 0", got)
	}
	if len(outcome.Processed) != 1 || outcome.Processed[0].ToolCallID != "read-call" {
		t.Fatalf("processed results = %#v", outcome.Processed)
	}
}
