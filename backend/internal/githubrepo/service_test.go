package githubrepo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeTokenProvider struct {
	token string
	err   error
	users []string
}

func (f *fakeTokenProvider) AccessToken(_ context.Context, userID string) (string, error) {
	f.users = append(f.users, userID)
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func TestListUsesFixedUserRepositoryEndpointAndBoundsPagination(t *testing.T) {
	provider := &fakeTokenProvider{token: "user-token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("page = %q", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Fatalf("per_page = %q", got)
		}
		if got := r.URL.Query().Get("affiliation"); got != "owner,collaborator,organization_member" {
			t.Fatalf("affiliation = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer user-token" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != githubAPIVersion {
			t.Fatalf("api version = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":7,"name":"repo","full_name":"octo/repo","private":true,"default_branch":"main","permissions":{"pull":true,"push":true}}]`))
	}))
	defer server.Close()

	service := newServiceForTest(provider, server.Client(), server.URL)
	page, err := service.List(context.Background(), "user-1", -5, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.users) != 1 || provider.users[0] != "user-1" {
		t.Fatalf("token owner = %#v", provider.users)
	}
	if page.Page != 1 || page.PerPage != 100 || page.HasMore {
		t.Fatalf("unexpected page: %#v", page)
	}
	if len(page.Repositories) != 1 || page.Repositories[0].FullName != "octo/repo" || !page.Repositories[0].Permissions.Push {
		t.Fatalf("unexpected repositories: %#v", page.Repositories)
	}
}

func TestGetUsesNumericRepositoryEndpoint(t *testing.T) {
	provider := &fakeTokenProvider{token: "token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/42" || r.URL.RawQuery != "" {
			t.Fatalf("unexpected repository request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":42,"name":"bound","full_name":"octo/bound","default_branch":"main"}`))
	}))
	defer server.Close()

	service := newServiceForTest(provider, server.Client(), server.URL)
	repository, err := service.Get(context.Background(), "user-2", 42)
	if err != nil {
		t.Fatal(err)
	}
	if repository.ID != 42 || repository.FullName != "octo/bound" {
		t.Fatalf("unexpected repository: %#v", repository)
	}
}

func TestGetMapsNotFoundWithoutProviderBodyLeak(t *testing.T) {
	provider := &fakeTokenProvider{token: "token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"private-secret-provider-detail"}`))
	}))
	defer server.Close()

	service := newServiceForTest(provider, server.Client(), server.URL)
	_, err := service.Get(context.Background(), "user-3", 99)
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if strings.Contains(err.Error(), "private-secret-provider-detail") {
		t.Fatalf("provider body leaked: %v", err)
	}
}

func TestListPropagatesCredentialFailureWithoutNetwork(t *testing.T) {
	tokenErr := errors.New("credential unavailable")
	provider := &fakeTokenProvider{err: tokenErr}
	service := newServiceForTest(provider, &http.Client{}, "http://127.0.0.1:1")
	_, err := service.List(context.Background(), "user-4", 1, 30)
	if !errors.Is(err, tokenErr) {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestListRejectsOversizedResponse(t *testing.T) {
	provider := &fakeTokenProvider{token: "token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[\"" + strings.Repeat("x", int(maxResponseBytes)) + "\"]"))
	}))
	defer server.Close()

	service := newServiceForTest(provider, server.Client(), server.URL)
	_, err := service.List(context.Background(), "user-5", 1, 30)
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected size error, got %v", err)
	}
}
