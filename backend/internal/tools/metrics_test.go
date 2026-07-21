package tools

import (
	"context"
	"testing"
)

func TestToolMetricsSnapshotAggregatesTerminalEvents(t *testing.T) {
	resetToolMetricsForTest()
	t.Cleanup(resetToolMetricsForTest)

	emitEvent(context.Background(), ToolEvent{
		Type: ToolEventCompleted, ToolCallID: "one", ToolName: "calculator",
		Data: map[string]interface{}{"duration_ms": int64(12), "attempt_count": 2, "is_error": false},
	})
	emitEvent(context.Background(), ToolEvent{
		Type: ToolEventTimedOut, ToolCallID: "two", ToolName: "calculator",
		Data: map[string]interface{}{"duration_ms": int64(30), "attempt_count": 1},
	})
	emitEvent(context.Background(), ToolEvent{
		Type: ToolEventFailed, ToolCallID: "three", ToolName: "browser_navigate",
		Data: map[string]interface{}{"duration_ms": int64(8), "attempt_count": 1},
	})

	snapshot := ToolMetricsSnapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot length = %d, want 2: %#v", len(snapshot), snapshot)
	}
	if snapshot[0].ToolName != "browser_navigate" || snapshot[1].ToolName != "calculator" {
		t.Fatalf("snapshot ordering = %#v", snapshot)
	}
	calculator := snapshot[1]
	if calculator.Calls != 2 || calculator.Successes != 1 || calculator.Timeouts != 1 {
		t.Fatalf("calculator metrics = %#v", calculator)
	}
	if calculator.Retries != 1 || calculator.TotalDurationMS != 42 || calculator.LastDurationMS != 30 {
		t.Fatalf("calculator timing/retry metrics = %#v", calculator)
	}
}

func TestToolMetricsSnapshotTracksCancellation(t *testing.T) {
	resetToolMetricsForTest()
	t.Cleanup(resetToolMetricsForTest)

	emitEvent(context.Background(), ToolEvent{
		Type: ToolEventCancelled, ToolCallID: "cancelled", ToolName: "file_search",
		Data: map[string]interface{}{"duration_ms": int64(3)},
	})
	snapshot := ToolMetricsSnapshot()
	if len(snapshot) != 1 || snapshot[0].Cancellations != 1 || snapshot[0].Calls != 1 {
		t.Fatalf("cancellation metrics = %#v", snapshot)
	}
}
