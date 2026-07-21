package tools

import (
	"context"
	"errors"
	"net"
)

// RetryableError can be implemented by tools or transport wrappers that know a
// failure is transient and safe to retry.
type RetryableError interface {
	error
	Retryable() bool
}

// IsRetryableExecutionError reports whether an execution error represents a
// transient failure. The executor still retries only read-only tools; this
// helper never makes side-effecting calls retryable on its own. Cancellation
// and deadline errors are terminal for the current invocation and must never
// trigger a replay, even when wrapped by a retry-aware transport error.
func IsRetryableExecutionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.Retryable()
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}
