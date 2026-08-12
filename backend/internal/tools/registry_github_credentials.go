package tools

import (
	"context"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

// GitHubCredentialOptions supplies user-scoped GitHub App credential state to
// the tool registry. Connected must remain local/non-network; Resolve may
// refresh an expiring token and is invoked only by credentialed network tools.
type GitHubCredentialOptions struct {
	Connected func(ctx context.Context, userID string) (bool, error)
	Resolve   func(ctx context.Context, userID string) (token string, connected bool, err error)
}

// RegistryOptions contains optional application-service dependencies for the
// tool registry. The zero value preserves NewRegistry behavior.
type RegistryOptions struct {
	GitHubCredentials *GitHubCredentialOptions
}

// NewRegistryWithOptions creates the normal registry and, when both GitHub App
// credential callbacks are supplied, rebinds the already-configured remote and
// GitHub tool families to a request-scoped credential adapter. The exact same
// RemoteService instance remains underneath the adapter, preserving the local
// service used for reviewed-state binding and every existing process/per-remote
// gate.
func NewRegistryWithOptions(options RegistryOptions) *Registry {
	registry := NewRegistry()
	resolver := registryGitHubCredentialResolver(options.GitHubCredentials)
	if resolver == nil {
		return registry
	}

	tool, ok := registry.Get("git_remotes")
	if !ok {
		return registry
	}
	remoteTool, ok := tool.(*gitRemoteTool)
	if !ok {
		return registry
	}
	base, ok := remoteTool.service.(*gitrepo.RemoteService)
	if !ok || base == nil {
		return registry
	}

	registry.rebindRemoteGitHubServices(gitrepo.NewUserScopedRemoteService(base, resolver))
	return registry
}

func registryGitHubCredentialResolver(options *GitHubCredentialOptions) gitrepo.GitHubCredentialResolver {
	if options == nil || options.Connected == nil || options.Resolve == nil {
		return nil
	}
	userID := func(ctx context.Context) string {
		return strings.TrimSpace(InvocationScopeFromContext(ctx).UserID)
	}
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
