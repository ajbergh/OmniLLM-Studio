package api

import (
	"net/http"
	"sync"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// serializedToolEventSink returns a request-local event sink that writes one
// complete SSE frame at a time. Planner-approved parallel tool steps execute in
// worker goroutines, but http.ResponseWriter and http.Flusher are not safe for
// concurrent use. Holding the lock across sendToolEventSSE also keeps each event
// and its flush atomic relative to the other tool lifecycle events in the turn.
func serializedToolEventSink(w http.ResponseWriter, flusher http.Flusher) tools.EventSink {
	var mu sync.Mutex
	return func(event tools.ToolEvent) {
		mu.Lock()
		defer mu.Unlock()
		sendToolEventSSE(w, flusher, event)
	}
}
