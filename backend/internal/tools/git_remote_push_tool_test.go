package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeRemotePusher struct {
	result *gitrepo.RemotePushResult
}

func (f *fakeRemotePusher) Push(context.Context, string, string, string, string) (*gitrepo.RemotePushResult, error) {
	return f.result, nil
}

func TestGitPushDefaultsToApproval(t *testing.T) {
	tool := &gitRemotePushTool{service: &fakeRemotePusher{}}
	definition := tool.Definition().Normalized()
	if definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || definition.SupportsParallel || definition.Risk != RiskCritical {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitPushRequiresReviewedStateAndRejectsArbitraryControls(t *testing.T) {
	tool := &gitRemotePushTool{service: &fakeRemotePusher{}}
	head := strings.Repeat("a", 40)
	remoteHead := strings.Repeat("b", 40)
	valid := json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","url":"https://example.com/repo.git"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","force":true}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_head":"` + remoteHead + `","refspec":"+refs/heads/main:refs/heads/main"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"short","expected_remote_head":"` + remoteHead + `"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_head":"short"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitPushReturnsStructuredResult(t *testing.T) {
	pusher := &fakeRemotePusher{result: &gitrepo.RemotePushResult{
		Remote: "origin", Repository: "repo", Branch: "feature/test",
		PreviousRemoteHead: strings.Repeat("a", 40), RemoteHead: strings.Repeat("b", 40), Updated: true,
	}}
	tool := &gitRemotePushTool{service: pusher}
	args := json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + strings.Repeat("b", 40) + `","expected_remote_head":"` + strings.Repeat("a", 40) + `"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if len(result.Structured) == 0 {
		t.Fatal("expected structured result")
	}
}

func TestRegistryRemotePushGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://example.com/repo.git","allow_push":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")

	withoutPush := NewRegistry()
	if _, ok := withoutPush.Get("git_fetch"); !ok {
		t.Fatal("git_fetch should remain registered when push gate is off")
	}
	if _, ok := withoutPush.Get("git_push"); ok {
		t.Fatal("git_push registered without remote push gate")
	}

	t.Setenv(gitrepo.RemotePushEnabledEnv, "true")
	withPush := NewRegistry()
	if _, ok := withPush.Get("git_push"); !ok {
		t.Fatal("git_push not registered when all global gates are enabled")
	}
}
