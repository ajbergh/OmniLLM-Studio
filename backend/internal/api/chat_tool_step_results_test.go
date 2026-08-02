package api

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

func TestProcessChatToolStepResultsPreservesOrderAndMessages(t *testing.T) {
	calls := []tools.ToolCall{{ID: "call-a", Name: "read_a"}, {ID: "call-b", Name: "read_b"}}
	results := []*tools.ToolResult{
		{ToolCallID: "call-a", Content: "alpha"},
		{ToolCallID: "call-b", Content: "beta"},
	}

	processed, used, limited := processChatToolStepResults(calls, results, 3, 20)
	if limited {
		t.Fatal("unexpected result limit")
	}
	if used != 12 {
		t.Fatalf("used chars = %d, want 12", used)
	}
	if len(processed) != 2 {
		t.Fatalf("processed count = %d, want 2", len(processed))
	}
	if processed[0].ToolCallID != "call-a" || processed[0].Message.ToolCallID != "call-a" || processed[0].Message.Content != "alpha" {
		t.Fatalf("first processed result = %#v", processed[0])
	}
	if processed[1].ToolCallID != "call-b" || processed[1].Message.ToolCallID != "call-b" || processed[1].Message.Content != "beta" {
		t.Fatalf("second processed result = %#v", processed[1])
	}
}

func TestProcessChatToolStepResultsTruncatesAndMarksRemainingCalls(t *testing.T) {
	calls := []tools.ToolCall{{ID: "call-a", Name: "read_a"}, {ID: "call-b", Name: "read_b"}}
	results := []*tools.ToolResult{
		{ToolCallID: "call-a", Content: "123456"},
		{ToolCallID: "call-b", Content: "second"},
	}

	processed, used, limited := processChatToolStepResults(calls, results, 0, 4)
	if !limited {
		t.Fatal("expected result limit")
	}
	if len(processed) != 2 {
		t.Fatalf("processed count = %d, want 2", len(processed))
	}
	if processed[0].Message.Content != "1234"+toolResultContextTruncationSuffix {
		t.Fatalf("first tool message = %q", processed[0].Message.Content)
	}
	if processed[1].Message.Content != toolResultContextLimitMessage {
		t.Fatalf("second tool message = %q", processed[1].Message.Content)
	}
	wantUsed := len(processed[0].Message.Content) + len(processed[1].Message.Content)
	if used != wantUsed {
		t.Fatalf("used chars = %d, want %d", used, wantUsed)
	}
}

func TestProcessChatToolStepResultsSanitizesMetadata(t *testing.T) {
	calls := []tools.ToolCall{{ID: "timeout-call", Name: "weather_lookup"}}
	results := []*tools.ToolResult{{ToolCallID: "timeout-call", Content: "operation timed out", IsError: true}}

	processed, _, _ := processChatToolStepResults(calls, results, 0, 1000)
	if len(processed) != 1 {
		t.Fatalf("processed count = %d, want 1", len(processed))
	}
	if processed[0].Message.Content != "operation timed out" {
		t.Fatalf("provider tool message = %q", processed[0].Message.Content)
	}
	if code, _ := processed[0].MetadataResult.Metadata["error_code"].(string); code != "TOOL_TIMEOUT" {
		t.Fatalf("error code = %q, metadata=%#v", code, processed[0].MetadataResult.Metadata)
	}
}

func TestProcessChatToolStepResultsReturnsOneMessageForMissingResult(t *testing.T) {
	calls := []tools.ToolCall{{ID: "missing-call", Name: "file_search"}}
	processed, _, _ := processChatToolStepResults(calls, nil, 0, 1000)
	if len(processed) != 1 || processed[0].Message.ToolCallID != "missing-call" {
		t.Fatalf("processed missing result = %#v", processed)
	}
	if !processed[0].MetadataResult.IsError {
		t.Fatalf("missing result metadata = %#v, want error", processed[0].MetadataResult)
	}
}

func TestSkippedChatToolResultsPreservesCardinalityAndLimitMetadata(t *testing.T) {
	calls := []tools.ToolCall{{ID: "skip-a", Name: "task_create"}, {ID: "skip-b", Name: "memory_save"}}
	processed := skippedChatToolResults(calls)
	if len(processed) != 2 {
		t.Fatalf("processed count = %d, want 2", len(processed))
	}
	for index, item := range processed {
		if item.ToolCallID != calls[index].ID || item.Message.ToolCallID != calls[index].ID {
			t.Fatalf("processed[%d] = %#v", index, item)
		}
		if !item.MetadataResult.IsError || item.MetadataResult.Content != toolSkippedAfterContextLimitMessage {
			t.Fatalf("limit result[%d] = %#v", index, item.MetadataResult)
		}
		if code, _ := item.MetadataResult.Metadata["error_code"].(string); code != "TOOL_RESULT_LIMIT" {
			t.Fatalf("limit error code[%d] = %q", index, code)
		}
	}
}
