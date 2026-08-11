package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	defaultGitHubReviewThreadLimit = 10
	maxGitHubReviewThreadLimit     = 20
	maxGitHubGraphQLCursorBytes    = 512
	maxGitHubGraphQLNodeIDBytes    = 256
)

const githubPullRequestReviewThreadsQuery = `
query PullRequestReviewThreads($owner: String!, $repository: String!, $number: Int!, $first: Int!, $after: String) {
  repository(owner: $owner, name: $repository) {
    pullRequest(number: $number) {
      headRefOid
      reviewThreads(first: $first, after: $after) {
        totalCount
        nodes {
          id
          isResolved
          isOutdated
          isCollapsed
          path
          line
          startLine
          diffSide
          startDiffSide
          subjectType
          resolvedBy { login }
          viewerCanReply
          viewerCanResolve
          viewerCanUnresolve
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`

// GitHubPullRequestReviewThreadReader exposes bounded thread state without any
// thread mutation or reviewer prose. Repository identity and credentials remain
// operator-derived and the current PR head is fetched before the GraphQL read.
type GitHubPullRequestReviewThreadReader interface {
	GetPullRequestReviewThreads(ctx context.Context, remoteID string, number int, after string, limit int) (*GitHubPullRequestReviewThreadsResult, error)
}

// GitHubPullRequestReviewThreadsResult is one cursor-bounded page of review
// thread state. NextCursor is an opaque GitHub cursor suitable only for the next
// call to this same read tool.
type GitHubPullRequestReviewThreadsResult struct {
	Remote      string                                `json:"remote"`
	Repository  string                                `json:"repository"`
	PullRequest int                                   `json:"pull_request"`
	Head        string                                `json:"head"`
	Limit       int                                   `json:"limit"`
	TotalCount  int                                   `json:"total_count"`
	Threads     []GitHubPullRequestReviewThreadResult `json:"threads"`
	HasNextPage bool                                  `json:"has_next_page"`
	NextCursor  string                                `json:"next_cursor,omitempty"`
}

// GitHubPullRequestReviewThreadResult intentionally contains state/location
// metadata only. Review bodies remain available through the separately bounded
// feedback tool and continue to pass through the untrusted-result boundary.
type GitHubPullRequestReviewThreadResult struct {
	ID                 string `json:"id"`
	IsResolved         bool   `json:"is_resolved"`
	IsOutdated         bool   `json:"is_outdated"`
	IsCollapsed        bool   `json:"is_collapsed"`
	Path               string `json:"path"`
	PathTruncated      bool   `json:"path_truncated,omitempty"`
	Line               *int   `json:"line,omitempty"`
	StartLine          *int   `json:"start_line,omitempty"`
	DiffSide           string `json:"diff_side"`
	StartDiffSide      string `json:"start_diff_side,omitempty"`
	SubjectType        string `json:"subject_type"`
	ResolvedBy         string `json:"resolved_by,omitempty"`
	ViewerCanReply     bool   `json:"viewer_can_reply"`
	ViewerCanResolve   bool   `json:"viewer_can_resolve"`
	ViewerCanUnresolve bool   `json:"viewer_can_unresolve"`
}

type githubGraphQLError struct{}

type githubPullRequestReviewThreadsGraphQLResponse struct {
	Data struct {
		Repository *struct {
			PullRequest *struct {
				HeadRefOID    string `json:"headRefOid"`
				ReviewThreads struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						ID            string `json:"id"`
						IsResolved    bool   `json:"isResolved"`
						IsOutdated    bool   `json:"isOutdated"`
						IsCollapsed   bool   `json:"isCollapsed"`
						Path          string `json:"path"`
						Line          *int   `json:"line"`
						StartLine     *int   `json:"startLine"`
						DiffSide      string `json:"diffSide"`
						StartDiffSide string `json:"startDiffSide"`
						SubjectType   string `json:"subjectType"`
						ResolvedBy    *struct {
							Login string `json:"login"`
						} `json:"resolvedBy"`
						ViewerCanReply     bool `json:"viewerCanReply"`
						ViewerCanResolve   bool `json:"viewerCanResolve"`
						ViewerCanUnresolve bool `json:"viewerCanUnresolve"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

// GetPullRequestReviewThreads fetches the PR through the existing REST boundary,
// then reads one fixed GraphQL review-thread page and requires GraphQL to report
// the same current head SHA. Cursor input is opaque data passed only as a GraphQL
// variable; it is never concatenated into the query text.
func (s *RemoteService) GetPullRequestReviewThreads(ctx context.Context, remoteID string, number int, after string, limit int) (*GitHubPullRequestReviewThreadsResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	after = strings.TrimSpace(after)
	if len([]byte(after)) > maxGitHubGraphQLCursorBytes {
		return nil, fmt.Errorf("review thread cursor exceeds %d bytes", maxGitHubGraphQLCursorBytes)
	}
	if limit == 0 {
		limit = defaultGitHubReviewThreadLimit
	}
	if limit < 1 || limit > maxGitHubReviewThreadLimit {
		return nil, fmt.Errorf("review thread limit must be between 1 and %d", maxGitHubReviewThreadLimit)
	}

	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if !validRemoteHash(pull.Head.SHA) {
		return nil, fmt.Errorf("GitHub pull request head could not be validated")
	}
	head := strings.ToLower(pull.Head.SHA)

	variables := map[string]interface{}{
		"owner": owner, "repository": repository, "number": number, "first": limit,
	}
	if after == "" {
		variables["after"] = nil
	} else {
		variables["after"] = after
	}
	var response githubPullRequestReviewThreadsGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, githubPullRequestReviewThreadsQuery, variables, &response); err != nil {
		return nil, fmt.Errorf("GitHub pull request review threads could not be inspected")
	}
	if len(response.Errors) > 0 || response.Data.Repository == nil || response.Data.Repository.PullRequest == nil {
		return nil, fmt.Errorf("GitHub pull request review threads could not be inspected")
	}
	graphPull := response.Data.Repository.PullRequest
	if !validRemoteHash(graphPull.HeadRefOID) || !strings.EqualFold(graphPull.HeadRefOID, head) {
		return nil, fmt.Errorf("pull request head changed while review threads were inspected; inspect the pull request again")
	}

	threads := make([]GitHubPullRequestReviewThreadResult, 0, len(graphPull.ReviewThreads.Nodes))
	for _, node := range graphPull.ReviewThreads.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" || len([]byte(id)) > maxGitHubGraphQLNodeIDBytes {
			return nil, fmt.Errorf("GitHub review thread identity could not be validated")
		}
		path, pathTruncated := truncateGitHubFeedbackText(node.Path, maxGitHubFeedbackPathBytes)
		resolvedBy := ""
		if node.ResolvedBy != nil {
			resolvedBy = node.ResolvedBy.Login
		}
		threads = append(threads, GitHubPullRequestReviewThreadResult{
			ID: id, IsResolved: node.IsResolved, IsOutdated: node.IsOutdated, IsCollapsed: node.IsCollapsed,
			Path: path, PathTruncated: pathTruncated, Line: node.Line, StartLine: node.StartLine,
			DiffSide: node.DiffSide, StartDiffSide: node.StartDiffSide, SubjectType: node.SubjectType,
			ResolvedBy: resolvedBy, ViewerCanReply: node.ViewerCanReply,
			ViewerCanResolve: node.ViewerCanResolve, ViewerCanUnresolve: node.ViewerCanUnresolve,
		})
	}

	nextCursor := ""
	if graphPull.ReviewThreads.PageInfo.HasNextPage {
		nextCursor = strings.TrimSpace(graphPull.ReviewThreads.PageInfo.EndCursor)
		if nextCursor == "" || len([]byte(nextCursor)) > maxGitHubGraphQLCursorBytes {
			return nil, fmt.Errorf("GitHub review thread pagination cursor could not be validated")
		}
	}
	return &GitHubPullRequestReviewThreadsResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number, Head: head,
		Limit: limit, TotalCount: graphPull.ReviewThreads.TotalCount, Threads: threads,
		HasNextPage: graphPull.ReviewThreads.PageInfo.HasNextPage, NextCursor: nextCursor,
	}, nil
}

func (s *RemoteService) doGitHubGraphQL(ctx context.Context, token, query string, variables map[string]interface{}, out interface{}) error {
	if s == nil || s.githubClient == nil || strings.TrimSpace(query) == "" || out == nil {
		return fmt.Errorf("GitHub GraphQL request is unavailable")
	}
	encoded, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAPIBaseURL+"/graphql", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	response, err := s.githubClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected GitHub GraphQL status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return err
	}
	return nil
}
