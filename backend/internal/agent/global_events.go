package agent

import "sync"

var globalAgentEvents struct {
	sync.RWMutex
	sink func(Event)
}

// SetGlobalEventSink installs an application-wide event sink for durable audit
// and replay. Per-request callbacks continue to receive live SSE events.
func SetGlobalEventSink(sink func(Event)) {
	globalAgentEvents.Lock()
	globalAgentEvents.sink = sink
	globalAgentEvents.Unlock()
}

func emitGlobalEvent(event Event) {
	globalAgentEvents.RLock()
	sink := globalAgentEvents.sink
	globalAgentEvents.RUnlock()
	if sink != nil {
		sink(event)
	}
}
