package sandbox

import "sync/atomic"

var processDefaultBroker atomic.Pointer[Broker]

// SetDefaultBroker installs the process-wide Broker selected by the application
// composition root. It exists as a narrow compatibility seam for core tools
// that are created by Registry before service-backed tools are wired. New code
// should prefer explicit Broker injection where practical.
func SetDefaultBroker(broker *Broker) {
	processDefaultBroker.Store(broker)
}

// DefaultBroker returns the process-wide Broker selected by the application.
func DefaultBroker() *Broker {
	return processDefaultBroker.Load()
}
