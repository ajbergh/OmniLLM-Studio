package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestRegistryRebindUsesScopedServiceForMergeEvidenceLayers(t *testing.T) {
	base := gitrepo.NewRemoteServiceFromEnvironment(nil)
	resolver := gitrepo.GitHubCredentialResolverFuncs{
		ConnectedFunc: func(context.Context) (bool, error) { return true, nil },
		ResolveFunc:   func(context.Context) (string, bool, error) { return "user-token", true, nil },
	}
	scoped := gitrepo.NewUserScopedRemoteService(base, resolver)

	m1 := &githubPullRequestMergeRequirementsTool{service: base}
	m2 := &githubPullRequestMergePolicyEvidenceTool{service: base}
	m3a := &githubPullRequestMergeEligibilityTool{service: base}
	registry := &Registry{tools: map[string]Tool{
		"github_get_pull_request_merge_requirements":    m1,
		"github_get_pull_request_merge_policy_evidence": m2,
		"github_get_pull_request_merge_eligibility":     m3a,
	}}

	registry.rebindRemoteGitHubServices(scoped)

	if m1.service != scoped {
		t.Fatal("M1 merge requirements were not rebound to request-scoped credential service")
	}
	if m2.service != scoped {
		t.Fatal("M2 merge policy evidence was not rebound to request-scoped credential service")
	}
	if m3a.service != scoped {
		t.Fatal("M3A merge eligibility was not rebound to request-scoped credential service")
	}
}
