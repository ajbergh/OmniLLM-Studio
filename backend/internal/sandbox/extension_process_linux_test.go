//go:build linux

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxExtensionRunnerBuildsNoNetworkBubblewrapCommand(t *testing.T) {
	rootFS := t.TempDir()
	commandDir := t.TempDir()
	commandPath := filepath.Join(commandDir, "plugin-helper")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", rootFS)
	t.Setenv("OMNILLM_SANDBOX_BWRAP", "/bin/echo")

	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: commandPath,
		Args:    []string{"--stdio"},
		Env:     map[string]string{"EXPLICIT_VALUE": "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := requireExecCommand(t, process)
	joined := strings.Join(cmd.Args, " ")
	for _, expected := range []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-all",
		"--ro-bind " + rootFS + " /",
		"--ro-bind " + commandDir + " /extension",
		"--chdir /extension",
		"--clearenv",
		"--setenv EXPLICIT_VALUE ok",
		"-- /extension/plugin-helper --stdio",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("Bubblewrap argv %q does not contain %q", joined, expected)
		}
	}
	if strings.Contains(joined, "--share-net") {
		t.Fatalf("persistent extension unexpectedly shares host network: %q", joined)
	}
}

func TestLinuxExtensionConfinementRejectsSensitiveEnvironmentByDefault(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", t.TempDir())
	t.Setenv("OMNILLM_SANDBOX_BWRAP", "/bin/echo")

	_, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: "echo",
		Env:     map[string]string{"GITHUB_TOKEN": "must-not-enter-extension-sandbox"},
	})
	if err == nil || !strings.Contains(err.Error(), "credential-sensitive") {
		t.Fatalf("sensitive extension environment error = %v", err)
	}
}

func TestLinuxExtensionConfinementAllowsExplicitSecretOverride(t *testing.T) {
	rootFS := t.TempDir()
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", rootFS)
	t.Setenv("OMNILLM_SANDBOX_BWRAP", "/bin/echo")
	t.Setenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV", "true")

	process, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: "echo",
		Env:     map[string]string{"GITHUB_TOKEN": "operator-approved"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := requireExecCommand(t, process)
	if joined := strings.Join(cmd.Args, " "); !strings.Contains(joined, "--setenv GITHUB_TOKEN operator-approved") {
		t.Fatalf("approved secret not forwarded to confined extension: %q", joined)
	}
}
