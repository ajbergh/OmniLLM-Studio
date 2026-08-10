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
