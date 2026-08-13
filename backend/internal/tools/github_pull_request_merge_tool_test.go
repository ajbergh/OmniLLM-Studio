package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestMerger struct {
	result *gitrepo.GitHubPullRequestMergeResult
}

func (f *fakeGitHubPullRequestMerger) MergePullRequest(context.Context, string, int, string) (*gitrepo.GitHubPullRequestMergeResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestMergeToolIsCriticalHostedMutation(t *testing.T) {
	items := NewGitHubPullRequestMergeTools(&fakeGitHubPullRequestMerger{})
	if len(items) != 1 {
		t.Fatalf("tool count = %d, want 1", len(items))
	}
	definition := items[0].Definition().Normalized()
	if definition.Name != "github_merge_pull_request" || definition.Risk != RiskCritical || definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestMergeToolAcceptsOnlyReviewedIdentity(t *testing.T) {
	tool := &githubPullRequestMergeTool{service: &fakeGitHubPullRequestMerger{}}
	head := strings.Repeat("a", 40)
	valid := json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","base":"develop"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","merge_method":"merge"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","delete_branch":true}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","commit_title":"custom"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","commit_message":"custom"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"short"}`),
		json.RawMessage(`{"remote":"origin","number":0,"expected_head":"` + head + `"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestMergeToolReturnsStructuredConfirmation(t *testing.T) {
	head := strings.Repeat("a", 40)
	mergeCommit := strings.Repeat("b", 40)
	merger := &fakeGitHubPullRequestMerger{result: &gitrepo.GitHubPullRequestMergeResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head, BaseBranch: "main",
		MergeMethod: "squash", MergeCommit: mergeCommit, Merged: true, Changed: true,
	}}
	tool := &githubPullRequestMergeTool{service: merger}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7,"expected_head":"`+head+`"}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	var confirmation gitrepo.GitHubPullRequestMergeResult
	if err := json.Unmarshal(result.Structured, &confirmation); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if confirmation.PullRequest != 7 || confirmation.Head != head || confirmation.BaseBranch != "main" || confirmation.MergeMethod != "squash" || confirmation.MergeCommit != mergeCommit || !confirmation.Merged || !confirmation.Changed {
		t.Fatalf("unexpected structured confirmation: %#v", confirmation)
	}
}

func TestRegistryGitHubPullRequestMergeRequiresReadAndIndependentMergeGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_merge":true,"pull_request_merge_method":"squash"}}`)
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestMergeEnabledEnv, "true")

	withoutRead := NewRegistry()
	if _, ok := withoutRead.Get("github_merge_pull_request"); ok {
		t.Fatal("merge tool registered without required PR-read gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestMergeEnabledEnv, "false")
	withoutMerge := NewRegistry()
	if _, ok := withoutMerge.Get("github_merge_pull_request"); ok {
		t.Fatal("merge tool registered without independent merge gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestMergeEnabledEnv, "true")
	withMerge := NewRegistry()
	if _, ok := withMerge.Get("github_merge_pull_request"); !ok {
		t.Fatal("merge tool not registered with remote/read/merge gates enabled")
	}
	for _, name := range []string{"github_create_draft_pull_request", "github_reply_to_pull_request_review_comment", "github_set_pull_request_review_thread_resolved", "github_mark_pull_request_ready_for_review", "git_push"} {
		if _, ok := withMerge.Get(name); ok {
			t.Fatalf("%s should remain disabled when its independent mutation gate is off", name)
		}
	}
}
