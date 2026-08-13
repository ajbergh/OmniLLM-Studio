package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func setBindingCapabilityBootstrapEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(gitrepo.RepositoriesEnv, "omni="+t.TempDir())
	t.Setenv(gitrepo.RemotesEnv, "")
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")
	t.Setenv(gitrepo.WriteEnabledEnv, "false")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "false")
	t.Setenv(gitrepo.RemoteBranchCreateEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestReadyEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubPullRequestMergeEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubBindingCapabilitiesEnv, "")
	t.Setenv(gitrepo.RemoteCloneEnabledEnv, "false")
	t.Setenv(gitrepo.RemoteCloneMaxBytesEnv, "")
	t.Setenv(gitrepo.RemoteCloneMaxEntriesEnv, "")
}

func bindingCapabilityBootstrapOptions(connectedCalls, resolveCalls, bindingCalls *int) *GitHubCredentialOptions {
	return &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) {
			if connectedCalls != nil {
				*connectedCalls++
			}
			return true, nil
		},
		Resolve: func(context.Context, string) (string, bool, error) {
			if resolveCalls != nil {
				*resolveCalls++
			}
			return "token", true, nil
		},
		Bindings: func(context.Context, string) ([]gitrepo.GitHubRemoteBinding, error) {
			if bindingCalls != nil {
				*bindingCalls++
			}
			return []gitrepo.GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}, nil
		},
	}
}

func TestBindingCapabilityBootstrapRegistersOnlyOperatorAuthorizedFamilies(t *testing.T) {
	setBindingCapabilityBootstrapEnvironment(t)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteBranchCreateEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReplyEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestThreadResolutionEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestReadyEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubPullRequestMergeEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteCloneEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteCloneMaxBytesEnv, "1048576")
	t.Setenv(gitrepo.RemoteCloneMaxEntriesEnv, "128")
	t.Setenv(gitrepo.GitHubBindingCapabilitiesEnv, `{
		"omni": {
			"allow_push": true,
			"allow_branch_create": true,
			"allow_pull_request_read": true,
			"allow_pull_request_create": true,
			"allow_pull_request_reply": true,
			"allow_pull_request_thread_resolution": true,
			"allow_pull_request_ready": true,
			"allow_pull_request_merge": true,
			"pull_request_merge_method": "squash"
		}
	}`)

	connectedCalls, resolveCalls, bindingCalls := 0, 0, 0
	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: bindingCapabilityBootstrapOptions(&connectedCalls, &resolveCalls, &bindingCalls)})

	for _, name := range []string{
		"git_remotes", "git_remote_status", "git_fetch", "git_push", "git_publish_branch",
		"github_get_pull_request", "github_get_pull_request_review_threads", "github_create_pull_request",
		"github_reply_to_pull_request_review_comment", "github_set_pull_request_review_thread_resolved",
		"github_mark_pull_request_ready_for_review", "github_get_pull_request_merge_requirements",
		"github_get_pull_request_merge_policy_evidence", "github_get_pull_request_merge_eligibility",
		"github_merge_pull_request",
	} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("binding capability bootstrap should register %s", name)
		}
	}
	if _, ok := registry.Get("git_clone"); ok {
		t.Fatal("binding capability bootstrap must never register git_clone")
	}
	if connectedCalls != 0 || resolveCalls != 0 || bindingCalls != 0 {
		t.Fatalf("registry construction consulted user GitHub state: connected=%d resolve=%d bindings=%d", connectedCalls, resolveCalls, bindingCalls)
	}
}

func TestBindingCapabilityBootstrapSeparatesPushFromBranchPublication(t *testing.T) {
	setBindingCapabilityBootstrapEnvironment(t)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")
	t.Setenv(gitrepo.RemotePushEnabledEnv, "true")
	t.Setenv(gitrepo.RemoteBranchCreateEnabledEnv, "true")
	t.Setenv(gitrepo.GitHubBindingCapabilitiesEnv, `{"omni":{"allow_push":true}}`)

	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: bindingCapabilityBootstrapOptions(nil, nil, nil)})
	if _, ok := registry.Get("git_push"); !ok {
		t.Fatal("allow_push policy should register git_push under enabled global gates")
	}
	if _, ok := registry.Get("git_publish_branch"); ok {
		t.Fatal("allow_push must not implicitly register git_publish_branch")
	}
}

func TestBindingCapabilityBootstrapCannotBypassProcessWideGate(t *testing.T) {
	setBindingCapabilityBootstrapEnvironment(t)
	t.Setenv(gitrepo.GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(gitrepo.GitHubBindingCapabilitiesEnv, `{"omni":{"allow_pull_request_read":true}}`)

	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: bindingCapabilityBootstrapOptions(nil, nil, nil)})
	if _, ok := registry.Get("github_get_pull_request"); ok {
		t.Fatal("binding policy must not bypass the process-wide pull-request read gate")
	}
	if _, ok := registry.Get("github_get_pull_request_review_threads"); ok {
		t.Fatal("binding policy must not bypass the process-wide review-thread read gate")
	}
}

func TestBindingCapabilityBootstrapFetchUsesIndependentLocalWriteGate(t *testing.T) {
	setBindingCapabilityBootstrapEnvironment(t)
	t.Setenv(gitrepo.WriteEnabledEnv, "true")

	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: bindingCapabilityBootstrapOptions(nil, nil, nil)})
	if _, ok := registry.Get("git_fetch"); !ok {
		t.Fatal("binding-aware remote access plus local Git write should register git_fetch")
	}
	for _, name := range []string{"git_push", "git_publish_branch", "github_get_pull_request", "github_create_pull_request"} {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("fetch prerequisites alone must not register %s", name)
		}
	}
}
