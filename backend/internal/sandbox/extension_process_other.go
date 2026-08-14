//go:build !linux && !windows && !darwin

package sandbox

import (
	"context"
	"fmt"
	"runtime"
)

func platformExtensionCommandContext(ctx context.Context, spec ProcessSpec, mode ExtensionSandboxMode) (CommandProcess, error) {
	if mode == ExtensionSandboxRequired {
		return nil, fmt.Errorf("persistent extension sandbox is required but no native %s confinement backend is implemented", runtime.GOOS)
	}
	// Unsupported platforms remain on the sanitized subprocess boundary in auto
	// mode. Required is the operator fail-closed switch.
	return (HostCommandRunner{}).CommandContext(ctx, spec)
}
