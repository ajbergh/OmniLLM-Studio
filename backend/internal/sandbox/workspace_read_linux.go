//go:build linux

package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// readWorkspaceRegularFile opens every requested path component relative to an
// already-open directory descriptor. O_NOFOLLOW on each lookup preserves the
// workspace policy that symlinks are never traversed and prevents a concurrent
// rename/symlink swap from redirecting the final read outside the granted tree.
func readWorkspaceRegularFile(root, relativePath string, maxBytes int64) ([]byte, string, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, "", err
	}
	if maxBytes <= 0 {
		return nil, "", fmt.Errorf("workspace read limit must be positive")
	}

	rootFD, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace root: %w", err)
	}
	defer unix.Close(rootFD)
	if err := requireLinuxDirectoryFD(rootFD); err != nil {
		return nil, "", fmt.Errorf("validate workspace root: %w", err)
	}

	parts := strings.Split(clean, "/")
	parentFD := rootFD
	ownedParentFD := -1
	defer func() {
		if ownedParentFD >= 0 {
			_ = unix.Close(ownedParentFD)
		}
	}()

	for _, part := range parts[:len(parts)-1] {
		nextFD, openErr := unix.Openat(parentFD, part, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			return nil, "", fmt.Errorf("open workspace parent: %w", openErr)
		}
		if err := requireLinuxDirectoryFD(nextFD); err != nil {
			_ = unix.Close(nextFD)
			return nil, "", fmt.Errorf("validate workspace parent: %w", err)
		}
		if ownedParentFD >= 0 {
			_ = unix.Close(ownedParentFD)
		}
		ownedParentFD = nextFD
		parentFD = nextFD
	}

	fileFD, err := unix.Openat(parentFD, parts[len(parts)-1], unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace file: %w", err)
	}
	file := os.NewFile(uintptr(fileFD), parts[len(parts)-1])
	if file == nil {
		_ = unix.Close(fileFD)
		return nil, "", fmt.Errorf("open workspace file handle")
	}
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(fileFD, &stat); err != nil {
		return nil, "", fmt.Errorf("stat workspace file handle: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, "", fmt.Errorf("workspace path is not a regular file")
	}
	if stat.Size > maxBytes {
		return nil, "", fmt.Errorf("workspace file exceeds %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read workspace file handle: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("workspace file exceeds %d bytes", maxBytes)
	}
	hash := sha256.Sum256(data)
	return data, hex.EncodeToString(hash[:]), nil
}

func requireLinuxDirectoryFD(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("path component is not a directory")
	}
	return nil
}
