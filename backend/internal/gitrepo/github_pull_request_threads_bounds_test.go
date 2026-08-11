package gitrepo

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestReviewThreadsRejectsProviderPageBeyondRequestedLimit(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("f", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/11":
			return jsonHTTPResponse(http.StatusOK, `{"number":11,"html_url":"https://github.com/example/repo/pull/11","title":"Threads","state":"open","head":{"ref":"feature/threads","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/graphql":
			nodes, _ := json.Marshal([]map[string]interface{}{
				{"id": "PRRT_one", "path": "one.go", "diffSide": "RIGHT", "subjectType": "LINE"},
				{"id": "PRRT_two", "path": "two.go", "diffSide": "RIGHT", "subjectType": "LINE"},
			})
			return jsonHTTPResponse(http.StatusOK, `{"data":{"repository":{"pullRequest":{"headRefOid":"`+head+`","reviewThreads":{"totalCount":2,"nodes":`+string(nodes)+`,"pageInfo":{"hasNextPage":false}}}}}}}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	_, err := svc.GetPullRequestReviewThreads(context.Background(), "origin", 11, "", 1)
	if err == nil || !strings.Contains(err.Error(), "exceeded its validated bounds") {
		t.Fatalf("unexpected error: %v", err)
	}
}
