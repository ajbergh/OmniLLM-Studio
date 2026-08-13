// Package sandbox defines the execution boundary used by local untrusted or
// model-directed subprocesses. HostCommandRunner remains the concrete sanitized
// host implementation; the historical NewHostCommandRunner constructor now
// returns the persistent extension policy runner so existing MCP/plugin call
// sites gain platform confinement without lifecycle rewrites.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// ProcessSpec describes one local process launch. Callers provide only the
// command, argv, optional working directory, and explicitly configured
// environment overrides. Ambient parent-process environment is never copied
// wholesale into the child.
type ProcessSpec struct {
	Command string
	Args    []string
	Dir     string
	Env     map[string]string
}

// CommandProcess is the lifecycle surface required by persistent stdio
// extensions. Keeping callers on this small interface lets platform runners
// supply native process-tree confinement without forcing MCP/plugins to depend
// on os/exec internals.
type CommandProcess interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
	Kill() error
}

// CommandRunner constructs a process for a ProcessSpec.
type CommandRunner interface {
	CommandContext(context.Context, ProcessSpec) (CommandProcess, error)
}

// execCommandProcess adapts the ordinary os/exec lifecycle to CommandProcess.
// Platform confinement runners may return their own implementation instead.
type execCommandProcess struct {
	cmd *exec.Cmd
}

func wrapExecCommand(cmd *exec.Cmd) CommandProcess {
	return &execCommandProcess{cmd: cmd}
}

func (p *execCommandProcess) StdinPipe() (io.WriteCloser, error)  { return p.cmd.StdinPipe() }
func (p *execCommandProcess) StdoutPipe() (io.ReadCloser, error) { return p.cmd.StdoutPipe() }
func (p *execCommandProcess) StderrPipe() (io.ReadCloser, error) { return p.cmd.StderrPipe() }
func (p *execCommandProcess) Start() error                      { return p.cmd.Start() }
func (p *execCommandProcess) Wait() error                       { return p.cmd.Wait() }
func (p *execCommandProcess) Kill() error {
	if p.cmd.Process == nil {
		return fmt.Errorf("process has not started")
	}
	return p.cmd.Process.Kill()
}

// HostCommandRunner creates host child processes with a deliberately small
// ambient environment. It is not itself an OS sandbox. Use the constructor for
// extension workloads so platform policy can select confinement first; direct
// zero-value HostCommandRunner use is reserved for explicit compatibility paths
// and tests.
type HostCommandRunner struct{}

// NewHostCommandRunner preserves the historical constructor name used by MCP
// and plugins, but returns the extension policy runner. In auto mode supported
// platforms use native confinement when it is available; explicit off mode
// retains the sanitized host compatibility boundary.
func NewHostCommandRunner() CommandRunner { return NewExtensionCommandRunner() }

// CommandContext constructs a sanitized host child process command.
func (HostCommandRunner) CommandContext(ctx context.Context, spec ProcessSpec) (CommandProcess, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return nil, fmt.Errorf("process command is required")
	}
	for key, value := range spec.Env {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
	}

	cmd := exec.CommandContext(ctx, command, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = SanitizedEnvironment(spec.Env)
	return wrapExecCommand(cmd), nil
}

// SanitizedEnvironment returns the minimal ambient environment required for
// ordinary subprocess compatibility plus explicit caller overrides. Secrets
// and application-specific variables from the OmniLLM backend are not inherited
// merely because they exist in os.Environ().
func SanitizedEnvironment(overrides map[string]string) []string {
	env := make(map[string]string)
	for _, key := range ambientEnvironmentAllowlist() {
		if value, ok := os.LookupEnv(key); ok && !strings.ContainsRune(value, '\x00') {
			env[key] = value
		}
	}
	for key, value := range overrides {
		if validateEnvironmentEntry(key, value) == nil {
			env[key] = value
		}
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func ambientEnvironmentAllowlist() []string {
	if runtime.GOOS == "windows" {
		return []string{
			"PATH",
			"PATHEXT",
			"SYSTEMROOT",
			"WINDIR",
			"COMSPEC",
			"TEMP",
			"TMP",
			"LANG",
			"LC_ALL",
			"LC_CTYPE",
			"TZ",
		}
	}
	return []string{
		"PATH",
		"LANG",
		"LC_ALL",
		"LC_CTYPE",
		"TZ",
		"TMPDIR",
	}
}

func validateEnvironmentEntry(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "=") || strings.ContainsRune(key, '\x00') {
		return fmt.Errorf("invalid process environment key")
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("process environment value for %q contains NUL", key)
	}
	return nil
}
