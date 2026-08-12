package tools

import (
	"context"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

// GitHubCredentialOptions supplies user-scoped GitHub App state to the tool
// registry. Connected and Bindings must remain local/non-network; Resolve may
// refresh an expiring token and is invoked only by credentialed network tools.
type GitHubCredentialOptions struct {
	Connected func(ctx context.Context, userID string) (bool, error)
	Resolve   func(ctx context.Context, userID string) (token string, connected bool, err error)
	Bindings  func(ctx context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error)
}

// RegistryOptions contains optional application-service dependencies for the
// tool registry. The zero value preserves NewRegistry behavior.
type RegistryOptions struct {
	GitHubCredentials *GitHubCredentialOptions
}

// NewRegistryWithOptions creates the normal registry and applies optional
// request-scoped credential dependencies without changing tool registration or
// permission policy.
func NewRegistryWithOptions(options RegistryOptions) *Registry {
	registry := NewRegistry()
	registry.ConfigureGitHubCredentials(options.GitHubCredentials)
	return registry
}

// ConfigureGitHubCredentials rebinds the already-configured remote and GitHub
// tool families to a request-scoped credential/binding adapter. It returns false
// when credential callbacks are incomplete or the expected built-in remote
// service is unavailable. Bindings are optional and never alter global/static
// remote configuration or permission gates.
func (r *Registry) ConfigureGitHubCredentials(options *GitHubCredentialOptions) bool {
	if r == nil {
		return false
	}
	resolver := registryGitHubCredentialResolver(options)
	if resolver == nil {
		return false
	}
	bindingResolver := registryGitHubBindingResolver(options)

	tool, ok := r.Get("git_remotes")
	if !ok {
		return false
	}
	remoteTool, ok := tool.(*gitRemoteTool)
	if !ok {
		return false
	}
	var base *gitrepo.RemoteService
	switch service := remoteTool.service.(type) {
	case *gitrepo.RemoteService:
		base = service
	case *gitrepo.UserScopedRemoteService:
		base = service.RemoteService
	}
	if base == nil {
		return false
	}

	r.rebindRemoteGitHubServices(gitrepo.NewUserScopedRemoteServiceWithBindings(base, resolver, bindingResolver))
	return true
}

func registryGitHubCredentialResolver(options *GitHubCredentialOptions) gitrepo.GitHubCredentialResolver {
	if options == nil || options.Connected == nil || options.Resolve == nil {
		return nil
	}
	userID := registryGitHubOwnerFromContext
	return gitrepo.GitHubCredentialResolverFuncs{
		ConnectedFunc: func(ctx context.Context) (bool, error) {
			owner := userID(ctx)
			if owner == "" {
				return false, nil
			}
			return options.Connected(ctx, owner)
		},
		ResolveFunc: func(ctx context.Context) (string, bool, error) {
			owner := userID(ctx)
			if owner == "" {
				return "", false, nil
			}
			return options.Resolve(ctx, owner)
		},
	}
}

func registryGitHubBindingResolver(options *GitHubCredentialOptions) gitrepo.GitHubRemoteBindingResolver {
	if options == nil || options.Bindings == nil {
		return nil
	}
	return gitrepo.GitHubRemoteBindingResolverFunc(func(ctx context.Context) ([]gitrepo.GitHubRemoteBinding, error) {
		owner := registryGitHubOwnerFromContext(ctx)
		if owner == "" {
			return nil, nil
		}
		return options.Bindings(ctx, owner)
	})
}

func registryGitHubOwnerFromContext(ctx context.Context) string {
	return strings.TrimSpace(InvocationScopeFromContext(ctx).UserID)
}

// rebindRemoteGitHubServices updates only service interfaces already registered
// by NewRegistry. It does not add a tool, bypass a global gate, or alter a tool
// definition/permission policy.
func (r *Registry) rebindRemoteGitHubServices(service *gitrepo.UserScopedRemoteService) {
	if r == nil || service == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, tool := range r.tools {
		switch typed := tool.(type) {
		case *gitRemoteTool:
			typed.service = service
		case *gitRemoteFetchTool:
			typed.service = service
		case *gitRemotePushTool:
			typed.service = service
		case *gitRemotePublishBranchTool:
			typed.service = service
		case *gitRemoteCloneTool:
			typed.service = service
		case *githubDraftPullRequestTool:
			typed.service = service
		case *githubPullRequestReadTool:
			typed.service = service
		case *githubPullRequestFeedbackTool:
			typed.service = service
		case *githubPullRequestReviewThreadsTool:
			typed.service = service
		case *githubPullRequestReviewReplyTool:
			typed.service = service
		case *githubPullRequestReviewThreadResolutionTool:
			typed.service = service
		case *githubPullRequestReadyTool:
			typed.service = service
		case *githubPullRequestMergeRequirementsTool:
			typed.service = service
		}
	}
}
