package sandbox

import (
	"context"
	"testing"
	"time"
)

func TestSandboxTaskWorkerExecutesQueuedTaskAndShutsDown(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner, create, execReq := sandboxTaskFixture()
	task, err := queue.Enqueue(context.Background(), owner, create, execReq, SandboxRetryNever)
	if err != nil {
		t.Fatal(err)
	}

	worker := NewSandboxTaskWorker(queue, broker)
	worker.IdleDelay = 5 * time.Millisecond
	worker.ErrorDelay = 5 * time.Millisecond
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		stored, err := queue.Get(context.Background(), task.ID, owner)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status == SandboxTaskSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task status = %q, want %q", stored.Status, SandboxTaskSucceeded)
		}
		time.Sleep(5 * time.Millisecond)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if runtime.execCount != 1 || runtime.destroyCount != 1 {
		t.Fatalf("runtime executions=%d destroys=%d, want 1/1", runtime.execCount, runtime.destroyCount)
	}
}

func TestSandboxTaskWorkerRejectsInvalidOrDuplicateStart(t *testing.T) {
	if err := (&SandboxTaskWorker{}).Start(context.Background()); err == nil {
		t.Fatal("unconfigured worker unexpectedly started")
	}

	queue := newSandboxTaskQueueForTest(t)
	broker, err := NewBroker(&fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewSandboxTaskWorker(queue, broker)
	worker.IdleDelay = 5 * time.Millisecond
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := worker.Start(context.Background()); err == nil {
		t.Fatal("duplicate Start() unexpectedly succeeded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxTaskWorkerParentCancellationStopsIdleLoop(t *testing.T) {
	queue := newSandboxTaskQueueForTest(t)
	broker, err := NewBroker(&fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewSandboxTaskWorker(queue, broker)
	worker.IdleDelay = time.Hour
	parent, cancelParent := context.WithCancel(context.Background())
	if err := worker.Start(parent); err != nil {
		t.Fatal(err)
	}
	cancelParent()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
