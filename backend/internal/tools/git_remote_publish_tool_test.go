package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeRemoteBranchPublisher struct {
	result *gitrepo.RemoteBranchPublishResult
}

func (f *fakeRemoteBranchPublisher) PublishBranch(context.Context, string, string, string, string) (*gitrepo.RemoteBranchPublishResult, error) {
	return f.result, nil
}

func TestGitPublishBranchDefaultsToApproval(t *testing.T) {
	tool := &gitRemotePublishBranchTool{service: &fakeRemoteBranchPublisher{}}
	definition := tool.Definition().Normalized()
	if definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || definition.SupportsParallel || definition.Risk != RiskCritical {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitPublishBranchRequiresReviewedStateAndRejectsArbitraryControls(t *testing.T) {
	tool := &gitRemotePublishBranchTool{service: &fakeRemoteBranchPublisher{}}
	head := strings.Repeat("a", 40)
	state := strings.Repeat("b", 64)
	valid := json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","url":"https://example.com/repo.git"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","force":true}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","refspec":"refs/heads/x:refs/heads/y"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"short","expected_remote_state_digest":"` + state + `"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + head + `","expected_remote_state_digest":"short"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitPublishBranchReturnsStructuredResult(t *testing.T) {
	publisher := &fakeRemoteBranchPublisher{result: &gitrepo.RemoteBranchPublishResult{
		Remote: "origin", Repository: "repo", Branch: "feature/test",
		RemoteHead: strings.Repeat("a", 40), Published: true,
	}}
	tool := &gitRemotePublishBranchTool{service: publisher}
	args := json.RawMessage(`{"remote":"origin","expected_branch":"feature/test","expected_head":"` + strings.Repeat("a", 40) + `","expected_remote_state_digest":"` + strings.Repeat("b", 64) + `"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if len(result.Structured) == 0 {
		t.Fatal("expected structured result")
	}
}

func TestRegistryRemoteBranchCreateGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://example.com/repo.git","allow_push":true,"allow_branch_create":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteBranchCreateEnabledEnv, "false")

	withoutCreate := NewRegistry()
	if _, ok := withoutCreate.Get("git_push"); !ok {
		t.Fatal("git_push should remain registered when branch-create gate is off")
	}
	if _, ok := withoutCreate.Get("git_publish_branch"); ok {
		t.Fatal("git_publish_branch registered without branch-create gate")
	}

	t.Setenv(gitrepo.RemoteBranchCreateEnabledEnv, "true")
	withCreate := NewRegistry()
	if _, ok := withCreate.Get("git_publish_branch"); !ok {
		t.Fatal("git_publish_branch not registered when all global gates are enabled")
	}
}
