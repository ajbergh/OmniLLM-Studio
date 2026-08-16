//go:build !linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
)

// enumerateWorkspaceSearchCandidates preserves the existing pathname-based
// search behavior on non-Linux platforms until native descriptor/identity
// primitives are implemented and proven there.
func enumerateWorkspaceSearchCandidates(root, _ string, maxBytes int64, visit func(string, []byte) bool) error {
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxBytes {
			return nil
		}
		data, _, readErr := readWorkspaceRegularFile(root, rel, maxBytes)
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
