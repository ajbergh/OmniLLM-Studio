package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestInspectGitHubStrictBaseCurrencyTreatsBehindAsKnownUnsatisfied(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	base := strings.Repeat("a", 40)
	head := strings.Repeat("b", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/git/ref/heads/main":
			return jsonHTTPResponse(http.StatusOK, `{"ref":"refs/heads/main","object":{"sha":"`+base+`"}}`), nil
		case "/repos/example/repo/compare/" + base + "..." + head:
			return jsonHTTPResponse(http.StatusOK, `{"status":"behind"}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	current, complete := svc.inspectGitHubStrictBaseCurrency(context.Background(), "test-token", "example", "repo", "main", head)
	if !complete || current {
		t.Fatalf("strict base currency = current %v complete %v, want false true", current, complete)
	}
}

func TestInspectGitHubRequiredChecksFailsClosedWhenCheckRunsAreTruncated(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("c", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/commits/" + head + "/check-runs":
			return jsonHTTPResponse(http.StatusOK, `{"total_count":51,"check_runs":[{"name":"Quality Gate","status":"completed","conclusion":"success","app":{"id":15368}}]}`), nil
		case "/repos/example/repo/commits/" + head + "/status":
			return jsonHTTPResponse(http.StatusOK, `{"state":"success","total_count":0,"statuses":[]}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	states, satisfied, complete := svc.inspectGitHubRequiredChecks(context.Background(), "test-token", "example", "repo", head, []GitHubRequiredStatusCheck{{Context: "Quality Gate"}})
	if complete || satisfied || states != nil {
		t.Fatalf("truncated required-check evidence = states %#v satisfied %v complete %v", states, satisfied, complete)
	}
}

func TestInspectGitHubRequiredDeploymentsRejectsEnvironmentBoundBeforeNetwork(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `[]`), nil
	})}
	environments := make([]string, maxGitHubRequiredDeployments+1)
	for i := range environments {
		environments[i] = "environment-" + string(rune('a'+i))
	}

	states, satisfied, complete := svc.inspectGitHubRequiredDeployments(context.Background(), "test-token", "example", "repo", strings.Repeat("d", 40), environments)
	if complete || satisfied || states != nil || called {
		t.Fatalf("deployment bound = states %#v satisfied %v complete %v called %v", states, satisfied, complete, called)
	}
}

func TestInspectGitHubRequiredSignaturesTreatsFullPageAsIncomplete(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	commit := `{"sha":"` + strings.Repeat("e", 40) + `","commit":{"verification":{"verified":true}}}`
	body := "[" + strings.TrimSuffix(strings.Repeat(commit+",", maxGitHubSignatureEvidenceCommits), ",") + "]"
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/repos/example/repo/pulls/21/commits" || request.URL.Query().Get("per_page") != "100" {
			t.Fatalf("unexpected request: %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		return jsonHTTPResponse(http.StatusOK, body), nil
	})}

	satisfied, complete := svc.inspectGitHubRequiredSignatures(context.Background(), "test-token", "example", "repo", 21)
	if complete || satisfied {
		t.Fatalf("full signature page = satisfied %v complete %v, want false false", satisfied, complete)
	}
}
