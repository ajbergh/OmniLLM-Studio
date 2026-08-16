//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsSandboxMemoryTestLimit = int64(256 << 20)
const windowsSandboxMemoryTestRequest = uintptr(512 << 20)

func TestWindowsSandboxJobMemoryLimitConfiguration(t *testing.T) {
	job, err := createWindowsSandboxJobWithLimits(3, windowsSandboxMemoryTestLimit)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	var returned uint32
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&returned,
	); err != nil {
		t.Fatalf("query Windows sandbox job limits: %v", err)
	}
	flags := info.BasicLimitInformation.LimitFlags
	for flag, name := range map[uint32]string{
		windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE: "KILL_ON_JOB_CLOSE",
		windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS:    "ACTIVE_PROCESS",
		windows.JOB_OBJECT_LIMIT_JOB_MEMORY:        "JOB_MEMORY",
	} {
		if flags&flag == 0 {
			t.Fatalf("job limit flags %#x do not include %s", flags, name)
		}
	}
	if got := info.BasicLimitInformation.ActiveProcessLimit; got != 3 {
		t.Fatalf("active process limit = %d, want 3", got)
	}
	if got := int64(info.JobMemoryLimit); got != windowsSandboxMemoryTestLimit {
		t.Fatalf("job memory limit = %d, want %d", got, windowsSandboxMemoryTestLimit)
	}
}

func TestWindowsSandboxJobRejectsNegativeMemoryLimit(t *testing.T) {
	if job, err := createWindowsSandboxJobWithLimits(0, -1); err == nil {
		windows.CloseHandle(job)
		t.Fatal("negative Windows sandbox memory limit was accepted")
	}
}

func TestWindowsSandboxJobLeavesMemoryUnboundedAtZero(t *testing.T) {
	job, err := createWindowsSandboxJobWithLimits(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	var returned uint32
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&returned,
	); err != nil {
		t.Fatalf("query Windows sandbox job limits: %v", err)
	}
	if flags := info.BasicLimitInformation.LimitFlags; flags&windows.JOB_OBJECT_LIMIT_JOB_MEMORY != 0 {
		t.Fatalf("zero memory request unexpectedly enabled JOB_MEMORY: flags=%#x", flags)
	}
	if info.JobMemoryLimit != 0 {
		t.Fatalf("zero memory request configured job memory limit %d", info.JobMemoryLimit)
	}
}

func TestWindowsLocalRuntimeEnforcesAggregateMemoryLimit(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_MEMORY_LIMIT_CHILD") == "1" {
		fmt.Println("memory_limit_child_started=1")
		address, err := windows.VirtualAlloc(
			0,
			windowsSandboxMemoryTestRequest,
			windows.MEM_RESERVE|windows.MEM_COMMIT,
			windows.PAGE_READWRITE,
		)
		if err != nil {
			fmt.Printf("memory_limit_allocation_denied=%v\n", err)
			os.Exit(0)
		}
		_ = windows.VirtualFree(address, 0, windows.MEM_RELEASE)
		fmt.Printf("memory_limit_bypassed=%d\n", windowsSandboxMemoryTestRequest)
		os.Exit(73)
	}
	if os.Getenv("OMNILLM_WINDOWS_MEMORY_LIMIT_ROOT") == "1" {
		fmt.Println("memory_limit_root_started=1")
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("memory_limit_executable_error=%v\n", err)
			os.Exit(71)
		}
		child := exec.Command(executable, "-test.run=^TestWindowsLocalRuntimeEnforcesAggregateMemoryLimit$")
		child.Env = append(os.Environ(), "OMNILLM_WINDOWS_MEMORY_LIMIT_CHILD=1")
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Run(); err != nil {
			fmt.Printf("memory_limit_child_error=%v\n", err)
			os.Exit(72)
		}
		os.Exit(0)
	}

	source := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyWindowsTestFile(executable, filepath.Join(source, "helper.exe")); err != nil {
		t.Fatal(err)
	}

	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeValue.Capabilities().MemoryLimit {
		t.Fatal("Windows runtime must advertise MemoryLimit after native enforcement is wired")
	}
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_memory_limit",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:  []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				MemoryBytes:    windowsSandboxMemoryTestLimit,
				MaxProcesses:   2,
				WallTimeMS:     20_000,
				MaxStdoutBytes: 64 << 10,
				MaxStderrBytes: 64 << 10,
			},
			Requirements: RuntimeRequirements{MemoryLimit: true, PIDLimit: true},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeValue.Destroy(context.Background(), runtimeID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := runtimeValue.Exec(ctx, runtimeID, ExecRequest{
		Command: "helper.exe",
		Args:    []string{"-test.run=^TestWindowsLocalRuntimeEnforcesAggregateMemoryLimit$"},
		Env:     map[string]string{"OMNILLM_WINDOWS_MEMORY_LIMIT_ROOT": "1"},
	})
	if err != nil {
		t.Fatalf("execute memory-limit root: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("memory-limit root exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "memory_limit_root_started=1") {
		t.Fatalf("memory-limit root did not start: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "memory_limit_child_started=1") {
		t.Fatalf("memory-limited descendant did not start before allocation attempt: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "memory_limit_allocation_denied=") {
		t.Fatalf("aggregate memory allocation denial not observed: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if strings.Contains(result.Stdout, "memory_limit_bypassed=") || strings.Contains(result.Stdout, "memory_limit_child_error=") {
		t.Fatalf("memory-limit evidence was not a clean allocation denial: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
}
