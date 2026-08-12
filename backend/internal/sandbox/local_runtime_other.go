//go:build !linux

package sandbox

import "fmt"

// NewLocalRuntime is implemented by an OS-enforced runtime on supported
// platforms. Until the Windows/macOS phases land, non-Linux builds fail closed
// instead of silently executing an unrestricted host process.
func NewLocalRuntime(LocalRuntimeConfig) (Runtime, error) {
	return nil, fmt.Errorf("first-party local sandbox runtime is not implemented on this platform")
}
