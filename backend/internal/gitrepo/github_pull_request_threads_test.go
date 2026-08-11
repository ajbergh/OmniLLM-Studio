package gitrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestReviewThreadsUsesFixedGraphQLQueryAndOpaqueCursorVariable(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("a", 40)
	cursor := "opaque-cursor-value"
	paths := make([]string, 0, 2)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "POST /graphql":
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read GraphQL request: %v", err)
			}
			var decoded struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode GraphQL request: %v", err)
			}
			if decoded.Query != githubPullRequestReviewThreadsQuery || strings.Contains(decoded.Query, cursor) {
				t.Fatalf("GraphQL query was not fixed: %q", decoded.Query)
			}
			if decoded.Variables["owner"] != "example" || decoded.Variables["repository"] != "repo" || decoded.Variables["after"] != cursor || decoded.Variables["first"] != float64(2) || decoded.Variables["number"] != float64(7) {
				t.Fatalf("unexpected GraphQL variables: %#v", decoded.Variables)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"repository":{"pullRequest":{"headRefOid":"`+head+`","reviewThreads":{"totalCount":3,"nodes":[{"id":"PRRT_thread_1","isResolved":false,"isOutdated":false,"isCollapsed":false,"path":"backend/main.go","line":21,"startLine":20,"diffSide":"RIGHT","startDiffSide":"RIGHT","subjectType":"LINE","viewerCanReply":true,"viewerCanResolve":true,"viewerCanUnresolve":false},{"id":"PRRT_thread_2","isResolved":true,"isOutdated":true,"isCollapsed":true,"path":"frontend/app.tsx","diffSide":"LEFT","subjectType":"FILE","resolvedBy":{"login":"reviewer"},"viewerCanReply":true,"viewerCanResolve":false,"viewerCanUnresolve":true}],"pageInfo":{"hasNextPage":true,"endCursor":"next-cursor"}}}}}}}`), nil
		default:
			t.Fatalf("unexpected GitHub request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", 7, cursor, 2)
	if err != nil {
		t.Fatalf("GetPullRequestReviewThreads() returned error: %v", err)
	}
	if len(paths) != 2 || result.Head != head || result.TotalCount != 3 || result.Limit != 2 || !result.HasNextPage || result.NextCursor != "next-cursor" || len(result.Threads) != 2 {
		t.Fatalf("unexpected result: %#v paths=%#v", result, paths)
	}
	first := result.Threads[0]
	if first.ID != "PRRT_thread_1" || first.IsResolved || first.IsOutdated || first.Path != "backend/main.go" || first.Line == nil || *first.Line != 21 || first.StartLine == nil || *first.StartLine != 20 || !first.ViewerCanResolve {
		t.Fatalf("unexpected first thread: %#v", first)
	}
	second := result.Threads[1]
	if !second.IsResolved || !second.IsOutdated || !second.IsCollapsed || second.ResolvedBy != "reviewer" || !second.ViewerCanUnresolve || second.SubjectType != "FILE" {
		t.Fatalf("unexpected second thread: %#v", second)
	}
}

func TestGetPullRequestReviewThreadsDefaultsFirstPageAndBoundsPath(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("b", 40)
	longPath := strings.Repeat("p", maxGitHubFeedbackPathBytes+100)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/8":
			return jsonHTTPResponse(http.StatusOK, `{"number":8,"html_url":"https://github.com/example/repo/pull/8","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/graphql":
			payload, _ := io.ReadAll(request.Body)
			var decoded struct {
				Variables map[string]interface{} `json:"variables"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode GraphQL request: %v", err)
			}
			if decoded.Variables["after"] != nil || decoded.Variables["first"] != float64(defaultGitHubReviewThreadLimit) {
				t.Fatalf("unexpected default variables: %#v", decoded.Variables)
			}
			response, _ := json.Marshal(map[string]interface{}{
				"data": map[string]interface{}{"repository": map[string]interface{}{"pullRequest": map[string]interface{}{
					"headRefOid": head,
					"reviewThreads": map[string]interface{}{
						"totalCount": 1,
						"nodes": []map[string]interface{}{{"id": "PRRT_thread", "path": longPath, "diffSide": "RIGHT", "subjectType": "LINE"}},
						"pageInfo": map[string]interface{}{"hasNextPage": false, "endCursor": nil},
					},
				}}},
			})
			return jsonHTTPResponse(http.StatusOK, string(response)), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", 8, "", 0)
	if err != nil {
		t.Fatalf("GetPullRequestReviewThreads() returned error: %v", err)
	}
	if len(result.Threads) != 1 || !result.Threads[0].PathTruncated || len(result.Threads[0].Path) > maxGitHubFeedbackPathBytes || result.NextCursor != "" || result.HasNextPage {
		t.Fatalf("unexpected bounded result: %#v", result)
	}
}

func TestGetPullRequestReviewThreadsRejectsInvalidBoundsBeforeNetwork(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	})}
	for _, test := range []struct {
		number int
		after  string
		limit  int
	}{
		{number: 0, limit: 10},
		{number: 7, after: strings.Repeat("x", maxGitHubGraphQLCursorBytes+1), limit: 10},
		{number: 7, limit: 21},
		{number: 7, limit: -1},
	} {
		if _, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", test.number, test.after, test.limit); err == nil {
			t.Fatalf("invalid request unexpectedly succeeded: %#v", test)
		}
	}
	if called {
		t.Fatal("GitHub API was called for invalid review thread bounds")
	}
}

func TestGetPullRequestReviewThreadsRejectsHeadRace(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	restHead := strings.Repeat("c", 40)
	graphHead := strings.Repeat("d", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/9":
			return jsonHTTPResponse(http.StatusOK, `{"number":9,"html_url":"https://github.com/example/repo/pull/9","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+restHead+`"},"base":{"ref":"main"}}`), nil
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"repository":{"pullRequest":{"headRefOid":"`+graphHead+`","reviewThreads":{"totalCount":0,"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}
	_, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", 9, "", 10)
	if err == nil || !strings.Contains(err.Error(), "head changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPullRequestReviewThreadsDoesNotExposeGraphQLErrorDetails(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("e", 40)
	for _, test := range []struct {
		name       string
		status     int
		graphReply string
	}{
		{name: "http error", status: http.StatusForbidden, graphReply: `{"message":"secret-provider-detail"}`},
		{name: "graphql error", status: http.StatusOK, graphReply: `{"errors":[{"message":"secret-provider-detail"}],"data":{"repository":null}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/repos/example/repo/pulls/10":
					return jsonHTTPResponse(http.StatusOK, `{"number":10,"html_url":"https://github.com/example/repo/pull/10","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
				case "/graphql":
					return jsonHTTPResponse(test.status, test.graphReply), nil
				default:
					t.Fatalf("unexpected path: %s", request.URL.Path)
					return nil, nil
				}
			})}
			_, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", 10, "", 10)
			if err == nil || strings.Contains(err.Error(), "secret-provider-detail") {
				t.Fatalf("GraphQL error details leaked: %v", err)
			}
		})
	}
}
