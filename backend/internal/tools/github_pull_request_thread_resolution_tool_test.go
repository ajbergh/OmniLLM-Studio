package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestReviewThreadResolver struct {
	result *gitrepo.GitHubPullRequestReviewThreadResolutionResult
}

func (f *fakeGitHubPullRequestReviewThreadResolver) SetPullRequestReviewThreadResolved(context.Context, string, int, string, string, bool, bool, bool) (*gitrepo.GitHubPullRequestReviewThreadResolutionResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestReviewThreadResolutionToolIsHighRiskHostedMutation(t *testing.T) {
	items := NewGitHubPullRequestReviewThreadResolutionTools(&fakeGitHubPullRequestReviewThreadResolver{})
	if len(items) != 1 {
		t.Fatalf("tool count = %d, want 1", len(items))
	}
	definition := items[0].Definition().Normalized()
	if definition.Name != "github_set_pull_request_review_thread_resolved" || definition.Risk != RiskHigh || definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestReviewThreadResolutionToolRejectsUnreviewedControlsAndNoop(t *testing.T) {
	tool := &githubPullRequestReviewThreadResolutionTool{service: &fakeGitHubPullRequestReviewThreadResolver{}}
	head := strings.Repeat("a", 40)
	valid := json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true,"repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true,"api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true,"token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true,"query":"mutation { anything }"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true,"clientMutationId":"x"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"short","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"` + strings.Repeat("x", 257) + `","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","thread_id":"PRRT_thread","expected_is_resolved":true,"expected_is_outdated":false,"resolved":true}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestReviewThreadResolutionToolReturnsStructuredConfirmation(t *testing.T) {
	head := strings.Repeat("a", 40)
	resolver := &fakeGitHubPullRequestReviewThreadResolver{result: &gitrepo.GitHubPullRequestReviewThreadResolutionResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head,
		ThreadID: "PRRT_thread", Resolved: true, Outdated: false, Changed: true,
	}}
	tool := &githubPullRequestReviewThreadResolutionTool{service: resolver}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7,"expected_head":"`+head+`","thread_id":"PRRT_thread","expected_is_resolved":false,"expected_is_outdated":false,"resolved":true}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	var confirmation gitrepo.GitHubPullRequestReviewThreadResolutionResult
	if err := json.Unmarshal(result.Structured, &confirmation); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if confirmation.ThreadID != "PRRT_thread" || !confirmation.Resolved || !confirmation.Changed || confirmation.PullRequest != 7 || confirmation.Head != head {
		t.Fatalf("unexpected structured confirmation: %#v", confirmation)
	}
}

func TestRegistryGitHubPullRequestReviewThreadResolutionUsesIndependentGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_create":true,"allow_pull_request_reply":true,"allow_pull_request_thread_resolution":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "false")

	withoutResolution := NewRegistry()
	if _, ok := withoutResolution.Get("github_set_pull_request_review_thread_resolved"); ok {
		t.Fatal("thread resolution tool registered without independent resolution gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "true")
	withResolution := NewRegistry()
	if _, ok := withResolution.Get("github_set_pull_request_review_thread_resolved"); !ok {
		t.Fatal("thread resolution tool not registered with remote/resolution gates enabled")
	}
	if _, ok := withResolution.Get("github_get_pull_request_review_threads"); ok {
		t.Fatal("review thread read tool should remain disabled when its independent read gate is off")
	}
	if _, ok := withResolution.Get("github_reply_to_pull_request_review_comment"); ok {
		t.Fatal("review reply mutation should remain disabled when its independent gate is off")
	}
	if _, ok := withResolution.Get("github_create_draft_pull_request"); ok {
		t.Fatal("draft PR creation should remain disabled when its independent gate is off")
	}
	if _, ok := withResolution.Get("git_push"); ok {
		t.Fatal("Git push should remain disabled when its independent write/push gates are off")
	}
}
