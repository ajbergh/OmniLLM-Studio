package sandbox

import (
	"context"
	"testing"
	"time"
)

// Crash recovery must never grant a second attempt to non-idempotent work,
// even when the orphaned runtime has been successfully destroyed. Queue unit
// tests cover policy; this exercises the real worker startup/claim lifecycle.
func TestSandboxWorkerRestartNeverReplaysNonIdempotentTask(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return now }
	owner, create, execReq := sandboxTaskFixture()
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever)
	if err != nil {
		t.Fatal(err)
	}
	old, attempt, err := queue.Claim(context.Background(), "abandoned-worker", time.Second)
	if err != nil || old == nil || attempt == nil {
		t.Fatalf("claim predecessor: task=%#v attempt=%#v err=%v", old, attempt, err)
	}
	if err := queue.BindRuntimeAssociation(
		context.Background(), old.ID, "abandoned-worker", old.LeaseToken,
		attempt.ID, "sbx_orphan", "runtime-orphan",
	); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)

	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "restart-nonidempotent-fixture"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSandboxTaskQueue(queue.db)
	if err != nil {
		t.Fatal(err)
	}
	reopened.now = func() time.Time { return now }
	worker := NewSandboxTaskWorker(reopened, broker)
	worker.IdleDelay = 5 * time.Millisecond
	if worker.Executor.WorkerID == "abandoned-worker" {
		t.Fatal("restarted worker reused predecessor identity")
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		stored, err := reopened.Get(context.Background(), task.ID, owner)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status == SandboxTaskFailed {
			if stored.AttemptCount != 1 || stored.SandboxSessionID != "" ||
				stored.RuntimeID != "" {
				t.Fatalf("failed recovery left stale identity or replayed work: %#v", stored)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("non-idempotent task was not marked failed after restart: %#v", stored)
		}
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if runtime.destroyCount != 1 || runtime.execCount != 0 {
		t.Fatalf("restart must destroy predecessor, never execute again: destroys=%d executions=%d",
			runtime.destroyCount, runtime.execCount)
	}
}

// A worker that loses SQLite authority after startup must publish a terminal
// error through Done/Err and return it from Shutdown. A supervising application
// can then fail closed instead of serving with an inert durable task worker.
func TestSandboxWorkerPostStartQueueFailureIsObservable(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	broker, err := NewBroker(&fakeRuntime{capabilities: RuntimeCapabilities{Name: "queue-failure-fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewSandboxTaskWorker(queue, broker)
	worker.IdleDelay = 5 * time.Millisecond
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := queue.db.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-worker.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after durable queue became inaccessible")
	}
	if worker.Err() == nil {
		t.Fatal("post-start queue failure was hidden from supervisor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err == nil {
		t.Fatal("shutdown hid the terminal worker failure")
	}
}
