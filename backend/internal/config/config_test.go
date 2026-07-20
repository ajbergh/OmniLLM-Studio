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
