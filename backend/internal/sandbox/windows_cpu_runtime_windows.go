//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

const windowsCPUAccountingPollInterval = 10 * time.Millisecond

type windowsCPUMonitorResult struct {
	quotaExceeded bool
	usageMS       uint64
	err           error
}

// monitorWindowsJobCPU enforces the cumulative process-tree CPU contract from
// Job Object accounting. The Job already owns the root process and every
// descendant from process creation, so TotalUserTime + TotalKernelTime is the
// accounting authority. This helper is staged separately from runtime wiring so
// CPULimit remains false until native descendant-pressure assurance passes.
func monitorWindowsJobCPU(
	ctx context.Context,
	job windows.Handle,
	baselineMS uint64,
	budgetMS uint64,
	done chan<- windowsCPUMonitorResult,
) {
	if budgetMS == 0 {
		done <- windowsCPUMonitorResult{err: fmt.Errorf("Windows CPU monitor requires a positive budget")}
		return
	}
	ticker := time.NewTicker(windowsCPUAccountingPollInterval)
	defer ticker.Stop()

	readUsage := func() (uint64, error) {
		currentMS, err := windowsJobCPUUsageMS(job)
		if err != nil {
			return 0, err
		}
		if currentMS < baselineMS {
			return 0, fmt.Errorf("Windows Job CPU accounting moved backwards")
		}
		return currentMS - baselineMS, nil
	}

	for {
		select {
		case <-ctx.Done():
			usageMS, err := readUsage()
			if err != nil {
				done <- windowsCPUMonitorResult{err: fmt.Errorf("read final Windows Job CPU usage: %w", err)}
				return
			}
			// Natural completion can race the sampling interval. Preserve the
			// cumulative contract by classifying a final over-budget observation
			// even when there is no remaining process to terminate.
			done <- windowsCPUMonitorResult{quotaExceeded: usageMS >= budgetMS, usageMS: usageMS}
			return
		case <-ticker.C:
			usageMS, err := readUsage()
			if err != nil {
				_ = windows.TerminateJobObject(job, 1)
				done <- windowsCPUMonitorResult{usageMS: usageMS, err: fmt.Errorf("monitor Windows Job CPU usage: %w", err)}
				return
			}
			if usageMS < budgetMS {
				continue
			}
			if err := windows.TerminateJobObject(job, 1); err != nil {
				done <- windowsCPUMonitorResult{usageMS: usageMS, err: fmt.Errorf("terminate Windows CPU-limited Job: %w", err)}
				return
			}
			done <- windowsCPUMonitorResult{quotaExceeded: true, usageMS: usageMS}
			return
		}
	}
}
