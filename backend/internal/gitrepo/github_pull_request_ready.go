package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errGitHubPullRequestReadyDisabled = errors.New("GitHub pull request ready-for-review mutation is disabled")

const githubPullRequestReadyStateQuery = `
query PullRequestReadyState($owner: String!, $repository: String!, $number: Int!) {
  repository(owner: $owner, name: $repository) {
    nameWithOwner
    pullRequest(number: $number) {
      id
      number
      isDraft
      state
      merged
      headRefOid
      baseRefName
    }
  }
}`

const githubMarkPullRequestReadyForReviewMutation = `
mutation MarkPullRequestReadyForReview($pullRequestId: ID!) {
  markPullRequestReadyForReview(input: {pullRequestId: $pullRequestId}) {
    pullRequest {
      id
      number
      isDraft
      state
      merged
      headRefOid
      baseRefName
      repository { nameWithOwner }
    }
  }
}`

// GitHubPullRequestReadyMarker is the narrow hosted mutation boundary for
// advancing one exact reviewed draft pull request to ready-for-review state.
// Repository identity, API host, credentials, node ID, base, and mutation text
// remain application/operator-controlled.
type GitHubPullRequestReadyMarker interface {
	MarkPullRequestReadyForReview(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestReadyResult, error)
}

// GitHubPullRequestReadyResult contains bounded post-mutation confirmation only.
type GitHubPullRequestReadyResult struct {
	Remote      string `json:"remote"`
	Repository  string `json:"repository"`
	PullRequest int    `json:"pull_request"`
	Head        string `json:"head"`
	BaseBranch  string `json:"base_branch"`
	Draft       bool   `json:"draft"`
	Ready       bool   `json:"ready"`
	Changed     bool   `json:"changed"`
}

type githubPullRequestReadyStateGraphQLResponse struct {
	Data struct {
		Repository *struct {
			NameWithOwner string                       `json:"nameWithOwner"`
			PullRequest   *githubPullRequestReadyState `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubPullRequestReadyMutationGraphQLResponse struct {
	Data struct {
		MarkPullRequestReadyForReview *struct {
			PullRequest *githubPullRequestReadyState `json:"pullRequest"`
		} `json:"markPullRequestReadyForReview"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubPullRequestReadyState struct {
	ID          string `json:"id"`
	Number      int    `json:"number"`
	IsDraft     bool   `json:"isDraft"`
	State       string `json:"state"`
	Merged      bool   `json:"merged"`
	HeadRefOID  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
	Repository  struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

func remoteSupportsGitHubPullRequestReady(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestReady && remote.TokenEnv != ""
}

func (s *RemoteService) githubPullRequestReadyConfig(remoteID string) (RemoteConfig, string, string, string, error) {
	if !s.GitHubPullRequestReadyMutationEnabled() {
		return RemoteConfig{}, "", "", "", errGitHubPullRequestReadyDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q is not configured", remoteID)
	}
	owner, repository, ok := githubRepositoryFromRemote(remote)
	if !ok || !remoteSupportsGitHubPullRequestReady(remote) {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q does not allow GitHub pull request ready-for-review mutation", remoteID)
	}
	if s.githubClient == nil || s.transport == nil {
		return RemoteConfig{}, "", "", "", fmt.Errorf("GitHub pull request transport is unavailable")
	}
	token, exists := s.lookupEnv(remote.TokenEnv)
	if !exists || strings.TrimSpace(token) == "" {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q GitHub credentials are unavailable", remoteID)
	}
	return remote, owner, repository, token, nil
}

// MarkPullRequestReadyForReview advances exactly one currently-open draft only
// after re-fetching hosted state, proving the reviewed head and configured
// default base are unchanged, and resolving the opaque PR node ID internally.
func (s *RemoteService) MarkPullRequestReadyForReview(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestReadyResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	expectedHead = strings.ToLower(strings.TrimSpace(expectedHead))
	if !validRemoteHash(expectedHead) {
		return nil, fmt.Errorf("expected_head must be the exact 40-character PR head from GitHub inspection")
	}
	remote, owner, repository, token, err := s.githubPullRequestReadyConfig(remoteID)
	if err != nil {
		return nil, err
	}

	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if pull.State != "open" || pull.Merged {
		return nil, fmt.Errorf("pull request is no longer an open unmerged draft; inspect it again")
	}
	if !pull.Draft {
		return nil, fmt.Errorf("pull request is already ready for review")
	}
	if !validRemoteHash(pull.Head.SHA) || !strings.EqualFold(pull.Head.SHA, expectedHead) {
		return nil, fmt.Errorf("pull request head changed; inspect it again before marking ready")
	}

	advertised, err := s.advertiseRemoteForGitHub(ctx, strings.TrimSpace(remoteID), remote)
	if err != nil {
		return nil, err
	}
	baseBranch, ok := advertisedDefaultBranch(advertised)
	if !ok || pull.Base.Ref != baseBranch {
		return nil, fmt.Errorf("pull request base no longer matches the configured remote default branch; inspect remote and pull request state again")
	}

	variables := map[string]interface{}{"owner": owner, "repository": repository, "number": number}
	var stateResponse githubPullRequestReadyStateGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, githubPullRequestReadyStateQuery, variables, &stateResponse); err != nil {
		return nil, fmt.Errorf("GitHub pull request ready state could not be inspected")
	}
	if len(stateResponse.Errors) > 0 || stateResponse.Data.Repository == nil || stateResponse.Data.Repository.PullRequest == nil {
		return nil, fmt.Errorf("GitHub pull request ready state could not be inspected")
	}
	state := stateResponse.Data.Repository.PullRequest
	if stateResponse.Data.Repository.NameWithOwner != owner+"/"+repository || !validGitHubPullRequestReadyState(state, number, expectedHead, baseBranch, true) {
		return nil, fmt.Errorf("pull request state changed while ready-for-review eligibility was checked; inspect it again")
	}

	var mutationResponse githubPullRequestReadyMutationGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, githubMarkPullRequestReadyForReviewMutation, map[string]interface{}{"pullRequestId": state.ID}, &mutationResponse); err != nil {
		return nil, fmt.Errorf("GitHub ready-for-review outcome is unknown; inspect the pull request before retrying")
	}
	if len(mutationResponse.Errors) > 0 || mutationResponse.Data.MarkPullRequestReadyForReview == nil || mutationResponse.Data.MarkPullRequestReadyForReview.PullRequest == nil {
		return nil, fmt.Errorf("GitHub ready-for-review outcome is unknown; inspect the pull request before retrying")
	}
	updated := mutationResponse.Data.MarkPullRequestReadyForReview.PullRequest
	if updated.ID != state.ID || updated.Repository.NameWithOwner != owner+"/"+repository || !validGitHubPullRequestReadyState(updated, number, expectedHead, baseBranch, false) {
		return nil, fmt.Errorf("GitHub ready-for-review outcome could not be validated; inspect the pull request before retrying")
	}
	return &GitHubPullRequestReadyResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: expectedHead, BaseBranch: baseBranch, Draft: false, Ready: true, Changed: true,
	}, nil
}

func validGitHubPullRequestReadyState(state *githubPullRequestReadyState, number int, expectedHead, baseBranch string, draft bool) bool {
	if state == nil {
		return false
	}
	id := strings.TrimSpace(state.ID)
	return id != "" && len([]byte(id)) <= maxGitHubGraphQLNodeIDBytes && state.Number == number &&
		state.IsDraft == draft && strings.EqualFold(state.State, "OPEN") && !state.Merged &&
		validRemoteHash(state.HeadRefOID) && strings.EqualFold(state.HeadRefOID, expectedHead) && state.BaseRefName == baseBranch
}
