package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestUserScopedRemoteServiceMergeUsesConnectedActorForPreflightAndMutation(t *testing.T) {
	base, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(base, "squash")
	original := base.githubClient.Transport
	mergeCommit := strings.Repeat("f", 40)
	mergeCalls := 0
	base.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("M3B used wrong credential: Authorization=%q", got)
		}
		if request.Method == http.MethodPut && request.URL.Path == "/repos/example/repo/pulls/21/merge" {
			mergeCalls++
			return jsonHTTPResponse(http.StatusOK, `{"sha":"`+mergeCommit+`","merged":true}`), nil
		}
		return original.RoundTrip(request)
	})

	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)
	var _ GitHubPullRequestMerger = scoped

	result, err := scoped.MergePullRequest(context.Background(), "origin", 21, head)
	if err != nil {
		t.Fatalf("MergePullRequest() returned error: %v", err)
	}
	if result == nil || !result.Merged || result.MergeCommit != mergeCommit || mergeCalls != 1 {
		t.Fatalf("unexpected scoped merge result: %#v mergeCalls=%d", result, mergeCalls)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one request-scoped credential resolution, got %d", resolver.calls)
	}
}
