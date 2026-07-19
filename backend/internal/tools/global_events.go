package tools

import "sync"

var globalToolEvents struct {
	sync.RWMutex
	sink EventSink
}

// SetGlobalEventSink installs an application-wide non-blocking lifecycle sink.
// Per-request sinks still receive the same event for live UI updates.
func SetGlobalEventSink(sink EventSink) {
	globalToolEvents.Lock()
	globalToolEvents.sink = sink
	globalToolEvents.Unlock()
}

func emitGlobalEvent(event ToolEvent) {
	globalToolEvents.RLock()
	sink := globalToolEvents.sink
	globalToolEvents.RUnlock()
	if sink != nil {
		sink(event)
	}
}
