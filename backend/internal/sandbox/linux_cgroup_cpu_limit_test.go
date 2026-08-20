//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	linuxCPULimitProbeMS   = 150
	linuxCPULimitTimeout   = 8 * time.Second
	linuxCPUBurnerDuration = 6 * time.Second
)

// TestLinuxCgroupCPUAccountingNative proves that cgroup-v2 CPU accounting is
// aggregate across descendants and that cgroup.kill terminates the complete
// process tree once the cumulative user+system budget is observed. Runtime
// CPULimit remains false until this primitive is wired into LocalRuntime.Exec.
func TestLinuxCgroupCPUAccountingNative(t *testing.T) {
	if os.Getenv("OMNILLM_LINUX_CPU_SPIN") == "1" {
		deadline := time.Now().Add(linuxCPUBurnerDuration)
		var x uint64 = 1
		for time.Now().Before(deadline) {
			x = x*1664525 + 1013904223
		}
		if x == 0 {
			fmt.Fprintln(os.Stdout, x)
		}
		return
	}
	if os.Getenv("OMNILLM_LINUX_CPU_BURNER") == "1" {
		runtime.GOMAXPROCS(2)
		executable, err := os.Executable()
		if err != nil {
			os.Exit(71)
		}
		children := make([]*exec.Cmd, 0, 2)
		for i := 0; i < 2; i++ {
			child := exec.Command(executable, "-test.run=^TestLinuxCgroupCPUAccountingNative$")
			child.Env = append(os.Environ(), "OMNILLM_LINUX_CPU_SPIN=1")
			if err := child.Start(); err != nil {
				os.Exit(72)
			}
			children = append(children, child)
		}
		for _, child := range children {
			_ = child.Wait()
		}
		return
	}

	root := strings.TrimSpace(os.Getenv(linuxCgroupTestRootEnv))
	if root == "" {
		t.Skip("native delegated cgroup-v2 root not configured")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := newLinuxCgroupManager(root, bwrap)
	if err != nil {
		t.Fatalf("initialize delegated cgroup manager: %v", err)
	}
	if manager == nil || !manager.cpuEnabled {
		t.Fatal("delegated cgroup root did not enable CPU control")
	}

	execution, err := manager.createExecutionWithCPU(0, 0, linuxCPULimitProbeMS)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := execution.cleanup(); err != nil {
			t.Errorf("cleanup execution cgroup: %v", err)
		}
	}()
	baseline, err := execution.cpuUsageUS()
	if err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestLinuxCgroupCPUAccountingNative$")
	cmd.Env = append(os.Environ(), "OMNILLM_LINUX_CPU_BURNER=1")
	if err := execution.attach(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(linuxCPULimitTimeout)
	thresholdUS := uint64(linuxCPULimitProbeMS * 1000)
	var observed uint64
	for time.Now().Before(deadline) {
		usage, readErr := execution.cpuUsageUS()
		if readErr != nil {
			_ = execution.kill()
			_ = cmd.Wait()
			t.Fatal(readErr)
		}
		if usage >= baseline {
			observed = usage - baseline
		}
		if observed >= thresholdUS {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if observed < thresholdUS {
		_ = execution.kill()
		_ = cmd.Wait()
		t.Fatalf("aggregate CPU usage reached %d usec, want at least %d", observed, thresholdUS)
	}
	if err := execution.kill(); err != nil {
		_ = cmd.Wait()
		t.Fatalf("kill CPU-limited execution cgroup: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("CPU burner exited normally after cgroup.kill; expected forced process-tree termination")
	}
	finalUsage, err := execution.cpuUsageUS()
	if err != nil {
		t.Fatal(err)
	}
	if finalUsage < baseline+thresholdUS {
		t.Fatalf("final aggregate CPU usage = %d usec, want at least baseline %d + %d", finalUsage, baseline, thresholdUS)
	}
}
