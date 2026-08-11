package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	githubAPIBaseURL                     = "https://api.github.com"
	maxGitHubAPIResponseBytes      int64 = 1 << 20
	maxGitHubPullRequestTitleRunes       = 256
	maxGitHubPullRequestBodyBytes        = 32 << 10
)

var (
	errGitHubPullRequestDisabled = errors.New("GitHub pull request creation is disabled")
	githubOwnerPattern           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,99}$`)
	githubRepositoryPattern      = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
)

// GitHubPullRequestCreator is the GitHub-specific remote collaboration contract.
// Repository identity and credentials come from the selected operator-configured
// Git remote; callers never provide a GitHub API URL, repository, token, or base.
type GitHubPullRequestCreator interface {
	CreateDraftPullRequest(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest, title, body string) (*GitHubPullRequestResult, error)
}

// GitHubPullRequestResult is safe to expose to the model. It omits the configured
// Git endpoint, API token reference/value, and local filesystem destination.
type GitHubPullRequestResult struct {
	Remote        string `json:"remote"`
	Repository    string `json:"repository"`
	Number        int    `json:"number"`
	URL           string `json:"url"`
	HeadBranch    string `json:"head_branch"`
	Head          string `json:"head"`
	BaseBranch    string `json:"base_branch"`
	Draft         bool   `json:"draft"`
	Created       bool   `json:"created"`
	AlreadyExists bool   `json:"already_exists,omitempty"`
}

type githubPullRequestResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Draft   bool   `json:"draft"`
	State   string `json:"state"`
	Head    struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type githubPullRequestCreatePayload struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body,omitempty"`
	Draft bool   `json:"draft"`
}

// CreateDraftPullRequest creates only a draft PR from the exact current local
// branch after proving that the same remote branch still equals expectedHead and
// that the reviewed complete remote branch namespace is unchanged. The base is
// the remote's advertised HEAD symref; callers cannot select another base.
func (s *RemoteService) CreateDraftPullRequest(ctx context.Context, remoteID, expectedBranch, expectedHead, expectedRemoteStateDigest, title, body string) (*GitHubPullRequestResult, error) {
	if !s.GitHubPullRequestMutationEnabled() {
		return nil, errGitHubPullRequestDisabled
	}
	remoteID = strings.TrimSpace(remoteID)
	remote, ok := s.remotes[remoteID]
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured", remoteID)
	}
	if !remote.AllowPullRequestCreate {
		return nil, fmt.Errorf("remote %q does not allow GitHub pull request creation", remoteID)
	}
	owner, repository, ok := githubRepositoryFromRemote(remote)
	if !ok || remote.TokenEnv == "" {
		return nil, fmt.Errorf("remote %q is not configured for GitHub pull request creation", remoteID)
	}
	if s.githubClient == nil || s.transport == nil {
		return nil, fmt.Errorf("GitHub pull request transport is unavailable")
	}
	if !validRemoteStateDigest(expectedRemoteStateDigest) {
		return nil, fmt.Errorf("expected_remote_state_digest must be the branch-state digest from git_remote_status")
	}
	if !validRemoteHash(expectedHead) {
		return nil, fmt.Errorf("expected_head must be the local HEAD hash from git_status")
	}
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxGitHubPullRequestTitleRunes {
		return nil, fmt.Errorf("pull request title must contain 1-%d characters", maxGitHubPullRequestTitleRunes)
	}
	if len([]byte(body)) > maxGitHubPullRequestBodyBytes {
		return nil, fmt.Errorf("pull request body exceeds the guarded %d-byte limit", maxGitHubPullRequestBodyBytes)
	}
	token, exists := s.lookupEnv(remote.TokenEnv)
	if !exists || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("remote %q GitHub credentials are unavailable", remoteID)
	}

	// Serialize with OmniLLM local Git mutations and recheck local state again
	// immediately before the external API mutation. Other Git clients may still
	// race, so the created PR's returned source SHA is verified after creation.
	s.local.writeMu.Lock()
	defer s.local.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo, err := s.local.open(remote.Repository)
	if err != nil {
		return nil, err
	}
	localHeadString, err := ensureExpectedHead(repo, expectedHead)
	if err != nil {
		return nil, err
	}
	branch, err := ensureExpectedBranch(repo, expectedBranch)
	if err != nil {
		return nil, err
	}
	_, branchRef, err := cleanBranchName(branch)
	if err != nil {
		return nil, fmt.Errorf("current branch cannot be used for a GitHub pull request")
	}
	localHead := plumbing.NewHash(localHeadString)
	if _, err := repo.CommitObject(localHead); err != nil {
		return nil, fmt.Errorf("local HEAD commit could not be read")
	}

	advertised, err := s.advertiseRemoteForGitHub(ctx, remoteID, remote)
	if err != nil {
		return nil, err
	}
	if remoteBranchStateDigest(advertised) != strings.ToLower(strings.TrimSpace(expectedRemoteStateDigest)) {
		return nil, fmt.Errorf("remote branch state changed; run git_remote_status again before creating a pull request")
	}
	remoteHead, exists := advertised.References[branchRef.String()]
	if !exists || remoteHead != localHead {
		return nil, fmt.Errorf("remote branch %q must exist at the exact reviewed local HEAD before creating a pull request", branch)
	}
	baseBranch, ok := advertisedDefaultBranch(advertised)
	if !ok {
		return nil, fmt.Errorf("remote default branch could not be determined safely")
	}
	if baseBranch == branch {
		return nil, fmt.Errorf("current branch is the remote default branch and cannot be used as a pull request head")
	}

	if _, err := ensureExpectedHead(repo, expectedHead); err != nil {
		return nil, err
	}
	if _, err := ensureExpectedBranch(repo, expectedBranch); err != nil {
		return nil, err
	}

	existing, err := s.findOpenGitHubPullRequest(ctx, token, owner, repository, branch, baseBranch)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !strings.EqualFold(existing.Head.SHA, localHeadString) {
			return nil, fmt.Errorf("an open pull request already exists for branch %q at a different head; refresh Git state before continuing", branch)
		}
		return githubPullRequestResult(remoteID, remote.Repository, branch, localHeadString, baseBranch, existing, false, true), nil
	}

	created, err := s.createGitHubDraftPullRequest(ctx, token, owner, repository, branch, baseBranch, title, body)
	if err != nil {
		return nil, err
	}
	if created.Number <= 0 || created.HTMLURL == "" || !created.Draft || created.State != "open" ||
		created.Head.Ref != branch || !strings.EqualFold(created.Head.SHA, localHeadString) || created.Base.Ref != baseBranch {
		cleanupErr := s.closeGitHubPullRequest(ctx, token, owner, repository, created.Number)
		if cleanupErr != nil {
			return nil, fmt.Errorf("GitHub draft pull request validation failed and automatic cleanup could not be confirmed; review pull request #%d", created.Number)
		}
		return nil, fmt.Errorf("GitHub draft pull request validation failed; the unexpected draft was closed")
	}
	return githubPullRequestResult(remoteID, remote.Repository, branch, localHeadString, baseBranch, created, true, false), nil
}

func (s *RemoteService) advertiseRemoteForGitHub(ctx context.Context, remoteID string, remote RemoteConfig) (*packp.AdvRefs, error) {
	endpoint, err := transport.NewEndpoint(remote.URL)
	if err != nil {
		return nil, fmt.Errorf("remote %q endpoint is invalid", remoteID)
	}
	auth, err := s.remoteAuth(remoteID, remote)
	if err != nil {
		return nil, err
	}
	session, err := s.transport.NewUploadPackSession(endpoint, auth)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be opened before pull request creation", remoteID)
	}
	defer session.Close()
	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote %q could not be inspected before pull request creation", remoteID)
	}
	return advertised, nil
}

func advertisedDefaultBranch(advertised *packp.AdvRefs) (string, bool) {
	if advertised == nil || advertised.Capabilities == nil {
		return "", false
	}
	for _, value := range advertised.Capabilities.Get(capability.SymRef) {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 || parts[0] != "HEAD" || !strings.HasPrefix(parts[1], "refs/heads/") {
			continue
		}
		branch := strings.TrimPrefix(parts[1], "refs/heads/")
		clean, ref, err := cleanBranchName(branch)
		if err != nil || clean != branch {
			return "", false
		}
		if hash, exists := advertised.References[ref.String()]; !exists || hash == plumbing.ZeroHash {
			return "", false
		}
		return branch, true
	}
	return "", false
}

func githubRepositoryFromRemote(remote RemoteConfig) (string, string, bool) {
	parsed, err := url.Parse(remote.URL)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.RawPath != "" {
		return "", "", false
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	owner := parts[0]
	repository := strings.TrimSuffix(parts[1], ".git")
	if !githubOwnerPattern.MatchString(owner) || !githubRepositoryPattern.MatchString(repository) {
		return "", "", false
	}
	return owner, repository, true
}

func remoteSupportsGitHubPullRequests(remote RemoteConfig) bool {
	_, _, ok := githubRepositoryFromRemote(remote)
	return ok && remote.AllowPullRequestCreate && remote.TokenEnv != ""
}

func newGitHubAPIClient() *http.Client {
	base := &http.Transport{
		DialContext:           remoteSafeDialContext(),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: boundedRemoteRoundTripper{base: base, max: maxGitHubAPIResponseBytes},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("GitHub API redirects are disabled")
		},
	}
}

func (s *RemoteService) findOpenGitHubPullRequest(ctx context.Context, token, owner, repository, headBranch, baseBranch string) (*githubPullRequestResponse, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+headBranch)
	query.Set("base", baseBranch)
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls?%s", owner, repository, query.Encode())
	var responses []githubPullRequestResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &responses); err != nil {
		return nil, fmt.Errorf("GitHub pull request inventory could not be checked")
	}
	if len(responses) == 0 {
		return nil, nil
	}
	return &responses[0], nil
}

func (s *RemoteService) createGitHubDraftPullRequest(ctx context.Context, token, owner, repository, headBranch, baseBranch, title, body string) (*githubPullRequestResponse, error) {
	payload := githubPullRequestCreatePayload{Title: title, Head: headBranch, Base: baseBranch, Body: body, Draft: true}
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls", owner, repository)
	var response githubPullRequestResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodPost, endpoint, payload, http.StatusCreated, &response); err != nil {
		return nil, fmt.Errorf("GitHub draft pull request could not be created")
	}
	return &response, nil
}

func (s *RemoteService) closeGitHubPullRequest(ctx context.Context, token, owner, repository string, number int) error {
	if number <= 0 {
		return fmt.Errorf("invalid pull request number")
	}
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repository, number)
	var response githubPullRequestResponse
	return s.doGitHubJSON(ctx, token, http.MethodPatch, endpoint, map[string]string{"state": "closed"}, http.StatusOK, &response)
}

func (s *RemoteService) doGitHubJSON(ctx context.Context, token, method, endpoint string, payload any, expectedStatus int, out any) error {
	if s == nil || s.githubClient == nil || !strings.HasPrefix(endpoint, "/repos/") {
		return fmt.Errorf("GitHub API request is unavailable")
	}
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, githubAPIBaseURL+endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.githubClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		return fmt.Errorf("unexpected GitHub API status %d", response.StatusCode)
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}

func githubPullRequestResult(remoteID, repositoryID, headBranch, head, baseBranch string, response *githubPullRequestResponse, created, alreadyExists bool) *GitHubPullRequestResult {
	return &GitHubPullRequestResult{
		Remote: remoteID, Repository: repositoryID, Number: response.Number, URL: response.HTMLURL,
		HeadBranch: headBranch, Head: head, BaseBranch: baseBranch, Draft: response.Draft,
		Created: created, AlreadyExists: alreadyExists,
	}
}
