package config

import "testing"

func TestBrowserRuntimeEnabledByDefault(t *testing.T) {
	t.Setenv("OMNILLM_BROWSER_ENABLED", "")

	if cfg := Load(); !cfg.BrowserEnabled {
		t.Fatal("BrowserEnabled = false, want true when OMNILLM_BROWSER_ENABLED is unset")
	}
}

func TestBrowserRuntimeHonorsExplicitSetting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "enabled", value: "true", want: true},
		{name: "enabled case insensitive", value: "TRUE", want: true},
		{name: "disabled", value: "false", want: false},
		{name: "invalid values fail closed", value: "invalid", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OMNILLM_BROWSER_ENABLED", tc.value)
			if got := Load().BrowserEnabled; got != tc.want {
				t.Fatalf("BrowserEnabled = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestSandboxRuntimeUsesNewURLAndPublishesLegacyCompositionAlias(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_URL", "http://127.0.0.1:8090")
	t.Setenv("OMNILLM_CODE_SANDBOX_URL", "")
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "runtime-token")

	cfg := Load()
	if cfg.SandboxURL != "http://127.0.0.1:8090" {
		t.Fatalf("SandboxURL = %q", cfg.SandboxURL)
	}
	if cfg.SandboxToken != "runtime-token" {
		t.Fatalf("SandboxToken = %q", cfg.SandboxToken)
	}
	if got := t.Context(); got == nil {
		t.Fatal("unexpected nil test context")
	}
	if legacy := getenvForTest("OMNILLM_CODE_SANDBOX_URL"); legacy != cfg.SandboxURL {
		t.Fatalf("legacy composition alias = %q, want %q", legacy, cfg.SandboxURL)
	}
}

func TestSandboxRuntimeFallsBackToLegacyURL(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_URL", "")
	t.Setenv("OMNILLM_CODE_SANDBOX_URL", "http://127.0.0.1:8091")
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "runtime-token")

	cfg := Load()
	if cfg.SandboxURL != "http://127.0.0.1:8091" {
		t.Fatalf("SandboxURL = %q", cfg.SandboxURL)
	}
}

func getenvForTest(key string) string {
	// Kept local to this package so tests can verify the transitional
	// composition-root environment alias without exporting config internals.
	return lookupEnvForTest(key)
}
