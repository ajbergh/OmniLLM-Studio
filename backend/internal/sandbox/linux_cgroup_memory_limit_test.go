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
import mmap
import os
import sys

def cgroup_path():
    with open("/proc/self/cgroup", "r", encoding="utf-8") as handle:
        return handle.read().strip()

print("memory_parent_cgroup=" + cgroup_path(), flush=True)
child = os.fork()
if child == 0:
    print("memory_child_cgroup=" + cgroup_path(), flush=True)
    size = 256 * 1024 * 1024
    try:
        region = mmap.mmap(-1, size)
        for offset in range(0, size, 4096):
            region[offset:offset + 1] = b"x"
        print("memory_limit_bypassed", flush=True)
        os._exit(9)
    except (MemoryError, OSError) as exc:
        print("memory_allocation_denied=" + repr(exc), flush=True)
        os._exit(0)

_, status = os.waitpid(child, 0)
if os.WIFEXITED(status) and os.WEXITSTATUS(status) == 9:
    print("memory_limit_bypassed_child", flush=True)
    sys.exit(9)
print("memory_child_constrained", flush=True)
sys.exit(0)
`

// TestLinuxCgroupMemoryLimitNative proves the aggregate cgroup-v2 memory
// boundary against a descendant process. A positive memory limit disables
// cgroup swap so anonymous pages cannot escape the memory.max byte ceiling.
// The kernel may reject an allocation or OOM-kill the allocating descendant;
// either path must produce memory.events evidence and must never let the
// over-limit allocation complete successfully.
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
	swapLimit, err := os.ReadFile(filepath.Join(execution.path, "memory.swap.max"))
	if err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(swapLimit)); got != "0" {
		_ = execution.cleanup()
		t.Fatalf("memory.swap.max = %q, want 0", got)
	}

	cmd := exec.Command(python, "-c", linuxMemoryLimitProbe)
	if err := execution.attach(cmd); err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	output, runErr := cmd.CombinedOutput()
	events, eventsErr := execution.memoryEvents()
	cleanupErr := execution.cleanup()
	outputText := string(output)
	if strings.Contains(outputText, "memory_limit_bypassed") {
		t.Fatalf("memory-limit probe bypassed configured ceiling: runErr=%v events=%#v\n%s", runErr, events, output)
	}
	if !strings.Contains(outputText, "memory_parent_cgroup=") || !strings.Contains(outputText, "memory_child_cgroup=") {
		t.Fatalf("memory-limit cgroup placement evidence missing: runErr=%v\n%s", runErr, output)
	}
	if runErr == nil && !strings.Contains(outputText, "memory_child_constrained") {
		t.Fatalf("memory-limit descendant evidence missing:\n%s", output)
	}
	if eventsErr != nil {
		t.Fatalf("read memory.events: %v", eventsErr)
	}
	if events["oom"] == 0 && events["oom_kill"] == 0 {
		t.Fatalf("memory.events did not record enforcement: runErr=%v events=%#v\n%s", runErr, events, output)
	}
	if cleanupErr != nil {
		t.Fatalf("cleanup execution cgroup: %v", cleanupErr)
	}
	if _, err := os.Stat(executionPath); !os.IsNotExist(err) {
		t.Fatalf("execution cgroup remained after cleanup: %v", err)
	}
}
