//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const (
	defaultWindowsLocalExecTimeout = 30 * time.Second
	defaultWindowsLocalOutputBytes = 1 << 20
	maxWindowsStagedWorkspaceBytes = int64(256 << 20)
	maxWindowsStagedWorkspaceFiles = 20_000
	windowsDestroyWaitTimeout      = 10 * time.Second
	windowsFileDeleteChild         = windows.ACCESS_MASK(0x00000040)
)

type windowsLocalRuntimeSession struct {
	id              string
	spec            CreateRequest
	profileName     string
	appContainerSID *windows.SID
	root            string
	workspace       string
	home            string
	tmp             string
	workspaceMode   string
	destroying      bool
}

type windowsActiveExecution struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// LocalRuntime is the first-party Windows AppContainer execution runtime. It
// stages read-only workspace input into an ephemeral AppContainer profile and
// launches each execution with AppContainer security capabilities, Job Object
// membership, and an explicit inherited-handle list applied at process creation.
type LocalRuntime struct {
	mu       sync.RWMutex
	sessions map[string]windowsLocalRuntimeSession
	active   map[string]windowsActiveExecution
}

// NewLocalRuntime constructs the Windows AppContainer runtime. Windows 10 is
// required because the runtime binds Job Object membership with the process
// creation attribute list; older Windows versions fail closed instead of using
// post-start process assignment.
func NewLocalRuntime(_ LocalRuntimeConfig) (Runtime, error) {
	major, _, _ := windows.RtlGetNtVersionNumbers()
	if major < 10 {
		return nil, fmt.Errorf("Windows AppContainer sandbox requires Windows 10 or newer")
	}
	for name, proc := range map[string]*windows.LazyProc{
		"CreateAppContainerProfile":                 windowsCreateAppContainerProfileProc,
		"DeriveAppContainerSidFromAppContainerName": windowsDeriveAppContainerSIDProc,
		"DeleteAppContainerProfile":                 windowsDeleteAppContainerProfileProc,
		"GetAppContainerFolderPath":                 windowsGetAppContainerFolderPathProc,
	} {
		if err := proc.Find(); err != nil {
			return nil, fmt.Errorf("Windows AppContainer API %s is unavailable: %w", name, err)
		}
	}
	return &LocalRuntime{
		sessions: make(map[string]windowsLocalRuntimeSession),
		active:   make(map[string]windowsActiveExecution),
	}, nil
}

// Capabilities reports only controls enforced by this implementation. Windows
// Job Objects enforce process-count quotas from process creation. Memory, CPU,
// disk, and destination allowlisting remain false until separately implemented
// and proven. AppContainer is launched with no network capabilities.
func (r *LocalRuntime) Capabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		Name:                 "windows-appcontainer",
		Version:              "1",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		NetworkAllowlist:     false,
		ProcessTreeIsolation: true,
		MemoryLimit:          false,
		CPULimit:             false,
		PIDLimit:             true,
		DiskLimit:            false,
	}
}

func (r *LocalRuntime) Create(_ context.Context, request RuntimeCreateRequest) (string, error) {
	if strings.TrimSpace(request.SessionID) == "" || request.Owner.Empty() {
		return "", fmt.Errorf("runtime create requires broker session and owner")
	}
	if err := validateCreateRequest(request.Spec); err != nil {
		return "", err
	}
	if err := requireCapabilities(r.Capabilities(), request.Spec.Requirements); err != nil {
		return "", err
	}
	if request.Spec.Network.Mode != "" && request.Spec.Network.Mode != NetworkNone {
		return "", fmt.Errorf("Windows AppContainer runtime currently supports only network mode %q", NetworkNone)
	}
	mounts, err := validateWindowsRuntimeMounts(request)
	if err != nil {
		return "", err
	}

	profileName := "OmniLLM.Sandbox." + strings.ReplaceAll(uuid.NewString(), "-", "")
	appSID, err := createWindowsAppContainerProfile(profileName)
	if err != nil {
		return "", err
	}
	cleanupProfile := true
	root := ""
	defer func() {
		if cleanupProfile {
			if root != "" {
				_ = os.RemoveAll(root)
			}
			_ = deleteWindowsAppContainerProfile(profileName)
		}
	}()

	root, err = windowsAppContainerFolderPath(appSID)
	if err != nil {
		return "", err
	}
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	for _, dir := range []string{workspace, home, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create Windows sandbox directory %q: %w", dir, err)
		}
	}

	readOnlyAccess := windows.ACCESS_MASK(
		windows.FILE_GENERIC_READ |
			windows.FILE_GENERIC_EXECUTE |
			windows.FILE_LIST_DIRECTORY |
			windows.FILE_TRAVERSE,
	)
	readWriteAccess := windows.ACCESS_MASK(
		windows.FILE_GENERIC_READ|
			windows.FILE_GENERIC_WRITE|
			windows.FILE_GENERIC_EXECUTE|
			windows.FILE_LIST_DIRECTORY|
			windows.FILE_TRAVERSE|
			windows.DELETE,
	) | windowsFileDeleteChild

	workspaceMode := "ephemeral"
	workspaceAccess := readWriteAccess
	if len(mounts) == 1 {
		workspaceMode = string(MountReadOnly)
		workspaceAccess = readOnlyAccess
	}
	// Protect the workspace DACL before staging any children so copied content
	// inherits the intended read-only package authority from creation time.
	if err := setWindowsAppContainerDirectoryAccess(workspace, appSID, workspaceAccess); err != nil {
		return "", err
	}
	if err := setWindowsAppContainerDirectoryAccess(home, appSID, readWriteAccess); err != nil {
		return "", err
	}
	if err := setWindowsAppContainerDirectoryAccess(tmp, appSID, readWriteAccess); err != nil {
		return "", err
	}
	if len(mounts) == 1 {
		if err := stageWindowsReadOnlyWorkspace(mounts[0].SourcePath, workspace); err != nil {
			return "", err
		}
	}

	runtimeID := "rt_" + uuid.NewString()
	session := windowsLocalRuntimeSession{
		id:              runtimeID,
		spec:            cloneCreateRequest(request.Spec),
		profileName:     profileName,
		appContainerSID: appSID,
		root:            root,
		workspace:       workspace,
		home:            home,
		tmp:             tmp,
		workspaceMode:   workspaceMode,
	}
	r.mu.Lock()
	r.sessions[runtimeID] = session
	r.mu.Unlock()
	cleanupProfile = false

	ttl := defaultSessionTTL
	if request.Spec.TTLSeconds > 0 {
		ttl = time.Duration(request.Spec.TTLSeconds) * time.Second
	}
	time.AfterFunc(ttl, func() {
		_ = r.Destroy(context.Background(), runtimeID)
	})
	return runtimeID, nil
}

func validateWindowsRuntimeMounts(request RuntimeCreateRequest) ([]RuntimeMount, error) {
	if len(request.Spec.Mounts) != len(request.ResolvedMounts) {
		return nil, fmt.Errorf("runtime workspace mount resolution mismatch")
	}
	if len(request.ResolvedMounts) > 1 {
		return nil, fmt.Errorf("Windows AppContainer runtime currently supports at most one workspace mount")
	}
	out := make([]RuntimeMount, 0, len(request.ResolvedMounts))
	for i, resolved := range request.ResolvedMounts {
		requested := request.Spec.Mounts[i]
		if strings.TrimSpace(resolved.WorkspaceID) == "" || resolved.WorkspaceID != requested.WorkspaceID {
			return nil, fmt.Errorf("runtime workspace mount identity mismatch")
		}
		if resolved.Mode != requested.Mode {
			return nil, fmt.Errorf("runtime workspace mount mode mismatch")
		}
		if err := validateMountMode(resolved.Mode); err != nil {
			return nil, err
		}
		if resolved.Mode != MountReadOnly {
			return nil, fmt.Errorf("Windows AppContainer runtime currently accepts only %q workspace mounts", MountReadOnly)
		}
		source, err := windowsCanonicalDirectory(resolved.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("runtime workspace %q: %w", resolved.WorkspaceID, err)
		}
		resolved.SourcePath = source
		out = append(out, resolved)
	}
	return out, nil
}

func (r *LocalRuntime) Exec(ctx context.Context, runtimeID string, request ExecRequest) (*ExecResult, error) {
	session, err := r.session(runtimeID)
	if err != nil {
		return nil, err
	}
	application, args, err := windowsSandboxCommand(session, request)
	if err != nil {
		return nil, err
	}
	directory, err := windowsSandboxDirectory(session.workspace, request.Directory)
	if err != nil {
		return nil, err
	}
	environment, err := windowsSandboxEnvironment(session, request)
	if err != nil {
		return nil, err
	}

	timeout := defaultWindowsLocalExecTimeout
	if session.spec.Resources.WallTimeMS > 0 {
		timeout = time.Duration(session.spec.Resources.WallTimeMS) * time.Millisecond
	}
	if request.TimeoutMS > 0 {
		requested := time.Duration(request.TimeoutMS) * time.Millisecond
		if timeout <= 0 || requested < timeout {
			timeout = requested
		}
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	executionID, err := executionIDOrNew(request.ExecutionID)
	if err != nil {
		cancel()
		return nil, err
	}
	activeKey := runtimeID + "\x00" + executionID
	done := make(chan struct{})
	r.mu.Lock()
	current, ok := r.sessions[runtimeID]
	if !ok || current.destroying {
		r.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("sandbox runtime session is being destroyed")
	}
	if _, exists := r.active[activeKey]; exists {
		r.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("sandbox execution id is already active")
	}
	r.active[activeKey] = windowsActiveExecution{cancel: cancel, done: done}
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.active, activeKey)
		r.mu.Unlock()
		close(done)
	}()

	stdoutLimit := session.spec.Resources.MaxStdoutBytes
	if stdoutLimit <= 0 {
		stdoutLimit = defaultWindowsLocalOutputBytes
	}
	stderrLimit := session.spec.Resources.MaxStderrBytes
	if stderrLimit <= 0 {
		stderrLimit = defaultWindowsLocalOutputBytes
	}

	started := time.Now()
	exitCode, stdout, stderr, stdoutTruncated, stderrTruncated, runErr := runWindowsAppContainerProcess(
		execCtx,
		session,
		application,
		args,
		directory,
		environment,
		request.Stdin,
		stdoutLimit,
		stderrLimit,
	)
	duration := time.Since(started).Milliseconds()
	if runErr != nil {
		return nil, runErr
	}
	return &ExecResult{
		ExecutionID: executionID,
		Stdout:      stdout,
		Stderr:      stderr,
		ExitCode:    exitCode,
		DurationMS:  duration,
		Metadata: map[string]any{
			"runtime":          r.Capabilities().Name,
			"stdout_truncated": stdoutTruncated,
			"stderr_truncated": stderrTruncated,
			"network":          "none",
			"workspace_mode":   session.workspaceMode,
		},
	}, nil
}

func (r *LocalRuntime) Cancel(_ context.Context, runtimeID, executionID string) error {
	if err := validateExecutionID(executionID); err != nil {
		return err
	}
	key := runtimeID + "\x00" + executionID
	r.mu.RLock()
	execution, ok := r.active[key]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sandbox execution not found")
	}
	execution.cancel()
	return nil
}

func (r *LocalRuntime) Status(_ context.Context, runtimeID string) (*Status, error) {
	if _, err := r.session(runtimeID); err != nil {
		return nil, err
	}
	return &Status{State: "ready", Capabilities: r.Capabilities()}, nil
}

// Destroy is idempotent so explicit cleanup and TTL expiry may race safely. It
// marks the session as destroying before cancellation so no new execution can
// register after teardown has begun. Profile data is removed only after every
// execution reports that its Job/process/pipe teardown is complete.
func (r *LocalRuntime) Destroy(ctx context.Context, runtimeID string) error {
	r.mu.Lock()
	session, ok := r.sessions[runtimeID]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	if !session.destroying {
		session.destroying = true
		r.sessions[runtimeID] = session
	}
	active := make([]windowsActiveExecution, 0)
	for key, execution := range r.active {
		if strings.HasPrefix(key, runtimeID+"\x00") {
			execution.cancel()
			active = append(active, execution)
		}
	}
	r.mu.Unlock()

	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if _, hasDeadline := waitCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(waitCtx, windowsDestroyWaitTimeout)
		defer cancel()
	}
	for _, execution := range active {
		select {
		case <-execution.done:
		case <-waitCtx.Done():
			return fmt.Errorf("wait for Windows sandbox execution teardown: %w", waitCtx.Err())
		}
	}

	var cleanupErr error
	if err := os.RemoveAll(session.root); err != nil {
		cleanupErr = fmt.Errorf("remove Windows sandbox profile data: %w", err)
	}
	if err := deleteWindowsAppContainerProfile(session.profileName); err != nil && cleanupErr == nil {
		cleanupErr = err
	}
	if cleanupErr != nil {
		// Keep the session registered in destroying state so a later explicit or
		// TTL cleanup can retry an idempotent profile teardown. Removing it here
		// would make a partially-cleaned AppContainer unreachable.
		return cleanupErr
	}

	r.mu.Lock()
	if current, exists := r.sessions[runtimeID]; exists && current.profileName == session.profileName {
		delete(r.sessions, runtimeID)
	}
	r.mu.Unlock()
	return nil
}

func (r *LocalRuntime) session(runtimeID string) (windowsLocalRuntimeSession, error) {
	r.mu.RLock()
	session, ok := r.sessions[runtimeID]
	r.mu.RUnlock()
	if !ok {
		return windowsLocalRuntimeSession{}, fmt.Errorf("sandbox runtime session not found")
	}
	if session.destroying {
		return windowsLocalRuntimeSession{}, fmt.Errorf("sandbox runtime session is being destroyed")
	}
	return session, nil
}

func windowsSandboxCommand(session windowsLocalRuntimeSession, request ExecRequest) (string, []string, error) {
	hasCode := strings.TrimSpace(request.Code) != "" || strings.TrimSpace(request.Language) != ""
	hasCommand := strings.TrimSpace(request.Command) != ""
	if hasCode == hasCommand {
		return "", nil, fmt.Errorf("sandbox execution must specify exactly one of code or command mode")
	}
	if hasCommand {
		if strings.ContainsRune(request.Command, '\x00') {
			return "", nil, fmt.Errorf("sandbox command contains NUL")
		}
		args := append([]string(nil), request.Args...)
		for _, arg := range args {
			if strings.ContainsRune(arg, '\x00') {
				return "", nil, fmt.Errorf("sandbox argument contains NUL")
			}
		}
		application, err := resolveWindowsSandboxExecutable(session.workspace, request.Command)
		return application, args, err
	}
	if strings.TrimSpace(request.Code) == "" {
		return "", nil, fmt.Errorf("sandbox code is required")
	}
	switch strings.ToLower(strings.TrimSpace(request.Language)) {
	case "shell":
		systemDir, err := windows.GetSystemDirectory()
		if err != nil {
			return "", nil, fmt.Errorf("resolve Windows system directory: %w", err)
		}
		return filepath.Join(systemDir, "cmd.exe"), []string{"/d", "/s", "/c", request.Code}, nil
	case "python", "javascript":
		return "", nil, fmt.Errorf("Windows AppContainer runtime does not yet advertise an AppContainer-readable %s interpreter", strings.ToLower(strings.TrimSpace(request.Language)))
	default:
		return "", nil, fmt.Errorf("unsupported sandbox language %q", request.Language)
	}
}

func resolveWindowsSandboxExecutable(workspace, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("sandbox command is required")
	}
	if filepath.IsAbs(command) || filepath.VolumeName(command) != "" {
		return "", fmt.Errorf("sandbox command path must be workspace-relative or a system command basename")
	}
	if strings.ContainsAny(command, `/\\`) {
		clean := filepath.Clean(command)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("sandbox command escapes workspace")
		}
		candidate := filepath.Join(workspace, clean)
		if !windowsPathWithin(workspace, candidate) {
			return "", fmt.Errorf("sandbox command escapes workspace")
		}
		if err := requireWindowsRegularFile(candidate); err != nil {
			return "", err
		}
		return candidate, nil
	}
	for _, name := range []string{command, command + ".exe"} {
		candidate := filepath.Join(workspace, name)
		if err := requireWindowsRegularFile(candidate); err == nil {
			return candidate, nil
		}
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return "", fmt.Errorf("resolve Windows system directory: %w", err)
	}
	name := command
	if filepath.Ext(name) == "" {
		name += ".exe"
	}
	candidate := filepath.Join(systemDir, name)
	if err := requireWindowsRegularFile(candidate); err != nil {
		return "", fmt.Errorf("sandbox command %q was not found in workspace or System32", command)
	}
	return candidate, nil
}

func requireWindowsRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("sandbox executable %q is not a regular file", path)
	}
	return nil
}

func windowsSandboxDirectory(workspace, directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" || directory == "." {
		return workspace, nil
	}
	if filepath.IsAbs(directory) || filepath.VolumeName(directory) != "" {
		return "", fmt.Errorf("sandbox directory must be workspace-relative")
	}
	clean := filepath.Clean(directory)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sandbox directory escapes workspace")
	}
	resolved := filepath.Join(workspace, clean)
	if !windowsPathWithin(workspace, resolved) {
		return "", fmt.Errorf("sandbox directory escapes workspace")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("sandbox directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("sandbox directory %q is not a directory", directory)
	}
	return resolved, nil
}

func windowsSandboxEnvironment(session windowsLocalRuntimeSession, request ExecRequest) ([]uint16, error) {
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows system directory: %w", err)
	}
	windowsDir, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows directory: %w", err)
	}
	values := map[string]string{
		"SystemRoot":   windowsDir,
		"WINDIR":       windowsDir,
		"PATH":         systemDir,
		"COMSPEC":      filepath.Join(systemDir, "cmd.exe"),
		"HOME":         session.home,
		"USERPROFILE":  session.home,
		"TEMP":         session.tmp,
		"TMP":          session.tmp,
		"LOCALAPPDATA": session.root,
	}
	reserved := map[string]struct{}{}
	for key := range values {
		reserved[strings.ToUpper(key)] = struct{}{}
	}
	merge := func(source map[string]string) error {
		for key, value := range source {
			if err := validateSandboxEnvironmentEntry(key, value); err != nil {
				return err
			}
			if _, exists := reserved[strings.ToUpper(strings.TrimSpace(key))]; exists {
				return fmt.Errorf("sandbox environment key %q is runtime-reserved on Windows", key)
			}
			values[key] = value
		}
		return nil
	}
	if err := merge(session.spec.Environment); err != nil {
		return nil, err
	}
	if err := merge(request.Env); err != nil {
		return nil, err
	}
	return windowsEnvironmentBlock(values)
}

func windowsEnvironmentBlock(values map[string]string) ([]uint16, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToUpper(keys[i]) < strings.ToUpper(keys[j])
	})
	block := make([]uint16, 0, len(keys)*16)
	for _, key := range keys {
		entry, err := windows.UTF16FromString(key + "=" + values[key])
		if err != nil {
			return nil, fmt.Errorf("encode Windows sandbox environment %q: %w", key, err)
		}
		block = append(block, entry...)
	}
	block = append(block, 0)
	return block, nil
}

func runWindowsAppContainerProcess(
	ctx context.Context,
	session windowsLocalRuntimeSession,
	application string,
	args []string,
	directory string,
	environment []uint16,
	stdin []byte,
	stdoutLimit int64,
	stderrLimit int64,
) (int, string, string, bool, bool, error) {
	job, err := createWindowsSandboxJob(session.spec.Resources.MaxProcesses)
	if err != nil {
		return 0, "", "", false, false, err
	}
	defer windows.CloseHandle(job)

	pipes, err := createWindowsSandboxPipes()
	if err != nil {
		return 0, "", "", false, false, err
	}
	defer pipes.closeAll()

	attributes, err := windows.NewProcThreadAttributeList(3)
	if err != nil {
		return 0, "", "", false, false, fmt.Errorf("create Windows process attribute list: %w", err)
	}
	defer attributes.Delete()
	securityCapabilities := windowsSecurityCapabilities{AppContainerSID: session.appContainerSID}
	if err := attributes.Update(
		windowsProcThreadAttributeSecurityCapabilities,
		unsafe.Pointer(&securityCapabilities),
		unsafe.Sizeof(securityCapabilities),
	); err != nil {
		return 0, "", "", false, false, fmt.Errorf("attach AppContainer security capabilities: %w", err)
	}
	jobs := []windows.Handle{job}
	if err := attributes.Update(
		windowsProcThreadAttributeJobList,
		unsafe.Pointer(&jobs[0]),
		unsafe.Sizeof(jobs[0]),
	); err != nil {
		return 0, "", "", false, false, fmt.Errorf("attach Job Object at process creation: %w", err)
	}
	handles := []windows.Handle{pipes.childStdin, pipes.childStdout, pipes.childStderr}
	if err := attributes.Update(
		windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		unsafe.Pointer(&handles[0]),
		uintptr(len(handles))*unsafe.Sizeof(handles[0]),
	); err != nil {
		return 0, "", "", false, false, fmt.Errorf("restrict inherited process handles: %w", err)
	}

	applicationPtr, err := windows.UTF16PtrFromString(application)
	if err != nil {
		return 0, "", "", false, false, fmt.Errorf("encode sandbox executable path: %w", err)
	}
	commandLine := windowsCommandLine(application, args)
	commandLineUTF16, err := windows.UTF16FromString(commandLine)
	if err != nil {
		return 0, "", "", false, false, fmt.Errorf("encode sandbox command line: %w", err)
	}
	directoryPtr, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return 0, "", "", false, false, fmt.Errorf("encode sandbox working directory: %w", err)
	}
	if len(environment) == 0 {
		return 0, "", "", false, false, fmt.Errorf("Windows sandbox environment block is empty")
	}

	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput = pipes.childStdin
	startup.StdOutput = pipes.childStdout
	startup.StdErr = pipes.childStderr
	startup.ProcThreadAttributeList = attributes.List()
	processInfo := windows.ProcessInformation{}
	creationFlags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NO_WINDOW)
	if err := windows.CreateProcess(
		applicationPtr,
		&commandLineUTF16[0],
		nil,
		nil,
		true,
		creationFlags,
		&environment[0],
		directoryPtr,
		&startup.StartupInfo,
		&processInfo,
	); err != nil {
		return 0, "", "", false, false, fmt.Errorf("create Windows AppContainer process: %w", err)
	}
	defer windows.CloseHandle(processInfo.Process)
	defer windows.CloseHandle(processInfo.Thread)
	pipes.closeChildEnds()

	stdout := newWindowsBoundedOutput(stdoutLimit)
	stderr := newWindowsBoundedOutput(stderrLimit)
	copyDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(stdout, pipes.parentStdout)
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(stderr, pipes.parentStderr)
		copyDone <- struct{}{}
	}()
	stdinDone := make(chan struct{}, 1)
	go func() {
		if len(stdin) > 0 {
			_, _ = pipes.parentStdin.Write(stdin)
		}
		_ = pipes.parentStdin.Close()
		stdinDone <- struct{}{}
	}()

	waitDone := make(chan error, 1)
	go func() {
		result, waitErr := windows.WaitForSingleObject(processInfo.Process, windows.INFINITE)
		if waitErr != nil {
			waitDone <- waitErr
			return
		}
		if result != windows.WAIT_OBJECT_0 {
			waitDone <- fmt.Errorf("unexpected process wait result %#x", result)
			return
		}
		waitDone <- nil
	}()

	cancelled := false
	select {
	case waitErr := <-waitDone:
		if waitErr != nil {
			_ = windows.TerminateJobObject(job, 1)
			_ = pipes.parentStdin.Close()
			<-stdinDone
			<-copyDone
			<-copyDone
			return 0, "", "", false, false, fmt.Errorf("wait for Windows sandbox process: %w", waitErr)
		}
	case <-ctx.Done():
		cancelled = true
		_ = windows.TerminateJobObject(job, 1)
		<-waitDone
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(processInfo.Process, &exitCode); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		_ = pipes.parentStdin.Close()
		<-stdinDone
		<-copyDone
		<-copyDone
		return 0, "", "", false, false, fmt.Errorf("query Windows sandbox exit code: %w", err)
	}
	// The root process defines execution completion. Terminate any descendants
	// still in the Job Object so they cannot outlive the execution or keep stdio
	// handles open indefinitely, then drain every pipe before returning.
	_ = windows.TerminateJobObject(job, 1)
	_ = pipes.parentStdin.Close()
	<-stdinDone
	<-copyDone
	<-copyDone
	if cancelled {
		return 0, "", "", false, false, fmt.Errorf("sandbox execution cancelled or timed out: %w", ctx.Err())
	}
	return int(exitCode), stdout.String(), stderr.String(), stdout.Truncated(), stderr.Truncated(), nil
}

func windowsCommandLine(application string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, syscall.EscapeArg(application))
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	return strings.Join(parts, " ")
}

type windowsSandboxPipes struct {
	childStdin   windows.Handle
	childStdout  windows.Handle
	childStderr  windows.Handle
	parentStdin  *os.File
	parentStdout *os.File
	parentStderr *os.File
}

func createWindowsSandboxPipes() (*windowsSandboxPipes, error) {
	security := windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	var childStdin, parentStdin windows.Handle
	if err := windows.CreatePipe(&childStdin, &parentStdin, &security, 0); err != nil {
		return nil, fmt.Errorf("create sandbox stdin pipe: %w", err)
	}
	var parentStdout, childStdout windows.Handle
	if err := windows.CreatePipe(&parentStdout, &childStdout, &security, 0); err != nil {
		windows.CloseHandle(childStdin)
		windows.CloseHandle(parentStdin)
		return nil, fmt.Errorf("create sandbox stdout pipe: %w", err)
	}
	var parentStderr, childStderr windows.Handle
	if err := windows.CreatePipe(&parentStderr, &childStderr, &security, 0); err != nil {
		windows.CloseHandle(childStdin)
		windows.CloseHandle(parentStdin)
		windows.CloseHandle(parentStdout)
		windows.CloseHandle(childStdout)
		return nil, fmt.Errorf("create sandbox stderr pipe: %w", err)
	}
	for _, handle := range []windows.Handle{parentStdin, parentStdout, parentStderr} {
		if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
			for _, h := range []windows.Handle{childStdin, parentStdin, parentStdout, childStdout, parentStderr, childStderr} {
				windows.CloseHandle(h)
			}
			return nil, fmt.Errorf("protect parent sandbox pipe handle: %w", err)
		}
	}
	return &windowsSandboxPipes{
		childStdin:   childStdin,
		childStdout:  childStdout,
		childStderr:  childStderr,
		parentStdin:  os.NewFile(uintptr(parentStdin), "sandbox-stdin"),
		parentStdout: os.NewFile(uintptr(parentStdout), "sandbox-stdout"),
		parentStderr: os.NewFile(uintptr(parentStderr), "sandbox-stderr"),
	}, nil
}

func (p *windowsSandboxPipes) closeChildEnds() {
	if p.childStdin != 0 {
		_ = windows.CloseHandle(p.childStdin)
		p.childStdin = 0
	}
	if p.childStdout != 0 {
		_ = windows.CloseHandle(p.childStdout)
		p.childStdout = 0
	}
	if p.childStderr != 0 {
		_ = windows.CloseHandle(p.childStderr)
		p.childStderr = 0
	}
}

func (p *windowsSandboxPipes) closeAll() {
	p.closeChildEnds()
	if p.parentStdin != nil {
		_ = p.parentStdin.Close()
	}
	if p.parentStdout != nil {
		_ = p.parentStdout.Close()
	}
	if p.parentStderr != nil {
		_ = p.parentStderr.Close()
	}
}

func windowsCanonicalDirectory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("directory is required")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", resolved)
	}
	return resolved, nil
}

func windowsFinalPathForHandle(handle windows.Handle) (string, error) {
	bufferSize := uint32(512)
	for {
		buffer := make([]uint16, bufferSize)
		length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], bufferSize, 0)
		if err != nil {
			return "", err
		}
		if length < bufferSize {
			value := windows.UTF16ToString(buffer[:length])
			switch {
			case strings.HasPrefix(value, `\\?\UNC\`):
				value = `\\` + strings.TrimPrefix(value, `\\?\UNC\`)
			case strings.HasPrefix(value, `\\?\`):
				value = strings.TrimPrefix(value, `\\?\`)
			}
			return filepath.Clean(value), nil
		}
		if length > 32768 {
			return "", fmt.Errorf("resolved Windows path is unexpectedly large")
		}
		bufferSize = length + 1
	}
}

func stageWindowsReadOnlyWorkspace(source, destination string) error {
	source, err := windowsCanonicalDirectory(source)
	if err != nil {
		return err
	}
	files := 0
	var bytes int64
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		files++
		if files > maxWindowsStagedWorkspaceFiles {
			return fmt.Errorf("workspace staging exceeds %d entries", maxWindowsStagedWorkspaceFiles)
		}
		pathPtr, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(pathPtr)
		if err != nil {
			return fmt.Errorf("inspect workspace entry %q: %w", path, err)
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("workspace staging rejects reparse point %q", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if !windowsPathWithin(destination, target) {
			return fmt.Errorf("workspace staging path escapes destination")
		}
		if entry.IsDir() {
			return os.Mkdir(target, 0o700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace staging rejects special file %q", path)
		}
		if info.Size() < 0 || bytes+info.Size() > maxWindowsStagedWorkspaceBytes {
			return fmt.Errorf("workspace staging exceeds %d bytes", maxWindowsStagedWorkspaceBytes)
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		openedPath, err := windowsFinalPathForHandle(windows.Handle(sourceFile.Fd()))
		if err != nil {
			_ = sourceFile.Close()
			return fmt.Errorf("resolve opened workspace file %q: %w", path, err)
		}
		if !windowsPathWithin(source, openedPath) {
			_ = sourceFile.Close()
			return fmt.Errorf("workspace staging opened file %q outside canonical workspace root", path)
		}
		handleInfo := windows.ByHandleFileInformation{}
		if err := windows.GetFileInformationByHandle(windows.Handle(sourceFile.Fd()), &handleInfo); err != nil {
			_ = sourceFile.Close()
			return fmt.Errorf("inspect workspace file handle %q: %w", path, err)
		}
		if handleInfo.NumberOfLinks > 1 {
			_ = sourceFile.Close()
			return fmt.Errorf("workspace staging rejects multiply-linked file %q", path)
		}

		targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = sourceFile.Close()
			return err
		}
		remaining := maxWindowsStagedWorkspaceBytes - bytes
		copied, copyErr := io.Copy(targetFile, io.LimitReader(sourceFile, remaining+1))
		targetCloseErr := targetFile.Close()
		sourceCloseErr := sourceFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if targetCloseErr != nil {
			return targetCloseErr
		}
		if sourceCloseErr != nil {
			return sourceCloseErr
		}
		if copied > remaining {
			return fmt.Errorf("workspace staging exceeds %d bytes", maxWindowsStagedWorkspaceBytes)
		}
		bytes += copied
		return nil
	})
}

func windowsPathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

type windowsBoundedOutput struct {
	mu        sync.Mutex
	buffer    []byte
	limit     int64
	truncated bool
}

func newWindowsBoundedOutput(limit int64) *windowsBoundedOutput {
	return &windowsBoundedOutput{limit: limit}
}

func (b *windowsBoundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - int64(len(b.buffer))
	if remaining > 0 {
		write := int64(len(p))
		if write > remaining {
			write = remaining
		}
		b.buffer = append(b.buffer, p[:write]...)
	}
	if int64(len(p)) > remaining {
		b.truncated = true
	}
	return len(p), nil
}

func (b *windowsBoundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.buffer...))
}

func (b *windowsBoundedOutput) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
