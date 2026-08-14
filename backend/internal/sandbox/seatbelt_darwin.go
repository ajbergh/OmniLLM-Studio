//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const darwinSandboxExecPath = "/usr/bin/sandbox-exec"

// darwinSeatbeltProfile returns the deliberately narrow Phase 13A profile used
// to prove the native Seatbelt boundary before it is wired into Runtime or
// persistent extension composition. This foundation profile permits ordinary
// process startup and host reads, denies network by default, and permits writes
// only below explicitly canonicalized roots. A later phase must narrow file
// reads before advertising filesystem isolation for the first-party runtime.
func darwinSeatbeltProfile(writeRoots []string) (string, error) {
	roots := make([]string, 0, len(writeRoots))
	seen := make(map[string]struct{}, len(writeRoots))
	for _, root := range writeRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("absolute Seatbelt write root: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", fmt.Errorf("resolve Seatbelt write root: %w", err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return "", fmt.Errorf("inspect Seatbelt write root: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("Seatbelt write root must be a directory")
		}
		resolved = filepath.Clean(resolved)
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		roots = append(roots, resolved)
	}
	sort.Strings(roots)

	var profile strings.Builder
	profile.WriteString("(version 1)\n")
	profile.WriteString("(deny default)\n")
	profile.WriteString("(allow process*)\n")
	profile.WriteString("(allow sysctl-read)\n")
	profile.WriteString("(allow mach-lookup)\n")
	profile.WriteString("(allow ipc-posix-shm*)\n")
	profile.WriteString("(allow file-read*)\n")
	profile.WriteString("(allow file-write* (literal \"/dev/null\"))\n")
	for _, root := range roots {
		profile.WriteString("(allow file-write* (subpath ")
		profile.WriteString(strconv.Quote(root))
		profile.WriteString("))\n")
	}
	return profile.String(), nil
}

// darwinSeatbeltCommand constructs a command under the fixed system Seatbelt
// launcher. The path is not caller-controlled and absence of the native
// primitive fails closed.
func darwinSeatbeltCommand(ctx context.Context, profile, command string, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(profile) == "" {
		return nil, fmt.Errorf("Seatbelt profile is required")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("Seatbelt command is required")
	}
	info, err := os.Stat(darwinSandboxExecPath)
	if err != nil {
		return nil, fmt.Errorf("macOS Seatbelt launcher unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return nil, fmt.Errorf("macOS Seatbelt launcher is not executable")
	}

	launcherArgs := []string{"-p", profile, command}
	launcherArgs = append(launcherArgs, args...)
	cmd := exec.CommandContext(ctx, darwinSandboxExecPath, launcherArgs...)
	cmd.Env = SanitizedEnvironment(nil)
	return cmd, nil
}
