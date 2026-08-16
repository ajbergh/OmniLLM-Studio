//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const linuxMemoryLimitBytes = int64(64 << 20)

const linuxMemoryLimitProbe = `
import os
import sys

child = os.fork()
if child == 0:
    try:
        chunks = []
        for _ in range(256):
            block = bytearray(1024 * 1024)
            block[0] = 1
            block[-1] = 1
            chunks.append(block)
        print("memory_limit_bypassed", flush=True)
        os._exit(9)
    except MemoryError:
        print("memory_allocation_denied", flush=True)
        os._exit(0)

_, status = os.waitpid(child, 0)
if os.WIFEXITED(status) and os.WEXITSTATUS(status) == 9:
    print("memory_limit_bypassed_child", flush=True)
    sys.exit(9)
print("memory_child_constrained", flush=True)
sys.exit(0)
`

// TestLinuxCgroupMemoryLimitNative proves the aggregate cgroup-v2 memory
// boundary against a descendant process. The kernel may satisfy an over-limit
// charge by returning allocation failure or by OOM-killing the allocating child;
// in either case memory.events must record an OOM event and the allocation must
// never complete successfully.
func TestLinuxCgroupMemoryLimitNative(t *testing.T) {
	root := strings.TrimSpace(os.Getenv(linuxCgroupTestRootEnv))
	if root == "" {
		t.Skip("native delegated cgroup-v2 root not configured")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := newLinuxCgroupManager(root, bwrap)
	if err != nil {
		t.Fatalf("initialize delegated cgroup manager: %v", err)
	}
	if manager == nil || !manager.memoryEnabled {
		t.Fatal("delegated cgroup root did not enable memory control")
	}

	execution, err := manager.createExecution(0, linuxMemoryLimitBytes)
	if err != nil {
		t.Fatal(err)
	}
	executionPath := execution.path
	limit, err := os.ReadFile(filepath.Join(execution.path, "memory.max"))
	if err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(limit)); got != strconv.FormatInt(linuxMemoryLimitBytes, 10) {
		_ = execution.cleanup()
		t.Fatalf("memory.max = %q, want %d", got, linuxMemoryLimitBytes)
	}

	cmd := exec.Command(python, "-c", linuxMemoryLimitProbe)
	if err := execution.attach(cmd); err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	output, runErr := cmd.CombinedOutput()
	events, eventsErr := execution.memoryEvents()
	cleanupErr := execution.cleanup()
	if runErr != nil {
		t.Fatalf("memory-limit probe failed: %v\n%s", runErr, output)
	}
	if strings.Contains(string(output), "memory_limit_bypassed") {
		t.Fatalf("memory-limit probe bypassed configured ceiling:\n%s", output)
	}
	if !strings.Contains(string(output), "memory_child_constrained") {
		t.Fatalf("memory-limit descendant evidence missing:\n%s", output)
	}
	if eventsErr != nil {
		t.Fatalf("read memory.events: %v", eventsErr)
	}
	if events["oom"] == 0 && events["oom_kill"] == 0 {
		t.Fatalf("memory.events did not record enforcement: %#v\n%s", events, output)
	}
	if cleanupErr != nil {
		t.Fatalf("cleanup execution cgroup: %v", cleanupErr)
	}
	if _, err := os.Stat(executionPath); !os.IsNotExist(err) {
		t.Fatalf("execution cgroup remained after cleanup: %v", err)
	}
}
