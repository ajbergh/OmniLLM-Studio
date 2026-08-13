//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func platformExtensionCommandContext(ctx context.Context, spec ProcessSpec, mode ExtensionSandboxMode) (CommandProcess, error) {
	rootFS := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_ROOTFS"))
	if rootFS == "" {
		if mode == ExtensionSandboxRequired {
			return nil, fmt.Errorf("persistent extension sandbox is required but OMNILLM_SANDBOX_ROOTFS is not configured")
		}
		return (HostCommandRunner{}).CommandContext(ctx, spec)
	}
	if err := validateExtensionEnvironment(spec); err != nil {
		return nil, err
	}
	rootFS, err := canonicalDirectory(rootFS)
	if err != nil {
		return nil, fmt.Errorf("extension sandbox rootfs: %w", err)
	}

	bwrap := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_BWRAP"))
	if bwrap == "" {
		bwrap = "bwrap"
	}
	bwrap, err = exec.LookPath(bwrap)
	if err != nil {
		return nil, fmt.Errorf("persistent extension sandbox requires Bubblewrap: %w", err)
	}
	bwrap, err = filepath.EvalSymlinks(bwrap)
	if err != nil {
		return nil, fmt.Errorf("resolve Bubblewrap: %w", err)
	}
	bwrap, err = filepath.Abs(bwrap)
	if err != nil {
		return nil, fmt.Errorf("absolute Bubblewrap path: %w", err)
	}

	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return nil, fmt.Errorf("process command is required")
	}

	args := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-all",
		"--ro-bind", rootFS, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--dir", "/tmp/home",
	}
	chdir := "/tmp/home"

	if strings.TrimSpace(spec.Dir) != "" {
		directory, err := canonicalDirectory(spec.Dir)
		if err != nil {
			return nil, fmt.Errorf("extension working directory: %w", err)
		}
		args = append(args, "--ro-bind", directory, "/workspace")
		chdir = "/workspace"
	}

	if filepath.IsAbs(command) {
		resolvedCommand, err := filepath.EvalSymlinks(command)
		if err != nil {
			return nil, fmt.Errorf("resolve extension command: %w", err)
		}
		info, err := os.Stat(resolvedCommand)
		if err != nil {
			return nil, fmt.Errorf("inspect extension command: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("extension command must be a regular file")
		}
		commandDir := filepath.Dir(resolvedCommand)
		args = append(args, "--ro-bind", commandDir, "/extension")
		command = filepath.ToSlash(filepath.Join("/extension", filepath.Base(resolvedCommand)))
		if strings.TrimSpace(spec.Dir) == "" {
			chdir = "/extension"
		}
	}

	args = append(args,
		"--chdir", chdir,
		"--clearenv",
		"--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv", "HOME", "/tmp/home",
		"--setenv", "TMPDIR", "/tmp",
	)

	keys := make([]string, 0, len(spec.Env))
	for key := range spec.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--setenv", key, spec.Env[key])
	}
	args = append(args, "--", command)
	args = append(args, spec.Args...)

	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Env = SanitizedEnvironment(nil)
	return wrapExecCommand(cmd), nil
}
