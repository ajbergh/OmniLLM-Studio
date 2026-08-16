//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const linuxCgroupTestRootEnv = "OMNILLM_TEST_CGROUP_ROOT"

const linuxPIDLimitProbe = `
import errno
import os
import sys
import time

first = os.fork()
if first == 0:
    time.sleep(5)
    os._exit(0)

try:
    second = os.fork()
except OSError as exc:
    if exc.errno != errno.EAGAIN:
        print("unexpected_fork_error", exc.errno, flush=True)
        sys.exit(2)
    print("pid_limit_enforced", flush=True)
    sys.exit(0)

if second == 0:
    os._exit(0)
print("pid_limit_bypassed", flush=True)
sys.exit(3)
`

// TestLinuxCgroupPIDLimitNative is intentionally opt-in because ordinary unit
// test environments do not own a delegated cgroup-v2 subtree. The dedicated CI
// assurance job supplies one through OMNILLM_TEST_CGROUP_ROOT.
func TestLinuxCgroupPIDLimitNative(t *testing.T) {
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
	if manager == nil || !manager.pidsEnabled {
		t.Fatal("delegated cgroup root did not enable PID control")
	}

	execution, err := manager.createExecution(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	executionPath := execution.path
	cmd := exec.Command(python, "-c", linuxPIDLimitProbe)
	if err := execution.attach(cmd); err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	output, runErr := cmd.CombinedOutput()
	cleanupErr := execution.cleanup()
	if runErr != nil {
		t.Fatalf("PID-limit probe failed: %v\n%s", runErr, output)
	}
	if !strings.Contains(string(output), "pid_limit_enforced") || strings.Contains(string(output), "pid_limit_bypassed") {
		t.Fatalf("PID-limit probe did not observe enforcement:\n%s", output)
	}
	if cleanupErr != nil {
		t.Fatalf("cleanup execution cgroup: %v", cleanupErr)
	}
	if _, err := os.Stat(executionPath); !os.IsNotExist(err) {
		t.Fatalf("execution cgroup remained after cleanup: %v", err)
	}
}

func TestLinuxCgroupUnlimitedExecutionNative(t *testing.T) {
	root := strings.TrimSpace(os.Getenv(linuxCgroupTestRootEnv))
	if root == "" {
		t.Skip("native delegated cgroup-v2 root not configured")
	}
	manager := &linuxCgroupManager{root: root, pidsEnabled: true, memoryEnabled: true}
	execution, err := manager.createExecution(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := execution.cleanup(); err != nil {
			t.Errorf("cleanup execution cgroup: %v", err)
		}
	}()
	pidLimit, err := os.ReadFile(filepath.Join(execution.path, "pids.max"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(pidLimit)) != "max" {
		t.Fatalf("pids.max = %q, want max", strings.TrimSpace(string(pidLimit)))
	}
	memoryLimit, err := os.ReadFile(filepath.Join(execution.path, "memory.max"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(memoryLimit)) != "max" {
		t.Fatalf("memory.max = %q, want max", strings.TrimSpace(string(memoryLimit)))
	}
}
