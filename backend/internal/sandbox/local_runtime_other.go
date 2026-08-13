//go:build !linux && !windows

package sandbox

import "fmt"

// NewLocalRuntime is implemented by an OS-enforced runtime on supported
// platforms. Until the macOS phase lands, unsupported non-Linux/non-Windows
// builds fail closed instead of silently executing an unrestricted host process.
func NewLocalRuntime(LocalRuntimeConfig) (Runtime, error) {
	return nil, fmt.Errorf("first-party local sandbox runtime is not implemented on this platform")
}
