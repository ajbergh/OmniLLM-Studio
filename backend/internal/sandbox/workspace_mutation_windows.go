//go:build windows

package sandbox

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsWorkspaceMutationTarget struct {
	rootFinal   string
	parent      windows.Handle
	parentFinal string
	name        string
}

func openWorkspaceMutationTarget(root, relativePath string) (workspaceMutationTarget, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	rootHandle, _, rootFinal, err := openWindowsWorkspaceDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	_ = windows.CloseHandle(rootHandle)

	parentRel := filepath.Dir(filepath.FromSlash(clean))
	parentPath := rootFinal
	if parentRel != "." {
		parentPath = filepath.Join(rootFinal, parentRel)
	}
	parent, _, parentFinal, err := openWindowsWorkspaceDirectory(parentPath)
	if err != nil {
		return nil, fmt.Errorf("open workspace mutation parent: %w", err)
	}
	if !windowsWorkspacePathWithin(rootFinal, parentFinal) || !windowsWorkspacePathEqual(parentPath, parentFinal) {
		_ = windows.CloseHandle(parent)
		return nil, fmt.Errorf("workspace mutation parent resolved outside its granted path")
	}
	return &windowsWorkspaceMutationTarget{
		rootFinal:   rootFinal,
		parent:      parent,
		parentFinal: parentFinal,
		name:        filepath.Base(filepath.FromSlash(clean)),
	}, nil
}

func (t *windowsWorkspaceMutationTarget) currentParentPath() (string, error) {
	if t.parent == windows.InvalidHandle || t.parent == 0 {
		return "", fmt.Errorf("workspace mutation target is closed")
	}
	path, err := windowsFinalPathForHandle(t.parent)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (t *windowsWorkspaceMutationTarget) openExisting(access uint32) (windows.Handle, *os.File, windows.ByHandleFileInformation, error) {
	parentPath, err := t.currentParentPath()
	if err != nil {
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	targetPath := filepath.Join(parentPath, t.name)
	ptr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	handle, err := windows.CreateFile(
		ptr,
		access|windows.FILE_READ_ATTRIBUTES,
		windowsWorkspaceShareMode,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, fmt.Errorf("workspace mutation target is not a regular non-reparse file")
	}
	finalPath, err := windowsFinalPathForHandle(handle)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	currentParent, err := t.currentParentPath()
	if err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, err
	}
	if !windowsWorkspacePathEqual(filepath.Dir(finalPath), currentParent) || !windowsWorkspacePathEqual(finalPath, filepath.Join(currentParent, t.name)) {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, fmt.Errorf("workspace mutation target resolved outside pinned parent")
	}
	file := os.NewFile(uintptr(handle), t.name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, nil, windows.ByHandleFileInformation{}, fmt.Errorf("open workspace mutation file handle")
	}
	return handle, file, info, nil
}

func (t *windowsWorkspaceMutationTarget) Capture() (FileState, error) {
	_, file, info, err := t.openExisting(windows.GENERIC_READ)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return FileState{Exists: false, Revertable: true}, nil
	}
	if err != nil {
		return FileState{}, fmt.Errorf("open workspace mutation target: %w", err)
	}
	defer file.Close()
	size := int64(uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow))
	if size > maxRevertSnapshotBytes {
		return FileState{Exists: true, Revertable: false}, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRevertSnapshotBytes+1))
	if err != nil {
		return FileState{}, fmt.Errorf("read workspace mutation target: %w", err)
	}
	if len(data) > maxRevertSnapshotBytes {
		return FileState{Exists: true, Revertable: false}, nil
	}
	stat, err := file.Stat()
	if err != nil {
		return FileState{}, err
	}
	hash := sha256.Sum256(data)
	return FileState{
		Exists:     true,
		SHA256:     hex.EncodeToString(hash[:]),
		Mode:       stat.Mode().Perm(),
		Content:    data,
		Revertable: true,
	}, nil
}

func (t *windowsWorkspaceMutationTarget) Write(data []byte, _ os.FileMode) error {
	parentPath, err := t.currentParentPath()
	if err != nil {
		return err
	}
	tempName, err := randomWindowsWorkspaceTempName()
	if err != nil {
		return err
	}
	tempPath := filepath.Join(parentPath, tempName)
	ptr, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE|windows.FILE_READ_ATTRIBUTES,
		windowsWorkspaceShareMode,
		nil,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fmt.Errorf("create workspace temporary file: %w", err)
	}
	file := os.NewFile(uintptr(handle), tempName)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return fmt.Errorf("open workspace temporary file handle")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = file.Close()
			_ = os.Remove(tempPath)
		}
	}()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fmt.Errorf("workspace temporary file is unsafe")
	}
	finalTemp, err := windowsFinalPathForHandle(handle)
	if err != nil {
		return err
	}
	currentParent, err := t.currentParentPath()
	if err != nil {
		return err
	}
	if !windowsWorkspacePathEqual(filepath.Dir(finalTemp), currentParent) {
		return fmt.Errorf("workspace temporary file escaped pinned parent")
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := renameWindowsWorkspaceHandle(handle, t.parent, t.name, true); err != nil {
		return fmt.Errorf("replace workspace file: %w", err)
	}
	cleanup = false
	return file.Close()
}

func (t *windowsWorkspaceMutationTarget) Delete() error {
	handle, file, _, err := t.openExisting(windows.DELETE | windows.GENERIC_READ)
	if err != nil {
		return fmt.Errorf("open workspace delete target: %w", err)
	}
	defer file.Close()
	deleteInfo := []byte{1}
	if err := windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteInfo[0], uint32(len(deleteInfo))); err != nil {
		return fmt.Errorf("mark workspace file for deletion: %w", err)
	}
	return nil
}

func (t *windowsWorkspaceMutationTarget) Restore(state FileState) error {
	if !state.Exists {
		if err := t.Delete(); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return err
		}
		return nil
	}
	if !state.Revertable {
		return fmt.Errorf("file state is not revertable")
	}
	return t.Write(state.Content, state.Mode)
}

func (t *windowsWorkspaceMutationTarget) Close() error {
	if t.parent == windows.InvalidHandle || t.parent == 0 {
		return nil
	}
	err := windows.CloseHandle(t.parent)
	t.parent = windows.InvalidHandle
	return err
}

func randomWindowsWorkspaceTempName() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate workspace temporary name: %w", err)
	}
	return ".omnillm-write-" + hex.EncodeToString(random[:]), nil
}

// renameWindowsWorkspaceHandle uses FILE_RENAME_INFORMATION with a pinned
// parent directory handle. NtSetInformationFile preserves RootDirectory-relative
// resolution against the opened directory object instead of reopening a mutable
// parent pathname between validation and replacement.
func renameWindowsWorkspaceHandle(handle, parent windows.Handle, name string, replace bool) error {
	encoded, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	encoded = encoded[:len(encoded)-1]
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	rootOffset := (1 + ptrSize - 1) & ^(ptrSize - 1)
	nameLengthOffset := rootOffset + ptrSize
	nameOffset := nameLengthOffset + 4
	buffer := make([]byte, nameOffset+len(encoded)*2)
	if replace {
		buffer[0] = 1
	}
	*(*windows.Handle)(unsafe.Pointer(&buffer[rootOffset])) = parent
	*(*uint32)(unsafe.Pointer(&buffer[nameLengthOffset])) = uint32(len(encoded) * 2)
	for i, value := range encoded {
		*(*uint16)(unsafe.Pointer(&buffer[nameOffset+i*2])) = value
	}
	var ioStatus windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(handle, &ioStatus, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation)
}
