package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const maxGitHubReviewReplyBodyBytes = 8 << 10

var errGitHubPullRequestReplyDisabled = errors.New("GitHub pull request review replies are disabled")

// GitHubPullRequestReviewReplier is the narrow hosted communication mutation
// boundary. Repository identity, API host, credentials, and target PR head are
// operator/GitHub derived and revalidated immediately before posting.
type GitHubPullRequestReviewReplier interface {
	ReplyToPullRequestReviewComment(ctx context.Context, remoteID string, number int, expectedHead string, commentID, expectedReviewID int64, expectedUpdatedAt, body string) (*GitHubPullRequestReviewReplyResult, error)
}

// GitHubPullRequestReviewReplyResult contains only bounded confirmation metadata.
// The posted body and provider-controlled API details are intentionally omitted.
type GitHubPullRequestReviewReplyResult struct {
	Remote          string `json:"remote"`
	Repository      string `json:"repository"`
	PullRequest     int    `json:"pull_request"`
	Head            string `json:"head"`
	ParentCommentID int64  `json:"parent_comment_id"`
	ReviewID        int64  `json:"review_id"`
	ReplyID         int64  `json:"reply_id"`
	CreatedAt       string `json:"created_at,omitempty"`
	Posted          bool   `json:"posted"`
}

type githubPullRequestReviewCommentMutationResponse struct {
	ID                  int64  `json:"id"`
	PullRequestReviewID int64  `json:"pull_request_review_id"`
	PullRequestURL      string `json:"pull_request_url"`
	InReplyToID         int64  `json:"in_reply_to_id"`
	UpdatedAt           string `json:"updated_at"`
	CreatedAt           string `json:"created_at"`
}

func remoteSupportsGitHubPullRequestReply(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestReply && remote.TokenEnv != ""
}

func (s *RemoteService) githubPullRequestReplyConfig(remoteID string) (RemoteConfig, string, string, string, error) {
	if !s.GitHubPullRequestReplyMutationEnabled() {
		return RemoteConfig{}, "", "", "", errGitHubPullRequestReplyDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q is not configured", remoteID)
	}
	owner, repository, ok := githubRepositoryFromRemote(remote)
	if !ok || !remoteSupportsGitHubPullRequestReply(remote) {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q does not allow GitHub pull request review replies", remoteID)
	}
	if s.githubClient == nil {
		return RemoteConfig{}, "", "", "", fmt.Errorf("GitHub pull request transport is unavailable")
	}
	token, exists := s.lookupEnv(remote.TokenEnv)
	if !exists || strings.TrimSpace(token) == "" {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q GitHub credentials are unavailable", remoteID)
	}
	return remote, owner, repository, token, nil
}

// ReplyToPullRequestReviewComment posts a reply only after revalidating the PR
// current head/open state and the exact reviewed top-level comment identity and
// version. This prevents stale feedback from authorizing a later hosted write.
func (s *RemoteService) ReplyToPullRequestReviewComment(ctx context.Context, remoteID string, number int, expectedHead string, commentID, expectedReviewID int64, expectedUpdatedAt, body string) (*GitHubPullRequestReviewReplyResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	expectedHead = strings.ToLower(strings.TrimSpace(expectedHead))
	if !validRemoteHash(expectedHead) {
		return nil, fmt.Errorf("expected_head must be the current pull request head from GitHub inspection")
	}
	if commentID <= 0 || expectedReviewID <= 0 {
		return nil, fmt.Errorf("comment_id and expected_review_id must be positive")
	}
	expectedUpdatedAt = strings.TrimSpace(expectedUpdatedAt)
	if _, err := time.Parse(time.RFC3339, expectedUpdatedAt); err != nil {
		return nil, fmt.Errorf("expected_updated_at must be the reviewed GitHub comment timestamp")
	}
	if strings.TrimSpace(body) == "" || len([]byte(body)) > maxGitHubReviewReplyBodyBytes || !utf8.ValidString(body) {
		return nil, fmt.Errorf("reply body must contain valid UTF-8 text within %d bytes", maxGitHubReviewReplyBodyBytes)
	}

	remote, owner, repository, token, err := s.githubPullRequestReplyConfig(remoteID)
	if err != nil {
		return nil, err
	}
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if pull.Merged || !strings.EqualFold(strings.TrimSpace(pull.State), "open") {
		return nil, fmt.Errorf("pull request is no longer open; inspect the pull request before replying")
	}
	if !strings.EqualFold(pull.Head.SHA, expectedHead) {
		return nil, fmt.Errorf("pull request head changed; inspect feedback again before replying")
	}

	commentEndpoint := fmt.Sprintf("/repos/%s/%s/pulls/comments/%d", owner, repository, commentID)
	var comment githubPullRequestReviewCommentMutationResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, commentEndpoint, nil, http.StatusOK, &comment); err != nil {
		return nil, fmt.Errorf("GitHub pull request review comment could not be revalidated")
	}
	expectedPullURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", githubAPIBaseURL, owner, repository, number)
	if comment.ID != commentID || comment.PullRequestReviewID != expectedReviewID || comment.PullRequestURL != expectedPullURL {
		return nil, fmt.Errorf("GitHub review comment identity changed; inspect feedback again before replying")
	}
	if comment.InReplyToID != 0 {
		return nil, fmt.Errorf("GitHub review replies must target a top-level review comment")
	}
	if comment.UpdatedAt != expectedUpdatedAt {
		return nil, fmt.Errorf("GitHub review comment changed; inspect feedback again before replying")
	}

	replyEndpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments/%d/replies", owner, repository, number, commentID)
	var reply githubPullRequestReviewCommentMutationResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodPost, replyEndpoint, map[string]string{"body": body}, http.StatusCreated, &reply); err != nil {
		return nil, fmt.Errorf("GitHub pull request review reply outcome is unknown; inspect feedback before retrying")
	}
	if reply.ID <= 0 || reply.PullRequestReviewID != expectedReviewID || reply.InReplyToID != commentID || reply.PullRequestURL != expectedPullURL {
		return nil, fmt.Errorf("GitHub pull request review reply outcome could not be validated; inspect feedback before retrying")
	}

	return &GitHubPullRequestReviewReplyResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number, Head: expectedHead,
		ParentCommentID: commentID, ReviewID: expectedReviewID, ReplyID: reply.ID, CreatedAt: reply.CreatedAt, Posted: true,
	}, nil
}
