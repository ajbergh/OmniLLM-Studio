//go:build windows

package sandbox

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsJobBasicAccountingInformation mirrors
// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION. Windows Job accounting aggregates the
// root process and every process that has belonged to the Job, which makes it
// the accounting identity for the sandbox process tree.
type windowsJobBasicAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func windowsJobCPUUsage100NS(job windows.Handle) (uint64, error) {
	if job == 0 || job == windows.InvalidHandle {
		return 0, fmt.Errorf("Windows sandbox Job Object is unavailable")
	}
	var info windowsJobBasicAccountingInformation
	var returned uint32
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&returned,
	); err != nil {
		return 0, fmt.Errorf("query Windows sandbox Job CPU accounting: %w", err)
	}
	if info.TotalUserTime < 0 || info.TotalKernelTime < 0 {
		return 0, fmt.Errorf("Windows sandbox Job returned negative CPU accounting")
	}
	return uint64(info.TotalUserTime) + uint64(info.TotalKernelTime), nil
}

func windowsJobCPUUsageMS(job windows.Handle) (uint64, error) {
	usage100NS, err := windowsJobCPUUsage100NS(job)
	if err != nil {
		return 0, err
	}
	// LARGE_INTEGER Job accounting is expressed in 100-nanosecond units.
	return usage100NS / 10_000, nil
}
