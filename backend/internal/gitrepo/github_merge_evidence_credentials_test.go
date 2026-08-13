package gitrepo

import (
	"context"
	"net/http"
	"testing"
)

func TestUserScopedRemoteServiceMergeEligibilityUsesConnectedActorCredential(t *testing.T) {
	base, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{
		checkAppID: 15368, checkConclusion: "success", statusState: "success",
		reviewDecision: "APPROVED", deploymentState: "success",
	})
	originalTransport := base.githubClient.Transport
	base.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("merge evidence used wrong credential: Authorization=%q", got)
		}
		return originalTransport.RoundTrip(request)
	})

	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)

	var _ GitHubPullRequestMergePolicyEvidenceReader = scoped
	var _ GitHubPullRequestMergeEligibilityReader = scoped

	result, err := scoped.GetPullRequestMergeEligibility(context.Background(), "origin", 21)
	if err != nil {
		t.Fatalf("GetPullRequestMergeEligibility() returned error: %v", err)
	}
	if result == nil || result.Head != head || !result.EligibilityComplete || !result.Eligible {
		t.Fatalf("unexpected scoped eligibility result: %#v", result)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one request-scoped credential resolution, got %d", resolver.calls)
	}
	if resolver.statusCalls != 0 {
		t.Fatalf("network evidence path unexpectedly used local status resolver %d times", resolver.statusCalls)
	}
}
