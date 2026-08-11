package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxGitHubPullRequestListResults = 20
	maxGitHubCheckResults           = 50
	maxGitHubStatusResults          = 50
)

var errGitHubPullRequestReadDisabled = errors.New("GitHub pull request inspection is disabled")

// GitHubPullRequestReader is the read-only GitHub collaboration boundary.
// Repository identity, API host, and credentials remain operator-controlled.
type GitHubPullRequestReader interface {
	GetPullRequest(ctx context.Context, remoteID string, number int) (*GitHubPullRequestReadResult, error)
	ListPullRequests(ctx context.Context, remoteID, state, headBranch string, limit int) (*GitHubPullRequestListResult, error)
	GetPullRequestChecks(ctx context.Context, remoteID string, number int) (*GitHubPullRequestChecksResult, error)
}

// GitHubPullRequestReadResult contains bounded pull request metadata that is safe
// to expose to the model. It intentionally omits the body, API endpoints, and
// credential information.
type GitHubPullRequestReadResult struct {
	Remote         string `json:"remote"`
	Repository     string `json:"repository"`
	Number         int    `json:"number"`
	URL            string `json:"url"`
	Title          string `json:"title"`
	State          string `json:"state"`
	Draft          bool   `json:"draft"`
	Merged         bool   `json:"merged"`
	Mergeable      *bool  `json:"mergeable,omitempty"`
	MergeableState string `json:"mergeable_state,omitempty"`
	HeadBranch     string `json:"head_branch"`
	Head           string `json:"head"`
	BaseBranch     string `json:"base_branch"`
	Author         string `json:"author,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

// GitHubPullRequestListResult is a bounded first-page inventory. Truncated is
// true when more matching results exist than the caller-requested limit.
type GitHubPullRequestListResult struct {
	Remote       string                        `json:"remote"`
	Repository   string                        `json:"repository"`
	State        string                        `json:"state"`
	HeadBranch   string                        `json:"head_branch,omitempty"`
	PullRequests []GitHubPullRequestReadResult `json:"pull_requests"`
	Truncated    bool                          `json:"truncated,omitempty"`
}

// GitHubCheckRunResult intentionally returns only execution metadata. Check
// output/annotations and arbitrary external details URLs are not copied into the
// model context in this first read-only slice.
type GitHubCheckRunResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	App        string `json:"app,omitempty"`
}

// GitHubCommitStatusResult contains the latest combined-status contexts without
// copying target URLs or provider-supplied descriptions into model context.
type GitHubCommitStatusResult struct {
	Context string `json:"context"`
	State   string `json:"state"`
}

// GitHubPullRequestChecksResult binds check/status inspection to the head SHA
// returned by GitHub for the requested PR. The model never supplies this SHA.
type GitHubPullRequestChecksResult struct {
	Remote                  string                     `json:"remote"`
	Repository              string                     `json:"repository"`
	PullRequest             int                        `json:"pull_request"`
	Head                    string                     `json:"head"`
	CheckRuns               []GitHubCheckRunResult     `json:"check_runs"`
	CheckRunsTruncated      bool                       `json:"check_runs_truncated,omitempty"`
	CombinedStatus          string                     `json:"combined_status"`
	CommitStatuses          []GitHubCommitStatusResult `json:"commit_statuses"`
	CommitStatusesTruncated bool                       `json:"commit_statuses_truncated,omitempty"`
}

type githubPullRequestReadResponse struct {
	Number         int    `json:"number"`
	HTMLURL        string `json:"html_url"`
	Title          string `json:"title"`
	Draft          bool   `json:"draft"`
	State          string `json:"state"`
	Merged         bool   `json:"merged"`
	Mergeable      *bool  `json:"mergeable"`
	MergeableState string `json:"mergeable_state"`
	UpdatedAt      string `json:"updated_at"`
	User           struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type githubCheckRunsResponse struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"app"`
	} `json:"check_runs"`
}

type githubCombinedStatusResponse struct {
	State      string `json:"state"`
	TotalCount int    `json:"total_count"`
	Statuses   []struct {
		Context string `json:"context"`
		State   string `json:"state"`
	} `json:"statuses"`
}

func remoteSupportsGitHubPullRequestRead(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestRead && remote.TokenEnv != ""
}

func (s *RemoteService) githubPullRequestReadConfig(remoteID string) (RemoteConfig, string, string, string, error) {
	if !s.GitHubPullRequestReadAccessEnabled() {
		return RemoteConfig{}, "", "", "", errGitHubPullRequestReadDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q is not configured", remoteID)
	}
	owner, repository, ok := githubRepositoryFromRemote(remote)
	if !ok || !remoteSupportsGitHubPullRequestRead(remote) {
		return RemoteConfig{}, "", "", "", fmt.Errorf("remote %q does not allow GitHub pull request inspection", remoteID)
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

// GetPullRequest returns one PR's bounded hosted metadata.
func (s *RemoteService) GetPullRequest(ctx context.Context, remoteID string, number int) (*GitHubPullRequestReadResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}
	response, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	return githubPullRequestReadResult(strings.TrimSpace(remoteID), remote.Repository, response), nil
}

// ListPullRequests returns one bounded first page sorted by most recently updated.
// Optional headBranch is converted to GitHub's owner:branch filter internally.
func (s *RemoteService) ListPullRequests(ctx context.Context, remoteID, state, headBranch string, limit int) (*GitHubPullRequestListResult, error) {
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "open"
	}
	switch state {
	case "open", "closed", "all":
	default:
		return nil, fmt.Errorf("pull request state must be open, closed, or all")
	}
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > maxGitHubPullRequestListResults {
		return nil, fmt.Errorf("pull request list limit must be between 1 and %d", maxGitHubPullRequestListResults)
	}
	headBranch = strings.TrimSpace(headBranch)
	if headBranch != "" {
		clean, _, cleanErr := cleanBranchName(headBranch)
		if cleanErr != nil || clean != headBranch {
			return nil, fmt.Errorf("head_branch is not a valid branch name")
		}
	}

	query := url.Values{}
	query.Set("state", state)
	query.Set("sort", "updated")
	query.Set("direction", "desc")
	query.Set("per_page", strconv.Itoa(limit+1))
	if headBranch != "" {
		query.Set("head", owner+":"+headBranch)
	}
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls?%s", owner, repository, query.Encode())
	var responses []githubPullRequestReadResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &responses); err != nil {
		return nil, fmt.Errorf("GitHub pull request inventory could not be inspected")
	}
	truncated := len(responses) > limit
	if truncated {
		responses = responses[:limit]
	}
	pulls := make([]GitHubPullRequestReadResult, 0, len(responses))
	for i := range responses {
		pulls = append(pulls, *githubPullRequestReadResult(strings.TrimSpace(remoteID), remote.Repository, &responses[i]))
	}
	return &GitHubPullRequestListResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, State: state,
		HeadBranch: headBranch, PullRequests: pulls, Truncated: truncated,
	}, nil
}

// GetPullRequestChecks fetches the PR first and then inspects both Checks API
// runs and legacy/combined commit statuses for that exact returned head SHA.
func (s *RemoteService) GetPullRequestChecks(ctx context.Context, remoteID string, number int) (*GitHubPullRequestChecksResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
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

	checkQuery := url.Values{}
	checkQuery.Set("filter", "latest")
	checkQuery.Set("per_page", strconv.Itoa(maxGitHubCheckResults+1))
	checkEndpoint := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?%s", owner, repository, pull.Head.SHA, checkQuery.Encode())
	var checks githubCheckRunsResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, checkEndpoint, nil, http.StatusOK, &checks); err != nil {
		return nil, fmt.Errorf("GitHub check runs could not be inspected")
	}

	statusQuery := url.Values{}
	statusQuery.Set("per_page", strconv.Itoa(maxGitHubStatusResults+1))
	statusEndpoint := fmt.Sprintf("/repos/%s/%s/commits/%s/status?%s", owner, repository, pull.Head.SHA, statusQuery.Encode())
	var combined githubCombinedStatusResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, statusEndpoint, nil, http.StatusOK, &combined); err != nil {
		return nil, fmt.Errorf("GitHub commit status could not be inspected")
	}

	checkLimit := len(checks.CheckRuns)
	checkTruncated := checks.TotalCount > maxGitHubCheckResults || checkLimit > maxGitHubCheckResults
	if checkLimit > maxGitHubCheckResults {
		checkLimit = maxGitHubCheckResults
	}
	checkRuns := make([]GitHubCheckRunResult, 0, checkLimit)
	for _, check := range checks.CheckRuns[:checkLimit] {
		app := check.App.Slug
		if app == "" {
			app = check.App.Name
		}
		checkRuns = append(checkRuns, GitHubCheckRunResult{Name: check.Name, Status: check.Status, Conclusion: check.Conclusion, App: app})
	}

	statusLimit := len(combined.Statuses)
	statusTruncated := combined.TotalCount > maxGitHubStatusResults || statusLimit > maxGitHubStatusResults
	if statusLimit > maxGitHubStatusResults {
		statusLimit = maxGitHubStatusResults
	}
	statuses := make([]GitHubCommitStatusResult, 0, statusLimit)
	for _, status := range combined.Statuses[:statusLimit] {
		statuses = append(statuses, GitHubCommitStatusResult{Context: status.Context, State: status.State})
	}

	return &GitHubPullRequestChecksResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number, Head: strings.ToLower(pull.Head.SHA),
		CheckRuns: checkRuns, CheckRunsTruncated: checkTruncated, CombinedStatus: combined.State,
		CommitStatuses: statuses, CommitStatusesTruncated: statusTruncated,
	}, nil
}

func (s *RemoteService) getGitHubPullRequest(ctx context.Context, token, owner, repository string, number int) (*githubPullRequestReadResponse, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repository, number)
	var response githubPullRequestReadResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &response); err != nil {
		return nil, fmt.Errorf("GitHub pull request could not be inspected")
	}
	if response.Number != number || response.HTMLURL == "" || response.Head.Ref == "" || response.Head.SHA == "" || response.Base.Ref == "" {
		return nil, fmt.Errorf("GitHub pull request response was incomplete")
	}
	return &response, nil
}

func githubPullRequestReadResult(remoteID, repository string, response *githubPullRequestReadResponse) *GitHubPullRequestReadResult {
	if response == nil {
		return nil
	}
	return &GitHubPullRequestReadResult{
		Remote: remoteID, Repository: repository, Number: response.Number, URL: response.HTMLURL,
		Title: response.Title, State: response.State, Draft: response.Draft, Merged: response.Merged,
		Mergeable: response.Mergeable, MergeableState: response.MergeableState,
		HeadBranch: response.Head.Ref, Head: strings.ToLower(response.Head.SHA), BaseBranch: response.Base.Ref,
		Author: response.User.Login, UpdatedAt: response.UpdatedAt,
	}
}
