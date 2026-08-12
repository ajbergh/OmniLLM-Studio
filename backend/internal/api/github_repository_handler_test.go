package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/githubrepo"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

type fakeGitHubRepositoryDiscovery struct {
	page       githubrepo.Page
	repository githubrepo.Repository
	err        error
	users      []string
	getIDs     []int64
}

func (f *fakeGitHubRepositoryDiscovery) List(_ context.Context, userID string, page, perPage int) (githubrepo.Page, error) {
	f.users = append(f.users, userID)
	return f.page, f.err
}

func (f *fakeGitHubRepositoryDiscovery) Get(_ context.Context, userID string, repositoryID int64) (githubrepo.Repository, error) {
	f.users = append(f.users, userID)
	f.getIDs = append(f.getIDs, repositoryID)
	return f.repository, f.err
}

type fakeGitHubRepositoryConnection struct {
	status githubauth.Status
	err    error
	users  []string
}

func (f *fakeGitHubRepositoryConnection) Status(userID string) (githubauth.Status, error) {
	f.users = append(f.users, userID)
	return f.status, f.err
}

type fakeGitHubRepositoryBindingStore struct {
	bindings []repository.GitHubRepositoryBinding
	err      error
	owners   []string
	saved    []repository.GitHubRepositoryBinding
	deleted  []string
}

func (f *fakeGitHubRepositoryBindingStore) List(ownerID string) ([]repository.GitHubRepositoryBinding, error) {
	f.owners = append(f.owners, ownerID)
	return append([]repository.GitHubRepositoryBinding(nil), f.bindings...), f.err
}

func (f *fakeGitHubRepositoryBindingStore) Upsert(ownerID string, binding repository.GitHubRepositoryBinding) error {
	f.owners = append(f.owners, ownerID)
	f.saved = append(f.saved, binding)
	return f.err
}

func (f *fakeGitHubRepositoryBindingStore) Delete(ownerID, localRepositoryID string) error {
	f.owners = append(f.owners, ownerID)
	f.deleted = append(f.deleted, localRepositoryID)
	return f.err
}

type fakeLocalGitRepositoryCatalog struct {
	ids map[string]bool
}

func (f *fakeLocalGitRepositoryCatalog) RepositoryIDs() []string {
	result := make([]string, 0, len(f.ids))
	for _, id := range []string{"alpha", "omni", "zeta"} {
		if f.ids[id] {
			result = append(result, id)
		}
	}
	return result
}

func (f *fakeLocalGitRepositoryCatalog) HasRepository(repositoryID string) bool {
	return f.ids[repositoryID]
}

func withRepositoryUser(request *http.Request, userID string) *http.Request {
	ctx := context.WithValue(request.Context(), auth.ContextKeyUser, &models.User{ID: userID})
	return request.WithContext(ctx)
}

func withChiParam(request *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx))
}

func TestGitHubRepositoryHandlerListsForAuthenticatedOwner(t *testing.T) {
	discovery := &fakeGitHubRepositoryDiscovery{page: githubrepo.Page{
		Repositories: []githubrepo.Repository{{ID: 7, Name: "repo", FullName: "octo/repo"}},
		Page: 2, PerPage: 10,
	}}
	handler := NewGitHubRepositoryHandler(discovery, nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := withRepositoryUser(httptest.NewRequest(http.MethodGet, "/v1/github/repositories?page=2&per_page=10", nil), "user-7")
	handler.ListRepositories(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected response: %d headers=%v", recorder.Code, recorder.Header())
	}
	if len(discovery.users) != 1 || discovery.users[0] != "user-7" {
		t.Fatalf("discovery escaped owner scope: %#v", discovery.users)
	}
	if strings.Contains(recorder.Body.String(), "token") || !strings.Contains(recorder.Body.String(), "octo/repo") {
		t.Fatalf("unexpected repository response: %s", recorder.Body.String())
	}
}

func TestGitHubRepositoryHandlerBindsDiscoveredRepositoryToAllowlistedLocalID(t *testing.T) {
	discovery := &fakeGitHubRepositoryDiscovery{repository: githubrepo.Repository{
		ID: 42, Name: "studio", FullName: "octo/studio", Private: true, DefaultBranch: "main",
	}}
	connection := &fakeGitHubRepositoryConnection{status: githubauth.Status{Configured: true, Connected: true, GitHubUserID: 99, GitHubLogin: "octo"}}
	store := &fakeGitHubRepositoryBindingStore{}
	locals := &fakeLocalGitRepositoryCatalog{ids: map[string]bool{"omni": true}}
	handler := NewGitHubRepositoryHandler(discovery, connection, store, locals)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/github/repository-bindings/omni", strings.NewReader(`{"github_repository_id":42}`))
	request = withRepositoryUser(request, "user-9")
	request = withChiParam(request, "localRepositoryId", "omni")
	handler.Bind(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("bind response: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(discovery.getIDs) != 1 || discovery.getIDs[0] != 42 || len(store.saved) != 1 {
		t.Fatalf("binding was not resolved server-side: ids=%v saved=%#v", discovery.getIDs, store.saved)
	}
	got := store.saved[0]
	if got.LocalRepositoryID != "omni" || got.GitHubUserID != 99 || got.GitHubRepositoryID != 42 || got.GitHubFullName != "octo/studio" || !got.Private {
		t.Fatalf("unexpected saved binding: %#v", got)
	}
	if len(store.owners) != 1 || store.owners[0] != "user-9" {
		t.Fatalf("binding escaped owner scope: %#v", store.owners)
	}
}

func TestGitHubRepositoryHandlerRejectsUnconfiguredLocalRepositoryBeforeDiscovery(t *testing.T) {
	discovery := &fakeGitHubRepositoryDiscovery{repository: githubrepo.Repository{ID: 42, Name: "repo", FullName: "octo/repo"}}
	handler := NewGitHubRepositoryHandler(discovery, &fakeGitHubRepositoryConnection{}, &fakeGitHubRepositoryBindingStore{}, &fakeLocalGitRepositoryCatalog{ids: map[string]bool{"omni": true}})
	recorder := httptest.NewRecorder()
	request := withChiParam(httptest.NewRequest(http.MethodPut, "/v1/github/repository-bindings/unknown", strings.NewReader(`{"github_repository_id":42}`)), "localRepositoryId", "unknown")
	handler.Bind(recorder, request)
	if recorder.Code != http.StatusNotFound || len(discovery.getIDs) != 0 {
		t.Fatalf("unexpected response/discovery: %d ids=%v", recorder.Code, discovery.getIDs)
	}
}

func TestGitHubRepositoryHandlerListsStaleAccountAndLocalBindingsWithoutPaths(t *testing.T) {
	store := &fakeGitHubRepositoryBindingStore{bindings: []repository.GitHubRepositoryBinding{
		{LocalRepositoryID: "omni", GitHubUserID: 10, GitHubRepositoryID: 100, GitHubFullName: "old/repo"},
		{LocalRepositoryID: "removed", GitHubUserID: 20, GitHubRepositoryID: 200, GitHubFullName: "current/repo"},
	}}
	connection := &fakeGitHubRepositoryConnection{status: githubauth.Status{Configured: true, Connected: true, GitHubUserID: 20}}
	locals := &fakeLocalGitRepositoryCatalog{ids: map[string]bool{"omni": true}}
	handler := NewGitHubRepositoryHandler(nil, connection, store, locals)
	recorder := httptest.NewRecorder()
	request := withRepositoryUser(httptest.NewRequest(http.MethodGet, "/v1/github/repository-bindings", nil), "owner")
	handler.ListBindings(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list response: %d %s", recorder.Code, recorder.Body.String())
	}
	var response githubRepositoryBindingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Bindings) != 2 || response.Bindings[0].AccountMatches || !response.Bindings[0].LocalConfigured || !response.Bindings[1].AccountMatches || response.Bindings[1].LocalConfigured {
		t.Fatalf("unexpected binding status: %#v", response.Bindings)
	}
	if strings.Contains(recorder.Body.String(), "/tmp/") || strings.Contains(recorder.Body.String(), `C:\\`) {
		t.Fatalf("filesystem path leaked: %s", recorder.Body.String())
	}
}

func TestGitHubRepositoryHandlerMapsCredentialErrors(t *testing.T) {
	discovery := &fakeGitHubRepositoryDiscovery{err: githubauth.ErrReauthorizationRequired}
	handler := NewGitHubRepositoryHandler(discovery, nil, nil, nil)
	recorder := httptest.NewRecorder()
	handler.ListRepositories(recorder, httptest.NewRequest(http.MethodGet, "/v1/github/repositories", nil))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "reauthorization") {
		t.Fatalf("unexpected credential error response: %d %s", recorder.Code, recorder.Body.String())
	}

	discovery.err = errors.New("provider body leaked secret")
	recorder = httptest.NewRecorder()
	handler.ListRepositories(recorder, httptest.NewRequest(http.MethodGet, "/v1/github/repositories", nil))
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("provider error leaked: %d %s", recorder.Code, recorder.Body.String())
	}
}
