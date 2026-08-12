package api

import (
	"database/sql"
	"errors"
	"log"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// GitHubAuthRuntime groups the user-scoped authentication service with the
// repository-selection surface composed from the same database/runtime state.
// It is an API-layer composition object only; credentials remain owned by
// githubauth.Service and repository bindings remain owned by their repository.
type GitHubAuthRuntime struct {
	Service      *githubauth.Service
	Repositories *GitHubRepositoryHandler
}

// NewGitHubAuthRuntimeFromEnvironment composes the encrypted credential store
// with the GitHub App device-flow service and returns one runtime used by
// authenticated routes, repository discovery, and request-scoped Git/GitHub
// tools. Missing operator configuration remains a supported state.
func NewGitHubAuthRuntimeFromEnvironment(database *sql.DB) (*GitHubAuthRuntime, *GitHubAuthHandler) {
	store := repository.NewGitHubAppConnectionRepo(database)
	service, err := githubauth.NewServiceFromEnvironment(store)
	if err == nil {
		repositories := NewGitHubRepositoryHandlerFromEnvironment(database, service)
		handler := NewGitHubAuthHandler(service)
		handler.repositories = repositories
		return &GitHubAuthRuntime{Service: service, Repositories: repositories}, handler
	}
	if !errors.Is(err, githubauth.ErrNotConfigured) {
		log.Printf("WARN: GitHub App authentication unavailable: %v", err)
	}
	handler := NewGitHubAuthHandler(nil)
	handler.repositories = NewGitHubRepositoryHandlerFromEnvironment(database, nil)
	return nil, handler
}

// NewGitHubAuthHandlerFromEnvironment preserves the existing handler-only
// composition API for callers that do not need the shared runtime service.
func NewGitHubAuthHandlerFromEnvironment(database *sql.DB) *GitHubAuthHandler {
	_, handler := NewGitHubAuthRuntimeFromEnvironment(database)
	return handler
}

// MountGitHubAuthRoutes mounts user-scoped GitHub connection and repository
// selection routes. The caller must mount this inside the existing authenticated
// /v1 route group.
func MountGitHubAuthRoutes(r chi.Router, handler *GitHubAuthHandler) {
	if r == nil {
		return
	}
	if handler == nil {
		handler = NewGitHubAuthHandler(nil)
	}
	r.Get("/github/auth", handler.Status)
	r.Post("/github/auth/device/start", handler.StartDeviceAuthorization)
	r.Post("/github/auth/device/poll", handler.PollDeviceAuthorization)
	r.Delete("/github/auth", handler.Disconnect)
	MountGitHubRepositoryRoutes(r, handler.repositories)
}
