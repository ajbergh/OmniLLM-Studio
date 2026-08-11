package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestCreator struct {
	result *gitrepo.GitHubPullRequestResult
}

func (f *fakeGitHubPullRequestCreator) CreateDraftPullRequest(context.Context, string, string, string, string, string, string) (*gitrepo.GitHubPullRequestResult, error) {
	return f.result, nil
}

func TestGitHubDraftPullRequestDefaultsToApproval(t *testing.T) {
	tool := &githubDraftPullRequestTool{service: &fakeGitHubPullRequestCreator{}}
	definition := tool.Definition().Normalized()
	if definition.Risk != RiskCritical || definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if policy := EffectivePolicy(definition, ""); policy != "ask" {
		t.Fatalf("policy = %q, want ask", policy)
	}
}

func TestGitHubDraftPullRequestStrictArguments(t *testing.T) {
	tool := &githubDraftPullRequestTool{service: &fakeGitHubPullRequestCreator{}}
	head := strings.Repeat("a", 40)
	state := strings.Repeat("b", 64)
	valid := json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","body":"Summary"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}

	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","base":"develop"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","draft":false}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":"Draft PR","merge":true}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"short","expected_remote_state_digest":"` + state + `","title":"Draft PR"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"short","title":"Draft PR"}`),
		json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + head + `","expected_remote_state_digest":"` + state + `","title":""}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubDraftPullRequestReturnsStructuredResult(t *testing.T) {
	creator := &fakeGitHubPullRequestCreator{result: &gitrepo.GitHubPullRequestResult{
		Remote: "origin", Repository: "repo", Number: 12, URL: "https://github.com/example/repo/pull/12",
		HeadBranch: "feature/pr", Head: strings.Repeat("a", 40), BaseBranch: "main", Draft: true, Created: true,
	}}
	tool := &githubDraftPullRequestTool{service: creator}
	args := json.RawMessage(`{"remote":"origin","expected_branch":"feature/pr","expected_head":"` + strings.Repeat("a", 40) + `","expected_remote_state_digest":"` + strings.Repeat("b", 64) + `","title":"Draft PR"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}

func TestRegistryGitHubDraftPullRequestGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_create":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")

	withoutPR := NewRegistry()
	if _, ok := withoutPR.Get("github_create_draft_pull_request"); ok {
		t.Fatal("GitHub draft PR tool registered without independent process gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "true")
	withPR := NewRegistry()
	if _, ok := withPR.Get("github_create_draft_pull_request"); !ok {
		t.Fatal("GitHub draft PR tool not registered with remote and PR gates enabled")
	}
	if _, ok := withPR.Get("git_push"); ok {
		t.Fatal("Git push should remain disabled when its independent write/push gates are off")
	}
}
