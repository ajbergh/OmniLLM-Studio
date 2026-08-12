package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestRegistryGitHubCredentialResolverUsesInvocationOwner(t *testing.T) {
	var statusUser string
	var resolveUser string
	resolver := registryGitHubCredentialResolver(&GitHubCredentialOptions{
		Connected: func(_ context.Context, userID string) (bool, error) {
			statusUser = userID
			return true, nil
		},
		Resolve: func(_ context.Context, userID string) (string, bool, error) {
			resolveUser = userID
			return "token", true, nil
		},
	})
	if resolver == nil {
		t.Fatal("expected credential resolver")
	}

	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-123"})
	connected, err := resolver.GitHubCredentialConnected(ctx)
	if err != nil || !connected {
		t.Fatalf("status callback failed: connected=%v err=%v", connected, err)
	}
	if statusUser != "user-123" {
		t.Fatalf("status callback received %q", statusUser)
	}
	if token, connected, err := resolver.ResolveGitHubCredential(ctx); err != nil || !connected || token != "token" {
		t.Fatalf("resolve callback failed: token=%q connected=%v err=%v", token, connected, err)
	}
	if resolveUser != "user-123" {
		t.Fatalf("resolve callback received %q", resolveUser)
	}
}

func TestRegistryGitHubCredentialResolverUsesSoloOwner(t *testing.T) {
	var users []string
	resolver := registryGitHubCredentialResolver(&GitHubCredentialOptions{
		Connected: func(_ context.Context, userID string) (bool, error) {
			users = append(users, userID)
			return false, nil
		},
		Resolve: func(_ context.Context, userID string) (string, bool, error) {
			users = append(users, userID)
			return "", false, nil
		},
	})
	if resolver == nil {
		t.Fatal("expected credential resolver")
	}
	if _, err := resolver.GitHubCredentialConnected(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolver.ResolveGitHubCredential(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0] != "local" || users[1] != "local" {
		t.Fatalf("expected stable solo owner local, got %#v", users)
	}
}

func TestRegistryGitHubCredentialResolverRequiresCompleteCallbacks(t *testing.T) {
	if resolver := registryGitHubCredentialResolver(nil); resolver != nil {
		t.Fatal("nil options should preserve existing registry behavior")
	}
	if resolver := registryGitHubCredentialResolver(&GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
	}); resolver != nil {
		t.Fatal("partial credential options must not enable request-scoped credentials")
	}
}

func TestRegistryRebindUsesScopedServiceForRemoteAndHostedTools(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	resolver := gitrepo.GitHubCredentialResolverFuncs{
		ConnectedFunc: func(context.Context) (bool, error) { return true, nil },
		ResolveFunc:   func(context.Context) (string, bool, error) { return "token", true, nil },
	}
	scoped := gitrepo.NewUserScopedRemoteService(base, resolver)
	remoteTool := &gitRemoteTool{service: base, name: "git_remotes"}
	readTool := &githubPullRequestReadTool{service: base, name: "github_get_pull_request"}
	registry := &Registry{tools: map[string]Tool{
		"git_remotes":             remoteTool,
		"github_get_pull_request": readTool,
	}}

	registry.rebindRemoteGitHubServices(scoped)
	if remoteTool.service != scoped {
		t.Fatal("remote reader was not rebound to request-scoped service")
	}
	if readTool.service != scoped {
		t.Fatal("GitHub PR reader was not rebound to request-scoped service")
	}
}

func TestConfigureGitHubCredentialsRebindsExistingRegistry(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	remoteTool := &gitRemoteTool{service: base, name: "git_remotes"}
	readTool := &githubPullRequestReadTool{service: base, name: "github_get_pull_request"}
	registry := &Registry{tools: map[string]Tool{
		"git_remotes":             remoteTool,
		"github_get_pull_request": readTool,
	}}
	options := &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
		Resolve:   func(context.Context, string) (string, bool, error) { return "token", true, nil },
	}

	if !registry.ConfigureGitHubCredentials(options) {
		t.Fatal("expected post-construction GitHub credential configuration to succeed")
	}
	remoteScoped, ok := remoteTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok {
		t.Fatalf("remote tool was not rebound: %T", remoteTool.service)
	}
	readScoped, ok := readTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok {
		t.Fatalf("hosted GitHub tool was not rebound: %T", readTool.service)
	}
	if readScoped != remoteScoped {
		t.Fatal("hosted GitHub tools must share the exact scoped remote service")
	}
}

func TestConfigureGitHubCredentialsCanReplaceResolverWithoutNesting(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	remoteTool := &gitRemoteTool{service: base, name: "git_remotes"}
	registry := &Registry{tools: map[string]Tool{"git_remotes": remoteTool}}
	options := func(token string) *GitHubCredentialOptions {
		return &GitHubCredentialOptions{
			Connected: func(context.Context, string) (bool, error) { return true, nil },
			Resolve:   func(context.Context, string) (string, bool, error) { return token, true, nil },
		}
	}

	if !registry.ConfigureGitHubCredentials(options("first")) {
		t.Fatal("first configuration failed")
	}
	first, ok := remoteTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok || first.RemoteService != base {
		t.Fatalf("first configuration lost base service: %T", remoteTool.service)
	}
	if !registry.ConfigureGitHubCredentials(options("second")) {
		t.Fatal("second configuration failed")
	}
	second, ok := remoteTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok || second.RemoteService != base {
		t.Fatalf("second configuration nested or lost base service: %T", remoteTool.service)
	}
	if second == first {
		t.Fatal("expected resolver replacement to create a fresh scoped adapter")
	}
}

func TestConfigureGitHubCredentialsRejectsIncompleteOptions(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	if registry.ConfigureGitHubCredentials(nil) {
		t.Fatal("nil options must not modify the registry")
	}
	if registry.ConfigureGitHubCredentials(&GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
	}) {
		t.Fatal("incomplete options must not modify the registry")
	}
}
