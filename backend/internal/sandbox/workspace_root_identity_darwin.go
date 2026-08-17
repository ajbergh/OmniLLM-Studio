//go:build darwin

package sandbox

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// captureWorkspaceRootIdentity returns the stable Darwin filesystem identity of
// an already-canonical workspace root. The final root is opened without
// following a symlink and identity is derived from the opened directory handle,
// binding the persisted grant to the directory object rather than pathname text.
func captureWorkspaceRootIdentity(path string) (string, error) {
	rootFD, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", fmt.Errorf("open workspace root identity: %w", err)
	}
	defer unix.Close(rootFD)

	var stat unix.Stat_t
	if err := unix.Fstat(rootFD, &stat); err != nil {
		return "", fmt.Errorf("stat workspace root identity handle: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", fmt.Errorf("workspace root is not a directory")
	}
	return fmt.Sprintf("darwin:%d:%d", uint64(stat.Dev), uint64(stat.Ino)), nil
}

func workspaceRootIdentityRequired() bool { return true }
