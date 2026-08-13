package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestRegistryRebindUsesScopedServiceForDirectMerge(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	resolver := gitrepo.GitHubCredentialResolverFuncs{
		ConnectedFunc: func(context.Context) (bool, error) { return true, nil },
		ResolveFunc:   func(context.Context) (string, bool, error) { return "user-token", true, nil },
	}
	scoped := gitrepo.NewUserScopedRemoteService(base, resolver)
	merge := &githubPullRequestMergeTool{service: base}
	registry := &Registry{tools: map[string]Tool{"github_merge_pull_request": merge}}

	registry.rebindRemoteGitHubServices(scoped)
	if merge.service != scoped {
		t.Fatal("M3B merge tool was not rebound to request-scoped credential service")
	}
}
