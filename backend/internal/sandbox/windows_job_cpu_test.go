//go:build windows

package sandbox

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsSandboxJobCPUAccountingAvailable(t *testing.T) {
	job, err := createWindowsSandboxJobWithLimits(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	usage, err := windowsJobCPUUsage100NS(job)
	if err != nil {
		t.Fatal(err)
	}
	// A newly created empty Job should have no meaningful CPU consumption. Do
	// not require exact zero because the contract only depends on monotonic
	// aggregate accounting after processes are assigned.
	if usage > 10_000_000 {
		t.Fatalf("unexpected CPU accounting for empty Job: %d x 100ns", usage)
	}
}
