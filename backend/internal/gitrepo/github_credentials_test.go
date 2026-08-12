package gitrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type testGitHubCredentialResolver struct {
	token       string
	connected   bool
	err         error
	statusErr   error
	calls       int
	statusCalls int
}

func (r *testGitHubCredentialResolver) GitHubCredentialConnected(context.Context) (bool, error) {
	r.statusCalls++
	return r.connected, r.statusErr
}

func (r *testGitHubCredentialResolver) ResolveGitHubCredential(context.Context) (string, bool, error) {
	r.calls++
	return r.token, r.connected, r.err
}

func testCredentialRemoteService(t *testing.T, remote RemoteConfig, env map[string]string) *RemoteService {
	t.Helper()
	lookup := func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
	return newRemoteService(map[string]RemoteConfig{"origin": remote}, true, true, nil, lookup)
}

func TestUserScopedRemoteServiceConnectedGitHubCredentialOverridesTokenEnv(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://github.com/acme/project.git",
		Username:   "operator-user",
		TokenEnv:   "GITHUB_TOKEN",
	}, map[string]string{"GITHUB_TOKEN": "operator-token"})
	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)

	service, err := scoped.scoped(context.Background(), "origin")
	if err != nil {
		t.Fatalf("scoped service: %v", err)
	}
	remote := service.remotes["origin"]
	if remote.TokenEnv != "GITHUB_TOKEN" {
		t.Fatalf("expected configured TokenEnv to remain stable, got %q", remote.TokenEnv)
	}
	if remote.Username != "operator-user" {
		t.Fatalf("expected configured username to remain stable, got %q", remote.Username)
	}
	if token, ok := service.lookupEnv("GITHUB_TOKEN"); !ok || token != "user-token" {
		t.Fatalf("expected connected user token to override TokenEnv, got %q, %v", token, ok)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one credential resolution, got %d", resolver.calls)
	}
	if resolver.statusCalls != 0 {
		t.Fatalf("network operation unexpectedly used status path %d times", resolver.statusCalls)
	}
}

func TestUserScopedRemoteServiceNoConnectionPreservesTokenEnvFallback(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://github.com/acme/project.git",
		Username:   "git",
		TokenEnv:   "GITHUB_TOKEN",
	}, map[string]string{"GITHUB_TOKEN": "operator-token"})
	resolver := &testGitHubCredentialResolver{connected: false}
	scoped := NewUserScopedRemoteService(base, resolver)

	service, err := scoped.scoped(context.Background(), "origin")
	if err != nil {
		t.Fatalf("scoped service: %v", err)
	}
	if service != base {
		t.Fatal("expected no-connection path to preserve the base service")
	}
	if token, ok := service.lookupEnv("GITHUB_TOKEN"); !ok || token != "operator-token" {
		t.Fatalf("expected TokenEnv fallback, got %q, %v", token, ok)
	}
}

func TestUserScopedRemoteServiceResolverFailureFailsClosed(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://github.com/acme/project.git",
		Username:   "git",
		TokenEnv:   "GITHUB_TOKEN",
	}, map[string]string{"GITHUB_TOKEN": "operator-token"})
	resolver := &testGitHubCredentialResolver{connected: true, err: errors.New("refresh secret detail")}
	scoped := NewUserScopedRemoteService(base, resolver)

	service, err := scoped.scoped(context.Background(), "origin")
	if err == nil || service != nil {
		t.Fatalf("expected fail-closed resolver error, got service=%v err=%v", service, err)
	}
	if strings.Contains(err.Error(), "refresh secret detail") || strings.Contains(err.Error(), "operator-token") {
		t.Fatalf("credential error leaked provider/operator detail: %v", err)
	}
	if !strings.Contains(err.Error(), "GitHub credentials are unavailable") {
		t.Fatalf("unexpected sanitized error: %v", err)
	}
}

func TestUserScopedRemoteServiceConnectedEmptyTokenFailsClosed(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://github.com/acme/project.git",
		Username:   "git",
		TokenEnv:   "GITHUB_TOKEN",
	}, map[string]string{"GITHUB_TOKEN": "operator-token"})
	resolver := &testGitHubCredentialResolver{connected: true, token: "   "}
	scoped := NewUserScopedRemoteService(base, resolver)

	if service, err := scoped.scoped(context.Background(), "origin"); err == nil || service != nil {
		t.Fatalf("expected empty connected credential to fail closed, got service=%v err=%v", service, err)
	}
}

func TestUserScopedRemoteServiceAppOnlyGitHubRemoteGetsSyntheticCredentialReference(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://github.com/acme/project.git",
	}, nil)
	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)

	service, err := scoped.scoped(context.Background(), "origin")
	if err != nil {
		t.Fatalf("scoped service: %v", err)
	}
	remote := service.remotes["origin"]
	if remote.TokenEnv != githubAppSyntheticTokenEnv {
		t.Fatalf("expected synthetic credential reference, got %q", remote.TokenEnv)
	}
	if remote.Username != "x-access-token" {
		t.Fatalf("expected safe GitHub HTTPS username, got %q", remote.Username)
	}
	if token, ok := service.lookupEnv(githubAppSyntheticTokenEnv); !ok || token != "user-token" {
		t.Fatalf("expected user token behind synthetic reference, got %q, %v", token, ok)
	}
	if base.remotes["origin"].TokenEnv != "" {
		t.Fatal("base remote configuration was mutated")
	}
}

func TestUserScopedRemoteServiceNeverAppliesGitHubCredentialToNonGitHubRemote(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository: "repo",
		URL:        "https://git.example.com/acme/project.git",
		Username:   "git",
		TokenEnv:   "OTHER_TOKEN",
	}, map[string]string{"OTHER_TOKEN": "operator-token"})
	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)

	service, err := scoped.scoped(context.Background(), "origin")
	if err != nil {
		t.Fatalf("scoped service: %v", err)
	}
	if service != base {
		t.Fatal("expected non-GitHub remote to use the base service")
	}
	if resolver.calls != 0 || resolver.statusCalls != 0 {
		t.Fatalf("GitHub credential resolver was used for non-GitHub remote: resolve=%d status=%d", resolver.calls, resolver.statusCalls)
	}
	if token, ok := service.lookupEnv("OTHER_TOKEN"); !ok || token != "operator-token" {
		t.Fatalf("non-GitHub credential changed: %q, %v", token, ok)
	}
}

func TestUserScopedRemoteServiceSummariesExposeAppOnlyGitHubCapabilitiesWithoutResolvingToken(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository:           "repo",
		URL:                  "https://github.com/acme/project.git",
		AllowPullRequestRead: true,
	}, nil)
	base.githubPullRequestReadEnabled = true
	resolver := &testGitHubCredentialResolver{token: "user-token", connected: true}
	scoped := NewUserScopedRemoteService(base, resolver)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 {
		t.Fatalf("expected one remote summary, got %d", len(summaries))
	}
	if !summaries[0].AuthenticationConfigured {
		t.Fatal("expected connected GitHub App credential to be reflected in safe summary")
	}
	if !summaries[0].PullRequestReadAllowed {
		t.Fatal("expected app-only GitHub PR read capability to remain visible")
	}
	if resolver.calls != 0 {
		t.Fatalf("git_remotes must not resolve or refresh a token; got %d resolve calls", resolver.calls)
	}
	if resolver.statusCalls != 1 {
		t.Fatalf("expected one local status check, got %d", resolver.statusCalls)
	}
}

func TestUserScopedRemoteServiceSummaryHidesEnvFallbackWhenConnectionStatusFails(t *testing.T) {
	base := testCredentialRemoteService(t, RemoteConfig{
		Repository:           "repo",
		URL:                  "https://github.com/acme/project.git",
		TokenEnv:             "GITHUB_TOKEN",
		Username:             "git",
		AllowPullRequestRead: true,
	}, map[string]string{"GITHUB_TOKEN": "operator-token"})
	base.githubPullRequestReadEnabled = true
	resolver := &testGitHubCredentialResolver{connected: true, statusErr: errors.New("credential store failed")}
	scoped := NewUserScopedRemoteService(base, resolver)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 {
		t.Fatalf("expected one remote summary, got %d", len(summaries))
	}
	if summaries[0].AuthenticationConfigured || summaries[0].PullRequestReadAllowed {
		t.Fatalf("failed connected credential status must not advertise TokenEnv fallback: %+v", summaries[0])
	}
	if resolver.calls != 0 {
		t.Fatalf("summary status failure must not resolve a token; got %d resolve calls", resolver.calls)
	}
}
