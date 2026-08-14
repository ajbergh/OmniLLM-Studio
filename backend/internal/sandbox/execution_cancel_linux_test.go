//go:build linux

package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxLocalRuntimeKnownExecutionIDIsCancellableAndDuplicateSafe(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "fake-bwrap")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	runtimeID := "rt_test_known_cancel"
	runtime := &LocalRuntime{
		rootFS:      t.TempDir(),
		scratchRoot: t.TempDir(),
		bwrapPath:   helper,
		sessions:    map[string]localRuntimeSession{},
		active:      map[string]context.CancelFunc{},
	}
	runtime.sessions[runtimeID] = localRuntimeSession{
		id:      runtimeID,
		spec:    CreateRequest{Resources: ResourceLimits{WallTimeMS: 30_000}},
		scratch: t.TempDir(),
	}

	executionID := NewExecutionID()
	request := ExecRequest{ExecutionID: executionID, Command: "ignored", TimeoutMS: 30_000}
	done := make(chan error, 1)
	go func() {
		_, err := runtime.Exec(context.Background(), runtimeID, request)
		done <- err
	}()

	waitForLinuxExecutionRegistration(t, runtime, runtimeID, executionID)
	if _, err := runtime.Exec(context.Background(), runtimeID, request); err == nil || !strings.Contains(strings.ToLower(err.Error()), "already active") {
		t.Fatalf("duplicate execution id error = %v", err)
	}
	if err := runtime.Cancel(context.Background(), runtimeID, executionID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled execution error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("known-id Linux execution did not cancel")
	}
	if err := runtime.Cancel(context.Background(), runtimeID, executionID); err == nil {
		t.Fatal("finished execution id remained cancellable")
	}
}

func waitForLinuxExecutionRegistration(t *testing.T, runtime *LocalRuntime, runtimeID, executionID string) {
	t.Helper()
	key := runtimeID + "\x00" + executionID
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.mu.RLock()
		_, ok := runtime.active[key]
		runtime.mu.RUnlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Linux execution id was not registered while Exec was running")
}
