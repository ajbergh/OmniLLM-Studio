package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

func TestSerializedToolEventSinkWritesCompleteFrames(t *testing.T) {
	recorder := httptest.NewRecorder()
	sink := serializedToolEventSink(recorder, recorder)

	const eventCount = 64
	var wg sync.WaitGroup
	wg.Add(eventCount)
	for i := 0; i < eventCount; i++ {
		go func(index int) {
			defer wg.Done()
			sink(tools.ToolEvent{
				Type:       tools.ToolEventProgress,
				ToolCallID: fmt.Sprintf("call-%d", index),
				ToolName:   "parallel_read",
				Data:       map[string]interface{}{"index": index},
			})
		}(i)
	}
	wg.Wait()

	body := strings.TrimSpace(recorder.Body.String())
	frames := strings.Split(body, "\n\n")
	if len(frames) != eventCount {
		t.Fatalf("SSE frame count = %d, want %d; body=%q", len(frames), eventCount, body)
	}
	seen := make(map[string]struct{}, eventCount)
	for _, frame := range frames {
		lines := strings.Split(frame, "\n")
		if len(lines) != 2 {
			t.Fatalf("malformed SSE frame %q", frame)
		}
		if lines[0] != "event: tool_progress" {
			t.Fatalf("unexpected event line %q", lines[0])
		}
		if !strings.HasPrefix(lines[1], "data: ") {
			t.Fatalf("unexpected data line %q", lines[1])
		}
		var event tools.ToolEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &event); err != nil {
			t.Fatalf("decode SSE event %q: %v", lines[1], err)
		}
		if event.ToolCallID == "" {
			t.Fatalf("missing tool call ID in frame %q", frame)
		}
		if _, duplicate := seen[event.ToolCallID]; duplicate {
			t.Fatalf("duplicate tool call ID %q", event.ToolCallID)
		}
		seen[event.ToolCallID] = struct{}{}
	}
}
