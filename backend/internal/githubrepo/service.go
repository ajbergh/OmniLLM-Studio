package githubrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPIBase       = "https://api.github.com"
	githubAPIVersion    = "2022-11-28"
	defaultPerPage      = 30
	maxPerPage          = 100
	maxResponseBytes    = int64(2 << 20)
	repositoryUserAgent = "OmniLLM-Studio"
)

var ErrRepositoryNotFound = errors.New("GitHub repository was not found")

// TokenProvider supplies a backend-only user access token for GitHub API calls.
// Implementations may refresh tokens as needed but must never expose them to API
// responses or model/tool results.
type TokenProvider interface {
	AccessToken(ctx context.Context, userID string) (string, error)
}

// Permissions is the authenticated user's repository permission summary.
type Permissions struct {
	Admin    bool `json:"admin"`
	Maintain bool `json:"maintain"`
	Push     bool `json:"push"`
	Triage   bool `json:"triage"`
	Pull     bool `json:"pull"`
}

// Repository is the bounded, secret-free repository metadata needed for user
// selection and later explicit local-worktree binding.
type Repository struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	Private       bool        `json:"private"`
	Fork          bool        `json:"fork"`
	Archived      bool        `json:"archived"`
	Disabled      bool        `json:"disabled"`
	DefaultBranch string      `json:"default_branch"`
	Permissions   Permissions `json:"permissions"`
}

// Page is one bounded page of repositories visible to the connected GitHub user.
type Page struct {
	Repositories []Repository `json:"repositories"`
	Page         int          `json:"page"`
	PerPage      int          `json:"per_page"`
	HasMore      bool         `json:"has_more"`
}

// Service performs fixed-endpoint, bounded GitHub repository discovery using a
// connected user's credential. It never accepts arbitrary provider URLs.
type Service struct {
	tokens  TokenProvider
	client  *http.Client
	baseURL string
}

func NewService(tokens TokenProvider) *Service {
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &Service{
		tokens: tokens,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("GitHub repository redirects are disabled")
			},
		},
		baseURL: githubAPIBase,
	}
}

func newServiceForTest(tokens TokenProvider, client *http.Client, baseURL string) *Service {
	return &Service{tokens: tokens, client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// List returns repositories explicitly visible to the connected GitHub user.
// Pagination is caller-controlled only within fixed service bounds.
func (s *Service) List(ctx context.Context, userID string, page, perPage int) (Page, error) {
	if s == nil || s.tokens == nil || s.client == nil {
		return Page{}, fmt.Errorf("GitHub repository discovery is unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Page{}, fmt.Errorf("user ID is required")
	}
	page, perPage = normalizePagination(page, perPage)
	token, err := s.tokens.AccessToken(ctx, userID)
	if err != nil {
		return Page{}, err
	}
	values := url.Values{}
	values.Set("affiliation", "owner,collaborator,organization_member")
	values.Set("visibility", "all")
	values.Set("sort", "full_name")
	values.Set("direction", "asc")
	values.Set("page", strconv.Itoa(page))
	values.Set("per_page", strconv.Itoa(perPage))

	var repositories []Repository
	if err := s.getJSON(ctx, token, "/user/repos?"+values.Encode(), &repositories); err != nil {
		return Page{}, err
	}
	clean := make([]Repository, 0, len(repositories))
	for _, repository := range repositories {
		repository = normalizeRepository(repository)
		if repository.ID <= 0 || repository.Name == "" || !validFullName(repository.FullName) {
			continue
		}
		clean = append(clean, repository)
	}
	return Page{
		Repositories: clean,
		Page:         page,
		PerPage:      perPage,
		HasMore:      len(repositories) == perPage,
	}, nil
}

// Get fetches one repository by immutable GitHub numeric ID. The endpoint is
// fixed and the ID is numeric, so callers cannot inject hostnames or paths.
func (s *Service) Get(ctx context.Context, userID string, repositoryID int64) (Repository, error) {
	if s == nil || s.tokens == nil || s.client == nil {
		return Repository{}, fmt.Errorf("GitHub repository discovery is unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Repository{}, fmt.Errorf("user ID is required")
	}
	if repositoryID <= 0 {
		return Repository{}, fmt.Errorf("GitHub repository ID is required")
	}
	token, err := s.tokens.AccessToken(ctx, userID)
	if err != nil {
		return Repository{}, err
	}
	var repository Repository
	if err := s.getJSON(ctx, token, "/repositories/"+strconv.FormatInt(repositoryID, 10), &repository); err != nil {
		return Repository{}, err
	}
	repository = normalizeRepository(repository)
	if repository.ID != repositoryID || repository.Name == "" || !validFullName(repository.FullName) {
		return Repository{}, fmt.Errorf("GitHub returned invalid repository metadata")
	}
	return repository, nil
}

func (s *Service) getJSON(ctx context.Context, token, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", repositoryUserAgent)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub repository request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrRepositoryNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub repository request was rejected")
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("GitHub repository response could not be read")
	}
	if int64(len(payload)) > maxResponseBytes {
		return fmt.Errorf("GitHub repository response exceeded the configured limit")
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("GitHub returned an invalid repository response")
	}
	return nil
}

func normalizePagination(page, perPage int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage
}

func normalizeRepository(repository Repository) Repository {
	repository.Name = strings.TrimSpace(repository.Name)
	repository.FullName = strings.TrimSpace(repository.FullName)
	repository.DefaultBranch = strings.TrimSpace(repository.DefaultBranch)
	return repository
}

func validFullName(value string) bool {
	if value == "" || len(value) > 256 || strings.Count(value, "/") != 1 {
		return false
	}
	parts := strings.SplitN(value, "/", 2)
	return strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != ""
}
