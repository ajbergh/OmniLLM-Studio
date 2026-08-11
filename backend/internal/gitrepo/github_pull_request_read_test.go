package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGitHubPullRequestReadGateIsIndependentFromCreate(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	if !svc.GitHubPullRequestReadAccessEnabled() {
		t.Fatal("GitHubPullRequestReadAccessEnabled() = false with remote/read gates enabled")
	}
	if svc.GitHubPullRequestMutationEnabled() {
		t.Fatal("GitHubPullRequestMutationEnabled() unexpectedly enabled by read gate")
	}
	summaries := svc.Remotes(context.Background())
	if len(summaries) != 1 || !summaries[0].PullRequestReadAllowed || summaries[0].PullRequestCreateAllowed {
		t.Fatalf("unexpected remote summary: %#v", summaries)
	}
}

func TestGetPullRequestUsesOperatorBoundRepositoryAndToken(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || request.URL.Host != "api.github.com" || request.URL.Path != "/repos/example/repo/pulls/42" {
			t.Fatalf("unexpected request URL: %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		return jsonHTTPResponse(http.StatusOK, `{"number":42,"html_url":"https://github.com/example/repo/pull/42","title":"Inspect me","draft":true,"state":"open","merged":false,"mergeable":true,"mergeable_state":"clean","updated_at":"2026-08-10T23:00:00Z","user":{"login":"octocat"},"head":{"ref":"feature/read","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"base":{"ref":"main"}}`), nil
	})}

	result, err := svc.GetPullRequest(context.Background(), "origin", 42)
	if err != nil {
		t.Fatalf("GetPullRequest() returned error: %v", err)
	}
	if result.Number != 42 || result.Repository != "repo" || result.HeadBranch != "feature/read" || result.Head != strings.Repeat("a", 40) || result.BaseBranch != "main" || result.Mergeable == nil || !*result.Mergeable {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestListPullRequestsIsBoundedAndUsesSameRepositoryHeadFilter(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/repos/example/repo/pulls" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("state") != "open" || query.Get("sort") != "updated" || query.Get("direction") != "desc" || query.Get("head") != "example:feature/read" || query.Get("per_page") != "3" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		return jsonHTTPResponse(http.StatusOK, `[
			{"number":3,"html_url":"https://github.com/example/repo/pull/3","title":"three","state":"open","head":{"ref":"feature/read","sha":"cccccccccccccccccccccccccccccccccccccccc"},"base":{"ref":"main"}},
			{"number":2,"html_url":"https://github.com/example/repo/pull/2","title":"two","state":"open","head":{"ref":"feature/read","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"base":{"ref":"main"}},
			{"number":1,"html_url":"https://github.com/example/repo/pull/1","title":"one","state":"open","head":{"ref":"feature/read","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"base":{"ref":"main"}}
		]`), nil
	})}

	result, err := svc.ListPullRequests(context.Background(), "origin", "open", "feature/read", 2)
	if err != nil {
		t.Fatalf("ListPullRequests() returned error: %v", err)
	}
	if !result.Truncated || len(result.PullRequests) != 2 || result.PullRequests[0].Number != 3 || result.PullRequests[1].Number != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGetPullRequestChecksBindsQueriesToFetchedHead(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("d", 40)
	requests := make([]string, 0, 3)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Checks","state":"open","head":{"ref":"feature/checks","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/commits/" + head + "/check-runs":
			if request.URL.Query().Get("filter") != "latest" || request.URL.Query().Get("per_page") != "51" {
				t.Fatalf("unexpected check query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":51,"check_runs":[{"name":"Quality Gate","status":"completed","conclusion":"success","details_url":"https://untrusted.example/details","app":{"slug":"github-actions","name":"GitHub Actions"}}]}`), nil
		case "/repos/example/repo/commits/" + head + "/status":
			if request.URL.Query().Get("per_page") != "51" {
				t.Fatalf("unexpected status query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"state":"pending","total_count":51,"statuses":[{"context":"security","state":"success","description":"untrusted text","target_url":"https://untrusted.example/status"}]}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestChecks(context.Background(), "origin", 7)
	if err != nil {
		t.Fatalf("GetPullRequestChecks() returned error: %v", err)
	}
	if result.Head != head || result.CombinedStatus != "pending" || len(result.CheckRuns) != 1 || result.CheckRuns[0].Name != "Quality Gate" || len(result.CommitStatuses) != 1 || result.CommitStatuses[0].Context != "security" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !result.CheckRunsTruncated || !result.CommitStatusesTruncated {
		t.Fatalf("expected bounded-result truncation flags: %#v", result)
	}
	if len(requests) != 3 || requests[1] != "/repos/example/repo/commits/"+head+"/check-runs" || requests[2] != "/repos/example/repo/commits/"+head+"/status" {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestGitHubPullRequestReadRejectsUnallowedRemoteBeforeAPI(t *testing.T) {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN"},
	}, true, false, nil, func(string) (string, bool) { return "test-token", true })
	svc.githubPullRequestReadEnabled = true
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	})}
	_, err := svc.GetPullRequest(context.Background(), "origin", 1)
	if err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("GetPullRequest() error = %v", err)
	}
	if called {
		t.Fatal("GitHub API was called for a remote without allow_pull_request_read")
	}
}

func TestGitHubPullRequestReadDoesNotExposeAPIErrorBody(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusForbidden, `{"message":"super-secret-provider-detail"}`), nil
	})}
	_, err := svc.GetPullRequest(context.Background(), "origin", 5)
	if err == nil {
		t.Fatal("GetPullRequest() unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "super-secret-provider-detail") {
		t.Fatalf("API error body leaked into error: %v", err)
	}
}

func newGitHubPullRequestReadTestService() *RemoteService {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {
			Repository: "repo", URL: "https://github.com/example/repo.git", Username: "git",
			TokenEnv: "GITHUB_TOKEN", AllowPullRequestRead: true,
		},
	}, true, false, nil, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.githubPullRequestReadEnabled = true
	return svc
}
