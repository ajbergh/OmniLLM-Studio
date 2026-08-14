//go:build windows

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

const windowsKnownCancelHelperEnv = "OMNILLM_WINDOWS_KNOWN_CANCEL_HELPER"

func TestWindowsLocalRuntimeKnownExecutionIDIsCancellableAndDuplicateSafe(t *testing.T) {
	if os.Getenv(windowsKnownCancelHelperEnv) == "1" {
		time.Sleep(30 * time.Second)
		return
	}

	source := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(source, "known-cancel-helper.exe")
	if err := copyWindowsTestFile(executable, helperPath); err != nil {
		t.Fatal(err)
	}

	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_known_cancel",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:    []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network:   NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{WallTimeMS: 30_000},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtimeValue.Destroy(context.Background(), runtimeID) }()

	localRuntime := runtimeValue.(*LocalRuntime)
	executionID := NewExecutionID()
	request := ExecRequest{
		ExecutionID: executionID,
		Command:     "known-cancel-helper.exe",
		Args:        []string{"-test.run=^TestWindowsLocalRuntimeKnownExecutionIDIsCancellableAndDuplicateSafe$"},
		Env:         map[string]string{windowsKnownCancelHelperEnv: "1"},
		TimeoutMS:   30_000,
	}
	done := make(chan error, 1)
	go func() {
		_, err := runtimeValue.Exec(context.Background(), runtimeID, request)
		done <- err
	}()

	waitForWindowsExecutionRegistration(t, localRuntime, runtimeID, executionID)
	if _, err := runtimeValue.Exec(context.Background(), runtimeID, request); err == nil || !strings.Contains(strings.ToLower(err.Error()), "already active") {
		t.Fatalf("duplicate execution id error = %v", err)
	}
	if err := runtimeValue.Cancel(context.Background(), runtimeID, executionID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled execution error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("known-id Windows execution did not cancel")
	}
	if err := runtimeValue.Cancel(context.Background(), runtimeID, executionID); err == nil {
		t.Fatal("finished execution id remained cancellable")
	}
}

func waitForWindowsExecutionRegistration(t *testing.T, runtime *LocalRuntime, runtimeID, executionID string) {
	t.Helper()
	key := runtimeID + "\x00" + executionID
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.mu.RLock()
		_, ok := runtime.active[key]
		runtime.mu.RUnlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Windows execution id was not registered while Exec was running")
}
