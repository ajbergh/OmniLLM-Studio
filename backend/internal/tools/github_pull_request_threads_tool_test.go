package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestReviewThreadReader struct {
	result *gitrepo.GitHubPullRequestReviewThreadsResult
}

func (f *fakeGitHubPullRequestReviewThreadReader) GetPullRequestReviewThreads(context.Context, string, int, string, int) (*gitrepo.GitHubPullRequestReviewThreadsResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestReviewThreadsToolIsLowRiskReadOnly(t *testing.T) {
	items := NewGitHubPullRequestReviewThreadTools(&fakeGitHubPullRequestReviewThreadReader{})
	if len(items) != 1 {
		t.Fatalf("tool count = %d, want 1", len(items))
	}
	definition := items[0].Definition().Normalized()
	if definition.Name != "github_get_pull_request_review_threads" || definition.Risk != RiskLow || !definition.ReadOnly || definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || !definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestReviewThreadsToolRejectsUntrustedControlsAndInvalidBounds(t *testing.T) {
	tool := &githubPullRequestReviewThreadsTool{service: &fakeGitHubPullRequestReviewThreadReader{}}
	if err := tool.Validate(json.RawMessage(`{"remote":"origin","number":7,"after":"opaque-cursor","limit":20}`)); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":7,"query":"mutation { anything }"}`),
		json.RawMessage(`{"remote":"origin","number":7,"thread_id":"PRRT_other"}`),
		json.RawMessage(`{"remote":"origin","number":7,"resolve":true}`),
		json.RawMessage(`{"remote":"origin","number":7,"limit":0}`),
		json.RawMessage(`{"remote":"origin","number":7,"limit":21}`),
		json.RawMessage(`{"remote":"origin","number":7,"after":"` + strings.Repeat("x", 513) + `"}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestReviewThreadsToolReturnsStructuredState(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestReviewThreadReader{result: &gitrepo.GitHubPullRequestReviewThreadsResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head, Limit: 10, TotalCount: 1,
		Threads: []gitrepo.GitHubPullRequestReviewThreadResult{{ID: "PRRT_thread", IsResolved: false, IsOutdated: true, Path: "backend/main.go", ViewerCanResolve: true}},
	}}
	tool := &githubPullRequestReviewThreadsTool{service: reader}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	var decoded gitrepo.GitHubPullRequestReviewThreadsResult
	if err := json.Unmarshal(result.Structured, &decoded); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}
	if decoded.Head != head || decoded.TotalCount != 1 || len(decoded.Threads) != 1 || decoded.Threads[0].ID != "PRRT_thread" || !decoded.Threads[0].IsOutdated || !decoded.Threads[0].ViewerCanResolve {
		t.Fatalf("unexpected structured state: %#v", decoded)
	}
}

func TestRegistryGitHubPullRequestReviewThreadsUsesReadGateOnly(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_create":true,"allow_pull_request_reply":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")

	withoutRead := NewRegistry()
	if _, ok := withoutRead.Get("github_get_pull_request_review_threads"); ok {
		t.Fatal("review thread tool registered without independent read gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	withRead := NewRegistry()
	if _, ok := withRead.Get("github_get_pull_request_review_threads"); !ok {
		t.Fatal("review thread tool not registered with remote/read gates enabled")
	}
	if _, ok := withRead.Get("github_reply_to_pull_request_review_comment"); ok {
		t.Fatal("review reply mutation should remain disabled when its independent gate is off")
	}
	if _, ok := withRead.Get("github_create_draft_pull_request"); ok {
		t.Fatal("draft PR creation should remain disabled when its independent gate is off")
	}
	if _, ok := withRead.Get("git_push"); ok {
		t.Fatal("Git push should remain disabled when its independent write/push gates are off")
	}
}
