//go:build windows

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// enumerateWorkspaceSearchCandidates keeps candidate bytes bound to the
// registered Windows workspace object. Enumeration names are treated as
// untrusted hints; every candidate is reopened without following a final
// reparse point and the opened handle must still resolve to the exact expected
// path under the opened root before any bytes are consumed.
func enumerateWorkspaceSearchCandidates(root, expectedRootIdentity string, maxBytes int64, visit func(string, []byte) bool) error {
	if maxBytes <= 0 {
		return fmt.Errorf("workspace search read limit must be positive")
	}
	rootHandle, info, rootFinal, err := openWindowsWorkspaceDirectory(root)
	if err != nil {
		return fmt.Errorf("open workspace search root: %w", err)
	}
	defer windows.CloseHandle(rootHandle)
	if expectedRootIdentity == "" || windowsWorkspaceIdentity(info) != expectedRootIdentity {
		return fmt.Errorf("workspace root identity changed before search")
	}

	err = filepath.WalkDir(rootFinal, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if windowsWorkspacePathEqual(path, rootFinal) {
			return nil
		}
		rel, relErr := filepath.Rel(rootFinal, path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		pathPtr, ptrErr := windows.UTF16PtrFromString(path)
		if ptrErr != nil {
			return nil
		}
		attrs, attrErr := windows.GetFileAttributes(pathPtr)
		if attrErr != nil {
			return nil
		}
		if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			if attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		data, _, readErr := readWorkspaceRegularFile(rootFinal, rel, maxBytes)
		if readErr != nil {
			return nil
		}
		if !visit(rel, data) {
			return filepath.SkipAll
		}
		return nil
	})
	if err == filepath.SkipAll {
		return nil
	}
	return err
}
