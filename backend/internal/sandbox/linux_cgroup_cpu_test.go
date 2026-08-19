//go:build linux

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinuxCPUStatUsageUsec(t *testing.T) {
	usage, err := parseLinuxCPUStat([]byte("usage_usec 123456\nuser_usec 100000\nsystem_usec 23456\nnr_periods 4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if usage != 123456 {
		t.Fatalf("usage_usec = %d, want 123456", usage)
	}
}

func TestParseLinuxCPUStatRequiresAggregateUsage(t *testing.T) {
	if _, err := parseLinuxCPUStat([]byte("user_usec 100\nsystem_usec 20\n")); err == nil {
		t.Fatal("cpu.stat without usage_usec unexpectedly succeeded")
	}
}

func TestMonitorLinuxCPUUsageClassifiesFinalOverBudgetUsage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cpu.stat"), []byte("usage_usec 250000\nuser_usec 200000\nsystem_usec 50000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	execution := &linuxExecutionCgroup{path: dir}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan linuxCPUMonitorResult, 1)
	monitorLinuxCPUUsage(ctx, execution, 100000, 100000, done)
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	if !result.quotaExceeded {
		t.Fatalf("final usage %d usec was not classified over budget", result.usageUS)
	}
	if result.usageUS != 150000 {
		t.Fatalf("final usage delta = %d, want 150000", result.usageUS)
	}
}
