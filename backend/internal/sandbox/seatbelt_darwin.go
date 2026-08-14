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
// only below explicitly canonicalized roots. The Phase 13B runtime uses
// darwinSeatbeltRuntimeProfile instead so host-wide reads are not granted.
func darwinSeatbeltProfile(writeRoots []string) (string, error) {
	writes, err := darwinCanonicalSeatbeltDirectories("write", writeRoots)
	if err != nil {
		return "", err
	}
	var profile strings.Builder
	darwinWriteSeatbeltBase(&profile)
	profile.WriteString("(allow file-read*)\n")
	darwinWriteSeatbeltWrites(&profile, writes)
	return profile.String(), nil
}

// darwinSeatbeltRuntimeProfile builds the Phase 13B runtime profile. Unlike the
// Phase 13A primitive proof, file reads are allowed only beneath explicit,
// canonicalized system/session roots and writes only beneath explicit session
// roots. Network remains denied because no network operation is granted.
func darwinSeatbeltRuntimeProfile(readRoots, writeRoots []string) (string, error) {
	reads, err := darwinCanonicalSeatbeltDirectories("read", readRoots)
	if err != nil {
		return "", err
	}
	writes, err := darwinCanonicalSeatbeltDirectories("write", writeRoots)
	if err != nil {
		return "", err
	}
	if len(reads) == 0 {
		return "", fmt.Errorf("Seatbelt runtime requires at least one read root")
	}

	var profile strings.Builder
	darwinWriteSeatbeltBase(&profile)
	for _, root := range reads {
		profile.WriteString("(allow file-read* (subpath ")
		profile.WriteString(strconv.Quote(root))
		profile.WriteString("))\n")
	}
	darwinWriteSeatbeltWrites(&profile, writes)
	return profile.String(), nil
}

func darwinCanonicalSeatbeltDirectories(kind string, roots []string) ([]string, error) {
	out := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("absolute Seatbelt %s root: %w", kind, err)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, fmt.Errorf("resolve Seatbelt %s root: %w", kind, err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("inspect Seatbelt %s root: %w", kind, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("Seatbelt %s root must be a directory", kind)
		}
		resolved = filepath.Clean(resolved)
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		out = append(out, resolved)
	}
	sort.Strings(out)
	return out, nil
}

func darwinWriteSeatbeltBase(profile *strings.Builder) {
	profile.WriteString("(version 1)\n")
	profile.WriteString("(deny default)\n")
	profile.WriteString("(allow process*)\n")
	profile.WriteString("(allow sysctl-read)\n")
	profile.WriteString("(allow mach-lookup)\n")
	profile.WriteString("(allow ipc-posix-shm*)\n")
}

func darwinWriteSeatbeltWrites(profile *strings.Builder, roots []string) {
	profile.WriteString("(allow file-write* (literal \"/dev/null\"))\n")
	for _, root := range roots {
		profile.WriteString("(allow file-write* (subpath ")
		profile.WriteString(strconv.Quote(root))
		profile.WriteString("))\n")
	}
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
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("macOS Seatbelt launcher is not executable")
	}

	launcherArgs := []string{"-p", profile, command}
	launcherArgs = append(launcherArgs, args...)
	cmd := exec.CommandContext(ctx, darwinSandboxExecPath, launcherArgs...)
	cmd.Env = SanitizedEnvironment(nil)
	return cmd, nil
}
