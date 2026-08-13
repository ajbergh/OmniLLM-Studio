package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var errGitHubPullRequestMergeDisabled = errors.New("GitHub pull request merge mutation is disabled")

// GitHubPullRequestMerger is the narrow M3B hosted mutation boundary. It accepts
// only one configured remote, one pull request number, and the exact reviewed
// head SHA. Repository identity, credential, merge method, base branch, policy,
// and API endpoint remain operator/application controlled.
type GitHubPullRequestMerger interface {
	MergePullRequest(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestMergeResult, error)
}

// GitHubPullRequestMergeResult contains bounded post-mutation confirmation only.
type GitHubPullRequestMergeResult struct {
	Remote                     string `json:"remote"`
	Repository                 string `json:"repository"`
	PullRequest                int    `json:"pull_request"`
	Head                       string `json:"head"`
	BaseBranch                 string `json:"base_branch"`
	MergeMethod                string `json:"merge_method"`
	MergeCommit                string `json:"merge_commit"`
	Merged                     bool   `json:"merged"`
	Changed                    bool   `json:"changed"`
	ConfirmedAfterReinspection bool   `json:"confirmed_after_reinspection,omitempty"`
}

type githubPullRequestMergePayload struct {
	SHA         string `json:"sha"`
	MergeMethod string `json:"merge_method"`
}

type githubPullRequestMergeResponse struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

func remoteSupportsGitHubPullRequestMerge(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestRead && remote.AllowPullRequestMerge && validGitHubMergeMethod(remote.PullRequestMergeMethod) && remote.TokenEnv != ""
}

func validGitHubMergeMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "merge", "squash", "rebase":
		return true
	default:
		return false
	}
}

func containsMergeMethod(methods []string, method string) bool {
	method = strings.ToLower(strings.TrimSpace(method))
	for _, candidate := range methods {
		if strings.ToLower(strings.TrimSpace(candidate)) == method {
			return true
		}
	}
	return false
}

func (s *RemoteService) githubPullRequestMergeConfig(remoteID string) (RemoteConfig, string, string, string, error) {
	if !s.GitHubPullRequestMergeMutationEnabled() {
		return RemoteConfig{}, "", "", "", errGitHubPullRequestMergeDisabled
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return RemoteConfig{}, "", "", "", err
	}
	if !remoteSupportsGitHubPullRequestMerge(remote) {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q does not allow GitHub pull request merge", strings.TrimSpace(remoteID))
	}
	return remote, owner, repository, token, nil
}

// MergePullRequest performs exactly one GitHub merge request after a fresh M3A
// preflight proves the exact reviewed PR/head/base is eligible. The configured
// merge method is operator-owned and must still be allowed by that fresh policy.
// No branch deletion is performed. Ambiguous transport/response outcomes are
// inspected once and are never retried automatically.
func (s *RemoteService) MergePullRequest(ctx context.Context, remoteID string, number int, expectedHead string) (*GitHubPullRequestMergeResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	expectedHead = strings.ToLower(strings.TrimSpace(expectedHead))
	if !validRemoteHash(expectedHead) {
		return nil, fmt.Errorf("expected_head must be the exact 40-character PR head from GitHub inspection")
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, owner, repository, token, err := s.githubPullRequestMergeConfig(remoteID)
	if err != nil {
		return nil, err
	}
	method := strings.ToLower(strings.TrimSpace(remote.PullRequestMergeMethod))

	eligibility, err := s.GetPullRequestMergeEligibility(ctx, remoteID, number)
	if err != nil {
		return nil, err
	}
	if eligibility == nil || !eligibility.EligibilityComplete || !eligibility.Eligible {
		return nil, fmt.Errorf("pull request merge eligibility is incomplete or unsatisfied; inspect it again")
	}
	if !validRemoteHash(eligibility.Head) || !strings.EqualFold(eligibility.Head, expectedHead) {
		return nil, fmt.Errorf("pull request head changed; inspect it again before merging")
	}
	if !eligibility.DefaultBaseVerified || strings.TrimSpace(eligibility.BaseBranch) == "" {
		return nil, fmt.Errorf("pull request base is not the configured repository default branch")
	}
	if !containsMergeMethod(eligibility.AllowedMergeMethods, method) {
		return nil, fmt.Errorf("configured merge method is not allowed by the current repository policy")
	}

	payload := githubPullRequestMergePayload{SHA: expectedHead, MergeMethod: method}
	response, status, requestErr := s.doGitHubMergeRequest(ctx, token, owner, repository, number, payload)
	if requestErr != nil {
		if confirmed, ok := s.confirmGitHubMergeOutcome(ctx, remoteID, token, owner, repository, number, expectedHead, eligibility.BaseBranch, method); ok {
			return confirmed, nil
		}
		return nil, fmt.Errorf("GitHub merge outcome is unknown; inspect the pull request before retrying")
	}

	switch status {
	case http.StatusOK:
		if response == nil || !response.Merged || !validRemoteHash(response.SHA) {
			if confirmed, ok := s.confirmGitHubMergeOutcome(ctx, remoteID, token, owner, repository, number, expectedHead, eligibility.BaseBranch, method); ok {
				return confirmed, nil
			}
			return nil, fmt.Errorf("GitHub merge outcome could not be validated; inspect the pull request before retrying")
		}
		return &GitHubPullRequestMergeResult{
			Remote: remoteID, Repository: remote.Repository, PullRequest: number,
			Head: expectedHead, BaseBranch: eligibility.BaseBranch, MergeMethod: method,
			MergeCommit: strings.ToLower(response.SHA), Merged: true, Changed: true,
		}, nil
	case http.StatusConflict:
		return nil, fmt.Errorf("GitHub rejected the merge because the pull request head or mergeability changed; inspect it again")
	case http.StatusMethodNotAllowed:
		return nil, fmt.Errorf("GitHub rejected the merge under the current repository policy; inspect the pull request again")
	case http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("GitHub rejected the merge request")
	default:
		if confirmed, ok := s.confirmGitHubMergeOutcome(ctx, remoteID, token, owner, repository, number, expectedHead, eligibility.BaseBranch, method); ok {
			return confirmed, nil
		}
		return nil, fmt.Errorf("GitHub merge outcome is unknown; inspect the pull request before retrying")
	}
}

func (s *RemoteService) doGitHubMergeRequest(ctx context.Context, token, owner, repository string, number int, payload githubPullRequestMergePayload) (*githubPullRequestMergeResponse, int, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repository, number)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, githubAPIBaseURL+endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.githubClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status != http.StatusOK {
		return nil, status, nil
	}
	var decoded githubPullRequestMergeResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, status, err
	}
	return &decoded, status, nil
}

func (s *RemoteService) confirmGitHubMergeOutcome(ctx context.Context, remoteID, token, owner, repository string, number int, expectedHead, expectedBase, method string) (*GitHubPullRequestMergeResult, bool) {
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil || pull == nil || !pull.Merged || !validRemoteHash(pull.Head.SHA) || !strings.EqualFold(pull.Head.SHA, expectedHead) || strings.TrimSpace(pull.Base.Ref) != expectedBase || !validRemoteHash(pull.MergeCommitSHA) {
		return nil, false
	}
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, false
	}
	return &GitHubPullRequestMergeResult{
		Remote: remoteID, Repository: remote.Repository, PullRequest: number,
		Head: expectedHead, BaseBranch: expectedBase, MergeMethod: method,
		MergeCommit: strings.ToLower(pull.MergeCommitSHA), Merged: true, Changed: true,
		ConfirmedAfterReinspection: true,
	}, true
}
