package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errGitHubPullRequestThreadResolutionDisabled = errors.New("GitHub pull request review thread resolution is disabled")

const githubPullRequestReviewThreadStateQuery = `
query PullRequestReviewThreadState($threadId: ID!) {
  node(id: $threadId) {
    ... on PullRequestReviewThread {
      id
      isResolved
      isOutdated
      viewerCanResolve
      viewerCanUnresolve
      repository { nameWithOwner }
      pullRequest { number headRefOid }
    }
  }
}`

const githubResolvePullRequestReviewThreadMutation = `
mutation ResolvePullRequestReviewThread($threadId: ID!) {
  resolveReviewThread(input: {threadId: $threadId}) {
    thread {
      id
      isResolved
      isOutdated
      repository { nameWithOwner }
      pullRequest { number headRefOid }
    }
  }
}`

const githubUnresolvePullRequestReviewThreadMutation = `
mutation UnresolvePullRequestReviewThread($threadId: ID!) {
  unresolveReviewThread(input: {threadId: $threadId}) {
    thread {
      id
      isResolved
      isOutdated
      repository { nameWithOwner }
      pullRequest { number headRefOid }
    }
  }
}`

// GitHubPullRequestReviewThreadResolver is the narrow hosted mutation boundary
// for changing review-thread resolution state. The caller must bind the request
// to a previously reviewed PR head and thread state; the service revalidates all
// hosted state before selecting one fixed application-owned GraphQL mutation.
type GitHubPullRequestReviewThreadResolver interface {
	SetPullRequestReviewThreadResolved(ctx context.Context, remoteID string, number int, expectedHead, threadID string, expectedResolved, expectedOutdated, resolved bool) (*GitHubPullRequestReviewThreadResolutionResult, error)
}

// GitHubPullRequestReviewThreadResolutionResult contains bounded confirmation
// metadata only. No reviewer prose or provider response details are returned.
type GitHubPullRequestReviewThreadResolutionResult struct {
	Remote      string `json:"remote"`
	Repository  string `json:"repository"`
	PullRequest int    `json:"pull_request"`
	Head        string `json:"head"`
	ThreadID    string `json:"thread_id"`
	Resolved    bool   `json:"resolved"`
	Outdated    bool   `json:"outdated"`
	Changed     bool   `json:"changed"`
}

type githubPullRequestReviewThreadStateGraphQLResponse struct {
	Data struct {
		Node *githubPullRequestReviewThreadMutationState `json:"node"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubPullRequestReviewThreadMutationGraphQLResponse struct {
	Data struct {
		ResolveReviewThread *struct {
			Thread *githubPullRequestReviewThreadMutationState `json:"thread"`
		} `json:"resolveReviewThread"`
		UnresolveReviewThread *struct {
			Thread *githubPullRequestReviewThreadMutationState `json:"thread"`
		} `json:"unresolveReviewThread"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubPullRequestReviewThreadMutationState struct {
	ID                 string `json:"id"`
	IsResolved         bool   `json:"isResolved"`
	IsOutdated         bool   `json:"isOutdated"`
	ViewerCanResolve   bool   `json:"viewerCanResolve"`
	ViewerCanUnresolve bool   `json:"viewerCanUnresolve"`
	Repository         struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	PullRequest struct {
		Number     int    `json:"number"`
		HeadRefOID string `json:"headRefOid"`
	} `json:"pullRequest"`
}

func remoteSupportsGitHubPullRequestThreadResolution(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestThreadResolution && remote.TokenEnv != ""
}

func (s *RemoteService) githubPullRequestThreadResolutionConfig(remoteID string) (RemoteConfig, string, string, string, error) {
	if !s.GitHubPullRequestThreadResolutionMutationEnabled() {
		return RemoteConfig{}, "", "", "", errGitHubPullRequestThreadResolutionDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q is not configured", remoteID)
	}
	owner, repository, ok := githubRepositoryFromRemote(remote)
	if !ok || !remoteSupportsGitHubPullRequestThreadResolution(remote) {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q does not allow GitHub pull request review thread resolution", remoteID)
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

// SetPullRequestReviewThreadResolved changes one thread's resolved state only
// after revalidating the current PR and exact hosted thread state. GitHub viewer
// capability is treated as an additional provider-side prerequisite, never as
// OmniLLM authorization; the independent operator gate and tool approval remain
// authoritative.
func (s *RemoteService) SetPullRequestReviewThreadResolved(ctx context.Context, remoteID string, number int, expectedHead, threadID string, expectedResolved, expectedOutdated, resolved bool) (*GitHubPullRequestReviewThreadResolutionResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	expectedHead = strings.ToLower(strings.TrimSpace(expectedHead))
	if !validRemoteHash(expectedHead) {
		return nil, fmt.Errorf("expected_head must be the current pull request head from GitHub inspection")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" || len([]byte(threadID)) > maxGitHubGraphQLNodeIDBytes {
		return nil, fmt.Errorf("thread_id must be the bounded opaque ID from GitHub review thread inspection")
	}
	if resolved == expectedResolved {
		return nil, fmt.Errorf("resolved must differ from expected_is_resolved")
	}

	remote, owner, repository, token, err := s.githubPullRequestThreadResolutionConfig(remoteID)
	if err != nil {
		return nil, err
	}
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if pull.Merged || !strings.EqualFold(strings.TrimSpace(pull.State), "open") {
		return nil, fmt.Errorf("pull request is no longer open; inspect the pull request before changing review thread state")
	}
	if !strings.EqualFold(pull.Head.SHA, expectedHead) {
		return nil, fmt.Errorf("pull request head changed; inspect review threads again before changing thread state")
	}

	variables := map[string]interface{}{"threadId": threadID}
	var preflight githubPullRequestReviewThreadStateGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, githubPullRequestReviewThreadStateQuery, variables, &preflight); err != nil {
		return nil, fmt.Errorf("GitHub pull request review thread could not be revalidated")
	}
	if len(preflight.Errors) > 0 || preflight.Data.Node == nil {
		return nil, fmt.Errorf("GitHub pull request review thread could not be revalidated")
	}
	state := preflight.Data.Node
	if err := validateGitHubPullRequestReviewThreadMutationState(state, owner, repository, number, expectedHead, threadID, expectedResolved, expectedOutdated); err != nil {
		return nil, err
	}
	if resolved && !state.ViewerCanResolve {
		return nil, fmt.Errorf("configured GitHub identity cannot resolve the reviewed thread")
	}
	if !resolved && !state.ViewerCanUnresolve {
		return nil, fmt.Errorf("configured GitHub identity cannot unresolve the reviewed thread")
	}

	mutation := githubResolvePullRequestReviewThreadMutation
	if !resolved {
		mutation = githubUnresolvePullRequestReviewThreadMutation
	}
	var mutationResponse githubPullRequestReviewThreadMutationGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, mutation, variables, &mutationResponse); err != nil {
		return nil, fmt.Errorf("GitHub pull request review thread resolution outcome is unknown; inspect review threads before retrying")
	}
	if len(mutationResponse.Errors) > 0 {
		return nil, fmt.Errorf("GitHub pull request review thread resolution outcome is unknown; inspect review threads before retrying")
	}
	var changed *githubPullRequestReviewThreadMutationState
	if resolved && mutationResponse.Data.ResolveReviewThread != nil {
		changed = mutationResponse.Data.ResolveReviewThread.Thread
	}
	if !resolved && mutationResponse.Data.UnresolveReviewThread != nil {
		changed = mutationResponse.Data.UnresolveReviewThread.Thread
	}
	if err := validateGitHubPullRequestReviewThreadMutationResult(changed, owner, repository, number, expectedHead, threadID, resolved, expectedOutdated); err != nil {
		return nil, fmt.Errorf("GitHub pull request review thread resolution outcome could not be validated; inspect review threads before retrying")
	}

	return &GitHubPullRequestReviewThreadResolutionResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number, Head: expectedHead,
		ThreadID: threadID, Resolved: resolved, Outdated: expectedOutdated, Changed: true,
	}, nil
}

func validateGitHubPullRequestReviewThreadMutationState(state *githubPullRequestReviewThreadMutationState, owner, repository string, number int, expectedHead, threadID string, expectedResolved, expectedOutdated bool) error {
	if state == nil || strings.TrimSpace(state.ID) != threadID {
		return fmt.Errorf("GitHub review thread identity changed; inspect review threads again before changing thread state")
	}
	if !strings.EqualFold(strings.TrimSpace(state.Repository.NameWithOwner), owner+"/"+repository) || state.PullRequest.Number != number {
		return fmt.Errorf("GitHub review thread ownership changed; inspect review threads again before changing thread state")
	}
	if !validRemoteHash(state.PullRequest.HeadRefOID) || !strings.EqualFold(state.PullRequest.HeadRefOID, expectedHead) {
		return fmt.Errorf("pull request head changed while review thread state was revalidated; inspect review threads again")
	}
	if state.IsResolved != expectedResolved || state.IsOutdated != expectedOutdated {
		return fmt.Errorf("GitHub review thread state changed; inspect review threads again before changing thread state")
	}
	return nil
}

func validateGitHubPullRequestReviewThreadMutationResult(state *githubPullRequestReviewThreadMutationState, owner, repository string, number int, expectedHead, threadID string, resolved, expectedOutdated bool) error {
	if state == nil || strings.TrimSpace(state.ID) != threadID || state.IsResolved != resolved || state.IsOutdated != expectedOutdated {
		return fmt.Errorf("mutation result state mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(state.Repository.NameWithOwner), owner+"/"+repository) || state.PullRequest.Number != number {
		return fmt.Errorf("mutation result ownership mismatch")
	}
	if !validRemoteHash(state.PullRequest.HeadRefOID) || !strings.EqualFold(state.PullRequest.HeadRefOID, expectedHead) {
		return fmt.Errorf("mutation result head mismatch")
	}
	return nil
}
