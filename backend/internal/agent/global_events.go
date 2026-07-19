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

// PublishEvent forwards an event to the application-wide durable sink. API
// transports call this before writing SSE so disconnected clients do not affect
// persistence.
func PublishEvent(event Event) {
	globalAgentEvents.RLock()
	sink := globalAgentEvents.sink
	globalAgentEvents.RUnlock()
	if sink != nil {
		sink(event)
	}
}
