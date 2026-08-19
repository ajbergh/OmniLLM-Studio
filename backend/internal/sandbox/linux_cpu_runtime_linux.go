//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const linuxCPUAccountingPollInterval = 10 * time.Millisecond

type linuxCPUExecutionOutcome struct {
	runErr       error
	quotaExceeded bool
	usageUS      uint64
}

type linuxCPUMonitorResult struct {
	quotaExceeded bool
	usageUS       uint64
	err           error
}

// runLinuxCommandWithCPUBudget starts cmd after the caller has atomically bound
// it to executionCgroup. A positive CPU budget is enforced from aggregate
// cgroup-v2 cpu.stat usage_usec and exhaustion kills the complete execution
// cgroup. cpu.max, when configured by createExecutionWithCPU, is only an
// overshoot bound and is not the application CPU-time contract.
func runLinuxCommandWithCPUBudget(
	ctx context.Context,
	cmd *exec.Cmd,
	executionCgroup *linuxExecutionCgroup,
	cpuTimeMS int,
) (linuxCPUExecutionOutcome, error) {
	if cmd == nil {
		return linuxCPUExecutionOutcome{}, fmt.Errorf("sandbox command is required")
	}
	if cpuTimeMS < 0 {
		return linuxCPUExecutionOutcome{}, fmt.Errorf("Linux sandbox CPU time limit cannot be negative")
	}
	if cpuTimeMS == 0 {
		return linuxCPUExecutionOutcome{runErr: cmd.Run()}, nil
	}
	if executionCgroup == nil {
		return linuxCPUExecutionOutcome{}, fmt.Errorf("Linux execution cgroup is required for CPU accounting")
	}

	baselineUS, err := executionCgroup.cpuUsageUS()
	if err != nil {
		return linuxCPUExecutionOutcome{}, fmt.Errorf("read Linux CPU baseline: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return linuxCPUExecutionOutcome{}, err
	}

	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	monitorDone := make(chan linuxCPUMonitorResult, 1)
	go monitorLinuxCPUUsage(monitorCtx, executionCgroup, baselineUS, uint64(cpuTimeMS)*1000, monitorDone)

	runErr := cmd.Wait()
	cancelMonitor()
	monitor := <-monitorDone
	if monitor.err != nil {
		return linuxCPUExecutionOutcome{runErr: runErr, usageUS: monitor.usageUS}, monitor.err
	}
	return linuxCPUExecutionOutcome{
		runErr:        runErr,
		quotaExceeded: monitor.quotaExceeded,
		usageUS:       monitor.usageUS,
	}, nil
}

func monitorLinuxCPUUsage(
	ctx context.Context,
	executionCgroup *linuxExecutionCgroup,
	baselineUS uint64,
	budgetUS uint64,
	done chan<- linuxCPUMonitorResult,
) {
	ticker := time.NewTicker(linuxCPUAccountingPollInterval)
	defer ticker.Stop()

	readUsage := func() (uint64, error) {
		currentUS, err := executionCgroup.cpuUsageUS()
		if err != nil {
			return 0, err
		}
		if currentUS < baselineUS {
			return 0, fmt.Errorf("Linux execution CPU accounting moved backwards")
		}
		return currentUS - baselineUS, nil
	}

	for {
		select {
		case <-ctx.Done():
			usageUS, err := readUsage()
			if err != nil {
				done <- linuxCPUMonitorResult{err: fmt.Errorf("read final Linux CPU usage: %w", err)}
				return
			}
			done <- linuxCPUMonitorResult{usageUS: usageUS}
			return
		case <-ticker.C:
			usageUS, err := readUsage()
			if err != nil {
				_ = executionCgroup.kill()
				done <- linuxCPUMonitorResult{usageUS: usageUS, err: fmt.Errorf("monitor Linux CPU usage: %w", err)}
				return
			}
			if usageUS < budgetUS {
				continue
			}
			if err := executionCgroup.kill(); err != nil {
				done <- linuxCPUMonitorResult{usageUS: usageUS, err: fmt.Errorf("terminate Linux CPU-limited execution: %w", err)}
				return
			}
			done <- linuxCPUMonitorResult{quotaExceeded: true, usageUS: usageUS}
			return
		}
	}
}
