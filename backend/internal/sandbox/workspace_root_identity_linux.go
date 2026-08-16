//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"syscall"
)

// captureWorkspaceRootIdentity returns the stable Linux filesystem identity of
// an already-canonical workspace root. Device+inode binds the grant to the
// directory object rather than only to the pathname that currently names it.
func captureWorkspaceRootIdentity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat workspace root identity: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root is not a directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return "", fmt.Errorf("workspace root identity is unavailable")
	}
	return fmt.Sprintf("linux:%d:%d", uint64(stat.Dev), uint64(stat.Ino)), nil
}

func workspaceRootIdentityRequired() bool { return true }
