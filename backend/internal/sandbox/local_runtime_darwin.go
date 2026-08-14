//go:build darwin

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	defaultDarwinLocalExecTimeout = 30 * time.Second
	defaultDarwinLocalOutputBytes = 1 << 20
	darwinDestroyWaitTimeout      = 10 * time.Second
	darwinProcessWaitDelay        = 2 * time.Second
)

type darwinLocalRuntimeSession struct {
	id            string
	spec          CreateRequest
	root          string
	workspace     string
	home          string
	tmp           string
	profile       string
	workspaceMode string
	destroying    bool
}

type darwinActiveExecution struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// LocalRuntime is the first-party macOS Seatbelt execution runtime. Host
// workspace input is copied into a per-session read-only staging tree before a
// model-directed process starts, and the child receives only explicit system and
// session read roots under a default-deny Seatbelt profile.
type LocalRuntime struct {
	mu          sync.RWMutex
	scratchRoot string
	sessions    map[string]darwinLocalRuntimeSession
	active      map[string]darwinActiveExecution
}

// NewLocalRuntime constructs the first-party Darwin runtime around the fixed
// system Seatbelt launcher. Missing native confinement fails closed.
func NewLocalRuntime(config LocalRuntimeConfig) (Runtime, error) {
	if err := validateDarwinSandboxExec(); err != nil {
		return nil, err
	}
	scratchRoot := strings.TrimSpace(config.ScratchRoot)
	if scratchRoot == "" {
		scratchRoot = filepath.Join(os.TempDir(), "omnillm-sandbox")
	}
	absolute, err := filepath.Abs(scratchRoot)
	if err != nil {
		return nil, fmt.Errorf("absolute macOS sandbox scratch root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create macOS sandbox scratch root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve macOS sandbox scratch root: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("path is not a directory")
		}
		return nil, fmt.Errorf("inspect macOS sandbox scratch root: %w", err)
	}
	return &LocalRuntime{
		scratchRoot: filepath.Clean(resolved),
		sessions:    make(map[string]darwinLocalRuntimeSession),
		active:      make(map[string]darwinActiveExecution),
	}, nil
}

// Capabilities reports only controls enforced by this implementation. The
// runtime denies network access and constrains file reads/writes with Seatbelt.
// Process-group cancellation is implemented, but adversarial descendants can
// still create independent process groups, so process-tree isolation remains
// false until Phase 13D proves a stronger boundary.
func (r *LocalRuntime) Capabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		Name:                 "darwin-seatbelt",
		Version:              "1",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		NetworkAllowlist:     false,
		ProcessTreeIsolation: false,
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
	if request.Spec.Network.Mode != "" && request.Spec.Network.Mode != NetworkNone {
		return "", fmt.Errorf("macOS Seatbelt runtime currently supports only network mode %q", NetworkNone)
	}
	for key, value := range request.Spec.Environment {
		if err := validateDarwinRuntimeEnvironmentEntry(key, value); err != nil {
			return "", err
		}
	}
	mounts, err := validateDarwinRuntimeMounts(request)
	if err != nil {
		return "", err
	}

	root, err := os.MkdirTemp(r.scratchRoot, "darwin-")
	if err != nil {
		return "", fmt.Errorf("create macOS sandbox session root: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(root)
		}
	}()
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve macOS sandbox session root: %w", err)
	}
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	for _, dir := range []string{workspace, home, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create macOS sandbox directory %q: %w", dir, err)
		}
	}

	workspaceMode := "ephemeral"
	if len(mounts) == 1 {
		workspaceMode = string(MountReadOnly)
		if err := stageDarwinReadOnlyWorkspace(mounts[0].SourcePath, workspace); err != nil {
			return "", err
		}
	}

	readRoots, err := darwinRuntimeReadRoots(workspace, home, tmp)
	if err != nil {
		return "", err
	}
	writeRoots := []string{home, tmp}
	if len(mounts) == 0 {
		writeRoots = append(writeRoots, workspace)
	}
	profile, err := darwinSeatbeltRuntimeProfile(readRoots, writeRoots)
	if err != nil {
		return "", err
	}

	runtimeID := "rt_" + uuid.NewString()
	session := darwinLocalRuntimeSession{
		id:            runtimeID,
		spec:          cloneCreateRequest(request.Spec),
		root:          root,
		workspace:     workspace,
		home:          home,
		tmp:           tmp,
		profile:       profile,
		workspaceMode: workspaceMode,
	}
	r.mu.Lock()
	r.sessions[runtimeID] = session
	r.mu.Unlock()
	cleanup = false

	ttl := defaultSessionTTL
	if request.Spec.TTLSeconds > 0 {
		ttl = time.Duration(request.Spec.TTLSeconds) * time.Second
	}
	time.AfterFunc(ttl, func() {
		_ = r.Destroy(context.Background(), runtimeID)
	})
	return runtimeID, nil
}

func validateDarwinRuntimeMounts(request RuntimeCreateRequest) ([]RuntimeMount, error) {
	if len(request.Spec.Mounts) != len(request.ResolvedMounts) {
		return nil, fmt.Errorf("runtime workspace mount resolution mismatch")
	}
	if len(request.ResolvedMounts) > 1 {
		return nil, fmt.Errorf("macOS Seatbelt runtime currently supports at most one workspace mount")
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
			return nil, fmt.Errorf("macOS Seatbelt runtime currently accepts only %q workspace mounts", MountReadOnly)
		}
		source, err := darwinCanonicalDirectory(resolved.SourcePath)
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
	if err := validateExecRequest(request); err != nil {
		return nil, err
	}
	for key, value := range request.Env {
		if err := validateDarwinRuntimeEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
	}

	command, args, err := darwinSandboxCommandLine(session, request)
	if err != nil {
		return nil, err
	}
	directory, err := darwinSandboxDirectory(session.workspace, request.Directory)
	if err != nil {
		return nil, err
	}
	environment, err := darwinSandboxEnvironment(session, request)
	if err != nil {
		return nil, err
	}

	timeout := defaultDarwinLocalExecTimeout
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
	r.active[activeKey] = darwinActiveExecution{cancel: cancel, done: done}
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
		stdoutLimit = defaultDarwinLocalOutputBytes
	}
	stderrLimit := session.spec.Resources.MaxStderrBytes
	if stderrLimit <= 0 {
		stderrLimit = defaultDarwinLocalOutputBytes
	}
	stdout := &darwinBoundedOutput{limit: stdoutLimit}
	stderr := &darwinBoundedOutput{limit: stderrLimit}

	cmd, err := darwinSeatbeltCommand(execCtx, session.profile, command, args...)
	if err != nil {
		return nil, err
	}
	cmd.Dir = directory
	cmd.Env = environment
	cmd.Stdin = bytes.NewReader(request.Stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = darwinProcessWaitDelay
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started).Milliseconds()
	if execCtx.Err() != nil {
		return nil, execCtx.Err()
	}
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return nil, fmt.Errorf("run macOS sandbox command: %w", runErr)
		}
		exitCode = exitErr.ExitCode()
	}
	return &ExecResult{
		ExecutionID: executionID,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ExitCode:    exitCode,
		DurationMS:  duration,
		Metadata: map[string]any{
			"runtime":          r.Capabilities().Name,
			"stdout_truncated": stdout.truncated,
			"stderr_truncated": stderr.truncated,
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
	session, err := r.session(runtimeID)
	if err != nil {
		return nil, err
	}
	return &Status{
		State:        "ready",
		Capabilities: r.Capabilities(),
		Metadata: map[string]any{
			"runtime":        r.Capabilities().Name,
			"network":        "none",
			"workspace_mode": session.workspaceMode,
		},
	}, nil
}

// Destroy is idempotent and waits for registered executions to finish their
// process-group cancellation before deleting the per-session staging tree.
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
	active := make([]darwinActiveExecution, 0)
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
		waitCtx, cancel = context.WithTimeout(waitCtx, darwinDestroyWaitTimeout)
		defer cancel()
	}
	for _, execution := range active {
		select {
		case <-execution.done:
		case <-waitCtx.Done():
			return fmt.Errorf("wait for macOS sandbox execution teardown: %w", waitCtx.Err())
		}
	}
	if err := os.RemoveAll(session.root); err != nil {
		return fmt.Errorf("remove macOS sandbox session root: %w", err)
	}
	r.mu.Lock()
	if current, exists := r.sessions[runtimeID]; exists && current.root == session.root {
		delete(r.sessions, runtimeID)
	}
	r.mu.Unlock()
	return nil
}

func (r *LocalRuntime) session(runtimeID string) (darwinLocalRuntimeSession, error) {
	r.mu.RLock()
	session, ok := r.sessions[runtimeID]
	r.mu.RUnlock()
	if !ok {
		return darwinLocalRuntimeSession{}, fmt.Errorf("sandbox runtime session not found")
	}
	if session.destroying {
		return darwinLocalRuntimeSession{}, fmt.Errorf("sandbox runtime session is being destroyed")
	}
	return session, nil
}
