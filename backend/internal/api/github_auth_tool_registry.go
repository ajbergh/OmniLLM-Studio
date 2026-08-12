package api

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// githubAuthToolCredentialOptions adapts the user-scoped GitHub auth service to
// the tool registry without exposing token-bearing values outside backend
// execution. Status stays local/non-network; AccessToken may refresh only when a
// credentialed Git/GitHub operation already permits network access.
func githubAuthToolCredentialOptions(service *githubauth.Service) *tools.GitHubCredentialOptions {
	return githubAuthToolCredentialOptionsWithBindings(service, nil)
}

func githubAuthToolCredentialOptionsWithBindings(service *githubauth.Service, bindings GitHubRepositoryBindingStore) *tools.GitHubCredentialOptions {
	if service == nil {
		return nil
	}
	options := &tools.GitHubCredentialOptions{
		Connected: func(_ context.Context, userID string) (bool, error) {
			status, err := service.Status(userID)
			if err != nil {
				return false, err
			}
			// A persisted identity still owns credential precedence when its token
			// is stale. The execution path will surface reauthorization/refresh
			// failure and fail closed instead of falling back to TokenEnv.
			return status.Connected || status.GitHubUserID > 0 || strings.TrimSpace(status.GitHubLogin) != "", nil
		},
		Resolve: func(ctx context.Context, userID string) (string, bool, error) {
			token, err := service.AccessToken(ctx, userID)
			if errors.Is(err, githubauth.ErrNotConnected) {
				return "", false, nil
			}
			if err != nil {
				// The gitrepo adapter interprets connected=true with an error as a
				// fail-closed connection failure and will not fall back to TokenEnv.
				return "", true, err
			}
			return token, true, nil
		},
	}
	if bindings != nil {
		options.Bindings = func(_ context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error) {
			status, err := service.Status(userID)
			if err != nil {
				return nil, err
			}
			// Disconnect clears the persisted identity. A stale token retains the
			// same identity and therefore keeps the binding visible for inventory,
			// while actual network execution still fails closed via AccessToken.
			if status.GitHubUserID <= 0 {
				return nil, nil
			}
			stored, err := bindings.List(userID)
			if err != nil {
				return nil, err
			}
			active := make([]gitrepo.GitHubRemoteBinding, 0, len(stored))
			for _, binding := range stored {
				if binding.GitHubUserID != status.GitHubUserID {
					continue
				}
				active = append(active, gitrepo.GitHubRemoteBinding{
					LocalRepositoryID: binding.LocalRepositoryID,
					GitHubFullName:    binding.GitHubFullName,
					Disabled:          binding.Disabled,
				})
			}
			return active, nil
		}
	}
	return options
}

// configureGitHubAuthToolRegistry binds the shared GitHub auth runtime and the
// existing owner-scoped repository-binding store to the already-created registry.
// Missing app configuration is supported and preserves operator/public behavior.
func configureGitHubAuthToolRegistry(registry *tools.Registry, service *githubauth.Service, repositories *GitHubRepositoryHandler) {
	if registry == nil || service == nil {
		return
	}
	var bindings GitHubRepositoryBindingStore
	if repositories != nil {
		bindings = repositories.bindings
	}
	if !registry.ConfigureGitHubCredentials(githubAuthToolCredentialOptionsWithBindings(service, bindings)) {
		log.Printf("WARN: GitHub App tool credential wiring unavailable")
	}
}
