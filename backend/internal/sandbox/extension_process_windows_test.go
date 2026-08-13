//go:build windows

package sandbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsExtensionHelperEnv = "OMNILLM_WINDOWS_EXTENSION_HELPER"

func TestWindowsExtensionNativeIsolationAndCrossProfileAuthority(t *testing.T) {
	switch os.Getenv(windowsExtensionHelperEnv) {
	case "hold":
		time.Sleep(30 * time.Second)
		return
	case "isolation":
		windowsExtensionIsolationHelper()
		return
	}

	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_MASTER_KEY", "ambient-secret-must-not-leak")

	helper := copyWindowsExtensionTestExecutable(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	outsideDir := t.TempDir()
	outsideRead := filepath.Join(outsideDir, "outside-secret.txt")
	outsideWrite := filepath.Join(outsideDir, "outside-write.txt")
	if err := os.WriteFile(outsideRead, []byte("host-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	peerProcess, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Args:    []string{"-test.run=^TestWindowsExtensionNativeIsolationAndCrossProfileAuthority$"},
		Env:     map[string]string{windowsExtensionHelperEnv: "hold"},
	})
	if err != nil {
		t.Fatal(err)
	}
	peer, ok := peerProcess.(*windowsExtensionProcess)
	if !ok {
		t.Fatalf("peer process type = %T, want *windowsExtensionProcess", peerProcess)
	}
	if err := peer.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = peer.Kill()
		_ = peer.Wait()
	}()

	peer.mu.Lock()
	peerRoot := peer.profileRoot
	peer.mu.Unlock()
	peerRead := filepath.Join(peerRoot, "home", "peer-secret.txt")
	peerWrite := filepath.Join(peerRoot, "home", "peer-write.txt")
	if err := os.WriteFile(peerRead, []byte("peer-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: helper,
		Args:    []string{"-test.run=^TestWindowsExtensionNativeIsolationAndCrossProfileAuthority$"},
		Env: map[string]string{
			windowsExtensionHelperEnv:                    "isolation",
			"OMNILLM_WINDOWS_EXTENSION_LOOPBACK":        listener.Addr().String(),
			"OMNILLM_WINDOWS_EXTENSION_OUTSIDE_READ":    outsideRead,
			"OMNILLM_WINDOWS_EXTENSION_OUTSIDE_WRITE":   outsideWrite,
			"OMNILLM_WINDOWS_EXTENSION_PEER_READ":       peerRead,
			"OMNILLM_WINDOWS_EXTENSION_PEER_WRITE":      peerWrite,
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
	stdoutBytes, stdoutErr := io.ReadAll(stdout)
	stderrBytes, stderrErr := io.ReadAll(stderr)
	if stdoutErr != nil {
		t.Fatal(stdoutErr)
	}
	if stderrErr != nil {
		t.Fatal(stderrErr)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("confined extension wait: %v stdout=%q stderr=%q", err, stdoutBytes, stderrBytes)
	}

	output := string(stdoutBytes)
	for _, expected := range []string{
		"appcontainer=1",
		"extension_write=denied",
		"home_write=ok",
		"outside_read=denied",
		"outside_write=denied",
		"peer_read=denied",
		"peer_write=denied",
		"ambient_secret=absent",
		"network=denied",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("extension helper output missing %q: stdout=%q stderr=%q", expected, output, stderrBytes)
		}
	}
	if _, err := os.Stat(outsideWrite); !os.IsNotExist(err) {
		t.Fatalf("confined extension wrote outside AppContainer profile: %v", err)
	}
	if _, err := os.Stat(peerWrite); !os.IsNotExist(err) {
		t.Fatalf("one extension reused another extension's profile authority: %v", err)
	}
}

func TestWindowsExtensionKillsDescendantsAfterRootExit(t *testing.T) {
	switch os.Getenv(windowsExtensionHelperEnv) {
	case "descendant-child":
		time.Sleep(30 * time.Second)
		return
	case "descendant-root":
		windowsExtensionSpawnChildHelper(t, "descendant-child", time.Second)
		return
	}

	process, stdout := startWindowsExtensionTreeTest(t, "descendant-root", context.Background())
	child := openWindowsExtensionChildFromOutput(t, stdout)
	defer windows.CloseHandle(child)

	if err := process.Wait(); err != nil {
		t.Fatalf("root extension wait: %v", err)
	}
	assertWindowsProcessTerminated(t, child, 3*time.Second)
}

func TestWindowsExtensionContextCancellationKillsProcessTree(t *testing.T) {
	switch os.Getenv(windowsExtensionHelperEnv) {
	case "cancel-child":
		time.Sleep(30 * time.Second)
		return
	case "cancel-root":
		windowsExtensionSpawnChildHelper(t, "cancel-child", 30*time.Second)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	process, stdout := startWindowsExtensionTreeTest(t, "cancel-root", ctx)
	child := openWindowsExtensionChildFromOutput(t, stdout)
	defer windows.CloseHandle(child)

	cancel()
	if err := process.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled extension wait error = %v, want context.Canceled", err)
	}
	assertWindowsProcessTerminated(t, child, 3*time.Second)
}

func TestWindowsWorkspaceStagingRejectsHardLinks(t *testing.T) {
	source := t.TempDir()
	first := filepath.Join(source, "first.txt")
	second := filepath.Join(source, "second.txt")
	if err := os.WriteFile(first, []byte("linked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatalf("create hard link: %v", err)
	}

	err := stageWindowsReadOnlyWorkspace(source, t.TempDir())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "multiply-linked") {
		t.Fatalf("hard-link staging error = %v", err)
	}
}

func TestWindowsWorkspaceStagingRejectsJunctionReparsePoint(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	junction := filepath.Join(source, "junction")
	command := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create junction reparse point: %v output=%s", err, output)
	}

	err := stageWindowsReadOnlyWorkspace(source, t.TempDir())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "reparse") {
		t.Fatalf("junction staging error = %v", err)
	}
}

func TestWindowsExtensionRejectsUnrelatedAbsoluteArgument(t *testing.T) {
	commandRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	outsideRoot := t.TempDir()
	outside := filepath.Join(outsideRoot, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := remapWindowsExtensionArgs([]string{outside}, windowsExtensionLaunchPlan{
		commandDir:      commandRoot,
		sourceWorkspace: workspaceRoot,
	}, filepath.Join(t.TempDir(), "extension"), filepath.Join(t.TempDir(), "workspace"))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "outside staged roots") {
		t.Fatalf("unrelated absolute argument error = %v", err)
	}
}

func TestWindowsExtensionRequiredModeRejectsSensitiveEnvironment(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV", "false")

	_, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: "cmd.exe",
		Env:     map[string]string{"GITHUB_TOKEN": "must-not-enter-sandbox"},
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "environment") {
		t.Fatalf("sensitive extension environment error = %v", err)
	}
}

func windowsExtensionIsolationHelper() {
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
		os.Exit(61)
	}
	fmt.Printf("appcontainer=%d\n", isAppContainer)

	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("executable_error=%v\n", err)
		os.Exit(62)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(executable), "extension-write.txt"), []byte("write"), 0o600); err != nil {
		fmt.Println("extension_write=denied")
	} else {
		fmt.Println("extension_write=allowed")
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), "home-write.txt"), []byte("write"), 0o600); err != nil {
		fmt.Printf("home_write=failed:%v\n", err)
	} else {
		fmt.Println("home_write=ok")
	}
	windowsExtensionReportFileAccess("outside_read", os.Getenv("OMNILLM_WINDOWS_EXTENSION_OUTSIDE_READ"), false)
	windowsExtensionReportFileAccess("outside_write", os.Getenv("OMNILLM_WINDOWS_EXTENSION_OUTSIDE_WRITE"), true)
	windowsExtensionReportFileAccess("peer_read", os.Getenv("OMNILLM_WINDOWS_EXTENSION_PEER_READ"), false)
	windowsExtensionReportFileAccess("peer_write", os.Getenv("OMNILLM_WINDOWS_EXTENSION_PEER_WRITE"), true)

	if os.Getenv("OMNILLM_MASTER_KEY") == "" {
		fmt.Println("ambient_secret=absent")
	} else {
		fmt.Println("ambient_secret=present")
	}
	connection, err := net.DialTimeout("tcp", os.Getenv("OMNILLM_WINDOWS_EXTENSION_LOOPBACK"), 750*time.Millisecond)
	if err != nil {
		fmt.Println("network=denied")
	} else {
		_ = connection.Close()
		fmt.Println("network=allowed")
	}
}

func windowsExtensionReportFileAccess(label, path string, write bool) {
	var err error
	if write {
		err = os.WriteFile(path, []byte("write"), 0o600)
	} else {
		_, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Printf("%s=denied\n", label)
	} else {
		fmt.Printf("%s=allowed\n", label)
	}
}

func copyWindowsExtensionTestExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	copyPath := filepath.Join(directory, "extension-helper.exe")
	if err := copyWindowsTestFile(executable, copyPath); err != nil {
		t.Fatalf("copy Windows extension helper: %v", err)
	}
	return copyPath
}

func startWindowsExtensionTreeTest(t *testing.T, helperMode string, ctx context.Context) (CommandProcess, io.ReadCloser) {
	t.Helper()
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	helper := copyWindowsExtensionTestExecutable(t)
	process, err := NewHostCommandRunner().CommandContext(ctx, ProcessSpec{
		Command: helper,
		Args:    []string{"-test.run=^" + t.Name() + "$"},
		Env:     map[string]string{windowsExtensionHelperEnv: helperMode},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := process.StderrPipe(); err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	return process, stdout
}

func windowsExtensionSpawnChildHelper(t *testing.T, childMode string, rootLifetime time.Duration) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("child_executable_error=%v\n", err)
		os.Exit(71)
	}
	command := exec.Command(executable, "-test.run=^"+t.Name()+"$")
	command.Env = append(os.Environ(), windowsExtensionHelperEnv+"="+childMode)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		fmt.Printf("child_start_error=%v\n", err)
		os.Exit(72)
	}
	fmt.Printf("child_pid=%d\n", command.Process.Pid)
	time.Sleep(rootLifetime)
}

func openWindowsExtensionChildFromOutput(t *testing.T, stdout io.Reader) windows.Handle {
	t.Helper()
	scanner := bufio.NewScanner(stdout)
	deadline := time.After(5 * time.Second)
	lineCh := make(chan string, 1)
	go func() {
		if scanner.Scan() {
			lineCh <- scanner.Text()
			return
		}
		lineCh <- ""
	}()
	var line string
	select {
	case line = <-lineCh:
	case <-deadline:
		t.Fatal("timed out waiting for descendant PID")
	}
	if !strings.HasPrefix(line, "child_pid=") {
		t.Fatalf("unexpected descendant output %q", line)
	}
	pidValue, err := strconv.ParseUint(strings.TrimPrefix(line, "child_pid="), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pidValue))
	if err != nil {
		t.Fatalf("open descendant process %d: %v", pidValue, err)
	}
	return handle
}

func assertWindowsProcessTerminated(t *testing.T, handle windows.Handle, timeout time.Duration) {
	t.Helper()
	result, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		t.Fatalf("wait for descendant termination: %v", err)
	}
	if result != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant process survived Job teardown: wait result %#x", result)
	}
}
