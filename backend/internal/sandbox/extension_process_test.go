package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestExtensionSandboxModeParsing(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  ExtensionSandboxMode
	}{
		{"", ExtensionSandboxAuto},
		{"AUTO", ExtensionSandboxAuto},
		{"required", ExtensionSandboxRequired},
		{"off", ExtensionSandboxOff},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", tc.value)
			got, err := extensionSandboxModeFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("mode = %q, want %q", got, tc.want)
			}
		})
	}

	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "sometimes")
	if _, err := extensionSandboxModeFromEnvironment(); err == nil {
		t.Fatal("expected invalid extension sandbox mode to fail")
	}
}

func TestExtensionAutoCompatibilityKeepsExplicitConfiguredEnvironment(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "auto")
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", "")

	cmd, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: "echo",
		Env: map[string]string{
			"GITHUB_TOKEN": "explicit-configured-token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := environmentMap(cmd.Env)
	if got := env["GITHUB_TOKEN"]; got != "explicit-configured-token" {
		t.Fatalf("explicit configured token = %q", got)
	}
}

func TestExtensionRequiredFailsClosedWithoutNativeConfiguration(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", "")

	_, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{Command: "echo"})
	if err == nil {
		t.Fatal("expected required extension confinement to fail without native configuration")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("required-mode error = %v", err)
	}
}

func TestExtensionOffUsesSanitizedHostBoundary(t *testing.T) {
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "off")
	t.Setenv("OMNILLM_MASTER_KEY", "ambient-secret")

	cmd, err := NewHostCommandRunner().CommandContext(context.Background(), ProcessSpec{
		Command: "echo",
		Env:     map[string]string{"EXPLICIT_VALUE": "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := environmentMap(cmd.Env)
	if _, ok := env["OMNILLM_MASTER_KEY"]; ok {
		t.Fatal("off compatibility mode leaked ambient backend secret")
	}
	if env["EXPLICIT_VALUE"] != "ok" {
		t.Fatalf("explicit value = %q", env["EXPLICIT_VALUE"])
	}
}
