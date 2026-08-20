//go:build windows

package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const windowsWorkspaceShareMode = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

func openWindowsWorkspaceDirectory(path string) (windows.Handle, windows.ByHandleFileInformation, string, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES|windows.FILE_TRAVERSE|windows.SYNCHRONIZE,
		windowsWorkspaceShareMode,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", fmt.Errorf("workspace path is not a directory")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", fmt.Errorf("workspace directory is a reparse point")
	}
	finalPath, err := windowsFinalPathForHandle(handle)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return windows.InvalidHandle, windows.ByHandleFileInformation{}, "", err
	}
	return handle, info, finalPath, nil
}

func windowsWorkspaceIdentity(info windows.ByHandleFileInformation) string {
	return fmt.Sprintf("windows:%08x:%08x%08x", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow)
}

func captureWorkspaceRootIdentity(path string) (string, error) {
	handle, info, _, err := openWindowsWorkspaceDirectory(path)
	if err != nil {
		return "", fmt.Errorf("open workspace root identity: %w", err)
	}
	defer windows.CloseHandle(handle)
	// The persisted security boundary is the opened filesystem object identity,
	// not the textual spelling of the path. Windows may expose the same object
	// through an 8.3 alias, a long path, or another case-insensitive DOS spelling.
	// openWindowsWorkspaceDirectory already rejects root reparse points before
	// this identity is accepted.
	return windowsWorkspaceIdentity(info), nil
}

func workspaceRootIdentityRequired() bool { return true }

func windowsWorkspaceComparablePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return filepath.Clean(`\\` + strings.TrimPrefix(path, `\\?\UNC\`))
	}
	if strings.HasPrefix(path, `\\?\`) {
		return filepath.Clean(strings.TrimPrefix(path, `\\?\`))
	}
	return path
}

func windowsWorkspacePathEqual(a, b string) bool {
	a = windowsWorkspaceComparablePath(a)
	b = windowsWorkspaceComparablePath(b)
	return strings.EqualFold(a, b)
}

func windowsWorkspacePathWithin(root, candidate string) bool {
	root = windowsWorkspaceComparablePath(root)
	candidate = windowsWorkspaceComparablePath(candidate)
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
