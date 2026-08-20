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
	mounts  []RuntimeMount
}

// LocalRuntime is the first-party Linux Bubblewrap execution runtime. It uses a
// configured immutable root filesystem rather than bind-mounting the host root,
// so sandboxed workloads do not gain read access to arbitrary host files.
type LocalRuntime struct {
	rootFS      string
	scratchRoot string
	bwrapPath   string
	cgroup      *linuxCgroupManager

	mu       sync.RWMutex
	sessions map[string]localRuntimeSession
	active   map[string]context.CancelFunc
}

// NewLocalRuntime constructs the Linux Bubblewrap runtime. RootFS and the bwrap
// executable are resolved/canonicalized at startup. A configured cgroup-v2
// subtree advertises only delegated controllers that pass setup, while atomic
// clone-into-cgroup placement is probed before any quota capability is exposed.
// Destination-allowlisted network access remains disabled until an enforceable
// egress layer is present.
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

	cgroup, err := newLinuxCgroupManager(config.CgroupRoot, bwrap)
	if err != nil {
		return nil, fmt.Errorf("sandbox cgroup: %w", err)
	}

	return &LocalRuntime{
		rootFS:      rootFS,
		scratchRoot: scratchRoot,
		bwrapPath:   bwrap,
		cgroup:      cgroup,
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
		NetworkAllowlist:     false,
		ProcessTreeIsolation: true,
		MemoryLimit:          r != nil && r.cgroup != nil && r.cgroup.memoryEnabled,
		CPULimit:             false,
		PIDLimit:             r != nil && r.cgroup != nil && r.cgroup.pidsEnabled,
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
	if request.Spec.Resources.MaxProcesses > 0 && !r.Capabilities().PIDLimit {
		return "", fmt.Errorf("linux sandbox cannot enforce requested process-count quota")
	}
	if request.Spec.Resources.MemoryBytes > 0 && !r.Capabilities().MemoryLimit {
		return "", fmt.Errorf("linux sandbox cannot enforce requested memory quota")
	}
	if request.Spec.Resources.CPUTimeMS > 0 && !r.Capabilities().CPULimit {
		return "", fmt.Errorf("linux sandbox cannot enforce requested CPU quota")
	}
	if request.Spec.Network.Mode != "" && request.Spec.Network.Mode != NetworkNone {
		return "", fmt.Errorf("linux sandbox network access is not enabled in this runtime revision")
	}
	mounts, err := validateRuntimeMounts(request)
	if err != nil {
		return "", err
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
	r.sessions[runtimeID] = localRuntimeSession{
		id:      runtimeID,
		spec:    cloneCreateRequest(request.Spec),
		scratch: scratch,
		mounts:  mounts,
	}
	r.mu.Unlock()

	ttl := defaultSessionTTL
	if request.Spec.TTLSeconds > 0 {
		ttl = time.Duration(request.Spec.TTLSeconds) * time.Second
	}
	time.AfterFunc(ttl, func() {
		_ = r.Destroy(context.Background(), runtimeID)
	})

	return runtimeID, nil
}

func validateRuntimeMounts(request RuntimeCreateRequest) ([]RuntimeMount, error) {
	if len(request.Spec.Mounts) != len(request.ResolvedMounts) {
		return nil, fmt.Errorf("runtime workspace mount resolution mismatch")
	}
	// The current tool contract exposes at most one project workspace per
	// execution. Reject additional mounts instead of inventing ambiguous target
	// paths or widening the model-facing filesystem surface.
	if len(request.ResolvedMounts) > 1 {
		return nil, fmt.Errorf("linux sandbox currently supports at most one workspace mount")
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
		source, err := canonicalDirectory(resolved.SourcePath)
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
	command, commandArgs, err := sandboxCommand(request)
	if err != nil {
		return nil, err
	}
	directory, err := sandboxDirectory(request.Directory)
	if err != nil {
		return nil, err
	}
	for key, value := range request.Env {
		if err := validateSandboxEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
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
	executionID, err := executionIDOrNew(request.ExecutionID)
	if err != nil {
		cancel()
		return nil, err
	}
	activeKey := runtimeID + "\x00" + executionID
	r.mu.Lock()
	if _, exists := r.active[activeKey]; exists {
		r.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("sandbox execution id is already active")
	}
	r.active[activeKey] = cancel
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.active, activeKey)
		r.mu.Unlock()
	}()

	var executionCgroup *linuxExecutionCgroup
	resources := session.spec.Resources
	if resources.MaxProcesses > 0 || resources.MemoryBytes > 0 || resources.CPUTimeMS > 0 {
		if r.cgroup == nil {
			return nil, fmt.Errorf("linux sandbox cgroup quota boundary became unavailable")
		}
		executionCgroup, err = r.cgroup.createExecutionWithCPU(resources.MaxProcesses, resources.MemoryBytes, resources.CPUTimeMS)
		if err != nil {
			return nil, fmt.Errorf("create Linux sandbox execution cgroup: %w", err)
		}
		defer func() {
			_ = executionCgroup.cleanup()
		}()
	}

	bwrapArgs := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-all",
		"--ro-bind", r.rootFS, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--dir", "/tmp/home",
		"--dir", "/workspace",
	}
	mountArgs, workspaceMode, err := runtimeWorkspaceMountArgs(session)
	if err != nil {
		return nil, err
	}
	bwrapArgs = append(bwrapArgs, mountArgs...)
	bwrapArgs = append(bwrapArgs,
		"--chdir", directory,
		"--clearenv",
		"--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv", "HOME", "/tmp/home",
		"--setenv", "TMPDIR", "/tmp",
	)

	environment := cloneStringMap(session.spec.Environment)
	if environment == nil {
		environment = make(map[string]string)
	}
	for key, value := range request.Env {
		environment[key] = value
	}
	for key, value := range environment {
		if err := validateSandboxEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
		bwrapArgs = append(bwrapArgs, "--setenv", key, value)
	}
	bwrapArgs = append(bwrapArgs, "--", command)
	bwrapArgs = append(bwrapArgs, commandArgs...)

	cmd := exec.CommandContext(execCtx, r.bwrapPath, bwrapArgs...)
	if executionCgroup != nil {
		if err := executionCgroup.attach(cmd); err != nil {
			return nil, fmt.Errorf("attach Linux sandbox execution cgroup: %w", err)
		}
	}
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
	outcome, runMonitorErr := runLinuxCommandWithCPUBudget(execCtx, cmd, executionCgroup, resources.CPUTimeMS)
	duration := time.Since(started).Milliseconds()
	if runMonitorErr != nil {
		return nil, fmt.Errorf("enforce Linux sandbox CPU quota: %w", runMonitorErr)
	}
	if execCtx.Err() != nil {
		return nil, fmt.Errorf("sandbox execution cancelled or timed out: %w", execCtx.Err())
	}
	runErr := outcome.runErr
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
		"workspace_mode":   workspaceMode,
	}
	if executionCgroup != nil && resources.MemoryBytes > 0 {
		events, err := executionCgroup.memoryEvents()
		if err != nil {
			return nil, fmt.Errorf("observe Linux sandbox memory quota: %w", err)
		}
		metadata["memory_events"] = events
		metadata["memory_limit_enforced"] = events["oom"] > 0 || events["oom_kill"] > 0
	}
	if executionCgroup != nil && resources.CPUTimeMS > 0 {
		metadata["cpu_limit_enforced"] = true
		metadata["cpu_limit_exceeded"] = outcome.quotaExceeded
		metadata["cpu_time_limit_ms"] = resources.CPUTimeMS
		metadata["cpu_usage_us"] = outcome.usageUS
		if outcome.quotaExceeded {
			metadata["termination_reason"] = "cpu_quota_exceeded"
		}
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

func runtimeWorkspaceMountArgs(session localRuntimeSession) ([]string, string, error) {
	if len(session.mounts) == 0 {
		return []string{"--bind", session.scratch, "/workspace"}, "ephemeral", nil
	}
	if len(session.mounts) != 1 {
		return nil, "", fmt.Errorf("linux sandbox currently supports at most one workspace mount")
	}
	mount := session.mounts[0]
	switch mount.Mode {
	case MountReadOnly:
		return []string{"--ro-bind", mount.SourcePath, "/workspace"}, string(MountReadOnly), nil
	case MountReadWriteNoDelete:
		// POSIX bind mounts cannot enforce write-without-delete semantics. Narrow
		// the grant to read-only rather than silently weakening it.
		return []string{"--ro-bind", mount.SourcePath, "/workspace"}, string(MountReadOnly), nil
	case MountReadWrite:
		args := []string{"--bind", mount.SourcePath, "/workspace"}
		gitPath := filepath.Join(mount.SourcePath, ".git")
		if info, err := os.Lstat(gitPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// A symlinked Git control path is difficult to protect without
				// widening traversal authority. Narrow the entire mount instead.
				return []string{"--ro-bind", mount.SourcePath, "/workspace"}, string(MountReadOnly), nil
			}
			args = append(args, "--ro-bind", gitPath, "/workspace/.git")
		} else if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("inspect workspace Git metadata: %w", err)
		}
		return args, string(MountReadWrite), nil
	default:
		return nil, "", fmt.Errorf("unsupported runtime workspace mode %q", mount.Mode)
	}
}

func (r *LocalRuntime) Cancel(_ context.Context, runtimeID, executionID string) error {
	if err := validateExecutionID(executionID); err != nil {
		return err
	}
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

// Destroy is idempotent so explicit cleanup and TTL expiry may race safely.
func (r *LocalRuntime) Destroy(_ context.Context, runtimeID string) error {
	r.mu.Lock()
	session, ok := r.sessions[runtimeID]
	if !ok {
		r.mu.Unlock()
		return nil
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
		}
		return request.Command, args, nil
	}
	if strings.TrimSpace(request.Code) == "" {
		return "", nil, fmt.Errorf("sandbox code is required")
	}
	switch strings.ToLower(strings.TrimSpace(request.Language)) {
	case "python":
		return "python3", []string{"-I", "-S", "-c", request.Code}, nil
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
