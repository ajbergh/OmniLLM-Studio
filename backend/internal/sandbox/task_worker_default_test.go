package sandbox

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewConfiguredSandboxTaskWorkerDisabledWithoutDefaultBroker(t *testing.T) {
	previous := DefaultBroker()
	SetDefaultBroker(nil)
	t.Cleanup(func() { SetDefaultBroker(previous) })

	worker, err := NewConfiguredSandboxTaskWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	if worker != nil {
		t.Fatalf("worker = %#v, want nil when sandbox Broker is not configured", worker)
	}
}

func TestNewConfiguredSandboxTaskWorkerReusesDefaultBrokerAndDatabase(t *testing.T) {
	previous := DefaultBroker()
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	SetDefaultBroker(broker)
	t.Cleanup(func() { SetDefaultBroker(previous) })

	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "sandbox-worker-composition.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	worker, err := NewConfiguredSandboxTaskWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	if worker == nil || worker.Executor == nil {
		t.Fatal("configured sandbox did not create durable worker")
	}
	if worker.Executor.Broker != broker {
		t.Fatal("durable worker did not reuse process-wide Broker")
	}
	if worker.Executor.Queue == nil || worker.Executor.Queue.db != database {
		t.Fatal("durable worker did not reuse application SQLite connection")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
