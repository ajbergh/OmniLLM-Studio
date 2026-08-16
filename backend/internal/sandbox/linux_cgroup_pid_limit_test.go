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
const linuxCgroupPIDHelperEnv = "OMNILLM_TEST_CGROUP_PID_HELPER"

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
	manager, err := newLinuxPIDCgroupManager(root, bwrap)
	if err != nil {
		t.Fatalf("initialize delegated PID cgroup manager: %v", err)
	}
	if manager == nil {
		t.Fatal("delegated cgroup root did not enable PID manager")
	}

	execution, err := manager.createExecution(2)
	if err != nil {
		t.Fatal(err)
	}
	executionPath := execution.path
	cmd := exec.Command(os.Args[0], "-test.run=^TestLinuxCgroupPIDLimitHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), linuxCgroupPIDHelperEnv+"=1")
	if err := execution.attach(cmd); err != nil {
		_ = execution.cleanup()
		t.Fatal(err)
	}
	output, runErr := cmd.CombinedOutput()
	cleanupErr := execution.cleanup()
	if runErr != nil {
		t.Fatalf("PID-limit helper failed: %v\n%s", runErr, output)
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
	manager := &linuxPIDCgroupManager{root: root}
	execution, err := manager.createExecution(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := execution.cleanup(); err != nil {
			t.Errorf("cleanup execution cgroup: %v", err)
		}
	}()
	limit, err := os.ReadFile(filepath.Join(execution.path, "pids.max"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(limit)) != "max" {
		t.Fatalf("pids.max = %q, want max", strings.TrimSpace(string(limit)))
	}
}

func TestLinuxCgroupPIDLimitHelper(t *testing.T) {
	if os.Getenv(linuxCgroupPIDHelperEnv) != "1" {
		return
	}
	first := exec.Command("sleep", "5")
	if err := first.Start(); err != nil {
		t.Fatalf("start first child within PID quota: %v", err)
	}
	defer func() {
		_ = first.Process.Kill()
		_ = first.Wait()
	}()

	second := exec.Command("sleep", "5")
	if err := second.Start(); err == nil {
		_ = second.Process.Kill()
		_ = second.Wait()
		t.Fatal("second child started beyond pids.max=2")
	}
}
