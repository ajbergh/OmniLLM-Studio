//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const windowsTokenIsAppContainerClass = 29

func TestWindowsAppContainerProfileLifecycle(t *testing.T) {
	name := "OmniLLM.Test." + strings.ReplaceAll(uuid.NewString(), "-", "")
	sid, err := createWindowsAppContainerProfile(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := deleteWindowsAppContainerProfile(name); err != nil {
			t.Errorf("delete AppContainer profile: %v", err)
		}
	}()
	if sid == nil || !sid.IsValid() {
		t.Fatal("AppContainer profile did not return a valid SID")
	}
	folder, err := windowsAppContainerFolderPath(sid)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		t.Fatalf("AppContainer folder is not available: info=%v err=%v", info, err)
	}
}

func TestWindowsLocalRuntimeCapabilities(t *testing.T) {
	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	capabilities := runtimeValue.Capabilities()
	if capabilities.Name != "windows-appcontainer" {
		t.Fatalf("runtime name = %q", capabilities.Name)
	}
	if !capabilities.OSIsolation || !capabilities.FilesystemIsolation || !capabilities.NetworkIsolation || !capabilities.ProcessTreeIsolation || !capabilities.PIDLimit || !capabilities.MemoryLimit {
		t.Fatalf("Windows runtime did not advertise required enforced capabilities: %+v", capabilities)
	}
	if capabilities.NetworkAllowlist || capabilities.CPULimit || capabilities.DiskLimit {
		t.Fatalf("Windows runtime over-advertised unenforced capabilities: %+v", capabilities)
	}
}

func TestWindowsLocalRuntimeRejectsWritableWorkspaceMount(t *testing.T) {
	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_writable",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:  []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadWrite}},
			Network: NetworkPolicy{Mode: NetworkNone},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: t.TempDir(), Mode: MountReadWrite}},
	})
	if err == nil || !strings.Contains(err.Error(), "only") {
		t.Fatalf("writable Windows runtime mount error = %v", err)
	}
}

func TestWindowsLocalRuntimeAppContainerReadOnlyAndNoNetwork(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_SANDBOX_HELPER") == "1" {
		windowsSandboxHelperProcess(t)
		return
	}

	source := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(source, "helper.exe")
	if err := copyWindowsTestFile(executable, helperPath); err != nil {
		t.Fatalf("copy helper executable: %v", err)
	}

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside-secret.txt")
	outsideWrite := filepath.Join(outsideDir, "sandbox-outside-write.txt")
	if err := os.WriteFile(outsideFile, []byte("must-not-be-readable"), 0o600); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	t.Setenv("OMNILLM_TEST_AMBIENT_SECRET", "must-not-leak")
	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_appcontainer",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:  []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				WallTimeMS:     15_000,
				MaxStdoutBytes: 64 << 10,
				MaxStderrBytes: 64 << 10,
			},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeValue.Destroy(context.Background(), runtimeID)

	result, err := runtimeValue.Exec(context.Background(), runtimeID, ExecRequest{
		Command: "helper.exe",
		Args:    []string{"-test.run=^TestWindowsLocalRuntimeAppContainerReadOnlyAndNoNetwork$"},
		Env: map[string]string{
			"OMNILLM_WINDOWS_SANDBOX_HELPER":       "1",
			"OMNILLM_WINDOWS_SANDBOX_LOOPBACK":     listener.Addr().String(),
			"OMNILLM_WINDOWS_SANDBOX_OUTSIDE_FILE": outsideFile,
		},
		TimeoutMS: 15_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("sandbox helper exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	for _, expected := range []string{
		"appcontainer=1",
		"workspace_write=denied",
		"outside_read=denied",
		"outside_write=denied",
		"ambient_secret=absent",
		"network=denied",
	} {
		if !strings.Contains(result.Stdout, expected) {
			t.Fatalf("sandbox helper output missing %q: stdout=%q stderr=%q", expected, result.Stdout, result.Stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "sandbox-write-test.txt")); !os.IsNotExist(err) {
		t.Fatalf("host source workspace was modified: %v", err)
	}
	if _, err := os.Stat(outsideWrite); !os.IsNotExist(err) {
		t.Fatalf("sandbox wrote outside its profile: %v", err)
	}
}

func TestWindowsLocalRuntimeKillsDescendantsAfterRootExit(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_SANDBOX_DESCENDANT_CHILD") == "1" {
		time.Sleep(1500 * time.Millisecond)
		if err := os.WriteFile(os.Getenv("OMNILLM_WINDOWS_SANDBOX_DESCENDANT_MARKER"), []byte("escaped"), 0o600); err != nil {
			fmt.Printf("descendant_marker_error=%v\n", err)
			os.Exit(31)
		}
		os.Exit(0)
	}
	if os.Getenv("OMNILLM_WINDOWS_SANDBOX_DESCENDANT_ROOT") == "1" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("descendant_executable_error=%v\n", err)
			os.Exit(32)
		}
		command := exec.Command(executable, "-test.run=^TestWindowsLocalRuntimeKillsDescendantsAfterRootExit$")
		command.Env = append(os.Environ(), "OMNILLM_WINDOWS_SANDBOX_DESCENDANT_CHILD=1")
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			fmt.Printf("descendant_start_error=%v\n", err)
			os.Exit(33)
		}
		fmt.Println("descendant_spawned=1")
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
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_descendants",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:  []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				WallTimeMS:     15_000,
				MaxStdoutBytes: 64 << 10,
				MaxStderrBytes: 64 << 10,
			},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeValue.Destroy(context.Background(), runtimeID)
	localRuntime, ok := runtimeValue.(*LocalRuntime)
	if !ok {
		t.Fatalf("runtime type = %T", runtimeValue)
	}
	session, err := localRuntime.session(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(session.tmp, "descendant-marker.txt")

	result, err := runtimeValue.Exec(context.Background(), runtimeID, ExecRequest{
		Command: "helper.exe",
		Args:    []string{"-test.run=^TestWindowsLocalRuntimeKillsDescendantsAfterRootExit$"},
		Env: map[string]string{
			"OMNILLM_WINDOWS_SANDBOX_DESCENDANT_ROOT":   "1",
			"OMNILLM_WINDOWS_SANDBOX_DESCENDANT_MARKER": marker,
		},
		TimeoutMS: 15_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || !strings.Contains(result.Stdout, "descendant_spawned=1") {
		t.Fatalf("descendant root exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	time.Sleep(2 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("sandbox descendant outlived root execution: %v", err)
	}
}

func TestWindowsLocalRuntimeDestroyWaitsForExecutionTeardown(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_SANDBOX_DESTROY_HELPER") == "1" {
		marker := os.Getenv("OMNILLM_WINDOWS_SANDBOX_DESTROY_READY")
		if err := os.WriteFile(marker, []byte("ready"), 0o600); err != nil {
			fmt.Printf("destroy_ready_error=%v\n", err)
			os.Exit(41)
		}
		time.Sleep(30 * time.Second)
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
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_destroy",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Mounts:  []WorkspaceMount{{WorkspaceID: "ws_test", Mode: MountReadOnly}},
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				WallTimeMS:     60_000,
				MaxStdoutBytes: 64 << 10,
				MaxStderrBytes: 64 << 10,
			},
		},
		ResolvedMounts: []RuntimeMount{{WorkspaceID: "ws_test", SourcePath: source, Mode: MountReadOnly}},
	})
	if err != nil {
		t.Fatal(err)
	}
	localRuntime, ok := runtimeValue.(*LocalRuntime)
	if !ok {
		t.Fatalf("runtime type = %T", runtimeValue)
	}
	session, err := localRuntime.session(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(session.tmp, "destroy-ready.txt")
	execDone := make(chan error, 1)
	go func() {
		_, execErr := runtimeValue.Exec(context.Background(), runtimeID, ExecRequest{
			Command: "helper.exe",
			Args:    []string{"-test.run=^TestWindowsLocalRuntimeDestroyWaitsForExecutionTeardown$"},
			Env: map[string]string{
				"OMNILLM_WINDOWS_SANDBOX_DESTROY_HELPER": "1",
				"OMNILLM_WINDOWS_SANDBOX_DESTROY_READY":  ready,
			},
			TimeoutMS: 60_000,
		})
		execDone <- execErr
	}()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sandbox helper did not signal execution readiness")
		}
		time.Sleep(50 * time.Millisecond)
	}

	destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer destroyCancel()
	if err := runtimeValue.Destroy(destroyCtx, runtimeID); err != nil {
		t.Fatalf("destroy active Windows sandbox: %v", err)
	}
	select {
	case execErr := <-execDone:
		if execErr == nil || (!strings.Contains(execErr.Error(), "cancelled") && !strings.Contains(execErr.Error(), "canceled")) {
			t.Fatalf("active execution result after destroy = %v", execErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active execution did not finish before Destroy returned")
	}
	if _, err := os.Stat(session.root); !os.IsNotExist(err) {
		t.Fatalf("Windows sandbox profile data still exists after Destroy: %v", err)
	}
	if _, err := localRuntime.session(runtimeID); err == nil {
		t.Fatal("destroyed Windows sandbox session remains addressable")
	}
}

func TestWindowsLocalRuntimeDestroyRetainsSessionAfterCleanupFailure(t *testing.T) {
	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_cleanup_retry",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				WallTimeMS:     15_000,
				MaxStdoutBytes: 64 << 10,
				MaxStderrBytes: 64 << 10,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	localRuntime, ok := runtimeValue.(*LocalRuntime)
	if !ok {
		t.Fatalf("runtime type = %T", runtimeValue)
	}
	session, err := localRuntime.session(runtimeID)
	if err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(session.tmp, "cleanup-lock.txt")
	if err := os.WriteFile(lockPath, []byte("locked"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPathPtr, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lockHandle, err := windows.CreateFile(
		lockPathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if lockHandle != 0 {
			_ = windows.CloseHandle(lockHandle)
		}
	}()

	if err := runtimeValue.Destroy(context.Background(), runtimeID); err == nil {
		t.Fatal("Destroy unexpectedly succeeded while profile file denied delete sharing")
	}
	localRuntime.mu.RLock()
	retained, exists := localRuntime.sessions[runtimeID]
	localRuntime.mu.RUnlock()
	if !exists || !retained.destroying {
		t.Fatalf("failed cleanup did not retain destroying session: exists=%v session=%+v", exists, retained)
	}
	if _, err := localRuntime.session(runtimeID); err == nil {
		t.Fatal("destroying session remained executable after cleanup failure")
	}

	if err := windows.CloseHandle(lockHandle); err != nil {
		t.Fatalf("close cleanup lock: %v", err)
	}
	lockHandle = 0
	if err := runtimeValue.Destroy(context.Background(), runtimeID); err != nil {
		t.Fatalf("retry Windows sandbox cleanup: %v", err)
	}
	localRuntime.mu.RLock()
	_, exists = localRuntime.sessions[runtimeID]
	localRuntime.mu.RUnlock()
	if exists {
		t.Fatal("successfully cleaned Windows sandbox session remains registered")
	}
	if _, err := os.Stat(session.root); !os.IsNotExist(err) {
		t.Fatalf("Windows sandbox profile data still exists after cleanup retry: %v", err)
	}
}

func windowsSandboxHelperProcess(t *testing.T) {
	var isAppContainer uint32
	var returned uint32
	if err := windows.GetTokenInformation(
		windows.GetCurrentProcessToken(),
		windowsTokenIsAppContainerClass,
		(*byte)(unsafe.Pointer(&isAppContainer)),
		uint32(unsafe.Sizeof(isAppContainer)),
		&returned,
	); err != nil {
		fmt.Printf("token_query_error=%v\n", err)
		os.Exit(21)
	}
	fmt.Printf("appcontainer=%d\n", isAppContainer)

	if err := os.WriteFile("sandbox-write-test.txt", []byte("blocked"), 0o600); err != nil {
		fmt.Println("workspace_write=denied")
	} else {
		fmt.Println("workspace_write=allowed")
	}
	outsideFile := os.Getenv("OMNILLM_WINDOWS_SANDBOX_OUTSIDE_FILE")
	if _, err := os.ReadFile(outsideFile); err != nil {
		fmt.Println("outside_read=denied")
	} else {
		fmt.Println("outside_read=allowed")
	}
	outsideWrite := filepath.Join(filepath.Dir(outsideFile), "sandbox-outside-write.txt")
	if err := os.WriteFile(outsideWrite, []byte("blocked"), 0o600); err != nil {
		fmt.Println("outside_write=denied")
	} else {
		fmt.Println("outside_write=allowed")
	}
	if os.Getenv("OMNILLM_TEST_AMBIENT_SECRET") == "" {
		fmt.Println("ambient_secret=absent")
	} else {
		fmt.Println("ambient_secret=present")
	}

	address := os.Getenv("OMNILLM_WINDOWS_SANDBOX_LOOPBACK")
	connection, err := net.DialTimeout("tcp", address, 1500*time.Millisecond)
	if err != nil {
		fmt.Println("network=denied")
	} else {
		_ = connection.Close()
		fmt.Println("network=allowed")
	}
	os.Exit(0)
}

func copyWindowsTestFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
