//go:build linux

package sandbox

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

type linuxWorkspaceMutationTarget struct {
	parentFD int
	name     string
}

func openWorkspaceMutationTarget(root, relativePath string) (workspaceMutationTarget, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	rootFD, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	if err := requireLinuxDirectoryFD(rootFD); err != nil {
		_ = unix.Close(rootFD)
		return nil, fmt.Errorf("validate workspace root: %w", err)
	}

	parts := strings.Split(clean, "/")
	parentFD := rootFD
	for _, part := range parts[:len(parts)-1] {
		nextFD, openErr := unix.Openat(parentFD, part, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if parentFD != rootFD {
			_ = unix.Close(parentFD)
		}
		if openErr != nil {
			_ = unix.Close(rootFD)
			return nil, fmt.Errorf("open workspace parent: %w", openErr)
		}
		if err := requireLinuxDirectoryFD(nextFD); err != nil {
			_ = unix.Close(nextFD)
			_ = unix.Close(rootFD)
			return nil, fmt.Errorf("validate workspace parent: %w", err)
		}
		parentFD = nextFD
	}
	if parentFD != rootFD {
		_ = unix.Close(rootFD)
	}
	return &linuxWorkspaceMutationTarget{parentFD: parentFD, name: parts[len(parts)-1]}, nil
}

func (t *linuxWorkspaceMutationTarget) Capture() (FileState, error) {
	fd, err := unix.Openat(t.parentFD, t.name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if errors.Is(err, unix.ENOENT) {
		return FileState{Exists: false, Revertable: true}, nil
	}
	if err != nil {
		return FileState{}, fmt.Errorf("open workspace mutation target: %w", err)
	}
	file := os.NewFile(uintptr(fd), t.name)
	if file == nil {
		_ = unix.Close(fd)
		return FileState{}, fmt.Errorf("open workspace mutation target handle")
	}
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return FileState{}, fmt.Errorf("stat workspace mutation target: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return FileState{Exists: true, Mode: os.FileMode(stat.Mode), Revertable: false}, nil
	}
	if stat.Size > maxRevertSnapshotBytes {
		return FileState{Exists: true, Mode: os.FileMode(stat.Mode & 0o777), Revertable: false}, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRevertSnapshotBytes+1))
	if err != nil {
		return FileState{}, fmt.Errorf("read workspace mutation target: %w", err)
	}
	if len(data) > maxRevertSnapshotBytes {
		return FileState{Exists: true, Mode: os.FileMode(stat.Mode & 0o777), Revertable: false}, nil
	}
	hash := sha256.Sum256(data)
	return FileState{
		Exists:     true,
		SHA256:     hex.EncodeToString(hash[:]),
		Mode:       os.FileMode(stat.Mode & 0o777),
		Content:    data,
		Revertable: true,
	}, nil
}

func (t *linuxWorkspaceMutationTarget) Write(data []byte, mode os.FileMode) error {
	tempName, err := randomWorkspaceTempName()
	if err != nil {
		return err
	}
	fd, err := unix.Openat(t.parentFD, tempName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return fmt.Errorf("create workspace temporary file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = unix.Unlinkat(t.parentFD, tempName, 0)
		}
	}()
	file := os.NewFile(uintptr(fd), tempName)
	if file == nil {
		_ = unix.Close(fd)
		return fmt.Errorf("open workspace temporary file handle")
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := unix.Renameat(t.parentFD, tempName, t.parentFD, t.name); err != nil {
		return fmt.Errorf("replace workspace file: %w", err)
	}
	cleanup = false
	return nil
}

func (t *linuxWorkspaceMutationTarget) Delete() error {
	if err := unix.Unlinkat(t.parentFD, t.name, 0); err != nil {
		return fmt.Errorf("delete workspace file: %w", err)
	}
	return nil
}

func (t *linuxWorkspaceMutationTarget) Restore(state FileState) error {
	if !state.Exists {
		if err := unix.Unlinkat(t.parentFD, t.name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("remove workspace rollback target: %w", err)
		}
		return nil
	}
	if !state.Revertable {
		return fmt.Errorf("file state is not revertable")
	}
	mode := state.Mode.Perm()
	if mode == 0 {
		mode = 0o600
	}
	return t.Write(state.Content, mode)
}

func (t *linuxWorkspaceMutationTarget) Close() error {
	if t.parentFD < 0 {
		return nil
	}
	err := unix.Close(t.parentFD)
	t.parentFD = -1
	return err
}

func randomWorkspaceTempName() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate workspace temporary name: %w", err)
	}
	return ".omnillm-write-" + hex.EncodeToString(random[:]), nil
}
