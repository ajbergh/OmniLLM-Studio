package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestCheckDiagnosticsBindsToFetchedHeadAndBoundsProviderText(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("e", 40)
	requests := make([]string, 0, 4)
	longMessage := strings.Repeat("x", maxGitHubDiagnosticMessageRunes+100)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/repos/example/repo/pulls/11":
			return jsonHTTPResponse(http.StatusOK, `{"number":11,"html_url":"https://github.com/example/repo/pull/11","title":"Diagnostics","state":"open","head":{"ref":"feature/diagnostics","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/commits/" + head + "/check-runs":
			if request.URL.Query().Get("filter") != "latest" || request.URL.Query().Get("per_page") != "51" {
				t.Fatalf("unexpected check query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":2,"check_runs":[{"id":101,"name":"Quality Gate","status":"completed","conclusion":"failure","app":{"slug":"github-actions"}},{"id":102,"name":"Security Scan","status":"completed","conclusion":"success","app":{"slug":"github-actions"}}]}`), nil
		case "/repos/example/repo/check-runs/101/annotations":
			if request.URL.Query().Get("per_page") != "21" {
				t.Fatalf("unexpected annotation query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `[{"path":"backend/internal/example.go","start_line":7,"end_line":7,"annotation_level":"failure","title":"compile error","message":"`+longMessage+`"}]`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestCheckDiagnostics(context.Background(), "origin", 11)
	if err != nil {
		t.Fatalf("GetPullRequestCheckDiagnostics() returned error: %v", err)
	}
	if result.Head != head || result.PullRequest != 11 || len(result.Checks) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	check := result.Checks[0]
	if check.Name != "Quality Gate" || check.Conclusion != "failure" || check.App != "github-actions" || len(check.Annotations) != 1 {
		t.Fatalf("unexpected check diagnostics: %#v", check)
	}
	annotation := check.Annotations[0]
	if annotation.Path != "backend/internal/example.go" || annotation.StartLine != 7 || annotation.Level != "failure" || len([]rune(annotation.Message)) != maxGitHubDiagnosticMessageRunes {
		t.Fatalf("unexpected annotation: %#v", annotation)
	}
	if len(requests) != 3 || requests[1] != "/repos/example/repo/commits/"+head+"/check-runs" || requests[2] != "/repos/example/repo/check-runs/101/annotations" {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestGetPullRequestCheckDiagnosticsRejectsUnallowedRemoteBeforeAPI(t *testing.T) {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN"},
	}, true, false, nil, func(string) (string, bool) { return "test-token", true })
	svc.githubPullRequestReadEnabled = true
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	})}
	_, err := svc.GetPullRequestCheckDiagnostics(context.Background(), "origin", 1)
	if err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("GetPullRequestCheckDiagnostics() error = %v", err)
	}
	if called {
		t.Fatal("GitHub API was called for a remote without allow_pull_request_read")
	}
}

func TestGetPullRequestCheckDiagnosticsDoesNotExposeAPIErrorBody(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "/pulls/") {
			return jsonHTTPResponse(http.StatusOK, `{"number":4,"html_url":"https://github.com/example/repo/pull/4","title":"Diagnostics","state":"open","head":{"ref":"feature/diagnostics","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"base":{"ref":"main"}}`), nil
		}
		return jsonHTTPResponse(http.StatusForbidden, `{"message":"provider-secret-detail"}`), nil
	})}
	_, err := svc.GetPullRequestCheckDiagnostics(context.Background(), "origin", 4)
	if err == nil {
		t.Fatal("GetPullRequestCheckDiagnostics() unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "provider-secret-detail") {
		t.Fatalf("API error body leaked into error: %v", err)
	}
}

func TestGitHubCheckNeedsDiagnostics(t *testing.T) {
	for _, test := range []struct {
		status     string
		conclusion string
		want       bool
	}{
		{status: "completed", conclusion: "failure", want: true},
		{status: "completed", conclusion: "cancelled", want: true},
		{status: "completed", conclusion: "success", want: false},
		{status: "completed", conclusion: "neutral", want: false},
		{status: "queued", conclusion: "", want: false},
	} {
		if got := githubCheckNeedsDiagnostics(test.status, test.conclusion); got != test.want {
			t.Fatalf("githubCheckNeedsDiagnostics(%q, %q) = %v, want %v", test.status, test.conclusion, got, test.want)
		}
	}
}
