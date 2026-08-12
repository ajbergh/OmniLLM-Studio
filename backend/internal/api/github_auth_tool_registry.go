package api

import (
	"context"
	"errors"
	"log"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// githubAuthToolCredentialOptions adapts the user-scoped GitHub auth service to
// the tool registry without exposing token-bearing values outside backend
// execution. Status stays local/non-network; AccessToken may refresh only when a
// credentialed Git/GitHub operation already permits network access.
func githubAuthToolCredentialOptions(service *githubauth.Service) *tools.GitHubCredentialOptions {
	if service == nil {
		return nil
	}
	return &tools.GitHubCredentialOptions{
		Connected: func(_ context.Context, userID string) (bool, error) {
			status, err := service.Status(userID)
			if err != nil {
				return false, err
			}
			return status.Connected, nil
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
}

// configureGitHubAuthToolRegistry binds the shared GitHub auth runtime to the
// already-created registry. Missing app configuration is a supported state and
// leaves the existing operator/public remote behavior unchanged.
func configureGitHubAuthToolRegistry(registry *tools.Registry, service *githubauth.Service) {
	if registry == nil || service == nil {
		return
	}
	if !registry.ConfigureGitHubCredentials(githubAuthToolCredentialOptions(service)) {
		log.Printf("WARN: GitHub App tool credential wiring unavailable")
	}
}
