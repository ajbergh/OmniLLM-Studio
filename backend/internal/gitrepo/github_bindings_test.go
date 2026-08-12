package gitrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type testGitHubBindingResolver struct {
	bindings []GitHubRemoteBinding
	err      error
	calls    int
}

func (r *testGitHubBindingResolver) GitHubRemoteBindings(context.Context) ([]GitHubRemoteBinding, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return append([]GitHubRemoteBinding(nil), r.bindings...), nil
}

func testBindingRemoteService(t *testing.T, remotes map[string]RemoteConfig) *RemoteService {
	t.Helper()
	local := NewServiceWithWriteAccess(map[string]string{
		"omni":  t.TempDir(),
		"other": t.TempDir(),
	}, true)
	base := newRemoteService(remotes, true, true, nil, func(string) (string, bool) { return "", false })
	base.local = local
	base.branchCreateEnabled = true
	base.githubPullRequestEnabled = true
	base.githubPullRequestReadEnabled = true
	base.githubPullRequestReplyEnabled = true
	base.githubPullRequestThreadResolutionEnabled = true
	base.githubPullRequestReadyEnabled = true
	base.cloneEnabled = true
	base.cloneMaxBytes = minRemoteCloneBytes
	base.cloneMaxEntries = minRemoteCloneEntries
	return base
}

func TestGitHubBindingRemoteIDIsDeterministicAndBounded(t *testing.T) {
	if got := GitHubBindingRemoteID("omni"); got != "github-omni" {
		t.Fatalf("short binding ID = %q", got)
	}
	long := strings.Repeat("a", maxRepositoryIDBytes)
	first := GitHubBindingRemoteID(long)
	second := GitHubBindingRemoteID(long)
	if first == "" || first != second || len(first) > maxRepositoryIDBytes || !ValidRepositoryID(first) {
		t.Fatalf("invalid deterministic binding ID: %q / %q", first, second)
	}
}

func TestUserScopedRemoteServiceSummariesIncludeBoundRemoteWithoutMutationCapabilities(t *testing.T) {
	base := testBindingRemoteService(t, map[string]RemoteConfig{})
	credentials := &testGitHubCredentialResolver{token: "user-token", connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 {
		t.Fatalf("expected one bound remote, got %#v", summaries)
	}
	summary := summaries[0]
	if summary.ID != "github-omni" || summary.Repository != "omni" || summary.Host != "github.com" || !summary.AuthenticationConfigured {
		t.Fatalf("unexpected bound summary: %#v", summary)
	}
	if summary.PushAllowed || summary.BranchCreateAllowed || summary.PullRequestReadAllowed || summary.PullRequestCreateAllowed ||
		summary.PullRequestReplyAllowed || summary.PullRequestThreadResolutionAllowed || summary.PullRequestReadyAllowed ||
		summary.DefaultBranchPushAllowed || summary.CloneAllowed {
		t.Fatalf("binding synthesized an authorization capability: %#v", summary)
	}
	if credentials.calls != 0 || credentials.statusCalls != 1 {
		t.Fatalf("git_remotes must be local-only: resolve=%d status=%d", credentials.calls, credentials.statusCalls)
	}
	if bindings.calls != 1 {
		t.Fatalf("expected one local binding lookup, got %d", bindings.calls)
	}
	if len(base.remotes) != 0 {
		t.Fatalf("base remote configuration was mutated: %#v", base.remotes)
	}
}

func TestUserScopedRemoteServiceScopedBoundRemoteInjectsCredentialOnlyAtExecution(t *testing.T) {
	base := testBindingRemoteService(t, map[string]RemoteConfig{})
	credentials := &testGitHubCredentialResolver{token: "user-token", connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	service, err := scoped.scoped(context.Background(), "github-omni")
	if err != nil {
		t.Fatal(err)
	}
	remote, ok := service.remotes["github-omni"]
	if !ok {
		t.Fatal("bound remote was not synthesized")
	}
	if remote.URL != "https://github.com/octo/studio.git" || remote.Repository != "omni" || remote.TokenEnv != githubAppSyntheticTokenEnv || remote.Username != "x-access-token" {
		t.Fatalf("unexpected bound remote: %#v", remote)
	}
	if remote.AllowPush || remote.AllowBranchCreate || remote.AllowPullRequestRead || remote.AllowPullRequestCreate ||
		remote.AllowPullRequestReply || remote.AllowPullRequestThreadResolution || remote.AllowPullRequestReady ||
		remote.AllowDefaultBranchPush || remote.AllowClone {
		t.Fatalf("bound remote has mutation permission: %#v", remote)
	}
	if token, ok := service.lookupEnv(githubAppSyntheticTokenEnv); !ok || token != "user-token" {
		t.Fatalf("bound credential unavailable: %q %v", token, ok)
	}
	if credentials.calls != 1 || credentials.statusCalls != 0 {
		t.Fatalf("execution credential calls: resolve=%d status=%d", credentials.calls, credentials.statusCalls)
	}
	if len(base.remotes) != 0 {
		t.Fatal("base remote configuration was mutated")
	}
}

func TestUserScopedRemoteServiceBindingValidationSkipsUnsafeOrInactiveBindings(t *testing.T) {
	base := testBindingRemoteService(t, map[string]RemoteConfig{})
	credentials := &testGitHubCredentialResolver{connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{
		{LocalRepositoryID: "omni", GitHubFullName: "https://evil.example/repo"},
		{LocalRepositoryID: "other", GitHubFullName: "octo/disabled", Disabled: true},
		{LocalRepositoryID: "missing", GitHubFullName: "octo/missing"},
	}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)
	if summaries := scoped.Remotes(context.Background()); len(summaries) != 0 {
		t.Fatalf("unsafe/inactive bindings were synthesized: %#v", summaries)
	}
}

func TestUserScopedRemoteServiceStaticRemoteWinsBindingIDCollision(t *testing.T) {
	static := RemoteConfig{Repository: "other", URL: "https://git.example.com/acme/static.git"}
	base := testBindingRemoteService(t, map[string]RemoteConfig{"github-omni": static})
	credentials := &testGitHubCredentialResolver{token: "user-token", connected: true}
	bindings := &testGitHubBindingResolver{bindings: []GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	service, err := scoped.scoped(context.Background(), "github-omni")
	if err != nil {
		t.Fatal(err)
	}
	if service != base || service.remotes["github-omni"].URL != static.URL {
		t.Fatalf("binding overrode static remote: %#v", service.remotes["github-omni"])
	}
	if bindings.calls != 0 || credentials.calls != 0 || credentials.statusCalls != 0 {
		t.Fatalf("static remote unexpectedly consulted GitHub binding/credential state: bindings=%d resolve=%d status=%d", bindings.calls, credentials.calls, credentials.statusCalls)
	}
}

func TestUserScopedRemoteServiceBindingLookupFailureDoesNotBreakStaticInventory(t *testing.T) {
	base := testBindingRemoteService(t, map[string]RemoteConfig{"origin": {Repository: "other", URL: "https://git.example.com/acme/static.git"}})
	credentials := &testGitHubCredentialResolver{connected: true}
	bindings := &testGitHubBindingResolver{err: errors.New("binding database secret detail")}
	scoped := NewUserScopedRemoteServiceWithBindings(base, credentials, bindings)

	summaries := scoped.Remotes(context.Background())
	if len(summaries) != 1 || summaries[0].ID != "origin" {
		t.Fatalf("static inventory lost on binding failure: %#v", summaries)
	}
	service, err := scoped.scoped(context.Background(), "github-omni")
	if err == nil || service != nil {
		t.Fatalf("dynamic binding lookup failure did not fail closed: service=%v err=%v", service, err)
	}
	if strings.Contains(err.Error(), "secret detail") {
		t.Fatalf("binding error detail leaked: %v", err)
	}
}
