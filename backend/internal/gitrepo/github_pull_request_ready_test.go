package gitrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGitHubPullRequestReadyGateIsIndependent(t *testing.T) {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN", AllowPullRequestReady: true},
	}, true, false, &githubPullRequestTestTransport{}, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.githubPullRequestReadyEnabled = true
	if !svc.GitHubPullRequestReadyMutationEnabled() {
		t.Fatal("GitHubPullRequestReadyMutationEnabled() = false with ready gate enabled")
	}
	if svc.GitHubPullRequestReadAccessEnabled() || svc.GitHubPullRequestMutationEnabled() || svc.GitHubPullRequestReplyMutationEnabled() || svc.GitHubPullRequestThreadResolutionMutationEnabled() || svc.PushMutationEnabled() {
		t.Fatal("ready-for-review gate unexpectedly enabled another Git/GitHub capability")
	}
	summaries := svc.Remotes(context.Background())
	if len(summaries) != 1 || !summaries[0].PullRequestReadyAllowed || summaries[0].PullRequestReadAllowed || summaries[0].PullRequestCreateAllowed || summaries[0].PullRequestReplyAllowed || summaries[0].PullRequestThreadResolutionAllowed || summaries[0].PushAllowed {
		t.Fatalf("unexpected remote summary: %#v", summaries)
	}
}

func TestMarkPullRequestReadyForReviewUsesFixedMutationAndReviewedState(t *testing.T) {
	svc, head, _ := newGitHubPullRequestTestService(t, "feature/ready")
	enableReadyForTest(svc)
	graphqlCalls := 0
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, readyRESTPull(head.String(), true)), nil
		case "POST /graphql":
			graphqlCalls++
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatal(err)
			}
			if graphqlCalls == 1 {
				if decoded.Query != githubPullRequestReadyStateQuery || decoded.Variables["owner"] != "example" || decoded.Variables["repository"] != "repo" || decoded.Variables["number"] != float64(7) {
					t.Fatalf("unexpected preflight GraphQL payload: %#v", decoded)
				}
				return jsonHTTPResponse(http.StatusOK, readyStateResponse(head.String(), true)), nil
			}
			if decoded.Query != githubMarkPullRequestReadyForReviewMutation || len(decoded.Variables) != 1 || decoded.Variables["pullRequestId"] != "PR_node_7" {
				t.Fatalf("unexpected mutation GraphQL payload: %#v", decoded)
			}
			return jsonHTTPResponse(http.StatusOK, readyMutationResponse(head.String(), false)), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.MarkPullRequestReadyForReview(context.Background(), "origin", 7, head.String())
	if err != nil {
		t.Fatalf("MarkPullRequestReadyForReview() returned error: %v", err)
	}
	if result.PullRequest != 7 || result.Head != head.String() || result.BaseBranch != "main" || result.Draft || !result.Ready || !result.Changed || graphqlCalls != 2 {
		t.Fatalf("unexpected result: %#v graphqlCalls=%d", result, graphqlCalls)
	}
}

func TestMarkPullRequestReadyForReviewRejectsStaleOrAlreadyReadyBeforeGraphQL(t *testing.T) {
	for _, test := range []struct {
		name         string
		returnedHead string
		draft        bool
		want         string
	}{
		{name: "stale head", returnedHead: strings.Repeat("b", 40), draft: true, want: "head changed"},
		{name: "already ready", draft: false, want: "already ready"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, head, _ := newGitHubPullRequestTestService(t, "feature/ready-reject")
			enableReadyForTest(svc)
			graphqlCalled := false
			returnedHead := test.returnedHead
			if returnedHead == "" {
				returnedHead = head.String()
			}
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path == "/graphql" {
					graphqlCalled = true
				}
				return jsonHTTPResponse(http.StatusOK, readyRESTPull(returnedHead, test.draft)), nil
			})}
			_, err := svc.MarkPullRequestReadyForReview(context.Background(), "origin", 7, head.String())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if graphqlCalled {
				t.Fatal("GraphQL called after REST state rejection")
			}
		})
	}
}

func TestMarkPullRequestReadyForReviewRejectsChangedGraphQLStateBeforeMutation(t *testing.T) {
	svc, head, _ := newGitHubPullRequestTestService(t, "feature/ready-state")
	enableReadyForTest(svc)
	graphqlCalls := 0
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, readyRESTPull(head.String(), true)), nil
		case "/graphql":
			graphqlCalls++
			return jsonHTTPResponse(http.StatusOK, readyStateResponse(strings.Repeat("b", 40), true)), nil
		default:
			t.Fatalf("unexpected request: %s", request.URL.Path)
			return nil, nil
		}
	})}
	_, err := svc.MarkPullRequestReadyForReview(context.Background(), "origin", 7, head.String())
	if err == nil || !strings.Contains(err.Error(), "state changed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if graphqlCalls != 1 {
		t.Fatalf("GraphQL calls = %d, want preflight only", graphqlCalls)
	}
}

func TestMarkPullRequestReadyForReviewRequiresInspectionAfterAmbiguousMutation(t *testing.T) {
	for _, test := range []struct {
		name         string
		status       int
		mutationBody string
		want         string
	}{
		{name: "provider error", status: http.StatusInternalServerError, mutationBody: `{"message":"secret-provider-detail"}`, want: "outcome is unknown"},
		{name: "graphql error", status: http.StatusOK, mutationBody: `{"errors":[{}]}`, want: "outcome is unknown"},
		{name: "invalid success", status: http.StatusOK, mutationBody: `{"data":{"markPullRequestReadyForReview":{"pullRequest":null}}}`, want: "outcome is unknown"},
		{name: "wrong returned head", status: http.StatusOK, mutationBody: readyMutationResponse(strings.Repeat("b", 40), false), want: "outcome could not be validated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, head, _ := newGitHubPullRequestTestService(t, "feature/ready-ambiguous")
			enableReadyForTest(svc)
			graphqlCalls := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, readyRESTPull(head.String(), true)), nil
				case "/graphql":
					graphqlCalls++
					if graphqlCalls == 1 {
						return jsonHTTPResponse(http.StatusOK, readyStateResponse(head.String(), true)), nil
					}
					return jsonHTTPResponse(test.status, test.mutationBody), nil
				default:
					t.Fatalf("unexpected request: %s", request.URL.Path)
					return nil, nil
				}
			})}
			_, err := svc.MarkPullRequestReadyForReview(context.Background(), "origin", 7, head.String())
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "inspect the pull request before retrying") {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), "secret-provider-detail") {
				t.Fatalf("provider detail leaked: %v", err)
			}
		})
	}
}

func enableReadyForTest(svc *RemoteService) {
	remote := svc.remotes["origin"]
	remote.AllowPullRequestReady = true
	svc.remotes["origin"] = remote
	svc.githubPullRequestReadyEnabled = true
}

func readyRESTPull(head string, draft bool) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"number": 7, "html_url": "https://github.com/example/repo/pull/7", "title": "Ready",
		"draft": draft, "state": "open", "merged": false,
		"head": map[string]string{"ref": "feature/ready", "sha": head},
		"base": map[string]string{"ref": "main"},
	})
	return string(payload)
}

func readyStateResponse(head string, draft bool) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{"repository": map[string]interface{}{
			"nameWithOwner": "example/repo",
			"pullRequest": map[string]interface{}{
				"id": "PR_node_7", "number": 7, "isDraft": draft, "state": "OPEN", "merged": false,
				"headRefOid": head, "baseRefName": "main",
			},
		}},
	})
	return string(payload)
}

func readyMutationResponse(head string, draft bool) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{"markPullRequestReadyForReview": map[string]interface{}{
			"pullRequest": map[string]interface{}{
				"id": "PR_node_7", "number": 7, "isDraft": draft, "state": "OPEN", "merged": false,
				"headRefOid": head, "baseRefName": "main", "repository": map[string]string{"nameWithOwner": "example/repo"},
			},
		}},
	})
	return string(payload)
}
