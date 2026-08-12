//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultLocalExecTimeout = 30 * time.Second
	defaultLocalOutputBytes = 1 << 20
)

type localRuntimeSession struct {
	id      string
	spec    CreateRequest
	scratch string
}

// LocalRuntime is the first-party Linux Bubblewrap execution runtime. It uses a
// configured immutable root filesystem rather than bind-mounting the host root,
// so sandboxed workloads do not gain read access to arbitrary host files.
type LocalRuntime struct {
	rootFS      string
	scratchRoot string
	bwrapPath   string

	mu       sync.RWMutex
	sessions map[string]localRuntimeSession
	active   map[string]context.CancelFunc
}

// NewLocalRuntime constructs the Linux Bubblewrap runtime. RootFS and the bwrap
// executable are resolved/canonicalized at startup. The runtime refuses
// workspace mounts and network-enabled sessions until those policies have
// dedicated Broker/runtime implementations.
func NewLocalRuntime(config LocalRuntimeConfig) (Runtime, error) {
	rootFS, err := canonicalDirectory(config.RootFS)
	if err != nil {
		return nil, fmt.Errorf("sandbox rootfs: %w", err)
	}
	bwrap := strings.TrimSpace(config.BwrapPath)
	if bwrap == "" {
		bwrap = "bwrap"
	}
	bwrap, err = exec.LookPath(bwrap)
	if err != nil {
		return nil, fmt.Errorf("find bubblewrap: %w", err)
	}
	bwrap, err = filepath.EvalSymlinks(bwrap)
	if err != nil {
		return nil, fmt.Errorf("resolve bubblewrap: %w", err)
	}
	bwrap, err = filepath.Abs(bwrap)
	if err != nil {
		return nil, fmt.Errorf("absolute bubblewrap path: %w", err)
	}

	scratchRoot := strings.TrimSpace(config.ScratchRoot)
	if scratchRoot == "" {
		scratchRoot = os.TempDir()
	}
	scratchRoot, err = canonicalDirectory(scratchRoot)
	if err != nil {
		return nil, fmt.Errorf("sandbox scratch root: %w", err)
	}

	return &LocalRuntime{
		rootFS:      rootFS,
		scratchRoot: scratchRoot,
		bwrapPath:   bwrap,
		sessions:    make(map[string]localRuntimeSession),
		active:      make(map[string]context.CancelFunc),
	}, nil
}

// Capabilities reports only controls this implementation actually enforces.
func (r *LocalRuntime) Capabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		Name:                 "linux-bubblewrap",
		Version:              "1",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		ProcessTreeIsolation: true,
		MemoryLimit:          false,
		CPULimit:             false,
		PIDLimit:             false,
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
	if len(request.Spec.Mounts) != 0 {
		return "", fmt.Errorf("linux sandbox workspace mounts are not enabled in this runtime revision")
	}
	if request.Spec.Network.Mode != "" && request.Spec.Network.Mode != NetworkNone {
		return "", fmt.Errorf("linux sandbox network access is not enabled in this runtime revision")
	}

	scratch, err := os.MkdirTemp(r.scratchRoot, "omnillm-sandbox-*")
	if err != nil {
		return "", fmt.Errorf("create sandbox scratch directory: %w", err)
	}
	if err := os.Chmod(scratch, 0o700); err != nil {
		_ = os.RemoveAll(scratch)
		return "", fmt.Errorf("protect sandbox scratch directory: %w", err)
	}

	runtimeID := "rt_" + uuid.NewString()
	r.mu.Lock()
	r.sessions[runtimeID] = localRuntimeSession{id: runtimeID, spec: cloneCreateRequest(request.Spec), scratch: scratch}
	r.mu.Unlock()
	return runtimeID, nil
}

func (r *LocalRuntime) Exec(ctx context.Context, runtimeID string, request ExecRequest) (*ExecResult, error) {
	session, err := r.session(runtimeID)
	if err != nil {
		return nil, err
	}
	command, commandArgs, err := sandboxCommand(request)
	if err != nil {
		return nil, err
	}
	directory, err := sandboxDirectory(request.Directory)
	if err != nil {
		return nil, err
	}
	for key, value := range request.Env {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return nil, err
		}

	timeout := defaultLocalExecTimeout
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
	executionID := "exec_" + uuid.NewString()
	activeKey := runtimeID + "\x00" + executionID
	r.mu.Lock()
	r.active[activeKey] = cancel
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.active, activeKey)
		r.mu.Unlock()
	}()

	bwrapArgs := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-all",
		"--ro-bind", r.rootFS, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--dir", "/workspace",
		"--bind", session.scratch, "/workspace",
		"--chdir", directory,
		"--clearenv",
		"--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv", "HOME", "/workspace",
		"--setenv", "TMPDIR", "/tmp",
	}
	environment := cloneStringMap(session.spec.Environment)
	for key, value := range request.Env {
		environment[key] = value
	}
	for key, value := range environment {
		bwrapArgs = append(bwrapArgs, "--setenv", key, value)
	}
	bwrapArgs = append(bwrapArgs, "--", command)
	bwrapArgs = append(bwrapArgs, commandArgs...)

	cmd := exec.CommandContext(execCtx, r.bwrapPath, bwrapArgs...)
	cmd.Env = SanitizedEnvironment(nil)
	if len(request.Stdin) > 0 {
		cmd.Stdin = strings.NewReader(string(request.Stdin))
	}
	stdoutLimit := session.spec.Resources.MaxStdoutBytes
	if stdoutLimit <= 0 {
		stdoutLimit = defaultLocalOutputBytes
	}
	stderrLimit := session.spec.Resources.MaxStderrBytes
	if stderrLimit <= 0 {
		stderrLimit = defaultLocalOutputBytes
	}
	stdout := newBoundedOutput(stdoutLimit)
	stderr := newBoundedOutput(stderrLimit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started).Milliseconds()
	if execCtx.Err() != nil {
		return nil, fmt.Errorf("sandbox execution cancelled or timed out: %w", execCtx.Err())
	}
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("run sandbox command: %w", runErr)
		}
	}
	metadata := map[string]any{
		"runtime":          r.Capabilities().Name,
		"stdout_truncated": stdout.Truncated(),
		"stderr_truncated": stderr.Truncated(),
		"network":          "none",
	}
	return &ExecResult{
		ExecutionID: executionID,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ExitCode:    exitCode,
		DurationMS:  duration,
		Metadata:    metadata,
	}, nil
}

func (r *LocalRuntime) Cancel(_ context.Context, runtimeID, executionID string) error {
	key := runtimeID + "\x00" + executionID
	r.mu.RLock()
	cancel, ok := r.active[key]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("sandbox execution not found")
	}
	cancel()
	return nil
}

func (r *LocalRuntime) Status(_ context.Context, runtimeID string) (*Status, error) {
	if _, err := r.session(runtimeID); err != nil {
		return nil, err
	}
	return &Status{State: "ready", Capabilities: r.Capabilities()}, nil
}

func (r *LocalRuntime) Destroy(_ context.Context, runtimeID string) error {
	r.mu.Lock()
	session, ok := r.sessions[runtimeID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("sandbox runtime session not found")
	}
	for key, cancel := range r.active {
		if strings.HasPrefix(key, runtimeID+"\x00") {
			cancel()
			delete(r.active, key)
		}
	}
	delete(r.sessions, runtimeID)
	r.mu.Unlock()
	if err := os.RemoveAll(session.scratch); err != nil {
		return fmt.Errorf("remove sandbox scratch directory: %w", err)
	}
	return nil
}

func (r *LocalRuntime) session(runtimeID string) (localRuntimeSession, error) {
	r.mu.RLock()
	session, ok := r.sessions[runtimeID]
	r.mu.RUnlock()
	if !ok {
		return localRuntimeSession{}, fmt.Errorf("sandbox runtime session not found")
	}
	return session, nil
}

func sandboxCommand(request ExecRequest) (string, []string, error) {
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
		return request.Command, args, nil
	}
	if strings.TrimSpace(request.Code) == "" {
		return "", nil, fmt.Errorf("sandbox code is required")
	}
	switch strings.ToLower(strings.TrimSpace(request.Language)) {
	case "python":
		return "python3", []string{"-c", request.Code}, nil
	case "javascript":
		return "node", []string{"-e", request.Code}, nil
	case "shell":
		return "sh", []string{"-lc", request.Code}, nil
	default:
		return "", nil, fmt.Errorf("unsupported sandbox language %q", request.Language)
	}
}

func sandboxDirectory(directory string) (string, error) {
	directory = strings.TrimSpace(strings.ReplaceAll(directory, "\\", "/"))
	if directory == "" || directory == "." {
		return "/workspace", nil
	}
	if strings.HasPrefix(directory, "/") {
		return "", fmt.Errorf("sandbox directory must be workspace-relative")
	}
	clean := path.Clean(directory)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("sandbox directory escapes workspace")
	}
	return "/workspace/" + clean, nil
}

func canonicalDirectory(value string) (string, error) {
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

type boundedOutput struct {
	mu        sync.Mutex
	buffer    []byte
	limit     int64
	truncated bool
}

func newBoundedOutput(limit int64) *boundedOutput {
	return &boundedOutput{limit: limit}
}

func (b *boundedOutput) Write(p []byte) (int, error) {
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

func (b *boundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.buffer...))
}

func (b *boundedOutput) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
