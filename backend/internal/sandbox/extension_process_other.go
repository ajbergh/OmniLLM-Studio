//go:build !linux

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func platformExtensionCommandContext(ctx context.Context, spec ProcessSpec, mode ExtensionSandboxMode) (*exec.Cmd, error) {
	if mode == ExtensionSandboxRequired {
		return nil, fmt.Errorf("persistent extension sandbox is required but no native %s confinement backend is implemented", runtime.GOOS)
	}
	// Windows/macOS remain on the existing sanitized subprocess boundary until
	// their dedicated native confinement roadmap phases land. Auto mode never
	// overclaims isolation and preserves explicitly configured extension env;
	// Required is the operator fail-closed switch.
	return (HostCommandRunner{}).CommandContext(ctx, spec)
}
