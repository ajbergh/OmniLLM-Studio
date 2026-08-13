//go:build windows

package sandbox

import (
	"context"
	"testing"
)

func TestWindowsExtensionAutoAndRequiredSelectNativeBackend(t *testing.T) {
	for _, mode := range []ExtensionSandboxMode{ExtensionSandboxAuto, ExtensionSandboxRequired} {
		t.Run(string(mode), func(t *testing.T) {
			t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", string(mode))
			process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{Command: "cmd.exe"})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := process.(*windowsExtensionProcess); !ok {
				t.Fatalf("Windows %s mode process type = %T, want *windowsExtensionProcess", mode, process)
			}
		})
	}
}

func TestWindowsExtensionOffSelectsSanitizedHostBoundary(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", string(ExtensionSandboxOff))
	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{Command: "cmd.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := process.(*execCommandProcess); !ok {
		t.Fatalf("Windows off mode process type = %T, want *execCommandProcess", process)
	}
}
