package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestSanitizedEnvironmentDropsAmbientSecrets(t *testing.T) {
	t.Setenv("OMNILLM_MASTER_KEY", "must-not-leak")
	t.Setenv("GITHUB_TOKEN", "must-not-leak")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/must-not-leak")
	t.Setenv("PATH", "/safe/test/path")

	env := environmentMap(SanitizedEnvironment(map[string]string{
		"EXPLICIT_TEST_VALUE": "allowed",
	}))

	for _, key := range []string{"OMNILLM_MASTER_KEY", "GITHUB_TOKEN", "SSH_AUTH_SOCK"} {
		if value, ok := env[key]; ok {
			t.Fatalf("ambient secret %s leaked with value %q", key, value)
		}
	}
	if got := env["EXPLICIT_TEST_VALUE"]; got != "allowed" {
		t.Fatalf("explicit override = %q, want allowed", got)
	}
	if got := env["PATH"]; got != "/safe/test/path" {
		t.Fatalf("PATH = %q, want preserved allowlisted value", got)
	}
}

func TestHostCommandRunnerRejectsInvalidEnvironment(t *testing.T) {
	runner := NewHostCommandRunner()
	if _, err := runner.CommandContext(context.Background(), ProcessSpec{
		Command: "ignored",
		Env:     map[string]string{"BAD=KEY": "value"},
	}); err == nil {
		t.Fatal("expected invalid environment key to be rejected")
	}
	if _, err := runner.CommandContext(context.Background(), ProcessSpec{
		Command: "ignored",
		Env:     map[string]string{"GOOD_KEY": "bad\x00value"},
	}); err == nil {
		t.Fatal("expected NUL-containing environment value to be rejected")
	}
}

func environmentMap(values []string) map[string]string {
	out := make(map[string]string, len(values))
	for _, value := range values {
		key, entry, ok := strings.Cut(value, "=")
		if ok {
			out[key] = entry
		}
	}
	return out
}
