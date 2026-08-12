package api

import (
	"database/sql"

	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/githubrepo"
	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// NewGitHubRepositoryHandlerFromEnvironment composes user-scoped GitHub
// discovery with the same startup allowlist used by local Git tooling. The
// handler receives only stable local repository IDs; filesystem paths remain
// encapsulated inside gitrepo.Service.
func NewGitHubRepositoryHandlerFromEnvironment(database *sql.DB, authService *githubauth.Service) *GitHubRepositoryHandler {
	bindings := repository.NewGitHubRepositoryBindingRepo(database)
	locals := gitrepo.NewServiceFromEnvironment()
	if authService == nil {
		return NewGitHubRepositoryHandler(nil, nil, bindings, locals)
	}
	discovery := githubrepo.NewService(authService)
	return NewGitHubRepositoryHandler(discovery, authService, bindings, locals)
}

// MountGitHubRepositoryRoutes mounts repository selection/binding routes inside
// the existing authenticated GitHub surface.
func MountGitHubRepositoryRoutes(r chi.Router, handler *GitHubRepositoryHandler) {
	if r == nil || handler == nil {
		return
	}
	r.Get("/github/repositories", handler.ListRepositories)
	r.Get("/github/repository-bindings", handler.ListBindings)
	r.Put("/github/repository-bindings/{localRepositoryId}", handler.Bind)
	r.Delete("/github/repository-bindings/{localRepositoryId}", handler.DeleteBinding)
}
