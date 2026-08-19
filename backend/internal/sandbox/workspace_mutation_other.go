//go:build !linux && !darwin && !windows

package sandbox

import (
	"os"
	"path/filepath"
)

type pathnameWorkspaceMutationTarget struct {
	path string
}

func openWorkspaceMutationTarget(root, relativePath string) (workspaceMutationTarget, error) {
	clean, err := cleanWorkspaceRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	return &pathnameWorkspaceMutationTarget{path: filepath.Join(root, filepath.FromSlash(clean))}, nil
}

func (t *pathnameWorkspaceMutationTarget) Capture() (FileState, error) { return CaptureFileState(t.path) }
func (t *pathnameWorkspaceMutationTarget) Write(data []byte, mode os.FileMode) error { return atomicWriteFile(t.path, data, mode) }
func (t *pathnameWorkspaceMutationTarget) Delete() error { return os.Remove(t.path) }
func (t *pathnameWorkspaceMutationTarget) Restore(state FileState) error { return restoreFileState(t.path, state) }
func (t *pathnameWorkspaceMutationTarget) Close() error { return nil }
