package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

type fakeGitHubPullRequestReviewReplier struct {
	result *gitrepo.GitHubPullRequestReviewReplyResult
}

func (f *fakeGitHubPullRequestReviewReplier) ReplyToPullRequestReviewComment(context.Context, string, int, string, int64, int64, string, string) (*gitrepo.GitHubPullRequestReviewReplyResult, error) {
	return f.result, nil
}

func TestGitHubPullRequestReviewReplyToolIsHighRiskHostedMutation(t *testing.T) {
	items := NewGitHubPullRequestReplyTools(&fakeGitHubPullRequestReviewReplier{})
	if len(items) != 1 {
		t.Fatalf("tool count = %d, want 1", len(items))
	}
	definition := items[0].Definition().Normalized()
	if definition.Name != "github_reply_to_pull_request_review_comment" || definition.Risk != RiskHigh || definition.ReadOnly || !definition.SideEffecting || !definition.RequiresNetwork || !definition.RequiresCredentials || definition.SupportsParallel {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestGitHubPullRequestReviewReplyToolRejectsUnreviewedControls(t *testing.T) {
	tool := &githubPullRequestReviewReplyTool{service: &fakeGitHubPullRequestReviewReplier{}}
	head := strings.Repeat("a", 40)
	valid := json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"Addressed."}`)
	if err := tool.Validate(valid); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"reply","repository":"other/repo"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"reply","api_url":"https://example.invalid"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"reply","token":"secret"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"short","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"reply"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"yesterday","body":"reply"}`),
		json.RawMessage(`{"remote":"origin","number":7,"expected_head":"` + head + `","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"   "}`),
	} {
		if err := tool.Validate(args); err == nil {
			t.Fatalf("Validate(%s) unexpectedly succeeded", args)
		}
	}
}

func TestGitHubPullRequestReviewReplyToolReturnsBoundedConfirmation(t *testing.T) {
	head := strings.Repeat("a", 40)
	reader := &fakeGitHubPullRequestReviewReplier{result: &gitrepo.GitHubPullRequestReviewReplyResult{
		Remote: "origin", Repository: "repo", PullRequest: 7, Head: head,
		ParentCommentID: 91, ReviewID: 44, ReplyID: 92, Posted: true,
	}}
	tool := &githubPullRequestReviewReplyTool{service: reader}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"remote":"origin","number":7,"expected_head":"`+head+`","comment_id":91,"expected_review_id":44,"expected_updated_at":"2026-08-11T10:00:00Z","body":"Addressed."}`))
	if err != nil || result == nil || result.IsError || len(result.Structured) == 0 || !strings.Contains(result.Content, `"reply_id":92`) {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}

func TestRegistryGitHubPullRequestReviewReplyUsesIndependentGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_create":true,"allow_pull_request_reply":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")

	withoutReply := NewRegistry()
	if _, ok := withoutReply.Get("github_reply_to_pull_request_review_comment"); ok {
		t.Fatal("review reply tool registered without independent reply gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "true")
	withReply := NewRegistry()
	if _, ok := withReply.Get("github_reply_to_pull_request_review_comment"); !ok {
		t.Fatal("review reply tool not registered with remote/reply gates enabled")
	}
	if _, ok := withReply.Get("github_get_pull_request_feedback"); ok {
		t.Fatal("read-only feedback tool should remain disabled when its independent gate is off")
	}
	if _, ok := withReply.Get("github_create_draft_pull_request"); ok {
		t.Fatal("draft PR creation should remain disabled when its independent gate is off")
	}
	if _, ok := withReply.Get("git_push"); ok {
		t.Fatal("Git push should remain disabled when its independent write/push gates are off")
	}
}
