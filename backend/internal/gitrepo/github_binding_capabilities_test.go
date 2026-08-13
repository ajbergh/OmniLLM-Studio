package gitrepo

import "testing"

func TestParseGitHubBindingCapabilitiesNormalizesValidPolicy(t *testing.T) {
	configured := ParseGitHubBindingCapabilities(`{
		"omni": {
			"allow_push": true,
			"allow_branch_create": true,
			"allow_pull_request_read": true,
			"allow_pull_request_create": true,
			"allow_pull_request_reply": true,
			"allow_pull_request_thread_resolution": true,
			"allow_pull_request_ready": true,
			"allow_pull_request_merge": true,
			"pull_request_merge_method": " SQUASH "
		}
	}`)
	policy, ok := configured["omni"]
	if !ok {
		t.Fatal("valid binding capability policy was not parsed")
	}
	if !policy.AllowPush || !policy.AllowBranchCreate || !policy.AllowPullRequestRead || !policy.AllowPullRequestCreate ||
		!policy.AllowPullRequestReply || !policy.AllowPullRequestThreadResolution || !policy.AllowPullRequestReady ||
		!policy.AllowPullRequestMerge || policy.PullRequestMergeMethod != "squash" {
		t.Fatalf("unexpected normalized policy: %#v", policy)
	}
}

func TestParseGitHubBindingCapabilitiesSkipsInvalidEntriesIndependently(t *testing.T) {
	configured := ParseGitHubBindingCapabilities(`{
		"valid": {"allow_pull_request_read": true},
		"branch-without-push": {"allow_branch_create": true},
		"merge-without-read": {"allow_pull_request_merge": true, "pull_request_merge_method": "squash"},
		"method-without-merge": {"pull_request_merge_method": "squash"},
		"unknown-field": {"allow_pull_request_read": true, "surprise": true},
		"bad id!": {"allow_pull_request_read": true}
	}`)
	if len(configured) != 1 {
		t.Fatalf("expected only one valid entry, got %#v", configured)
	}
	if policy, ok := configured["valid"]; !ok || !policy.AllowPullRequestRead {
		t.Fatalf("valid entry was lost: %#v", configured)
	}
}

func TestApplyGitHubBindingCapabilitiesNeverGrantsDefaultBranchPushOrClone(t *testing.T) {
	remote := RemoteConfig{
		Repository:             "omni",
		URL:                    "https://github.com/octo/studio.git",
		AllowDefaultBranchPush: true,
		AllowClone:             true,
	}
	applied := applyGitHubBindingCapabilities(remote, GitHubBindingCapabilities{
		AllowPush:              true,
		AllowBranchCreate:      true,
		AllowPullRequestRead:   true,
		AllowPullRequestCreate: true,
	})
	if !applied.AllowPush || !applied.AllowBranchCreate || !applied.AllowPullRequestRead || !applied.AllowPullRequestCreate {
		t.Fatalf("expected configured feature-branch capabilities: %#v", applied)
	}
	if applied.AllowDefaultBranchPush || applied.AllowClone {
		t.Fatalf("binding capability policy widened excluded operations: %#v", applied)
	}
}
