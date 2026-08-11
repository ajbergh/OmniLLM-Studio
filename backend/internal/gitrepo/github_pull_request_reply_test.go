package gitrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGitHubPullRequestReplyGateIsIndependent(t *testing.T) {
	svc := newGitHubPullRequestReplyTestService()
	if !svc.GitHubPullRequestReplyMutationEnabled() {
		t.Fatal("GitHubPullRequestReplyMutationEnabled() = false with reply gate enabled")
	}
	if svc.GitHubPullRequestReadAccessEnabled() || svc.GitHubPullRequestMutationEnabled() || svc.PushMutationEnabled() {
		t.Fatal("review reply gate unexpectedly enabled another Git/GitHub capability")
	}
	summaries := svc.Remotes(context.Background())
	if len(summaries) != 1 || !summaries[0].PullRequestReplyAllowed || summaries[0].PullRequestReadAllowed || summaries[0].PullRequestCreateAllowed || summaries[0].PushAllowed {
		t.Fatalf("unexpected remote summary: %#v", summaries)
	}
}

func TestReplyToPullRequestReviewCommentRevalidatesStateAndPostsBoundedBody(t *testing.T) {
	svc := newGitHubPullRequestReplyTestService()
	head := strings.Repeat("a", 40)
	updatedAt := "2026-08-11T10:00:00Z"
	paths := make([]string, 0, 3)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"open","head":{"ref":"feature/reply","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "GET /repos/example/repo/pulls/comments/91":
			return jsonHTTPResponse(http.StatusOK, `{"id":91,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","updated_at":"`+updatedAt+`"}`), nil
		case "POST /repos/example/repo/pulls/7/comments/91/replies":
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read reply body: %v", err)
			}
			var decoded map[string]string
			if err := json.Unmarshal(payload, &decoded); err != nil || decoded["body"] != "Addressed in the latest commit." || len(decoded) != 1 {
				t.Fatalf("unexpected reply payload: %s (%v)", payload, err)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"id":92,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","in_reply_to_id":91,"created_at":"2026-08-11T10:05:00Z"}`), nil
		default:
			t.Fatalf("unexpected GitHub API request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, head, 91, 44, updatedAt, "Addressed in the latest commit.")
	if err != nil {
		t.Fatalf("ReplyToPullRequestReviewComment() returned error: %v", err)
	}
	if !result.Posted || result.Head != head || result.ParentCommentID != 91 || result.ReviewID != 44 || result.ReplyID != 92 || result.Repository != "repo" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(paths) != 3 {
		t.Fatalf("request sequence = %#v", paths)
	}
}

func TestReplyToPullRequestReviewCommentRejectsClosedPullRequestBeforeCommentLookup(t *testing.T) {
	svc := newGitHubPullRequestReplyTestService()
	head := strings.Repeat("a", 40)
	calls := 0
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Path != "/repos/example/repo/pulls/7" {
			t.Fatalf("unexpected request after closed PR: %s", request.URL.Path)
		}
		return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"closed","head":{"ref":"feature/reply","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
	})}

	_, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, head, 91, 44, "2026-08-11T10:00:00Z", "reply")
	if err == nil || !strings.Contains(err.Error(), "no longer open") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("GitHub calls = %d, want 1", calls)
	}
}

func TestReplyToPullRequestReviewCommentRejectsStaleHeadBeforeCommentLookup(t *testing.T) {
	svc := newGitHubPullRequestReplyTestService()
	expectedHead := strings.Repeat("a", 40)
	currentHead := strings.Repeat("b", 40)
	calls := 0
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Path != "/repos/example/repo/pulls/7" {
			t.Fatalf("unexpected request after stale head: %s", request.URL.Path)
		}
		return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"open","head":{"ref":"feature/reply","sha":"`+currentHead+`"},"base":{"ref":"main"}}`), nil
	})}

	_, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, expectedHead, 91, 44, "2026-08-11T10:00:00Z", "reply")
	if err == nil || !strings.Contains(err.Error(), "head changed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("GitHub calls = %d, want 1", calls)
	}
}

func TestReplyToPullRequestReviewCommentRejectsEditedOrNestedCommentBeforePost(t *testing.T) {
	for _, test := range []struct {
		name    string
		comment string
		want    string
	}{
		{name: "edited", comment: `{"id":91,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","updated_at":"2026-08-11T10:01:00Z"}`, want: "comment changed"},
		{name: "nested", comment: `{"id":91,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","in_reply_to_id":80,"updated_at":"2026-08-11T10:00:00Z"}`, want: "top-level"},
		{name: "wrong review", comment: `{"id":91,"pull_request_review_id":45,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","updated_at":"2026-08-11T10:00:00Z"}`, want: "identity changed"},
		{name: "wrong pr", comment: `{"id":91,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/8","updated_at":"2026-08-11T10:00:00Z"}`, want: "identity changed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestReplyTestService()
			head := strings.Repeat("a", 40)
			posts := 0
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.Method + " " + request.URL.Path {
				case "GET /repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"open","head":{"ref":"feature/reply","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
				case "GET /repos/example/repo/pulls/comments/91":
					return jsonHTTPResponse(http.StatusOK, test.comment), nil
				default:
					posts++
					return jsonHTTPResponse(http.StatusCreated, `{}`), nil
				}
			})}
			_, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, head, 91, 44, "2026-08-11T10:00:00Z", "reply")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if posts != 0 {
				t.Fatalf("reply POST attempted after failed revalidation")
			}
		})
	}
}

func TestReplyToPullRequestReviewCommentRequiresInspectionAfterAmbiguousPostOutcome(t *testing.T) {
	for _, test := range []struct {
		name       string
		postStatus int
		postBody   string
		want       string
	}{
		{name: "provider error", postStatus: http.StatusInternalServerError, postBody: `{"message":"secret-provider-detail"}`, want: "outcome is unknown"},
		{name: "invalid success response", postStatus: http.StatusCreated, postBody: `{}`, want: "outcome could not be validated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := newGitHubPullRequestReplyTestService()
			head := strings.Repeat("a", 40)
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.Method + " " + request.URL.Path {
				case "GET /repos/example/repo/pulls/7":
					return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"open","head":{"ref":"feature/reply","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
				case "GET /repos/example/repo/pulls/comments/91":
					return jsonHTTPResponse(http.StatusOK, `{"id":91,"pull_request_review_id":44,"pull_request_url":"https://api.github.com/repos/example/repo/pulls/7","updated_at":"2026-08-11T10:00:00Z"}`), nil
				case "POST /repos/example/repo/pulls/7/comments/91/replies":
					return jsonHTTPResponse(test.postStatus, test.postBody), nil
				default:
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			})}
			_, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, head, 91, 44, "2026-08-11T10:00:00Z", "reply")
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "inspect feedback before retrying") {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(err.Error(), "secret-provider-detail") {
				t.Fatalf("provider error body leaked: %v", err)
			}
		})
	}
}

func TestReplyToPullRequestReviewCommentDoesNotExposeProviderErrorBody(t *testing.T) {
	svc := newGitHubPullRequestReplyTestService()
	head := strings.Repeat("a", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Reply","state":"open","head":{"ref":"feature/reply","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo/pulls/comments/91":
			return jsonHTTPResponse(http.StatusForbidden, `{"message":"secret-provider-detail"}`), nil
		default:
			t.Fatalf("unexpected request: %s", request.URL.Path)
			return nil, nil
		}
	})}
	_, err := svc.ReplyToPullRequestReviewComment(context.Background(), "origin", 7, head, 91, 44, "2026-08-11T10:00:00Z", "reply")
	if err == nil || strings.Contains(err.Error(), "secret-provider-detail") {
		t.Fatalf("provider error body leaked: %v", err)
	}
}

func newGitHubPullRequestReplyTestService() *RemoteService {
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {
			Repository: "repo", URL: "https://github.com/example/repo.git", Username: "git",
			TokenEnv: "GITHUB_TOKEN", AllowPullRequestReply: true,
		},
	}, true, false, nil, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.githubPullRequestReplyEnabled = true
	return svc
}
