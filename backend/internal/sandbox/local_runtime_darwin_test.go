//go:build darwin

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	darwinRuntimeTestAction = "OMNILLM_TEST_DARWIN_RUNTIME_ACTION"
	darwinRuntimeTestTarget = "OMNILLM_TEST_DARWIN_RUNTIME_TARGET"
)

func TestDarwinLocalRuntimeCapabilitiesAreTruthful(t *testing.T) {
	runtime, err := NewLocalRuntime(LocalRuntimeConfig{ScratchRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	capabilities := runtime.Capabilities()
	if capabilities.Name != "darwin-seatbelt" || !capabilities.OSIsolation || !capabilities.FilesystemIsolation || !capabilities.NetworkIsolation {
		t.Fatalf("unexpected Darwin capabilities: %#v", capabilities)
	}
	if capabilities.NetworkAllowlist || capabilities.ProcessTreeIsolation || capabilities.MemoryLimit || capabilities.CPULimit || capabilities.PIDLimit || capabilities.DiskLimit {
		t.Fatalf("Darwin runtime overclaims unsupported controls: %#v", capabilities)
	}
}

func TestDarwinLocalRuntimeReadOnlyWorkspaceDeniesHostReadsWritesAndNetwork(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "input.txt"), []byte("workspace-input\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyDarwinRuntimeTestExecutable(t, filepath.Join(source, "helper-bin"))

	runtime := newDarwinLocalRuntimeForTest(t)
	runtimeID := createDarwinRuntimeForTest(t, runtime, source, MountReadOnly, ResourceLimits{})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()

	result := execDarwinRuntimeHelper(t, runtime, runtimeID, "read", "input.txt")
	if result.ExitCode != 0 || !strings.Contains(result.Stdout, "workspace-input\n") {
		t.Fatalf("workspace read result = %#v", result)
	}

	result = execDarwinRuntimeHelper(t, runtime, runtimeID, "write", "denied.txt")
	if result.ExitCode == 0 {
		t.Fatalf("read-only workspace write unexpectedly succeeded: %#v", result)
	}

	const hostSecret = "OMNILLM-DARWIN-HOST-SECRET-CONTENT"
	outsideRoot := t.TempDir()
	outsidePath := filepath.Join(outsideRoot, "outside.txt")
	if err := os.WriteFile(outsidePath, []byte(hostSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result = execDarwinRuntimeHelper(t, runtime, runtimeID, "read", outsidePath)
	if result.ExitCode == 0 || strings.Contains(result.Stdout, hostSecret) {
		t.Fatalf("host read outside granted roots unexpectedly succeeded: %#v", result)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	result = execDarwinRuntimeHelper(t, runtime, runtimeID, "dial", listener.Addr().String())
	if result.ExitCode == 0 {
		t.Fatalf("network access unexpectedly succeeded: %#v", result)
	}
}

func TestDarwinLocalRuntimeEphemeralWorkspacePersistsWithinSessionAndBoundsOutput(t *testing.T) {
	runtime := newDarwinLocalRuntimeForTest(t)
	runtimeID := createDarwinRuntimeForTest(t, runtime, "", "", ResourceLimits{MaxStdoutBytes: 8, MaxStderrBytes: 8})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()

	result, err := runtime.Exec(context.Background(), runtimeID, ExecRequest{
		Command: "/bin/sh",
		Args:    []string{"-c", "printf first > note.txt"},
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("write ephemeral workspace = %#v, %v", result, err)
	}
	result, err = runtime.Exec(context.Background(), runtimeID, ExecRequest{
		Command: "/bin/sh",
		Args:    []string{"-c", "cat note.txt; printf 0123456789abcdef >&2"},
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("read ephemeral workspace = %#v, %v", result, err)
	}
	if result.Stdout != "first" {
		t.Fatalf("stdout = %q, want first", result.Stdout)
	}
	if len(result.Stderr) != 8 {
		t.Fatalf("stderr length = %d, want 8 bounded bytes; content = %q", len(result.Stderr), result.Stderr)
	}
	if truncated, _ := result.Metadata["stderr_truncated"].(bool); !truncated {
		t.Fatalf("stderr truncation metadata missing: %#v", result.Metadata)
	}
}

func TestDarwinLocalRuntimeSanitizesEnvironmentAndRejectsLoaderOrCredentialOverrides(t *testing.T) {
	source := t.TempDir()
	copyDarwinRuntimeTestExecutable(t, filepath.Join(source, "helper-bin"))
	t.Setenv("OMNILLM_TEST_HOST_SECRET", "must-not-cross-boundary")

	runtime := newDarwinLocalRuntimeForTest(t)
	runtimeID := createDarwinRuntimeForTest(t, runtime, source, MountReadOnly, ResourceLimits{})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()

	result := execDarwinRuntimeHelper(t, runtime, runtimeID, "env", "OMNILLM_TEST_HOST_SECRET")
	if result.ExitCode != 0 || strings.Contains(result.Stdout, "must-not-cross-boundary") {
		t.Fatalf("ambient host secret crossed sandbox boundary: %#v", result)
	}

	for _, environment := range []map[string]string{
		{"API_TOKEN": "secret"},
		{"DYLD_LIBRARY_PATH": "/tmp/escape"},
		{"PATH": "/tmp/escape"},
	} {
		_, err := runtime.Exec(context.Background(), runtimeID, ExecRequest{
			Command: "/bin/echo",
			Args:    []string{"no"},
			Env:     environment,
		})
		if err == nil {
			t.Fatalf("sensitive/runtime-owned environment unexpectedly accepted: %#v", environment)
		}
	}
}

func TestDarwinLocalRuntimeCallerKnownCancellationKillsNormalDescendant(t *testing.T) {
	source := t.TempDir()
	copyDarwinRuntimeTestExecutable(t, filepath.Join(source, "helper-bin"))

	runtime := newDarwinLocalRuntimeForTest(t)
	runtimeID := createDarwinRuntimeForTest(t, runtime, source, MountReadOnly, ResourceLimits{WallTimeMS: 60_000})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()
	session, err := runtime.session(runtimeID)
	if err != nil {
		t.Fatal(err)
	}

	executionID := NewExecutionID()
	type outcome struct {
		result *ExecResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, execErr := runtime.Exec(context.Background(), runtimeID, ExecRequest{
			ExecutionID: executionID,
			Command:     "./helper-bin",
			Args:        []string{"-test.run=^TestDarwinLocalRuntimeHelperProcess$"},
			Env: map[string]string{
				darwinRuntimeTestAction: "spawn",
			},
		})
		done <- outcome{result: result, err: execErr}
	}()

	pidPath := filepath.Join(session.tmp, "child.pid")
	pid := waitForDarwinRuntimeChildPID(t, pidPath)
	if err := runtime.Cancel(context.Background(), runtimeID, executionID); err != nil {
		t.Fatalf("Cancel(%q): %v", executionID, err)
	}
	select {
	case got := <-done:
		if got.result != nil || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("cancelled Exec = %#v, %v", got.result, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled Darwin execution did not finish")
	}
	waitForDarwinPIDGone(t, pid)
	if err := runtime.Cancel(context.Background(), runtimeID, executionID); err == nil {
		t.Fatal("finished execution ID remained cancellable")
	}
}

func TestDarwinLocalRuntimeRejectsUnsafeWorkspaceStaging(t *testing.T) {
	runtime := newDarwinLocalRuntimeForTest(t)

	symlinkSource := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(symlinkSource, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Create(context.Background(), darwinRuntimeCreateRequest(symlinkSource, MountReadOnly, ResourceLimits{})); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symbolic") {
		t.Fatalf("symlink staging error = %v", err)
	}

	hardlinkSource := t.TempDir()
	first := filepath.Join(hardlinkSource, "first.txt")
	second := filepath.Join(hardlinkSource, "second.txt")
	if err := os.WriteFile(first, []byte("same inode"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Create(context.Background(), darwinRuntimeCreateRequest(hardlinkSource, MountReadOnly, ResourceLimits{})); err == nil || !strings.Contains(strings.ToLower(err.Error()), "hard-linked") {
		t.Fatalf("hard-link staging error = %v", err)
	}
}

func TestDarwinLocalRuntimeRejectsWritableMountAndUnapprovedHostExecutable(t *testing.T) {
	runtime := newDarwinLocalRuntimeForTest(t)
	source := t.TempDir()
	if _, err := runtime.Create(context.Background(), darwinRuntimeCreateRequest(source, MountReadWrite, ResourceLimits{})); err == nil || !strings.Contains(err.Error(), string(MountReadOnly)) {
		t.Fatalf("writable mount error = %v", err)
	}

	runtimeID := createDarwinRuntimeForTest(t, runtime, "", "", ResourceLimits{})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()
	unapproved := filepath.Join(t.TempDir(), "host-command")
	copyDarwinRuntimeTestExecutable(t, unapproved)
	if _, err := runtime.Exec(context.Background(), runtimeID, ExecRequest{Command: unapproved}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "approved executable roots") {
		t.Fatalf("unapproved executable error = %v", err)
	}
}

func TestDarwinLocalRuntimeHelperProcess(t *testing.T) {
	action := os.Getenv(darwinRuntimeTestAction)
	if action == "" {
		t.Skip("helper process only")
	}
	target := os.Getenv(darwinRuntimeTestTarget)
	switch action {
	case "read":
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read %q: %v", target, err)
		}
		_, _ = os.Stdout.Write(data)
	case "write":
		if err := os.WriteFile(target, []byte("denied\n"), 0o600); err != nil {
			t.Fatalf("write %q: %v", target, err)
		}
	case "dial":
		connection, err := net.DialTimeout("tcp", target, 750*time.Millisecond)
		if err != nil {
			t.Fatalf("dial %q: %v", target, err)
		}
		_ = connection.Close()
	case "env":
		fmt.Print(os.Getenv(target))
	case "spawn":
		child := exec.Command("/bin/sleep", "30")
		if err := child.Start(); err != nil {
			t.Fatalf("start descendant: %v", err)
		}
		pidPath := filepath.Join(os.Getenv("TMPDIR"), "child.pid")
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_ = child.Process.Kill()
			t.Fatalf("write descendant pid: %v", err)
		}
		if err := child.Wait(); err != nil {
			t.Fatalf("wait descendant: %v", err)
		}
	default:
		t.Fatalf("unknown Darwin runtime helper action %q", action)
	}
}

func newDarwinLocalRuntimeForTest(t *testing.T) *LocalRuntime {
	t.Helper()
	runtime, err := NewLocalRuntime(LocalRuntimeConfig{ScratchRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	local, ok := runtime.(*LocalRuntime)
	if !ok {
		t.Fatalf("NewLocalRuntime() returned %T", runtime)
	}
	return local
}

func createDarwinRuntimeForTest(t *testing.T, runtime *LocalRuntime, source string, mode MountMode, resources ResourceLimits) string {
	t.Helper()
	request := darwinRuntimeCreateRequest(source, mode, resources)
	runtimeID, err := runtime.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return runtimeID
}

func darwinRuntimeCreateRequest(source string, mode MountMode, resources ResourceLimits) RuntimeCreateRequest {
	request := RuntimeCreateRequest{
		SessionID: "sess-darwin-test",
		Owner:     OwnerScope{UserID: "darwin-test-user"},
		Spec: CreateRequest{
			Network:   NetworkPolicy{Mode: NetworkNone},
			Resources: resources,
			Requirements: RuntimeRequirements{
				OSIsolation:         true,
				FilesystemIsolation: true,
				NetworkIsolation:    true,
			},
		},
	}
	if source != "" {
		request.Spec.Mounts = []WorkspaceMount{{WorkspaceID: "workspace-1", Mode: mode}}
		request.ResolvedMounts = []RuntimeMount{{WorkspaceID: "workspace-1", SourcePath: source, Mode: mode}}
	}
	return request
}

func execDarwinRuntimeHelper(t *testing.T, runtime *LocalRuntime, runtimeID, action, target string) *ExecResult {
	t.Helper()
	environment := map[string]string{darwinRuntimeTestAction: action}
	if target != "" {
		environment[darwinRuntimeTestTarget] = target
	}
	result, err := runtime.Exec(context.Background(), runtimeID, ExecRequest{
		Command: "./helper-bin",
		Args:    []string{"-test.run=^TestDarwinLocalRuntimeHelperProcess$"},
		Env:     environment,
	})
	if err != nil {
		t.Fatalf("Exec helper action %q error = %v", action, err)
	}
	return result
}

func copyDarwinRuntimeTestExecutable(t *testing.T, target string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitForDarwinRuntimeChildPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("descendant PID file %q did not appear", path)
	return 0
}

func waitForDarwinPIDGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("descendant pid %d remained alive after cancellation", pid)
}
