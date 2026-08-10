package gitrepo

import "testing"

func TestParseRepositoryConfig(t *testing.T) {
	got := ParseRepositoryConfig(`omni=C:\src\OmniLLM-Studio; twynn = D:\src\Twynn ; ../bad=/tmp/bad; missing`)
	if got["omni"] != `C:\src\OmniLLM-Studio` {
		t.Fatalf("omni path = %q", got["omni"])
	}
	if got["twynn"] != `D:\src\Twynn` {
		t.Fatalf("twynn path = %q", got["twynn"])
	}
	if _, ok := got["../bad"]; ok {
		t.Fatal("invalid repository ID should be ignored")
	}
}

func TestNewServiceFromEnvironmentWriteGate(t *testing.T) {
	t.Setenv(RepositoriesEnv, "repo=/tmp/repo")
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"", false},
		{"false", false},
		{"not-a-bool", false},
		{"true", true},
		{"TRUE", true},
	} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv(WriteEnabledEnv, test.value)
			if got := NewServiceFromEnvironment().WriteEnabled(); got != test.want {
				t.Fatalf("WriteEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}
