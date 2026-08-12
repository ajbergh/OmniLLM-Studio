package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestRegistryGitHubBindingResolverUsesInvocationOwner(t *testing.T) {
	var bindingUser string
	resolver := registryGitHubBindingResolver(&GitHubCredentialOptions{
		Bindings: func(_ context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error) {
			bindingUser = userID
			return []gitrepo.GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}, nil
		},
	})
	if resolver == nil {
		t.Fatal("expected binding resolver")
	}
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-456"})
	bindings, err := resolver.GitHubRemoteBindings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bindingUser != "user-456" || len(bindings) != 1 {
		t.Fatalf("binding callback escaped invocation owner: user=%q bindings=%#v", bindingUser, bindings)
	}
}

func TestRegistryGitHubBindingResolverUsesSoloOwner(t *testing.T) {
	var bindingUser string
	resolver := registryGitHubBindingResolver(&GitHubCredentialOptions{
		Bindings: func(_ context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error) {
			bindingUser = userID
			return nil, nil
		},
	})
	if resolver == nil {
		t.Fatal("expected binding resolver")
	}
	if _, err := resolver.GitHubRemoteBindings(context.Background()); err != nil {
		t.Fatal(err)
	}
	if bindingUser != "local" {
		t.Fatalf("expected stable solo owner local, got %q", bindingUser)
	}
}

func TestConfigureGitHubCredentialsCarriesOptionalBindingResolver(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	remoteTool := &gitRemoteTool{service: base, name: "git_remotes"}
	registry := &Registry{tools: map[string]Tool{"git_remotes": remoteTool}}
	var bindingUser string
	options := &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
		Resolve:   func(context.Context, string) (string, bool, error) { return "token", true, nil },
		Bindings: func(_ context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error) {
			bindingUser = userID
			return nil, nil
		},
	}
	if !registry.ConfigureGitHubCredentials(options) {
		t.Fatal("expected binding-aware configuration to succeed")
	}
	scoped, ok := remoteTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok {
		t.Fatalf("unexpected remote service type %T", remoteTool.service)
	}
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-789"})
	_ = scoped.Remotes(ctx)
	if bindingUser != "user-789" {
		t.Fatalf("configured binding resolver received %q", bindingUser)
	}
}
