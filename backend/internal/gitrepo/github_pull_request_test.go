package gitrepo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestGitHubRepositoryFromRemoteIsGitHubComOnly(t *testing.T) {
	owner, repository, ok := githubRepositoryFromRemote(RemoteConfig{URL: "https://github.com/example-org/example.repo.git"})
	if !ok || owner != "example-org" || repository != "example.repo" {
		t.Fatalf("githubRepositoryFromRemote() = %q, %q, %v", owner, repository, ok)
	}
	for _, raw := range []string{
		"https://gitlab.com/example-org/repo.git",
		"https://github.com/example-org/repo/extra.git",
		"https://github.com/example-org/",
		"https://github.com/example-org/repo%2Fextra.git",
	} {
		if _, _, ok := githubRepositoryFromRemote(RemoteConfig{URL: raw}); ok {
			t.Fatalf("githubRepositoryFromRemote(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestGitHubPullRequestGateDoesNotRequireLocalWrite(t *testing.T) {
	local := NewService(map[string]string{"repo": t.TempDir()})
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", TokenEnv: "GITHUB_TOKEN", AllowPullRequestCreate: true},
	}, true, false, nil, nil)
	svc.local = local
	svc.githubPullRequestEnabled = true
	if !svc.GitHubPullRequestMutationEnabled() {
		t.Fatal("GitHubPullRequestMutationEnabled() = false with remote/read and PR gates enabled")
	}
	if svc.PushMutationEnabled() {
		t.Fatal("PushMutationEnabled() unexpectedly enabled without local write/push gates")
	}
}

func TestCreateDraftPullRequestBindsPublishedHeadAndDefaultBase(t *testing.T) {
	svc, head, advertised := newGitHubPullRequestTestService(t, "feature/pr")
	requests := make([]string, 0, 2)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.github.com" {
			t.Fatalf("GitHub API host = %q", request.URL.Host)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch request.Method {
		case http.MethodGet:
			if request.URL.Query().Get("head") != "example:feature/pr" || request.URL.Query().Get("base") != "main" {
				t.Fatalf("unexpected duplicate-check query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case http.MethodPost:
			var payload githubPullRequestCreatePayload
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if !payload.Draft || payload.Head != "feature/pr" || payload.Base != "main" || payload.Title != "Guarded PR" {
				t.Fatalf("unexpected create payload: %#v", payload)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"number":42,"html_url":"https://github.com/example/repo/pull/42","draft":true,"state":"open","head":{"ref":"feature/pr","sha":"`+head.String()+`"},"base":{"ref":"main"}}`), nil
		default:
			return nil, errors.New("unexpected GitHub API method")
		}
	})}

	result, err := svc.CreateDraftPullRequest(context.Background(), "origin", "feature/pr", head.String(), remoteBranchStateDigest(advertised), "Guarded PR", "Body")
	if err != nil {
		t.Fatalf("CreateDraftPullRequest() returned error: %v", err)
	}
	if !result.Created || result.AlreadyExists || !result.Draft || result.Number != 42 || result.Head != head.String() || result.BaseBranch != "main" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(requests) != 2 || requests[0] != "GET /repos/example/repo/pulls" || requests[1] != "POST /repos/example/repo/pulls" {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestCreateDraftPullRequestReusesMatchingOpenPullRequest(t *testing.T) {
	svc, head, advertised := newGitHubPullRequestTestService(t, "feature/existing-pr")
	postCalled := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			postCalled = true
		}
		return jsonHTTPResponse(http.StatusOK, `[{"number":7,"html_url":"https://github.com/example/repo/pull/7","draft":true,"state":"open","head":{"ref":"feature/existing-pr","sha":"`+head.String()+`"},"base":{"ref":"main"}}]`), nil
	})}

	result, err := svc.CreateDraftPullRequest(context.Background(), "origin", "feature/existing-pr", head.String(), remoteBranchStateDigest(advertised), "Duplicate", "")
	if err != nil {
		t.Fatalf("CreateDraftPullRequest() returned error: %v", err)
	}
	if result.Created || !result.AlreadyExists || result.Number != 7 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if postCalled {
		t.Fatal("duplicate PR check still issued a POST")
	}
}

func TestCreateDraftPullRequestClosesUnexpectedCreatedHead(t *testing.T) {
	svc, head, advertised := newGitHubPullRequestTestService(t, "feature/race")
	closed := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.Method {
		case http.MethodGet:
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case http.MethodPost:
			return jsonHTTPResponse(http.StatusCreated, `{"number":99,"html_url":"https://github.com/example/repo/pull/99","draft":true,"state":"open","head":{"ref":"feature/race","sha":"ffffffffffffffffffffffffffffffffffffffff"},"base":{"ref":"main"}}`), nil
		case http.MethodPatch:
			closed = true
			return jsonHTTPResponse(http.StatusOK, `{"number":99,"state":"closed"}`), nil
		default:
			return nil, errors.New("unexpected method")
		}
	})}

	_, err := svc.CreateDraftPullRequest(context.Background(), "origin", "feature/race", head.String(), remoteBranchStateDigest(advertised), "Race", "")
	if err == nil || !strings.Contains(err.Error(), "unexpected draft was closed") {
		t.Fatalf("CreateDraftPullRequest() error = %v", err)
	}
	if !closed {
		t.Fatal("unexpected PR head was not closed")
	}
}

func TestCreateDraftPullRequestRejectsStaleRemoteStateBeforeAPI(t *testing.T) {
	svc, head, _ := newGitHubPullRequestTestService(t, "feature/stale-pr")
	called := false
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("should not be called")
	})}
	_, err := svc.CreateDraftPullRequest(context.Background(), "origin", "feature/stale-pr", head.String(), strings.Repeat("a", 64), "Stale", "")
	if err == nil || !strings.Contains(err.Error(), "remote branch state changed") {
		t.Fatalf("CreateDraftPullRequest() error = %v", err)
	}
	if called {
		t.Fatal("GitHub API was called after stale remote-state rejection")
	}
}

func newGitHubPullRequestTestService(t *testing.T, branch string) (*RemoteService, plumbing.Hash, *packp.AdvRefs) {
	t.Helper()
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("base.txt"); err != nil {
		t.Fatal(err)
	}
	base, err := worktree.Commit("base", &git.CommitOptions{Author: &object.Signature{Name: "PR Test", Email: "pr-test@example.invalid", When: time.Unix(1_700_000_000, 0).UTC()}})
	if err != nil {
		t.Fatal(err)
	}
	branchRef := plumbing.NewBranchReferenceName(branch)
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: branchRef, Create: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("feature.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := worktree.Commit("feature", &git.CommitOptions{Author: &object.Signature{Name: "PR Test", Email: "pr-test@example.invalid", When: time.Unix(1_700_000_100, 0).UTC()}})
	if err != nil {
		t.Fatal(err)
	}

	advertised := packp.NewAdvRefs()
	advertised.References["refs/heads/main"] = base
	advertised.References[branchRef.String()] = head
	advertised.Head = &base
	if err := advertised.Capabilities.Add(capability.SymRef, "HEAD:refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	remoteTransport := &githubPullRequestTestTransport{advertised: advertised}
	local := NewService(map[string]string{"repo": repoDir})
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://github.com/example/repo.git", Username: "git", TokenEnv: "GITHUB_TOKEN", AllowPullRequestCreate: true},
	}, true, false, remoteTransport, func(name string) (string, bool) {
		if name == "GITHUB_TOKEN" {
			return "test-token", true
		}
		return "", false
	})
	svc.local = local
	svc.githubPullRequestEnabled = true
	return svc, head, advertised
}

type githubPullRequestTestTransport struct {
	advertised *packp.AdvRefs
}

func (t *githubPullRequestTestTransport) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (transport.UploadPackSession, error) {
	return &githubPullRequestTestUploadSession{advertised: t.advertised}, nil
}

func (t *githubPullRequestTestTransport) NewReceivePackSession(*transport.Endpoint, transport.AuthMethod) (transport.ReceivePackSession, error) {
	return nil, errors.New("receive-pack is not supported by PR test transport")
}

type githubPullRequestTestUploadSession struct {
	advertised *packp.AdvRefs
}

func (s *githubPullRequestTestUploadSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *githubPullRequestTestUploadSession) AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *githubPullRequestTestUploadSession) UploadPack(context.Context, *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	return nil, errors.New("upload-pack is not needed by PR tests")
}

func (s *githubPullRequestTestUploadSession) Close() error { return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
