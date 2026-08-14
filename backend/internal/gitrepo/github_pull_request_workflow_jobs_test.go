package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestWorkflowJobsBindsRunsAndJobsToFetchedHead(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("b", 40)
	requests := make([]string, 0, 4)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.RequestURI())
		switch request.URL.Path {
		case "/repos/example/repo/pulls/12":
			return jsonHTTPResponse(http.StatusOK, `{"number":12,"html_url":"https://github.com/example/repo/pull/12","title":"Workflow metadata","state":"open","head":{"ref":"feature/jobs","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/actions/runs":
			if request.URL.Query().Get("head_sha") != head || request.URL.Query().Get("per_page") != "11" {
				t.Fatalf("unexpected workflow run query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":1,"workflow_runs":[{"id":101,"name":"Quality Gate","event":"pull_request","status":"completed","conclusion":"failure","run_number":44,"run_attempt":2,"head_sha":"`+head+`","created_at":"2026-08-14T00:00:00Z","updated_at":"2026-08-14T00:01:00Z"}]}`), nil
		case "/repos/example/repo/actions/runs/101/jobs":
			if request.URL.Query().Get("filter") != "latest" || request.URL.Query().Get("per_page") != "21" {
				t.Fatalf("unexpected workflow job query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":1,"jobs":[{"id":202,"name":"backend-unit","status":"completed","conclusion":"failure","started_at":"2026-08-14T00:00:05Z","completed_at":"2026-08-14T00:00:30Z","runner_name":"must-not-cross-boundary","steps":[{"name":"Checkout","status":"completed","conclusion":"success","number":1},{"name":"go test","status":"completed","conclusion":"failure","number":2}]}]}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestWorkflowJobs(context.Background(), "origin", 12)
	if err != nil {
		t.Fatalf("GetPullRequestWorkflowJobs() returned error: %v", err)
	}
	if result.Head != head || result.PullRequest != 12 || len(result.WorkflowRuns) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	run := result.WorkflowRuns[0]
	if run.Name != "Quality Gate" || run.RunNumber != 44 || run.Attempt != 2 || len(run.Jobs) != 1 {
		t.Fatalf("unexpected run: %#v", run)
	}
	job := run.Jobs[0]
	if job.Name != "backend-unit" || job.Conclusion != "failure" || len(job.Steps) != 2 || job.Steps[1].Name != "go test" {
		t.Fatalf("unexpected job: %#v", job)
	}
	if len(requests) != 3 || !strings.Contains(requests[1], "head_sha="+head) || !strings.Contains(requests[2], "/actions/runs/101/jobs") {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestGetPullRequestWorkflowJobsRejectsProviderHeadMismatch(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("c", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/3":
			return jsonHTTPResponse(http.StatusOK, `{"number":3,"html_url":"https://github.com/example/repo/pull/3","title":"Mismatch","state":"open","head":{"ref":"feature/mismatch","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/actions/runs":
			return jsonHTTPResponse(http.StatusOK, `{"total_count":1,"workflow_runs":[{"id":9,"name":"CI","status":"completed","head_sha":"dddddddddddddddddddddddddddddddddddddddd"}]}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}
	_, err := svc.GetPullRequestWorkflowJobs(context.Background(), "origin", 3)
	if err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("GetPullRequestWorkflowJobs() error = %v", err)
	}
}

func TestGetPullRequestWorkflowJobsRejectsUnallowedRemoteBeforeAPI(t *testing.T) {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN"},
	}, true, false, nil, func(string) (string, bool) { return "test-token", true })
	svc.githubPullRequestReadEnabled = true
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	})}
	_, err := svc.GetPullRequestWorkflowJobs(context.Background(), "origin", 1)
	if err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("GetPullRequestWorkflowJobs() error = %v", err)
	}
	if called {
		t.Fatal("GitHub API was called for a remote without allow_pull_request_read")
	}
}

func TestGetPullRequestWorkflowJobsDoesNotExposeAPIErrorBody(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "/pulls/") {
			return jsonHTTPResponse(http.StatusOK, `{"number":5,"html_url":"https://github.com/example/repo/pull/5","title":"Metadata","state":"open","head":{"ref":"feature/jobs","sha":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},"base":{"ref":"main"}}`), nil
		}
		return jsonHTTPResponse(http.StatusForbidden, `{"message":"provider-secret-detail"}`), nil
	})}
	_, err := svc.GetPullRequestWorkflowJobs(context.Background(), "origin", 5)
	if err == nil {
		t.Fatal("GetPullRequestWorkflowJobs() unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "provider-secret-detail") {
		t.Fatalf("API error body leaked into error: %v", err)
	}
}

func TestGetPullRequestWorkflowJobsBoundsRunsJobsAndSteps(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("f", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/8":
			return jsonHTTPResponse(http.StatusOK, `{"number":8,"html_url":"https://github.com/example/repo/pull/8","title":"Bounds","state":"open","head":{"ref":"feature/bounds","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/actions/runs":
			return jsonHTTPResponse(http.StatusOK, `{"total_count":11,"workflow_runs":[{"id":1,"name":"CI","status":"completed","head_sha":"`+head+`"}]}`), nil
		case "/repos/example/repo/actions/runs/1/jobs":
			steps := make([]string, 0, maxGitHubStepsPerJob+1)
			for i := 0; i < maxGitHubStepsPerJob+1; i++ {
				steps = append(steps, `{"name":"step","status":"completed","conclusion":"success","number":`+string(rune('1'+i%9))+`}`)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":1,"jobs":[{"id":2,"name":"job","status":"completed","conclusion":"failure","steps":[`+strings.Join(steps, ",")+`]}]}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}
	result, err := svc.GetPullRequestWorkflowJobs(context.Background(), "origin", 8)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RunsTruncated || !result.StepsTruncated || len(result.WorkflowRuns) != 1 || len(result.WorkflowRuns[0].Jobs) != 1 || len(result.WorkflowRuns[0].Jobs[0].Steps) != maxGitHubStepsPerJob {
		t.Fatalf("unexpected bounds result: %#v", result)
	}
}
