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

const darwin13DExtensionAction = "OMNILLM_DARWIN_13D_EXTENSION_ACTION"

func TestDarwinAdversarialDetachedExtensionDescendantRemainsConfinedAfterRootKill(t *testing.T) {
	if action := os.Getenv(darwin13DExtensionAction); action != "" {
		darwin13DExtensionHelper(t, action)
		return
	}
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	helper := copyDarwinExtensionTestExecutable(t)
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	writePath := filepath.Join(outsideDir, "write.txt")
	if err := os.WriteFile(outsidePath, []byte("extension-detached-outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Dir:     filepath.Dir(helper),
		Args:    []string{"-test.run=^TestDarwinAdversarialDetachedExtensionDescendantRemainsConfinedAfterRootKill$"},
		Env: map[string]string{
			darwin13DExtensionAction: "spawn-detached",
			darwin13DOutsidePath:     outsidePath,
			darwin13DWritePath:       writePath,
			darwin13DLoopback:        listener.Addr().String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	confined, ok := process.(*darwinExtensionProcess)
	if !ok {
		t.Fatalf("process type = %T, want *darwinExtensionProcess", process)
	}
	confined.mu.Lock()
	root := confined.root
	confined.mu.Unlock()
	if root == "" {
		t.Fatal("native extension scratch root missing before start")
	}
	if _, err := confined.StdoutPipe(); err != nil {
		t.Fatal(err)
	}
	if _, err := confined.StderrPipe(); err != nil {
		t.Fatal(err)
	}
	if err := confined.Start(); err != nil {
		t.Fatal(err)
	}

	pid := waitForDarwin13DPID(t, filepath.Join(root, "tmp", "detached-extension.pid"))
	report := waitForDarwin13DReport(t, filepath.Join(root, "tmp", "detached-extension.report"))
	for _, expected := range []string{"outside_read=denied", "outside_write=denied", "network=denied"} {
		if !strings.Contains(report, expected) {
			t.Fatalf("detached extension report missing %q: %q", expected, report)
		}
	}
	if strings.Contains(report, "extension-detached-outside-secret") {
		t.Fatalf("detached extension leaked host content: %q", report)
	}

	if err := confined.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = confined.Wait()
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("detached extension pid %d did not survive root process-group kill as expected by the documented limitation: %v", pid, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("extension scratch root was not cleaned after root wait: %v", err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("cleanup detached extension pid %d: %v", pid, err)
	}
	waitForDarwin13DPIDGone(t, pid)
	if _, err := os.Stat(writePath); !os.IsNotExist(err) {
		t.Fatalf("detached extension wrote outside confinement root: %v", err)
	}
}

func darwin13DExtensionHelper(t *testing.T, action string) {
	t.Helper()
	switch action {
	case "spawn-detached":
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.Command(executable, "-test.run=^TestDarwinAdversarialDetachedExtensionDescendantRemainsConfinedAfterRootKill$")
		child.Env = darwin13DOverrideEnvironment(os.Environ(), darwin13DExtensionAction, "detached-worker")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("TMPDIR"), "detached-extension.pid"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			_ = child.Process.Kill()
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	case "detached-worker":
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
		if err := os.WriteFile(filepath.Join(os.Getenv("TMPDIR"), "detached-extension.report"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Second)
	default:
		t.Fatalf("unknown 13D extension helper action %q", action)
	}
}

func darwin13DOverrideEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, fmt.Sprintf("%s=%s", key, value))
}
