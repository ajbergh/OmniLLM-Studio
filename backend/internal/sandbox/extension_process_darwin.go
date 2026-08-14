//go:build darwin

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// darwinExtensionProcess wraps one persistent stdio extension in a per-process
// Seatbelt profile and scratch root. The profile is installed by the fixed
// system sandbox-exec launcher before extension code begins executing.
type darwinExtensionProcess struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	root string
}

func platformExtensionCommandContext(ctx context.Context, spec ProcessSpec, mode ExtensionSandboxMode) (CommandProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateDarwinSandboxExec(); err != nil {
		if mode == ExtensionSandboxRequired {
			return nil, fmt.Errorf("persistent extension sandbox is required: %w", err)
		}
		return (HostCommandRunner{}).CommandContext(ctx, spec)
	}
	if err := validateExtensionEnvironment(spec); err != nil {
		return nil, err
	}

	command, commandDir, workDir, err := planDarwinExtensionLaunch(spec)
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "omnillm-extension-darwin-")
	if err != nil {
		return nil, fmt.Errorf("create macOS extension scratch root: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(root)
		}
	}()
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve macOS extension scratch root: %w", err)
	}
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	for _, dir := range []string{home, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create macOS extension scratch directory: %w", err)
		}
	}

	readRoots, err := darwinRuntimeReadRoots(commandDir, home, tmp)
	if err != nil {
		return nil, err
	}
	if workDir != "" && !darwinPathWithinRoot(commandDir, workDir) {
		readRoots = append(readRoots, workDir)
	}
	profile, err := darwinSeatbeltRuntimeProfile(readRoots, []string{home, tmp})
	if err != nil {
		return nil, err
	}
	environment, err := darwinExtensionEnvironment(commandDir, home, tmp, spec.Env)
	if err != nil {
		return nil, err
	}

	cmd, err := darwinSeatbeltCommand(ctx, profile, command, spec.Args...)
	if err != nil {
		return nil, err
	}
	cmd.Dir = workDir
	cmd.Env = environment
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

	cleanup = false
	return &darwinExtensionProcess{cmd: cmd, root: root}, nil
}

func planDarwinExtensionLaunch(spec ProcessSpec) (command, commandDir, workDir string, err error) {
	command = strings.TrimSpace(spec.Command)
	if command == "" {
		return "", "", "", fmt.Errorf("process command is required")
	}
	if strings.ContainsRune(command, '\x00') {
		return "", "", "", fmt.Errorf("process command contains NUL")
	}
	for _, arg := range spec.Args {
		if strings.ContainsRune(arg, '\x00') {
			return "", "", "", fmt.Errorf("process argument contains NUL")
		}
	}

	if !filepath.IsAbs(command) && !strings.ContainsRune(command, filepath.Separator) {
		resolved, lookupErr := exec.LookPath(command)
		if lookupErr != nil {
			return "", "", "", fmt.Errorf("resolve extension command %q: %w", command, lookupErr)
		}
		command = resolved
	} else if !filepath.IsAbs(command) {
		base := strings.TrimSpace(spec.Dir)
		if base == "" {
			base, err = os.Getwd()
			if err != nil {
				return "", "", "", fmt.Errorf("resolve extension command base: %w", err)
			}
		}
		command = filepath.Join(base, command)
	}
	command, err = filepath.Abs(command)
	if err != nil {
		return "", "", "", fmt.Errorf("absolute extension command: %w", err)
	}
	command, err = filepath.EvalSymlinks(command)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve extension command: %w", err)
	}
	info, err := os.Stat(command)
	if err != nil {
		return "", "", "", fmt.Errorf("inspect extension command: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", "", "", fmt.Errorf("extension command must be an executable regular file")
	}
	commandDir, err = darwinCanonicalDirectory(filepath.Dir(command))
	if err != nil {
		return "", "", "", fmt.Errorf("resolve extension command directory: %w", err)
	}

	workDir = commandDir
	if strings.TrimSpace(spec.Dir) != "" {
		workDir, err = darwinCanonicalDirectory(spec.Dir)
		if err != nil {
			return "", "", "", fmt.Errorf("extension working directory: %w", err)
		}
	}
	return command, commandDir, workDir, nil
}

func darwinExtensionEnvironment(commandDir, home, tmp string, explicit map[string]string) ([]string, error) {
	values := map[string]string{
		"PATH":   commandDir + string(os.PathListSeparator) + darwinSandboxPath,
		"HOME":   home,
		"TMPDIR": tmp,
		"LANG":   "C",
	}
	for key, value := range explicit {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
		upper := strings.ToUpper(strings.TrimSpace(key))
		switch upper {
		case "PATH", "HOME", "TMP", "TMPDIR", "SHELL":
			return nil, fmt.Errorf("macOS extension environment key %q is confinement-owned", key)
		}
		if strings.HasPrefix(upper, "DYLD_") {
			return nil, fmt.Errorf("macOS extension environment key %q may alter dynamic loading", key)
		}
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out, nil
}

func (p *darwinExtensionProcess) StdinPipe() (io.WriteCloser, error) {
	return p.cmd.StdinPipe()
}

func (p *darwinExtensionProcess) StdoutPipe() (io.ReadCloser, error) {
	return p.cmd.StdoutPipe()
}

func (p *darwinExtensionProcess) StderrPipe() (io.ReadCloser, error) {
	return p.cmd.StderrPipe()
}

func (p *darwinExtensionProcess) Start() error {
	if err := p.cmd.Start(); err != nil {
		p.cleanup()
		return err
	}
	return nil
}

func (p *darwinExtensionProcess) Wait() error {
	err := p.cmd.Wait()
	p.cleanup()
	return err
}

func (p *darwinExtensionProcess) Kill() error {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process has not started")
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (p *darwinExtensionProcess) cleanup() {
	p.mu.Lock()
	root := p.root
	p.root = ""
	p.mu.Unlock()
	if root != "" {
		_ = os.RemoveAll(root)
	}
}
