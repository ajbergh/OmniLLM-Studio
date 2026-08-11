package gitrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	defaultGitHubFeedbackLimit = 10
	maxGitHubFeedbackLimit     = 20
	maxGitHubFeedbackPage      = 100
	maxGitHubFeedbackBodyBytes = 1536
	maxGitHubFeedbackPathBytes = 1024
)

// GitHubPullRequestFeedbackReader is the read-only hosted review-feedback
// boundary. Repository identity, API host, credentials, and PR head SHA remain
// operator/GitHub derived rather than model supplied.
type GitHubPullRequestFeedbackReader interface {
	GetPullRequestFeedback(ctx context.Context, remoteID string, number int, kind string, page, limit int) (*GitHubPullRequestFeedbackResult, error)
}

// GitHubPullRequestFeedbackResult is one bounded page of hosted collaboration
// evidence for a pull request. Body text is intentionally preserved as evidence
// (subject only to explicit byte truncation) and is protected by the LLM tool-
// result trust boundary before it reaches a model.
type GitHubPullRequestFeedbackResult struct {
	Remote      string                          `json:"remote"`
	Repository  string                          `json:"repository"`
	PullRequest int                             `json:"pull_request"`
	Head        string                          `json:"head"`
	Kind        string                          `json:"kind"`
	Page        int                             `json:"page"`
	Limit       int                             `json:"limit"`
	Order       string                          `json:"order"`
	Items       []GitHubPullRequestFeedbackItem `json:"items"`
	MayHaveMore bool                            `json:"may_have_more,omitempty"`
}

// GitHubPullRequestFeedbackItem is a union-style, model-safe representation of
// submitted reviews, inline review comments, general PR comments, or outstanding
// review requests. Fields not relevant to the selected kind are omitted.
type GitHubPullRequestFeedbackItem struct {
	Type                string `json:"type"`
	ID                  int64  `json:"id,omitempty"`
	ReviewID            int64  `json:"review_id,omitempty"`
	Author              string `json:"author,omitempty"`
	Team                string `json:"team,omitempty"`
	State               string `json:"state,omitempty"`
	Body                string `json:"body,omitempty"`
	BodyTruncated       bool   `json:"body_truncated,omitempty"`
	AuthorAssociation   string `json:"author_association,omitempty"`
	SubmittedAt         string `json:"submitted_at,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	Commit              string `json:"commit,omitempty"`
	CommitIsCurrentHead *bool  `json:"commit_is_current_head,omitempty"`
	OriginalCommit      string `json:"original_commit,omitempty"`
	Path                string `json:"path,omitempty"`
	PathTruncated       bool   `json:"path_truncated,omitempty"`
	Line                *int   `json:"line,omitempty"`
	Side                string `json:"side,omitempty"`
	StartLine           *int   `json:"start_line,omitempty"`
	StartSide           string `json:"start_side,omitempty"`
	InReplyToID         int64  `json:"in_reply_to_id,omitempty"`
}

type githubPullRequestReviewFeedbackResponse struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	State             string `json:"state"`
	SubmittedAt       string `json:"submitted_at"`
	CommitID          string `json:"commit_id"`
	AuthorAssociation string `json:"author_association"`
	User              struct {
		Login string `json:"login"`
	} `json:"user"`
}

type githubPullRequestReviewCommentFeedbackResponse struct {
	ID                  int64  `json:"id"`
	PullRequestReviewID int64  `json:"pull_request_review_id"`
	Body                string `json:"body"`
	Path                string `json:"path"`
	Line                *int   `json:"line"`
	Side                string `json:"side"`
	StartLine           *int   `json:"start_line"`
	StartSide           string `json:"start_side"`
	CommitID            string `json:"commit_id"`
	OriginalCommitID    string `json:"original_commit_id"`
	InReplyToID         int64  `json:"in_reply_to_id"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	AuthorAssociation   string `json:"author_association"`
	User                struct {
		Login string `json:"login"`
	} `json:"user"`
}

type githubPullRequestIssueCommentFeedbackResponse struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	AuthorAssociation string `json:"author_association"`
	User              struct {
		Login string `json:"login"`
	} `json:"user"`
}

type githubPullRequestReviewRequestsResponse struct {
	Users []struct {
		Login string `json:"login"`
	} `json:"users"`
	Teams []struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"teams"`
}

// GetPullRequestFeedback reads one bounded collaboration surface after fetching
// the PR itself. Every result therefore carries the current hosted PR head SHA,
// allowing callers to distinguish feedback on an older commit from feedback on
// the current head without supplying an arbitrary commit ref.
func (s *RemoteService) GetPullRequestFeedback(ctx context.Context, remoteID string, number int, kind string, page, limit int) (*GitHubPullRequestFeedbackResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	kind, page, limit, err := normalizeGitHubFeedbackRequest(kind, page, limit)
	if err != nil {
		return nil, err
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
	result := &GitHubPullRequestFeedbackResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: head, Kind: kind, Page: page, Limit: limit, Items: []GitHubPullRequestFeedbackItem{},
	}

	switch kind {
	case "reviews":
		result.Order = "chronological"
		query := feedbackPageQuery(page, limit)
		endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?%s", owner, repository, number, query.Encode())
		var responses []githubPullRequestReviewFeedbackResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &responses); err != nil {
			return nil, fmt.Errorf("GitHub pull request reviews could not be inspected")
		}
		for _, response := range responses {
			body, bodyTruncated := truncateGitHubFeedbackText(response.Body, maxGitHubFeedbackBodyBytes)
			commit, current := githubFeedbackCommit(response.CommitID, head)
			result.Items = append(result.Items, GitHubPullRequestFeedbackItem{
				Type: "review", ID: response.ID, Author: response.User.Login, State: response.State,
				Body: body, BodyTruncated: bodyTruncated, AuthorAssociation: response.AuthorAssociation,
				SubmittedAt: response.SubmittedAt, Commit: commit, CommitIsCurrentHead: current,
			})
		}
		result.MayHaveMore = len(responses) == limit
	case "review_comments":
		result.Order = "updated_desc"
		query := feedbackPageQuery(page, limit)
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?%s", owner, repository, number, query.Encode())
		var responses []githubPullRequestReviewCommentFeedbackResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &responses); err != nil {
			return nil, fmt.Errorf("GitHub pull request review comments could not be inspected")
		}
		for _, response := range responses {
			body, bodyTruncated := truncateGitHubFeedbackText(response.Body, maxGitHubFeedbackBodyBytes)
			path, pathTruncated := truncateGitHubFeedbackText(response.Path, maxGitHubFeedbackPathBytes)
			commit, current := githubFeedbackCommit(response.CommitID, head)
			originalCommit, _ := githubFeedbackCommit(response.OriginalCommitID, "")
			result.Items = append(result.Items, GitHubPullRequestFeedbackItem{
				Type: "review_comment", ID: response.ID, ReviewID: response.PullRequestReviewID, Author: response.User.Login,
				Body: body, BodyTruncated: bodyTruncated, AuthorAssociation: response.AuthorAssociation,
				CreatedAt: response.CreatedAt, UpdatedAt: response.UpdatedAt, Commit: commit, CommitIsCurrentHead: current,
				OriginalCommit: originalCommit, Path: path, PathTruncated: pathTruncated, Line: response.Line, Side: response.Side,
				StartLine: response.StartLine, StartSide: response.StartSide, InReplyToID: response.InReplyToID,
			})
		}
		result.MayHaveMore = len(responses) == limit
	case "comments":
		result.Order = "github_default"
		query := feedbackPageQuery(page, limit)
		endpoint := fmt.Sprintf("/repos/%s/%s/issues/%d/comments?%s", owner, repository, number, query.Encode())
		var responses []githubPullRequestIssueCommentFeedbackResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &responses); err != nil {
			return nil, fmt.Errorf("GitHub pull request comments could not be inspected")
		}
		for _, response := range responses {
			body, bodyTruncated := truncateGitHubFeedbackText(response.Body, maxGitHubFeedbackBodyBytes)
			result.Items = append(result.Items, GitHubPullRequestFeedbackItem{
				Type: "comment", ID: response.ID, Author: response.User.Login, Body: body, BodyTruncated: bodyTruncated,
				AuthorAssociation: response.AuthorAssociation, CreatedAt: response.CreatedAt, UpdatedAt: response.UpdatedAt,
			})
		}
		result.MayHaveMore = len(responses) == limit
	case "review_requests":
		result.Order = "users_then_teams"
		endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", owner, repository, number)
		var response githubPullRequestReviewRequestsResponse
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &response); err != nil {
			return nil, fmt.Errorf("GitHub pull request review requests could not be inspected")
		}
		for _, user := range response.Users {
			if len(result.Items) >= limit {
				result.MayHaveMore = true
				break
			}
			result.Items = append(result.Items, GitHubPullRequestFeedbackItem{Type: "review_request_user", Author: user.Login})
		}
		if !result.MayHaveMore {
			for _, team := range response.Teams {
				if len(result.Items) >= limit {
					result.MayHaveMore = true
					break
				}
				teamName := team.Slug
				if teamName == "" {
					teamName = team.Name
				}
				result.Items = append(result.Items, GitHubPullRequestFeedbackItem{Type: "review_request_team", Team: teamName})
			}
		}
	}

	return result, nil
}

func normalizeGitHubFeedbackRequest(kind string, page, limit int) (string, int, int, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "reviews", "review_comments", "comments", "review_requests":
	default:
		return "", 0, 0, fmt.Errorf("feedback kind must be reviews, review_comments, comments, or review_requests")
	}
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = defaultGitHubFeedbackLimit
	}
	if page < 1 || page > maxGitHubFeedbackPage {
		return "", 0, 0, fmt.Errorf("feedback page must be between 1 and %d", maxGitHubFeedbackPage)
	}
	if limit < 1 || limit > maxGitHubFeedbackLimit {
		return "", 0, 0, fmt.Errorf("feedback limit must be between 1 and %d", maxGitHubFeedbackLimit)
	}
	if kind == "review_requests" && page != 1 {
		return "", 0, 0, fmt.Errorf("review_requests supports only page 1")
	}
	return kind, page, limit, nil
}

func feedbackPageQuery(page, limit int) url.Values {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(limit))
	query.Set("page", strconv.Itoa(page))
	return query
}

func githubFeedbackCommit(value, currentHead string) (string, *bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !validRemoteHash(value) {
		return "", nil
	}
	if currentHead == "" {
		return value, nil
	}
	matches := strings.EqualFold(value, currentHead)
	return value, &matches
}

func truncateGitHubFeedbackText(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	cut := maxBytes
	for cut > 0 && cut < len(value) && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}
