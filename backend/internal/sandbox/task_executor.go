package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SandboxTaskExecutor leases durable sandbox work and executes it through the
// existing Broker. The executor may run in the API control plane or a dedicated
// coordinator; arbitrary code still executes only through the configured
// sandbox Runtime (for server deployments, normally the authenticated HTTP
// sandboxd worker).
type SandboxTaskExecutor struct {
	Queue    *SandboxTaskQueue
	Broker   *Broker
	WorkerID string
	Lease    time.Duration
}

// RunOne executes at most one queued task and returns whether work was claimed.
// Before claiming anything, recovery destroys every runtime attached to an
// expired lease. A cleanup failure stops the worker instead of allowing a retry
// to overlap an unproven prior process tree.
//
// Lease renewal runs independently of the sandbox wall timeout. If lease
// ownership is lost, the known caller-provided execution ID is cancelled and the
// task is not completed by the stale worker.
func (e *SandboxTaskExecutor) RunOne(ctx context.Context) (bool, error) {
	if e == nil || e.Queue == nil || e.Broker == nil {
		return false, fmt.Errorf("sandbox task executor is not configured")
	}
	workerID := strings.TrimSpace(e.WorkerID)
	if workerID == "" {
		return false, fmt.Errorf("sandbox task executor worker id is required")
	}
	lease := e.Lease
	if lease <= 0 {
		lease = defaultSandboxTaskLease
	}
	if err := e.Queue.CleanupExpiredRuntimeAssociations(ctx, e.Broker); err != nil {
		return false, fmt.Errorf("recover expired sandbox work: %w", err)
	}
	task, attempt, err := e.Queue.Claim(ctx, workerID, lease)
	if err != nil || task == nil {
		return task != nil, err
	}

	owner := task.Owner
	create := cloneCreateRequest(task.Create)
	session, err := e.Broker.Create(ctx, owner, create)
	if err != nil {
		_ = e.Queue.Complete(context.Background(), task.ID, workerID, task.LeaseToken, attempt.ID, nil, err)
		return true, err
	}
	if err := e.Queue.BindRuntimeAssociation(
		ctx,
		task.ID,
		workerID,
		task.LeaseToken,
		attempt.ID,
		session.ID,
		session.RuntimeID,
	); err != nil {
		_ = e.Broker.Destroy(context.Background(), owner, session.ID)
		return true, err
	}

	execReq := task.Exec
	execReq.ExecutionID = attempt.ExecutionID
	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	renewEvery := lease / 3
	if renewEvery < time.Second {
		renewEvery = time.Second
	}
	heartbeatDone := make(chan struct{})
	heartbeatErr := make(chan error, 1)
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(renewEvery)
		defer ticker.Stop()
		for {
			select {
			case <-execCtx.Done():
				return
			case <-ticker.C:
				if err := e.Queue.Renew(context.Background(), task.ID, workerID, task.LeaseToken, lease); err != nil {
					select {
					case heartbeatErr <- err:
					default:
					}
					cancelExec()
					return
				}
			}
		}
	}()

	result, runErr := e.Broker.Exec(execCtx, owner, session.ID, execReq)
	cancelExec()
	<-heartbeatDone
	select {
	case leaseErr := <-heartbeatErr:
		_ = e.Broker.Cancel(context.Background(), owner, session.ID, attempt.ExecutionID)
		_ = e.Broker.Destroy(context.Background(), owner, session.ID)
		return true, fmt.Errorf("sandbox task lease lost during execution: %w", leaseErr)
	default:
	}

	// Destroy before recording terminal success so no session is intentionally
	// left alive after a task has become immutable completed history.
	destroyErr := e.Broker.Destroy(context.Background(), owner, session.ID)
	if runErr == nil && destroyErr != nil {
		runErr = fmt.Errorf("destroy sandbox after task execution: %w", destroyErr)
	}
	if err := e.Queue.Complete(ctx, task.ID, workerID, task.LeaseToken, attempt.ID, result, runErr); err != nil {
		return true, err
	}
	return true, runErr
}
