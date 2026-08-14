//go:build !linux && !windows && !darwin

package sandbox

import "fmt"

// NewLocalRuntime is implemented by an OS-enforced runtime on supported
// platforms. Unsupported builds fail closed instead of silently executing an
// unrestricted host process.
func NewLocalRuntime(LocalRuntimeConfig) (Runtime, error) {
	return nil, fmt.Errorf("first-party local sandbox runtime is not implemented on this platform")
}
