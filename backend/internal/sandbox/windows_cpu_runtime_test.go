//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsNativeCPUBudgetMS = 150

func TestWindowsJobCPUMonitorDescendantFanout(t *testing.T) {
	mode := windowsCPUTestMode(os.Args)
	switch mode {
	case "root":
		executable, err := os.Executable()
		if err != nil {
			os.Exit(71)
		}
		children := make([]*exec.Cmd, 0, 2)
		for i := 0; i < 2; i++ {
			child := exec.Command(executable, "-test.run=^TestWindowsJobCPUMonitorDescendantFanout$", "--", "cpu-child")
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Start(); err != nil {
				fmt.Printf("cpu_child_start_error=%v\n", err)
				os.Exit(72)
			}
			children = append(children, child)
		}
		windowsCPUBurn()
	case "child":
		windowsCPUBurn()
	}

	job, err := createWindowsSandboxJobWithLimits(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commandLine, err := windows.UTF16FromString(
		syscallEscapeWindowsArg(executable) + " -test.run=^TestWindowsJobCPUMonitorDescendantFanout$ -- cpu-root",
	)
	if err != nil {
		t.Fatal(err)
	}
	application, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		t.Fatal(err)
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	processInfo := windows.ProcessInformation{}
	if err := windows.CreateProcess(
		application,
		&commandLine[0],
		nil,
		nil,
		false,
		windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW,
		nil,
		nil,
		&startup,
		&processInfo,
	); err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(processInfo.Process)
	defer windows.CloseHandle(processInfo.Thread)
	if err := windows.AssignProcessToJobObject(job, processInfo.Process); err != nil {
		_ = windows.TerminateProcess(processInfo.Process, 1)
		t.Fatalf("assign CPU test process to Job: %v", err)
	}
	baseline, err := windowsJobCPUUsageMS(job)
	if err != nil {
		_ = windows.TerminateJobObject(job, 1)
		t.Fatal(err)
	}
	if _, err := windows.ResumeThread(processInfo.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		t.Fatalf("resume CPU test process: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	monitorDone := make(chan windowsCPUMonitorResult, 1)
	go monitorWindowsJobCPU(ctx, job, baseline, windowsNativeCPUBudgetMS, monitorDone)
	monitor := <-monitorDone
	if monitor.err != nil {
		t.Fatal(monitor.err)
	}
	if !monitor.quotaExceeded {
		t.Fatalf("quotaExceeded=false usage=%dms", monitor.usageMS)
	}
	if monitor.usageMS < windowsNativeCPUBudgetMS {
		t.Fatalf("aggregate Job CPU usage=%dms, want at least %dms", monitor.usageMS, windowsNativeCPUBudgetMS)
	}

	wait, err := windows.WaitForSingleObject(processInfo.Process, 5_000)
	if err != nil {
		t.Fatalf("wait for CPU-limited root after Job termination: %v", err)
	}
	if wait != windows.WAIT_OBJECT_0 {
		t.Fatalf("CPU-limited root remained alive after Job termination: wait=%#x", wait)
	}
	info := windows.JOBOBJECT_BASIC_ACCOUNTING_INFORMATION{}
	var returned uint32
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&returned,
	); err != nil {
		t.Fatal(err)
	}
	if info.TotalProcesses < 2 {
		t.Fatalf("Job recorded only %d process(es); descendant fan-out was not exercised", info.TotalProcesses)
	}
}

func windowsCPUTestMode(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "--" {
			continue
		}
		switch strings.TrimSpace(args[i+1]) {
		case "cpu-root":
			return "root"
		case "cpu-child":
			return "child"
		}
	}
	return ""
}

func windowsCPUBurn() {
	var value uint64 = 1
	for {
		value = value*1664525 + 1013904223
		if value == 0 {
			fmt.Fprint(os.Stderr, "")
		}
	}
}

func syscallEscapeWindowsArg(value string) string {
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
