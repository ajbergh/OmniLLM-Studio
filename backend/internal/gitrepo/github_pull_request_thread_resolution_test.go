package gitrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGitHubPullRequestThreadResolutionGateIsIndependent(t *testing.T) {
	svc := newGitHubPullRequestThreadResolutionTestService()
	if !svc.GitHubPullRequestThreadResolutionMutationEnabled() {
		t.Fatal("GitHubPullRequestThreadResolutionMutationEnabled() = false with resolution gate enabled")
	}
	if svc.GitHubPullRequestReadAccessEnabled() || svc.GitHubPullRequestReplyMutationEnabled() || svc.GitHubPullRequestMutationEnabled() || svc.PushMutationEnabled() {
		t.Fatal("thread resolution gate unexpectedly enabled another Git/GitHub capability")
	}
	summaries := svc.Remotes(context.Background())
	if len(summaries) != 1 || !summaries[0].PullRequestThreadResolutionAllowed || summaries[0].PullRequestReadAllowed || summaries[0].PullRequestReplyAllowed || summaries[0].PullRequestCreateAllowed || summaries[0].PushAllowed {
		t.Fatalf("unexpected remote summary: %#v", summaries)
	}
}

func TestSetPullRequestReviewThreadResolvedUsesFixedQueriesAndExactState(t *testing.T) {
	for _, test := range []struct {
		name             string
		expectedResolved bool
		resolved         bool
		viewerCanResolve bool
		viewerCanUndo    bool
		mutation         string
		payloadField     string
	}{
		{name: "resolve", expectedResolved: false, resolved: true, viewerCanResolve: true, mutation: githubResolvePullRequestReviewThreadMutation, payloadField: "resolveReviewThread"},
		{name: "unresolve", expectedResolved: true, resolved: false, viewerCanUndo: true, mutation: githubUnresolvePullRequestReviewThreadMutation, payloadField: "unresolveReviewThread"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestThreadResolutionTestService()
			head := strings.Repeat("a", 40)
			threadID := "PRRT_thread_123"
			graphqlCalls := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Header.Get("Authorization") != "Bearer test-token" {
					t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
				}
				switch request.Method + " " + request.URL.Path {
				case "GET /repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
				case "POST /graphql":
					graphqlCalls++
					payload, err := io.ReadAll(request.Body)
					if err != nil {
						t.Fatalf("read GraphQL body: %v", err)
					}
					var decoded struct {
						Query     string                 `json:"query"`
						Variables map[string]interface{} `json:"variables"`
					}
					if err := json.Unmarshal(payload, &decoded); err != nil {
						t.Fatalf("decode GraphQL body: %v", err)
					}
					if decoded.Variables["threadId"] != threadID || len(decoded.Variables) != 1 {
						t.Fatalf("unexpected variables: %#v", decoded.Variables)
					}
					if graphqlCalls == 1 {
						if decoded.Query != githubPullRequestReviewThreadStateQuery || strings.Contains(decoded.Query, threadID) {
							t.Fatalf("preflight query was not fixed: %q", decoded.Query)
						}
						return jsonHTTPResponse(http.StatusOK, threadStateGraphQLResponse(threadID, head, test.expectedResolved, false, test.viewerCanResolve, test.viewerCanUndo)), nil
					}
					if decoded.Query != test.mutation || strings.Contains(decoded.Query, threadID) {
						t.Fatalf("mutation was not fixed: %q", decoded.Query)
					}
					return jsonHTTPResponse(http.StatusOK, threadMutationGraphQLResponse(test.payloadField, threadID, head, test.resolved, false)), nil
				default:
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			})}

			result, err := svc.SetPullRequestReviewThreadResolved(context.Background(), "origin", 7, head, threadID, test.expectedResolved, false, test.resolved)
			if err != nil {
				t.Fatalf("SetPullRequestReviewThreadResolved() returned error: %v", err)
			}
			if !result.Changed || result.Resolved != test.resolved || result.Outdated || result.ThreadID != threadID || result.Head != head || result.PullRequest != 7 || result.Repository != "repo" || graphqlCalls != 2 {
				t.Fatalf("unexpected result: %#v graphqlCalls=%d", result, graphqlCalls)
			}
		})
	}
}

func TestSetPullRequestReviewThreadResolvedRejectsInvalidRequestBeforeNetwork(t *testing.T) {
	svc := newGitHubPullRequestThreadResolutionTestService()
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	})}
	head := strings.Repeat("a", 40)
	for _, test := range []struct {
		number           int
		expectedHead     string
		threadID         string
		expectedResolved bool
		resolved         bool
	}{
		{number: 0, expectedHead: head, threadID: "PRRT_thread", expectedResolved: false, resolved: true},
		{number: 7, expectedHead: "short", threadID: "PRRT_thread", expectedResolved: false, resolved: true},
		{number: 7, expectedHead: head, threadID: "", expectedResolved: false, resolved: true},
		{number: 7, expectedHead: head, threadID: strings.Repeat("x", maxGitHubGraphQLNodeIDBytes+1), expectedResolved: false, resolved: true},
		{number: 7, expectedHead: head, threadID: "PRRT_thread", expectedResolved: true, resolved: true},
	} {
		if _, err := svc.SetPullRequestReviewThreadResolved(context.Background(), "origin", test.number, test.expectedHead, test.threadID, test.expectedResolved, false, test.resolved); err == nil {
			t.Fatalf("invalid request unexpectedly succeeded: %#v", test)
		}
	}
	if called {
		t.Fatal("GitHub API was called for invalid thread resolution input")
	}
}

func TestSetPullRequestReviewThreadResolvedRejectsClosedOrStalePullRequestBeforeThreadLookup(t *testing.T) {
	for _, test := range []struct {
		name       string
		state      string
		currentHead string
		want       string
	}{
		{name: "closed", state: "closed", currentHead: strings.Repeat("a", 40), want: "no longer open"},
		{name: "stale head", state: "open", currentHead: strings.Repeat("b", 40), want: "head changed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestThreadResolutionTestService()
			expectedHead := strings.Repeat("a", 40)
			calls := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.Path != "/repos/example/repo/pulls/7" {
					t.Fatalf("unexpected request: %s", request.URL.Path)
				}
				return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Threads","state":"`+test.state+`","head":{"ref":"feature/threads","sha":"`+test.currentHead+`"},"base":{"ref":"main"}}`), nil
			})}
			_, err := svc.SetPullRequestReviewThreadResolved(context.Background(), "origin", 7, expectedHead, "PRRT_thread", false, false, true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 1 {
				t.Fatalf("GitHub calls = %d, want 1", calls)
			}
		})
	}
}

func TestSetPullRequestReviewThreadResolvedRejectsChangedThreadStateBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name             string
		threadID         string
		repository       string
		pullNumber       int
		head             string
		resolved         bool
		outdated         bool
		viewerCanResolve bool
		want             string
	}{
		{name: "wrong thread", threadID: "PRRT_other", repository: "example/repo", pullNumber: 7, head: strings.Repeat("a", 40), viewerCanResolve: true, want: "identity changed"},
		{name: "wrong repository", threadID: "PRRT_thread", repository: "other/repo", pullNumber: 7, head: strings.Repeat("a", 40), viewerCanResolve: true, want: "ownership changed"},
		{name: "wrong pull", threadID: "PRRT_thread", repository: "example/repo", pullNumber: 8, head: strings.Repeat("a", 40), viewerCanResolve: true, want: "ownership changed"},
		{name: "wrong head", threadID: "PRRT_thread", repository: "example/repo", pullNumber: 7, head: strings.Repeat("b", 40), viewerCanResolve: true, want: "head changed"},
		{name: "resolved changed", threadID: "PRRT_thread", repository: "example/repo", pullNumber: 7, head: strings.Repeat("a", 40), resolved: true, viewerCanResolve: true, want: "state changed"},
		{name: "outdated changed", threadID: "PRRT_thread", repository: "example/repo", pullNumber: 7, head: strings.Repeat("a", 40), outdated: true, viewerCanResolve: true, want: "state changed"},
		{name: "viewer cannot resolve", threadID: "PRRT_thread", repository: "example/repo", pullNumber: 7, head: strings.Repeat("a", 40), viewerCanResolve: false, want: "cannot resolve"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestThreadResolutionTestService()
			expectedHead := strings.Repeat("a", 40)
			graphqlCalls := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+expectedHead+`"},"base":{"ref":"main"}}`), nil
				case "/graphql":
					graphqlCalls++
					return jsonHTTPResponse(http.StatusOK, threadStateGraphQLResponseWithOwnership(test.threadID, test.repository, test.pullNumber, test.head, test.resolved, test.outdated, test.viewerCanResolve, false)), nil
				default:
					t.Fatalf("unexpected request: %s", request.URL.Path)
					return nil, nil
				}
			})}
			_, err := svc.SetPullRequestReviewThreadResolved(context.Background(), "origin", 7, expectedHead, "PRRT_thread", false, false, true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if graphqlCalls != 1 {
				t.Fatalf("GraphQL calls = %d, want preflight only", graphqlCalls)
			}
		})
	}
}

func TestSetPullRequestReviewThreadResolvedRequiresInspectionAfterAmbiguousMutation(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		mutationBody string
		want       string
	}{
		{name: "provider error", status: http.StatusInternalServerError, mutationBody: `{"message":"secret-provider-detail"}`, want: "outcome is unknown"},
		{name: "graphql error", status: http.StatusOK, mutationBody: `{"errors":[{"message":"secret-provider-detail"}]}`, want: "outcome is unknown"},
		{name: "invalid success", status: http.StatusOK, mutationBody: `{"data":{"resolveReviewThread":{"thread":null}}}`, want: "outcome could not be validated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestThreadResolutionTestService()
			head := strings.Repeat("a", 40)
			graphqlCalls := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
				case "/graphql":
					graphqlCalls++
					if graphqlCalls == 1 {
						return jsonHTTPResponse(http.StatusOK, threadStateGraphQLResponse("PRRT_thread", head, false, false, true, false)), nil
					}
					return jsonHTTPResponse(test.status, test.mutationBody), nil
				default:
					t.Fatalf("unexpected request: %s", request.URL.Path)
					return nil, nil
				}
			})}
			_, err := svc.SetPullRequestReviewThreadResolved(context.Background(), "origin", 7, head, "PRRT_thread", false, false, true)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "inspect review threads before retrying") {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), "secret-provider-detail") {
				t.Fatalf("provider error detail leaked: %v", err)
			}
		})
	}
}

func newGitHubPullRequestThreadResolutionTestService() *RemoteService {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {
			Repository: "repo", URL: "https://github.com/example/repo.git", Username: "git",
			TokenEnv: "GITHUB_TOKEN", AllowPullRequestThreadResolution: true,
		},
	}, true, false, nil, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.githubPullRequestThreadResolutionEnabled = true
	return svc
}

func threadStateGraphQLResponse(threadID, head string, resolved, outdated, viewerCanResolve, viewerCanUnresolve bool) string {
	return threadStateGraphQLResponseWithOwnership(threadID, "example/repo", 7, head, resolved, outdated, viewerCanResolve, viewerCanUnresolve)
}

func threadStateGraphQLResponseWithOwnership(threadID, repository string, pullNumber int, head string, resolved, outdated, viewerCanResolve, viewerCanUnresolve bool) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{"node": map[string]interface{}{
			"id": threadID, "isResolved": resolved, "isOutdated": outdated,
			"viewerCanResolve": viewerCanResolve, "viewerCanUnresolve": viewerCanUnresolve,
			"repository": map[string]string{"nameWithOwner": repository},
			"pullRequest": map[string]interface{}{"number": pullNumber, "headRefOid": head},
		}},
	})
	return string(payload)
}

func threadMutationGraphQLResponse(field, threadID, head string, resolved, outdated bool) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{field: map[string]interface{}{"thread": map[string]interface{}{
			"id": threadID, "isResolved": resolved, "isOutdated": outdated,
			"repository": map[string]string{"nameWithOwner": "example/repo"},
			"pullRequest": map[string]interface{}{"number": 7, "headRefOid": head},
		}}},
	})
	return string(payload)
}
