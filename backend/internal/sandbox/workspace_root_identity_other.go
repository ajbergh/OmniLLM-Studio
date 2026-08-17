//go:build !linux && !darwin

package sandbox

// Platforms without a proven native workspace-root identity primitive retain
// the existing canonical-path contract until their own implementation is added.
func captureWorkspaceRootIdentity(string) (string, error) { return "", nil }

func workspaceRootIdentityRequired() bool { return false }
