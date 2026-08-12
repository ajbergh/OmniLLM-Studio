// Package sandbox defines the execution boundary used by local untrusted or
// model-directed subprocesses. The initial host runner in this file provides a
// single, sanitized process-construction seam; platform sandbox runtimes replace
// the host runner behind the same higher-level boundary as the sandbox program
// advances.
package sandbox

import (
	"context"
	"fmt"
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

// CommandRunner constructs a process command for a ProcessSpec. The current
// HostCommandRunner is a compatibility implementation with environment
// sanitization; future OS/container sandbox runners plug in at this seam.
type CommandRunner interface {
	CommandContext(context.Context, ProcessSpec) (*exec.Cmd, error)
}

// HostCommandRunner creates host child processes with a deliberately small
// ambient environment. It is not itself an OS sandbox and must not be treated
// as one; its purpose is to remove ambient-secret inheritance while providing a
// common construction boundary for migration to platform sandbox runtimes.
type HostCommandRunner struct{}

// NewHostCommandRunner returns the compatibility host runner.
func NewHostCommandRunner() HostCommandRunner { return HostCommandRunner{} }

// CommandContext constructs a sanitized child process command.
func (HostCommandRunner) CommandContext(ctx context.Context, spec ProcessSpec) (*exec.Cmd, error) {
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
	return cmd, nil
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
