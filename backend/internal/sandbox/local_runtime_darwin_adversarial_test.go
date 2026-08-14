//go:build darwin

package sandbox

import (
	"context"
	"errors"
	"fmt"
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
	darwin13DAction      = "OMNILLM_DARWIN_13D_ACTION"
	darwin13DCrossPath   = "OMNILLM_DARWIN_13D_CROSS_PATH"
	darwin13DOutsidePath = "OMNILLM_DARWIN_13D_OUTSIDE_PATH"
	darwin13DWritePath   = "OMNILLM_DARWIN_13D_WRITE_PATH"
	darwin13DLoopback    = "OMNILLM_DARWIN_13D_LOOPBACK"
)

func TestDarwinAdversarialStagingRejectsSourceSwapAfterObservation(t *testing.T) {
	source := t.TempDir()
	path := filepath.Join(source, "input.txt")
	if err := os.WriteFile(path, []byte("trusted-before-swap"), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside-secret-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "staged.txt")
	if _, err := stageDarwinReadOnlyFile(path, target, "input.txt", observed); err == nil || !strings.Contains(err.Error(), "changed while staging") {
		t.Fatalf("source-swap staging error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("source-swap staging created destination unexpectedly: %v", err)
	}
}

func TestDarwinAdversarialRuntimeDeniesCrossRuntimeAndSymlinkAliases(t *testing.T) {
	if action := os.Getenv(darwin13DAction); action != "" {
		darwin13DRuntimeHelper(t, action)
		return
	}
	runtime := newDarwinLocalRuntimeForTest(t)
	firstID := createDarwinRuntimeForTest(t, runtime, "", "", ResourceLimits{})
	defer func() { _ = runtime.Destroy(context.Background(), firstID) }()
	secondID := createDarwinRuntimeForTest(t, runtime, "", "", ResourceLimits{})
	defer func() { _ = runtime.Destroy(context.Background(), secondID) }()

	first, err := runtime.session(firstID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.session(secondID)
	if err != nil {
		t.Fatal(err)
	}
	crossPath := filepath.Join(first.home, "authority.txt")
	if err := os.WriteFile(crossPath, []byte("cross-runtime-secret-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	writePath := filepath.Join(outsideDir, "write.txt")
	if err := os.WriteFile(outsidePath, []byte("outside-secret-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	helper := filepath.Join(second.workspace, "adversarial-helper")
	copyDarwinRuntimeTestExecutable(t, helper)
	result, err := runtime.Exec(context.Background(), secondID, ExecRequest{
		Command: "./adversarial-helper",
		Args:    []string{"-test.run=^TestDarwinAdversarialRuntimeDeniesCrossRuntimeAndSymlinkAliases$"},
		Env: map[string]string{
			darwin13DAction:      "alias",
			darwin13DCrossPath:   crossPath,
			darwin13DOutsidePath: outsidePath,
			darwin13DWritePath:   writePath,
			darwin13DLoopback:    listener.Addr().String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("adversarial helper failed: %#v", result)
	}
	for _, expected := range []string{"cross_read=denied", "symlink_read=denied", "symlink_write=denied", "network=denied"} {
		if !strings.Contains(result.Stdout, expected) {
			t.Fatalf("adversarial output missing %q: %#v", expected, result)
		}
	}
	if strings.Contains(result.Stdout, "cross-runtime-secret-content") || strings.Contains(result.Stdout, "outside-secret-content") {
		t.Fatalf("adversarial helper leaked protected content: %#v", result)
	}
	if _, err := os.Stat(writePath); !os.IsNotExist(err) {
		t.Fatalf("symlink alias wrote outside runtime root: %v", err)
	}
}

func TestDarwinAdversarialDetachedRuntimeDescendantRemainsConfinedAfterCancel(t *testing.T) {
	if action := os.Getenv(darwin13DAction); action != "" {
		darwin13DRuntimeHelper(t, action)
		return
	}
	runtime := newDarwinLocalRuntimeForTest(t)
	runtimeID := createDarwinRuntimeForTest(t, runtime, "", "", ResourceLimits{WallTimeMS: 60_000})
	defer func() { _ = runtime.Destroy(context.Background(), runtimeID) }()
	session, err := runtime.session(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(session.workspace, "detached-helper")
	copyDarwinRuntimeTestExecutable(t, helper)
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	writePath := filepath.Join(outsideDir, "write.txt")
	if err := os.WriteFile(outsidePath, []byte("detached-outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	executionID := NewExecutionID()
	type outcome struct {
		result *ExecResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, execErr := runtime.Exec(context.Background(), runtimeID, ExecRequest{
			ExecutionID: executionID,
			Command:     "./detached-helper",
			Args:        []string{"-test.run=^TestDarwinAdversarialDetachedRuntimeDescendantRemainsConfinedAfterCancel$"},
			Env: map[string]string{
				darwin13DAction:      "spawn-detached",
				darwin13DOutsidePath: outsidePath,
				darwin13DWritePath:   writePath,
				darwin13DLoopback:    listener.Addr().String(),
			},
		})
		done <- outcome{result: result, err: execErr}
	}()

	pid := waitForDarwin13DPID(t, filepath.Join(session.tmp, "detached.pid"))
	if err := runtime.Cancel(context.Background(), runtimeID, executionID); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.result != nil || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("cancelled root execution = %#v, %v", got.result, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("root execution did not finish after cancellation")
	}

	report := waitForDarwin13DReport(t, filepath.Join(session.tmp, "detached.report"))
	for _, expected := range []string{"outside_read=denied", "outside_write=denied", "network=denied"} {
		if !strings.Contains(report, expected) {
			t.Fatalf("detached report missing %q: %q", expected, report)
		}
	}
	if strings.Contains(report, "detached-outside-secret") {
		t.Fatalf("detached process leaked host content: %q", report)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("detached pid %d did not survive process-group cancellation as expected by process_tree_isolation=false: %v", pid, err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("cleanup detached pid %d: %v", pid, err)
	}
	waitForDarwin13DPIDGone(t, pid)
	if _, err := os.Stat(writePath); !os.IsNotExist(err) {
		t.Fatalf("detached descendant wrote outside runtime root: %v", err)
	}
}

func darwin13DRuntimeHelper(t *testing.T, action string) {
	t.Helper()
	switch action {
	case "alias":
		if data, err := os.ReadFile(os.Getenv(darwin13DCrossPath)); err != nil {
			fmt.Println("cross_read=denied")
		} else {
			fmt.Printf("cross_read=allowed:%s\n", data)
		}
		readLink := filepath.Join(os.Getenv("HOME"), "read-link")
		if err := os.Symlink(os.Getenv(darwin13DOutsidePath), readLink); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(readLink); err != nil {
			fmt.Println("symlink_read=denied")
		} else {
			fmt.Printf("symlink_read=allowed:%s\n", data)
		}
		writeLink := filepath.Join(os.Getenv("HOME"), "write-link")
		if err := os.Symlink(os.Getenv(darwin13DWritePath), writeLink); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(writeLink, []byte("escape"), 0o600); err != nil {
			fmt.Println("symlink_write=denied")
		} else {
			fmt.Println("symlink_write=allowed")
		}
		connection, err := net.DialTimeout("tcp", os.Getenv(darwin13DLoopback), 750*time.Millisecond)
		if err != nil {
			fmt.Println("network=denied")
		} else {
			_ = connection.Close()
			fmt.Println("network=allowed")
		}
	case "spawn-detached":
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.Command(executable, "-test.run=^TestDarwinAdversarialDetachedRuntimeDescendantRemainsConfinedAfterCancel$")
		child.Env = append(os.Environ(), darwin13DAction+"=detached-worker")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("TMPDIR"), "detached.pid"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_ = child.Process.Kill()
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	case "detached-worker":
		time.Sleep(300 * time.Millisecond)
		lines := make([]string, 0, 3)
		if data, err := os.ReadFile(os.Getenv(darwin13DOutsidePath)); err != nil {
			lines = append(lines, "outside_read=denied")
		} else {
			lines = append(lines, "outside_read=allowed:"+string(data))
		}
		if err := os.WriteFile(os.Getenv(darwin13DWritePath), []byte("escape"), 0o600); err != nil {
			lines = append(lines, "outside_write=denied")
		} else {
			lines = append(lines, "outside_write=allowed")
		}
		connection, err := net.DialTimeout("tcp", os.Getenv(darwin13DLoopback), 750*time.Millisecond)
		if err != nil {
			lines = append(lines, "network=denied")
		} else {
			_ = connection.Close()
			lines = append(lines, "network=allowed")
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("TMPDIR"), "detached.report"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Second)
	default:
		t.Fatalf("unknown 13D helper action %q", action)
	}
}

func waitForDarwin13DPID(t *testing.T, path string) int {
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
	t.Fatalf("PID file %q did not appear", path)
	return 0
}

func waitForDarwin13DReport(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("detached report %q did not appear", path)
	return ""
}

func waitForDarwin13DPIDGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("detached pid %d remained alive after host test cleanup", pid)
}
