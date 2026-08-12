package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestGitHubAuthToolBindingsFilterDifferentGitHubIdentity(t *testing.T) {
	credentialStore := &testGitHubAuthCredentialStore{credential: &githubauth.Credential{
		AccessToken:  "user-token",
		TokenType:    "bearer",
		GitHubUserID: 42,
		GitHubLogin:  "octocat",
	}}
	service := testGitHubAuthService(t, credentialStore)
	bindings := &fakeGitHubRepositoryBindingStore{bindings: []repository.GitHubRepositoryBinding{
		{LocalRepositoryID: "omni", GitHubUserID: 42, GitHubRepositoryID: 1, GitHubFullName: "octo/active"},
		{LocalRepositoryID: "other", GitHubUserID: 99, GitHubRepositoryID: 2, GitHubFullName: "other/stale"},
	}}
	options := githubAuthToolCredentialOptionsWithBindings(service, bindings)
	if options == nil || options.Bindings == nil {
		t.Fatal("expected binding callback")
	}
	active, err := options.Bindings(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].LocalRepositoryID != "omni" || active[0].GitHubFullName != "octo/active" {
		t.Fatalf("unexpected active bindings: %#v", active)
	}
	if len(bindings.owners) != 1 || bindings.owners[0] != "user-1" {
		t.Fatalf("binding lookup escaped owner scope: %#v", bindings.owners)
	}
}

func TestGitHubAuthToolBindingsPreserveStaleSameIdentityForFailClosedExecution(t *testing.T) {
	expired := time.Now().UTC().Add(-time.Hour)
	credentialStore := &testGitHubAuthCredentialStore{credential: &githubauth.Credential{
		AccessToken:     "expired-token",
		TokenType:       "bearer",
		AccessExpiresAt: &expired,
		GitHubUserID:    42,
		GitHubLogin:     "octocat",
	}}
	service := testGitHubAuthService(t, credentialStore)
	bindings := &fakeGitHubRepositoryBindingStore{bindings: []repository.GitHubRepositoryBinding{
		{LocalRepositoryID: "omni", GitHubUserID: 42, GitHubRepositoryID: 1, GitHubFullName: "octo/active"},
	}}
	options := githubAuthToolCredentialOptionsWithBindings(service, bindings)
	active, err := options.Bindings(context.Background(), "user-2")
	if err != nil || len(active) != 1 {
		t.Fatalf("stale same-identity binding disappeared: %#v err=%v", active, err)
	}
	if token, connected, err := options.Resolve(context.Background(), "user-2"); token != "" || !connected || !errors.Is(err, githubauth.ErrReauthorizationRequired) {
		t.Fatalf("execution did not retain fail-closed reauthorization: token=%q connected=%v err=%v", token, connected, err)
	}
}

func TestGitHubAuthToolBindingsDisappearAfterDisconnect(t *testing.T) {
	service := testGitHubAuthService(t, &testGitHubAuthCredentialStore{})
	bindings := &fakeGitHubRepositoryBindingStore{bindings: []repository.GitHubRepositoryBinding{
		{LocalRepositoryID: "omni", GitHubUserID: 42, GitHubRepositoryID: 1, GitHubFullName: "octo/old"},
	}}
	options := githubAuthToolCredentialOptionsWithBindings(service, bindings)
	active, err := options.Bindings(context.Background(), "user-3")
	if err != nil || len(active) != 0 {
		t.Fatalf("disconnected account retained bindings: %#v err=%v", active, err)
	}
	if len(bindings.owners) != 0 {
		t.Fatalf("binding store should not be queried without a persisted GitHub identity: %#v", bindings.owners)
	}
}

func TestGitHubAuthToolBindingsPropagateLocalStoreFailure(t *testing.T) {
	credentialStore := &testGitHubAuthCredentialStore{credential: &githubauth.Credential{
		AccessToken: "token", TokenType: "bearer", GitHubUserID: 42, GitHubLogin: "octo",
	}}
	service := testGitHubAuthService(t, credentialStore)
	storeErr := errors.New("binding database unavailable")
	bindings := &fakeGitHubRepositoryBindingStore{err: storeErr}
	options := githubAuthToolCredentialOptionsWithBindings(service, bindings)
	if _, err := options.Bindings(context.Background(), "user-4"); !errors.Is(err, storeErr) {
		t.Fatalf("expected binding store failure, got %v", err)
	}
}
