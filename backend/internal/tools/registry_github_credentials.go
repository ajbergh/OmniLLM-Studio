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
// request-scoped credential dependencies without changing permission policy.
// When complete GitHub credential + binding callbacks are supplied, startup
// operator policy may additionally bootstrap binding-backed remote/GitHub tool
// shells even when no static remote grants the corresponding capability.
func NewRegistryWithOptions(options RegistryOptions) *Registry {
	registry := NewRegistry()
	registry.ConfigureGitHubCredentials(options.GitHubCredentials)
	return registry
}

// ConfigureGitHubCredentials rebinds configured remote and GitHub tool families
// to a request-scoped credential/binding adapter. Binding-aware setup may add
// tool shells only from startup operator policy plus the existing process-wide
// gates. Registration never calls the owner binding or token resolvers and never
// treats authentication as authorization.
func (r *Registry) ConfigureGitHubCredentials(options *GitHubCredentialOptions) bool {
	if r == nil {
		return false
	}
	resolver := registryGitHubCredentialResolver(options)
	if resolver == nil {
		return false
	}
	bindingResolver := registryGitHubBindingResolver(options)
	if bindingResolver != nil {
		r.bootstrapGitHubBindingRemoteTools()
	}

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

	if bindingResolver != nil {
		r.bootstrapGitHubBindingCapabilityTools(base)
	}
	r.rebindRemoteGitHubServices(gitrepo.NewUserScopedRemoteServiceWithBindings(base, resolver, bindingResolver))
	return true
}

// bootstrapGitHubBindingRemoteTools creates the remote read/inventory tool
// shells required for owner-scoped GitHub bindings when static remote
// configuration is absent. Binding lookup itself remains request-scoped and
// local-only; this function never calls the binding resolver or token resolver.
func (r *Registry) bootstrapGitHubBindingRemoteTools() bool {
	if r == nil {
		return false
	}
	if _, ok := r.Get("git_remotes"); ok {
		return true
	}
	if _, ok := r.Get("git_remote_status"); ok {
		return false
	}
	local := gitrepo.NewServiceFromEnvironment()
	if !local.Configured() {
		return false
	}
	base := gitrepo.NewRemoteServiceFromEnvironment(local)
	if !base.Enabled() {
		return false
	}
	for _, tool := range NewGitRemoteTools(base) {
		r.MustRegister(tool)
	}
	return true
}

// bootstrapGitHubBindingCapabilityTools adds missing tool shells that at least
// one startup-allowlisted binding policy can potentially authorize underneath
// its corresponding process-wide gate. It never consults owner connection,
// binding, or credential state. Exact request-scoped authority is rechecked by
// UserScopedRemoteService when a tool executes. Clone is intentionally excluded
// because binding-derived policy cannot grant it.
func (r *Registry) bootstrapGitHubBindingCapabilityTools(base *gitrepo.RemoteService) {
	if r == nil || base == nil {
		return
	}
	capabilities := base.GitHubBindingToolCapabilities()
	if capabilities.Fetch {
		r.registerMissingTools(NewGitRemoteMutationTools(base), nil)
	}
	if capabilities.Push || capabilities.BranchCreate {
		r.registerMissingTools(NewGitRemotePushTools(base), func(name string) bool {
			switch name {
			case "git_push":
				return capabilities.Push
			case "git_publish_branch":
				return capabilities.BranchCreate
			default:
				return false
			}
		})
	}
	if capabilities.PullRequestRead {
		r.registerMissingTools(NewGitHubPullRequestReadTools(base), nil)
		r.registerMissingTools(NewGitHubPullRequestReviewThreadTools(base), nil)
	}
	if capabilities.PullRequestCreate {
		r.registerMissingTools(NewGitHubPullRequestTools(base), nil)
	}
	if capabilities.PullRequestReply {
		r.registerMissingTools(NewGitHubPullRequestReplyTools(base), nil)
	}
	if capabilities.PullRequestThreadResolution {
		r.registerMissingTools(NewGitHubPullRequestReviewThreadResolutionTools(base), nil)
	}
	if capabilities.PullRequestReady {
		r.registerMissingTools(NewGitHubPullRequestReadyTools(base), nil)
	}
	if capabilities.PullRequestMerge {
		r.registerMissingTools(NewGitHubPullRequestMergeTools(base), nil)
	}
}

func (r *Registry) registerMissingTools(tools []Tool, allowed func(string) bool) {
	if r == nil {
		return
	}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		name := strings.TrimSpace(tool.Definition().Normalized().Name)
		if allowed != nil && !allowed(name) {
			continue
		}
		if _, exists := r.Get(name); exists {
			continue
		}
		r.MustRegister(tool)
	}
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
// by NewRegistry or the binding-aware bootstrap. It does not add mutation
// authority, bypass a global gate, or alter a tool definition or permission
// policy.
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
		case *githubPullRequestMergePolicyEvidenceTool:
			typed.service = service
		case *githubPullRequestMergeEligibilityTool:
			typed.service = service
		case *githubPullRequestMergeTool:
			typed.service = service
		}
	}
}
