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

func TestWindowsSandboxJobProcessLimitConfiguration(t *testing.T) {
	job, err := createWindowsSandboxJob(3)
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
	if flags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
		t.Fatalf("job limit flags %#x do not include KILL_ON_JOB_CLOSE", flags)
	}
	if flags&windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS == 0 {
		t.Fatalf("job limit flags %#x do not include ACTIVE_PROCESS", flags)
	}
	if got := info.BasicLimitInformation.ActiveProcessLimit; got != 3 {
		t.Fatalf("active process limit = %d, want 3", got)
	}
}

func TestWindowsSandboxJobRejectsNegativeProcessLimit(t *testing.T) {
	if job, err := createWindowsSandboxJob(-1); err == nil {
		windows.CloseHandle(job)
		t.Fatal("negative Windows sandbox process limit was accepted")
	}
}

func TestWindowsLocalRuntimeEnforcesProcessCountLimit(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_PID_LIMIT_CHILD") == "1" {
		fmt.Println("pid_limit_child_ran=1")
		os.Exit(0)
	}
	if os.Getenv("OMNILLM_WINDOWS_PID_LIMIT_ROOT") == "1" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("pid_limit_executable_error=%v\n", err)
			os.Exit(61)
		}
		child := exec.Command(executable, "-test.run=^TestWindowsLocalRuntimeEnforcesProcessCountLimit$")
		child.Env = append(os.Environ(), "OMNILLM_WINDOWS_PID_LIMIT_CHILD=1")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		err = child.Run()
		if err == nil {
			fmt.Println("pid_limit_child_unexpected_success=1")
			os.Exit(62)
		}
		fmt.Printf("pid_limit_child_denied=%v\n", err)
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
	if !runtimeValue.Capabilities().PIDLimit {
		t.Fatal("Windows runtime must advertise PIDLimit after native enforcement is wired")
	}
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_pid_limit",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:       []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network:      NetworkPolicy{Mode: NetworkNone},
			Resources:    ResourceLimits{MaxProcesses: 1, WallTimeMS: 15_000, MaxStdoutBytes: 64 << 10, MaxStderrBytes: 64 << 10},
			Requirements: RuntimeRequirements{PIDLimit: true},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeValue.Destroy(context.Background(), runtimeID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := runtimeValue.Exec(ctx, runtimeID, ExecRequest{
		Command: "helper.exe",
		Args:    []string{"-test.run=^TestWindowsLocalRuntimeEnforcesProcessCountLimit$"},
		Env:     map[string]string{"OMNILLM_WINDOWS_PID_LIMIT_ROOT": "1"},
	})
	if err != nil {
		t.Fatalf("execute PID-limit root: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("PID-limit root exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "pid_limit_child_denied=") {
		t.Fatalf("PID-limit child denial not observed: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if strings.Contains(result.Stdout, "pid_limit_child_ran=1") || strings.Contains(result.Stdout, "pid_limit_child_unexpected_success=1") {
		t.Fatalf("child process escaped MaxProcesses=1: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
}
