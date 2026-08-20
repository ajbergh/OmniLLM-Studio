//go:build windows

package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// readWorkspaceRegularFile opens the requested Windows file without following
// a final reparse point and validates the opened object path before consuming
// bytes. Parent junction/symlink redirection therefore cannot return bytes from
// outside the granted root even when pathname entries change concurrently.
func readWorkspaceRegularFile(root, relativePath string, maxBytes int64) ([]byte, string, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, "", err
	}
	if maxBytes <= 0 {
		return nil, "", fmt.Errorf("workspace read limit must be positive")
	}
	rootHandle, _, rootFinal, err := openWindowsWorkspaceDirectory(root)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace root: %w", err)
	}
	defer windows.CloseHandle(rootHandle)

	targetPath := filepath.Join(rootFinal, filepath.FromSlash(clean))
	ptr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return nil, "", err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ|windows.FILE_READ_ATTRIBUTES,
		windowsWorkspaceShareMode,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, "", fmt.Errorf("open workspace file: %w", err)
	}
	file := os.NewFile(uintptr(handle), filepath.Base(targetPath))
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, "", fmt.Errorf("open workspace file handle")
	}
	defer file.Close()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, "", fmt.Errorf("stat workspace file handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, "", fmt.Errorf("workspace path is not a regular non-reparse file")
	}
	finalPath, err := windowsFinalPathForHandle(handle)
	if err != nil {
		return nil, "", fmt.Errorf("resolve workspace file handle: %w", err)
	}
	if !windowsWorkspacePathWithin(rootFinal, finalPath) || !windowsWorkspacePathEqual(targetPath, finalPath) {
		return nil, "", fmt.Errorf("workspace file resolved outside its granted path")
	}
	size := int64(uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow))
	if size > maxBytes {
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
