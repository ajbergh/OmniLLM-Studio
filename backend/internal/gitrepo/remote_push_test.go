package gitrepo

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
)

func TestRemotePushRequiresAllGlobalGates(t *testing.T) {
	configured := map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowPush: true},
	}

	writable := NewServiceWithWriteAccess(map[string]string{"repo": t.TempDir()}, true)
	readOnly := NewService(map[string]string{"repo": t.TempDir()})

	cases := []struct {
		name        string
		enabled     bool
		pushEnabled bool
		local       *Service
		want        bool
	}{
		{name: "all enabled", enabled: true, pushEnabled: true, local: writable, want: true},
		{name: "remote disabled", enabled: false, pushEnabled: true, local: writable},
		{name: "push disabled", enabled: true, pushEnabled: false, local: writable},
		{name: "local write disabled", enabled: true, pushEnabled: true, local: readOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newRemoteService(configured, tc.enabled, tc.pushEnabled, nil, nil)
			svc.local = tc.local
			if got := svc.PushMutationEnabled(); got != tc.want {
				t.Fatalf("PushMutationEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemoteConfigRequiresAllowPushForDefaultBranchOptIn(t *testing.T) {
	configured := ParseRemoteConfig(`{
		"bad":{"repository":"repo","url":"https://example.com/repo.git","allow_default_branch_push":true},
		"good":{"repository":"repo","url":"https://example.com/repo.git","allow_push":true,"allow_default_branch_push":true}
	}`)
	if _, ok := configured["bad"]; ok {
		t.Fatal("default-branch push opt-in accepted without allow_push")
	}
	if good, ok := configured["good"]; !ok || !good.AllowDefaultBranchPush {
		t.Fatalf("valid default-branch push config rejected: %#v", good)
	}
}

func TestProtectedRemoteBranchFallback(t *testing.T) {
	advertised := &packp.AdvRefs{}
	if !isProtectedRemoteBranch(advertised, "main") {
		t.Fatal("main should be protected when no symref is advertised")
	}
	if !isProtectedRemoteBranch(advertised, "master") {
		t.Fatal("master should be protected when no symref is advertised")
	}
	if isProtectedRemoteBranch(advertised, "feature/test") {
		t.Fatal("feature branch unexpectedly protected by fallback")
	}
}

func TestRemotePushLimitWriterRejectsOversizedPack(t *testing.T) {
	var destination bytes.Buffer
	writer := &remotePushLimitWriter{writer: &destination, remaining: 5}
	n, err := writer.Write([]byte("abcdef"))
	if n != 5 || !errors.Is(err, errRemotePushTooLarge) {
		t.Fatalf("Write() = (%d, %v), want (5, %v)", n, err, errRemotePushTooLarge)
	}
	if destination.String() != "abcde" || writer.written != 5 {
		t.Fatalf("unexpected limited output: %q, written=%d", destination.String(), writer.written)
	}
}

func TestRemotePushLimitWriterAllowsExactLimit(t *testing.T) {
	var destination bytes.Buffer
	writer := &remotePushLimitWriter{writer: &destination, remaining: 5}
	n, err := writer.Write([]byte("abcde"))
	if err != nil || n != 5 {
		t.Fatalf("Write() = (%d, %v), want (5, nil)", n, err)
	}
	if destination.String() != "abcde" {
		t.Fatalf("unexpected output %q", destination.String())
	}
}
