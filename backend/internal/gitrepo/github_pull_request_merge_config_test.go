package gitrepo

import (
	"context"
	"testing"
)

func TestParseRemoteConfigRequiresReadAndFixedMethodForMerge(t *testing.T) {
	valid := ParseRemoteConfig(`{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_merge":true,"pull_request_merge_method":"SQUASH"}}`)
	remote, ok := valid["origin"]
	if !ok || !remote.AllowPullRequestMerge || remote.PullRequestMergeMethod != "squash" {
		t.Fatalf("valid merge remote not normalized: %#v", valid)
	}

	for _, raw := range []string{
		`{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_merge":true,"pull_request_merge_method":"squash"}}`,
		`{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_merge":true}}`,
		`{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"allow_pull_request_merge":true,"pull_request_merge_method":"octopus"}}`,
		`{"origin":{"repository":"repo","url":"https://github.com/example/repo.git","token_env":"GITHUB_TOKEN","allow_pull_request_read":true,"pull_request_merge_method":"squash"}}`,
	} {
		if got := ParseRemoteConfig(raw); len(got) != 0 {
			t.Fatalf("invalid merge config unexpectedly accepted: %s => %#v", raw, got)
		}
	}
}

func TestGitHubPullRequestMergeGateIsIndependent(t *testing.T) {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {
			Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN",
			AllowPullRequestRead: true, AllowPullRequestMerge: true, PullRequestMergeMethod: "squash",
		},
	}, true, false, nil, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.githubPullRequestReadEnabled = true
	if svc.GitHubPullRequestMergeMutationEnabled() {
		t.Fatal("merge mutation unexpectedly enabled without independent process gate")
	}
	svc.githubPullRequestMergeEnabled = true
	if !svc.GitHubPullRequestMergeMutationEnabled() {
		t.Fatal("merge mutation not enabled with remote/read/merge process gates")
	}
	if svc.GitHubPullRequestMutationEnabled() || svc.GitHubPullRequestReplyMutationEnabled() || svc.GitHubPullRequestThreadResolutionMutationEnabled() || svc.GitHubPullRequestReadyMutationEnabled() || svc.PushMutationEnabled() {
		t.Fatal("merge gate unexpectedly enabled another Git/GitHub mutation")
	}

	summaries := svc.Remotes(context.Background())
	if len(summaries) != 1 || !summaries[0].PullRequestMergeAllowed || summaries[0].PullRequestMergeMethod != "squash" || summaries[0].PullRequestCreateAllowed || summaries[0].PushAllowed {
		t.Fatalf("unexpected remote summary: %#v", summaries)
	}
}

func TestBindingBackedRemoteNeverSynthesizesMergePermission(t *testing.T) {
	id, remote, ok := githubRemoteConfigFromBinding(GitHubRemoteBinding{LocalRepositoryID: "repo", GitHubFullName: "example/repo"})
	if !ok || id == "" {
		t.Fatal("valid binding was rejected")
	}
	if remote.AllowPullRequestMerge || remote.PullRequestMergeMethod != "" || remote.AllowPullRequestRead {
		t.Fatalf("binding synthesized hosted merge/read permission: %#v", remote)
	}
}
