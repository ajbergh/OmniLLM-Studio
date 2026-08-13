package gitrepo

import (
	"context"
	"strconv"
	"testing"
)

func TestNewRemoteServiceSnapshotsAllowlistedGitHubBindingCapabilities(t *testing.T) {
	local := NewServiceWithWriteAccess(map[string]string{"omni": t.TempDir()}, true)
	t.Setenv(GitHubBindingCapabilitiesEnv, `{
		"omni": {"allow_pull_request_read": true},
		"missing": {"allow_push": true}
	}`)

	service := NewRemoteServiceFromEnvironment(local)
	if len(service.githubBindingCapabilities) != 1 {
		t.Fatalf("binding capability snapshot = %#v", service.githubBindingCapabilities)
	}
	policy, ok := service.githubBindingCapabilities["omni"]
	if !ok || !policy.AllowPullRequestRead {
		t.Fatalf("allowlisted binding policy was not snapshotted: %#v", service.githubBindingCapabilities)
	}
	if _, ok := service.githubBindingCapabilities["missing"]; ok {
		t.Fatal("binding policy for a non-allowlisted local repository was retained")
	}

	t.Setenv(GitHubBindingCapabilitiesEnv, `{"omni":{"allow_push":true}}`)
	policy = service.githubBindingCapabilities["omni"]
	if policy.AllowPush || !policy.AllowPullRequestRead {
		t.Fatalf("binding policy changed after startup snapshot: %#v", policy)
	}
}

func TestBindingCapabilityPolicyAppliesOnlyThroughExistingGlobalGates(t *testing.T) {
	local := NewServiceWithWriteAccess(map[string]string{"omni": t.TempDir()}, true)
	t.Setenv(RemoteEnabledEnv, "true")
	t.Setenv(RemotePushEnabledEnv, "true")
	t.Setenv(RemoteBranchCreateEnabledEnv, "true")
	t.Setenv(GitHubPullRequestEnabledEnv, "true")
	t.Setenv(GitHubPullRequestReadEnabledEnv, "true")
	t.Setenv(GitHubPullRequestReplyEnabledEnv, "true")
	t.Setenv(GitHubPullRequestThreadResolutionEnabledEnv, "true")
	t.Setenv(GitHubPullRequestReadyEnabledEnv, "true")
	t.Setenv(GitHubPullRequestMergeEnabledEnv, "true")
	t.Setenv(RemoteCloneEnabledEnv, "true")
	t.Setenv(RemoteCloneMaxBytesEnv, strconv.FormatInt(minRemoteCloneBytes, 10))
	t.Setenv(RemoteCloneMaxEntriesEnv, strconv.FormatInt(minRemoteCloneEntries, 10))
	t.Setenv(GitHubBindingCapabilitiesEnv, `{
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

	base := NewRemoteServiceFromEnvironment(local)
	credentials := &testGitHubCredentialResolver{token: "user-token", connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 {
		t.Fatalf("expected one binding-backed remote, got %#v", summaries)
	}
	summary := summaries[0]
	if !summary.AuthenticationConfigured || !summary.PushAllowed || !summary.BranchCreateAllowed ||
		!summary.PullRequestReadAllowed || !summary.PullRequestCreateAllowed || !summary.PullRequestReplyAllowed ||
		!summary.PullRequestThreadResolutionAllowed || !summary.PullRequestReadyAllowed || !summary.PullRequestMergeAllowed {
		t.Fatalf("configured binding capabilities were not applied through global gates: %#v", summary)
	}
	if summary.PullRequestMergeMethod != "squash" {
		t.Fatalf("binding merge method = %q", summary.PullRequestMergeMethod)
	}
	if summary.DefaultBranchPushAllowed || summary.CloneAllowed {
		t.Fatalf("binding policy widened excluded operations: %#v", summary)
	}
	if len(base.remotes) != 0 {
		t.Fatalf("binding policy mutated base remote configuration: %#v", base.remotes)
	}
}

func TestBindingCapabilityPolicyCannotBypassDisabledGlobalGate(t *testing.T) {
	local := NewServiceWithWriteAccess(map[string]string{"omni": t.TempDir()}, true)
	t.Setenv(RemoteEnabledEnv, "true")
	t.Setenv(RemotePushEnabledEnv, "false")
	t.Setenv(GitHubPullRequestReadEnabledEnv, "false")
	t.Setenv(GitHubBindingCapabilitiesEnv, `{
		"omni": {
			"allow_push": true,
			"allow_pull_request_read": true
		}
	}`)

	base := NewRemoteServiceFromEnvironment(local)
	credentials := &testGitHubCredentialResolver{token: "user-token", connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 {
		t.Fatalf("expected one binding-backed remote, got %#v", summaries)
	}
	if summaries[0].PushAllowed || summaries[0].PullRequestReadAllowed {
		t.Fatalf("per-binding policy bypassed a process-wide gate: %#v", summaries[0])
	}
}
