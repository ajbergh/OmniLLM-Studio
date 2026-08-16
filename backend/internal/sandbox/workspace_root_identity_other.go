//go:build !linux

package sandbox

// Non-Linux workspace roots retain the existing canonical-path contract until
// a platform-native durable identity primitive is implemented and proven.
func captureWorkspaceRootIdentity(string) (string, error) { return "", nil }

func workspaceRootIdentityRequired() bool { return false }
