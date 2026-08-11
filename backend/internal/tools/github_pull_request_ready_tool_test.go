package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestReadyMarker struct {
	result *gitrepo.GitHubPullRequestReadyResult
}

func (f *fakeGitHubPullRequestReadyMarker) MarkPullRequestReadyForReview(context.Context, string, int, string) (*gitrepo.GitHubPullRequestReadyResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestReadyToolIsHighRiskHostedMutation(t *testing.T) {
	items := NewGitHubPullRequestReadyTools(&fakeGitHubPullRequestReadyMarker{})
	if len(items) != 1 {
		t.Fatalf("tool count = %d, want 1", len(items))
	}
	definition := items[0].Definition().Normalized()
	if definition.Name != "github_mark_pull_request_ready_for_review" || definition.Risk != RiskHigh || definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestReadyToolAcceptsOnlyReviewedIdentity(t *testing.T) {
	tool := &githubPullRequestReadyTool{service: &fakeGitHubPullRequestReadyMarker{}}
	head := strings.Repeat("a", 40)
	valid := json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `"}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","pull_request_id":"PR_node_7"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","base":"develop"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","reviewers":["octocat"]}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"short"}`),
		json.RawMessage(`{"remote":"origin","number":0,"expected_head":"` + head + `"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestReadyToolReturnsStructuredConfirmation(t *testing.T) {
	head := strings.Repeat("a", 40)
	marker := &fakeGitHubPullRequestReadyMarker{result: &gitrepo.GitHubPullRequestReadyResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head, BaseBranch: "main", Ready: true, Changed: true,
	}}
	tool := &githubPullRequestReadyTool{service: marker}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7,"expected_head":"`+head+`"}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	var confirmation gitrepo.GitHubPullRequestReadyResult
	if err := json.Unmarshal(result.Structured, &confirmation); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if confirmation.PullRequest != 7 || confirmation.Head != head || confirmation.BaseBranch != "main" || !confirmation.Ready || !confirmation.Changed || confirmation.Draft {
		t.Fatalf("unexpected structured confirmation: %#v", confirmation)
	}
}

func TestRegistryGitHubPullRequestReadyUsesIndependentGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_create":true,"allow_pull_request_reply":true,"allow_pull_request_thread_resolution":true,"allow_pull_request_ready":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadyEnabledEnv, "false")

	withoutReady := NewRegistry()
	if _, ok := withoutReady.Get("github_mark_pull_request_ready_for_review"); ok {
		t.Fatal("ready-for-review tool registered without independent ready gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadyEnabledEnv, "true")
	withReady := NewRegistry()
	if _, ok := withReady.Get("github_mark_pull_request_ready_for_review"); !ok {
		t.Fatal("ready-for-review tool not registered with remote/ready gates enabled")
	}
	for _, name := range []string{"github_get_pull_request", "github_create_draft_pull_request", "github_reply_to_pull_request_review_comment", "github_set_pull_request_review_thread_resolved", "git_push"} {
		if _, ok := withReady.Get(name); ok {
			t.Fatalf("%s should remain disabled when its independent gate is off", name)
		}
	}
}
