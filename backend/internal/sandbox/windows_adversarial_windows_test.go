//go:build windows

package sandbox

import (
	"bufio"
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

	"golang.org/x/sys/windows"
)

func TestWindowsAdversarialStagingRejectsHardLinksAndJunctions(t *testing.T) {
	t.Run("hard_link", func(t *testing.T) {
		source := t.TempDir()
		original := filepath.Join(source, "original.txt")
		linked := filepath.Join(source, "linked.txt")
		if err := os.WriteFile(original, []byte("same-file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(original, linked); err != nil {
			t.Fatalf("create hard link: %v", err)
		}

		err := stageWindowsReadOnlyWorkspace(source, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "multiply-linked") {
			t.Fatalf("hard-linked workspace staging error = %v", err)
		}
	})

	t.Run("junction", func(t *testing.T) {
		source := t.TempDir()
		outside := t.TempDir()
		junction := filepath.Join(source, "outside-junction")
		command := exec.Command("cmd.exe", "/d", "/s", "/c", fmt.Sprintf(`mklink /J "%s" "%s"`, junction, outside))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("create directory junction: %v output=%q", err, output)
		}
		defer os.Remove(junction)

		err := stageWindowsReadOnlyWorkspace(source, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "reparse point") {
			t.Fatalf("junction workspace staging error = %v", err)
		}
	})
}

func TestWindowsAdversarialOpenedHandleTracksRenameOutsideWorkspace(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	original := filepath.Join(source, "payload.txt")
	moved := filepath.Join(outside, "payload.txt")
	if err := os.WriteFile(original, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	pathPtr, err := windows.UTF16PtrFromString(original)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	if err := os.Rename(original, moved); err != nil {
		t.Fatalf("rename opened source outside workspace: %v", err)
	}
	resolved, err := windowsFinalPathForHandle(handle)
	if err != nil {
		t.Fatal(err)
	}
	if windowsPathWithin(source, resolved) {
		t.Fatalf("renamed opened handle still appeared inside original workspace: %q", resolved)
	}
	if !windowsPathWithin(outside, resolved) {
		t.Fatalf("renamed opened handle did not resolve beneath destination: %q", resolved)
	}
}

func TestWindowsAdversarialCrossSandboxAppContainerIsolation(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_ISOLATION_HELPER") == "1" {
		windowsAdversarialIsolationHelperProcess()
		return
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
	create := func(sessionID string) string {
		runtimeID, createErr := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
			SessionID: sessionID,
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
		if createErr != nil {
			t.Fatal(createErr)
		}
		return runtimeID
	}
	runtimeA := create("sbx_adversarial_a")
	defer runtimeValue.Destroy(context.Background(), runtimeA)
	runtimeB := create("sbx_adversarial_b")
	defer runtimeValue.Destroy(context.Background(), runtimeB)

	localRuntime := runtimeValue.(*LocalRuntime)
	sessionA, err := localRuntime.session(runtimeA)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err := localRuntime.session(runtimeB)
	if err != nil {
		t.Fatal(err)
	}
	if sessionA.appContainerSID.String() == sessionB.appContainerSID.String() {
		t.Fatal("independent sandboxes reused the same AppContainer SID")
	}
	target := filepath.Join(sessionA.tmp, "sandbox-a-private.txt")
	if err := os.WriteFile(target, []byte("sandbox-a-only"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := runtimeValue.Exec(context.Background(), runtimeB, ExecRequest{
		Command: "helper.exe",
		Args:    []string{"-test.run=^TestWindowsAdversarialCrossSandboxAppContainerIsolation$"},
		Env: map[string]string{
			"OMNILLM_WINDOWS_ADVERSARIAL_ISOLATION_HELPER": "1",
			"OMNILLM_WINDOWS_ADVERSARIAL_TARGET":           target,
		},
		TimeoutMS: 15_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("cross-sandbox helper exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	for _, expected := range []string{"appcontainer=1", "outside_read=denied", "outside_write=denied"} {
		if !strings.Contains(result.Stdout, expected) {
			t.Fatalf("cross-sandbox helper output missing %q: %q", expected, result.Stdout)
		}
	}
	if _, err := os.Stat(target + ".write"); !os.IsNotExist(err) {
		t.Fatalf("sandbox B wrote into sandbox A authority: %v", err)
	}
}

func TestWindowsAdversarialExtensionDeniesHostNetworkAndAmbientSecrets(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_ISOLATION_HELPER") == "1" {
		windowsAdversarialIsolationHelperProcess()
		return
	}

	commandDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(commandDir, "helper.exe")
	if err := copyWindowsTestFile(executable, helperPath); err != nil {
		t.Fatal(err)
	}
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("host-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_WINDOWS_ADVERSARIAL_AMBIENT_SECRET", "must-not-leak")
	process, err := NewExtensionCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helperPath,
		Args:    []string{"-test.run=^TestWindowsAdversarialExtensionDeniesHostNetworkAndAmbientSecrets$"},
		Env: map[string]string{
			"OMNILLM_WINDOWS_ADVERSARIAL_ISOLATION_HELPER": "1",
			"OMNILLM_WINDOWS_ADVERSARIAL_TARGET":           outsideFile,
			"OMNILLM_WINDOWS_ADVERSARIAL_LOOPBACK":         listener.Addr().String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutDone := make(chan []byte, 1)
	stderrDone := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(stdout)
		stdoutDone <- data
	}()
	go func() {
		data, _ := io.ReadAll(stderr)
		stderrDone <- data
	}()
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	waitErr := process.Wait()
	stdoutBytes := <-stdoutDone
	stderrBytes := <-stderrDone
	if waitErr != nil {
		t.Fatalf("confined extension wait: %v stdout=%q stderr=%q", waitErr, stdoutBytes, stderrBytes)
	}
	output := string(stdoutBytes)
	for _, expected := range []string{
		"appcontainer=1",
		"workspace_write=denied",
		"outside_read=denied",
		"outside_write=denied",
		"ambient_secret=absent",
		"network=denied",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("confined extension output missing %q: stdout=%q stderr=%q", expected, output, stderrBytes)
		}
	}
	if _, err := os.Stat(outsideFile + ".write"); !os.IsNotExist(err) {
		t.Fatalf("confined extension wrote outside AppContainer profile: %v", err)
	}
}

func TestWindowsAdversarialExtensionRejectsUnrelatedAbsoluteHostArgument(t *testing.T) {
	commandDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(commandDir, "helper.exe")
	if err := copyWindowsTestFile(executable, helperPath); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := planWindowsExtensionLaunch(ProcessSpec{Command: helperPath, Dir: workspace})
	if err != nil {
		t.Fatal(err)
	}
	_, err = remapWindowsExtensionArgs([]string{outside}, plan, `C:\sandbox\extension`, `C:\sandbox\workspace`)
	if err == nil || !strings.Contains(err.Error(), "outside staged roots") {
		t.Fatalf("unrelated absolute argument remap error = %v", err)
	}
}

func TestWindowsAdversarialExtensionRejectsCredentialEnvironment(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV", "false")
	err := validateExtensionEnvironment(ProcessSpec{Env: map[string]string{"GITHUB_TOKEN": "must-not-enter-sandbox"}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "environment") {
		t.Fatalf("credential-sensitive extension environment error = %v", err)
	}
}

func TestWindowsAdversarialExtensionKillTerminatesDescendants(t *testing.T) {
	if os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_EXTENSION_CHILD") == "1" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_EXTENSION_ROOT") == "1" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("child_executable_error=%v\n", err)
			os.Exit(61)
		}
		child := exec.Command(executable, "-test.run=^TestWindowsAdversarialExtensionKillTerminatesDescendants$")
		child.Env = append(os.Environ(), "OMNILLM_WINDOWS_ADVERSARIAL_EXTENSION_CHILD=1")
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			fmt.Printf("child_start_error=%v\n", err)
			os.Exit(62)
		}
		fmt.Printf("child_pid=%d\n", child.Process.Pid)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}

	commandDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(commandDir, "helper.exe")
	if err := copyWindowsTestFile(executable, helperPath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	process, err := NewExtensionCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helperPath,
		Args:    []string{"-test.run=^TestWindowsAdversarialExtensionKillTerminatesDescendants$"},
		Env: map[string]string{
			"OMNILLM_WINDOWS_ADVERSARIAL_EXTENSION_ROOT": "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	lineDone := make(chan string, 1)
	go func() {
		line, _ := reader.ReadString('\n')
		lineDone <- line
	}()
	var line string
	select {
	case line = <-lineDone:
	case <-time.After(10 * time.Second):
		_ = process.Kill()
		t.Fatal("confined extension did not report descendant PID")
	}
	var childPID uint32
	if _, err := fmt.Sscanf(strings.TrimSpace(line), "child_pid=%d", &childPID); err != nil || childPID == 0 {
		_ = process.Kill()
		remaining, _ := io.ReadAll(stderr)
		t.Fatalf("parse descendant PID from %q: err=%v stderr=%q", line, err, remaining)
	}
	childHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, childPID)
	if err != nil {
		_ = process.Kill()
		t.Fatalf("open confined extension descendant %d: %v", childPID, err)
	}
	defer windows.CloseHandle(childHandle)

	if err := process.Kill(); err != nil {
		t.Fatalf("kill confined extension process tree: %v", err)
	}
	_ = process.Wait()
	waitResult, err := windows.WaitForSingleObject(childHandle, 2_000)
	if err != nil {
		t.Fatalf("wait for confined extension descendant teardown: %v", err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		t.Fatalf("confined extension descendant survived forced shutdown: wait=%#x", waitResult)
	}
}

func windowsAdversarialIsolationHelperProcess() {
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
		os.Exit(51)
	}
	fmt.Printf("appcontainer=%d\n", isAppContainer)

	if err := os.WriteFile("adversarial-workspace-write.txt", []byte("blocked"), 0o600); err != nil {
		fmt.Println("workspace_write=denied")
	} else {
		fmt.Println("workspace_write=allowed")
	}
	target := os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_TARGET")
	if target != "" {
		if _, err := os.ReadFile(target); err != nil {
			fmt.Println("outside_read=denied")
		} else {
			fmt.Println("outside_read=allowed")
		}
		if err := os.WriteFile(target+".write", []byte("blocked"), 0o600); err != nil {
			fmt.Println("outside_write=denied")
		} else {
			fmt.Println("outside_write=allowed")
		}
	}
	if os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_AMBIENT_SECRET") == "" {
		fmt.Println("ambient_secret=absent")
	} else {
		fmt.Println("ambient_secret=present")
	}
	if address := os.Getenv("OMNILLM_WINDOWS_ADVERSARIAL_LOOPBACK"); address != "" {
		connection, err := net.DialTimeout("tcp", address, 1500*time.Millisecond)
		if err != nil {
			fmt.Println("network=denied")
		} else {
			_ = connection.Close()
			fmt.Println("network=allowed")
		}
	}
	os.Exit(0)
}
