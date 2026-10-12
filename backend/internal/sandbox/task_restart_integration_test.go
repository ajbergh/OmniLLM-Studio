package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// A failed remote destroy must not be interpreted as a clean restart: the
// predecessor sandbox process might still be executing externally.
type recoveryRefusingDestroyRuntime struct {
	fakeRuntime
}

func (r *recoveryRefusingDestroyRuntime) Destroy(context.Context, string) error {
	r.destroyCount++
	return errors.New("remote runtime cleanup was not confirmed")
}

func TestSandboxWorkerRestartBlocksReplayUntilStaleRuntimeDestroyed(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return now }
	owner, create, execReq := sandboxTaskFixture()
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryIdempotent)
	if err != nil {
		t.Fatal(err)
	}
	first, attempt, err := queue.Claim(context.Background(), "previous-worker", time.Second)
	if err != nil || first == nil || attempt == nil {
		t.Fatalf("initial claim: task=%#v attempt=%#v err=%v", first, attempt, err)
	}
	if err := queue.BindRuntimeAssociation(
		context.Background(), first.ID, "previous-worker", first.LeaseToken,
		attempt.ID, "sbx_previous", "runtime-previous",
	); err != nil {
		t.Fatal(err)
	}

	// Simulate an expired lease after the original coordinator crashed.
	now = now.Add(2 * time.Second)
	refusing := &recoveryRefusingDestroyRuntime{
		fakeRuntime: fakeRuntime{capabilities: RuntimeCapabilities{Name: "restart-fixture"}},
	}
	broker, err := NewBroker(refusing)
	if err != nil {
		t.Fatal(err)
	}
	failedWorker := NewSandboxTaskWorker(queue, broker)
	err = failedWorker.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "remote runtime cleanup was not confirmed") {
		t.Fatalf("startup must fail closed on unconfirmed runtime destruction: %v", err)
	}
	select {
	case <-failedWorker.Done():
	default:
		t.Fatal("failed worker did not terminate its startup lifecycle")
	}
	if failedWorker.Err() == nil || refusing.destroyCount != 1 || refusing.execCount != 0 {
		t.Fatalf("unconfirmed cleanup ran work: err=%v destroys=%d executions=%d",
			failedWorker.Err(), refusing.destroyCount, refusing.execCount)
	}
	stale, err := queue.Get(context.Background(), task.ID, owner)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Status != SandboxTaskRunning || stale.AttemptCount != 1 ||
		stale.SandboxSessionID != "sbx_previous" || stale.RuntimeID != "runtime-previous" {
		t.Fatalf("failed recovery discarded durable cleanup authority: %#v", stale)
	}

	// Simulate a new process opening the SAME persistent database with a
	// functioning remote worker. Its first step must destroy the old runtime,
	// and only then can an idempotent task receive a second execution attempt.
	restartedQueue, err := NewSandboxTaskQueue(queue.db)
	if err != nil {
		t.Fatal(err)
	}
	restartedQueue.now = func() time.Time { return now }
	working := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "restart-fixture"}}
	newBroker, err := NewBroker(working)
	if err != nil {
		t.Fatal(err)
	}
	restartedWorker := NewSandboxTaskWorker(restartedQueue, newBroker)
	restartedWorker.IdleDelay = 5 * time.Millisecond
	restartedWorker.ErrorDelay = 5 * time.Millisecond
	if err := restartedWorker.Start(context.Background()); err != nil {
		t.Fatalf("restart recovery: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		stored, getErr := restartedQueue.Get(context.Background(), task.ID, owner)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if stored.Status == SandboxTaskSucceeded {
			if stored.AttemptCount != 2 || stored.SandboxSessionID != "" || stored.RuntimeID != "" {
				t.Fatalf("recovered task was not recorded with a fresh completed attempt: %#v", stored)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("restart task did not finish: %#v", stored)
		}
		time.Sleep(5 * time.Millisecond)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := restartedWorker.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if working.execCount != 1 || working.destroyCount != 2 {
		t.Fatalf("expected cleanup of predecessor before exactly one execution: execs=%d destroys=%d",
			working.execCount, working.destroyCount)
	}
}
