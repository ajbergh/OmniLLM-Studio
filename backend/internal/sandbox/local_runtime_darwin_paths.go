//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const darwinSandboxPath = "/usr/local/bin:/opt/homebrew/bin:/opt/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

var darwinSystemReadRootCandidates = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/dev",
	"/private/etc",
	"/opt/homebrew",
	"/opt/local",
}

var darwinExecutableRootCandidates = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/opt/homebrew",
	"/opt/local",
}

func validateDarwinSandboxExec() error {
	info, err := os.Stat(darwinSandboxExecPath)
	if err != nil {
		return fmt.Errorf("macOS Seatbelt launcher unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("macOS Seatbelt launcher is not executable")
	}
	return nil
}

func darwinRuntimeReadRoots(workspace, home, tmp string) ([]string, error) {
	roots := []string{workspace, home, tmp}
	for _, candidate := range darwinSystemReadRootCandidates {
		info, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect macOS system read root %q: %w", candidate, err)
		}
		if info.IsDir() {
			roots = append(roots, candidate)
		}
	}
	return roots, nil
}

func darwinSandboxCommandLine(session darwinLocalRuntimeSession, request ExecRequest) (string, []string, error) {
	command := request.Command
	args := append([]string(nil), request.Args...)
	if strings.TrimSpace(request.Language) != "" {
		switch strings.ToLower(strings.TrimSpace(request.Language)) {
		case "python", "python3":
			command = "python3"
			args = []string{"-c", request.Code}
		case "javascript", "js", "node":
			command = "node"
			args = []string{"-e", request.Code}
		case "shell", "sh", "bash":
			command = "/bin/sh"
			args = []string{"-c", request.Code}
		default:
			return "", nil, fmt.Errorf("unsupported sandbox language %q", request.Language)
		}
	}
	resolved, err := darwinResolveExecutable(session.workspace, command)
	if err != nil {
		return "", nil, err
	}
	return resolved, args, nil
}

func darwinResolveExecutable(workspace, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("sandbox command is required")
	}
	var candidates []string
	if filepath.IsAbs(command) {
		candidates = []string{command}
	} else if strings.ContainsRune(command, filepath.Separator) {
		candidates = []string{filepath.Join(workspace, command)}
	} else {
		for _, dir := range strings.Split(darwinSandboxPath, ":") {
			candidates = append(candidates, filepath.Join(dir, command))
		}
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			continue
		}
		if darwinExecutableAllowed(workspace, resolved) {
			return filepath.Clean(resolved), nil
		}
		return "", fmt.Errorf("sandbox command resolves outside approved executable roots")
	}
	return "", fmt.Errorf("sandbox command %q was not found in the fixed runtime path", command)
}

func darwinExecutableAllowed(workspace, executable string) bool {
	if darwinPathWithinRoot(workspace, executable) {
		return true
	}
	for _, candidate := range darwinExecutableRootCandidates {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		if darwinPathWithinRoot(resolved, executable) {
			return true
		}
	}
	return false
}

func darwinSandboxDirectory(workspace, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" || requested == "." {
		return workspace, nil
	}
	if filepath.IsAbs(requested) {
		return "", fmt.Errorf("sandbox working directory must be workspace-relative")
	}
	cleaned := filepath.Clean(requested)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sandbox working directory escapes the workspace")
	}
	candidate := filepath.Join(workspace, cleaned)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve sandbox working directory: %w", err)
	}
	if !darwinPathWithinRoot(workspace, resolved) {
		return "", fmt.Errorf("sandbox working directory escapes the workspace")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("path is not a directory")
		}
		return "", fmt.Errorf("inspect sandbox working directory: %w", err)
	}
	return resolved, nil
}

func darwinSandboxEnvironment(session darwinLocalRuntimeSession, request ExecRequest) ([]string, error) {
	env := map[string]string{
		"PATH":   darwinSandboxPath,
		"HOME":   session.home,
		"TMPDIR": session.tmp,
		"LANG":   "C",
	}
	for key, value := range session.spec.Environment {
		if err := validateDarwinRuntimeEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
		env[key] = value
	}
	for key, value := range request.Env {
		if err := validateDarwinRuntimeEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
		env[key] = value
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
	return out, nil
}

func validateDarwinRuntimeEnvironmentEntry(key, value string) error {
	if err := validateSandboxEnvironmentEntry(key, value); err != nil {
		return err
	}
	upper := strings.ToUpper(strings.TrimSpace(key))
	switch upper {
	case "PATH", "HOME", "TMP", "TMPDIR", "SHELL":
		return fmt.Errorf("macOS sandbox environment key %q is runtime-owned", key)
	}
	if strings.HasPrefix(upper, "DYLD_") {
		return fmt.Errorf("macOS sandbox environment key %q may alter dynamic loading", key)
	}
	return nil
}

func darwinCanonicalDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("workspace source path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("absolute workspace source: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace source: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect workspace source: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace source must be a directory")
	}
	return filepath.Clean(resolved), nil
}

func darwinPathWithinRoot(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
