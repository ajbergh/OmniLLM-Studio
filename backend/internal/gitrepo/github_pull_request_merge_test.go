package gitrepo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func enableMergeForTest(svc *RemoteService, method string) {
	remote := svc.remotes["origin"]
	remote.AllowPullRequestRead = true
	remote.AllowPullRequestMerge = true
	remote.PullRequestMergeMethod = method
	svc.remotes["origin"] = remote
	svc.githubPullRequestReadEnabled = true
	svc.githubPullRequestMergeEnabled = true
}

func TestMergePullRequestRunsFreshEligibilityThenOneExactHeadPut(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	mergeCommit := strings.Repeat("f", 40)
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && request.URL.Path == "/repos/example/repo/pulls/21/merge" {
			mergeCalls++
			if request.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(body), `{"sha":"`+head+`","merge_method":"squash"}`; got != want {
				t.Fatalf("merge payload = %s, want %s", got, want)
			}
			return jsonHTTPResponse(http.StatusOK, `{"sha":"`+mergeCommit+`","merged":true,"message":"merged"}`), nil
		}
		return base.RoundTrip(request)
	})

	result, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err != nil {
		t.Fatalf("MergePullRequest() returned error: %v", err)
	}
	if mergeCalls != 1 || result == nil || !result.Merged || !result.Changed || result.Head != head || result.MergeMethod != "squash" || result.MergeCommit != mergeCommit || result.ConfirmedAfterReinspection {
		t.Fatalf("unexpected merge result: %#v mergeCalls=%d", result, mergeCalls)
	}
}

func TestMergePullRequestRejectsStaleExpectedHeadBeforePut(t *testing.T) {
	svc, _ := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
		}
		return base.RoundTrip(request)
	})

	_, err := svc.MergePullRequest(context.Background(), "origin", 21, strings.Repeat("a", 40))
	if err == nil || !strings.Contains(err.Error(), "head changed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if mergeCalls != 0 {
		t.Fatalf("merge PUT called %d times for stale expected head", mergeCalls)
	}
}

func TestMergePullRequestRejectsIncompleteEligibilityBeforePut(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{requireLastPushApproval: true})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
		}
		return base.RoundTrip(request)
	})

	_, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err == nil || !strings.Contains(err.Error(), "eligibility is incomplete or unsatisfied") {
		t.Fatalf("unexpected error: %v", err)
	}
	if mergeCalls != 0 {
		t.Fatalf("merge PUT called %d times for incomplete eligibility", mergeCalls)
	}
}

func TestMergePullRequestRejectsConfiguredMethodOutsideFreshPolicy(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "merge")
	base := svc.githubClient.Transport
	mergeCalls := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
		}
		return base.RoundTrip(request)
	})

	_, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err == nil || !strings.Contains(err.Error(), "configured merge method is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if mergeCalls != 0 {
		t.Fatalf("merge PUT called %d times for disallowed method", mergeCalls)
	}
}

func TestMergePullRequestConflictNeverRetriesMutation(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
			return jsonHTTPResponse(http.StatusConflict, `{"message":"secret-provider-conflict-detail"}`), nil
		}
		return base.RoundTrip(request)
	})

	_, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err == nil || !strings.Contains(err.Error(), "head or mergeability changed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "secret-provider-conflict-detail") {
		t.Fatalf("provider response leaked into error: %v", err)
	}
	if mergeCalls != 1 {
		t.Fatalf("merge PUT called %d times, want exactly one", mergeCalls)
	}
}

func TestMergePullRequestAmbiguousTransportReinspectsWithoutRetry(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	postMergePullReads := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
			return nil, errors.New("transport lost after request")
		}
		if mergeCalls > 0 && request.Method == http.MethodGet && request.URL.Path == "/repos/example/repo/pulls/21" {
			postMergePullReads++
			return jsonHTTPResponse(http.StatusOK, `{"number":21,"html_url":"https://github.com/example/repo/pull/21","title":"M3B","draft":false,"state":"closed","merged":true,"merge_commit_sha":"`+strings.Repeat("d", 40)+`","head":{"ref":"feature/m3a","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		}
		return base.RoundTrip(request)
	})

	result, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err != nil {
		t.Fatalf("MergePullRequest() returned error: %v", err)
	}
	if mergeCalls != 1 || postMergePullReads != 1 || result == nil || !result.Merged || !result.ConfirmedAfterReinspection {
		t.Fatalf("unexpected ambiguous-outcome result: %#v mergeCalls=%d postReads=%d", result, mergeCalls, postMergePullReads)
	}
}

func TestMergePullRequestAmbiguousUnconfirmedOutcomeDoesNotRetry(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{})
	enableMergeForTest(svc, "squash")
	base := svc.githubClient.Transport
	mergeCalls := 0
	svc.githubClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/merge") {
			mergeCalls++
			return nil, errors.New("transport lost after request")
		}
		return base.RoundTrip(request)
	})

	_, err := svc.MergePullRequest(context.Background(), "origin", 21, head)
	if err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
		t.Fatalf("unexpected error: %v", err)
	}
	if mergeCalls != 1 {
		t.Fatalf("merge PUT called %d times, want exactly one", mergeCalls)
	}
}
