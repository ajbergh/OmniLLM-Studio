package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
)

type testGitHubAuthCredentialStore struct {
	credential *githubauth.Credential
	err        error
}

func (s *testGitHubAuthCredentialStore) Get(string) (*githubauth.Credential, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.credential == nil {
		return nil, nil
	}
	copy := *s.credential
	return &copy, nil
}

func (s *testGitHubAuthCredentialStore) Save(_ string, credential githubauth.Credential) error {
	copy := credential
	s.credential = &copy
	return nil
}

func (s *testGitHubAuthCredentialStore) Clear(string) error {
	s.credential = nil
	return nil
}

func testGitHubAuthService(t *testing.T, store githubauth.CredentialStore) *githubauth.Service {
	t.Helper()
	service, err := githubauth.NewService(store, "test-client-id")
	if err != nil {
		t.Fatalf("new GitHub auth service: %v", err)
	}
	return service
}

func TestGitHubAuthToolCredentialOptionsNilService(t *testing.T) {
	if options := githubAuthToolCredentialOptions(nil); options != nil {
		t.Fatal("nil service must preserve existing operator/public credential behavior")
	}
}

func TestGitHubAuthToolCredentialOptionsNoConnectionAllowsFallback(t *testing.T) {
	service := testGitHubAuthService(t, &testGitHubAuthCredentialStore{})
	options := githubAuthToolCredentialOptions(service)
	if options == nil {
		t.Fatal("expected credential options")
	}
	connected, err := options.Connected(context.Background(), "user-1")
	if err != nil || connected {
		t.Fatalf("expected no stored connection, connected=%v err=%v", connected, err)
	}
	token, connected, err := options.Resolve(context.Background(), "user-1")
	if err != nil || connected || token != "" {
		t.Fatalf("ErrNotConnected must preserve fallback, token=%q connected=%v err=%v", token, connected, err)
	}
}

func TestGitHubAuthToolCredentialOptionsUsableConnectionWins(t *testing.T) {
	store := &testGitHubAuthCredentialStore{credential: &githubauth.Credential{
		AccessToken:  "user-token",
		TokenType:    "bearer",
		GitHubUserID: 42,
		GitHubLogin:  "octocat",
	}}
	service := testGitHubAuthService(t, store)
	options := githubAuthToolCredentialOptions(service)

	connected, err := options.Connected(context.Background(), "user-1")
	if err != nil || !connected {
		t.Fatalf("expected stored usable connection, connected=%v err=%v", connected, err)
	}
	token, connected, err := options.Resolve(context.Background(), "user-1")
	if err != nil || !connected || token != "user-token" {
		t.Fatalf("expected user credential, token=%q connected=%v err=%v", token, connected, err)
	}
}

func TestGitHubAuthToolCredentialOptionsReauthorizationStillOwnsPrecedence(t *testing.T) {
	expired := time.Now().UTC().Add(-time.Hour)
	store := &testGitHubAuthCredentialStore{credential: &githubauth.Credential{
		AccessToken:     "expired-user-token",
		TokenType:       "bearer",
		AccessExpiresAt: &expired,
		GitHubUserID:    42,
		GitHubLogin:     "octocat",
	}}
	service := testGitHubAuthService(t, store)
	options := githubAuthToolCredentialOptions(service)

	connected, err := options.Connected(context.Background(), "user-1")
	if err != nil || !connected {
		t.Fatalf("persisted identity must retain precedence, connected=%v err=%v", connected, err)
	}
	token, connected, err := options.Resolve(context.Background(), "user-1")
	if token != "" || !connected || !errors.Is(err, githubauth.ErrReauthorizationRequired) {
		t.Fatalf("reauthorization must fail closed, token=%q connected=%v err=%v", token, connected, err)
	}
}

func TestGitHubAuthToolCredentialOptionsStoreFailureFailsClosed(t *testing.T) {
	storeErr := errors.New("credential store unavailable")
	service := testGitHubAuthService(t, &testGitHubAuthCredentialStore{err: storeErr})
	options := githubAuthToolCredentialOptions(service)

	connected, err := options.Connected(context.Background(), "user-1")
	if connected || !errors.Is(err, storeErr) {
		t.Fatalf("status failure must surface locally, connected=%v err=%v", connected, err)
	}
	token, connected, err := options.Resolve(context.Background(), "user-1")
	if token != "" || !connected || !errors.Is(err, storeErr) {
		t.Fatalf("execution store failure must fail closed, token=%q connected=%v err=%v", token, connected, err)
	}
}
