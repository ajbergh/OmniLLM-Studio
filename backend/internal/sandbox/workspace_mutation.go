package sandbox

import "os"

// workspaceMutationTarget keeps one workspace file mutation bound to the same
// parent directory identity for capture, mutation, verification, and rollback.
// Linux uses descriptor-relative operations; other platforms preserve the
// existing pathname behavior until equivalent native hardening is implemented.
type workspaceMutationTarget interface {
	Capture() (FileState, error)
	Write(data []byte, mode os.FileMode) error
	Delete() error
	Restore(state FileState) error
	Close() error
}
