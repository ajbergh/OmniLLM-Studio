//go:build darwin

package sandbox

import (
	"bufio"
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
	darwinExtensionAction       = "OMNILLM_DARWIN_EXTENSION_ACTION"
	darwinExtensionOutsideRead  = "OMNILLM_DARWIN_EXTENSION_OUTSIDE_READ"
	darwinExtensionOutsideWrite = "OMNILLM_DARWIN_EXTENSION_OUTSIDE_WRITE"
	darwinExtensionPeerRead     = "OMNILLM_DARWIN_EXTENSION_PEER_READ"
	darwinExtensionLoopback     = "OMNILLM_DARWIN_EXTENSION_LOOPBACK"
)

func TestDarwinExtensionNativeIsolation(t *testing.T) {
	if action := os.Getenv(darwinExtensionAction); action != "" {
		darwinExtensionHelper(t, action)
		return
	}

	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV", "false")
	t.Setenv("OMNILLM_MASTER_KEY", "ambient-secret-must-not-leak")

	helper := copyDarwinExtensionTestExecutable(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	const outsideSecret = "OMNILLM-DARWIN-EXTENSION-HOST-CONTENT"
	outsideDir := t.TempDir()
	outsideRead := filepath.Join(outsideDir, "outside.txt")
	outsideWrite := filepath.Join(outsideDir, "outside-write.txt")
	if err := os.WriteFile(outsideRead, []byte(outsideSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	peerProcess, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Dir:     filepath.Dir(helper),
		Args:    []string{"-test.run=^TestDarwinExtensionNativeIsolation$"},
		Env:     map[string]string{darwinExtensionAction: "hold"},
	})
	if err != nil {
		t.Fatal(err)
	}
	peer, ok := peerProcess.(*darwinExtensionProcess)
	if !ok {
		t.Fatalf("peer process type = %T, want *darwinExtensionProcess", peerProcess)
	}
	peer.mu.Lock()
	peerRoot := peer.root
	peer.mu.Unlock()
	peerRead := filepath.Join(peerRoot, "home", "peer-secret.txt")
	if err := os.WriteFile(peerRead, []byte("peer-secret-content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := peer.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = peer.Kill()
		_ = peer.Wait()
	}()

	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Dir:     filepath.Dir(helper),
		Args:    []string{"-test.run=^TestDarwinExtensionNativeIsolation$"},
		Env: map[string]string{
			darwinExtensionAction:       "isolation",
			darwinExtensionOutsideRead:  outsideRead,
			darwinExtensionOutsideWrite: outsideWrite,
			darwinExtensionPeerRead:     peerRead,
			darwinExtensionLoopback:     listener.Addr().String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	confined, ok := process.(*darwinExtensionProcess)
	if !ok {
		t.Fatalf("process type = %T, want *darwinExtensionProcess", process)
	}
	stdout, err := confined.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := confined.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := confined.Start(); err != nil {
		t.Fatal(err)
	}
	stdoutBytes, stdoutErr := io.ReadAll(stdout)
	stderrBytes, stderrErr := io.ReadAll(stderr)
	if stdoutErr != nil {
		t.Fatal(stdoutErr)
	}
	if stderrErr != nil {
		t.Fatal(stderrErr)
	}
	if err := confined.Wait(); err != nil {
		t.Fatalf("confined extension wait: %v stdout=%q stderr=%q", err, stdoutBytes, stderrBytes)
	}

	output := string(stdoutBytes)
	for _, expected := range []string{
		"extension_write=denied",
		"home_write=ok",
		"outside_read=denied",
		"outside_write=denied",
		"peer_read=denied",
		"ambient_secret=absent",
		"network=denied",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("extension helper output missing %q: stdout=%q stderr=%q", expected, output, stderrBytes)
		}
	}
	if strings.Contains(output, outsideSecret) || strings.Contains(output, "peer-secret-content") {
		t.Fatalf("confined extension leaked host/peer content: stdout=%q", output)
	}
	if _, err := os.Stat(outsideWrite); !os.IsNotExist(err) {
		t.Fatalf("confined extension wrote outside granted roots: %v", err)
	}
}

func TestDarwinExtensionAutoUsesNativeSeatbelt(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "auto")
	helper := copyDarwinExtensionTestExecutable(t)
	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{Command: helper, Dir: filepath.Dir(helper)})
	if err != nil {
		t.Fatal(err)
	}
	confined, ok := process.(*darwinExtensionProcess)
	if !ok {
		t.Fatalf("auto mode process type = %T, want *darwinExtensionProcess", process)
	}
	confined.cleanup()
}

func TestDarwinExtensionStdioLifecycle(t *testing.T) {
	if action := os.Getenv(darwinExtensionAction); action != "" {
		darwinExtensionHelper(t, action)
		return
	}
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	helper := copyDarwinExtensionTestExecutable(t)
	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Dir:     filepath.Dir(helper),
		Args:    []string{"-test.run=^TestDarwinExtensionStdioLifecycle$"},
		Env:     map[string]string{darwinExtensionAction: "stdio"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := process.StderrPipe(); err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "ping\n"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "echo=ping\n" {
		t.Fatalf("stdio response = %q", line)
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestDarwinExtensionRejectsSensitiveAndLoaderEnvironment(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV", "false")
	helper := copyDarwinExtensionTestExecutable(t)

	for _, environment := range []map[string]string{
		{"GITHUB_TOKEN": "secret"},
		{"DYLD_LIBRARY_PATH": "/tmp/escape"},
		{"PATH": "/tmp/escape"},
		{"HOME": "/tmp/escape"},
	} {
		_, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
			Command: helper,
			Dir:     filepath.Dir(helper),
			Env:     environment,
		})
		if err == nil {
			t.Fatalf("unsafe extension environment unexpectedly accepted: %#v", environment)
		}
	}
}

func TestDarwinExtensionContextCancellationKillsNormalDescendant(t *testing.T) {
	if action := os.Getenv(darwinExtensionAction); action != "" {
		darwinExtensionHelper(t, action)
		return
	}
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	helper := copyDarwinExtensionTestExecutable(t)
	ctx, cancel := context.WithCancel(context.Background())
	process, err := NewHostCommandRunner().CommandContext(ctx, ProcessSpec{
		Command: helper,
		Dir:     filepath.Dir(helper),
		Args:    []string{"-test.run=^TestDarwinExtensionContextCancellationKillsNormalDescendant$"},
		Env:     map[string]string{darwinExtensionAction: "spawn"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := process.StderrPipe(); err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "child_pid=") {
		t.Fatalf("unexpected child PID output %q", scanner.Text())
	}
	pid, err := strconv.Atoi(strings.TrimPrefix(scanner.Text(), "child_pid="))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid child pid: %v", err)
	}
	cancel()
	_ = process.Wait()
	waitForDarwinExtensionPIDGone(t, pid)
}

func darwinExtensionHelper(t *testing.T, action string) {
	t.Helper()
	switch action {
	case "hold":
		time.Sleep(30 * time.Second)
	case "isolation":
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(executable), "extension-write.txt"), []byte("write"), 0o600); err != nil {
			fmt.Println("extension_write=denied")
		} else {
			fmt.Println("extension_write=allowed")
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), "home-write.txt"), []byte("write"), 0o600); err != nil {
			fmt.Printf("home_write=failed:%v\n", err)
		} else {
			fmt.Println("home_write=ok")
		}
		darwinExtensionReportFileAccess("outside_read", os.Getenv(darwinExtensionOutsideRead), false)
		darwinExtensionReportFileAccess("outside_write", os.Getenv(darwinExtensionOutsideWrite), true)
		darwinExtensionReportFileAccess("peer_read", os.Getenv(darwinExtensionPeerRead), false)
		if os.Getenv("OMNILLM_MASTER_KEY") == "" {
			fmt.Println("ambient_secret=absent")
		} else {
			fmt.Println("ambient_secret=present")
		}
		connection, err := net.DialTimeout("tcp", os.Getenv(darwinExtensionLoopback), 750*time.Millisecond)
		if err != nil {
			fmt.Println("network=denied")
		} else {
			_ = connection.Close()
			fmt.Println("network=allowed")
		}
	case "stdio":
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("echo=%s", line)
	case "spawn":
		child := exec.Command("/bin/sleep", "30")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("child_pid=%d\n", child.Process.Pid)
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown Darwin extension helper action %q", action)
	}
}

func darwinExtensionReportFileAccess(label, path string, write bool) {
	var err error
	if write {
		err = os.WriteFile(path, []byte("write"), 0o600)
	} else {
		_, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Printf("%s=denied\n", label)
	} else {
		fmt.Printf("%s=allowed\n", label)
	}
}

func copyDarwinExtensionTestExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	copyPath := filepath.Join(directory, "extension-helper")
	input, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(copyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
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
	return copyPath
}

func waitForDarwinExtensionPIDGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("descendant pid %d remained alive after extension cancellation", pid)
}
