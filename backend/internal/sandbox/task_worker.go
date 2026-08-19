package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultSandboxTaskIdleDelay  = 250 * time.Millisecond
	defaultSandboxTaskErrorDelay = time.Second
)

// SandboxTaskWorker owns the application lifecycle for durable sandbox task
// execution. It performs expired-lease/runtime recovery synchronously before the
// background claim loop starts, then reuses one existing Broker for every task.
// The worker never creates a second runtime authority or database connection.
type SandboxTaskWorker struct {
	Executor   *SandboxTaskExecutor
	IdleDelay  time.Duration
	ErrorDelay time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	runErr  error
	started bool
}

// NewSandboxTaskWorker constructs one worker around the application's existing
// durable queue and Broker. Worker IDs are intentionally unique per process
// lifetime; durable lease tokens, rather than a reusable process identity,
// authorize queue mutations.
func NewSandboxTaskWorker(queue *SandboxTaskQueue, broker *Broker) *SandboxTaskWorker {
	return &SandboxTaskWorker{
		Executor: &SandboxTaskExecutor{
			Queue:    queue,
			Broker:   broker,
			WorkerID: "sandbox-worker-" + uuid.NewString(),
		},
	}
}

// Start recovers expired runtime associations before returning and then starts
// the claim loop. A recovery failure is fatal because retrying new work before
// proving the prior process tree is gone could overlap side effects.
func (w *SandboxTaskWorker) Start(parent context.Context) error {
	if w == nil || w.Executor == nil || w.Executor.Queue == nil || w.Executor.Broker == nil {
		return fmt.Errorf("sandbox task worker is not configured")
	}
	if parent == nil {
		parent = context.Background()
	}

	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return fmt.Errorf("sandbox task worker is already started")
	}
	w.started = true
	w.done = make(chan struct{})
	w.runErr = nil
	workerCtx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.mu.Unlock()

	if err := w.Executor.Queue.CleanupExpiredRuntimeAssociations(workerCtx, w.Executor.Broker); err != nil {
		cancel()
		w.finish(fmt.Errorf("recover durable sandbox tasks at startup: %w", err))
		return w.runError()
	}

	go func() {
		w.finish(w.run(workerCtx))
	}()
	return nil
}

func (w *SandboxTaskWorker) run(ctx context.Context) error {
	idleDelay := w.IdleDelay
	if idleDelay <= 0 {
		idleDelay = defaultSandboxTaskIdleDelay
	}
	errorDelay := w.ErrorDelay
	if errorDelay <= 0 {
		errorDelay = defaultSandboxTaskErrorDelay
	}

	for {
		claimed, err := w.Executor.RunOne(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			// An unclaimed error is a recovery/claim infrastructure failure. Stop
			// instead of continuing with an unproven queue/runtime boundary. A
			// claimed task error has already been contained/completed or remains
			// protected by its durable lease, so back off before accepting more.
			if !claimed {
				return err
			}
			if !waitSandboxTaskWorker(ctx, errorDelay) {
				return nil
			}
			continue
		}
		if claimed {
			continue
		}
		if !waitSandboxTaskWorker(ctx, idleDelay) {
			return nil
		}
	}
}

// Shutdown cancels in-flight Broker execution through the executor context and
// waits for its cleanup path to finish. It is safe to call more than once.
func (w *SandboxTaskWorker) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return w.runError()
	case <-ctx.Done():
		return fmt.Errorf("wait for sandbox task worker shutdown: %w", ctx.Err())
	}
}

func (w *SandboxTaskWorker) finish(err error) {
	w.mu.Lock()
	w.runErr = err
	done := w.done
	w.mu.Unlock()
	if done != nil {
		close(done)
	}
}

func (w *SandboxTaskWorker) runError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.runErr
}

func waitSandboxTaskWorker(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
