package sandbox

import (
	"database/sql"
	"fmt"
)

// NewConfiguredSandboxTaskWorker reuses the process-wide Broker installed by
// application composition and the caller's existing SQLite connection. A nil
// worker means sandbox execution is not configured for this process.
func NewConfiguredSandboxTaskWorker(database *sql.DB) (*SandboxTaskWorker, error) {
	broker := DefaultBroker()
	if broker == nil {
		return nil, nil
	}
	if database == nil {
		return nil, fmt.Errorf("sandbox task worker database is required")
	}
	queue, err := NewSandboxTaskQueue(database)
	if err != nil {
		return nil, fmt.Errorf("initialize durable sandbox task queue: %w", err)
	}
	return NewSandboxTaskWorker(queue, broker), nil
}
