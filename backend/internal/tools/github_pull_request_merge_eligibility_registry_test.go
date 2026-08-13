package tools

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestRegistryGitHubPullRequestMergeEligibilityUsesReadGate(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv(gitrepo.RepositoriesEnv, "repo="+repoPath)
	t.Setenv(gitrepo.RemotesEnv, `{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true}}`)
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")

	withoutRead := NewRegistry()
	if _, ok := withoutRead.Get("github_get_pull_request_merge_eligibility"); ok {
		t.Fatal("eligibility tool registered without the PR read gate")
	}

	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	withRead := NewRegistry()
	if _, ok := withRead.Get("github_get_pull_request_merge_eligibility"); !ok {
		t.Fatal("eligibility tool not registered with remote/read gates enabled")
	}
}
